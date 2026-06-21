package config

import (
	"strings"
	"testing"
)

func TestProductionConfigRejectsLocalDefaultsAndDemoMode(t *testing.T) {
	cfg := MustLoad()
	cfg.App.Env = "production"
	cfg.Auth.TokenSecret = "starline-local-dev-secret"
	cfg.MySQL.DSN = "app:app123@tcp(127.0.0.1:3317)/starline"
	cfg.Wechat.AppID = ""
	cfg.Wechat.Secret = ""
	cfg.Demo.SeedData = true
	cfg.Demo.StudentPasswordLogin = true
	cfg.Demo.AdminPasswordLogin = true

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected production config with local defaults to fail")
	}
	message := err.Error()
	for _, want := range []string{"AUTH_TOKEN_SECRET", "MYSQL_DSN", "WECHAT_APPID", "WECHAT_SECRET", "DEMO_SEED_DATA=false", "DEMO_STUDENT_LOGIN_ENABLED=false"} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected error to mention %s, got %q", want, message)
		}
	}
}

func TestProductionConfigAcceptsExplicitProductionValues(t *testing.T) {
	cfg := MustLoad()
	cfg.App.Env = "production"
	cfg.Auth.TokenSecret = "prod-secret-with-enough-entropy"
	cfg.MySQL.DSN = "prod_user:prod_password@tcp(mysql.internal:3306)/starline"
	cfg.Wechat.AppID = "wx-prod"
	cfg.Wechat.Secret = "wx-secret"
	cfg.Demo.SeedData = false
	cfg.Demo.StudentPasswordLogin = false
	cfg.Demo.AdminPasswordLogin = true

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected production config to pass: %v", err)
	}
}
