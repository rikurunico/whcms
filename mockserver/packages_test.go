package main

import (
	"net/http"
	"net/url"
	"testing"
)

// --- WHM addpkg / editpkg / killpkg -----------------------------------------

func TestWHMAddEditKillPkg(t *testing.T) {
	srv, ts := newTestServer(t)

	// addpkg creates the package and records its limits.
	_, m := whmCall(t, ts, http.MethodPost, "addpkg", url.Values{
		"name":        {"whcms_s1"},
		"featurelist": {"default"},
		"quota":       {"10240"},
		"bwlimit":     {"unlimited"},
		"maxaddon":    {"5"},
	}, whmGoodAuth)
	if result, reason := whmMeta(t, m); result != 1 {
		t.Fatalf("addpkg failed: result=%v reason=%q", result, reason)
	}
	srv.mu.Lock()
	pkg, ok := srv.whmPackages["whcms_s1"]
	srv.mu.Unlock()
	if !ok {
		t.Fatal("package not stored")
	}
	if pkg.Params["quota"] != "10240" || pkg.Params["bwlimit"] != "unlimited" || pkg.Params["maxaddon"] != "5" {
		t.Fatalf("params not recorded: %v", pkg.Params)
	}

	// addpkg again -> conflict.
	_, m = whmCall(t, ts, http.MethodPost, "addpkg", url.Values{"name": {"whcms_s1"}}, whmGoodAuth)
	if result, reason := whmMeta(t, m); result != 0 || reason == "" {
		t.Fatalf("expected addpkg conflict, got result=%v reason=%q", result, reason)
	}

	// editpkg updates limits.
	_, m = whmCall(t, ts, http.MethodPost, "editpkg", url.Values{"name": {"whcms_s1"}, "quota": {"20480"}}, whmGoodAuth)
	if result, _ := whmMeta(t, m); result != 1 {
		t.Fatalf("editpkg failed: %v", m)
	}
	srv.mu.Lock()
	updated := srv.whmPackages["whcms_s1"].Params["quota"]
	srv.mu.Unlock()
	if updated != "20480" {
		t.Fatalf("editpkg did not update quota, got %q", updated)
	}

	// editpkg on a missing package -> error.
	_, m = whmCall(t, ts, http.MethodPost, "editpkg", url.Values{"name": {"ghost"}}, whmGoodAuth)
	if result, _ := whmMeta(t, m); result != 0 {
		t.Fatalf("expected editpkg on missing to fail, got %v", m)
	}

	// killpkg removes it; a second kill fails.
	_, m = whmCall(t, ts, http.MethodPost, "killpkg", url.Values{"pkg": {"whcms_s1"}}, whmGoodAuth)
	if result, _ := whmMeta(t, m); result != 1 {
		t.Fatalf("killpkg failed: %v", m)
	}
	_, m = whmCall(t, ts, http.MethodPost, "killpkg", url.Values{"pkg": {"whcms_s1"}}, whmGoodAuth)
	if result, _ := whmMeta(t, m); result != 0 {
		t.Fatalf("expected killpkg on missing to fail, got %v", m)
	}
}

// --- WHM listpkgs ------------------------------------------------------------

func TestWHMListPkgs(t *testing.T) {
	srv, ts := newTestServer(t)

	// Seeded defaults are present out of the box.
	_, m := whmCall(t, ts, http.MethodGet, "listpkgs", url.Values{}, whmGoodAuth)
	if result, _ := whmMeta(t, m); result != 1 {
		t.Fatalf("listpkgs failed: %v", m)
	}
	names := whmPkgNames(t, m)
	for _, want := range []string{"default", "starter", "business"} {
		if !contains(names, want) {
			t.Fatalf("expected seeded package %q in %v", want, names)
		}
	}

	// Newly created packages show up too.
	_, m = whmCall(t, ts, http.MethodPost, "addpkg", url.Values{"name": {"whcms_s1"}}, whmGoodAuth)
	if result, _ := whmMeta(t, m); result != 1 {
		t.Fatalf("addpkg failed: %v", m)
	}
	_, m = whmCall(t, ts, http.MethodGet, "listpkgs", url.Values{}, whmGoodAuth)
	if !contains(whmPkgNames(t, m), "whcms_s1") {
		t.Fatalf("expected newly created package in listpkgs, got %v", m)
	}

	srv.resetState()
	_, m = whmCall(t, ts, http.MethodGet, "listpkgs", url.Values{}, whmGoodAuth)
	if contains(whmPkgNames(t, m), "whcms_s1") {
		t.Fatal("expected reset to clear on-the-fly packages")
	}
}

func whmPkgNames(t *testing.T, m map[string]any) []string {
	t.Helper()
	data, _ := m["data"].(map[string]any)
	raw, _ := data["pkg"].([]any)
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		if obj, ok := r.(map[string]any); ok {
			if name, ok := obj["name"].(string); ok {
				out = append(out, name)
			}
		}
	}
	return out
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// --- DirectAdmin MANAGE_USER_PACKAGES ---------------------------------------

func TestDAManageUserPackages(t *testing.T) {
	srv, ts := newTestServer(t)
	const path = "/CMD_API_MANAGE_USER_PACKAGES"

	// create
	_, v := daCall(t, ts, http.MethodPost, path, url.Values{
		"action":      {"create"},
		"add":         {"Submit"},
		"packagename": {"whcms_s1"},
		"quota":       {"10240"},
		"ubandwidth":  {"ON"},
		"bandwidth":   {"0"},
		"vdomains":    {"5"},
	}, "admin", "dapass")
	if v.Get("error") != "0" {
		t.Fatalf("create failed: %v", v)
	}
	srv.mu.Lock()
	pkg, ok := srv.daPackages["whcms_s1"]
	srv.mu.Unlock()
	if !ok || pkg.Params["quota"] != "10240" || pkg.Params["ubandwidth"] != "ON" {
		t.Fatalf("package not stored correctly: ok=%v params=%v", ok, pkg)
	}

	// duplicate create -> conflict (error=1, message contains "already exists")
	_, v = daCall(t, ts, http.MethodPost, path, url.Values{
		"action": {"create"}, "packagename": {"whcms_s1"},
	}, "admin", "dapass")
	if v.Get("error") != "1" {
		t.Fatalf("expected duplicate create to fail: %v", v)
	}

	// modify
	_, v = daCall(t, ts, http.MethodPost, path, url.Values{
		"action": {"modify"}, "packagename": {"whcms_s1"}, "quota": {"20480"},
	}, "admin", "dapass")
	if v.Get("error") != "0" {
		t.Fatalf("modify failed: %v", v)
	}
	srv.mu.Lock()
	updated := srv.daPackages["whcms_s1"].Params["quota"]
	srv.mu.Unlock()
	if updated != "20480" {
		t.Fatalf("modify did not update quota, got %q", updated)
	}

	// delete, then delete again fails
	_, v = daCall(t, ts, http.MethodPost, path, url.Values{
		"action": {"delete"}, "delete": {"Submit"}, "select0": {"whcms_s1"},
	}, "admin", "dapass")
	if v.Get("error") != "0" {
		t.Fatalf("delete failed: %v", v)
	}
	_, v = daCall(t, ts, http.MethodPost, path, url.Values{
		"action": {"delete"}, "delete": {"Submit"}, "select0": {"whcms_s1"},
	}, "admin", "dapass")
	if v.Get("error") != "1" {
		t.Fatalf("expected delete of missing to fail: %v", v)
	}
}

func TestDAPackagesUser(t *testing.T) {
	_, ts := newTestServer(t)
	const listPath = "/CMD_API_PACKAGES_USER"

	// Seeded defaults are present out of the box, under the real "list[]" key.
	// Real DirectAdmin's success response for this dump-style endpoint carries
	// no `error` key at all - only a failure sets error=1.
	_, v := daCall(t, ts, http.MethodGet, listPath, url.Values{}, "admin", "dapass")
	if v.Get("error") != "" {
		t.Fatalf("expected no error key on success, got: %v", v)
	}
	for _, want := range []string{"default", "starter", "business"} {
		if !contains(v["list[]"], want) {
			t.Fatalf("expected seeded package %q in %v", want, v["list[]"])
		}
	}

	_, v = daCall(t, ts, http.MethodPost, "/CMD_API_MANAGE_USER_PACKAGES", url.Values{
		"action": {"create"}, "packagename": {"whcms_s1"},
	}, "admin", "dapass")
	if v.Get("error") != "0" {
		t.Fatalf("create failed: %v", v)
	}
	_, v = daCall(t, ts, http.MethodGet, listPath, url.Values{}, "admin", "dapass")
	if !contains(v["list[]"], "whcms_s1") {
		t.Fatalf("expected newly created package in list, got %v", v["list[]"])
	}
}
