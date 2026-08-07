package main

import (
	"net/http"
)

// Cloudflare Turnstile testing secret keys (docs). The mock mirrors their
// documented behaviour so the backend's siteverify path can be exercised
// hermetically (no outbound internet) in dev/E2E.
const (
	turnstileSecretAlwaysPass = "1x0000000000000000000000000000000AA"
	turnstileSecretAlwaysFail = "2x0000000000000000000000000000000AA"
	turnstileSecretTokenSpent = "3x0000000000000000000000000000000AA"
)

// handleTurnstileVerify mocks POST /turnstile/v0/siteverify. It reads the
// url-encoded form (secret, response[, remoteip]) and returns Cloudflare's
// JSON shape. An empty response -> missing-input-response; the always-fail /
// token-spent test secrets -> success:false; every other secret (including the
// always-pass test secret) -> success:true when a response is present.
func (s *Server) handleTurnstileVerify(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, turnstileResult(false, "bad-request"))
		return
	}
	secret := r.PostForm.Get("secret")
	response := r.PostForm.Get("response")

	switch {
	case response == "":
		writeJSON(w, http.StatusOK, turnstileResult(false, "missing-input-response"))
	case secret == "":
		writeJSON(w, http.StatusOK, turnstileResult(false, "missing-input-secret"))
	case secret == turnstileSecretAlwaysFail:
		writeJSON(w, http.StatusOK, turnstileResult(false, "invalid-input-response"))
	case secret == turnstileSecretTokenSpent:
		writeJSON(w, http.StatusOK, turnstileResult(false, "timeout-or-duplicate"))
	default:
		// always-pass test secret and any other configured secret succeed.
		writeJSON(w, http.StatusOK, turnstileResult(true, ""))
	}
}

func turnstileResult(success bool, errCode string) map[string]any {
	out := map[string]any{
		"success":      success,
		"challenge_ts": "2026-07-18T00:00:00.000Z",
		"hostname":     "localhost",
		"action":       "",
		"cdata":        "",
	}
	if errCode == "" {
		out["error-codes"] = []string{}
	} else {
		out["error-codes"] = []string{errCode}
	}
	return out
}
