package main

import (
	"strings"
	"testing"
)

// AC27: the process, not only config.Load, refuses to start in production
// with a development value, naming the variable.
func TestRunRefusesDevelopmentValuesInProduction(t *testing.T) {
	env := map[string]string{
		"WORKSPOTS_ENV": "production",
		"BASE_URL":      "https://workspots.example",
		"DATABASE_URL":  "postgres://workspots_app:dev-app@localhost:5432/workspots_dev",
	}
	err := run(nil, func(k string) string { return env[k] })
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || strings.Contains(err.Error(), "dev-app") {
		t.Fatalf("got %v", err)
	}
}
