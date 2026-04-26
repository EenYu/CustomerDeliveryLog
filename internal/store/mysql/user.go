package mysql

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"customerdeliverylog/internal/model"
	"customerdeliverylog/internal/store"
)

func (s *MySQLStore) SeedAdmin(ctx context.Context, username, realName, passwordHash string) error {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM sys_user WHERE username = ? AND is_deleted = 0", username).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return execTx(ctx, s.db, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO sys_user (
				username, real_name, password_hash, status, created_at, updated_at, is_deleted
			) VALUES (?, ?, ?, 1, NOW(), NOW(), 0)
		`, username, realName, passwordHash)
		if err != nil {
			return err
		}
		userID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		var roleID int64
		if err := tx.QueryRowContext(ctx, "SELECT id FROM sys_role WHERE role_code = 'admin' AND is_deleted = 0 LIMIT 1").Scan(&roleID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO sys_user_role (user_id, role_id, created_at)
			VALUES (?, ?, NOW())
		`, userID, roleID)
		return err
	})
}

func (s *MySQLStore) FindUserByUsername(ctx context.Context, username string) (*model.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			u.id,
			u.username,
			u.real_name,
			u.password_hash,
			u.status,
			u.created_at,
			u.updated_at,
			IFNULL(GROUP_CONCAT(r.role_code ORDER BY r.id SEPARATOR ','), '')
		FROM sys_user u
		LEFT JOIN sys_user_role ur ON ur.user_id = u.id
		LEFT JOIN sys_role r ON r.id = ur.role_id AND r.is_deleted = 0
		WHERE u.username = ? AND u.is_deleted = 0
		GROUP BY u.id, u.username, u.real_name, u.password_hash, u.status, u.created_at, u.updated_at
		LIMIT 1
	`, username)
	item, err := scanUserRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func (s *MySQLStore) GetUser(ctx context.Context, id int64) (*model.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			u.id,
			u.username,
			u.real_name,
			u.password_hash,
			u.status,
			u.created_at,
			u.updated_at,
			IFNULL(GROUP_CONCAT(r.role_code ORDER BY r.id SEPARATOR ','), '')
		FROM sys_user u
		LEFT JOIN sys_user_role ur ON ur.user_id = u.id
		LEFT JOIN sys_role r ON r.id = ur.role_id AND r.is_deleted = 0
		WHERE u.id = ? AND u.is_deleted = 0
		GROUP BY u.id, u.username, u.real_name, u.password_hash, u.status, u.created_at, u.updated_at
		LIMIT 1
	`, id)
	item, err := scanUserRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func (s *MySQLStore) ListUsers(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			u.id,
			u.username,
			u.real_name,
			u.password_hash,
			u.status,
			u.created_at,
			u.updated_at,
			IFNULL(GROUP_CONCAT(r.role_code ORDER BY r.id SEPARATOR ','), '')
		FROM sys_user u
		LEFT JOIN sys_user_role ur ON ur.user_id = u.id
		LEFT JOIN sys_role r ON r.id = ur.role_id AND r.is_deleted = 0
		WHERE u.is_deleted = 0
		GROUP BY u.id, u.username, u.real_name, u.password_hash, u.status, u.created_at, u.updated_at
		ORDER BY u.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]model.User, 0)
	for rows.Next() {
		item, err := scanUserRows(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *item)
	}
	return list, rows.Err()
}

func (s *MySQLStore) CreateUser(ctx context.Context, user *model.User) error {
	return execTx(ctx, s.db, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO sys_user (
				username, real_name, password_hash, status, created_at, updated_at, is_deleted
			) VALUES (?, ?, ?, ?, NOW(), NOW(), 0)
		`, user.Username, user.RealName, user.PasswordHash, boolToTiny(user.Status))
		if err != nil {
			return err
		}
		userID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		roleIDs, err := queryRoleIDsByCodes(ctx, tx, user.Roles)
		if err != nil {
			return err
		}
		for _, roleID := range roleIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO sys_user_role (user_id, role_id, created_at)
				VALUES (?, ?, NOW())
			`, userID, roleID); err != nil {
				return err
			}
		}
		saved, err := getUserByIDTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		*user = *saved
		return nil
	})
}

func (s *MySQLStore) UpdateUser(ctx context.Context, user *model.User) error {
	return execTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			UPDATE sys_user
			SET real_name = ?, status = ?, updated_at = NOW()
			WHERE id = ? AND is_deleted = 0
		`, user.RealName, boolToTiny(user.Status), user.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sys_user_role WHERE user_id = ?`, user.ID); err != nil {
			return err
		}
		roleIDs, err := queryRoleIDsByCodes(ctx, tx, user.Roles)
		if err != nil {
			return err
		}
		for _, roleID := range roleIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO sys_user_role (user_id, role_id, created_at)
				VALUES (?, ?, NOW())
			`, user.ID, roleID); err != nil {
				return err
			}
		}
		saved, err := getUserByIDTx(ctx, tx, user.ID)
		if err != nil {
			return err
		}
		*user = *saved
		return nil
	})
}

func (s *MySQLStore) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE sys_user
		SET password_hash = ?, pwd_changed_at = NOW(), updated_at = NOW()
		WHERE id = ? AND is_deleted = 0
	`, passwordHash, userID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *MySQLStore) DeleteUser(ctx context.Context, userID int64) error {
	return execTx(ctx, s.db, func(tx *sql.Tx) error {
		var username string
		if err := tx.QueryRowContext(ctx, `
			SELECT username
			FROM sys_user
			WHERE id = ? AND is_deleted = 0
			FOR UPDATE
		`, userID).Scan(&username); err != nil {
			if err == sql.ErrNoRows {
				return store.ErrNotFound
			}
			return err
		}
		archivedUsername := deletedUsername(username, userID)
		result, err := tx.ExecContext(ctx, `
			UPDATE sys_user
			SET username = ?, is_deleted = 1, updated_at = NOW()
			WHERE id = ? AND is_deleted = 0
		`, archivedUsername, userID)
		if err != nil {
			return err
		}
		if rows, _ := result.RowsAffected(); rows == 0 {
			return store.ErrNotFound
		}
		return nil
	})
}

func (s *MySQLStore) CreateLoginLog(ctx context.Context, log *model.LoginLog) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO sys_login_log (
			user_id, username, login_result, login_message, login_ip, user_agent, login_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, nullableInt(log.UserID), log.Username, log.LoginResult, log.LoginMessage, log.LoginIP, log.UserAgent, log.LoginAt)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	log.ID = id
	return nil
}

func (s *MySQLStore) ListLoginLogs(ctx context.Context) ([]model.LoginLog, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, IFNULL(user_id, 0), username, login_result, IFNULL(login_message, ''), IFNULL(login_ip, ''), IFNULL(user_agent, ''), login_at
		FROM sys_login_log
		ORDER BY login_at DESC
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]model.LoginLog, 0)
	for rows.Next() {
		var item model.LoginLog
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Username,
			&item.LoginResult,
			&item.LoginMessage,
			&item.LoginIP,
			&item.UserAgent,
			&item.LoginAt,
		); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func scanUserRow(row *sql.Row) (*model.User, error) {
	var item model.User
	var status int
	var rolesCSV string
	if err := row.Scan(
		&item.ID,
		&item.Username,
		&item.RealName,
		&item.PasswordHash,
		&status,
		&item.CreatedAt,
		&item.UpdatedAt,
		&rolesCSV,
	); err != nil {
		return nil, err
	}
	item.Status = tinyToBool(status)
	item.Roles = splitRoles(rolesCSV)
	return &item, nil
}

func getUserByIDTx(ctx context.Context, tx *sql.Tx, id int64) (*model.User, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT
			u.id,
			u.username,
			u.real_name,
			u.password_hash,
			u.status,
			u.created_at,
			u.updated_at,
			IFNULL(GROUP_CONCAT(r.role_code ORDER BY r.id SEPARATOR ','), '')
		FROM sys_user u
		LEFT JOIN sys_user_role ur ON ur.user_id = u.id
		LEFT JOIN sys_role r ON r.id = ur.role_id AND r.is_deleted = 0
		WHERE u.id = ? AND u.is_deleted = 0
		GROUP BY u.id, u.username, u.real_name, u.password_hash, u.status, u.created_at, u.updated_at
		LIMIT 1
	`, id)
	item, err := scanUserRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func scanUserRows(rows *sql.Rows) (*model.User, error) {
	var item model.User
	var status int
	var rolesCSV string
	if err := rows.Scan(
		&item.ID,
		&item.Username,
		&item.RealName,
		&item.PasswordHash,
		&status,
		&item.CreatedAt,
		&item.UpdatedAt,
		&rolesCSV,
	); err != nil {
		return nil, err
	}
	item.Status = tinyToBool(status)
	item.Roles = splitRoles(rolesCSV)
	return &item, nil
}

func splitRoles(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return []string{}
	}
	items := strings.Split(csv, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func nullableInt(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

func deletedUsername(username string, userID int64) string {
	suffix := "~del~" + strconv.FormatInt(userID, 10)
	maxBaseLen := 50 - len([]rune(suffix))
	if maxBaseLen < 0 {
		maxBaseLen = 0
	}
	baseRunes := []rune(username)
	if len(baseRunes) > maxBaseLen {
		baseRunes = baseRunes[:maxBaseLen]
	}
	return string(baseRunes) + suffix
}

func (s *MySQLStore) DashboardOverview(ctx context.Context, month string) (model.DashboardOverview, error) {
	var overview model.DashboardOverview
	selectedMonth, err := resolveDashboardMonth(month)
	if err != nil {
		return overview, err
	}
	overview.SelectedMonth = selectedMonth.Format("2006-01")
	start := time.Date(selectedMonth.Year(), selectedMonth.Month(), 1, 0, 0, 0, 0, selectedMonth.Location())
	end := start.AddDate(0, 1, 0)
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM project WHERE is_deleted = 0`).Scan(&overview.ProjectTotal); err != nil {
		return overview, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM project WHERE is_deleted = 0 AND project_status = 'maintenance'`).Scan(&overview.MaintenanceProjectNum); err != nil {
		return overview, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM project_upgrade_record u
		INNER JOIN project p ON p.id = u.project_id AND p.is_deleted = 0
		WHERE u.is_deleted = 0
		  AND u.upgrade_date >= ?
		  AND u.upgrade_date < ?
	`, start, end).Scan(&overview.MonthlyUpgradeNum); err != nil {
		return overview, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM project_service_record sr
		INNER JOIN project p ON p.id = sr.project_id AND p.is_deleted = 0
		WHERE sr.is_deleted = 0
		  AND sr.service_type <> 'incident'
		  AND sr.service_date >= ?
		  AND sr.service_date < ?
	`, start, end).Scan(&overview.MonthlyServiceNum); err != nil {
		return overview, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM project_service_record sr
		INNER JOIN project p ON p.id = sr.project_id AND p.is_deleted = 0
		WHERE sr.is_deleted = 0
		  AND sr.service_type = 'incident'
		  AND sr.service_date >= ?
		  AND sr.service_date < ?
	`, start, end).Scan(&overview.MonthlyIssueNum); err != nil {
		return overview, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM project p
		WHERE p.is_deleted = 0
		  AND NOT EXISTS (
		    SELECT 1 FROM project_attachment a
		    WHERE a.project_id = p.id AND a.is_deleted = 0
		  )
	`).Scan(&overview.MissingDocumentNum); err != nil {
		return overview, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT IFNULL(NULLIF(current_version, ''), '未填写') AS current_version, COUNT(1) AS project_count
		FROM project
		WHERE is_deleted = 0
		GROUP BY IFNULL(NULLIF(current_version, ''), '未填写')
		ORDER BY project_count DESC, current_version ASC
		LIMIT 20
	`)
	if err != nil {
		return overview, err
	}
	defer rows.Close()
	overview.VersionStats = make([]model.DashboardVersionStat, 0)
	for rows.Next() {
		var item model.DashboardVersionStat
		if err := rows.Scan(&item.CurrentVersion, &item.ProjectCount); err != nil {
			return overview, err
		}
		overview.VersionStats = append(overview.VersionStats, item)
	}
	if err := rows.Err(); err != nil {
		return overview, err
	}
	issueRows, err := s.db.QueryContext(ctx, `
		SELECT IFNULL(NULLIF(sr.issue_version, ''), '未填写版本') AS issue_version, COUNT(1) AS issue_count
		FROM project_service_record sr
		INNER JOIN project p ON p.id = sr.project_id AND p.is_deleted = 0
		WHERE sr.is_deleted = 0
		  AND sr.service_type = 'incident'
		GROUP BY IFNULL(NULLIF(sr.issue_version, ''), '未填写版本')
		ORDER BY issue_count DESC, issue_version ASC
		LIMIT 20
	`)
	if err != nil {
		return overview, err
	}
	defer issueRows.Close()
	overview.IssueVersionStats = make([]model.DashboardIssueVersionStat, 0)
	for issueRows.Next() {
		var item model.DashboardIssueVersionStat
		if err := issueRows.Scan(&item.IssueVersion, &item.IssueCount); err != nil {
			return overview, err
		}
		overview.IssueVersionStats = append(overview.IssueVersionStats, item)
	}
	if err := issueRows.Err(); err != nil {
		return overview, err
	}
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
