package model

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	RealName     string    `json:"real_name"`
	PasswordHash string    `json:"-"`
	Roles        []string  `json:"roles"`
	Status       bool      `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Project struct {
	ID                 int64      `json:"id"`
	ProjectCode        string     `json:"project_code"`
	ProjectName        string     `json:"project_name"`
	CustomerName       string     `json:"customer_name"`
	ProjectStatus      string     `json:"project_status"`
	ImplementationDate string     `json:"implementation_date"`
	OnlineDate         string     `json:"online_date"`
	AcceptanceDate     string     `json:"acceptance_date"`
	CurrentVersion     string     `json:"current_version"`
	OwnerUserID        int64      `json:"owner_user_id"`
	OwnerName          string     `json:"owner_name,omitempty"`
	DeployMode         string     `json:"deploy_mode"`
	EnvironmentSummary string     `json:"environment_summary"`
	CustomerContact    string     `json:"customer_contact"`
	RemarkText         string     `json:"remark_text"`
	LastUpgradeAt      *time.Time `json:"last_upgrade_at,omitempty"`
	LastChangeAt       *time.Time `json:"last_change_at,omitempty"`
	ArchivedAt         *time.Time `json:"archived_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type UpgradeRecord struct {
	ID              int64     `json:"id"`
	UpgradeNo       string    `json:"upgrade_no"`
	ProjectID       int64     `json:"project_id"`
	UpgradeDate     time.Time `json:"upgrade_date"`
	SourceVersion   string    `json:"source_version"`
	TargetVersion   string    `json:"target_version"`
	UpgradeStatus   string    `json:"upgrade_status"`
	OwnerUserID     int64     `json:"owner_user_id"`
	OwnerName       string    `json:"owner_name,omitempty"`
	CustomRetention string    `json:"custom_retention"`
	IssueSolution   string    `json:"issue_solution"`
	TestResult      string    `json:"test_result"`
	RemarkText      string    `json:"remark_text"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ConfigChange struct {
	ID               int64     `json:"id"`
	ConfigNo         string    `json:"config_no"`
	ProjectID        int64     `json:"project_id"`
	RelatedUpgradeID int64     `json:"related_upgrade_id"`
	EffectiveVersion string    `json:"effective_version"`
	ConfigPath       string    `json:"config_path"`
	ChangeReason     string    `json:"change_reason"`
	BeforeContent    string    `json:"before_content"`
	AfterContent     string    `json:"after_content"`
	TestResult       string    `json:"test_result"`
	ChangedBy        int64     `json:"changed_by"`
	ChangedByName    string    `json:"changed_by_name,omitempty"`
	ChangedAt        time.Time `json:"changed_at"`
	RemarkText       string    `json:"remark_text"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type SQLChange struct {
	ID               int64     `json:"id"`
	SQLNo            string    `json:"sql_no"`
	ProjectID        int64     `json:"project_id"`
	RelatedUpgradeID int64     `json:"related_upgrade_id"`
	EffectiveVersion string    `json:"effective_version"`
	ChangeTitle      string    `json:"change_title"`
	DBObjects        string    `json:"db_objects"`
	ChangeReason     string    `json:"change_reason"`
	ChangeSQL        string    `json:"change_sql"`
	RollbackSQL      string    `json:"rollback_sql"`
	TestResult       string    `json:"test_result"`
	ChangedBy        int64     `json:"changed_by"`
	ChangedByName    string    `json:"changed_by_name,omitempty"`
	ChangedAt        time.Time `json:"changed_at"`
	RemarkText       string    `json:"remark_text"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type IntegrationRecord struct {
	ID                   int64     `json:"id"`
	IntegrationNo        string    `json:"integration_no"`
	ProjectID            int64     `json:"project_id"`
	ExternalSystemName   string    `json:"external_system_name"`
	IntegrationType      string    `json:"integration_type"`
	IntegrationDirection string    `json:"integration_direction"`
	ContentDesc          string    `json:"content_desc"`
	JointStatus          string    `json:"joint_status"`
	ExternalOwner        string    `json:"external_owner"`
	InternalOwnerUserID  int64     `json:"internal_owner_user_id"`
	InternalOwnerName    string    `json:"internal_owner_name,omitempty"`
	EndpointDesc         string    `json:"endpoint_desc"`
	RemarkText           string    `json:"remark_text"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type ScriptAsset struct {
	ID               int64     `json:"id"`
	AssetNo          string    `json:"asset_no"`
	ProjectID        int64     `json:"project_id"`
	RelatedUpgradeID int64     `json:"related_upgrade_id"`
	AssetName        string    `json:"asset_name"`
	AssetType        string    `json:"asset_type"`
	DeployPath       string    `json:"deploy_path"`
	PurposeDesc      string    `json:"purpose_desc"`
	ExecuteCommand   string    `json:"execute_command"`
	RollbackMethod   string    `json:"rollback_method"`
	TestResult       string    `json:"test_result"`
	ChangedBy        int64     `json:"changed_by"`
	ChangedByName    string    `json:"changed_by_name,omitempty"`
	ChangedAt        time.Time `json:"changed_at"`
	RemarkText       string    `json:"remark_text"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ServiceRecord struct {
	ID               int64     `json:"id"`
	ServiceNo        string    `json:"service_no"`
	ProjectID        int64     `json:"project_id"`
	ProjectName      string    `json:"project_name,omitempty"`
	CustomerName     string    `json:"customer_name,omitempty"`
	RelatedUpgradeID int64     `json:"related_upgrade_id"`
	ServiceType      string    `json:"service_type"`
	ServiceMode      string    `json:"service_mode"`
	ServiceDate      time.Time `json:"service_date"`
	Summary          string    `json:"summary"`
	IssueVersion     string    `json:"issue_version"`
	ProblemDesc      string    `json:"problem_desc"`
	ProcessDesc      string    `json:"process_desc"`
	ResultDesc       string    `json:"result_desc"`
	NextAction       string    `json:"next_action"`
	OwnerUserID      int64     `json:"owner_user_id"`
	OwnerName        string    `json:"owner_name,omitempty"`
	RemarkText       string    `json:"remark_text"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Attachment struct {
	ID             int64     `json:"id"`
	ProjectID      int64     `json:"project_id"`
	RefType        string    `json:"ref_type"`
	RefID          int64     `json:"ref_id"`
	Title          string    `json:"title"`
	DocCategory    string    `json:"doc_category"`
	FileName       string    `json:"file_name"`
	OriginalName   string    `json:"original_name"`
	FileExt        string    `json:"file_ext"`
	MimeType       string    `json:"mime_type"`
	FileSize       int64     `json:"file_size"`
	StorageType    string    `json:"storage_type"`
	RelativePath   string    `json:"relative_path"`
	ThumbnailPath  string    `json:"thumbnail_path"`
	Tags           string    `json:"tags"`
	Description    string    `json:"description"`
	UploadedBy     int64     `json:"uploaded_by"`
	UploadedByName string    `json:"uploaded_by_name,omitempty"`
	UploadedAt     time.Time `json:"uploaded_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type AuditLog struct {
	ID               int64     `json:"id"`
	ProjectID        int64     `json:"project_id"`
	ObjectType       string    `json:"object_type"`
	ObjectID         int64     `json:"object_id"`
	OperationType    string    `json:"operation_type"`
	OperationSummary string    `json:"operation_summary"`
	BeforeSnapshot   string    `json:"before_snapshot"`
	AfterSnapshot    string    `json:"after_snapshot"`
	OperatorUserID   int64     `json:"operator_user_id"`
	OperatorUserName string    `json:"operator_user_name,omitempty"`
	OperatedAt       time.Time `json:"operated_at"`
	IPAddress        string    `json:"ip_address"`
	UserAgent        string    `json:"user_agent"`
}

type LoginLog struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Username     string    `json:"username"`
	LoginResult  string    `json:"login_result"`
	LoginMessage string    `json:"login_message"`
	LoginIP      string    `json:"login_ip"`
	UserAgent    string    `json:"user_agent"`
	LoginAt      time.Time `json:"login_at"`
}

type DashboardVersionStat struct {
	CurrentVersion string `json:"current_version"`
	ProjectCount   int    `json:"project_count"`
}

type DashboardIssueVersionStat struct {
	IssueVersion string `json:"issue_version"`
	IssueCount   int    `json:"issue_count"`
}

type DashboardOverview struct {
	SelectedMonth         string                      `json:"selected_month"`
	ProjectTotal          int                         `json:"project_total"`
	MaintenanceProjectNum int                         `json:"maintenance_project_num"`
	MonthlyUpgradeNum     int                         `json:"monthly_upgrade_num"`
	MonthlyServiceNum     int                         `json:"monthly_service_num"`
	MonthlyIssueNum       int                         `json:"monthly_issue_num"`
	MissingDocumentNum    int                         `json:"missing_document_num"`
	VersionStats          []DashboardVersionStat      `json:"version_stats"`
	IssueVersionStats     []DashboardIssueVersionStat `json:"issue_version_stats"`
}

type ProjectFilter struct {
	Keyword        string
	CustomerName   string
	ProjectStatus  string
	CurrentVersion string
	Page           int
	PageSize       int
}

type ListFilter struct {
	Keyword   string
	StartDate *time.Time
	EndDate   *time.Time
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}

type IssueFilter struct {
	Keyword      string
	IssueVersion string
	Page         int
	PageSize     int
}

type AttachmentFilter struct {
	RefType            string
	RefID              int64
	DocCategory        string
	ExcludeDocCategory string
	Page               int
	PageSize           int
}

type PagedResult[T any] struct {
	List     []T `json:"list"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

type ProjectOverview struct {
	Project             Project             `json:"project"`
	RecentUpgrades      []UpgradeRecord     `json:"recent_upgrades"`
	RecentConfigChanges []ConfigChange      `json:"recent_config_changes"`
	RecentSQLChanges    []SQLChange         `json:"recent_sql_changes"`
	RecentAssets        []ScriptAsset       `json:"recent_assets"`
	RecentServices      []ServiceRecord     `json:"recent_services"`
	RecentAttachments   []Attachment        `json:"recent_attachments"`
	Integrations        []IntegrationRecord `json:"integrations"`
}
