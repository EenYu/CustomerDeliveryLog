package memory

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"customerdeliverylog/internal/model"
	"customerdeliverylog/internal/store"
)

type MemoryStore struct {
	mu sync.RWMutex

	nextID map[string]int64

	users        map[int64]model.User
	loginLogs    map[int64]model.LoginLog
	projects     map[int64]model.Project
	upgrades     map[int64]model.UpgradeRecord
	configs      map[int64]model.ConfigChange
	sqlChanges   map[int64]model.SQLChange
	integrations map[int64]model.IntegrationRecord
	assets       map[int64]model.ScriptAsset
	services     map[int64]model.ServiceRecord
	attachments  map[int64]model.Attachment
	auditLogs    map[int64]model.AuditLog
}

func New() *MemoryStore {
	return &MemoryStore{
		nextID: map[string]int64{},

		users:        map[int64]model.User{},
		loginLogs:    map[int64]model.LoginLog{},
		projects:     map[int64]model.Project{},
		upgrades:     map[int64]model.UpgradeRecord{},
		configs:      map[int64]model.ConfigChange{},
		sqlChanges:   map[int64]model.SQLChange{},
		integrations: map[int64]model.IntegrationRecord{},
		assets:       map[int64]model.ScriptAsset{},
		services:     map[int64]model.ServiceRecord{},
		attachments:  map[int64]model.Attachment{},
		auditLogs:    map[int64]model.AuditLog{},
	}
}

func (s *MemoryStore) next(entity string) int64 {
	s.nextID[entity]++
	return s.nextID[entity]
}

func nowIfZero(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}

func (s *MemoryStore) SeedAdmin(_ context.Context, username, realName, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, user := range s.users {
		if user.Username == username {
			return nil
		}
	}
	id := s.next("user")
	now := time.Now()
	s.users[id] = model.User{
		ID:           id,
		Username:     username,
		RealName:     realName,
		PasswordHash: passwordHash,
		Roles:        []string{"admin"},
		Status:       true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return nil
}

func (s *MemoryStore) FindUserByUsername(_ context.Context, username string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, user := range s.users {
		if strings.EqualFold(user.Username, username) {
			copy := user
			return &copy, nil
		}
	}
	return nil, store.ErrNotFound
}

func (s *MemoryStore) GetUser(_ context.Context, id int64) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.users[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	copy := item
	return &copy, nil
}

func (s *MemoryStore) ListUsers(_ context.Context) ([]model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]model.User, 0, len(s.users))
	for _, item := range s.users {
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	return list, nil
}

func (s *MemoryStore) CreateUser(_ context.Context, user *model.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.users {
		if strings.EqualFold(item.Username, user.Username) {
			return errors.New("username already exists")
		}
	}
	now := time.Now()
	user.ID = s.next("user")
	user.CreatedAt = now
	user.UpdatedAt = now
	s.users[user.ID] = *user
	return nil
}

func (s *MemoryStore) UpdateUser(_ context.Context, user *model.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.users[user.ID]
	if !ok {
		return store.ErrNotFound
	}
	current.RealName = user.RealName
	current.Roles = append([]string{}, user.Roles...)
	current.Status = user.Status
	current.UpdatedAt = time.Now()
	s.users[user.ID] = current
	return nil
}

func (s *MemoryStore) UpdateUserPassword(_ context.Context, userID int64, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.users[userID]
	if !ok {
		return store.ErrNotFound
	}
	current.PasswordHash = passwordHash
	current.UpdatedAt = time.Now()
	s.users[userID] = current
	return nil
}

func (s *MemoryStore) DeleteUser(_ context.Context, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.users, userID)
	return nil
}

func (s *MemoryStore) CreateLoginLog(_ context.Context, log *model.LoginLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.ID = s.next("loginLog")
	log.LoginAt = nowIfZero(log.LoginAt)
	s.loginLogs[log.ID] = *log
	return nil
}

func (s *MemoryStore) ListLoginLogs(_ context.Context) ([]model.LoginLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]model.LoginLog, 0, len(s.loginLogs))
	for _, item := range s.loginLogs {
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].LoginAt.After(list[j].LoginAt)
	})
	return list, nil
}

func (s *MemoryStore) DashboardOverview(_ context.Context, month string) (model.DashboardOverview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var overview model.DashboardOverview
	selectedMonth, err := resolveDashboardMonth(month)
	if err != nil {
		return overview, err
	}
	overview.SelectedMonth = selectedMonth.Format("2006-01")
	overview.ProjectTotal = len(s.projects)
	currentMonth := selectedMonth.Month()
	currentYear := selectedMonth.Year()
	for _, p := range s.projects {
		if p.ProjectStatus == "maintenance" {
			overview.MaintenanceProjectNum++
		}
	}
	for _, item := range s.upgrades {
		if _, ok := s.projects[item.ProjectID]; !ok {
			continue
		}
		if item.UpgradeDate.Month() == currentMonth && item.UpgradeDate.Year() == currentYear {
			overview.MonthlyUpgradeNum++
		}
	}
	for _, item := range s.services {
		if _, ok := s.projects[item.ProjectID]; !ok {
			continue
		}
		if item.ServiceDate.Month() == currentMonth && item.ServiceDate.Year() == currentYear && item.ServiceType != "incident" {
			overview.MonthlyServiceNum++
		}
		if item.ServiceDate.Month() == currentMonth && item.ServiceDate.Year() == currentYear && item.ServiceType == "incident" {
			overview.MonthlyIssueNum++
		}
	}
	for _, project := range s.projects {
		hasDoc := false
		for _, attachment := range s.attachments {
			if attachment.ProjectID == project.ID {
				hasDoc = true
				break
			}
		}
		if !hasDoc {
			overview.MissingDocumentNum++
		}
	}
	versionCount := map[string]int{}
	for _, project := range s.projects {
		version := strings.TrimSpace(project.CurrentVersion)
		if version == "" {
			version = "未填写"
		}
		versionCount[version]++
	}
	overview.VersionStats = make([]model.DashboardVersionStat, 0, len(versionCount))
	for version, count := range versionCount {
		overview.VersionStats = append(overview.VersionStats, model.DashboardVersionStat{
			CurrentVersion: version,
			ProjectCount:   count,
		})
	}
	sort.Slice(overview.VersionStats, func(i, j int) bool {
		if overview.VersionStats[i].ProjectCount == overview.VersionStats[j].ProjectCount {
			return overview.VersionStats[i].CurrentVersion < overview.VersionStats[j].CurrentVersion
		}
		return overview.VersionStats[i].ProjectCount > overview.VersionStats[j].ProjectCount
	})
	issueVersionCount := map[string]int{}
	for _, item := range s.services {
		if _, ok := s.projects[item.ProjectID]; !ok {
			continue
		}
		if item.ServiceType != "incident" {
			continue
		}
		version := strings.TrimSpace(item.IssueVersion)
		if version == "" {
			version = "未填写版本"
		}
		issueVersionCount[version]++
	}
	overview.IssueVersionStats = make([]model.DashboardIssueVersionStat, 0, len(issueVersionCount))
	for version, count := range issueVersionCount {
		overview.IssueVersionStats = append(overview.IssueVersionStats, model.DashboardIssueVersionStat{
			IssueVersion: version,
			IssueCount:   count,
		})
	}
	sort.Slice(overview.IssueVersionStats, func(i, j int) bool {
		if overview.IssueVersionStats[i].IssueCount == overview.IssueVersionStats[j].IssueCount {
			return overview.IssueVersionStats[i].IssueVersion < overview.IssueVersionStats[j].IssueVersion
		}
		return overview.IssueVersionStats[i].IssueCount > overview.IssueVersionStats[j].IssueCount
	})
	return overview, nil
}

func resolveDashboardMonth(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), nil
	}
	selected, err := time.ParseInLocation("2006-01", value, time.Local)
	if err != nil {
		return time.Time{}, errors.New("月份格式应为 YYYY-MM")
	}
	return selected, nil
}

func (s *MemoryStore) ListProjects(_ context.Context, filter model.ProjectFilter) (model.PagedResult[model.Project], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]model.Project, 0, len(s.projects))
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	customer := strings.ToLower(strings.TrimSpace(filter.CustomerName))
	for _, item := range s.projects {
		if keyword != "" && !strings.Contains(strings.ToLower(item.ProjectName), keyword) && !strings.Contains(strings.ToLower(item.CustomerName), keyword) {
			continue
		}
		if customer != "" && !strings.Contains(strings.ToLower(item.CustomerName), customer) {
			continue
		}
		if filter.ProjectStatus != "" && item.ProjectStatus != filter.ProjectStatus {
			continue
		}
		if filter.CurrentVersion != "" && item.CurrentVersion != filter.CurrentVersion {
			continue
		}
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return paginate(list, filter.Page, filter.PageSize), nil
}

func (s *MemoryStore) GetProject(_ context.Context, projectID int64) (*model.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.projects[projectID]
	if !ok {
		return nil, store.ErrNotFound
	}
	copy := item
	return &copy, nil
}

func (s *MemoryStore) CreateProject(_ context.Context, project *model.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	project.ID = s.next("project")
	project.CreatedAt = now
	project.UpdatedAt = now
	s.projects[project.ID] = *project
	return nil
}

func (s *MemoryStore) UpdateProject(_ context.Context, project *model.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.projects[project.ID]
	if !ok {
		return store.ErrNotFound
	}
	project.CreatedAt = current.CreatedAt
	project.UpdatedAt = time.Now()
	s.projects[project.ID] = *project
	return nil
}

func (s *MemoryStore) ArchiveProject(_ context.Context, projectID int64, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	project, ok := s.projects[projectID]
	if !ok {
		return store.ErrNotFound
	}
	now := time.Now()
	project.ProjectStatus = "archived"
	project.ArchivedAt = &now
	project.UpdatedAt = now
	s.projects[projectID] = project
	return nil
}

func (s *MemoryStore) DeleteProject(_ context.Context, projectID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.projects[projectID]; !ok {
		return store.ErrNotFound
	}
	delete(s.projects, projectID)
	for id, item := range s.upgrades {
		if item.ProjectID == projectID {
			delete(s.upgrades, id)
		}
	}
	for id, item := range s.configs {
		if item.ProjectID == projectID {
			delete(s.configs, id)
		}
	}
	for id, item := range s.sqlChanges {
		if item.ProjectID == projectID {
			delete(s.sqlChanges, id)
		}
	}
	for id, item := range s.integrations {
		if item.ProjectID == projectID {
			delete(s.integrations, id)
		}
	}
	for id, item := range s.assets {
		if item.ProjectID == projectID {
			delete(s.assets, id)
		}
	}
	for id, item := range s.services {
		if item.ProjectID == projectID {
			delete(s.services, id)
		}
	}
	for id, item := range s.attachments {
		if item.ProjectID == projectID {
			delete(s.attachments, id)
		}
	}
	for id, item := range s.auditLogs {
		if item.ProjectID == projectID {
			delete(s.auditLogs, id)
		}
	}
	return nil
}

func (s *MemoryStore) ListUpgrades(_ context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.UpgradeRecord], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]model.UpgradeRecord, 0)
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	for _, item := range s.upgrades {
		if item.ProjectID != projectID {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.UpgradeNo), keyword) && !strings.Contains(strings.ToLower(item.SourceVersion), keyword) && !strings.Contains(strings.ToLower(item.TargetVersion), keyword) {
			continue
		}
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpgradeDate.After(list[j].UpgradeDate)
	})
	return paginate(list, filter.Page, filter.PageSize), nil
}

func (s *MemoryStore) GetUpgrade(_ context.Context, id int64) (*model.UpgradeRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.upgrades[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	copy := item
	return &copy, nil
}

func (s *MemoryStore) CreateUpgrade(_ context.Context, item *model.UpgradeRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	item.ID = s.next("upgrade")
	item.CreatedAt = now
	item.UpdatedAt = now
	s.upgrades[item.ID] = *item
	return nil
}

func (s *MemoryStore) UpdateUpgrade(_ context.Context, item *model.UpgradeRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.upgrades[item.ID]
	if !ok {
		return store.ErrNotFound
	}
	item.CreatedAt = current.CreatedAt
	item.UpdatedAt = time.Now()
	s.upgrades[item.ID] = *item
	return nil
}

func (s *MemoryStore) DeleteUpgrade(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.upgrades, id)
	return nil
}

func (s *MemoryStore) ListConfigChanges(_ context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.ConfigChange], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]model.ConfigChange, 0)
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	for _, item := range s.configs {
		if item.ProjectID != projectID {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.ConfigNo), keyword) && !strings.Contains(strings.ToLower(item.ConfigPath), keyword) {
			continue
		}
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ChangedAt.After(list[j].ChangedAt)
	})
	return paginate(list, filter.Page, filter.PageSize), nil
}

func (s *MemoryStore) GetConfigChange(_ context.Context, id int64) (*model.ConfigChange, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.configs[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	copy := item
	return &copy, nil
}

func (s *MemoryStore) CreateConfigChange(_ context.Context, item *model.ConfigChange) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	item.ID = s.next("config")
	item.CreatedAt = now
	item.UpdatedAt = now
	s.configs[item.ID] = *item
	return nil
}

func (s *MemoryStore) UpdateConfigChange(_ context.Context, item *model.ConfigChange) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.configs[item.ID]
	if !ok {
		return store.ErrNotFound
	}
	item.CreatedAt = current.CreatedAt
	item.UpdatedAt = time.Now()
	s.configs[item.ID] = *item
	return nil
}

func (s *MemoryStore) DeleteConfigChange(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.configs, id)
	return nil
}

func (s *MemoryStore) ListSQLChanges(_ context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.SQLChange], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]model.SQLChange, 0)
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	for _, item := range s.sqlChanges {
		if item.ProjectID != projectID {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.SQLNo), keyword) && !strings.Contains(strings.ToLower(item.ChangeTitle), keyword) {
			continue
		}
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ChangedAt.After(list[j].ChangedAt)
	})
	return paginate(list, filter.Page, filter.PageSize), nil
}

func (s *MemoryStore) GetSQLChange(_ context.Context, id int64) (*model.SQLChange, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.sqlChanges[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	copy := item
	return &copy, nil
}

func (s *MemoryStore) CreateSQLChange(_ context.Context, item *model.SQLChange) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	item.ID = s.next("sql")
	item.CreatedAt = now
	item.UpdatedAt = now
	s.sqlChanges[item.ID] = *item
	return nil
}

func (s *MemoryStore) UpdateSQLChange(_ context.Context, item *model.SQLChange) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.sqlChanges[item.ID]
	if !ok {
		return store.ErrNotFound
	}
	item.CreatedAt = current.CreatedAt
	item.UpdatedAt = time.Now()
	s.sqlChanges[item.ID] = *item
	return nil
}

func (s *MemoryStore) DeleteSQLChange(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sqlChanges, id)
	return nil
}

func (s *MemoryStore) ListIntegrations(_ context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.IntegrationRecord], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]model.IntegrationRecord, 0)
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	for _, item := range s.integrations {
		if item.ProjectID != projectID {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.IntegrationNo), keyword) && !strings.Contains(strings.ToLower(item.ExternalSystemName), keyword) {
			continue
		}
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return paginate(list, filter.Page, filter.PageSize), nil
}

func (s *MemoryStore) GetIntegration(_ context.Context, id int64) (*model.IntegrationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.integrations[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	copy := item
	return &copy, nil
}

func (s *MemoryStore) CreateIntegration(_ context.Context, item *model.IntegrationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	item.ID = s.next("integration")
	item.CreatedAt = now
	item.UpdatedAt = now
	s.integrations[item.ID] = *item
	return nil
}

func (s *MemoryStore) UpdateIntegration(_ context.Context, item *model.IntegrationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.integrations[item.ID]
	if !ok {
		return store.ErrNotFound
	}
	item.CreatedAt = current.CreatedAt
	item.UpdatedAt = time.Now()
	s.integrations[item.ID] = *item
	return nil
}

func (s *MemoryStore) DeleteIntegration(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.integrations, id)
	return nil
}

func (s *MemoryStore) ListAssets(_ context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.ScriptAsset], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]model.ScriptAsset, 0)
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	for _, item := range s.assets {
		if item.ProjectID != projectID {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.AssetNo), keyword) && !strings.Contains(strings.ToLower(item.AssetName), keyword) {
			continue
		}
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ChangedAt.After(list[j].ChangedAt)
	})
	return paginate(list, filter.Page, filter.PageSize), nil
}

func (s *MemoryStore) GetAsset(_ context.Context, id int64) (*model.ScriptAsset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.assets[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	copy := item
	return &copy, nil
}

func (s *MemoryStore) CreateAsset(_ context.Context, item *model.ScriptAsset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	item.ID = s.next("asset")
	item.CreatedAt = now
	item.UpdatedAt = now
	s.assets[item.ID] = *item
	return nil
}

func (s *MemoryStore) UpdateAsset(_ context.Context, item *model.ScriptAsset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.assets[item.ID]
	if !ok {
		return store.ErrNotFound
	}
	item.CreatedAt = current.CreatedAt
	item.UpdatedAt = time.Now()
	s.assets[item.ID] = *item
	return nil
}

func (s *MemoryStore) DeleteAsset(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.assets, id)
	return nil
}

func (s *MemoryStore) ListServiceRecords(_ context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.ServiceRecord], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]model.ServiceRecord, 0)
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	for _, item := range s.services {
		if item.ProjectID != projectID {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.ServiceNo), keyword) && !strings.Contains(strings.ToLower(item.Summary), keyword) {
			continue
		}
		list = append(list, s.decorateServiceRecord(item))
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ServiceDate.After(list[j].ServiceDate)
	})
	return paginate(list, filter.Page, filter.PageSize), nil
}

func (s *MemoryStore) ListIssueRecords(_ context.Context, filter model.IssueFilter) (model.PagedResult[model.ServiceRecord], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]model.ServiceRecord, 0)
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	version := strings.ToLower(strings.TrimSpace(filter.IssueVersion))
	for _, item := range s.services {
		if item.ServiceType != "incident" {
			continue
		}
		if _, ok := s.projects[item.ProjectID]; !ok {
			continue
		}
		decorated := s.decorateServiceRecord(item)
		if keyword != "" {
			if !strings.Contains(strings.ToLower(decorated.ServiceNo), keyword) &&
				!strings.Contains(strings.ToLower(decorated.Summary), keyword) &&
				!strings.Contains(strings.ToLower(decorated.ProjectName), keyword) &&
				!strings.Contains(strings.ToLower(decorated.CustomerName), keyword) &&
				!strings.Contains(strings.ToLower(decorated.ProblemDesc), keyword) {
				continue
			}
		}
		if version != "" && !strings.Contains(strings.ToLower(strings.TrimSpace(decorated.IssueVersion)), version) {
			continue
		}
		list = append(list, decorated)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ServiceDate.After(list[j].ServiceDate)
	})
	return paginate(list, filter.Page, filter.PageSize), nil
}

func (s *MemoryStore) GetServiceRecord(_ context.Context, id int64) (*model.ServiceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.services[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	copy := s.decorateServiceRecord(item)
	return &copy, nil
}

func (s *MemoryStore) CreateServiceRecord(_ context.Context, item *model.ServiceRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	item.ID = s.next("service")
	item.CreatedAt = now
	item.UpdatedAt = now
	s.services[item.ID] = *item
	return nil
}

func (s *MemoryStore) UpdateServiceRecord(_ context.Context, item *model.ServiceRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.services[item.ID]
	if !ok {
		return store.ErrNotFound
	}
	item.CreatedAt = current.CreatedAt
	item.UpdatedAt = time.Now()
	s.services[item.ID] = *item
	return nil
}

func (s *MemoryStore) DeleteServiceRecord(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.services, id)
	return nil
}

func (s *MemoryStore) decorateServiceRecord(item model.ServiceRecord) model.ServiceRecord {
	if project, ok := s.projects[item.ProjectID]; ok {
		item.ProjectName = project.ProjectName
		item.CustomerName = project.CustomerName
	}
	return item
}

func (s *MemoryStore) ListAttachments(_ context.Context, projectID int64, filter model.AttachmentFilter) (model.PagedResult[model.Attachment], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]model.Attachment, 0)
	for _, item := range s.attachments {
		if item.ProjectID != projectID {
			continue
		}
		if filter.RefType != "" && item.RefType != filter.RefType {
			continue
		}
		if filter.RefID > 0 && item.RefID != filter.RefID {
			continue
		}
		if filter.DocCategory != "" && item.DocCategory != filter.DocCategory {
			continue
		}
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UploadedAt.After(list[j].UploadedAt)
	})
	return paginate(list, filter.Page, filter.PageSize), nil
}

func (s *MemoryStore) GetAttachment(_ context.Context, id int64) (*model.Attachment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.attachments[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	copy := item
	return &copy, nil
}

func (s *MemoryStore) CreateAttachment(_ context.Context, item *model.Attachment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	item.ID = s.next("attachment")
	item.CreatedAt = now
	item.UpdatedAt = now
	item.UploadedAt = nowIfZero(item.UploadedAt)
	s.attachments[item.ID] = *item
	return nil
}

func (s *MemoryStore) UpdateAttachment(_ context.Context, item *model.Attachment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.attachments[item.ID]
	if !ok {
		return store.ErrNotFound
	}
	item.CreatedAt = current.CreatedAt
	item.UploadedAt = current.UploadedAt
	item.UpdatedAt = time.Now()
	s.attachments[item.ID] = *item
	return nil
}

func (s *MemoryStore) DeleteAttachment(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.attachments, id)
	return nil
}

func (s *MemoryStore) CreateAuditLog(_ context.Context, log *model.AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.ID = s.next("auditLog")
	log.OperatedAt = nowIfZero(log.OperatedAt)
	s.auditLogs[log.ID] = *log
	return nil
}

func (s *MemoryStore) ListAuditLogs(_ context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.AuditLog], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]model.AuditLog, 0)
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	for _, item := range s.auditLogs {
		if projectID > 0 && item.ProjectID != projectID {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.OperationSummary), keyword) {
			continue
		}
		list = append(list, item)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].OperatedAt.After(list[j].OperatedAt)
	})
	return paginate(list, filter.Page, filter.PageSize), nil
}

func paginate[T any](list []T, page, pageSize int) model.PagedResult[T] {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	total := len(list)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return model.PagedResult[T]{
		List:     list[start:end],
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}
}
