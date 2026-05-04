// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/TRC-Loop/cairn/internal/crypto"
	"github.com/TRC-Loop/cairn/internal/notifier"
	"github.com/TRC-Loop/cairn/internal/store"
)

type NotificationsHandler struct {
	q          *store.Queries
	db         *sql.DB
	secretBox  *crypto.SecretBox
	dispatcher NotificationEnqueuer
	logger     *slog.Logger
}

// NotificationEnqueuer is the narrow surface this handler needs from the
// dispatcher. Defined here to keep the api package free of a notifier import
// dependency for tests.
type NotificationEnqueuer interface {
	Enqueue(ctx context.Context, channelID int64, eventType string, eventID int64, payload notifier.Payload, opts notifier.EnqueueOptions) (int64, error)
}

func NewNotificationsHandler(q *store.Queries, db *sql.DB, secretBox *crypto.SecretBox, dispatcher NotificationEnqueuer, logger *slog.Logger) *NotificationsHandler {
	return &NotificationsHandler{q: q, db: db, secretBox: secretBox, dispatcher: dispatcher, logger: logger}
}

type channelResponse struct {
	ID                  int64          `json:"id"`
	Name                string         `json:"name"`
	Type                string         `json:"type"`
	Enabled             bool           `json:"enabled"`
	RetryMax            int64          `json:"retry_max"`
	RetryBackoffSeconds int64          `json:"retry_backoff_seconds"`
	Config              map[string]any `json:"config"`
	UsedByCheckCount    int64          `json:"used_by_check_count"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

// scrubConfig returns a copy of the channel config with secret fields
// replaced by a placeholder, so plaintext credentials never leave the server.
func scrubConfig(typ string, raw string) map[string]any {
	cfg := map[string]any{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	switch typ {
	case notifier.ChannelEmail:
		delete(cfg, "smtp_password_enc")
		if _, ok := cfg["smtp_password"]; ok {
			cfg["smtp_password"] = ""
		}
		cfg["smtp_password_set"] = hasNonEmpty(raw, "smtp_password_enc")
	case notifier.ChannelDiscord:
		delete(cfg, "webhook_url_enc")
		cfg["webhook_url_set"] = hasNonEmpty(raw, "webhook_url_enc")
	case notifier.ChannelWebhook:
		delete(cfg, "secret_enc")
		cfg["secret_set"] = hasNonEmpty(raw, "secret_enc")
	}
	return cfg
}

func hasNonEmpty(raw, key string) bool {
	if raw == "" {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	s, _ := v.(string)
	return s != ""
}

func (h *NotificationsHandler) toChannelResponse(ctx context.Context, c store.NotificationChannel) channelResponse {
	count, _ := h.q.CountChecksForChannel(ctx, c.ID)
	return channelResponse{
		ID:                  c.ID,
		Name:                c.Name,
		Type:                c.Type,
		Enabled:             c.Enabled,
		RetryMax:            c.RetryMax,
		RetryBackoffSeconds: c.RetryBackoffSeconds,
		Config:              scrubConfig(c.Type, c.ConfigJson),
		UsedByCheckCount:    count,
		CreatedAt:           c.CreatedAt,
		UpdatedAt:           c.UpdatedAt,
	}
}

func (h *NotificationsHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.q.ListNotificationChannels(r.Context())
	if err != nil {
		h.logger.Error("list channels failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	out := make([]channelResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, h.toChannelResponse(r.Context(), c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": out})
}

func (h *NotificationsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	c, err := h.q.GetNotificationChannel(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("get channel failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channel": h.toChannelResponse(r.Context(), c)})
}

type channelWriteRequest struct {
	Name                *string         `json:"name"`
	Type                *string         `json:"type"`
	Enabled             *bool           `json:"enabled"`
	RetryMax            *int64          `json:"retry_max"`
	RetryBackoffSeconds *int64          `json:"retry_backoff_seconds"`
	Config              json.RawMessage `json:"config"`
}

var validChannelTypes = map[string]struct{}{
	notifier.ChannelEmail: {}, notifier.ChannelDiscord: {}, notifier.ChannelWebhook: {},
}

func (h *NotificationsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req channelWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	if req.Name == nil || req.Type == nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"name": FieldRequired, "type": FieldRequired})
		return
	}
	if _, ok := validChannelTypes[*req.Type]; !ok {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"type": FieldInvalidValue})
		return
	}
	if fields := validateChannelMeta(*req.Name, valueOr(req.RetryMax, 3), valueOr(req.RetryBackoffSeconds, 1)); len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}

	cfgJSON, fields, err := h.encodeConfig(*req.Type, req.Config, "")
	if err != nil {
		h.logger.Error("encode config failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	row, err := h.q.CreateNotificationChannel(r.Context(), store.CreateNotificationChannelParams{
		Name:                *req.Name,
		Type:                *req.Type,
		Enabled:             enabled,
		ConfigJson:          cfgJSON,
		RetryMax:            valueOr(req.RetryMax, 3),
		RetryBackoffSeconds: valueOr(req.RetryBackoffSeconds, 1),
	})
	if err != nil {
		if isUniqueErr(err) {
			writeError(w, http.StatusConflict, CodeConflict, "name must be unique", map[string]string{"name": "conflict"})
			return
		}
		h.logger.Error("create channel failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("notification channel created", "id", row.ID, "type", row.Type)
	writeJSON(w, http.StatusCreated, map[string]any{"channel": h.toChannelResponse(r.Context(), row)})
}

func (h *NotificationsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	existing, err := h.q.GetNotificationChannel(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	var req channelWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	if req.Type != nil && *req.Type != existing.Type {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "cannot change type", map[string]string{"type": FieldImmutable})
		return
	}

	name := existing.Name
	if req.Name != nil {
		name = *req.Name
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	retryMax := existing.RetryMax
	if req.RetryMax != nil {
		retryMax = *req.RetryMax
	}
	backoff := existing.RetryBackoffSeconds
	if req.RetryBackoffSeconds != nil {
		backoff = *req.RetryBackoffSeconds
	}
	if fields := validateChannelMeta(name, retryMax, backoff); len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}

	cfgJSON := existing.ConfigJson
	if len(req.Config) > 0 {
		newJSON, fields, err := h.encodeConfig(existing.Type, req.Config, existing.ConfigJson)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}
		if len(fields) > 0 {
			writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
			return
		}
		cfgJSON = newJSON
	}

	row, err := h.q.UpdateNotificationChannel(r.Context(), store.UpdateNotificationChannelParams{
		Name: name, Enabled: enabled, ConfigJson: cfgJSON,
		RetryMax: retryMax, RetryBackoffSeconds: backoff, ID: id,
	})
	if err != nil {
		if isUniqueErr(err) {
			writeError(w, http.StatusConflict, CodeConflict, "name must be unique", map[string]string{"name": "conflict"})
			return
		}
		h.logger.Error("update channel failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("notification channel updated", "id", row.ID)
	writeJSON(w, http.StatusOK, map[string]any{"channel": h.toChannelResponse(r.Context(), row)})
}

func (h *NotificationsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.q.DeleteNotificationChannel(r.Context(), id); err != nil {
		h.logger.Error("delete channel failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("notification channel deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationsHandler) Test(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	channel, err := h.q.GetNotificationChannel(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if !channel.Enabled {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "channel disabled", nil)
		return
	}
	if h.dispatcher == nil {
		writeError(w, http.StatusServiceUnavailable, CodeInternalError, "dispatcher not available", nil)
		return
	}
	payload := notifier.Payload{
		EventType: notifier.EventTest,
		EventID:   0,
		Subject:   fmt.Sprintf("Cairn test notification: %s", channel.Name),
		Body:      "This is a test notification from Cairn — your channel is configured correctly.",
		Severity:  "info",
		Timestamp: time.Now().UTC(),
	}
	deliveryID, err := h.dispatcher.Enqueue(r.Context(), channel.ID, notifier.EventTest, 0, payload, notifier.EnqueueOptions{})
	if err != nil {
		h.logger.Error("test enqueue failed", "channel_id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "enqueue failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"delivery_id": deliveryID, "status": "pending"})
}

type deliveryResponse struct {
	ID              int64      `json:"id"`
	ChannelID       int64      `json:"channel_id"`
	EventType       string     `json:"event_type"`
	EventID         int64      `json:"event_id"`
	Status          string     `json:"status"`
	AttemptCount    int64      `json:"attempt_count"`
	LastError       *string    `json:"last_error"`
	LastAttemptedAt *time.Time `json:"last_attempted_at"`
	NextAttemptAt   *time.Time `json:"next_attempt_at"`
	SentAt          *time.Time `json:"sent_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

func toDeliveryResponse(d store.NotificationDelivery) deliveryResponse {
	out := deliveryResponse{
		ID: d.ID, ChannelID: d.ChannelID, EventType: d.EventType, EventID: d.EventID,
		Status: d.Status, AttemptCount: d.AttemptCount, CreatedAt: d.CreatedAt,
	}
	if d.LastError.Valid {
		v := d.LastError.String
		out.LastError = &v
	}
	if d.LastAttemptedAt.Valid {
		v := d.LastAttemptedAt.Time
		out.LastAttemptedAt = &v
	}
	if d.NextAttemptAt.Valid {
		v := d.NextAttemptAt.Time
		out.NextAttemptAt = &v
	}
	if d.SentAt.Valid {
		v := d.SentAt.Time
		out.SentAt = &v
	}
	return out
}

func (h *NotificationsHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	rows, err := h.q.ListRecentDeliveriesForChannel(r.Context(), store.ListRecentDeliveriesForChannelParams{
		ChannelID: id, Limit: 50,
	})
	if err != nil {
		h.logger.Error("list deliveries failed", "channel_id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	out := make([]deliveryResponse, 0, len(rows))
	for _, d := range rows {
		out = append(out, toDeliveryResponse(d))
	}
	writeJSON(w, http.StatusOK, map[string]any{"deliveries": out})
}

func (h *NotificationsHandler) GetDelivery(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	d, err := h.q.GetNotificationDelivery(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"delivery": toDeliveryResponse(d)})
}

func validateChannelMeta(name string, retryMax, backoff int64) map[string]string {
	f := map[string]string{}
	if name == "" || len(name) > 100 {
		f["name"] = "must be 1-100 characters"
	}
	if retryMax < 0 || retryMax > 10 {
		f["retry_max"] = "must be 0-10"
	}
	if backoff < 1 {
		f["retry_backoff_seconds"] = "must be >= 1"
	}
	return f
}

// encodeConfig validates the input config for the channel type, encrypts
// secret fields, preserves existing encrypted fields when the user submits a
// blank, and returns the JSON to store. Returns either a JSON string or a map
// of validation errors.
func (h *NotificationsHandler) encodeConfig(typ string, raw json.RawMessage, existing string) (string, map[string]string, error) {
	if len(raw) == 0 {
		return existing, nil, nil
	}
	in := map[string]any{}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", map[string]string{"config": "must be an object"}, nil
	}
	prev := map[string]any{}
	if existing != "" {
		_ = json.Unmarshal([]byte(existing), &prev)
	}
	switch typ {
	case notifier.ChannelEmail:
		return h.encodeEmailConfig(in, prev)
	case notifier.ChannelDiscord:
		return h.encodeDiscordConfig(in, prev)
	case notifier.ChannelWebhook:
		return h.encodeWebhookConfig(in, prev)
	}
	return "", map[string]string{"type": "unknown channel type"}, nil
}

func (h *NotificationsHandler) encodeEmailConfig(in, prev map[string]any) (string, map[string]string, error) {
	out := notifier.EmailConfig{
		SMTPHost:     getString(in, "smtp_host"),
		SMTPPort:     getInt(in, "smtp_port", 587),
		SMTPStartTLS: getBoolDefault(in, "smtp_starttls", true),
		SMTPUsername: getString(in, "smtp_username"),
		FromAddress:  getString(in, "from_address"),
		FromName:     getStringDefault(in, "from_name", "Cairn"),
		ToAddresses:  getStringSlice(in, "to_addresses"),
	}
	fields := map[string]string{}
	if out.SMTPHost == "" {
		fields["smtp_host"] = FieldRequired
	}
	if out.SMTPPort < 1 || out.SMTPPort > 65535 {
		fields["smtp_port"] = FieldOutOfRange
	}
	if out.FromAddress == "" {
		fields["from_address"] = FieldRequired
	} else if _, err := mail.ParseAddress(out.FromAddress); err != nil {
		fields["from_address"] = FieldInvalidFormat
	}
	if len(out.ToAddresses) == 0 {
		fields["to_addresses"] = FieldRequired
	} else {
		for _, a := range out.ToAddresses {
			if _, err := mail.ParseAddress(a); err != nil {
				_ = a
				fields["to_addresses"] = FieldInvalidFormat
				break
			}
		}
	}
	if pw := getString(in, "smtp_password"); pw != "" {
		enc, err := h.secretBox.EncryptString(pw)
		if err != nil {
			return "", nil, err
		}
		out.SMTPPasswordEnc = enc
	} else if v, ok := prev["smtp_password_enc"].(string); ok {
		out.SMTPPasswordEnc = v
	}
	if len(fields) > 0 {
		return "", fields, nil
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", nil, err
	}
	return string(b), nil, nil
}

func (h *NotificationsHandler) encodeDiscordConfig(in, prev map[string]any) (string, map[string]string, error) {
	out := notifier.DiscordConfig{
		Username:  getStringDefault(in, "username", "Cairn"),
		AvatarURL: getString(in, "avatar_url"),
	}
	fields := map[string]string{}
	if u := getString(in, "webhook_url"); u != "" {
		if _, err := url.ParseRequestURI(u); err != nil {
			fields["webhook_url"] = FieldInvalidFormat
		} else {
			enc, err := h.secretBox.EncryptString(u)
			if err != nil {
				return "", nil, err
			}
			out.WebhookURLEnc = enc
		}
	} else if v, ok := prev["webhook_url_enc"].(string); ok && v != "" {
		out.WebhookURLEnc = v
	} else {
		fields["webhook_url"] = FieldRequired
	}
	if len(fields) > 0 {
		return "", fields, nil
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", nil, err
	}
	return string(b), nil, nil
}

func (h *NotificationsHandler) encodeWebhookConfig(in, prev map[string]any) (string, map[string]string, error) {
	out := notifier.WebhookConfig{
		URL:    getString(in, "url"),
		Method: strings.ToUpper(getStringDefault(in, "method", "POST")),
	}
	fields := map[string]string{}
	if out.URL == "" {
		fields["url"] = FieldRequired
	} else if _, err := url.ParseRequestURI(out.URL); err != nil {
		fields["url"] = FieldInvalidFormat
	}
	if out.Method != "POST" && out.Method != "PUT" {
		fields["method"] = FieldInvalidValue
	}
	if hdrs, ok := in["extra_headers"].(map[string]any); ok {
		out.ExtraHeaders = map[string]string{}
		for k, v := range hdrs {
			if s, ok := v.(string); ok {
				out.ExtraHeaders[k] = s
			}
		}
	}
	if s := getString(in, "secret"); s != "" {
		enc, err := h.secretBox.EncryptString(s)
		if err != nil {
			return "", nil, err
		}
		out.SecretEnc = enc
	} else if v, ok := prev["secret_enc"].(string); ok {
		out.SecretEnc = v
	}
	if len(fields) > 0 {
		return "", fields, nil
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", nil, err
	}
	return string(b), nil, nil
}

func valueOr(p *int64, def int64) int64 {
	if p != nil {
		return *p
	}
	return def
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getStringDefault(m map[string]any, key, def string) string {
	if v := getString(m, key); v != "" {
		return v
	}
	return def
}

func getInt(m map[string]any, key string, def int) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return def
}

func getBoolDefault(m map[string]any, key string, def bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return def
}

func getStringSlice(m map[string]any, key string) []string {
	v, ok := m[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(v))
	for _, x := range v {
		if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func isUniqueErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE") || strings.Contains(msg, "unique constraint")
}
