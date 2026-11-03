package httpapi

import (
	"context"
	"net/http"
	"time"
)

type healthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// handleHealth returns the liveness payload for the API server.
func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	resp := healthResponse{Status: "ok", Checks: make(map[string]string)}
	if s.redis == nil || s.redis.Client() == nil || s.redis.Client().Ping(ctx).Err() != nil {
		resp.Checks["redis"] = "error"
		resp.Status = "degraded"
	} else {
		resp.Checks["redis"] = "ok"
	}
	if err := s.db.WithContext(ctx).Exec("SELECT 1").Error; err != nil {
		resp.Checks["database"] = "error"
		resp.Status = "degraded"
	} else {
		resp.Checks["database"] = "ok"
	}

	code := http.StatusOK
	if resp.Status != "ok" {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, resp)
}
