package repository

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
)

// AuditLogger implements ports.AuditLogger over an AuditRepo. Failures are
// logged, never propagated - auditing must not break business flows.
type AuditLogger struct {
	repo ports.AuditRepo
	log  *slog.Logger
}

// NewAuditLogger builds an AuditLogger.
func NewAuditLogger(repo ports.AuditRepo, log *slog.Logger) *AuditLogger {
	return &AuditLogger{repo: repo, log: log}
}

// Log records a sensitive action. before/after are JSON-marshaled snapshots.
func (a *AuditLogger) Log(ctx context.Context, actorUserID int64, action, entity string, entityID int64, before, after any) {
	row := &domain.AuditLog{
		Action:   action,
		Entity:   entity,
		EntityID: entityID,
		Before:   marshalOrNull(before),
		After:    marshalOrNull(after),
	}
	if actorUserID != 0 {
		row.UserID = &actorUserID
	}
	if err := a.repo.Create(ctx, row); err != nil {
		a.log.ErrorContext(ctx, "audit log write failed",
			"action", action, "entity", entity, "entity_id", entityID, "error", err)
	}
}

// IntegrationLogger implements ports.IntegrationLogger over an
// IntegrationLogRepo, redacting secret keys before persisting.
type IntegrationLogger struct {
	repo ports.IntegrationLogRepo
	log  *slog.Logger
}

// NewIntegrationLogger builds an IntegrationLogger.
func NewIntegrationLogger(repo ports.IntegrationLogRepo, log *slog.Logger) *IntegrationLogger {
	return &IntegrationLogger{repo: repo, log: log}
}

// Log records one external API call with redacted request/response payloads.
func (l *IntegrationLogger) Log(ctx context.Context, call ports.IntegrationCall) {
	row := &domain.IntegrationLog{
		Provider:   call.Provider,
		Endpoint:   call.Endpoint,
		Method:     call.Method,
		StatusCode: call.StatusCode,
		Success:    call.Success,
		LatencyMS:  call.LatencyMS,
		Request:    Redact(marshalOrNull(call.Request)),
		Response:   Redact(marshalOrNull(call.Response)),
		Error:      call.Error,
	}
	if err := l.repo.Create(ctx, row); err != nil {
		l.log.ErrorContext(ctx, "integration log write failed",
			"provider", call.Provider, "endpoint", call.Endpoint, "error", err)
	}
}

func marshalOrNull(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	if raw, ok := v.(json.RawMessage); ok {
		return raw
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{"_marshal_error":true}`)
	}
	return raw
}

// redactedKeys are matched case-insensitively at any nesting depth
// (CONTRACTS.md §3).
var redactedKeys = map[string]bool{
	"apikey":        true,
	"api_key":       true,
	"password":      true,
	"passwd":        true,
	"token":         true,
	"signature":     true,
	"authorization": true,
}

// Redact replaces the values of secret keys in a JSON document with
// "[REDACTED]" at every nesting level. Non-object/array JSON passes through
// unchanged; invalid JSON becomes null.
func Redact(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return json.RawMessage("null")
	}
	out, err := json.Marshal(redactValue(v))
	if err != nil {
		return json.RawMessage("null")
	}
	return out
}

func redactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if redactedKeys[strings.ToLower(k)] {
				t[k] = "[REDACTED]"
				continue
			}
			t[k] = redactValue(val)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = redactValue(val)
		}
		return t
	default:
		return v
	}
}
