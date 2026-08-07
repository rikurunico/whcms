package main

import (
	"net/http"
	"strings"
	"time"
)

// Mail capture: the backend's `http` mail driver POSTs JSON messages here so
// that E2E tests can assert on outbound email without a real SMTP server.

type mailMessage struct {
	ID         int64     `json:"id"`
	To         string    `json:"to"`
	From       string    `json:"from"`
	Subject    string    `json:"subject"`
	HTML       string    `json:"html"`
	Text       string    `json:"text"`
	ReceivedAt time.Time `json:"receivedAt"`
}

// POST /mail/send
func (s *Server) handleMailSend(w http.ResponseWriter, r *http.Request) {
	var msg mailMessage
	if err := decodeJSON(r, &msg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(msg.To) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required field: to"})
		return
	}

	s.mu.Lock()
	s.mailSeq++
	msg.ID = s.mailSeq
	msg.ReceivedAt = time.Now().UTC()
	s.mails = append(s.mails, msg)
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"id": msg.ID, "status": "stored"})
}

// GET /mail/messages?to=<email> - newest first, optionally filtered by
// recipient (case-insensitive exact match).
func (s *Server) handleMailList(w http.ResponseWriter, r *http.Request) {
	to := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("to")))

	s.mu.Lock()
	out := make([]mailMessage, 0, len(s.mails))
	for i := len(s.mails) - 1; i >= 0; i-- {
		m := s.mails[i]
		if to != "" && strings.ToLower(m.To) != to {
			continue
		}
		out = append(out, m)
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, out)
}

// DELETE /mail/messages
func (s *Server) handleMailClear(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	deleted := len(s.mails)
	s.mails = nil
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
}
