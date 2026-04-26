package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"customerdeliverylog/internal/model"
	"customerdeliverylog/internal/store"
)

func (s *MySQLStore) ListUpgrades(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.UpgradeRecord], error) {
	where, args := recordListWhere("u", projectID, filter, []string{"upgrade_no", "source_version", "target_version"})
	total, err := queryCount(ctx, s.db, "SELECT COUNT(1) FROM project_upgrade_record u "+where, args...)
	if err != nil {
		return model.PagedResult[model.UpgradeRecord]{}, err
	}
	limit, offset := paginateClause(filter.Page, filter.PageSize)
	query := `
		SELECT
			u.id, u.upgrade_no, u.project_id, u.upgrade_date, u.source_version, u.target_version,
			u.upgrade_status, u.owner_user_id, IFNULL(su.real_name, ''), u.custom_retention,
			IFNULL(u.issue_solution, ''), IFNULL(u.test_result, ''), IFNULL(u.remark_text, ''),
			u.created_at, u.updated_at
		FROM project_upgrade_record u
		LEFT JOIN sys_user su ON su.id = u.owner_user_id AND su.is_deleted = 0
	` + where + ` ORDER BY u.upgrade_date DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return model.PagedResult[model.UpgradeRecord]{}, err
	}
	defer rows.Close()
	list := make([]model.UpgradeRecord, 0)
	for rows.Next() {
		item, err := scanUpgradeRows(rows)
		if err != nil {
			return model.PagedResult[model.UpgradeRecord]{}, err
		}
		list = append(list, *item)
	}
	return pagedResult(list, filter.Page, filter.PageSize, total), rows.Err()
}

func (s *MySQLStore) GetUpgrade(ctx context.Context, id int64) (*model.UpgradeRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			u.id, u.upgrade_no, u.project_id, u.upgrade_date, u.source_version, u.target_version,
			u.upgrade_status, u.owner_user_id, IFNULL(su.real_name, ''), u.custom_retention,
			IFNULL(u.issue_solution, ''), IFNULL(u.test_result, ''), IFNULL(u.remark_text, ''),
			u.created_at, u.updated_at
		FROM project_upgrade_record u
		LEFT JOIN sys_user su ON su.id = u.owner_user_id AND su.is_deleted = 0
		WHERE u.id = ? AND u.is_deleted = 0
		LIMIT 1
	`, id)
	item, err := scanUpgradeRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func (s *MySQLStore) CreateUpgrade(ctx context.Context, item *model.UpgradeRecord) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO project_upgrade_record (
			upgrade_no, project_id, upgrade_date, source_version, target_version, upgrade_status,
			owner_user_id, custom_retention, issue_solution, test_result, remark_text,
			created_at, updated_at, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), 0)
	`,
		item.UpgradeNo, item.ProjectID, item.UpgradeDate, item.SourceVersion, item.TargetVersion,
		item.UpgradeStatus, item.OwnerUserID, item.CustomRetention, nullString(item.IssueSolution),
		nullString(item.TestResult), nullString(item.RemarkText),
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	saved, err := s.GetUpgrade(ctx, id)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) UpdateUpgrade(ctx context.Context, item *model.UpgradeRecord) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE project_upgrade_record
		SET upgrade_date = ?, source_version = ?, target_version = ?, upgrade_status = ?,
		    owner_user_id = ?, custom_retention = ?, issue_solution = ?, test_result = ?,
		    remark_text = ?, updated_at = NOW()
		WHERE id = ? AND is_deleted = 0
	`,
		item.UpgradeDate, item.SourceVersion, item.TargetVersion, item.UpgradeStatus,
		item.OwnerUserID, item.CustomRetention, nullString(item.IssueSolution), nullString(item.TestResult),
		nullString(item.RemarkText), item.ID,
	)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return store.ErrNotFound
	}
	saved, err := s.GetUpgrade(ctx, item.ID)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) DeleteUpgrade(ctx context.Context, id int64) error {
	return softDeleteByID(ctx, s.db, "project_upgrade_record", id)
}

func (s *MySQLStore) ListConfigChanges(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.ConfigChange], error) {
	where, args := recordListWhere("c", projectID, filter, []string{"config_no", "config_path"})
	total, err := queryCount(ctx, s.db, "SELECT COUNT(1) FROM project_config_change c "+where, args...)
	if err != nil {
		return model.PagedResult[model.ConfigChange]{}, err
	}
	limit, offset := paginateClause(filter.Page, filter.PageSize)
	query := `
		SELECT
			c.id, c.config_no, c.project_id, IFNULL(c.related_upgrade_id, 0), IFNULL(c.effective_version, ''),
			c.config_path, c.change_reason, IFNULL(c.before_content, ''), c.after_content,
			IFNULL(c.test_result, ''), c.changed_by, IFNULL(su.real_name, ''), c.changed_at,
			IFNULL(c.remark_text, ''), c.created_at, c.updated_at
		FROM project_config_change c
		LEFT JOIN sys_user su ON su.id = c.changed_by AND su.is_deleted = 0
	` + where + ` ORDER BY c.changed_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return model.PagedResult[model.ConfigChange]{}, err
	}
	defer rows.Close()
	list := make([]model.ConfigChange, 0)
	for rows.Next() {
		item, err := scanConfigRows(rows)
		if err != nil {
			return model.PagedResult[model.ConfigChange]{}, err
		}
		list = append(list, *item)
	}
	return pagedResult(list, filter.Page, filter.PageSize, total), rows.Err()
}

func (s *MySQLStore) GetConfigChange(ctx context.Context, id int64) (*model.ConfigChange, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			c.id, c.config_no, c.project_id, IFNULL(c.related_upgrade_id, 0), IFNULL(c.effective_version, ''),
			c.config_path, c.change_reason, IFNULL(c.before_content, ''), c.after_content,
			IFNULL(c.test_result, ''), c.changed_by, IFNULL(su.real_name, ''), c.changed_at,
			IFNULL(c.remark_text, ''), c.created_at, c.updated_at
		FROM project_config_change c
		LEFT JOIN sys_user su ON su.id = c.changed_by AND su.is_deleted = 0
		WHERE c.id = ? AND c.is_deleted = 0
		LIMIT 1
	`, id)
	item, err := scanConfigRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func (s *MySQLStore) CreateConfigChange(ctx context.Context, item *model.ConfigChange) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO project_config_change (
			config_no, project_id, related_upgrade_id, effective_version, config_path,
			change_reason, before_content, after_content, test_result, changed_by, changed_at,
			remark_text, created_at, updated_at, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), 0)
	`,
		item.ConfigNo, item.ProjectID, nullableInt(item.RelatedUpgradeID), nullString(item.EffectiveVersion),
		item.ConfigPath, item.ChangeReason, nullString(item.BeforeContent), item.AfterContent,
		nullString(item.TestResult), item.ChangedBy, item.ChangedAt, nullString(item.RemarkText),
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	saved, err := s.GetConfigChange(ctx, id)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) UpdateConfigChange(ctx context.Context, item *model.ConfigChange) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE project_config_change
		SET related_upgrade_id = ?, effective_version = ?, config_path = ?, change_reason = ?,
		    before_content = ?, after_content = ?, test_result = ?, changed_by = ?, changed_at = ?,
		    remark_text = ?, updated_at = NOW()
		WHERE id = ? AND is_deleted = 0
	`,
		nullableInt(item.RelatedUpgradeID), nullString(item.EffectiveVersion), item.ConfigPath, item.ChangeReason,
		nullString(item.BeforeContent), item.AfterContent, nullString(item.TestResult), item.ChangedBy,
		item.ChangedAt, nullString(item.RemarkText), item.ID,
	)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return store.ErrNotFound
	}
	saved, err := s.GetConfigChange(ctx, item.ID)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) DeleteConfigChange(ctx context.Context, id int64) error {
	return softDeleteByID(ctx, s.db, "project_config_change", id)
}

func (s *MySQLStore) ListSQLChanges(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.SQLChange], error) {
	where, args := recordListWhere("sq", projectID, filter, []string{"sql_no", "change_title", "db_objects"})
	total, err := queryCount(ctx, s.db, "SELECT COUNT(1) FROM project_sql_change sq "+where, args...)
	if err != nil {
		return model.PagedResult[model.SQLChange]{}, err
	}
	limit, offset := paginateClause(filter.Page, filter.PageSize)
	query := `
		SELECT
			sq.id, sq.sql_no, sq.project_id, IFNULL(sq.related_upgrade_id, 0), IFNULL(sq.effective_version, ''),
			sq.change_title, IFNULL(sq.db_objects, ''), sq.change_reason, sq.change_sql,
			IFNULL(sq.rollback_sql, ''), IFNULL(sq.test_result, ''), sq.changed_by, IFNULL(su.real_name, ''),
			sq.changed_at, IFNULL(sq.remark_text, ''), sq.created_at, sq.updated_at
		FROM project_sql_change sq
		LEFT JOIN sys_user su ON su.id = sq.changed_by AND su.is_deleted = 0
	` + where + ` ORDER BY sq.changed_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return model.PagedResult[model.SQLChange]{}, err
	}
	defer rows.Close()
	list := make([]model.SQLChange, 0)
	for rows.Next() {
		item, err := scanSQLRows(rows)
		if err != nil {
			return model.PagedResult[model.SQLChange]{}, err
		}
		list = append(list, *item)
	}
	return pagedResult(list, filter.Page, filter.PageSize, total), rows.Err()
}

func (s *MySQLStore) GetSQLChange(ctx context.Context, id int64) (*model.SQLChange, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			sq.id, sq.sql_no, sq.project_id, IFNULL(sq.related_upgrade_id, 0), IFNULL(sq.effective_version, ''),
			sq.change_title, IFNULL(sq.db_objects, ''), sq.change_reason, sq.change_sql,
			IFNULL(sq.rollback_sql, ''), IFNULL(sq.test_result, ''), sq.changed_by, IFNULL(su.real_name, ''),
			sq.changed_at, IFNULL(sq.remark_text, ''), sq.created_at, sq.updated_at
		FROM project_sql_change sq
		LEFT JOIN sys_user su ON su.id = sq.changed_by AND su.is_deleted = 0
		WHERE sq.id = ? AND sq.is_deleted = 0
		LIMIT 1
	`, id)
	item, err := scanSQLRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func (s *MySQLStore) CreateSQLChange(ctx context.Context, item *model.SQLChange) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO project_sql_change (
			sql_no, project_id, related_upgrade_id, effective_version, change_title, db_objects,
			change_reason, change_sql, rollback_sql, test_result, changed_by, changed_at,
			remark_text, created_at, updated_at, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), 0)
	`,
		item.SQLNo, item.ProjectID, nullableInt(item.RelatedUpgradeID), nullString(item.EffectiveVersion),
		item.ChangeTitle, nullString(item.DBObjects), item.ChangeReason, item.ChangeSQL,
		nullString(item.RollbackSQL), nullString(item.TestResult), item.ChangedBy, item.ChangedAt,
		nullString(item.RemarkText),
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	saved, err := s.GetSQLChange(ctx, id)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) UpdateSQLChange(ctx context.Context, item *model.SQLChange) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE project_sql_change
		SET related_upgrade_id = ?, effective_version = ?, change_title = ?, db_objects = ?,
		    change_reason = ?, change_sql = ?, rollback_sql = ?, test_result = ?,
		    changed_by = ?, changed_at = ?, remark_text = ?, updated_at = NOW()
		WHERE id = ? AND is_deleted = 0
	`,
		nullableInt(item.RelatedUpgradeID), nullString(item.EffectiveVersion), item.ChangeTitle,
		nullString(item.DBObjects), item.ChangeReason, item.ChangeSQL, nullString(item.RollbackSQL),
		nullString(item.TestResult), item.ChangedBy, item.ChangedAt, nullString(item.RemarkText), item.ID,
	)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return store.ErrNotFound
	}
	saved, err := s.GetSQLChange(ctx, item.ID)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) DeleteSQLChange(ctx context.Context, id int64) error {
	return softDeleteByID(ctx, s.db, "project_sql_change", id)
}

func (s *MySQLStore) ListIntegrations(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.IntegrationRecord], error) {
	where, args := recordListWhere("i", projectID, filter, []string{"integration_no", "external_system_name", "endpoint_desc"})
	total, err := queryCount(ctx, s.db, "SELECT COUNT(1) FROM project_integration_record i "+where, args...)
	if err != nil {
		return model.PagedResult[model.IntegrationRecord]{}, err
	}
	limit, offset := paginateClause(filter.Page, filter.PageSize)
	query := `
		SELECT
			i.id, i.integration_no, i.project_id, i.external_system_name, i.integration_type,
			IFNULL(i.integration_direction, ''), i.content_desc, i.joint_status,
			IFNULL(i.external_owner, ''), IFNULL(i.internal_owner_user_id, 0), IFNULL(su.real_name, ''),
			IFNULL(i.endpoint_desc, ''), IFNULL(i.remark_text, ''), i.created_at, i.updated_at
		FROM project_integration_record i
		LEFT JOIN sys_user su ON su.id = i.internal_owner_user_id AND su.is_deleted = 0
	` + where + ` ORDER BY i.updated_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return model.PagedResult[model.IntegrationRecord]{}, err
	}
	defer rows.Close()
	list := make([]model.IntegrationRecord, 0)
	for rows.Next() {
		item, err := scanIntegrationRows(rows)
		if err != nil {
			return model.PagedResult[model.IntegrationRecord]{}, err
		}
		list = append(list, *item)
	}
	return pagedResult(list, filter.Page, filter.PageSize, total), rows.Err()
}

func (s *MySQLStore) GetIntegration(ctx context.Context, id int64) (*model.IntegrationRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			i.id, i.integration_no, i.project_id, i.external_system_name, i.integration_type,
			IFNULL(i.integration_direction, ''), i.content_desc, i.joint_status,
			IFNULL(i.external_owner, ''), IFNULL(i.internal_owner_user_id, 0), IFNULL(su.real_name, ''),
			IFNULL(i.endpoint_desc, ''), IFNULL(i.remark_text, ''), i.created_at, i.updated_at
		FROM project_integration_record i
		LEFT JOIN sys_user su ON su.id = i.internal_owner_user_id AND su.is_deleted = 0
		WHERE i.id = ? AND i.is_deleted = 0
		LIMIT 1
	`, id)
	item, err := scanIntegrationRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func (s *MySQLStore) CreateIntegration(ctx context.Context, item *model.IntegrationRecord) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO project_integration_record (
			integration_no, project_id, external_system_name, integration_type, integration_direction,
			content_desc, joint_status, external_owner, internal_owner_user_id, endpoint_desc,
			remark_text, created_at, updated_at, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), 0)
	`,
		item.IntegrationNo, item.ProjectID, item.ExternalSystemName, item.IntegrationType, nullString(item.IntegrationDirection),
		item.ContentDesc, item.JointStatus, nullString(item.ExternalOwner), nullableInt(item.InternalOwnerUserID),
		nullString(item.EndpointDesc), nullString(item.RemarkText),
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	saved, err := s.GetIntegration(ctx, id)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) UpdateIntegration(ctx context.Context, item *model.IntegrationRecord) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE project_integration_record
		SET external_system_name = ?, integration_type = ?, integration_direction = ?,
		    content_desc = ?, joint_status = ?, external_owner = ?, internal_owner_user_id = ?,
		    endpoint_desc = ?, remark_text = ?, updated_at = NOW()
		WHERE id = ? AND is_deleted = 0
	`,
		item.ExternalSystemName, item.IntegrationType, nullString(item.IntegrationDirection),
		item.ContentDesc, item.JointStatus, nullString(item.ExternalOwner), nullableInt(item.InternalOwnerUserID),
		nullString(item.EndpointDesc), nullString(item.RemarkText), item.ID,
	)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return store.ErrNotFound
	}
	saved, err := s.GetIntegration(ctx, item.ID)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) DeleteIntegration(ctx context.Context, id int64) error {
	return softDeleteByID(ctx, s.db, "project_integration_record", id)
}

func (s *MySQLStore) ListAssets(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.ScriptAsset], error) {
	where, args := recordListWhere("a", projectID, filter, []string{"asset_no", "asset_name", "deploy_path"})
	total, err := queryCount(ctx, s.db, "SELECT COUNT(1) FROM project_script_asset a "+where, args...)
	if err != nil {
		return model.PagedResult[model.ScriptAsset]{}, err
	}
	limit, offset := paginateClause(filter.Page, filter.PageSize)
	query := `
		SELECT
			a.id, a.asset_no, a.project_id, IFNULL(a.related_upgrade_id, 0), a.asset_name, a.asset_type,
			IFNULL(a.deploy_path, ''), a.purpose_desc, IFNULL(a.execute_command, ''),
			IFNULL(a.rollback_method, ''), IFNULL(a.test_result, ''), a.changed_by, IFNULL(su.real_name, ''),
			a.changed_at, IFNULL(a.remark_text, ''), a.created_at, a.updated_at
		FROM project_script_asset a
		LEFT JOIN sys_user su ON su.id = a.changed_by AND su.is_deleted = 0
	` + where + ` ORDER BY a.changed_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return model.PagedResult[model.ScriptAsset]{}, err
	}
	defer rows.Close()
	list := make([]model.ScriptAsset, 0)
	for rows.Next() {
		item, err := scanAssetRows(rows)
		if err != nil {
			return model.PagedResult[model.ScriptAsset]{}, err
		}
		list = append(list, *item)
	}
	return pagedResult(list, filter.Page, filter.PageSize, total), rows.Err()
}

func (s *MySQLStore) GetAsset(ctx context.Context, id int64) (*model.ScriptAsset, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			a.id, a.asset_no, a.project_id, IFNULL(a.related_upgrade_id, 0), a.asset_name, a.asset_type,
			IFNULL(a.deploy_path, ''), a.purpose_desc, IFNULL(a.execute_command, ''),
			IFNULL(a.rollback_method, ''), IFNULL(a.test_result, ''), a.changed_by, IFNULL(su.real_name, ''),
			a.changed_at, IFNULL(a.remark_text, ''), a.created_at, a.updated_at
		FROM project_script_asset a
		LEFT JOIN sys_user su ON su.id = a.changed_by AND su.is_deleted = 0
		WHERE a.id = ? AND a.is_deleted = 0
		LIMIT 1
	`, id)
	item, err := scanAssetRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func (s *MySQLStore) CreateAsset(ctx context.Context, item *model.ScriptAsset) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO project_script_asset (
			asset_no, project_id, related_upgrade_id, asset_name, asset_type, deploy_path,
			purpose_desc, execute_command, rollback_method, test_result, changed_by, changed_at,
			remark_text, created_at, updated_at, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), 0)
	`,
		item.AssetNo, item.ProjectID, nullableInt(item.RelatedUpgradeID), item.AssetName, item.AssetType,
		nullString(item.DeployPath), item.PurposeDesc, nullString(item.ExecuteCommand), nullString(item.RollbackMethod),
		nullString(item.TestResult), item.ChangedBy, item.ChangedAt, nullString(item.RemarkText),
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	saved, err := s.GetAsset(ctx, id)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) UpdateAsset(ctx context.Context, item *model.ScriptAsset) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE project_script_asset
		SET related_upgrade_id = ?, asset_name = ?, asset_type = ?, deploy_path = ?,
		    purpose_desc = ?, execute_command = ?, rollback_method = ?, test_result = ?,
		    changed_by = ?, changed_at = ?, remark_text = ?, updated_at = NOW()
		WHERE id = ? AND is_deleted = 0
	`,
		nullableInt(item.RelatedUpgradeID), item.AssetName, item.AssetType, nullString(item.DeployPath),
		item.PurposeDesc, nullString(item.ExecuteCommand), nullString(item.RollbackMethod), nullString(item.TestResult),
		item.ChangedBy, item.ChangedAt, nullString(item.RemarkText), item.ID,
	)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return store.ErrNotFound
	}
	saved, err := s.GetAsset(ctx, item.ID)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) DeleteAsset(ctx context.Context, id int64) error {
	return softDeleteByID(ctx, s.db, "project_script_asset", id)
}

func (s *MySQLStore) ListServiceRecords(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.ServiceRecord], error) {
	where, args := recordListWhere("sr", projectID, filter, []string{"service_no", "summary"})
	total, err := queryCount(ctx, s.db, "SELECT COUNT(1) FROM project_service_record sr "+where, args...)
	if err != nil {
		return model.PagedResult[model.ServiceRecord]{}, err
	}
	limit, offset := paginateClause(filter.Page, filter.PageSize)
	query := `
		SELECT
			sr.id, sr.service_no, sr.project_id, IFNULL(p.project_name, ''), IFNULL(p.customer_name, ''), IFNULL(sr.related_upgrade_id, 0), sr.service_type,
			IFNULL(sr.service_mode, ''), sr.service_date, sr.summary, IFNULL(sr.issue_version, ''), IFNULL(sr.problem_desc, ''),
			IFNULL(sr.process_desc, ''), IFNULL(sr.result_desc, ''), IFNULL(sr.next_action, ''),
			sr.owner_user_id, IFNULL(su.real_name, ''), IFNULL(sr.remark_text, ''),
			sr.created_at, sr.updated_at
		FROM project_service_record sr
		LEFT JOIN project p ON p.id = sr.project_id AND p.is_deleted = 0
		LEFT JOIN sys_user su ON su.id = sr.owner_user_id AND su.is_deleted = 0
	` + where + ` ORDER BY sr.service_date DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return model.PagedResult[model.ServiceRecord]{}, err
	}
	defer rows.Close()
	list := make([]model.ServiceRecord, 0)
	for rows.Next() {
		item, err := scanServiceRows(rows)
		if err != nil {
			return model.PagedResult[model.ServiceRecord]{}, err
		}
		list = append(list, *item)
	}
	return pagedResult(list, filter.Page, filter.PageSize, total), rows.Err()
}

func (s *MySQLStore) ListIssueRecords(ctx context.Context, filter model.IssueFilter) (model.PagedResult[model.ServiceRecord], error) {
	clauses := []string{"sr.is_deleted = 0", "sr.service_type = 'incident'", "p.is_deleted = 0"}
	args := make([]any, 0)
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		clauses = append(clauses, "(sr.service_no LIKE ? OR sr.summary LIKE ? OR IFNULL(sr.problem_desc, '') LIKE ? OR p.project_name LIKE ? OR p.customer_name LIKE ?)")
		args = append(args, like, like, like, like, like)
	}
	if version := strings.TrimSpace(filter.IssueVersion); version != "" {
		args = append(args, "%"+version+"%")
		clauses = append(clauses, "IFNULL(sr.issue_version, '') LIKE ?")
	}
	where := "WHERE " + strings.Join(clauses, " AND ")
	total, err := queryCount(ctx, s.db, "SELECT COUNT(1) FROM project_service_record sr INNER JOIN project p ON p.id = sr.project_id "+where, args...)
	if err != nil {
		return model.PagedResult[model.ServiceRecord]{}, err
	}
	limit, offset := paginateClause(filter.Page, filter.PageSize)
	query := `
		SELECT
			sr.id, sr.service_no, sr.project_id, IFNULL(p.project_name, ''), IFNULL(p.customer_name, ''), IFNULL(sr.related_upgrade_id, 0), sr.service_type,
			IFNULL(sr.service_mode, ''), sr.service_date, sr.summary, IFNULL(sr.issue_version, ''), IFNULL(sr.problem_desc, ''),
			IFNULL(sr.process_desc, ''), IFNULL(sr.result_desc, ''), IFNULL(sr.next_action, ''),
			sr.owner_user_id, IFNULL(su.real_name, ''), IFNULL(sr.remark_text, ''),
			sr.created_at, sr.updated_at
		FROM project_service_record sr
		INNER JOIN project p ON p.id = sr.project_id
		LEFT JOIN sys_user su ON su.id = sr.owner_user_id AND su.is_deleted = 0
	` + where + ` ORDER BY sr.service_date DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return model.PagedResult[model.ServiceRecord]{}, err
	}
	defer rows.Close()
	list := make([]model.ServiceRecord, 0)
	for rows.Next() {
		item, err := scanServiceRows(rows)
		if err != nil {
			return model.PagedResult[model.ServiceRecord]{}, err
		}
		list = append(list, *item)
	}
	return pagedResult(list, filter.Page, filter.PageSize, total), rows.Err()
}

func (s *MySQLStore) GetServiceRecord(ctx context.Context, id int64) (*model.ServiceRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			sr.id, sr.service_no, sr.project_id, IFNULL(p.project_name, ''), IFNULL(p.customer_name, ''), IFNULL(sr.related_upgrade_id, 0), sr.service_type,
			IFNULL(sr.service_mode, ''), sr.service_date, sr.summary, IFNULL(sr.issue_version, ''), IFNULL(sr.problem_desc, ''),
			IFNULL(sr.process_desc, ''), IFNULL(sr.result_desc, ''), IFNULL(sr.next_action, ''),
			sr.owner_user_id, IFNULL(su.real_name, ''), IFNULL(sr.remark_text, ''),
			sr.created_at, sr.updated_at
		FROM project_service_record sr
		LEFT JOIN project p ON p.id = sr.project_id AND p.is_deleted = 0
		LEFT JOIN sys_user su ON su.id = sr.owner_user_id AND su.is_deleted = 0
		WHERE sr.id = ? AND sr.is_deleted = 0
		LIMIT 1
	`, id)
	item, err := scanServiceRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func (s *MySQLStore) CreateServiceRecord(ctx context.Context, item *model.ServiceRecord) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO project_service_record (
			service_no, project_id, related_upgrade_id, service_type, service_mode, service_date,
			summary, issue_version, problem_desc, process_desc, result_desc, next_action, owner_user_id,
			remark_text, created_at, updated_at, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), 0)
	`,
		item.ServiceNo, item.ProjectID, nullableInt(item.RelatedUpgradeID), item.ServiceType,
		nullString(item.ServiceMode), item.ServiceDate, item.Summary, nullString(item.IssueVersion), nullString(item.ProblemDesc),
		nullString(item.ProcessDesc), nullString(item.ResultDesc), nullString(item.NextAction),
		item.OwnerUserID, nullString(item.RemarkText),
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	saved, err := s.GetServiceRecord(ctx, id)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) UpdateServiceRecord(ctx context.Context, item *model.ServiceRecord) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE project_service_record
		SET related_upgrade_id = ?, service_type = ?, service_mode = ?, service_date = ?,
		    summary = ?, issue_version = ?, problem_desc = ?, process_desc = ?, result_desc = ?, next_action = ?,
		    owner_user_id = ?, remark_text = ?, updated_at = NOW()
		WHERE id = ? AND is_deleted = 0
	`,
		nullableInt(item.RelatedUpgradeID), item.ServiceType, nullString(item.ServiceMode), item.ServiceDate,
		item.Summary, nullString(item.IssueVersion), nullString(item.ProblemDesc), nullString(item.ProcessDesc), nullString(item.ResultDesc),
		nullString(item.NextAction), item.OwnerUserID, nullString(item.RemarkText), item.ID,
	)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return store.ErrNotFound
	}
	saved, err := s.GetServiceRecord(ctx, item.ID)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) DeleteServiceRecord(ctx context.Context, id int64) error {
	return softDeleteByID(ctx, s.db, "project_service_record", id)
}

func recordListWhere(alias string, projectID int64, filter model.ListFilter, keywordColumns []string) (string, []any) {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	clauses := []string{prefix + "is_deleted = 0", prefix + "project_id = ?"}
	args := []any{projectID}
	if filter.Keyword != "" && len(keywordColumns) > 0 {
		sub := make([]string, 0, len(keywordColumns))
		like := "%" + strings.TrimSpace(filter.Keyword) + "%"
		for _, column := range keywordColumns {
			if !strings.Contains(column, ".") {
				column = prefix + column
			}
			sub = append(sub, column+" LIKE ?")
			args = append(args, like)
		}
		clauses = append(clauses, "("+strings.Join(sub, " OR ")+")")
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func pagedResult[T any](list []T, page, pageSize, total int) model.PagedResult[T] {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return model.PagedResult[T]{
		List:     list,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}
}

func softDeleteByID(ctx context.Context, db *sql.DB, table string, id int64) error {
	query := fmt.Sprintf("UPDATE %s SET is_deleted = 1, updated_at = NOW() WHERE id = ? AND is_deleted = 0", table)
	result, err := db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return store.ErrNotFound
	}
	return nil
}

func scanUpgradeRow(row *sql.Row) (*model.UpgradeRecord, error) {
	var item model.UpgradeRecord
	if err := row.Scan(
		&item.ID, &item.UpgradeNo, &item.ProjectID, &item.UpgradeDate, &item.SourceVersion, &item.TargetVersion,
		&item.UpgradeStatus, &item.OwnerUserID, &item.OwnerName, &item.CustomRetention, &item.IssueSolution,
		&item.TestResult, &item.RemarkText, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanUpgradeRows(rows *sql.Rows) (*model.UpgradeRecord, error) {
	var item model.UpgradeRecord
	if err := rows.Scan(
		&item.ID, &item.UpgradeNo, &item.ProjectID, &item.UpgradeDate, &item.SourceVersion, &item.TargetVersion,
		&item.UpgradeStatus, &item.OwnerUserID, &item.OwnerName, &item.CustomRetention, &item.IssueSolution,
		&item.TestResult, &item.RemarkText, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanConfigRow(row *sql.Row) (*model.ConfigChange, error) {
	var item model.ConfigChange
	if err := row.Scan(
		&item.ID, &item.ConfigNo, &item.ProjectID, &item.RelatedUpgradeID, &item.EffectiveVersion,
		&item.ConfigPath, &item.ChangeReason, &item.BeforeContent, &item.AfterContent,
		&item.TestResult, &item.ChangedBy, &item.ChangedByName, &item.ChangedAt,
		&item.RemarkText, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanConfigRows(rows *sql.Rows) (*model.ConfigChange, error) {
	var item model.ConfigChange
	if err := rows.Scan(
		&item.ID, &item.ConfigNo, &item.ProjectID, &item.RelatedUpgradeID, &item.EffectiveVersion,
		&item.ConfigPath, &item.ChangeReason, &item.BeforeContent, &item.AfterContent,
		&item.TestResult, &item.ChangedBy, &item.ChangedByName, &item.ChangedAt,
		&item.RemarkText, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanSQLRow(row *sql.Row) (*model.SQLChange, error) {
	var item model.SQLChange
	if err := row.Scan(
		&item.ID, &item.SQLNo, &item.ProjectID, &item.RelatedUpgradeID, &item.EffectiveVersion,
		&item.ChangeTitle, &item.DBObjects, &item.ChangeReason, &item.ChangeSQL, &item.RollbackSQL,
		&item.TestResult, &item.ChangedBy, &item.ChangedByName, &item.ChangedAt,
		&item.RemarkText, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanSQLRows(rows *sql.Rows) (*model.SQLChange, error) {
	var item model.SQLChange
	if err := rows.Scan(
		&item.ID, &item.SQLNo, &item.ProjectID, &item.RelatedUpgradeID, &item.EffectiveVersion,
		&item.ChangeTitle, &item.DBObjects, &item.ChangeReason, &item.ChangeSQL, &item.RollbackSQL,
		&item.TestResult, &item.ChangedBy, &item.ChangedByName, &item.ChangedAt,
		&item.RemarkText, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanIntegrationRow(row *sql.Row) (*model.IntegrationRecord, error) {
	var item model.IntegrationRecord
	if err := row.Scan(
		&item.ID, &item.IntegrationNo, &item.ProjectID, &item.ExternalSystemName, &item.IntegrationType,
		&item.IntegrationDirection, &item.ContentDesc, &item.JointStatus, &item.ExternalOwner,
		&item.InternalOwnerUserID, &item.InternalOwnerName, &item.EndpointDesc,
		&item.RemarkText, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanIntegrationRows(rows *sql.Rows) (*model.IntegrationRecord, error) {
	var item model.IntegrationRecord
	if err := rows.Scan(
		&item.ID, &item.IntegrationNo, &item.ProjectID, &item.ExternalSystemName, &item.IntegrationType,
		&item.IntegrationDirection, &item.ContentDesc, &item.JointStatus, &item.ExternalOwner,
		&item.InternalOwnerUserID, &item.InternalOwnerName, &item.EndpointDesc,
		&item.RemarkText, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAssetRow(row *sql.Row) (*model.ScriptAsset, error) {
	var item model.ScriptAsset
	if err := row.Scan(
		&item.ID, &item.AssetNo, &item.ProjectID, &item.RelatedUpgradeID, &item.AssetName, &item.AssetType,
		&item.DeployPath, &item.PurposeDesc, &item.ExecuteCommand, &item.RollbackMethod,
		&item.TestResult, &item.ChangedBy, &item.ChangedByName, &item.ChangedAt,
		&item.RemarkText, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAssetRows(rows *sql.Rows) (*model.ScriptAsset, error) {
	var item model.ScriptAsset
	if err := rows.Scan(
		&item.ID, &item.AssetNo, &item.ProjectID, &item.RelatedUpgradeID, &item.AssetName, &item.AssetType,
		&item.DeployPath, &item.PurposeDesc, &item.ExecuteCommand, &item.RollbackMethod,
		&item.TestResult, &item.ChangedBy, &item.ChangedByName, &item.ChangedAt,
		&item.RemarkText, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanServiceRow(row *sql.Row) (*model.ServiceRecord, error) {
	var item model.ServiceRecord
	if err := row.Scan(
		&item.ID, &item.ServiceNo, &item.ProjectID, &item.ProjectName, &item.CustomerName, &item.RelatedUpgradeID, &item.ServiceType,
		&item.ServiceMode, &item.ServiceDate, &item.Summary, &item.IssueVersion, &item.ProblemDesc,
		&item.ProcessDesc, &item.ResultDesc, &item.NextAction, &item.OwnerUserID,
		&item.OwnerName, &item.RemarkText, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanServiceRows(rows *sql.Rows) (*model.ServiceRecord, error) {
	var item model.ServiceRecord
	if err := rows.Scan(
		&item.ID, &item.ServiceNo, &item.ProjectID, &item.ProjectName, &item.CustomerName, &item.RelatedUpgradeID, &item.ServiceType,
		&item.ServiceMode, &item.ServiceDate, &item.Summary, &item.IssueVersion, &item.ProblemDesc,
		&item.ProcessDesc, &item.ResultDesc, &item.NextAction, &item.OwnerUserID,
		&item.OwnerName, &item.RemarkText, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}
