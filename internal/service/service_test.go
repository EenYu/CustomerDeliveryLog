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

func TestUpgradeSyncsProjectVersion(t *testing.T) {
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

	project := &model.Project{
		ProjectName:        "测试项目",
		CustomerName:       "测试客户",
		ProjectStatus:      "online",
		ImplementationDate: "2026-04-25",
		CurrentVersion:     "V1.0.0",
	}
	if err := svc.CreateProject(ctx, 1, project); err != nil {
		t.Fatalf("create project failed: %v", err)
	}

	upgrade := &model.UpgradeRecord{
		ProjectID:       project.ID,
		UpgradeDate:     time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC),
		SourceVersion:   "V1.0.0",
		TargetVersion:   "V1.1.0",
		UpgradeStatus:   "completed",
		CustomRetention: "partial",
	}
	if err := svc.CreateUpgrade(ctx, 1, upgrade); err != nil {
		t.Fatalf("create upgrade failed: %v", err)
	}

	savedProject, err := svc.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("get project failed: %v", err)
	}
	if savedProject.CurrentVersion != "V1.1.0" {
		t.Fatalf("expected current version to be updated, got %s", savedProject.CurrentVersion)
	}
	if savedProject.LastUpgradeAt == nil {
		t.Fatalf("expected last upgrade time to be set")
	}
	if savedProject.OwnerUserID != 1 {
		t.Fatalf("expected project owner default to creator, got %d", savedProject.OwnerUserID)
	}

	savedUpgrade, err := svc.GetUpgrade(ctx, upgrade.ID)
	if err != nil {
		t.Fatalf("get upgrade failed: %v", err)
	}
	if savedUpgrade.OwnerUserID != 1 {
		t.Fatalf("expected upgrade owner default to creator, got %d", savedUpgrade.OwnerUserID)
	}
}

func TestDeleteProjectCascadesRecordsAndDashboardStats(t *testing.T) {
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

	project := &model.Project{
		ProjectName:        "待删除项目",
		CustomerName:       "测试客户",
		ProjectStatus:      "online",
		ImplementationDate: "2026-04-25",
		CurrentVersion:     "V1.0.0",
	}
	if err := svc.CreateProject(ctx, 1, project); err != nil {
		t.Fatalf("create project failed: %v", err)
	}

	upgrade := &model.UpgradeRecord{
		ProjectID:       project.ID,
		UpgradeDate:     time.Now(),
		SourceVersion:   "V1.0.0",
		TargetVersion:   "V1.1.0",
		UpgradeStatus:   "completed",
		CustomRetention: "partial",
	}
	if err := svc.CreateUpgrade(ctx, 1, upgrade); err != nil {
		t.Fatalf("create upgrade failed: %v", err)
	}

	if err := svc.DeleteProject(ctx, 1, project.ID); err != nil {
		t.Fatalf("delete project failed: %v", err)
	}

	if _, err := svc.GetProject(ctx, project.ID); err == nil {
		t.Fatalf("expected deleted project to be unavailable")
	}

	upgrades, err := svc.ListUpgrades(ctx, project.ID, model.ListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list upgrades failed: %v", err)
	}
	if len(upgrades.List) != 0 {
		t.Fatalf("expected project upgrades to be removed, got %d", len(upgrades.List))
	}

	overview, err := svc.DashboardOverview(ctx, "")
	if err != nil {
		t.Fatalf("dashboard overview failed: %v", err)
	}
	if overview.ProjectTotal != 0 {
		t.Fatalf("expected dashboard project total 0, got %d", overview.ProjectTotal)
	}
	if overview.MonthlyUpgradeNum != 0 {
		t.Fatalf("expected dashboard monthly upgrade count 0, got %d", overview.MonthlyUpgradeNum)
	}
}

func TestIssueVersionStatsAndOptionalIssueVersion(t *testing.T) {
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

	project := &model.Project{
		ProjectName:        "问题版本测试项目",
		CustomerName:       "测试客户",
		ProjectStatus:      "online",
		ImplementationDate: "2026-04-01",
		CurrentVersion:     "V3.2.0",
	}
	if err := svc.CreateProject(ctx, 1, project); err != nil {
		t.Fatalf("create project failed: %v", err)
	}

	issueAtVersion := &model.ServiceRecord{
		ProjectID:     project.ID,
		ServiceType:   "incident",
		ServiceMode:   "onsite",
		ServiceDate:   time.Date(2026, 4, 10, 10, 0, 0, 0, time.Local),
		Summary:       "登录失败",
		IssueVersion:  "V3.2.0",
		ProblemDesc:   "用户反馈无法登录",
		ResultDesc:    "修复配置",
		OwnerUserID:   1,
	}
	if err := svc.CreateServiceRecord(ctx, 1, issueAtVersion); err != nil {
		t.Fatalf("create issue with version failed: %v", err)
	}

	issueWithoutVersion := &model.ServiceRecord{
		ProjectID:    project.ID,
		ServiceType:  "incident",
		ServiceMode:  "remote",
		ServiceDate:  time.Date(2026, 4, 12, 9, 0, 0, 0, time.Local),
		Summary:      "误操作导致数据异常",
		ProblemDesc:  "用户使用不当",
		ResultDesc:   "指导恢复",
		OwnerUserID:  1,
	}
	if err := svc.CreateServiceRecord(ctx, 1, issueWithoutVersion); err != nil {
		t.Fatalf("create issue without version failed: %v", err)
	}

	issueOtherMonth := &model.ServiceRecord{
		ProjectID:     project.ID,
		ServiceType:   "incident",
		ServiceMode:   "remote",
		ServiceDate:   time.Date(2026, 5, 3, 11, 0, 0, 0, time.Local),
		Summary:       "打印异常",
		IssueVersion:  "V4.0.0",
		ProblemDesc:   "跨月份问题记录",
		ResultDesc:    "已处理",
		OwnerUserID:   1,
	}
	if err := svc.CreateServiceRecord(ctx, 1, issueOtherMonth); err != nil {
		t.Fatalf("create issue in other month failed: %v", err)
	}

	serviceRecord := &model.ServiceRecord{
		ProjectID:    project.ID,
		ServiceType:  "support",
		ServiceMode:  "remote",
		ServiceDate:  time.Date(2026, 4, 15, 15, 0, 0, 0, time.Local),
		Summary:      "日常巡检",
		IssueVersion: "V3.2.0",
		OwnerUserID:  1,
	}
	if err := svc.CreateServiceRecord(ctx, 1, serviceRecord); err != nil {
		t.Fatalf("create service record failed: %v", err)
	}

	savedService, err := svc.GetServiceRecord(ctx, serviceRecord.ID)
	if err != nil {
		t.Fatalf("get service record failed: %v", err)
	}
	if savedService.IssueVersion != "" {
		t.Fatalf("expected non-incident service record issue version to be cleared, got %q", savedService.IssueVersion)
	}

	savedIssue, err := svc.GetServiceRecord(ctx, issueAtVersion.ID)
	if err != nil {
		t.Fatalf("get issue record failed: %v", err)
	}
	if savedIssue.IssueVersion != "V3.2.0" {
		t.Fatalf("expected issue version to persist, got %q", savedIssue.IssueVersion)
	}

	overview, err := svc.DashboardOverview(ctx, "2026-04")
	if err != nil {
		t.Fatalf("dashboard overview failed: %v", err)
	}
	if overview.MonthlyIssueNum != 2 {
		t.Fatalf("expected monthly issue count 2, got %d", overview.MonthlyIssueNum)
	}
	if overview.MonthlyServiceNum != 1 {
		t.Fatalf("expected monthly service count 1, got %d", overview.MonthlyServiceNum)
	}
	stats := map[string]int{}
	for _, item := range overview.IssueVersionStats {
		stats[item.IssueVersion] = item.IssueCount
	}
	if stats["V3.2.0"] != 1 {
		t.Fatalf("expected V3.2.0 issue count 1, got %d", stats["V3.2.0"])
	}
	if stats["未填写版本"] != 1 {
		t.Fatalf("expected empty issue version bucket count 1, got %d", stats["未填写版本"])
	}
	if stats["V4.0.0"] != 1 {
		t.Fatalf("expected cross-month issue version bucket count 1, got %d", stats["V4.0.0"])
	}

	issues, err := svc.ListIssueRecords(ctx, model.IssueFilter{
		Keyword:      "问题版本测试项目",
		IssueVersion: "V3.2.0",
		Page:         1,
		PageSize:     20,
	})
	if err != nil {
		t.Fatalf("list issue records failed: %v", err)
	}
	if len(issues.List) != 1 {
		t.Fatalf("expected 1 filtered issue record, got %d", len(issues.List))
	}
	if issues.List[0].ProjectName != "问题版本测试项目" {
		t.Fatalf("expected project name to be filled, got %q", issues.List[0].ProjectName)
	}
}
