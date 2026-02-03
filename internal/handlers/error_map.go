package handlers

import (
	"net/http"

	"delivery-system/internal/apperror"
	"delivery-system/internal/logger"
)

func writeServiceError(w http.ResponseWriter, log *logger.Logger, err error, internalMessage string) {
	switch {
	case apperror.Is(err, apperror.KindNotFound):
		writeErrorResponse(w, http.StatusNotFound, err.Error())
	case apperror.Is(err, apperror.KindValidation):
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
	case apperror.Is(err, apperror.KindConflict):
		writeErrorResponse(w, http.StatusConflict, err.Error())
	default:
		if log != nil {
			log.WithError(err).Error(internalMessage)
		}
		writeErrorResponse(w, http.StatusInternalServerError, internalMessage)
	}
}
