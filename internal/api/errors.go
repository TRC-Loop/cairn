// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"encoding/json"
	"net/http"
)

const (
	CodeValidationFailed       = "validation_failed"
	CodeInvalidJSON            = "invalid_json"
	CodeInvalidStatusTransition = "invalid_status_transition"
	CodeNotFound               = "not_found"
	CodeForbidden              = "forbidden"
	CodeUnauthorized           = "unauthorized"
	CodeConflict               = "conflict"
	CodeRateLimited            = "rate_limited"
	CodeInternalError          = "internal_error"
	CodeBadRequest             = "bad_request"

	FieldRequired       = "required"
	FieldTooShort       = "too_short"
	FieldTooLong        = "too_long"
	FieldInvalidFormat  = "invalid_format"
	FieldOutOfRange     = "out_of_range"
	FieldInvalidValue   = "invalid_value"
	FieldNotFound       = "not_found"
	FieldImmutable      = "immutable"
)

type ErrorResponse struct {
	Code        string            `json:"code"`
	Description string            `json:"description"`
	Fields      map[string]string `json:"fields,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, description string, fields map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Code:        code,
		Description: description,
		Fields:      fields,
	})
}
