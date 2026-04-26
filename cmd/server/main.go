package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"customerdeliverylog/internal/config"
	"customerdeliverylog/internal/httpserver"
	"customerdeliverylog/internal/pkg/storage"
	"customerdeliverylog/internal/service"
	storeiface "customerdeliverylog/internal/store"
	mysqlstore "customerdeliverylog/internal/store/mysql"
	"customerdeliverylog/internal/store/memory"
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
	}

	log.Printf("server listening on %s", cfg.ListenAddr)
	log.Printf("storage backend: %s", cfg.StorageBackend)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen failed: %v", err)
	}
}
