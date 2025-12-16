package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"delivery-system/internal/logger"
	"delivery-system/internal/models"
	"delivery-system/internal/services"
)

// PromoHandler обрабатывает промокоды.
type PromoHandler struct {
	promoService *services.PromoService
	log          *logger.Logger
}

// NewPromoHandler создаёт новый обработчик промокодов.
func NewPromoHandler(promoService *services.PromoService, log *logger.Logger) *PromoHandler {
	return &PromoHandler{
		promoService: promoService,
		log:          log,
	}
}

// CreatePromoCode создаёт промокод.
func (h *PromoHandler) CreatePromoCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.CreatePromoCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validatePromoRequest(req.Code, req.DiscountType, req.Amount); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	promo, err := h.promoService.CreatePromoCode(&req)
	if err != nil {
		if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "between") || strings.Contains(err.Error(), "non-negative") {
			writeErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		h.log.WithError(err).Error("Failed to create promo code")
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to create promo code")
		return
	}

	writeJSONResponse(w, http.StatusCreated, promo)
}

// ListPromoCodes возвращает список промокодов.
func (h *PromoHandler) ListPromoCodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	promos, err := h.promoService.ListPromoCodes(limit, offset)
	if err != nil {
		h.log.WithError(err).Error("Failed to list promo codes")
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to list promo codes")
		return
	}

	writeJSONResponse(w, http.StatusOK, promos)
}

// GetPromoCode возвращает промокод по коду.
func (h *PromoHandler) GetPromoCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	code, err := extractPromoCodeFromPath(r.URL.Path)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	promo, err := h.promoService.GetPromoCode(code)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, err.Error())
		} else {
			h.log.WithError(err).Error("Failed to get promo code")
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to get promo code")
		}
		return
	}

	writeJSONResponse(w, http.StatusOK, promo)
}

// UpdatePromoCode обновляет промокод.
func (h *PromoHandler) UpdatePromoCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	code, err := extractPromoCodeFromPath(r.URL.Path)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	var req models.UpdatePromoCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validatePromoRequest(code, req.DiscountType, req.Amount); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	promo, err := h.promoService.UpdatePromoCode(code, &req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "between") {
			writeErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		h.log.WithError(err).Error("Failed to update promo code")
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to update promo code")
		return
	}

	writeJSONResponse(w, http.StatusOK, promo)
}

// DeletePromoCode удаляет промокод.
func (h *PromoHandler) DeletePromoCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	code, err := extractPromoCodeFromPath(r.URL.Path)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.promoService.DeletePromoCode(code); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(w, http.StatusNotFound, err.Error())
		} else {
			h.log.WithError(err).Error("Failed to delete promo code")
			writeErrorResponse(w, http.StatusInternalServerError, "Failed to delete promo code")
		}
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{"message": "Promo code deleted"})
}

func validatePromoRequest(code string, discountType models.DiscountType, amount float64) error {
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("promo code is required")
	}
	if len(code) > 64 {
		return fmt.Errorf("promo code is too long")
	}
	// amount/type validated in service; keep simple checks for percent
	if discountType == models.DiscountTypePercent && (amount <= 0 || amount > 100) {
		return fmt.Errorf("percent amount must be between 0 and 100")
	}
	return nil
}

func extractPromoCodeFromPath(path string) (string, error) {
	if !strings.HasPrefix(path, "/api/promo-codes/") {
		return "", fmt.Errorf("invalid path format")
	}
	code := strings.TrimPrefix(path, "/api/promo-codes/")
	if code == "" {
		return "", fmt.Errorf("promo code is required")
	}
	// Отрезаем возможный суффикс со слешем
	code = strings.Split(code, "/")[0]
	return code, nil
}
