package policy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"promptgate/backend/internal/platform/redisstore"
)

type Evaluator struct {
	client     *Client
	redis      *redisstore.Store
	policyPath string
	cacheTTL   time.Duration
	logger     *slog.Logger
	tracer     trace.Tracer
	events     metric.Int64Counter
	duration   metric.Float64Histogram
}

func NewEvaluator(client *Client, redis *redisstore.Store, policyPath string, cacheTTL time.Duration, logger *slog.Logger) *Evaluator {
	if logger == nil {
		logger = slog.Default()
	}
	meter := otel.Meter("promptgate-policy")
	events, _ := meter.Int64Counter("promptgate.policy.events")
	duration, _ := meter.Float64Histogram("promptgate.policy.opa.duration", metric.WithUnit("ms"))
	return &Evaluator{
		client: client, redis: redis, policyPath: policyPath, cacheTTL: cacheTTL, logger: logger,
		tracer: otel.Tracer("promptgate-policy"), events: events, duration: duration,
	}
}

func (e *Evaluator) Evaluate(ctx context.Context, input Input) (Decision, error) {
	ctx, span := e.tracer.Start(ctx, "policy.evaluate")
	defer span.End()
	key, err := CacheKey(e.policyPath, input)
	if err != nil {
		return Decision{}, err
	}
	var cached Decision
	if e.redis != nil {
		if ok, cacheErr := e.redis.GetJSON(ctx, key, &cached); cacheErr == nil && ok {
			e.record(ctx, "cache_hit", cached.Allow)
			span.SetAttributes(attribute.Bool("policy.cache_hit", true), attribute.Bool("policy.allow", cached.Allow), attribute.String("policy.reason", cached.Reason))
			e.logger.Info("policy decision", "source", "cache", "allow", cached.Allow, "reason", cached.Reason)
			return cached, nil
		} else if cacheErr != nil {
			e.logger.Warn("policy cache read failed", "error", cacheErr)
		}
	}
	e.record(ctx, "cache_miss", false)
	start := time.Now()
	decision, err := e.client.Decide(ctx, input)
	e.duration.Record(ctx, float64(time.Since(start).Microseconds())/1000)
	if err != nil {
		outcome := "unavailable"
		if errors.Is(err, context.DeadlineExceeded) {
			outcome = "timeout"
		} else if errors.Is(err, ErrInvalidResponse) {
			outcome = "invalid_response"
		}
		e.record(ctx, outcome, false)
		e.logger.Error("policy decision unavailable", "outcome", outcome, "error", err)
		span.RecordError(err)
		return Decision{}, err
	}
	e.record(ctx, "opa", decision.Allow)
	span.SetAttributes(attribute.Bool("policy.cache_hit", false), attribute.Bool("policy.allow", decision.Allow), attribute.String("policy.reason", decision.Reason))
	e.logger.Info("policy decision", "source", "opa", "allow", decision.Allow, "reason", decision.Reason)
	if e.redis != nil {
		if cacheErr := e.redis.SetJSON(ctx, key, decision, e.cacheTTL); cacheErr != nil {
			e.logger.Warn("policy cache write failed", "error", cacheErr)
		}
	}
	return decision, nil
}

func (e *Evaluator) record(ctx context.Context, source string, allowed bool) {
	e.events.Add(ctx, 1, metric.WithAttributes(
		attribute.String("policy.source", source),
		attribute.Bool("policy.allow", allowed),
	))
}

func CacheKey(policyPath string, input Input) (string, error) {
	payload, err := json.Marshal(struct {
		PolicyPath string `json:"policy_path"`
		Input      Input  `json:"input"`
	}{PolicyPath: policyPath, Input: input})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return "promptgate:proxy:policy:" + hex.EncodeToString(sum[:]), nil
}
