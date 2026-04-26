package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

type MySQLStore struct {
	db *sql.DB
}

func New(dsn string) (*MySQLStore, error) {
	cfg, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ParseTime = true
	cfg.Loc = time.Local
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	if _, ok := cfg.Params["charset"]; !ok {
		cfg.Params["charset"] = "utf8mb4"
	}
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	if err := ensureUserUsernameUniqueIndex(db, cfg.DBName); err != nil {
		return nil, err
	}
	if err := ensureProjectSchema(db, cfg.DBName); err != nil {
		return nil, err
	}
	if err := ensureServiceRecordSchema(db, cfg.DBName); err != nil {
		return nil, err
	}
	return &MySQLStore{db: db}, nil
}

type tableIndex struct {
	Name    string
	Unique  bool
	Columns []string
}

func ensureUserUsernameUniqueIndex(db *sql.DB, dbName string) error {
	if strings.TrimSpace(dbName) == "" {
		return nil
	}
	indexes, err := loadTableIndexes(db, dbName, "sys_user")
	if err != nil {
		return err
	}
	dropIndexes, addComposite := planUserUsernameIndexMigration(indexes)
	for _, indexName := range dropIndexes {
		if _, err := db.Exec(fmt.Sprintf("ALTER TABLE sys_user DROP INDEX %s", quoteIdentifier(indexName))); err != nil {
			return err
		}
	}
	if addComposite {
		_, err := db.Exec(`
			ALTER TABLE sys_user
			ADD UNIQUE KEY uk_username_deleted (username, is_deleted)
		`)
		return err
	}
	return nil
}

func ensureServiceRecordSchema(db *sql.DB, dbName string) error {
	if strings.TrimSpace(dbName) == "" {
		return nil
	}
	var count int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ?
		  AND TABLE_NAME = 'project_service_record'
		  AND COLUMN_NAME = 'issue_version'
	`, dbName).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := db.Exec(`
		ALTER TABLE project_service_record
		ADD COLUMN issue_version VARCHAR(50) DEFAULT NULL COMMENT '问题发生版本' AFTER summary,
		ADD KEY idx_service_issue_version (issue_version)
	`)
	return err
}

func ensureProjectSchema(db *sql.DB, dbName string) error {
	if strings.TrimSpace(dbName) == "" {
		return nil
	}
	exists, err := hasColumn(db, dbName, "project", "online_date")
	if err != nil {
		return err
	}
	if !exists {
		if _, err := db.Exec(`
			ALTER TABLE project
			ADD COLUMN online_date DATE DEFAULT NULL COMMENT 'online date' AFTER implementation_date
		`); err != nil {
			return err
		}
	}
	exists, err = hasColumn(db, dbName, "project", "acceptance_date")
	if err != nil {
		return err
	}
	if !exists {
		if _, err := db.Exec(`
			ALTER TABLE project
			ADD COLUMN acceptance_date DATE DEFAULT NULL COMMENT 'acceptance date' AFTER online_date
		`); err != nil {
			return err
		}
	}
	return nil
}

func hasColumn(db *sql.DB, dbName, tableName, columnName string) (bool, error) {
	var count int
	if err := db.QueryRow(`
		SELECT COUNT(1)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ?
		  AND TABLE_NAME = ?
		  AND COLUMN_NAME = ?
	`, dbName, tableName, columnName).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func loadTableIndexes(db *sql.DB, dbName, tableName string) ([]tableIndex, error) {
	rows, err := db.Query(`
		SELECT INDEX_NAME, NON_UNIQUE, COLUMN_NAME
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ?
		  AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX
	`, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orderedNames := make([]string, 0)
	indexMap := make(map[string]*tableIndex)
	for rows.Next() {
		var name string
		var nonUnique int
		var column string
		if err := rows.Scan(&name, &nonUnique, &column); err != nil {
			return nil, err
		}
		meta, ok := indexMap[name]
		if !ok {
			meta = &tableIndex{
				Name:    name,
				Unique:  nonUnique == 0,
				Columns: make([]string, 0, 2),
			}
			indexMap[name] = meta
			orderedNames = append(orderedNames, name)
		}
		meta.Columns = append(meta.Columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	indexes := make([]tableIndex, 0, len(orderedNames))
	for _, name := range orderedNames {
		indexes = append(indexes, *indexMap[name])
	}
	return indexes, nil
}

func planUserUsernameIndexMigration(indexes []tableIndex) ([]string, bool) {
	dropIndexes := make([]string, 0)
	hasComposite := false
	for _, index := range indexes {
		if !index.Unique {
			continue
		}
		switch {
		case hasIndexColumns(index.Columns, "username"):
			dropIndexes = append(dropIndexes, index.Name)
		case hasIndexColumns(index.Columns, "username", "is_deleted"):
			hasComposite = true
		}
	}
	sort.Strings(dropIndexes)
	return dropIndexes, len(dropIndexes) > 0 && !hasComposite
}

func hasIndexColumns(columns []string, expected ...string) bool {
	if len(columns) != len(expected) {
		return false
	}
	for i := range columns {
		if !strings.EqualFold(strings.TrimSpace(columns[i]), expected[i]) {
			return false
		}
	}
	return true
}

func quoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func stringOrEmpty(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func timePtr(value sql.NullTime) *time.Time {
	if value.Valid {
		t := value.Time
		return &t
	}
	return nil
}

func boolToTiny(value bool) int {
	if value {
		return 1
	}
	return 0
}

func tinyToBool(value int) bool {
	return value == 1
}

func buildWhere(base string, clauses []string) string {
	if len(clauses) == 0 {
		return base
	}
	return base + " AND " + strings.Join(clauses, " AND ")
}

func paginateClause(page, pageSize int) (limit, offset int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset = (page - 1) * pageSize
	return pageSize, offset
}

func queryCount(ctx context.Context, db *sql.DB, query string, args ...any) (int, error) {
	var total int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func execTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func placeholders(count int) string {
	if count <= 0 {
		return ""
	}
	items := make([]string, count)
	for i := range items {
		items[i] = "?"
	}
	return strings.Join(items, ",")
}

func queryRoleIDsByCodes(ctx context.Context, tx *sql.Tx, roleCodes []string) ([]int64, error) {
	if len(roleCodes) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(roleCodes))
	for _, code := range roleCodes {
		args = append(args, code)
	}
	query := fmt.Sprintf("SELECT id FROM sys_role WHERE is_deleted = 0 AND role_code IN (%s)", placeholders(len(roleCodes)))
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0, len(roleCodes))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
