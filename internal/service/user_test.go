package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"customerdeliverylog/internal/config"
	"customerdeliverylog/internal/model"
	"customerdeliverylog/internal/pkg/storage"
	"customerdeliverylog/internal/store/memory"
)

func TestDeleteUserRules(t *testing.T) {
	ctx := context.Background()
	st := memory.New()
	cfg := config.Config{
		TokenSecret:       "secret",
		AccessTokenTTL:    time.Hour,
		RefreshTokenTTL:   24 * time.Hour,
		UploadDir:         filepath.Join(t.TempDir(), "uploads"),
		SeedAdminUsername: "admin",
		SeedAdminPassword: "Admin@123456",
		SeedAdminRealName: "管理员",
	}
	svc := New(cfg, st, storage.LocalStorage{BaseDir: cfg.UploadDir})
	if err := svc.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	if err := svc.DeleteUser(ctx, 1, 1); err == nil {
		t.Fatalf("expected admin user deletion to be blocked")
	}

	user := &model.User{
		Username: "public",
		RealName: "普通用户",
		Roles:    []string{},
		Status:   true,
	}
	if err := svc.CreateUser(ctx, 1, user, "Public@123"); err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	firstID := user.ID
	if err := svc.DeleteUser(ctx, 1, firstID); err != nil {
		t.Fatalf("first delete failed: %v", err)
	}

	userAgain := &model.User{
		Username: "public",
		RealName: "普通用户2",
		Roles:    []string{},
		Status:   true,
	}
	if err := svc.CreateUser(ctx, 1, userAgain, "Public@123"); err != nil {
		t.Fatalf("recreate user failed: %v", err)
	}
	if userAgain.ID == firstID {
		t.Fatalf("expected recreated user to have a new id")
	}
	if err := svc.DeleteUser(ctx, 1, userAgain.ID); err != nil {
		t.Fatalf("second delete failed: %v", err)
	}
}
