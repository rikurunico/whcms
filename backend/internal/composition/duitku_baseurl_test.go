package composition

import "testing"

// TestResolveDuitkuBaseURL covers the live base URL precedence in isolation
// (pure function, no DB/network - see resolveDuitkuCredentials in build.go
// for how it's wired). This exists specifically because getting this
// precedence wrong once broke the local E2E stack: with an explicit
// DUITKU_BASE_URL env pin (dev/E2E always points it at the mockserver) and no
// DB override, a naive "derive from mode" implementation ignored the pin and
// sent every request to real sandbox.duitku.com instead.
func TestResolveDuitkuBaseURL(t *testing.T) {
	cases := []struct {
		name        string
		settingURL  string
		envExplicit string
		mode        string
		want        string
	}{
		{
			name:       "DB setting override wins over everything",
			settingURL: "https://staging.duitku.example",
			mode:       "production",
			want:       "https://staging.duitku.example",
		},
		{
			name:        "explicit env pin wins when no DB override, regardless of mode",
			envExplicit: "http://localhost:9090",
			mode:        "sandbox",
			want:        "http://localhost:9090",
		},
		{
			name:        "explicit env pin wins even when mode says production",
			envExplicit: "http://localhost:9090",
			mode:        "production",
			want:        "http://localhost:9090",
		},
		{
			name: "neither override: sandbox mode derives sandbox.duitku.com",
			mode: "sandbox",
			want: "https://sandbox.duitku.com",
		},
		{
			name: "neither override: production mode derives passport.duitku.com",
			mode: "production",
			want: "https://passport.duitku.com",
		},
		{
			name:        "DB setting wins over the explicit env pin too",
			settingURL:  "https://staging.duitku.example",
			envExplicit: "http://localhost:9090",
			mode:        "sandbox",
			want:        "https://staging.duitku.example",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveDuitkuBaseURL(tc.settingURL, tc.envExplicit, tc.mode)
			if got != tc.want {
				t.Fatalf("resolveDuitkuBaseURL(%q, %q, %q) = %q, want %q",
					tc.settingURL, tc.envExplicit, tc.mode, got, tc.want)
			}
		})
	}
}
