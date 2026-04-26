package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"customerdeliverylog/internal/model"
	"customerdeliverylog/internal/pkg/security"
	"customerdeliverylog/internal/service"
	"customerdeliverylog/internal/store"
)

type contextKey string

const claimsContextKey contextKey = "claims"

const attachmentMultipartMemory = 64 << 20

type Server struct {
	svc    *service.Service
	mux    *http.ServeMux
	webDir string
}

func New(svc *service.Service, webDir string) *Server {
	s := &Server{
		svc:    svc,
		mux:    http.NewServeMux(),
		webDir: webDir,
	}
	s.routes(webDir)
	return s
}

func (s *Server) Handler() http.Handler {
	return s.loggingMiddleware(s.mux)
}

func (s *Server) routes(webDir string) {
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/v1/auth/refresh", s.handleRefresh)
	s.mux.Handle("GET /api/v1/auth/me", s.auth(http.HandlerFunc(s.handleMe)))
	s.mux.Handle("POST /api/v1/auth/change-password", s.auth(http.HandlerFunc(s.handleChangePassword)))

	s.mux.Handle("GET /api/v1/dashboard/overview", s.auth(http.HandlerFunc(s.handleDashboardOverview)))
	s.mux.Handle("GET /api/v1/issues", s.auth(http.HandlerFunc(s.handleListIssues)))

	s.mux.Handle("GET /api/v1/users", s.authz(http.HandlerFunc(s.handleListUsers), "admin"))
	s.mux.Handle("POST /api/v1/users", s.authz(http.HandlerFunc(s.handleCreateUser), "admin"))
	s.mux.Handle("PUT /api/v1/users/{id}", s.authz(http.HandlerFunc(s.handleUpdateUser), "admin"))
	s.mux.Handle("DELETE /api/v1/users/{id}", s.authz(http.HandlerFunc(s.handleDeleteUser), "admin"))
	s.mux.Handle("POST /api/v1/users/{id}/reset-password", s.authz(http.HandlerFunc(s.handleResetPassword), "admin"))

	s.mux.Handle("GET /api/v1/login-logs", s.authz(http.HandlerFunc(s.handleListLoginLogs), "admin"))

	s.mux.Handle("GET /api/v1/projects", s.auth(http.HandlerFunc(s.handleListProjects)))
	s.mux.Handle("POST /api/v1/projects", s.authz(http.HandlerFunc(s.handleCreateProject), "admin", "delivery_manager", "engineer"))
	s.mux.Handle("GET /api/v1/projects/{id}", s.auth(http.HandlerFunc(s.handleGetProject)))
	s.mux.Handle("PUT /api/v1/projects/{id}", s.authz(http.HandlerFunc(s.handleUpdateProject), "admin", "delivery_manager", "engineer"))
	s.mux.Handle("DELETE /api/v1/projects/{id}", s.authz(http.HandlerFunc(s.handleDeleteProject), "admin"))
	s.mux.Handle("PATCH /api/v1/projects/{id}/archive", s.authz(http.HandlerFunc(s.handleArchiveProject), "admin", "delivery_manager"))
	s.mux.Handle("GET /api/v1/projects/{id}/overview", s.auth(http.HandlerFunc(s.handleProjectOverview)))

	s.mux.Handle("GET /api/v1/projects/{projectId}/upgrades", s.auth(http.HandlerFunc(s.handleListUpgrades)))
	s.mux.Handle("POST /api/v1/projects/{projectId}/upgrades", s.authz(http.HandlerFunc(s.handleCreateUpgrade), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("GET /api/v1/upgrades/{id}", s.auth(http.HandlerFunc(s.handleGetUpgrade)))
	s.mux.Handle("PUT /api/v1/upgrades/{id}", s.authz(http.HandlerFunc(s.handleUpdateUpgrade), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("DELETE /api/v1/upgrades/{id}", s.authz(http.HandlerFunc(s.handleDeleteUpgrade), "admin", "delivery_manager"))

	s.mux.Handle("GET /api/v1/projects/{projectId}/config-changes", s.auth(http.HandlerFunc(s.handleListConfigChanges)))
	s.mux.Handle("POST /api/v1/projects/{projectId}/config-changes", s.authz(http.HandlerFunc(s.handleCreateConfigChange), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("GET /api/v1/config-changes/{id}", s.auth(http.HandlerFunc(s.handleGetConfigChange)))
	s.mux.Handle("PUT /api/v1/config-changes/{id}", s.authz(http.HandlerFunc(s.handleUpdateConfigChange), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("DELETE /api/v1/config-changes/{id}", s.authz(http.HandlerFunc(s.handleDeleteConfigChange), "admin", "delivery_manager"))

	s.mux.Handle("GET /api/v1/projects/{projectId}/sql-changes", s.auth(http.HandlerFunc(s.handleListSQLChanges)))
	s.mux.Handle("POST /api/v1/projects/{projectId}/sql-changes", s.authz(http.HandlerFunc(s.handleCreateSQLChange), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("GET /api/v1/sql-changes/{id}", s.auth(http.HandlerFunc(s.handleGetSQLChange)))
	s.mux.Handle("PUT /api/v1/sql-changes/{id}", s.authz(http.HandlerFunc(s.handleUpdateSQLChange), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("DELETE /api/v1/sql-changes/{id}", s.authz(http.HandlerFunc(s.handleDeleteSQLChange), "admin", "delivery_manager"))

	s.mux.Handle("GET /api/v1/projects/{projectId}/integrations", s.auth(http.HandlerFunc(s.handleListIntegrations)))
	s.mux.Handle("POST /api/v1/projects/{projectId}/integrations", s.authz(http.HandlerFunc(s.handleCreateIntegration), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("GET /api/v1/integrations/{id}", s.auth(http.HandlerFunc(s.handleGetIntegration)))
	s.mux.Handle("PUT /api/v1/integrations/{id}", s.authz(http.HandlerFunc(s.handleUpdateIntegration), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("DELETE /api/v1/integrations/{id}", s.authz(http.HandlerFunc(s.handleDeleteIntegration), "admin", "delivery_manager"))

	s.mux.Handle("GET /api/v1/projects/{projectId}/assets", s.auth(http.HandlerFunc(s.handleListAssets)))
	s.mux.Handle("POST /api/v1/projects/{projectId}/assets", s.authz(http.HandlerFunc(s.handleCreateAsset), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("GET /api/v1/assets/{id}", s.auth(http.HandlerFunc(s.handleGetAsset)))
	s.mux.Handle("PUT /api/v1/assets/{id}", s.authz(http.HandlerFunc(s.handleUpdateAsset), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("DELETE /api/v1/assets/{id}", s.authz(http.HandlerFunc(s.handleDeleteAsset), "admin", "delivery_manager"))

	s.mux.Handle("GET /api/v1/projects/{projectId}/service-records", s.auth(http.HandlerFunc(s.handleListServices)))
	s.mux.Handle("POST /api/v1/projects/{projectId}/service-records", s.authz(http.HandlerFunc(s.handleCreateService), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("GET /api/v1/service-records/{id}", s.auth(http.HandlerFunc(s.handleGetService)))
	s.mux.Handle("PUT /api/v1/service-records/{id}", s.authz(http.HandlerFunc(s.handleUpdateService), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("DELETE /api/v1/service-records/{id}", s.authz(http.HandlerFunc(s.handleDeleteService), "admin", "delivery_manager"))

	s.mux.Handle("GET /api/v1/projects/{projectId}/attachments", s.auth(http.HandlerFunc(s.handleListAttachments)))
	s.mux.Handle("POST /api/v1/projects/{projectId}/attachments", s.authz(http.HandlerFunc(s.handleCreateAttachment), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("GET /api/v1/attachments/{id}", s.auth(http.HandlerFunc(s.handleGetAttachment)))
	s.mux.Handle("PUT /api/v1/attachments/{id}", s.authz(http.HandlerFunc(s.handleUpdateAttachment), "admin", "delivery_manager", "engineer", "rd_support"))
	s.mux.Handle("DELETE /api/v1/attachments/{id}", s.authz(http.HandlerFunc(s.handleDeleteAttachment), "admin", "delivery_manager"))
	s.mux.Handle("GET /api/v1/attachments/{id}/preview", s.auth(http.HandlerFunc(s.handlePreviewAttachment)))
	s.mux.Handle("GET /api/v1/attachments/{id}/download", s.auth(http.HandlerFunc(s.handleDownloadAttachment)))

	s.mux.Handle("GET /api/v1/projects/{projectId}/audit-logs", s.authz(http.HandlerFunc(s.handleListProjectAuditLogs), "admin", "delivery_manager"))
	s.mux.Handle("GET /api/v1/audit-logs", s.authz(http.HandlerFunc(s.handleListAuditLogs), "admin", "delivery_manager"))

	if webDir != "" {
		if _, err := os.Stat(webDir); err == nil {
			s.mux.HandleFunc("GET /login", s.handleLoginPage)
			fileServer := http.FileServer(http.Dir(webDir))
			s.mux.Handle("/", fileServer)
		}
	}
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if s.webDir == "" {
		http.NotFound(w, r)
		return
	}
	indexPath := filepath.Join(s.webDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, indexPath)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"message": "success",
		"data": map[string]any{
			"status": "ok",
		},
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	accessToken, refreshToken, user, err := s.svc.Login(r.Context(), req.Username, req.Password, clientIP(r), r.UserAgent())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"message": "success",
		"data": map[string]any{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"expires_in":    int(s.svcTokenTTL().Seconds()),
			"user":          user,
		},
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	accessToken, err := s.svc.RefreshAccessToken(r.Context(), req.RefreshToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"code": 1, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"message": "success",
		"data": map[string]any{
			"access_token": accessToken,
		},
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	user, err := s.svc.GetUser(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, user)
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if req.NewPassword == "" {
		writeError(w, errors.New("新密码不能为空"))
		return
	}
	if err := s.svc.ResetPassword(r.Context(), claims.UserID, claims.UserID, req.NewPassword); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"changed": true})
}

func (s *Server) handleDashboardOverview(w http.ResponseWriter, r *http.Request) {
	data, err := s.svc.DashboardOverview(r.Context(), queryString(r, "month"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, data)
}

func (s *Server) handleListIssues(w http.ResponseWriter, r *http.Request) {
	data, err := s.svc.ListIssueRecords(r.Context(), issueFilterFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, data)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	items, err := s.svc.ListUsers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, items)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	var req struct {
		Username string   `json:"username"`
		RealName string   `json:"real_name"`
		Password string   `json:"password"`
		Roles    []string `json:"roles"`
		Status   bool     `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	user := &model.User{
		Username: req.Username,
		RealName: req.RealName,
		Roles:    req.Roles,
		Status:   req.Status,
	}
	if err := s.svc.CreateUser(r.Context(), claims.UserID, user, req.Password); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, user)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Username string   `json:"username"`
		Password string   `json:"password"`
		RealName string   `json:"real_name"`
		Roles    []string `json:"roles"`
		Status   *bool    `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	user, err := s.svc.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	if strings.TrimSpace(req.RealName) != "" {
		user.RealName = strings.TrimSpace(req.RealName)
	}
	if req.Roles != nil {
		user.Roles = req.Roles
	}
	if req.Status != nil {
		user.Status = *req.Status
	}
	if err := s.svc.UpdateUser(r.Context(), claims.UserID, user); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, user)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteUser(r.Context(), claims.UserID, id); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"deleted": true})
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.ResetPassword(r.Context(), claims.UserID, id, req.NewPassword); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"reset": true})
}

func (s *Server) handleListLoginLogs(w http.ResponseWriter, r *http.Request) {
	items, err := s.svc.ListLoginLogs(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, items)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	filter := model.ProjectFilter{
		Keyword:        queryString(r, "keyword"),
		CustomerName:   queryString(r, "customer_name"),
		ProjectStatus:  queryString(r, "project_status"),
		CurrentVersion: queryString(r, "current_version"),
		Page:           queryInt(r, "page", 1),
		PageSize:       queryInt(r, "page_size", 20),
	}
	data, err := s.svc.ListProjects(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, data)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.svc.GetProject(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	var item model.Project
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	if item.ProjectName == "" || item.CustomerName == "" || item.ImplementationDate == "" || item.CurrentVersion == "" {
		writeError(w, errors.New("项目名称、客户名称、实施日期、当前版本不能为空"))
		return
	}
	if err := s.svc.CreateProject(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var item model.Project
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	item.ID = id
	if err := s.svc.UpdateProject(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleArchiveProject(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.ArchiveProject(r.Context(), claims.UserID, id); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"archived": true})
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteProject(r.Context(), claims.UserID, id); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"deleted": true})
}

func (s *Server) handleProjectOverview(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.svc.ProjectOverview(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleListUpgrades(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := s.svc.ListUpgrades(r.Context(), projectID, listFilterFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, data)
}

func (s *Server) handleGetUpgrade(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.svc.GetUpgrade(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleCreateUpgrade(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	var item model.UpgradeRecord
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	item.ProjectID = projectID
	if err := s.svc.CreateUpgrade(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleUpdateUpgrade(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	old, err := s.svc.GetUpgrade(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	var item model.UpgradeRecord
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	item.ID = id
	item.ProjectID = old.ProjectID
	item.UpgradeNo = old.UpgradeNo
	if err := s.svc.UpdateUpgrade(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleDeleteUpgrade(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteUpgrade(r.Context(), claims.UserID, id); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"deleted": true})
}

func (s *Server) handleListConfigChanges(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := s.svc.ListConfigChanges(r.Context(), projectID, listFilterFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, data)
}

func (s *Server) handleGetConfigChange(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.svc.GetConfigChange(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleCreateConfigChange(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	var item model.ConfigChange
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	item.ProjectID = projectID
	if err := s.svc.CreateConfigChange(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleUpdateConfigChange(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	old, err := s.svc.GetConfigChange(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	var item model.ConfigChange
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	item.ID = id
	item.ProjectID = old.ProjectID
	item.ConfigNo = old.ConfigNo
	if err := s.svc.UpdateConfigChange(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleDeleteConfigChange(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteConfigChange(r.Context(), claims.UserID, id); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"deleted": true})
}

func (s *Server) handleListSQLChanges(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := s.svc.ListSQLChanges(r.Context(), projectID, listFilterFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, data)
}

func (s *Server) handleGetSQLChange(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.svc.GetSQLChange(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleCreateSQLChange(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	var item model.SQLChange
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	item.ProjectID = projectID
	if err := s.svc.CreateSQLChange(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleUpdateSQLChange(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	old, err := s.svc.GetSQLChange(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	var item model.SQLChange
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	item.ID = id
	item.ProjectID = old.ProjectID
	item.SQLNo = old.SQLNo
	if err := s.svc.UpdateSQLChange(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleDeleteSQLChange(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteSQLChange(r.Context(), claims.UserID, id); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"deleted": true})
}

func (s *Server) handleListIntegrations(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := s.svc.ListIntegrations(r.Context(), projectID, listFilterFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, data)
}

func (s *Server) handleGetIntegration(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.svc.GetIntegration(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleCreateIntegration(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	var item model.IntegrationRecord
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	item.ProjectID = projectID
	if err := s.svc.CreateIntegration(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleUpdateIntegration(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	old, err := s.svc.GetIntegration(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	var item model.IntegrationRecord
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	item.ID = id
	item.ProjectID = old.ProjectID
	item.IntegrationNo = old.IntegrationNo
	if err := s.svc.UpdateIntegration(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleDeleteIntegration(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteIntegration(r.Context(), claims.UserID, id); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"deleted": true})
}

func (s *Server) handleListAssets(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := s.svc.ListAssets(r.Context(), projectID, listFilterFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, data)
}

func (s *Server) handleGetAsset(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.svc.GetAsset(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleCreateAsset(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	var item model.ScriptAsset
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	item.ProjectID = projectID
	if err := s.svc.CreateAsset(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleUpdateAsset(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	old, err := s.svc.GetAsset(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	var item model.ScriptAsset
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	item.ID = id
	item.ProjectID = old.ProjectID
	item.AssetNo = old.AssetNo
	if err := s.svc.UpdateAsset(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteAsset(r.Context(), claims.UserID, id); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"deleted": true})
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := s.svc.ListServiceRecords(r.Context(), projectID, listFilterFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, data)
}

func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.svc.GetServiceRecord(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleCreateService(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	var item model.ServiceRecord
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	item.ProjectID = projectID
	if err := s.svc.CreateServiceRecord(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleUpdateService(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	old, err := s.svc.GetServiceRecord(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	var item model.ServiceRecord
	if err := decodeJSON(r, &item); err != nil {
		writeError(w, err)
		return
	}
	item.ID = id
	item.ProjectID = old.ProjectID
	item.ServiceNo = old.ServiceNo
	if err := s.svc.UpdateServiceRecord(r.Context(), claims.UserID, &item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleDeleteService(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteServiceRecord(r.Context(), claims.UserID, id); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"deleted": true})
}

func (s *Server) handleListAttachments(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	filter := model.AttachmentFilter{
		RefType:     queryString(r, "ref_type"),
		RefID:       queryInt64(r, "ref_id", 0),
		DocCategory: queryString(r, "doc_category"),
		Page:        queryInt(r, "page", 1),
		PageSize:    queryInt(r, "page_size", 20),
	}
	data, err := s.svc.ListAttachments(r.Context(), projectID, filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, data)
}

func (s *Server) handleGetAttachment(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.svc.GetAttachment(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleCreateAttachment(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	if maxUploadSizeBytes := s.svc.MaxUploadSizeBytes(); maxUploadSizeBytes > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSizeBytes)
	}
	if err := r.ParseMultipartForm(attachmentMultipartMemory); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
				"code":    1,
				"message": "request body too large",
			})
			return
		}
		writeError(w, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, err)
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(header.Filename))
	}

	item := &model.Attachment{
		ProjectID:   projectID,
		RefType:     r.FormValue("ref_type"),
		RefID:       queryFormInt64(r, "ref_id"),
		Title:       r.FormValue("title"),
		DocCategory: r.FormValue("doc_category"),
		Tags:        r.FormValue("tags"),
		Description: r.FormValue("description"),
		UploadedBy:  claims.UserID,
		UploadedAt:  time.Now(),
	}
	if item.RefType == "" {
		item.RefType = "project"
	}
	if item.Title == "" || item.DocCategory == "" {
		writeError(w, errors.New("附件标题和分类不能为空"))
		return
	}
	if err := s.svc.CreateAttachment(r.Context(), claims.UserID, item, file, header, mimeType); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleUpdateAttachment(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.svc.GetAttachment(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Title       string `json:"title"`
		DocCategory string `json:"doc_category"`
		Tags        string `json:"tags"`
		Description string `json:"description"`
		RefType     string `json:"ref_type"`
		RefID       int64  `json:"ref_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	item.Title = req.Title
	item.DocCategory = req.DocCategory
	item.Tags = req.Tags
	item.Description = req.Description
	item.RefType = req.RefType
	item.RefID = req.RefID
	if err := s.svc.UpdateAttachment(r.Context(), claims.UserID, item); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, item)
}

func (s *Server) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteAttachment(r.Context(), claims.UserID, id); err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"deleted": true})
}

func (s *Server) handlePreviewAttachment(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.svc.GetAttachment(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	path, err := s.svc.AttachmentAbsolutePath(item.RelativePath)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", fallbackContentType(item.MimeType, item.FileExt))
	http.ServeFile(w, r, path)
}

func (s *Server) handleDownloadAttachment(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.svc.GetAttachment(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	path, err := s.svc.AttachmentAbsolutePath(item.RelativePath)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", item.OriginalName))
	http.ServeFile(w, r, path)
}

func (s *Server) handleListProjectAuditLogs(w http.ResponseWriter, r *http.Request) {
	projectID, err := pathInt64(r, "projectId")
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := s.svc.ListAuditLogs(r.Context(), projectID, listFilterFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, data)
}

func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	data, err := s.svc.ListAuditLogs(r.Context(), 0, listFilterFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, data)
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			var err error
			token, err = security.BearerToken(authHeader)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"code": 1, "message": "未登录"})
				return
			}
		} else {
			token = strings.TrimSpace(r.URL.Query().Get("access_token"))
			if token == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"code": 1, "message": "未登录"})
				return
			}
		}
		claims, err := s.svc.ParseAccessToken(token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"code": 1, "message": "登录已过期"})
			return
		}
		ctx := context.WithValue(r.Context(), claimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) authz(next http.Handler, roles ...string) http.Handler {
	return s.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	}))
}

func hasRole(userRoles []string, expected ...string) bool {
	for _, role := range userRoles {
		if role == "admin" {
			return true
		}
		for _, target := range expected {
			if role == target {
				return true
			}
		}
	}
	return false
}

func claimsFromContext(ctx context.Context) security.TokenClaims {
	value, _ := ctx.Value(claimsContextKey).(security.TokenClaims)
	return value
}

func decodeJSON(r *http.Request, dest any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dest)
}

func writeSuccess(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	message := err.Error()
	if errors.Is(err, store.ErrNotFound) {
		status = http.StatusNotFound
		message = "记录不存在"
	}
	writeJSON(w, status, map[string]any{
		"code":    1,
		"message": message,
	})
}

func pathInt64(r *http.Request, name string) (int64, error) {
	value := r.PathValue(name)
	if value == "" {
		return 0, errors.New("缺少路径参数")
	}
	return strconv.ParseInt(value, 10, 64)
}

func queryInt(r *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func queryInt64(r *http.Request, key string, fallback int64) int64 {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func queryFormInt64(r *http.Request, key string) int64 {
	value := strings.TrimSpace(r.FormValue(key))
	if value == "" {
		return 0
	}
	n, _ := strconv.ParseInt(value, 10, 64)
	return n
}

func queryString(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

func listFilterFromQuery(r *http.Request) model.ListFilter {
	return model.ListFilter{
		Keyword:   queryString(r, "keyword"),
		Page:      queryInt(r, "page", 1),
		PageSize:  queryInt(r, "page_size", 20),
		SortBy:    queryString(r, "sort_by"),
		SortOrder: queryString(r, "sort_order"),
	}
}

func issueFilterFromQuery(r *http.Request) model.IssueFilter {
	return model.IssueFilter{
		Keyword:      queryString(r, "keyword"),
		IssueVersion: queryString(r, "issue_version"),
		Page:         queryInt(r, "page", 1),
		PageSize:     queryInt(r, "page_size", 100),
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func fallbackContentType(mimeType, ext string) string {
	if mimeType != "" {
		return mimeType
	}
	if ext != "" {
		if c := mime.TypeByExtension("." + ext); c != "" {
			return c
		}
	}
	return "application/octet-stream"
}

func (s *Server) svcTokenTTL() time.Duration {
	if s == nil || s.svc == nil {
		return 0
	}
	return s.svc.AccessTokenTTL()
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
