// Package config reads the configuration from the environment (rule 14).
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Config is everything the process needs from its environment.
type Config struct {
	Env          string // "development" or "production"
	Addr         string
	DatabaseURL  string
	BaseURL      string
	TrustedProxy string
}

// Production reports whether the process runs in production mode.
func (c Config) Production() bool { return c.Env == "production" }

// developmentPasswords are the ones scripts/db-init.sql creates.
var developmentPasswords = map[string]bool{"dev-app": true, "dev-migrate": true}

// Load reads the configuration through getenv (os.Getenv in main; a map in
// tests). In production mode it refuses missing or development values (AC27),
// naming the variable and never its value.
func Load(getenv func(string) string) (Config, error) {
	or := func(name, fallback string) string {
		if v := getenv(name); v != "" {
			return v
		}
		return fallback
	}
	c := Config{
		Env:          or("WORKSPOTS_ENV", "development"),
		Addr:         or("ADDR", "127.0.0.1:8080"),
		DatabaseURL:  getenv("DATABASE_URL"),
		BaseURL:      or("BASE_URL", "http://127.0.0.1:8080"),
		TrustedProxy: getenv("TRUSTED_PROXY"),
	}
	if c.Env != "development" && c.Env != "production" {
		return c, errors.New("WORKSPOTS_ENV must be development or production")
	}
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is not set")
	}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return c, errors.New("DATABASE_URL is not a valid URL")
	}
	if !c.Production() {
		return c, nil
	}
	if pw, _ := u.User.Password(); developmentPasswords[pw] || pw == "" {
		return c, errors.New("DATABASE_URL carries a development or empty password in production")
	}
	if getenv("BASE_URL") == "" {
		return c, errors.New("BASE_URL is not set")
	}
	if !strings.HasPrefix(c.BaseURL, "https://") {
		return c, fmt.Errorf("BASE_URL must be https in production")
	}
	return c, nil
}
