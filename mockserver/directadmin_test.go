package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// daCall issues a DirectAdmin legacy API request with Basic auth and parses
// the URL-encoded response body.
func daCall(t *testing.T, ts *httptest.Server, method, path string, params url.Values, user, pass string) (*http.Response, url.Values) {
	t.Helper()
	var req *http.Request
	var err error
	if method == http.MethodGet {
		req, err = http.NewRequest(http.MethodGet, ts.URL+path+"?"+params.Encode(), nil)
	} else {
		req, err = http.NewRequest(method, ts.URL+path, strings.NewReader(params.Encode()))
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body := bodyString(t, resp)
	return resp, formValues(t, body)
}

func daCreate(t *testing.T, ts *httptest.Server, user string) {
	t.Helper()
	_, v := daCall(t, ts, http.MethodPost, "/CMD_API_ACCOUNT_USER", url.Values{
		"action":   {"create"},
		"username": {user},
		"email":    {user + "@example.com"},
		"passwd":   {"S3cret!!"},
		"domain":   {user + ".id"},
		"package":  {"starter"},
		"ip":       {"127.0.0.1"},
	}, "admin", "dapass")
	if v.Get("error") != "0" {
		t.Fatalf("create %s failed: %v", user, v)
	}
}

func TestDAAuth(t *testing.T) {
	_, ts := newTestServer(t)

	t.Run("missing auth 401", func(t *testing.T) {
		resp, v := daCall(t, ts, http.MethodGet, "/CMD_API_SHOW_USER_CONFIG", url.Values{"user": {"x"}}, "", "")
		wantStatus(t, resp, http.StatusUnauthorized)
		if v.Get("error") != "1" {
			t.Fatalf("expected error=1, got %v", v)
		}
	})

	t.Run("user bad rejected", func(t *testing.T) {
		resp, _ := daCall(t, ts, http.MethodGet, "/CMD_API_SHOW_USER_CONFIG", url.Values{"user": {"x"}}, "bad", "anything")
		wantStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("any other credentials accepted", func(t *testing.T) {
		resp, _ := daCall(t, ts, http.MethodGet, "/CMD_API_SHOW_USER_CONFIG", url.Values{"user": {"ghost"}}, "admin", "anything")
		wantStatus(t, resp, http.StatusOK) // API-level error, but auth passed
	})
}

func TestDAShowUsers(t *testing.T) {
	_, ts := newTestServer(t)
	daCreate(t, ts, "auser")
	daCreate(t, ts, "buser")

	resp, v := daCall(t, ts, http.MethodGet, "/CMD_API_SHOW_USERS", url.Values{}, "admin", "dapass")
	wantStatus(t, resp, http.StatusOK)
	if v.Get("error") == "1" {
		t.Fatalf("show_users returned error: %v", v)
	}
	users := v["list[]"]
	if len(users) != 2 {
		t.Fatalf("list[] = %v, want 2 users", users)
	}

	// Auth is what the probe validates - bad creds are rejected.
	resp, _ = daCall(t, ts, http.MethodGet, "/CMD_API_SHOW_USERS", url.Values{}, "bad", "x")
	wantStatus(t, resp, http.StatusUnauthorized)
}

func TestDAAccountUserCreate(t *testing.T) {
	_, ts := newTestServer(t)

	t.Run("success", func(t *testing.T) {
		_, v := daCall(t, ts, http.MethodPost, "/CMD_API_ACCOUNT_USER", url.Values{
			"action": {"create"}, "username": {"dauser"}, "email": {"d@e.id"},
			"passwd": {"pw"}, "domain": {"dauser.id"}, "package": {"basic"},
		}, "admin", "pass")
		if v.Get("error") != "0" || v.Get("text") != "Success" {
			t.Fatalf("unexpected response: %v", v)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		_, v := daCall(t, ts, http.MethodPost, "/CMD_API_ACCOUNT_USER", url.Values{
			"action": {"create"}, "username": {"dauser"}, "domain": {"other.id"},
		}, "admin", "pass")
		if v.Get("error") != "1" {
			t.Fatalf("duplicate should fail: %v", v)
		}
	})

	t.Run("failme forced failure", func(t *testing.T) {
		_, v := daCall(t, ts, http.MethodPost, "/CMD_API_ACCOUNT_USER", url.Values{
			"action": {"create"}, "username": {"failme"}, "domain": {"failme.id"},
		}, "admin", "pass")
		if v.Get("error") != "1" || v.Get("details") != "forced failure" {
			t.Fatalf("failme should force failure: %v", v)
		}
	})

	t.Run("missing username", func(t *testing.T) {
		_, v := daCall(t, ts, http.MethodPost, "/CMD_API_ACCOUNT_USER", url.Values{"action": {"create"}}, "admin", "pass")
		if v.Get("error") != "1" {
			t.Fatalf("missing username should fail: %v", v)
		}
	})

	t.Run("unknown action", func(t *testing.T) {
		_, v := daCall(t, ts, http.MethodPost, "/CMD_API_ACCOUNT_USER", url.Values{"action": {"destroy"}, "username": {"x"}}, "admin", "pass")
		if v.Get("error") != "1" {
			t.Fatalf("unknown action should fail: %v", v)
		}
	})

	t.Run("GET with query params also works", func(t *testing.T) {
		_, v := daCall(t, ts, http.MethodGet, "/CMD_API_ACCOUNT_USER", url.Values{
			"action": {"create"}, "username": {"getda"}, "domain": {"getda.id"},
		}, "admin", "pass")
		if v.Get("error") != "0" {
			t.Fatalf("GET create should work: %v", v)
		}
	})
}

func TestDASuspendDeleteFlow(t *testing.T) {
	_, ts := newTestServer(t)
	daCreate(t, ts, "flowuser")

	showConfig := func(user string) url.Values {
		t.Helper()
		_, v := daCall(t, ts, http.MethodGet, "/CMD_API_SHOW_USER_CONFIG", url.Values{"user": {user}}, "admin", "pass")
		return v
	}

	cfg := showConfig("flowuser")
	if cfg.Get("suspended") != "no" || cfg.Get("package") != "starter" || cfg.Get("domain") != "flowuser.id" {
		t.Fatalf("unexpected initial config: %v", cfg)
	}

	// suspend
	_, v := daCall(t, ts, http.MethodPost, "/CMD_API_SELECT_USERS", url.Values{
		"select0": {"flowuser"}, "suspend": {"Suspend"},
	}, "admin", "pass")
	if v.Get("error") != "0" {
		t.Fatalf("suspend failed: %v", v)
	}
	if cfg = showConfig("flowuser"); cfg.Get("suspended") != "yes" {
		t.Fatalf("suspended should be yes: %v", cfg)
	}

	// unsuspend
	_, v = daCall(t, ts, http.MethodPost, "/CMD_API_SELECT_USERS", url.Values{
		"select0": {"flowuser"}, "suspend": {"Unsuspend"},
	}, "admin", "pass")
	if v.Get("error") != "0" {
		t.Fatalf("unsuspend failed: %v", v)
	}
	if cfg = showConfig("flowuser"); cfg.Get("suspended") != "no" {
		t.Fatalf("suspended should be no: %v", cfg)
	}

	// delete
	_, v = daCall(t, ts, http.MethodPost, "/CMD_API_SELECT_USERS", url.Values{
		"select0": {"flowuser"}, "delete": {"yes"},
	}, "admin", "pass")
	if v.Get("error") != "0" {
		t.Fatalf("delete failed: %v", v)
	}
	if cfg = showConfig("flowuser"); cfg.Get("error") != "1" {
		t.Fatalf("deleted user should not resolve: %v", cfg)
	}
}

func TestDASelectUsersErrors(t *testing.T) {
	_, ts := newTestServer(t)
	daCreate(t, ts, "erruser")

	tests := []struct {
		name   string
		params url.Values
	}{
		{"no users selected", url.Values{"suspend": {"Suspend"}}},
		{"unknown user", url.Values{"select0": {"ghost"}, "suspend": {"Suspend"}}},
		{"nothing to do", url.Values{"select0": {"erruser"}}},
		{"invalid suspend value", url.Values{"select0": {"erruser"}, "suspend": {"Maybe"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := daCall(t, ts, http.MethodPost, "/CMD_API_SELECT_USERS", tt.params, "admin", "pass")
			if v.Get("error") != "1" {
				t.Fatalf("expected error=1: %v", v)
			}
		})
	}

	t.Run("multiple selects handled", func(t *testing.T) {
		daCreate(t, ts, "multi1")
		daCreate(t, ts, "multi2")
		_, v := daCall(t, ts, http.MethodPost, "/CMD_API_SELECT_USERS", url.Values{
			"select0": {"multi1"}, "select1": {"multi2"}, "suspend": {"Suspend"},
		}, "admin", "pass")
		if v.Get("error") != "0" || !strings.Contains(v.Get("details"), "2 user(s)") {
			t.Fatalf("multi-suspend failed: %v", v)
		}
	})
}

func TestDAModifyUser(t *testing.T) {
	_, ts := newTestServer(t)
	daCreate(t, ts, "pkguser")

	t.Run("change package", func(t *testing.T) {
		_, v := daCall(t, ts, http.MethodPost, "/CMD_API_MODIFY_USER", url.Values{
			"action": {"package"}, "user": {"pkguser"}, "package": {"business"},
		}, "admin", "pass")
		if v.Get("error") != "0" {
			t.Fatalf("modify failed: %v", v)
		}
		_, cfg := daCall(t, ts, http.MethodGet, "/CMD_API_SHOW_USER_CONFIG", url.Values{"user": {"pkguser"}}, "admin", "pass")
		if cfg.Get("package") != "business" {
			t.Fatalf("package not changed: %v", cfg)
		}
	})

	t.Run("unknown user", func(t *testing.T) {
		_, v := daCall(t, ts, http.MethodPost, "/CMD_API_MODIFY_USER", url.Values{
			"action": {"package"}, "user": {"ghost"}, "package": {"x"},
		}, "admin", "pass")
		if v.Get("error") != "1" {
			t.Fatalf("expected error=1: %v", v)
		}
	})

	t.Run("wrong action", func(t *testing.T) {
		_, v := daCall(t, ts, http.MethodPost, "/CMD_API_MODIFY_USER", url.Values{
			"action": {"quota"}, "user": {"pkguser"},
		}, "admin", "pass")
		if v.Get("error") != "1" {
			t.Fatalf("expected error=1: %v", v)
		}
	})
}

func TestDAUserPasswd(t *testing.T) {
	_, ts := newTestServer(t)
	daCreate(t, ts, "pwuser")

	tests := []struct {
		name    string
		params  url.Values
		wantErr string
	}{
		{"success", url.Values{"username": {"pwuser"}, "passwd": {"NewPw!1"}, "passwd2": {"NewPw!1"}}, "0"},
		{"success without passwd2", url.Values{"username": {"pwuser"}, "passwd": {"NewPw!2"}}, "0"},
		{"mismatch", url.Values{"username": {"pwuser"}, "passwd": {"a"}, "passwd2": {"b"}}, "1"},
		{"unknown user", url.Values{"username": {"ghost"}, "passwd": {"x"}}, "1"},
		{"missing passwd", url.Values{"username": {"pwuser"}}, "1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, v := daCall(t, ts, http.MethodPost, "/CMD_API_USER_PASSWD", tt.params, "admin", "pass")
			if v.Get("error") != tt.wantErr {
				t.Fatalf("error = %q, want %q (%v)", v.Get("error"), tt.wantErr, v)
			}
		})
	}
}
