package admin

import (
	"errors"
	"net/http"

	"promptgate/backend/internal/domain/pricing"
)

func (h *Handler) HandleAdminGetPricing(w http.ResponseWriter, r *http.Request) {
	if h.pricing == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "pricing service unavailable"})
		return
	}
	config, err := h.pricing.Config(r.Context())
	if err != nil {
		writePricingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) HandleAdminUpdatePricing(w http.ResponseWriter, r *http.Request) {
	if h.pricing == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "pricing service unavailable"})
		return
	}
	var input pricing.UpdateConfigInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	config, err := h.pricing.UpdateConfig(r.Context(), input)
	if err != nil {
		writePricingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) HandleAdminUpdatePricingFallback(w http.ResponseWriter, r *http.Request) {
	if h.pricing == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "pricing service unavailable"})
		return
	}
	var input pricing.PriceRates
	if !decodeRequestBody(w, r, &input) {
		return
	}
	fallback, err := h.pricing.UpdateFallback(r.Context(), input)
	if err != nil {
		writePricingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fallback)
}

func (h *Handler) HandleAdminCreateModelPrice(w http.ResponseWriter, r *http.Request) {
	if h.pricing == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "pricing service unavailable"})
		return
	}
	var input pricing.ModelPriceRecord
	if !decodeRequestBody(w, r, &input) {
		return
	}
	record, err := h.pricing.CreateModelPrice(r.Context(), input)
	if err != nil {
		writePricingError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func (h *Handler) HandleAdminGetModelPrice(w http.ResponseWriter, r *http.Request) {
	if h.pricing == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "pricing service unavailable"})
		return
	}
	record, err := h.pricing.GetModelPrice(r.Context(), r.PathValue("id"))
	if err != nil {
		writePricingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *Handler) HandleAdminUpdateModelPrice(w http.ResponseWriter, r *http.Request) {
	if h.pricing == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "pricing service unavailable"})
		return
	}
	var input pricing.ModelPriceRecord
	if !decodeRequestBody(w, r, &input) {
		return
	}
	record, err := h.pricing.UpdateModelPrice(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writePricingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *Handler) HandleAdminDeleteModelPrice(w http.ResponseWriter, r *http.Request) {
	if h.pricing == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "pricing service unavailable"})
		return
	}
	if err := h.pricing.DeleteModelPrice(r.Context(), r.PathValue("id")); err != nil {
		writePricingError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) HandleAdminPricingConfigurationCheck(w http.ResponseWriter, r *http.Request) {
	if h.pricing == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "pricing service unavailable"})
		return
	}
	check, err := h.pricing.ConfigurationCheck(r.Context())
	if err != nil {
		writePricingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, check)
}

func writePricingError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := err.Error()
	switch {
	case errors.Is(err, pricing.ErrInvalidPrice):
		status = http.StatusBadRequest
		code = "invalid_price"
	case errors.Is(err, pricing.ErrInvalidPriceTarget):
		status = http.StatusBadRequest
		code = "invalid_price_target"
	case errors.Is(err, pricing.ErrImmutablePriceTarget):
		status = http.StatusBadRequest
		code = "immutable_price_target"
	case errors.Is(err, pricing.ErrPriceProviderNotFound):
		status = http.StatusNotFound
		code = "provider_not_found"
	case errors.Is(err, pricing.ErrPriceNotFound):
		status = http.StatusNotFound
		code = "pricing_not_found"
	case errors.Is(err, pricing.ErrPriceConflict):
		status = http.StatusConflict
		code = "pricing_conflict"
	}
	writeJSON(w, status, map[string]string{"error": code})
}
