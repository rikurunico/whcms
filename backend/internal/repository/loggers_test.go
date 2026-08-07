package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestAuditLoggerWritesRow(t *testing.T) {
	var got *domain.AuditLog
	repo := &mocks.MockAuditRepo{
		CreateFn: func(ctx context.Context, a *domain.AuditLog) error {
			got = a
			return nil
		},
	}
	al := repository.NewAuditLogger(repo, discard)
	al.Log(context.Background(), 42, "settings.update", "settings", 0,
		map[string]any{"old": 1}, map[string]any{"new": 2})

	require.NotNil(t, got)
	require.NotNil(t, got.UserID)
	assert.Equal(t, int64(42), *got.UserID)
	assert.Equal(t, "settings.update", got.Action)
	assert.JSONEq(t, `{"old":1}`, string(got.Before))
	assert.JSONEq(t, `{"new":2}`, string(got.After))
}

func TestAuditLoggerSystemActorHasNullUser(t *testing.T) {
	var got *domain.AuditLog
	repo := &mocks.MockAuditRepo{
		CreateFn: func(ctx context.Context, a *domain.AuditLog) error { got = a; return nil },
	}
	repository.NewAuditLogger(repo, discard).
		Log(context.Background(), 0, "cron.suspend", "service", 9, nil, nil)
	require.NotNil(t, got)
	assert.Nil(t, got.UserID, "actor 0 -> NULL user_id")
	assert.Nil(t, got.Before)
}

func TestAuditLoggerSwallowsRepoError(t *testing.T) {
	repo := &mocks.MockAuditRepo{
		CreateFn: func(ctx context.Context, a *domain.AuditLog) error {
			return errors.New("db down")
		},
	}
	assert.NotPanics(t, func() {
		repository.NewAuditLogger(repo, discard).
			Log(context.Background(), 1, "x", "y", 1, nil, nil)
	})
}

func TestAuditLoggerMarshalOrNullPassthroughAndError(t *testing.T) {
	var got *domain.AuditLog
	repo := &mocks.MockAuditRepo{
		CreateFn: func(ctx context.Context, a *domain.AuditLog) error {
			got = a
			return nil
		},
	}
	al := repository.NewAuditLogger(repo, discard)

	// before is already a json.RawMessage: marshalOrNull must pass it through
	// unchanged rather than re-marshaling it.
	raw := json.RawMessage(`{"already":"json"}`)
	al.Log(context.Background(), 1, "x", "y", 1, raw, nil)
	require.NotNil(t, got)
	assert.JSONEq(t, `{"already":"json"}`, string(got.Before))

	// after is unmarshalable (a channel): marshalOrNull must fall back to the
	// sentinel error payload instead of propagating the marshal error.
	al.Log(context.Background(), 1, "x", "y", 1, nil, map[string]any{"bad": make(chan int)})
	require.NotNil(t, got)
	assert.JSONEq(t, `{"_marshal_error":true}`, string(got.After))
}

func TestIntegrationLoggerSwallowsRepoError(t *testing.T) {
	repo := &mocks.MockIntegrationLogRepo{
		CreateFn: func(ctx context.Context, l *domain.IntegrationLog) error {
			return errors.New("db down")
		},
	}
	il := repository.NewIntegrationLogger(repo, discard)
	assert.NotPanics(t, func() {
		il.Log(context.Background(), ports.IntegrationCall{Provider: "duitku"})
	})
}

func TestIntegrationLoggerRedactsSecrets(t *testing.T) {
	var got *domain.IntegrationLog
	repo := &mocks.MockIntegrationLogRepo{
		CreateFn: func(ctx context.Context, l *domain.IntegrationLog) error {
			got = l
			return nil
		},
	}
	il := repository.NewIntegrationLogger(repo, discard)
	il.Log(context.Background(), ports.IntegrationCall{
		Provider: "duitku", Endpoint: "/inquiry", Method: "POST",
		StatusCode: 200, Success: true, LatencyMS: 120,
		Request: map[string]any{
			"merchantCode": "DEMO",
			"apiKey":       "super-secret",
			"nested":       map[string]any{"Password": "hunter2", "ok": "visible"},
			"list":         []any{map[string]any{"token": "abc"}},
		},
		Response: map[string]any{"signature": "deadbeef", "reference": "REF-1"},
	})

	require.NotNil(t, got)
	assert.Equal(t, "duitku", got.Provider)

	var req map[string]any
	require.NoError(t, json.Unmarshal(got.Request, &req))
	assert.Equal(t, "DEMO", req["merchantCode"])
	assert.Equal(t, "[REDACTED]", req["apiKey"])
	nested := req["nested"].(map[string]any)
	assert.Equal(t, "[REDACTED]", nested["Password"], "case-insensitive")
	assert.Equal(t, "visible", nested["ok"])
	inList := req["list"].([]any)[0].(map[string]any)
	assert.Equal(t, "[REDACTED]", inList["token"])

	var resp map[string]any
	require.NoError(t, json.Unmarshal(got.Response, &resp))
	assert.Equal(t, "[REDACTED]", resp["signature"])
	assert.Equal(t, "REF-1", resp["reference"])
}

func TestRedact(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"flat", `{"api_key":"k","x":1}`, `{"api_key":"[REDACTED]","x":1}`},
		{"authorization", `{"Authorization":"Bearer x"}`, `{"Authorization":"[REDACTED]"}`},
		{"passwd", `{"passwd":"x"}`, `{"passwd":"[REDACTED]"}`},
		{"scalar passthrough", `"hello"`, `"hello"`},
		{"number passthrough", `42`, `42`},
		{"array of objects", `[{"token":"t"},{"a":1}]`, `[{"token":"[REDACTED]"},{"a":1}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repository.Redact(json.RawMessage(tt.in))
			assert.JSONEq(t, tt.want, string(got))
		})
	}
	assert.Nil(t, repository.Redact(nil))
	assert.Equal(t, "null", string(repository.Redact(json.RawMessage("{invalid"))))
}
