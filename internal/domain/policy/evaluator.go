package policy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"strconv"
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
	cacheScope string
	cacheTTL   time.Duration
	logger     *slog.Logger
	tracer     trace.Tracer
	events     metric.Int64Counter
	duration   metric.Float64Histogram
}

func NewEvaluator(client *Client, redis *redisstore.Store, cacheTTL time.Duration, logger *slog.Logger) *Evaluator {
	if logger == nil {
		logger = slog.Default()
	}
	meter := otel.Meter("promptgate-policy")
	events, _ := meter.Int64Counter("promptgate.policy.events")
	duration, _ := meter.Float64Histogram("promptgate.policy.ip_validation.duration", metric.WithUnit("ms"))
	scope := sha256.Sum256([]byte(client.urlTemplate + "\x00" + client.bearerToken + "\x00" + strconv.FormatInt(int64(cacheTTL), 10)))
	return &Evaluator{
		client: client, redis: redis, cacheScope: hex.EncodeToString(scope[:]),
		cacheTTL: cacheTTL, logger: logger,
		tracer: otel.Tracer("promptgate-policy"), events: events, duration: duration,
	}
}

func (e *Evaluator) Evaluate(ctx context.Context, clientIP string) (Decision, error) {
	ctx, span := e.tracer.Start(ctx, "policy.evaluate")
	defer span.End()
	ip, err := normalizeIP(clientIP)
	if err != nil {
		return Decision{}, err
	}
	key := cacheKey(e.cacheScope, ip)
	var cached Decision
	if e.redis != nil {
		if ok, cacheErr := e.redis.GetJSON(ctx, key, &cached); cacheErr == nil && ok {
			// Hits never refresh the TTL. Only explicit positive entries are trusted.
			if cached.Allow {
				e.record(ctx, "cache_hit", true)
				span.SetAttributes(attribute.Bool("policy.cache_hit", true), attribute.Bool("policy.allow", true))
				return Decision{Allow: true}, nil
			}
		} else if cacheErr != nil {
			e.logger.Warn("policy cache read failed", "error", cacheErr)
		}
	}
	e.record(ctx, "cache_miss", false)
	start := time.Now()
	decision, err := e.client.Decide(ctx, ip)
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
	e.record(ctx, "ip_validation", decision.Allow)
	span.SetAttributes(attribute.Bool("policy.cache_hit", false), attribute.Bool("policy.allow", decision.Allow), attribute.String("policy.reason", decision.Reason))
	e.logger.Info("policy decision", "source", "ip_validation", "allow", decision.Allow, "reason", decision.Reason)
	if e.redis != nil && decision.Allow {
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

func cacheKey(scope, clientIP string) string {
	sum := sha256.Sum256([]byte(scope + "\x00" + clientIP))
	return "promptgate:proxy:ip-validation:v1:" + hex.EncodeToString(sum[:])
}
