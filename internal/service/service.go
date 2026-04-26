package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"customerdeliverylog/internal/config"
	"customerdeliverylog/internal/model"
	"customerdeliverylog/internal/pkg/security"
	"customerdeliverylog/internal/pkg/storage"
	"customerdeliverylog/internal/store"
)

type Service struct {
	cfg      config.Config
	store    store.Store
	storage  storage.LocalStorage
	sequence atomic.Uint64
}

func New(cfg config.Config, st store.Store, localStorage storage.LocalStorage) *Service {
	return &Service{
		cfg:     cfg,
		store:   st,
		storage: localStorage,
	}
}

func (s *Service) MaxUploadSizeBytes() int64 {
	return s.cfg.MaxUploadSizeBytes
}

func (s *Service) AccessTokenTTL() time.Duration {
	return s.cfg.AccessTokenTTL
}

func (s *Service) Bootstrap(ctx context.Context) error {
	passwordHash, err := security.HashPassword(s.cfg.SeedAdminPassword)
	if err != nil {
		return err
	}
	return s.store.SeedAdmin(ctx, s.cfg.SeedAdminUsername, s.cfg.SeedAdminRealName, passwordHash)
}

func (s *Service) Login(ctx context.Context, username, password, ip, userAgent string) (string, string, *model.User, error) {
	user, err := s.store.FindUserByUsername(ctx, username)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return "", "", nil, err
		}
		_ = s.store.CreateLoginLog(ctx, &model.LoginLog{
			Username:     username,
			LoginResult:  "fail",
			LoginMessage: "用户不存在",
			LoginIP:      ip,
			UserAgent:    userAgent,
			LoginAt:      time.Now(),
		})
		return "", "", nil, errors.New("账号或密码错误")
	}
	if !user.Status {
		return "", "", nil, errors.New("账号已停用，请联系管理员")
	}
	if !security.VerifyPassword(user.PasswordHash, password) {
		_ = s.store.CreateLoginLog(ctx, &model.LoginLog{
			UserID:       user.ID,
			Username:     username,
			LoginResult:  "fail",
			LoginMessage: "密码错误",
			LoginIP:      ip,
			UserAgent:    userAgent,
			LoginAt:      time.Now(),
		})
		return "", "", nil, errors.New("账号或密码错误")
	}
	accessToken, refreshToken, err := security.IssueTokenPair(
		s.cfg.TokenSecret,
		user.ID,
		user.Username,
		user.Roles,
		s.cfg.AccessTokenTTL,
		s.cfg.RefreshTokenTTL,
	)
	if err != nil {
		return "", "", nil, err
	}
	_ = s.store.CreateLoginLog(ctx, &model.LoginLog{
		UserID:       user.ID,
		Username:     user.Username,
		LoginResult:  "success",
		LoginMessage: "登录成功",
		LoginIP:      ip,
		UserAgent:    userAgent,
		LoginAt:      time.Now(),
	})
	return accessToken, refreshToken, user, nil
}

func (s *Service) RefreshAccessToken(ctx context.Context, refreshToken string) (string, error) {
	claims, err := security.ParseToken(s.cfg.TokenSecret, refreshToken)
	if err != nil {
		return "", err
	}
	if claims.Kind != "refresh" {
		return "", errors.New("invalid refresh token")
	}
	user, err := s.store.GetUser(ctx, claims.UserID)
	if err != nil {
		return "", err
	}
	accessToken, _, err := security.IssueTokenPair(
		s.cfg.TokenSecret,
		user.ID,
		user.Username,
		user.Roles,
		s.cfg.AccessTokenTTL,
		s.cfg.RefreshTokenTTL,
	)
	return accessToken, err
}

func (s *Service) ParseAccessToken(token string) (security.TokenClaims, error) {
	claims, err := security.ParseToken(s.cfg.TokenSecret, token)
	if err != nil {
		return security.TokenClaims{}, err
	}
	if claims.Kind != "access" {
		return security.TokenClaims{}, errors.New("invalid access token")
	}
	return claims, nil
}

func (s *Service) GetUser(ctx context.Context, id int64) (*model.User, error) {
	return s.store.GetUser(ctx, id)
}

func (s *Service) ListUsers(ctx context.Context) ([]model.User, error) {
	return s.store.ListUsers(ctx)
}

func (s *Service) CreateUser(ctx context.Context, currentUserID int64, user *model.User, rawPassword string) error {
	passwordHash, err := security.HashPassword(rawPassword)
	if err != nil {
		return err
	}
	user.PasswordHash = passwordHash
	if err := s.store.CreateUser(ctx, user); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "user", user.ID, "create", "新增用户 "+user.Username, nil, user)
}

func (s *Service) UpdateUser(ctx context.Context, currentUserID int64, user *model.User) error {
	before, err := s.store.GetUser(ctx, user.ID)
	if err != nil {
		return err
	}
	if err := s.store.UpdateUser(ctx, user); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "user", user.ID, "update", "编辑用户 "+user.Username, before, user)
}

func (s *Service) ResetPassword(ctx context.Context, currentUserID, userID int64, rawPassword string) error {
	passwordHash, err := security.HashPassword(rawPassword)
	if err != nil {
		return err
	}
	if err := s.store.UpdateUserPassword(ctx, userID, passwordHash); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "user", userID, "update", "重置用户密码", nil, nil)
}

func (s *Service) DeleteUser(ctx context.Context, currentUserID, userID int64) error {
	before, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if s.isProtectedUser(before) {
		return errors.New("admin 管理员账号不允许删除")
	}
	if err := s.store.DeleteUser(ctx, userID); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "user", userID, "delete", "删除用户", nil, nil)
}

func (s *Service) DashboardOverview(ctx context.Context, month string) (model.DashboardOverview, error) {
	return s.store.DashboardOverview(ctx, month)
}

func (s *Service) ListProjects(ctx context.Context, filter model.ProjectFilter) (model.PagedResult[model.Project], error) {
	return s.store.ListProjects(ctx, filter)
}

func (s *Service) GetProject(ctx context.Context, projectID int64) (*model.Project, error) {
	return s.store.GetProject(ctx, projectID)
}

func (s *Service) CreateProject(ctx context.Context, currentUserID int64, project *model.Project) error {
	project.ProjectCode = s.generateCode("PRJ")
	if project.ProjectStatus == "" {
		project.ProjectStatus = "implementing"
	}
	if project.OwnerUserID == 0 {
		project.OwnerUserID = currentUserID
	}
	if err := s.store.CreateProject(ctx, project); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "project", project.ID, "create", "新增项目 "+project.ProjectName, nil, project)
}

func (s *Service) UpdateProject(ctx context.Context, currentUserID int64, project *model.Project) error {
	before, err := s.store.GetProject(ctx, project.ID)
	if err != nil {
		return err
	}
	if err := s.store.UpdateProject(ctx, project); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "project", project.ID, "update", "编辑项目 "+project.ProjectName, before, project)
}

func (s *Service) ArchiveProject(ctx context.Context, currentUserID, projectID int64) error {
	if err := s.store.ArchiveProject(ctx, projectID, currentUserID); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "project", projectID, "update", "归档项目", nil, nil)
}

func (s *Service) DeleteProject(ctx context.Context, currentUserID, projectID int64) error {
	before, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	attachments, err := s.store.ListAttachments(ctx, projectID, model.AttachmentFilter{Page: 1, PageSize: 10000})
	if err != nil {
		return err
	}
	if err := s.store.DeleteProject(ctx, projectID); err != nil {
		return err
	}
	for _, item := range attachments.List {
		_ = s.storage.Delete(item.RelativePath)
	}
	return s.audit(ctx, currentUserID, "project", projectID, "delete", "删除项目 "+before.ProjectName, before, nil)
}

func (s *Service) ProjectOverview(ctx context.Context, projectID int64) (model.ProjectOverview, error) {
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return model.ProjectOverview{}, err
	}
	upgrades, err := s.store.ListUpgrades(ctx, projectID, model.ListFilter{Page: 1, PageSize: 5})
	if err != nil {
		return model.ProjectOverview{}, err
	}
	configs, err := s.store.ListConfigChanges(ctx, projectID, model.ListFilter{Page: 1, PageSize: 5})
	if err != nil {
		return model.ProjectOverview{}, err
	}
	sqls, err := s.store.ListSQLChanges(ctx, projectID, model.ListFilter{Page: 1, PageSize: 5})
	if err != nil {
		return model.ProjectOverview{}, err
	}
	assets, err := s.store.ListAssets(ctx, projectID, model.ListFilter{Page: 1, PageSize: 5})
	if err != nil {
		return model.ProjectOverview{}, err
	}
	services, err := s.store.ListServiceRecords(ctx, projectID, model.ListFilter{Page: 1, PageSize: 5})
	if err != nil {
		return model.ProjectOverview{}, err
	}
	attachments, err := s.store.ListAttachments(ctx, projectID, model.AttachmentFilter{Page: 1, PageSize: 8})
	if err != nil {
		return model.ProjectOverview{}, err
	}
	integrations, err := s.store.ListIntegrations(ctx, projectID, model.ListFilter{Page: 1, PageSize: 10})
	if err != nil {
		return model.ProjectOverview{}, err
	}

	return model.ProjectOverview{
		Project:             *project,
		RecentUpgrades:      upgrades.List,
		RecentConfigChanges: configs.List,
		RecentSQLChanges:    sqls.List,
		RecentAssets:        assets.List,
		RecentServices:      services.List,
		RecentAttachments:   attachments.List,
		Integrations:        integrations.List,
	}, nil
}

func (s *Service) ListUpgrades(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.UpgradeRecord], error) {
	return s.store.ListUpgrades(ctx, projectID, filter)
}

func (s *Service) GetUpgrade(ctx context.Context, id int64) (*model.UpgradeRecord, error) {
	return s.store.GetUpgrade(ctx, id)
}

func (s *Service) CreateUpgrade(ctx context.Context, currentUserID int64, item *model.UpgradeRecord) error {
	item.UpgradeNo = s.generateCode("UPG")
	if item.OwnerUserID == 0 {
		item.OwnerUserID = currentUserID
	}
	if err := s.store.CreateUpgrade(ctx, item); err != nil {
		return err
	}
	if err := s.syncProjectVersion(ctx, item.ProjectID); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "upgrade", item.ID, "create", "新增升级记录 "+item.UpgradeNo, nil, item)
}

func (s *Service) UpdateUpgrade(ctx context.Context, currentUserID int64, item *model.UpgradeRecord) error {
	before, err := s.store.GetUpgrade(ctx, item.ID)
	if err != nil {
		return err
	}
	if err := s.store.UpdateUpgrade(ctx, item); err != nil {
		return err
	}
	if err := s.syncProjectVersion(ctx, item.ProjectID); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "upgrade", item.ID, "update", "编辑升级记录 "+item.UpgradeNo, before, item)
}

func (s *Service) DeleteUpgrade(ctx context.Context, currentUserID, id int64) error {
	before, err := s.store.GetUpgrade(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteUpgrade(ctx, id); err != nil {
		return err
	}
	if err := s.syncProjectVersion(ctx, before.ProjectID); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "upgrade", id, "delete", "删除升级记录 "+before.UpgradeNo, before, nil)
}

func (s *Service) ListConfigChanges(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.ConfigChange], error) {
	return s.store.ListConfigChanges(ctx, projectID, filter)
}

func (s *Service) GetConfigChange(ctx context.Context, id int64) (*model.ConfigChange, error) {
	return s.store.GetConfigChange(ctx, id)
}

func (s *Service) CreateConfigChange(ctx context.Context, currentUserID int64, item *model.ConfigChange) error {
	item.ConfigNo = s.generateCode("CFG")
	item.RelatedUpgradeID = 0
	if item.ChangedBy == 0 {
		item.ChangedBy = currentUserID
	}
	if item.ChangedAt.IsZero() {
		item.ChangedAt = time.Now()
	}
	if err := s.store.CreateConfigChange(ctx, item); err != nil {
		return err
	}
	if err := s.touchProjectChange(ctx, item.ProjectID, item.ChangedAt); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "config_change", item.ID, "create", "新增配置变更 "+item.ConfigNo, nil, item)
}

func (s *Service) UpdateConfigChange(ctx context.Context, currentUserID int64, item *model.ConfigChange) error {
	before, err := s.store.GetConfigChange(ctx, item.ID)
	if err != nil {
		return err
	}
	item.RelatedUpgradeID = 0
	if err := s.store.UpdateConfigChange(ctx, item); err != nil {
		return err
	}
	if err := s.touchProjectChange(ctx, item.ProjectID, item.ChangedAt); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "config_change", item.ID, "update", "编辑配置变更 "+item.ConfigNo, before, item)
}

func (s *Service) DeleteConfigChange(ctx context.Context, currentUserID, id int64) error {
	before, err := s.store.GetConfigChange(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteConfigChange(ctx, id); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "config_change", id, "delete", "删除配置变更 "+before.ConfigNo, before, nil)
}

func (s *Service) ListSQLChanges(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.SQLChange], error) {
	return s.store.ListSQLChanges(ctx, projectID, filter)
}

func (s *Service) GetSQLChange(ctx context.Context, id int64) (*model.SQLChange, error) {
	return s.store.GetSQLChange(ctx, id)
}

func (s *Service) CreateSQLChange(ctx context.Context, currentUserID int64, item *model.SQLChange) error {
	item.SQLNo = s.generateCode("SQL")
	item.RelatedUpgradeID = 0
	if item.ChangedBy == 0 {
		item.ChangedBy = currentUserID
	}
	if item.ChangedAt.IsZero() {
		item.ChangedAt = time.Now()
	}
	if err := s.store.CreateSQLChange(ctx, item); err != nil {
		return err
	}
	if err := s.touchProjectChange(ctx, item.ProjectID, item.ChangedAt); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "sql_change", item.ID, "create", "新增SQL变更 "+item.SQLNo, nil, item)
}

func (s *Service) UpdateSQLChange(ctx context.Context, currentUserID int64, item *model.SQLChange) error {
	before, err := s.store.GetSQLChange(ctx, item.ID)
	if err != nil {
		return err
	}
	item.RelatedUpgradeID = 0
	if err := s.store.UpdateSQLChange(ctx, item); err != nil {
		return err
	}
	if err := s.touchProjectChange(ctx, item.ProjectID, item.ChangedAt); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "sql_change", item.ID, "update", "编辑SQL变更 "+item.SQLNo, before, item)
}

func (s *Service) DeleteSQLChange(ctx context.Context, currentUserID, id int64) error {
	before, err := s.store.GetSQLChange(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteSQLChange(ctx, id); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "sql_change", id, "delete", "删除SQL变更 "+before.SQLNo, before, nil)
}

func (s *Service) ListIntegrations(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.IntegrationRecord], error) {
	return s.store.ListIntegrations(ctx, projectID, filter)
}

func (s *Service) GetIntegration(ctx context.Context, id int64) (*model.IntegrationRecord, error) {
	return s.store.GetIntegration(ctx, id)
}

func (s *Service) CreateIntegration(ctx context.Context, currentUserID int64, item *model.IntegrationRecord) error {
	item.IntegrationNo = s.generateCode("INT")
	if item.InternalOwnerUserID == 0 {
		item.InternalOwnerUserID = currentUserID
	}
	if err := s.store.CreateIntegration(ctx, item); err != nil {
		return err
	}
	if err := s.touchProjectChange(ctx, item.ProjectID, time.Now()); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "integration", item.ID, "create", "新增外部对接 "+item.IntegrationNo, nil, item)
}

func (s *Service) UpdateIntegration(ctx context.Context, currentUserID int64, item *model.IntegrationRecord) error {
	before, err := s.store.GetIntegration(ctx, item.ID)
	if err != nil {
		return err
	}
	if err := s.store.UpdateIntegration(ctx, item); err != nil {
		return err
	}
	if err := s.touchProjectChange(ctx, item.ProjectID, time.Now()); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "integration", item.ID, "update", "编辑外部对接 "+item.IntegrationNo, before, item)
}

func (s *Service) DeleteIntegration(ctx context.Context, currentUserID, id int64) error {
	before, err := s.store.GetIntegration(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteIntegration(ctx, id); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "integration", id, "delete", "删除外部对接 "+before.IntegrationNo, before, nil)
}

func (s *Service) ListAssets(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.ScriptAsset], error) {
	return s.store.ListAssets(ctx, projectID, filter)
}

func (s *Service) GetAsset(ctx context.Context, id int64) (*model.ScriptAsset, error) {
	return s.store.GetAsset(ctx, id)
}

func (s *Service) CreateAsset(ctx context.Context, currentUserID int64, item *model.ScriptAsset) error {
	item.AssetNo = s.generateCode("AST")
	item.RelatedUpgradeID = 0
	if item.ChangedBy == 0 {
		item.ChangedBy = currentUserID
	}
	if item.ChangedAt.IsZero() {
		item.ChangedAt = time.Now()
	}
	if err := s.store.CreateAsset(ctx, item); err != nil {
		return err
	}
	if err := s.touchProjectChange(ctx, item.ProjectID, item.ChangedAt); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "asset", item.ID, "create", "新增脚本资产 "+item.AssetNo, nil, item)
}

func (s *Service) UpdateAsset(ctx context.Context, currentUserID int64, item *model.ScriptAsset) error {
	before, err := s.store.GetAsset(ctx, item.ID)
	if err != nil {
		return err
	}
	item.RelatedUpgradeID = 0
	if err := s.store.UpdateAsset(ctx, item); err != nil {
		return err
	}
	if err := s.touchProjectChange(ctx, item.ProjectID, item.ChangedAt); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "asset", item.ID, "update", "编辑脚本资产 "+item.AssetNo, before, item)
}

func (s *Service) DeleteAsset(ctx context.Context, currentUserID, id int64) error {
	before, err := s.store.GetAsset(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteAsset(ctx, id); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "asset", id, "delete", "删除脚本资产 "+before.AssetNo, before, nil)
}

func (s *Service) ListServiceRecords(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.ServiceRecord], error) {
	return s.store.ListServiceRecords(ctx, projectID, filter)
}

func (s *Service) ListIssueRecords(ctx context.Context, filter model.IssueFilter) (model.PagedResult[model.ServiceRecord], error) {
	return s.store.ListIssueRecords(ctx, filter)
}

func (s *Service) GetServiceRecord(ctx context.Context, id int64) (*model.ServiceRecord, error) {
	return s.store.GetServiceRecord(ctx, id)
}

func (s *Service) CreateServiceRecord(ctx context.Context, currentUserID int64, item *model.ServiceRecord) error {
	item.ServiceNo = s.generateCode("SRV")
	item.RelatedUpgradeID = 0
	if item.OwnerUserID == 0 {
		item.OwnerUserID = currentUserID
	}
	if item.ServiceDate.IsZero() {
		item.ServiceDate = time.Now()
	}
	if item.ServiceType != "incident" {
		item.IssueVersion = ""
	}
	if err := s.store.CreateServiceRecord(ctx, item); err != nil {
		return err
	}
	if err := s.touchProjectChange(ctx, item.ProjectID, item.ServiceDate); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "service_record", item.ID, "create", "新增服务记录 "+item.ServiceNo, nil, item)
}

func (s *Service) UpdateServiceRecord(ctx context.Context, currentUserID int64, item *model.ServiceRecord) error {
	before, err := s.store.GetServiceRecord(ctx, item.ID)
	if err != nil {
		return err
	}
	item.RelatedUpgradeID = 0
	if item.ServiceType != "incident" {
		item.IssueVersion = ""
	}
	if err := s.store.UpdateServiceRecord(ctx, item); err != nil {
		return err
	}
	if err := s.touchProjectChange(ctx, item.ProjectID, item.ServiceDate); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "service_record", item.ID, "update", "编辑服务记录 "+item.ServiceNo, before, item)
}

func (s *Service) DeleteServiceRecord(ctx context.Context, currentUserID, id int64) error {
	before, err := s.store.GetServiceRecord(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteServiceRecord(ctx, id); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "service_record", id, "delete", "删除服务记录 "+before.ServiceNo, before, nil)
}

func (s *Service) ListAttachments(ctx context.Context, projectID int64, filter model.AttachmentFilter) (model.PagedResult[model.Attachment], error) {
	return s.store.ListAttachments(ctx, projectID, filter)
}

func (s *Service) GetAttachment(ctx context.Context, id int64) (*model.Attachment, error) {
	return s.store.GetAttachment(ctx, id)
}

func (s *Service) CreateAttachment(ctx context.Context, currentUserID int64, item *model.Attachment, file multipart.File, header *multipart.FileHeader, mimeType string) error {
	saved, err := s.storage.Save(file, header)
	if err != nil {
		return err
	}
	item.FileName = saved.FileName
	item.OriginalName = header.Filename
	item.FileExt = strings.TrimPrefix(filepath.Ext(header.Filename), ".")
	item.FileSize = saved.Size
	item.MimeType = mimeType
	item.StorageType = "local"
	item.RelativePath = saved.RelativePath
	item.ThumbnailPath = saved.ThumbnailPath
	if err := s.store.CreateAttachment(ctx, item); err != nil {
		_ = s.storage.Delete(saved.RelativePath)
		return err
	}
	if err := s.touchProjectChange(ctx, item.ProjectID, time.Now()); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "attachment", item.ID, "create", "上传附件 "+item.Title, nil, item)
}

func (s *Service) UpdateAttachment(ctx context.Context, currentUserID int64, item *model.Attachment) error {
	before, err := s.store.GetAttachment(ctx, item.ID)
	if err != nil {
		return err
	}
	if err := s.store.UpdateAttachment(ctx, item); err != nil {
		return err
	}
	return s.audit(ctx, currentUserID, "attachment", item.ID, "update", "编辑附件 "+item.Title, before, item)
}

func (s *Service) DeleteAttachment(ctx context.Context, currentUserID, id int64) error {
	before, err := s.store.GetAttachment(ctx, id)
	if err != nil {
		return err
	}
	if _, err := s.storage.AbsolutePath(before.RelativePath); err != nil {
		return err
	}
	if err := s.store.DeleteAttachment(ctx, id); err != nil {
		return err
	}
	_ = s.storage.Delete(before.RelativePath)
	return s.audit(ctx, currentUserID, "attachment", id, "delete", "删除附件 "+before.Title, before, nil)
}

func (s *Service) AttachmentAbsolutePath(relativePath string) (string, error) {
	return s.storage.AbsolutePath(relativePath)
}

func (s *Service) ListAuditLogs(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.AuditLog], error) {
	return s.store.ListAuditLogs(ctx, projectID, filter)
}

func (s *Service) ListLoginLogs(ctx context.Context) ([]model.LoginLog, error) {
	return s.store.ListLoginLogs(ctx)
}

func (s *Service) touchProjectChange(ctx context.Context, projectID int64, changedAt time.Time) error {
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	project.LastChangeAt = &changedAt
	return s.store.UpdateProject(ctx, project)
}

func (s *Service) syncProjectVersion(ctx context.Context, projectID int64) error {
	project, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	upgrades, err := s.store.ListUpgrades(ctx, projectID, model.ListFilter{Page: 1, PageSize: 1000})
	if err != nil {
		return err
	}
	var latest *model.UpgradeRecord
	for i := range upgrades.List {
		item := upgrades.List[i]
		if item.UpgradeStatus != "completed" {
			continue
		}
		if latest == nil || item.UpgradeDate.After(latest.UpgradeDate) {
			copy := item
			latest = &copy
		}
	}
	if latest != nil {
		project.CurrentVersion = latest.TargetVersion
		project.LastUpgradeAt = &latest.UpgradeDate
		project.LastChangeAt = &latest.UpgradeDate
	}
	return s.store.UpdateProject(ctx, project)
}

func (s *Service) generateCode(prefix string) string {
	now := time.Now()
	seq := (now.UnixNano() / 1000) % 1000000
	return fmt.Sprintf("%s-%s-%06d", prefix, now.Format("20060102150405"), seq)
}

func (s *Service) audit(ctx context.Context, operatorUserID int64, objectType string, objectID int64, operationType, summary string, before, after any) error {
	beforeJSON := toJSONString(before)
	afterJSON := toJSONString(after)
	projectID := extractProjectID(before, after)
	operatorName := ""
	if operatorUserID > 0 {
		if user, err := s.store.GetUser(ctx, operatorUserID); err == nil {
			operatorName = user.RealName
		}
	}
	return s.store.CreateAuditLog(ctx, &model.AuditLog{
		ProjectID:        projectID,
		ObjectType:       objectType,
		ObjectID:         objectID,
		OperationType:    operationType,
		OperationSummary: summary,
		BeforeSnapshot:   beforeJSON,
		AfterSnapshot:    afterJSON,
		OperatorUserID:   operatorUserID,
		OperatorUserName: operatorName,
		OperatedAt:       time.Now(),
	})
}

func toJSONString(v any) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

func extractProjectID(before, after any) int64 {
	for _, item := range []any{after, before} {
		switch value := item.(type) {
		case *model.Project:
			return value.ID
		case *model.UpgradeRecord:
			return value.ProjectID
		case *model.ConfigChange:
			return value.ProjectID
		case *model.SQLChange:
			return value.ProjectID
		case *model.IntegrationRecord:
			return value.ProjectID
		case *model.ScriptAsset:
			return value.ProjectID
		case *model.ServiceRecord:
			return value.ProjectID
		case *model.Attachment:
			return value.ProjectID
		}
	}
	return 0
}

func (s *Service) isProtectedUser(user *model.User) bool {
	if user == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(user.Username), strings.TrimSpace(s.cfg.SeedAdminUsername)) {
		return true
	}
	for _, role := range user.Roles {
		if strings.EqualFold(strings.TrimSpace(role), "admin") {
			return true
		}
	}
	return false
}
