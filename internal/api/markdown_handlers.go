// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"encoding/json"
	"net/http"

	"github.com/TRC-Loop/cairn/internal/incident"
)

func previewMarkdown(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Markdown string `json:"markdown"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid JSON body", nil)
		return
	}
	if len(body.Markdown) > 20000 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "markdown too long", map[string]string{"markdown": FieldTooLong})
		return
	}
	html := incident.RenderMarkdown(body.Markdown)
	writeJSON(w, http.StatusOK, map[string]string{"html": string(html)})
}
