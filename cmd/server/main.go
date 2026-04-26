package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"customerdeliverylog/internal/config"
	"customerdeliverylog/internal/httpserver"
	"customerdeliverylog/internal/pkg/storage"
	"customerdeliverylog/internal/service"
	storeiface "customerdeliverylog/internal/store"
	"customerdeliverylog/internal/store/memory"
	mysqlstore "customerdeliverylog/internal/store/mysql"
)

func main() {
	cfg := config.Load()

	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("create upload dir failed: %v", err)
	}

	var st storeiface.Store
	switch cfg.StorageBackend {
	case "mysql":
		if cfg.MySQLDSN == "" {
			log.Fatalf("storage backend is mysql but MYSQL_DSN is empty")
		}
		mysqlSt, err := mysqlstore.New(cfg.MySQLDSN)
		if err != nil {
			log.Fatalf("init mysql store failed: %v", err)
		}
		st = mysqlSt
	default:
		st = memory.New()
	}

	svc := service.New(cfg, st, storage.LocalStorage{BaseDir: cfg.UploadDir})
	if err := svc.Bootstrap(context.Background()); err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}

	server := httpserver.New(svc, cfg.WebDir)

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("server listening on %s", cfg.ListenAddr)
	log.Printf("storage backend: %s", cfg.StorageBackend)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen failed: %v", err)
		}
	case <-ctx.Done():
		log.Printf("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("shutdown failed: %v", err)
		}
	}
}
