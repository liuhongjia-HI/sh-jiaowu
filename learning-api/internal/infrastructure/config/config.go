package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	App struct {
		Name string
		Env  string
	}
	HTTP struct {
		Port int
	}
	CORS struct {
		AllowedOrigins []string
	}
	Auth struct {
		TokenSecret string
	}
	Demo struct {
		SeedData             bool
		AdminPasswordLogin   bool
		StudentPasswordLogin bool
	}
	BootstrapAdmin struct {
		Name     string
		Phone    string
		Password string
	}
	Wechat struct {
		AppID  string
		Secret string
	}
	MySQL struct {
		DSN string
	}
	Redis struct {
		Addr string
	}
	RabbitMQ struct {
		URL string
	}
	Nacos struct {
		Enabled bool
		DataID  string
		Group   string
	}
}

func MustLoad() *Config {
	cfg := &Config{}
	cfg.App.Name = getString("APP_NAME", "starline-learning-api")
	cfg.App.Env = getString("APP_ENV", "local")
	cfg.HTTP.Port = getInt("HTTP_PORT", 8892)
	cfg.CORS.AllowedOrigins = getCSV("CORS_ALLOWED_ORIGINS", defaultCORSAllowedOrigins(cfg.App.Env))
	cfg.Auth.TokenSecret = getString("AUTH_TOKEN_SECRET", "starline-local-dev-secret")
	cfg.Demo.SeedData = getBool("DEMO_SEED_DATA", cfg.App.Env != "production")
	cfg.Demo.AdminPasswordLogin = getBool("ADMIN_PASSWORD_LOGIN_ENABLED", cfg.App.Env != "production")
	cfg.Demo.StudentPasswordLogin = getBool("DEMO_STUDENT_LOGIN_ENABLED", cfg.App.Env != "production")
	cfg.BootstrapAdmin.Name = getString("BOOTSTRAP_ADMIN_NAME", "超级管理员")
	cfg.BootstrapAdmin.Phone = getString("BOOTSTRAP_ADMIN_PHONE", "")
	cfg.BootstrapAdmin.Password = getString("BOOTSTRAP_ADMIN_PASSWORD", "")
	cfg.Wechat.AppID = getString("WECHAT_APPID", "")
	cfg.Wechat.Secret = getString("WECHAT_SECRET", "")
	cfg.MySQL.DSN = getString("MYSQL_DSN", "app:app123@tcp(127.0.0.1:3317)/starline?charset=utf8mb4&parseTime=True&loc=Local")
	cfg.Redis.Addr = getString("REDIS_ADDR", "127.0.0.1:6380")
	cfg.RabbitMQ.URL = getString("RABBITMQ_URL", "amqp://app:app123@127.0.0.1:5674/starline")
	cfg.Nacos.Enabled = getBool("NACOS_ENABLED", false)
	cfg.Nacos.DataID = getString("NACOS_DATA_ID", "starline-learning-api.yaml")
	cfg.Nacos.Group = getString("NACOS_GROUP", "mall")
	return cfg
}

func defaultCORSAllowedOrigins(env string) []string {
	origins := []string{"https://sa.starlineeducation.com.cn"}
	if env == "production" {
		return origins
	}
	return append(origins,
		"http://sa.starlineeducation.com.cn",
		"http://localhost:5173",
		"http://127.0.0.1:5173",
	)
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}
	if c.App.Env != "production" {
		return nil
	}
	missing := []string{}
	if isDefaultOrEmpty(c.Auth.TokenSecret, "starline-local-dev-secret") {
		missing = append(missing, "AUTH_TOKEN_SECRET")
	}
	if strings.TrimSpace(c.MySQL.DSN) == "" || strings.Contains(c.MySQL.DSN, "127.0.0.1:3317") || strings.Contains(c.MySQL.DSN, "app:app123") {
		missing = append(missing, "MYSQL_DSN")
	}
	if strings.TrimSpace(c.Wechat.AppID) == "" {
		missing = append(missing, "WECHAT_APPID")
	}
	if strings.TrimSpace(c.Wechat.Secret) == "" {
		missing = append(missing, "WECHAT_SECRET")
	}
	if c.Demo.SeedData {
		missing = append(missing, "DEMO_SEED_DATA=false")
	}
	if c.Demo.StudentPasswordLogin {
		missing = append(missing, "DEMO_STUDENT_LOGIN_ENABLED=false")
	}
	if !c.Demo.AdminPasswordLogin {
		return errors.New("production admin password login must remain enabled until SSO is implemented")
	}
	if len(missing) > 0 {
		return errors.New("production config is not ready: " + strings.Join(missing, ", "))
	}
	return nil
}

func isDefaultOrEmpty(value, fallback string) bool {
	value = strings.TrimSpace(value)
	return value == "" || value == fallback
}

func getString(env string, fallback string) string {
	if value := os.Getenv(env); value != "" {
		return value
	}
	return fallback
}

func getCSV(env string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(env))
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return fallback
	}
	return items
}

func getInt(env string, fallback int) int {
	if value := os.Getenv(env); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func getBool(env string, fallback bool) bool {
	if value := os.Getenv(env); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
