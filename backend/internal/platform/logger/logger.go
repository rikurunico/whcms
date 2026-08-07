// Package logger provides the application slog JSON logger with request_id
// support (CONTRACTS.md §2).
package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey struct{}

// New builds a JSON slog.Logger writing to stdout. Level is debug in
// development, info otherwise.
func New(appEnv string) *slog.Logger {
	level := slog.LevelInfo
	if appEnv == "development" {
		level = slog.LevelDebug
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(&requestIDHandler{Handler: h})
}

// WithRequestID stores a request id in ctx; the handler attaches it to every
// record logged with that context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, requestID)
}

// RequestID extracts the request id from ctx ("" if unset).
func RequestID(ctx context.Context) string {
	s, _ := ctx.Value(ctxKey{}).(string)
	return s
}

// requestIDHandler decorates records with request_id from the context.
type requestIDHandler struct {
	slog.Handler
}

func (h *requestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &requestIDHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *requestIDHandler) WithGroup(name string) slog.Handler {
	return &requestIDHandler{Handler: h.Handler.WithGroup(name)}
}
