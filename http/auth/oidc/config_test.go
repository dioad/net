package oidc

import (
	"testing"
	"time"
)

func TestOIDCConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name   string
		config OIDCConfig
		want   OIDCConfig
	}{
		{
			name:   "empty config gets all defaults",
			config: OIDCConfig{},
			want: OIDCConfig{
				TokenCookieConfig: OIDCCookieConfig{
					Name:   "token",
					Domain: "",
					MaxAge: 3600,
					Path:   "/",
				},
				StateCookieConfig: OIDCCookieConfig{
					Name:   "state",
					Domain: "",
					MaxAge: 600,
					Path:   "/",
				},
				TokenExpiryCookieConfig: OIDCCookieConfig{
					Name:   "token_expiry",
					Domain: "",
					MaxAge: 3600,
					Path:   "/",
				},
				RefreshCookieConfig: OIDCCookieConfig{
					Name:   "refresh",
					Domain: "",
					MaxAge: 86400,
					Path:   "/",
				},
				RedirectCookieConfig: OIDCCookieConfig{
					Name:   "redirect",
					Domain: "",
					MaxAge: 600,
					Path:   "/",
				},
			},
		},
		{
			name: "partial config preserves set values",
			config: OIDCConfig{
				TokenCookieConfig: OIDCCookieConfig{
					Name:   "custom_token",
					Domain: "example.com",
					MaxAge: 7200,
					Path:   "/auth",
				},
			},
			want: OIDCConfig{
				TokenCookieConfig: OIDCCookieConfig{
					Name:   "custom_token",
					Domain: "example.com",
					MaxAge: 7200,
					Path:   "/auth",
				},
				StateCookieConfig: OIDCCookieConfig{
					Name:   "state",
					Domain: "",
					MaxAge: 600,
					Path:   "/",
				},
				TokenExpiryCookieConfig: OIDCCookieConfig{
					Name:   "token_expiry",
					Domain: "",
					MaxAge: 3600,
					Path:   "/",
				},
				RefreshCookieConfig: OIDCCookieConfig{
					Name:   "refresh",
					Domain: "",
					MaxAge: 86400,
					Path:   "/",
				},
				RedirectCookieConfig: OIDCCookieConfig{
					Name:   "redirect",
					Domain: "",
					MaxAge: 600,
					Path:   "/",
				},
			},
		},
		{
			name: "zero MaxAge gets replaced with default",
			config: OIDCConfig{
				TokenCookieConfig: OIDCCookieConfig{
					Name:   "token",
					Domain: "example.com",
					MaxAge: 0,
					Path:   "/auth",
				},
			},
			want: OIDCConfig{
				TokenCookieConfig: OIDCCookieConfig{
					Name:   "token",
					Domain: "example.com",
					MaxAge: 3600,
					Path:   "/auth",
				},
				StateCookieConfig: OIDCCookieConfig{
					Name:   "state",
					Domain: "",
					MaxAge: 600,
					Path:   "/",
				},
				TokenExpiryCookieConfig: OIDCCookieConfig{
					Name:   "token_expiry",
					Domain: "",
					MaxAge: 3600,
					Path:   "/",
				},
				RefreshCookieConfig: OIDCCookieConfig{
					Name:   "refresh",
					Domain: "",
					MaxAge: 86400,
					Path:   "/",
				},
				RedirectCookieConfig: OIDCCookieConfig{
					Name:   "redirect",
					Domain: "",
					MaxAge: 600,
					Path:   "/",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.config
			config.SetDefaults()

			// Check each cookie config
			checkCookieConfig(t, "TokenCookieConfig", config.TokenCookieConfig, tt.want.TokenCookieConfig)
			checkCookieConfig(t, "StateCookieConfig", config.StateCookieConfig, tt.want.StateCookieConfig)
			checkCookieConfig(t, "TokenExpiryCookieConfig", config.TokenExpiryCookieConfig, tt.want.TokenExpiryCookieConfig)
			checkCookieConfig(t, "RefreshCookieConfig", config.RefreshCookieConfig, tt.want.RefreshCookieConfig)
			checkCookieConfig(t, "RedirectCookieConfig", config.RedirectCookieConfig, tt.want.RedirectCookieConfig)

			// Check that Now function was set
			if config.Now == nil {
				t.Errorf("Now function should be set")
			}
		})
	}
}

func checkCookieConfig(t *testing.T, name string, got, want OIDCCookieConfig) {
	t.Helper()
	if got.Name != want.Name {
		t.Errorf("%s.Name = %v, want %v", name, got.Name, want.Name)
	}
	if got.Domain != want.Domain {
		t.Errorf("%s.Domain = %v, want %v", name, got.Domain, want.Domain)
	}
	if got.MaxAge != want.MaxAge {
		t.Errorf("%s.MaxAge = %v, want %v", name, got.MaxAge, want.MaxAge)
	}
	if got.Path != want.Path {
		t.Errorf("%s.Path = %v, want %v", name, got.Path, want.Path)
	}
}

func TestOIDCConfig_SetDefaults_NowFunction(t *testing.T) {
	config := OIDCConfig{}
	config.SetDefaults()

	if config.Now == nil {
		t.Fatal("Now function should be set after SetDefaults")
	}

	// Verify the Now function works
	now := config.Now()
	if now.IsZero() {
		t.Error("Now() should return a non-zero time")
	}

	// Verify it's close to the current time
	if time.Since(now) > time.Second {
		t.Error("Now() should return approximately the current time")
	}
}

func TestOIDCConfig_SetDefaults_CustomNowFunction(t *testing.T) {
	customTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	customNow := func() time.Time {
		return customTime
	}

	config := OIDCConfig{
		Now: customNow,
	}
	config.SetDefaults()

	// Verify custom Now function is preserved
	if config.Now() != customTime {
		t.Errorf("Custom Now function should be preserved, got %v, want %v", config.Now(), customTime)
	}
}

func TestOIDCCookieConfig_Defaults(t *testing.T) {
	config := OIDCConfig{}
	config.SetDefaults()

	// Verify each cookie has appropriate defaults
	tests := []struct {
		name       string
		cookieConf OIDCCookieConfig
		wantName   string
		wantMaxAge int
	}{
		{"TokenCookieConfig", config.TokenCookieConfig, "token", 3600},
		{"StateCookieConfig", config.StateCookieConfig, "state", 600},
		{"TokenExpiryCookieConfig", config.TokenExpiryCookieConfig, "token_expiry", 3600},
		{"RefreshCookieConfig", config.RefreshCookieConfig, "refresh", 86400},
		{"RedirectCookieConfig", config.RedirectCookieConfig, "redirect", 600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cookieConf.Name != tt.wantName {
				t.Errorf("Name = %v, want %v", tt.cookieConf.Name, tt.wantName)
			}
			if tt.cookieConf.MaxAge != tt.wantMaxAge {
				t.Errorf("MaxAge = %v, want %v", tt.cookieConf.MaxAge, tt.wantMaxAge)
			}
			if tt.cookieConf.Path != "/" {
				t.Errorf("Path = %v, want /", tt.cookieConf.Path)
			}
		})
	}
}
