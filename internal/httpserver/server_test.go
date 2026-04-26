package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"customerdeliverylog/internal/config"
	"customerdeliverylog/internal/pkg/storage"
	"customerdeliverylog/internal/service"
	"customerdeliverylog/internal/store/memory"
)

func TestLoginProjectAndUpgradeFlow(t *testing.T) {
	cfg := config.Config{
		TokenSecret:       "secret",
		AccessTokenTTL:    time.Hour,
		RefreshTokenTTL:   24 * time.Hour,
		UploadDir:         filepath.Join(t.TempDir(), "uploads"),
		SeedAdminUsername: "admin",
		SeedAdminPassword: "Admin@123456",
		SeedAdminRealName: "系统管理员",
	}

	st := memory.New()
	svc := service.New(cfg, st, storage.LocalStorage{BaseDir: cfg.UploadDir})
	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	server := New(svc, "")

	loginBody := map[string]any{
		"username": "admin",
		"password": "Admin@123456",
	}
	loginResp := performJSONRequest(t, server.Handler(), http.MethodPost, "/api/v1/auth/login", "", loginBody)
	accessToken := loginResp["data"].(map[string]any)["access_token"].(string)

	projectBody := map[string]any{
		"project_name":        "接口测试项目",
		"customer_name":       "接口测试客户",
		"project_status":      "online",
		"implementation_date": "2026-04-25",
		"online_date":         "2026-04-28",
		"acceptance_date":     "2026-05-06",
		"current_version":     "V2.0.0",
		"deploy_mode":         "on_premise",
		"environment_summary": "1台应用 + 1台数据库",
		"customer_contact":    "测试联系人",
		"remark_text":         "自动化测试创建",
	}
	projectResp := performJSONRequest(t, server.Handler(), http.MethodPost, "/api/v1/projects", accessToken, projectBody)
	projectID := int64(projectResp["data"].(map[string]any)["id"].(float64))

	upgradeBody := map[string]any{
		"upgrade_date":     "2026-04-25T12:00:00Z",
		"source_version":   "V2.0.0",
		"target_version":   "V2.1.0",
		"upgrade_status":   "completed",
		"custom_retention": "partial",
		"issue_solution":   "无",
		"test_result":      "通过",
		"remark_text":      "接口测试升级",
	}
	performJSONRequest(t, server.Handler(), http.MethodPost, "/api/v1/projects/"+itoa(projectID)+"/upgrades", accessToken, upgradeBody)

	projectDetail := performJSONRequest(t, server.Handler(), http.MethodGet, "/api/v1/projects/"+itoa(projectID), accessToken, nil)
	currentVersion := projectDetail["data"].(map[string]any)["current_version"].(string)
	if currentVersion != "V2.1.0" {
		t.Fatalf("expected current version V2.1.0, got %s (projectID=%d)", currentVersion, projectID)
	}
	if got := projectDetail["data"].(map[string]any)["online_date"].(string); got != "2026-04-28" {
		t.Fatalf("expected online_date 2026-04-28, got %s", got)
	}
	if got := projectDetail["data"].(map[string]any)["acceptance_date"].(string); got != "2026-05-06" {
		t.Fatalf("expected acceptance_date 2026-05-06, got %s", got)
	}
	ownerUserID := int64(projectDetail["data"].(map[string]any)["owner_user_id"].(float64))
	if ownerUserID != 1 {
		t.Fatalf("expected owner_user_id default to creator, got %d", ownerUserID)
	}
}

func TestLoginReturnsPasswordErrorMessage(t *testing.T) {
	cfg := config.Config{
		TokenSecret:       "secret",
		AccessTokenTTL:    time.Hour,
		RefreshTokenTTL:   24 * time.Hour,
		UploadDir:         filepath.Join(t.TempDir(), "uploads"),
		SeedAdminUsername: "admin",
		SeedAdminPassword: "Admin@123456",
		SeedAdminRealName: "系统管理员",
	}

	st := memory.New()
	svc := service.New(cfg, st, storage.LocalStorage{BaseDir: cfg.UploadDir})
	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	server := New(svc, "")

	resp := performJSONRequestWithStatus(t, server.Handler(), http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"username": "admin",
		"password": "wrong-password",
	}, http.StatusUnauthorized)

	if got := resp["message"]; got != "账号或密码错误" {
		t.Fatalf("expected password error message, got %#v", got)
	}
}

func TestNonAdminUserCanAccessAllDataAndManageProjects(t *testing.T) {
	cfg := config.Config{
		TokenSecret:       "secret",
		AccessTokenTTL:    time.Hour,
		RefreshTokenTTL:   24 * time.Hour,
		UploadDir:         filepath.Join(t.TempDir(), "uploads"),
		SeedAdminUsername: "admin",
		SeedAdminPassword: "Admin@123456",
		SeedAdminRealName: "系统管理员",
	}

	st := memory.New()
	svc := service.New(cfg, st, storage.LocalStorage{BaseDir: cfg.UploadDir})
	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	server := New(svc, "")

	adminLogin := performJSONRequest(t, server.Handler(), http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"username": "admin",
		"password": "Admin@123456",
	})
	adminToken := adminLogin["data"].(map[string]any)["access_token"].(string)

	performJSONRequest(t, server.Handler(), http.MethodPost, "/api/v1/users", adminToken, map[string]any{
		"username":  "engineer01",
		"real_name": "实施工程师",
		"password":  "Pass@123456",
		"status":    true,
	})

	userLogin := performJSONRequest(t, server.Handler(), http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"username": "engineer01",
		"password": "Pass@123456",
	})
	userToken := userLogin["data"].(map[string]any)["access_token"].(string)

	usersResp := performJSONRequest(t, server.Handler(), http.MethodGet, "/api/v1/users", userToken, nil)
	users := usersResp["data"].([]any)
	if len(users) < 2 {
		t.Fatalf("expected non-admin user to list all users, got %d", len(users))
	}

	projectResp := performJSONRequest(t, server.Handler(), http.MethodPost, "/api/v1/projects", userToken, map[string]any{
		"project_name":        "普通用户项目",
		"customer_name":       "普通用户客户",
		"project_status":      "online",
		"implementation_date": "2026-04-25",
		"current_version":     "V1.0.0",
		"deploy_mode":         "standalone",
	})
	project := projectResp["data"].(map[string]any)
	if got := int64(project["owner_user_id"].(float64)); got != 2 {
		t.Fatalf("expected new project owner to be current user, got %d", got)
	}
}

func TestUpdateUserAcceptsLegacyFieldsAndPreservesRoles(t *testing.T) {
	cfg := config.Config{
		TokenSecret:       "secret",
		AccessTokenTTL:    time.Hour,
		RefreshTokenTTL:   24 * time.Hour,
		UploadDir:         filepath.Join(t.TempDir(), "uploads"),
		SeedAdminUsername: "admin",
		SeedAdminPassword: "Admin@123456",
		SeedAdminRealName: "系统管理员",
	}

	st := memory.New()
	svc := service.New(cfg, st, storage.LocalStorage{BaseDir: cfg.UploadDir})
	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	server := New(svc, "")

	adminLogin := performJSONRequest(t, server.Handler(), http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"username": "admin",
		"password": "Admin@123456",
	})
	adminToken := adminLogin["data"].(map[string]any)["access_token"].(string)

	createResp := performJSONRequest(t, server.Handler(), http.MethodPost, "/api/v1/users", adminToken, map[string]any{
		"username":  "tester01",
		"real_name": "测试用户",
		"password":  "Pass@123456",
		"roles":     []string{"engineer"},
		"status":    true,
	})
	userID := int64(createResp["data"].(map[string]any)["id"].(float64))

	updateResp := performJSONRequest(t, server.Handler(), http.MethodPut, "/api/v1/users/"+itoa(userID), adminToken, map[string]any{
		"username":  "tester01",
		"password":  "",
		"real_name": "测试用户已更新",
		"status":    true,
	})

	updated := updateResp["data"].(map[string]any)
	if got := updated["username"].(string); got != "tester01" {
		t.Fatalf("expected username stay unchanged, got %s", got)
	}
	if got := updated["real_name"].(string); got != "测试用户已更新" {
		t.Fatalf("expected real_name updated, got %s", got)
	}
	roles := updated["roles"].([]any)
	if len(roles) != 1 || roles[0].(string) != "engineer" {
		t.Fatalf("expected roles preserved, got %#v", roles)
	}
}

func TestIssueRecordAcceptsIssueVersionAndDashboardStats(t *testing.T) {
	cfg := config.Config{
		TokenSecret:       "secret",
		AccessTokenTTL:    time.Hour,
		RefreshTokenTTL:   24 * time.Hour,
		UploadDir:         filepath.Join(t.TempDir(), "uploads"),
		SeedAdminUsername: "admin",
		SeedAdminPassword: "Admin@123456",
		SeedAdminRealName: "系统管理员",
	}

	st := memory.New()
	svc := service.New(cfg, st, storage.LocalStorage{BaseDir: cfg.UploadDir})
	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	server := New(svc, "")

	loginResp := performJSONRequest(t, server.Handler(), http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"username": "admin",
		"password": "Admin@123456",
	})
	accessToken := loginResp["data"].(map[string]any)["access_token"].(string)

	projectResp := performJSONRequest(t, server.Handler(), http.MethodPost, "/api/v1/projects", accessToken, map[string]any{
		"project_name":        "问题版本接口测试",
		"customer_name":       "测试客户",
		"project_status":      "online",
		"implementation_date": "2026-04-25",
		"current_version":     "V5.0.0",
		"deploy_mode":         "standalone",
	})
	projectID := int64(projectResp["data"].(map[string]any)["id"].(float64))

	issueResp := performJSONRequest(t, server.Handler(), http.MethodPost, "/api/v1/projects/"+itoa(projectID)+"/service-records", accessToken, map[string]any{
		"service_type":  "incident",
		"service_mode":  "onsite",
		"service_date":  "2026-04-25T12:00:00Z",
		"summary":       "导出报错",
		"issue_version": "V5.0.0",
		"problem_desc":  "用户导出时失败",
		"result_desc":   "修复参数",
	})
	issueID := int64(issueResp["data"].(map[string]any)["id"].(float64))

	performJSONRequest(t, server.Handler(), http.MethodPost, "/api/v1/projects/"+itoa(projectID)+"/service-records", accessToken, map[string]any{
		"service_type":  "incident",
		"service_mode":  "remote",
		"service_date":  "2026-05-02T08:00:00Z",
		"summary":       "次月问题",
		"issue_version": "V6.0.0",
		"problem_desc":  "跨月份统计验证",
		"result_desc":   "已处理",
	})

	serviceListResp := performJSONRequest(t, server.Handler(), http.MethodGet, "/api/v1/projects/"+itoa(projectID)+"/service-records?page=1&page_size=20", accessToken, nil)
	list := serviceListResp["data"].(map[string]any)["list"].([]any)
	if len(list) != 2 {
		t.Fatalf("expected 2 service records, got %d", len(list))
	}
	versions := map[string]bool{}
	for _, raw := range list {
		item := raw.(map[string]any)
		versions[item["issue_version"].(string)] = true
	}
	if !versions["V5.0.0"] || !versions["V6.0.0"] {
		t.Fatalf("expected issue versions V5.0.0 and V6.0.0, got %#v", versions)
	}

	dashboardResp := performJSONRequest(t, server.Handler(), http.MethodGet, "/api/v1/dashboard/overview?month=2026-04", accessToken, nil)
	stats := dashboardResp["data"].(map[string]any)["issue_version_stats"].([]any)
	if len(stats) != 2 {
		t.Fatalf("expected 2 issue version stats, got %d", len(stats))
	}
	statMap := map[string]int{}
	for _, raw := range stats {
		item := raw.(map[string]any)
		statMap[item["issue_version"].(string)] = int(item["issue_count"].(float64))
	}
	if statMap["V5.0.0"] != 1 || statMap["V6.0.0"] != 1 {
		t.Fatalf("unexpected issue version stats: %#v", statMap)
	}

	issueDetail := performJSONRequest(t, server.Handler(), http.MethodGet, "/api/v1/service-records/"+itoa(issueID), accessToken, nil)
	if got := issueDetail["data"].(map[string]any)["issue_version"].(string); got != "V5.0.0" {
		t.Fatalf("expected issue detail version V5.0.0, got %q", got)
	}

	issueSummaryResp := performJSONRequest(t, server.Handler(), http.MethodGet, "/api/v1/issues?keyword=问题版本接口测试&issue_version=V5.0.0&page=1&page_size=20", accessToken, nil)
	issueRows := issueSummaryResp["data"].(map[string]any)["list"].([]any)
	if len(issueRows) != 1 {
		t.Fatalf("expected 1 issue summary row, got %d", len(issueRows))
	}
	summaryItem := issueRows[0].(map[string]any)
	if summaryItem["project_name"].(string) != "问题版本接口测试" {
		t.Fatalf("expected project_name in issue summary, got %#v", summaryItem["project_name"])
	}
}

func performJSONRequest(t *testing.T, handler http.Handler, method, path, token string, body any) map[string]any {
	t.Helper()
	return performJSONRequestWithStatus(t, handler, method, path, token, body, 0)
}

func performJSONRequestWithStatus(t *testing.T, handler http.Handler, method, path, token string, body any, expectedStatus int) map[string]any {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body failed: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if expectedStatus > 0 {
		if recorder.Code != expectedStatus {
			t.Fatalf("expected status %d, got %d: %s", expectedStatus, recorder.Code, recorder.Body.String())
		}
	} else if recorder.Code >= 400 {
		t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	return resp
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
