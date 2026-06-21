package main

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"starline/learning-api/internal/application/learningapp"
	"starline/learning-api/internal/infrastructure/config"
	"starline/learning-api/internal/infrastructure/logger"
	"starline/learning-api/internal/infrastructure/store"
	"starline/learning-api/internal/interfaces/http/router"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.MustLoad()
	log := logger.New(cfg.App.Env)
	if err := cfg.Validate(); err != nil {
		log.Errorf("invalid configuration: %v", err)
		return
	}
	repo := store.NewMemoryStoreWithOptions(store.Options{SeedDemoData: cfg.Demo.SeedData})
	if cfg.Wechat.AppID != "" && cfg.Wechat.Secret != "" {
		repo.UseWechatAPI(cfg.Wechat.AppID, cfg.Wechat.Secret)
		log.Infof("wechat login enabled via jscode2session")
	} else {
		log.Infof("wechat login running in demo mode (no WECHAT_APPID/WECHAT_SECRET)")
	}
	if err := repo.ConnectDatabase(cfg.MySQL.DSN); err != nil {
		log.Errorf("mysql is required but unavailable: %v", err)
		return
	}
	log.Infof("mysql persistence enabled")
	service := learningapp.NewService(repo)

	r := router.New(router.Dependencies{
		Config:  cfg,
		Logger:  log,
		Service: service,
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler: r,
	}
	errCh := make(chan error, 1)

	log.Infof("starline learning-api listening on %s", server.Addr)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		log.Errorf("api server failed: %v", err)
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Errorf("api server shutdown failed: %v", err)
	}
}
