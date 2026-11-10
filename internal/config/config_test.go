package config_test

import (
	"strings"
	"testing"

	"github.com/rydncn/workspots/internal/config"
)

const devDB = "postgres://workspots_app:dev-app@localhost:5432/workspots_dev"

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// AC27: production refuses missing or development values, naming the variable
// and never its value; development mode starts with the same configuration.
func TestAC27ProductionRefusesDevelopmentValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		env  map[string]string
		want string // substring of the error; the variable's name
	}{
		{"missing DATABASE_URL", map[string]string{"WORKSPOTS_ENV": "production", "BASE_URL": "https://workspots.example"}, "DATABASE_URL"},
		{"development password", map[string]string{"WORKSPOTS_ENV": "production", "BASE_URL": "https://workspots.example", "DATABASE_URL": devDB}, "DATABASE_URL"},
		{"missing BASE_URL", map[string]string{"WORKSPOTS_ENV": "production", "DATABASE_URL": "postgres://workspots_app:s3cret-prod@db/workspots"}, "BASE_URL"},
		{"http BASE_URL", map[string]string{"WORKSPOTS_ENV": "production", "BASE_URL": "http://workspots.example", "DATABASE_URL": "postgres://workspots_app:s3cret-prod@db/workspots"}, "BASE_URL"},
		// Production always sits behind the TLS proxy, so without TRUSTED_PROXY every
		// client would share the proxy's address and one client's failures would
		// rate-limit everyone (security audit, slice 6).
		{"missing TRUSTED_PROXY", map[string]string{"WORKSPOTS_ENV": "production", "BASE_URL": "https://workspots.example", "DATABASE_URL": "postgres://workspots_app:s3cret-prod@db/workspots"}, "TRUSTED_PROXY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Load(env(tc.env))
			if err == nil {
				t.Fatalf("expected an error naming %s", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %s", err, tc.want)
			}
			if strings.Contains(err.Error(), "s3cret-prod") || strings.Contains(err.Error(), "dev-app") {
				t.Errorf("error %q repeats a value", err)
			}
		})
	}

	t.Run("production with real values starts", func(t *testing.T) {
		cfg, err := config.Load(env(map[string]string{"WORKSPOTS_ENV": "production", "BASE_URL": "https://workspots.example", "DATABASE_URL": "postgres://workspots_app:s3cret-prod@db/workspots", "TRUSTED_PROXY": "10.0.0.2"}))
		if err != nil || !cfg.Production() {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
	t.Run("development accepts the same values production refuses", func(t *testing.T) {
		cfg, err := config.Load(env(map[string]string{"DATABASE_URL": devDB}))
		if err != nil || cfg.Production() || cfg.DatabaseURL != devDB || cfg.Addr == "" || cfg.BaseURL == "" {
			t.Fatalf("got %+v, %v", cfg, err)
		}
	})
}
