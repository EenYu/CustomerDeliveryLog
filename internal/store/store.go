package store

import (
	"context"

	"customerdeliverylog/internal/model"
)

type Store interface {
	SeedAdmin(ctx context.Context, username, realName, passwordHash string) error

	FindUserByUsername(ctx context.Context, username string) (*model.User, error)
	GetUser(ctx context.Context, id int64) (*model.User, error)
	ListUsers(ctx context.Context) ([]model.User, error)
	CreateUser(ctx context.Context, user *model.User) error
	UpdateUser(ctx context.Context, user *model.User) error
	UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error
	DeleteUser(ctx context.Context, userID int64) error

	CreateLoginLog(ctx context.Context, log *model.LoginLog) error
	ListLoginLogs(ctx context.Context) ([]model.LoginLog, error)

	DashboardOverview(ctx context.Context, month string) (model.DashboardOverview, error)

	ListProjects(ctx context.Context, filter model.ProjectFilter) (model.PagedResult[model.Project], error)
	GetProject(ctx context.Context, projectID int64) (*model.Project, error)
	CreateProject(ctx context.Context, project *model.Project) error
	UpdateProject(ctx context.Context, project *model.Project) error
	ArchiveProject(ctx context.Context, projectID int64, archivedBy int64) error
	DeleteProject(ctx context.Context, projectID int64) error

	ListUpgrades(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.UpgradeRecord], error)
	GetUpgrade(ctx context.Context, id int64) (*model.UpgradeRecord, error)
	CreateUpgrade(ctx context.Context, item *model.UpgradeRecord) error
	UpdateUpgrade(ctx context.Context, item *model.UpgradeRecord) error
	DeleteUpgrade(ctx context.Context, id int64) error

	ListConfigChanges(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.ConfigChange], error)
	GetConfigChange(ctx context.Context, id int64) (*model.ConfigChange, error)
	CreateConfigChange(ctx context.Context, item *model.ConfigChange) error
	UpdateConfigChange(ctx context.Context, item *model.ConfigChange) error
	DeleteConfigChange(ctx context.Context, id int64) error

	ListSQLChanges(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.SQLChange], error)
	GetSQLChange(ctx context.Context, id int64) (*model.SQLChange, error)
	CreateSQLChange(ctx context.Context, item *model.SQLChange) error
	UpdateSQLChange(ctx context.Context, item *model.SQLChange) error
	DeleteSQLChange(ctx context.Context, id int64) error

	ListIntegrations(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.IntegrationRecord], error)
	GetIntegration(ctx context.Context, id int64) (*model.IntegrationRecord, error)
	CreateIntegration(ctx context.Context, item *model.IntegrationRecord) error
	UpdateIntegration(ctx context.Context, item *model.IntegrationRecord) error
	DeleteIntegration(ctx context.Context, id int64) error

	ListAssets(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.ScriptAsset], error)
	GetAsset(ctx context.Context, id int64) (*model.ScriptAsset, error)
	CreateAsset(ctx context.Context, item *model.ScriptAsset) error
	UpdateAsset(ctx context.Context, item *model.ScriptAsset) error
	DeleteAsset(ctx context.Context, id int64) error

	ListServiceRecords(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.ServiceRecord], error)
	ListIssueRecords(ctx context.Context, filter model.IssueFilter) (model.PagedResult[model.ServiceRecord], error)
	GetServiceRecord(ctx context.Context, id int64) (*model.ServiceRecord, error)
	CreateServiceRecord(ctx context.Context, item *model.ServiceRecord) error
	UpdateServiceRecord(ctx context.Context, item *model.ServiceRecord) error
	DeleteServiceRecord(ctx context.Context, id int64) error

	ListAttachments(ctx context.Context, projectID int64, filter model.AttachmentFilter) (model.PagedResult[model.Attachment], error)
	GetAttachment(ctx context.Context, id int64) (*model.Attachment, error)
	CreateAttachment(ctx context.Context, item *model.Attachment) error
	UpdateAttachment(ctx context.Context, item *model.Attachment) error
	DeleteAttachment(ctx context.Context, id int64) error

	CreateAuditLog(ctx context.Context, log *model.AuditLog) error
	ListAuditLogs(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.AuditLog], error)
}
