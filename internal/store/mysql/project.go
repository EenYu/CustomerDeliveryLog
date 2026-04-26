package mysql

import (
	"context"
	"database/sql"
	"strings"

	"customerdeliverylog/internal/model"
	"customerdeliverylog/internal/store"
)

func (s *MySQLStore) ListProjects(ctx context.Context, filter model.ProjectFilter) (model.PagedResult[model.Project], error) {
	clauses := []string{"p.is_deleted = 0"}
	args := make([]any, 0)
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		clauses = append(clauses, "(p.project_name LIKE ? OR p.customer_name LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	if filter.CustomerName != "" {
		clauses = append(clauses, "p.customer_name LIKE ?")
		args = append(args, "%"+filter.CustomerName+"%")
	}
	if filter.ProjectStatus != "" {
		clauses = append(clauses, "p.project_status = ?")
		args = append(args, filter.ProjectStatus)
	}
	if filter.CurrentVersion != "" {
		clauses = append(clauses, "p.current_version = ?")
		args = append(args, filter.CurrentVersion)
	}
	where := buildWhere("WHERE 1=1", clauses)
	countQuery := "SELECT COUNT(1) FROM project p " + where
	total, err := queryCount(ctx, s.db, countQuery, args...)
	if err != nil {
		return model.PagedResult[model.Project]{}, err
	}
	limit, offset := paginateClause(filter.Page, filter.PageSize)
	query := `
		SELECT
			p.id, p.project_code, p.project_name, p.customer_name, p.project_status,
			DATE_FORMAT(p.implementation_date, '%Y-%m-%d') AS implementation_date,
			IFNULL(DATE_FORMAT(p.online_date, '%Y-%m-%d'), '') AS online_date,
			IFNULL(DATE_FORMAT(p.acceptance_date, '%Y-%m-%d'), '') AS acceptance_date,
			p.current_version, IFNULL(p.owner_user_id, 0), IFNULL(u.real_name, ''),
			IFNULL(p.deploy_mode, ''), IFNULL(p.environment_summary, ''),
			IFNULL(p.customer_contact, ''), IFNULL(p.remark_text, ''),
			p.last_upgrade_at, p.last_change_at, p.archived_at,
			p.created_at, p.updated_at
		FROM project p
		LEFT JOIN sys_user u ON u.id = p.owner_user_id AND u.is_deleted = 0
	` + where + `
		ORDER BY p.updated_at DESC
		LIMIT ? OFFSET ?
	`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return model.PagedResult[model.Project]{}, err
	}
	defer rows.Close()
	list := make([]model.Project, 0)
	for rows.Next() {
		item, err := scanProjectRows(rows)
		if err != nil {
			return model.PagedResult[model.Project]{}, err
		}
		list = append(list, *item)
	}
	if err := rows.Err(); err != nil {
		return model.PagedResult[model.Project]{}, err
	}
	return model.PagedResult[model.Project]{
		List:     list,
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
	}, nil
}

func (s *MySQLStore) GetProject(ctx context.Context, projectID int64) (*model.Project, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			p.id, p.project_code, p.project_name, p.customer_name, p.project_status,
			DATE_FORMAT(p.implementation_date, '%Y-%m-%d') AS implementation_date,
			IFNULL(DATE_FORMAT(p.online_date, '%Y-%m-%d'), '') AS online_date,
			IFNULL(DATE_FORMAT(p.acceptance_date, '%Y-%m-%d'), '') AS acceptance_date,
			p.current_version, IFNULL(p.owner_user_id, 0), IFNULL(u.real_name, ''),
			IFNULL(p.deploy_mode, ''), IFNULL(p.environment_summary, ''),
			IFNULL(p.customer_contact, ''), IFNULL(p.remark_text, ''),
			p.last_upgrade_at, p.last_change_at, p.archived_at,
			p.created_at, p.updated_at
		FROM project p
		LEFT JOIN sys_user u ON u.id = p.owner_user_id AND u.is_deleted = 0
		WHERE p.id = ? AND p.is_deleted = 0
		LIMIT 1
	`, projectID)
	item, err := scanProjectRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func (s *MySQLStore) CreateProject(ctx context.Context, project *model.Project) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO project (
			project_code, project_name, customer_name, project_status, implementation_date,
			online_date, acceptance_date, current_version, owner_user_id, deploy_mode, environment_summary,
			customer_contact, remark_text, last_upgrade_at, last_change_at,
			archived_at, created_at, updated_at, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), 0)
	`,
		project.ProjectCode,
		project.ProjectName,
		project.CustomerName,
		project.ProjectStatus,
		project.ImplementationDate,
		nullString(project.OnlineDate),
		nullString(project.AcceptanceDate),
		project.CurrentVersion,
		nullableInt(project.OwnerUserID),
		nullString(project.DeployMode),
		nullString(project.EnvironmentSummary),
		nullString(project.CustomerContact),
		nullString(project.RemarkText),
		project.LastUpgradeAt,
		project.LastChangeAt,
		project.ArchivedAt,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	saved, err := s.GetProject(ctx, id)
	if err != nil {
		return err
	}
	*project = *saved
	return nil
}

func (s *MySQLStore) UpdateProject(ctx context.Context, project *model.Project) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE project
		SET
			project_name = ?,
			customer_name = ?,
			project_status = ?,
			implementation_date = ?,
			online_date = ?,
			acceptance_date = ?,
			current_version = ?,
			owner_user_id = ?,
			deploy_mode = ?,
			environment_summary = ?,
			customer_contact = ?,
			remark_text = ?,
			last_upgrade_at = ?,
			last_change_at = ?,
			archived_at = ?,
			updated_at = NOW()
		WHERE id = ? AND is_deleted = 0
	`,
		project.ProjectName,
		project.CustomerName,
		project.ProjectStatus,
		project.ImplementationDate,
		nullString(project.OnlineDate),
		nullString(project.AcceptanceDate),
		project.CurrentVersion,
		nullableInt(project.OwnerUserID),
		nullString(project.DeployMode),
		nullString(project.EnvironmentSummary),
		nullString(project.CustomerContact),
		nullString(project.RemarkText),
		project.LastUpgradeAt,
		project.LastChangeAt,
		project.ArchivedAt,
		project.ID,
	)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		if _, err := s.GetProject(ctx, project.ID); err != nil {
			return err
		}
	}
	saved, err := s.GetProject(ctx, project.ID)
	if err != nil {
		return err
	}
	*project = *saved
	return nil
}

func (s *MySQLStore) ArchiveProject(ctx context.Context, projectID int64, _ int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE project
		SET project_status = 'archived', archived_at = NOW(), updated_at = NOW()
		WHERE id = ? AND is_deleted = 0
	`, projectID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *MySQLStore) DeleteProject(ctx context.Context, projectID int64) error {
	return execTx(ctx, s.db, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			UPDATE project
			SET is_deleted = 1, updated_at = NOW()
			WHERE id = ? AND is_deleted = 0
		`, projectID)
		if err != nil {
			return err
		}
		if rows, _ := result.RowsAffected(); rows == 0 {
			return store.ErrNotFound
		}

		for _, table := range []string{
			"project_upgrade_record",
			"project_config_change",
			"project_sql_change",
			"project_integration_record",
			"project_script_asset",
			"project_service_record",
			"project_attachment",
		} {
			if _, err := tx.ExecContext(ctx, `
				UPDATE `+table+`
				SET is_deleted = 1, updated_at = NOW()
				WHERE project_id = ? AND is_deleted = 0
			`, projectID); err != nil {
				return err
			}
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM sys_audit_log WHERE project_id = ?`, projectID); err != nil {
			return err
		}
		return nil
	})
}

func scanProjectRow(row *sql.Row) (*model.Project, error) {
	var item model.Project
	var ownerName sql.NullString
	var deployMode sql.NullString
	var envSummary sql.NullString
	var contact sql.NullString
	var remark sql.NullString
	var lastUpgrade sql.NullTime
	var lastChange sql.NullTime
	var archivedAt sql.NullTime
	if err := row.Scan(
		&item.ID,
		&item.ProjectCode,
		&item.ProjectName,
		&item.CustomerName,
		&item.ProjectStatus,
		&item.ImplementationDate,
		&item.OnlineDate,
		&item.AcceptanceDate,
		&item.CurrentVersion,
		&item.OwnerUserID,
		&ownerName,
		&deployMode,
		&envSummary,
		&contact,
		&remark,
		&lastUpgrade,
		&lastChange,
		&archivedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.OwnerName = stringOrEmpty(ownerName)
	item.DeployMode = stringOrEmpty(deployMode)
	item.EnvironmentSummary = stringOrEmpty(envSummary)
	item.CustomerContact = stringOrEmpty(contact)
	item.RemarkText = stringOrEmpty(remark)
	item.LastUpgradeAt = timePtr(lastUpgrade)
	item.LastChangeAt = timePtr(lastChange)
	item.ArchivedAt = timePtr(archivedAt)
	return &item, nil
}

func scanProjectRows(rows *sql.Rows) (*model.Project, error) {
	var item model.Project
	var ownerName sql.NullString
	var deployMode sql.NullString
	var envSummary sql.NullString
	var contact sql.NullString
	var remark sql.NullString
	var lastUpgrade sql.NullTime
	var lastChange sql.NullTime
	var archivedAt sql.NullTime
	if err := rows.Scan(
		&item.ID,
		&item.ProjectCode,
		&item.ProjectName,
		&item.CustomerName,
		&item.ProjectStatus,
		&item.ImplementationDate,
		&item.OnlineDate,
		&item.AcceptanceDate,
		&item.CurrentVersion,
		&item.OwnerUserID,
		&ownerName,
		&deployMode,
		&envSummary,
		&contact,
		&remark,
		&lastUpgrade,
		&lastChange,
		&archivedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.OwnerName = stringOrEmpty(ownerName)
	item.DeployMode = stringOrEmpty(deployMode)
	item.EnvironmentSummary = stringOrEmpty(envSummary)
	item.CustomerContact = stringOrEmpty(contact)
	item.RemarkText = stringOrEmpty(remark)
	item.LastUpgradeAt = timePtr(lastUpgrade)
	item.LastChangeAt = timePtr(lastChange)
	item.ArchivedAt = timePtr(archivedAt)
	return &item, nil
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
