package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func sendMail(t *testing.T, ts *httptest.Server, to, subject string) {
	t.Helper()
	resp, m := postJSON(t, ts.URL+"/mail/send", map[string]any{
		"to":      to,
		"from":    "noreply@whcms.test",
		"subject": subject,
		"html":    "<p>" + subject + "</p>",
		"text":    subject,
	})
	wantStatus(t, resp, http.StatusOK)
	wantField(t, m, "status", "stored")
}

func listMail(t *testing.T, ts *httptest.Server, query string) []mailMessage {
	t.Helper()
	resp, err := http.Get(ts.URL + "/mail/messages" + query)
	if err != nil {
		t.Fatal(err)
	}
	wantStatus(t, resp, http.StatusOK)
	var msgs []mailMessage
	if err := json.Unmarshal([]byte(bodyString(t, resp)), &msgs); err != nil {
		t.Fatalf("decode messages: %v", err)
	}
	return msgs
}

func TestMailCapture(t *testing.T) {
	_, ts := newTestServer(t)

	sendMail(t, ts, "alice@example.com", "Welcome Alice")
	sendMail(t, ts, "bob@example.com", "Invoice INV-1")
	sendMail(t, ts, "alice@example.com", "Password Reset")

	t.Run("list all newest first", func(t *testing.T) {
		msgs := listMail(t, ts, "")
		if len(msgs) != 3 {
			t.Fatalf("len = %d, want 3", len(msgs))
		}
		if msgs[0].Subject != "Password Reset" || msgs[2].Subject != "Welcome Alice" {
			t.Fatalf("wrong order: %v", msgs)
		}
		if msgs[0].ReceivedAt.IsZero() {
			t.Fatal("receivedAt not set")
		}
		if msgs[0].HTML == "" || msgs[0].Text == "" || msgs[0].From == "" {
			t.Fatalf("message fields not stored: %+v", msgs[0])
		}
	})

	t.Run("filter by recipient", func(t *testing.T) {
		msgs := listMail(t, ts, "?to=alice@example.com")
		if len(msgs) != 2 {
			t.Fatalf("len = %d, want 2", len(msgs))
		}
		for _, m := range msgs {
			if m.To != "alice@example.com" {
				t.Fatalf("wrong recipient: %v", m)
			}
		}
	})

	t.Run("filter is case-insensitive", func(t *testing.T) {
		msgs := listMail(t, ts, "?to=ALICE@example.com")
		if len(msgs) != 2 {
			t.Fatalf("len = %d, want 2", len(msgs))
		}
	})

	t.Run("filter no match returns empty array", func(t *testing.T) {
		msgs := listMail(t, ts, "?to=nobody@example.com")
		if len(msgs) != 0 {
			t.Fatalf("len = %d, want 0", len(msgs))
		}
	})

	t.Run("missing to rejected", func(t *testing.T) {
		resp, _ := postJSON(t, ts.URL+"/mail/send", map[string]any{"subject": "no recipient"})
		wantStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("delete clears store", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/mail/messages", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		m := decodeBody(t, resp)
		wantStatus(t, resp, http.StatusOK)
		wantField(t, m, "deleted", 3)

		if msgs := listMail(t, ts, ""); len(msgs) != 0 {
			t.Fatalf("store not cleared: %v", msgs)
		}
	})
}
