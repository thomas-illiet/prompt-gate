package modellab

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	ticketIssuer    = "promptgate-api"
	ticketAudience  = "promptgate-model-lab"
	ticketOperation = "model-lab.generate"
)

type ticketRecord struct {
	TokenHash    string    `json:"tokenHash"`
	Subject      string    `json:"subject"`
	ProviderName string    `json:"providerName"`
	Model        string    `json:"model"`
	Protocol     Protocol  `json:"protocol"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	Origin       string    `json:"origin,omitempty"`
	ComparisonID string    `json:"comparisonId"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

func (s *Service) IssueTickets(
	ctx context.Context,
	subject string,
	role string,
	origin string,
	input IssueTicketsInput,
	access AccessFilter,
) (IssueTicketsResponse, error) {
	subject = strings.TrimSpace(subject)
	role = strings.TrimSpace(role)
	origin = strings.TrimSpace(origin)
	targets, err := normalizeTargets(input.Targets, s.maxTargets)
	if err != nil || subject == "" || role == "" {
		return IssueTicketsResponse{}, ErrInvalidTargets
	}
	if s.store == nil || !s.store.Enabled() {
		return IssueTicketsResponse{}, ErrTicketUnavailable
	}

	catalog, err := s.Catalog(ctx, access)
	if err != nil {
		return IssueTicketsResponse{}, fmt.Errorf("%w: %v", ErrCatalogUnavailable, err)
	}
	available := make(map[string]CatalogModel)
	providerStates := make(map[string]CatalogState)
	providerTypes := make(map[string]Protocol)
	for _, item := range catalog.Providers {
		providerStates[item.Name] = item.Catalog.State
		for _, model := range item.Models {
			available[targetKey(item.Name, model.ID)] = model
			providerTypes[item.Name] = model.Playground.Protocol
		}
	}
	for _, target := range targets {
		model, ok := available[targetKey(target.ProviderName, target.Model)]
		if !ok {
			if providerStates[target.ProviderName] == CatalogUnavailable {
				return IssueTicketsResponse{}, ErrCatalogUnavailable
			}
			return IssueTicketsResponse{}, ErrTargetNotAllowed
		}
		if !model.Playground.Supported {
			return IssueTicketsResponse{}, ErrTargetNotAllowed
		}
	}

	now := s.now().UTC().Truncate(time.Second)
	expiresAt := now.Add(s.ticketTTL)
	comparisonID := uuid.NewString()
	reservedIDs := make([]string, len(targets))
	for index := range targets {
		reservedIDs[index] = uuid.NewString()
	}
	if err := s.reserveTicketIDs(ctx, subject, reservedIDs, now, expiresAt); err != nil {
		return IssueTicketsResponse{}, err
	}
	response := IssueTicketsResponse{
		ComparisonID: comparisonID,
		ExpiresAt:    expiresAt,
		Tickets:      make([]IssuedTicket, 0, len(targets)),
	}
	storedKeys := make([]string, 0, len(targets))
	rollback := func() {
		if len(storedKeys) > 0 {
			_ = s.store.Del(context.WithoutCancel(ctx), storedKeys...)
		}
		s.releaseTicketIDs(context.WithoutCancel(ctx), subject, reservedIDs...)
	}

	for index, target := range targets {
		protocol := providerTypes[target.ProviderName]
		path := proxyPath(target.ProviderName, protocol)
		if path == "" {
			rollback()
			return IssueTicketsResponse{}, ErrTargetNotAllowed
		}
		jti := reservedIDs[index]
		claims := TicketClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    ticketIssuer,
				Subject:   subject,
				Audience:  jwt.ClaimStrings{ticketAudience},
				ExpiresAt: jwt.NewNumericDate(expiresAt),
				NotBefore: jwt.NewNumericDate(now),
				IssuedAt:  jwt.NewNumericDate(now),
				ID:        jti,
			},
			Role:         role,
			ProviderName: target.ProviderName,
			Model:        target.Model,
			Protocol:     protocol,
			Operation:    ticketOperation,
			Method:       http.MethodPost,
			Path:         path,
			Origin:       origin,
			ComparisonID: comparisonID,
		}
		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.ticketSecret)
		if err != nil {
			rollback()
			return IssueTicketsResponse{}, fmt.Errorf("%w: sign ticket", ErrTicketUnavailable)
		}
		rawToken := TicketPrefix + signed
		record := ticketRecord{
			TokenHash:    ticketHash(rawToken),
			Subject:      subject,
			ProviderName: target.ProviderName,
			Model:        target.Model,
			Protocol:     protocol,
			Method:       http.MethodPost,
			Path:         path,
			Origin:       origin,
			ComparisonID: comparisonID,
			ExpiresAt:    expiresAt,
		}
		key := ticketStoreKey(jti)
		if err := s.store.SetJSON(ctx, key, record, expiresAt.Sub(now)); err != nil {
			rollback()
			return IssueTicketsResponse{}, fmt.Errorf("%w: store ticket", ErrTicketUnavailable)
		}
		storedKeys = append(storedKeys, key)
		response.Tickets = append(response.Tickets, IssuedTicket{
			ProviderName: target.ProviderName,
			Model:        target.Model,
			Protocol:     protocol,
			Method:       http.MethodPost,
			Endpoint:     s.proxyBaseURL + path,
			Token:        rawToken,
		})
	}
	return response, nil
}

// ParseTicket verifies the signed envelope but does not consume its Redis
// record. Call ConsumeTicket before allowing any upstream request.
func (s *Service) ParseTicket(rawToken string) (TicketClaims, error) {
	rawToken = normalizeRawTicket(rawToken)
	if rawToken == "" {
		return TicketClaims{}, ErrInvalidTicket
	}
	claims := &TicketClaims{}
	parsed, err := jwt.ParseWithClaims(
		strings.TrimPrefix(rawToken, TicketPrefix),
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidTicket
			}
			return s.ticketSecret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(ticketIssuer),
		jwt.WithAudience(ticketAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(2*time.Second),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil || parsed == nil || !parsed.Valid || !validRequiredClaims(*claims) {
		return TicketClaims{}, ErrInvalidTicket
	}
	return *claims, nil
}

// ConsumeTicket validates request binding and atomically destroys the Redis
// record. Exactly one concurrent caller can succeed.
func (s *Service) ConsumeTicket(ctx context.Context, input ConsumeTicketInput) (TicketClaims, error) {
	rawToken := normalizeRawTicket(input.Token)
	claims, err := s.ParseTicket(rawToken)
	if err != nil {
		return TicketClaims{}, err
	}
	if !requestMatchesClaims(input, claims) {
		return TicketClaims{}, ErrInvalidTicket
	}
	if s.store == nil || !s.store.Enabled() {
		return TicketClaims{}, ErrTicketUnavailable
	}

	var record ticketRecord
	found, err := s.store.GetDelJSON(ctx, ticketStoreKey(claims.ID), &record)
	if err != nil {
		return TicketClaims{}, fmt.Errorf("%w: consume ticket", ErrTicketUnavailable)
	}
	if !found || !recordMatchesClaims(record, claims) || !constantTimeEqual(record.TokenHash, ticketHash(rawToken)) {
		return TicketClaims{}, ErrInvalidTicket
	}
	s.releaseTicketIDs(context.WithoutCancel(ctx), claims.Subject, claims.ID)
	return claims, nil
}

func normalizeTargets(targets []TicketTarget, maxTargets int) ([]TicketTarget, error) {
	if maxTargets <= 0 || maxTargets > MaxTargets || len(targets) < 1 || len(targets) > maxTargets {
		return nil, ErrInvalidTargets
	}
	seen := make(map[string]struct{}, len(targets))
	out := make([]TicketTarget, 0, len(targets))
	for _, target := range targets {
		target.ProviderName = strings.TrimSpace(target.ProviderName)
		target.Model = strings.TrimSpace(target.Model)
		if target.ProviderName == "" || target.Model == "" {
			return nil, ErrInvalidTargets
		}
		key := targetKey(target.ProviderName, target.Model)
		if _, ok := seen[key]; ok {
			return nil, ErrInvalidTargets
		}
		seen[key] = struct{}{}
		out = append(out, target)
	}
	return out, nil
}

func targetKey(providerName, model string) string {
	return providerName + "\x00" + model
}

func proxyPath(providerName string, protocol Protocol) string {
	switch protocol {
	case ProtocolOpenAIChat:
		return "/" + providerName + "/v1/chat/completions"
	case ProtocolAnthropicMessages:
		return "/" + providerName + "/v1/messages"
	default:
		return ""
	}
}

func normalizeRawTicket(rawToken string) string {
	rawToken = strings.TrimSpace(rawToken)
	if len(rawToken) > len("Bearer ") && strings.EqualFold(rawToken[:len("Bearer ")], "Bearer ") {
		rawToken = strings.TrimSpace(rawToken[len("Bearer "):])
	}
	if !strings.HasPrefix(rawToken, TicketPrefix) {
		return ""
	}
	return rawToken
}

func validRequiredClaims(claims TicketClaims) bool {
	return strings.TrimSpace(claims.Subject) != "" &&
		strings.TrimSpace(claims.ID) != "" &&
		strings.TrimSpace(claims.Role) != "" &&
		strings.TrimSpace(claims.ProviderName) != "" &&
		strings.TrimSpace(claims.Model) != "" &&
		claims.Operation == ticketOperation &&
		claims.Method == http.MethodPost &&
		claims.Path == proxyPath(claims.ProviderName, claims.Protocol) &&
		strings.TrimSpace(claims.ComparisonID) != ""
}

func requestMatchesClaims(input ConsumeTicketInput, claims TicketClaims) bool {
	if input.Method != claims.Method {
		return false
	}
	if strings.TrimSpace(input.Path) != claims.Path {
		return false
	}
	if claims.Origin != "" && strings.TrimSpace(input.Origin) != claims.Origin {
		return false
	}
	return true
}

func recordMatchesClaims(record ticketRecord, claims TicketClaims) bool {
	return record.Subject == claims.Subject &&
		record.ProviderName == claims.ProviderName &&
		record.Model == claims.Model &&
		record.Protocol == claims.Protocol &&
		record.Method == claims.Method &&
		record.Path == claims.Path &&
		record.Origin == claims.Origin &&
		record.ComparisonID == claims.ComparisonID &&
		record.ExpiresAt.Equal(claims.ExpiresAt.Time)
}

func ticketStoreKey(jti string) string {
	return "promptgate:model-lab:ticket:v1:" + jti
}

func ticketHash(rawToken string) string {
	hash := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(hash[:])
}

func constantTimeEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
