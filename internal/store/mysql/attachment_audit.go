package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"customerdeliverylog/internal/model"
	"customerdeliverylog/internal/store"
)

func (s *MySQLStore) ListAttachments(ctx context.Context, projectID int64, filter model.AttachmentFilter) (model.PagedResult[model.Attachment], error) {
	clauses := []string{"a.is_deleted = 0", "a.project_id = ?"}
	args := []any{projectID}
	if filter.RefType != "" {
		clauses = append(clauses, "a.ref_type = ?")
		args = append(args, filter.RefType)
	}
	if filter.RefID > 0 {
		clauses = append(clauses, "a.ref_id = ?")
		args = append(args, filter.RefID)
	}
	if filter.DocCategory != "" {
		clauses = append(clauses, "a.doc_category = ?")
		args = append(args, filter.DocCategory)
	}
	if filter.ExcludeDocCategory != "" {
		clauses = append(clauses, "a.doc_category <> ?")
		args = append(args, filter.ExcludeDocCategory)
	}
	where := "WHERE " + stringsJoinAnd(clauses)
	total, err := queryCount(ctx, s.db, "SELECT COUNT(1) FROM project_attachment a "+where, args...)
	if err != nil {
		return model.PagedResult[model.Attachment]{}, err
	}
	limit, offset := paginateClause(filter.Page, filter.PageSize)
	query := `
		SELECT
			a.id, a.project_id, IFNULL(a.ref_type, 'project'), IFNULL(a.ref_id, 0), a.title,
			a.doc_category, a.file_name, a.original_name, a.file_ext, IFNULL(a.mime_type, ''),
			a.file_size, a.storage_type, a.relative_path, IFNULL(a.thumbnail_path, ''),
			IFNULL(a.tags, ''), IFNULL(a.description, ''), a.uploaded_by, IFNULL(u.real_name, ''),
			a.uploaded_at, a.created_at, a.updated_at
		FROM project_attachment a
		LEFT JOIN sys_user u ON u.id = a.uploaded_by AND u.is_deleted = 0
	` + where + ` ORDER BY a.uploaded_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return model.PagedResult[model.Attachment]{}, err
	}
	defer rows.Close()
	list := make([]model.Attachment, 0)
	for rows.Next() {
		item, err := scanAttachmentRows(rows)
		if err != nil {
			return model.PagedResult[model.Attachment]{}, err
		}
		list = append(list, *item)
	}
	return pagedResult(list, filter.Page, filter.PageSize, total), rows.Err()
}

func (s *MySQLStore) GetAttachment(ctx context.Context, id int64) (*model.Attachment, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			a.id, a.project_id, IFNULL(a.ref_type, 'project'), IFNULL(a.ref_id, 0), a.title,
			a.doc_category, a.file_name, a.original_name, a.file_ext, IFNULL(a.mime_type, ''),
			a.file_size, a.storage_type, a.relative_path, IFNULL(a.thumbnail_path, ''),
			IFNULL(a.tags, ''), IFNULL(a.description, ''), a.uploaded_by, IFNULL(u.real_name, ''),
			a.uploaded_at, a.created_at, a.updated_at
		FROM project_attachment a
		LEFT JOIN sys_user u ON u.id = a.uploaded_by AND u.is_deleted = 0
		WHERE a.id = ? AND a.is_deleted = 0
		LIMIT 1
	`, id)
	item, err := scanAttachmentRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return item, nil
}

func (s *MySQLStore) CreateAttachment(ctx context.Context, item *model.Attachment) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO project_attachment (
			project_id, ref_type, ref_id, title, doc_category, file_name, original_name, file_ext,
			mime_type, file_size, storage_type, relative_path, thumbnail_path, tags, description,
			uploaded_by, uploaded_at, created_at, updated_at, is_deleted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), 0)
	`,
		item.ProjectID, item.RefType, nullableInt(item.RefID), item.Title, item.DocCategory,
		item.FileName, item.OriginalName, item.FileExt, nullString(item.MimeType), item.FileSize,
		item.StorageType, item.RelativePath, nullString(item.ThumbnailPath), nullString(item.Tags),
		nullString(item.Description), item.UploadedBy, item.UploadedAt,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	saved, err := s.GetAttachment(ctx, id)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) UpdateAttachment(ctx context.Context, item *model.Attachment) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE project_attachment
		SET ref_type = ?, ref_id = ?, title = ?, doc_category = ?, tags = ?, description = ?, updated_at = NOW()
		WHERE id = ? AND is_deleted = 0
	`,
		item.RefType, nullableInt(item.RefID), item.Title, item.DocCategory, nullString(item.Tags),
		nullString(item.Description), item.ID,
	)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return store.ErrNotFound
	}
	saved, err := s.GetAttachment(ctx, item.ID)
	if err != nil {
		return err
	}
	*item = *saved
	return nil
}

func (s *MySQLStore) DeleteAttachment(ctx context.Context, id int64) error {
	return softDeleteByID(ctx, s.db, "project_attachment", id)
}

func (s *MySQLStore) CreateAuditLog(ctx context.Context, log *model.AuditLog) error {
	beforePayload := nullableJSON(log.BeforeSnapshot)
	afterPayload := nullableJSON(log.AfterSnapshot)
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO sys_audit_log (
			project_id, object_type, object_id, operation_type, operation_summary,
			before_snapshot, after_snapshot, operator_user_id, operated_at, ip_address, user_agent
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		nullableInt(log.ProjectID), log.ObjectType, log.ObjectID, log.OperationType,
		log.OperationSummary, beforePayload, afterPayload, log.OperatorUserID, log.OperatedAt,
		nullString(log.IPAddress), nullString(log.UserAgent),
	)
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

func (s *MySQLStore) ListAuditLogs(ctx context.Context, projectID int64, filter model.ListFilter) (model.PagedResult[model.AuditLog], error) {
	clauses := []string{"1=1"}
	args := make([]any, 0)
	if projectID > 0 {
		clauses = append(clauses, "a.project_id = ?")
		args = append(args, projectID)
	}
	if filter.Keyword != "" {
		clauses = append(clauses, "a.operation_summary LIKE ?")
		args = append(args, "%"+filter.Keyword+"%")
	}
	where := "WHERE " + stringsJoinAnd(clauses)
	total, err := queryCount(ctx, s.db, "SELECT COUNT(1) FROM sys_audit_log a "+where, args...)
	if err != nil {
		return model.PagedResult[model.AuditLog]{}, err
	}
	limit, offset := paginateClause(filter.Page, filter.PageSize)
	query := `
		SELECT
			a.id, IFNULL(a.project_id, 0), a.object_type, a.object_id, a.operation_type,
			a.operation_summary, IFNULL(CAST(a.before_snapshot AS CHAR), ''), IFNULL(CAST(a.after_snapshot AS CHAR), ''),
			a.operator_user_id, IFNULL(u.real_name, ''), a.operated_at, IFNULL(a.ip_address, ''), IFNULL(a.user_agent, '')
		FROM sys_audit_log a
		LEFT JOIN sys_user u ON u.id = a.operator_user_id AND u.is_deleted = 0
	` + where + ` ORDER BY a.operated_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return model.PagedResult[model.AuditLog]{}, err
	}
	defer rows.Close()
	list := make([]model.AuditLog, 0)
	for rows.Next() {
		var item model.AuditLog
		if err := rows.Scan(
			&item.ID, &item.ProjectID, &item.ObjectType, &item.ObjectID, &item.OperationType,
			&item.OperationSummary, &item.BeforeSnapshot, &item.AfterSnapshot, &item.OperatorUserID,
			&item.OperatorUserName, &item.OperatedAt, &item.IPAddress, &item.UserAgent,
		); err != nil {
			return model.PagedResult[model.AuditLog]{}, err
		}
		list = append(list, item)
	}
	return pagedResult(list, filter.Page, filter.PageSize, total), rows.Err()
}

func scanAttachmentRow(row *sql.Row) (*model.Attachment, error) {
	var item model.Attachment
	if err := row.Scan(
		&item.ID, &item.ProjectID, &item.RefType, &item.RefID, &item.Title, &item.DocCategory,
		&item.FileName, &item.OriginalName, &item.FileExt, &item.MimeType, &item.FileSize,
		&item.StorageType, &item.RelativePath, &item.ThumbnailPath, &item.Tags, &item.Description,
		&item.UploadedBy, &item.UploadedByName, &item.UploadedAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAttachmentRows(rows *sql.Rows) (*model.Attachment, error) {
	var item model.Attachment
	if err := rows.Scan(
		&item.ID, &item.ProjectID, &item.RefType, &item.RefID, &item.Title, &item.DocCategory,
		&item.FileName, &item.OriginalName, &item.FileExt, &item.MimeType, &item.FileSize,
		&item.StorageType, &item.RelativePath, &item.ThumbnailPath, &item.Tags, &item.Description,
		&item.UploadedBy, &item.UploadedByName, &item.UploadedAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func nullableJSON(raw string) any {
	if raw == "" {
		return nil
	}
	var temp any
	if err := json.Unmarshal([]byte(raw), &temp); err != nil {
		return raw
	}
	return raw
}

func stringsJoinAnd(items []string) string {
	return fmt.Sprintf("%s", joinStrings(items, " AND "))
}

func joinStrings(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	result := items[0]
	for i := 1; i < len(items); i++ {
		result += sep + items[i]
	}
	return result
}
