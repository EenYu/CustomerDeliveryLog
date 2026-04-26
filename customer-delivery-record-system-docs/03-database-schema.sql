-- 现场交付档案中心
-- 当前源码版核心表结构
-- 基线说明：
-- 1. 本文件按当前 Go 模型与 MySQL Store 行为整理。
-- 2. 仅保留当前运行期真正依赖的核心表，不再沿用早期未消费的字典扩展表。
-- 3. 所有业务删除默认采用逻辑删除 is_deleted。
-- 4. 备注截图通过 project_attachment 的 screenshot 分类落表。
-- 5. deploy_mode 当前前端实际取值为 standalone / cluster。

CREATE DATABASE IF NOT EXISTS customer_delivery_log
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_general_ci;

USE customer_delivery_log;

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS sys_user_role;
DROP TABLE IF EXISTS sys_login_log;
DROP TABLE IF EXISTS sys_audit_log;
DROP TABLE IF EXISTS project_attachment;
DROP TABLE IF EXISTS project_service_record;
DROP TABLE IF EXISTS project_script_asset;
DROP TABLE IF EXISTS project_integration_record;
DROP TABLE IF EXISTS project_sql_change;
DROP TABLE IF EXISTS project_config_change;
DROP TABLE IF EXISTS project_upgrade_record;
DROP TABLE IF EXISTS project;
DROP TABLE IF EXISTS sys_user;
DROP TABLE IF EXISTS sys_role;

CREATE TABLE sys_role (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  role_code VARCHAR(50) NOT NULL COMMENT '角色编码：admin/delivery_manager/engineer/rd_support/viewer',
  role_name VARCHAR(100) NOT NULL COMMENT '角色名称',
  role_desc VARCHAR(255) DEFAULT NULL COMMENT '角色描述',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1启用 0停用',
  is_system TINYINT NOT NULL DEFAULT 1 COMMENT '是否系统角色',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  is_deleted TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  PRIMARY KEY (id),
  UNIQUE KEY uk_role_code (role_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统角色表';

CREATE TABLE sys_user (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  username VARCHAR(50) NOT NULL COMMENT '登录账号',
  real_name VARCHAR(50) NOT NULL COMMENT '真实姓名',
  password_hash VARCHAR(255) NOT NULL COMMENT '密码哈希',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '1启用 0停用',
  failed_login_count INT NOT NULL DEFAULT 0 COMMENT '连续失败次数',
  locked_until DATETIME DEFAULT NULL COMMENT '锁定截止时间',
  pwd_changed_at DATETIME DEFAULT NULL COMMENT '最近改密时间',
  last_login_at DATETIME DEFAULT NULL COMMENT '最后登录时间',
  last_login_ip VARCHAR(64) DEFAULT NULL COMMENT '最后登录IP',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  is_deleted TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  PRIMARY KEY (id),
  UNIQUE KEY uk_username_deleted (username, is_deleted),
  KEY idx_real_name (real_name),
  KEY idx_user_status (status),
  KEY idx_user_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统用户表';

CREATE TABLE sys_user_role (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  role_id BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_user_role (user_id, role_id),
  KEY idx_role_id (role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户角色关联表';

CREATE TABLE sys_login_log (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  user_id BIGINT UNSIGNED DEFAULT NULL COMMENT '用户ID',
  username VARCHAR(50) NOT NULL COMMENT '登录账号',
  login_result VARCHAR(20) NOT NULL COMMENT 'success/fail',
  login_message VARCHAR(255) DEFAULT NULL COMMENT '结果说明',
  login_ip VARCHAR(64) DEFAULT NULL COMMENT '登录IP',
  user_agent VARCHAR(500) DEFAULT NULL COMMENT '浏览器信息',
  login_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '登录时间',
  PRIMARY KEY (id),
  KEY idx_login_user (user_id),
  KEY idx_login_at (login_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='登录日志表';

CREATE TABLE project (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  project_code VARCHAR(30) NOT NULL COMMENT '项目编号，系统自动生成 PRJ-...',
  project_name VARCHAR(200) NOT NULL COMMENT '项目名称',
  customer_name VARCHAR(200) NOT NULL COMMENT '客户名称',
  project_status VARCHAR(20) NOT NULL COMMENT 'implementing/online/maintenance/archived',
  implementation_date DATE NOT NULL COMMENT '实施日期',
  online_date DATE DEFAULT NULL COMMENT '上线日期',
  acceptance_date DATE DEFAULT NULL COMMENT '验收日期',
  current_version VARCHAR(50) NOT NULL COMMENT '当前版本',
  owner_user_id BIGINT UNSIGNED DEFAULT NULL COMMENT '负责人',
  deploy_mode VARCHAR(30) DEFAULT NULL COMMENT '部署环境：standalone/cluster',
  environment_summary TEXT DEFAULT NULL COMMENT '环境说明',
  customer_contact VARCHAR(100) DEFAULT NULL COMMENT '客户联系人',
  remark_text TEXT DEFAULT NULL COMMENT '备注',
  last_upgrade_at DATETIME DEFAULT NULL COMMENT '最近升级时间',
  last_change_at DATETIME DEFAULT NULL COMMENT '最近变更时间',
  archived_at DATETIME DEFAULT NULL COMMENT '归档时间',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  is_deleted TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  PRIMARY KEY (id),
  UNIQUE KEY uk_project_code (project_code),
  KEY idx_project_name (project_name),
  KEY idx_customer_name (customer_name),
  KEY idx_project_status (project_status),
  KEY idx_project_version (current_version),
  KEY idx_project_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目主档';

CREATE TABLE project_upgrade_record (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  upgrade_no VARCHAR(30) NOT NULL COMMENT '升级编号 UPG-...',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  upgrade_date DATETIME NOT NULL COMMENT '升级日期',
  source_version VARCHAR(50) NOT NULL COMMENT '原版本',
  target_version VARCHAR(50) NOT NULL COMMENT '目标版本',
  upgrade_status VARCHAR(20) NOT NULL COMMENT 'planned/completed/rolled_back',
  owner_user_id BIGINT UNSIGNED NOT NULL COMMENT '负责人',
  custom_retention VARCHAR(20) NOT NULL COMMENT 'all/partial/none',
  issue_solution MEDIUMTEXT DEFAULT NULL COMMENT '问题与方案',
  test_result TEXT DEFAULT NULL COMMENT '测试结果',
  remark_text TEXT DEFAULT NULL COMMENT '备注',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  is_deleted TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  PRIMARY KEY (id),
  UNIQUE KEY uk_upgrade_no (upgrade_no),
  KEY idx_upgrade_project (project_id),
  KEY idx_upgrade_date (upgrade_date),
  KEY idx_upgrade_status (upgrade_status),
  KEY idx_upgrade_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='升级记录表';

CREATE TABLE project_config_change (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  config_no VARCHAR(30) NOT NULL COMMENT '记录编号 CFG-...',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  related_upgrade_id BIGINT UNSIGNED DEFAULT NULL COMMENT '预留，当前服务层固定写0',
  effective_version VARCHAR(50) DEFAULT NULL COMMENT '生效版本',
  config_path VARCHAR(500) NOT NULL COMMENT '配置文件路径',
  change_reason VARCHAR(500) NOT NULL COMMENT '修改原因',
  before_content MEDIUMTEXT DEFAULT NULL COMMENT '原配置内容',
  after_content MEDIUMTEXT NOT NULL COMMENT '修改后配置内容',
  test_result TEXT DEFAULT NULL COMMENT '测试结果',
  changed_by BIGINT UNSIGNED NOT NULL COMMENT '修改人',
  changed_at DATETIME NOT NULL COMMENT '修改时间',
  remark_text TEXT DEFAULT NULL COMMENT '备注',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  is_deleted TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  PRIMARY KEY (id),
  UNIQUE KEY uk_config_no (config_no),
  KEY idx_config_project (project_id),
  KEY idx_config_changed_at (changed_at),
  KEY idx_config_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='配置变更表';

CREATE TABLE project_sql_change (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  sql_no VARCHAR(30) NOT NULL COMMENT '记录编号 SQL-...',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  related_upgrade_id BIGINT UNSIGNED DEFAULT NULL COMMENT '预留，当前服务层固定写0',
  effective_version VARCHAR(50) DEFAULT NULL COMMENT '生效版本',
  change_title VARCHAR(200) NOT NULL COMMENT '修改标题',
  db_objects VARCHAR(500) DEFAULT NULL COMMENT '数据库对象',
  change_reason VARCHAR(500) NOT NULL COMMENT '修改原因',
  change_sql MEDIUMTEXT NOT NULL COMMENT '变更SQL',
  rollback_sql MEDIUMTEXT DEFAULT NULL COMMENT '回退SQL',
  test_result TEXT DEFAULT NULL COMMENT '测试结果',
  changed_by BIGINT UNSIGNED NOT NULL COMMENT '修改人',
  changed_at DATETIME NOT NULL COMMENT '修改时间',
  remark_text TEXT DEFAULT NULL COMMENT '备注',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  is_deleted TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  PRIMARY KEY (id),
  UNIQUE KEY uk_sql_no (sql_no),
  KEY idx_sql_project (project_id),
  KEY idx_sql_changed_at (changed_at),
  KEY idx_sql_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SQL变更表';

CREATE TABLE project_integration_record (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  integration_no VARCHAR(30) NOT NULL COMMENT '记录编号 INT-...',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  external_system_name VARCHAR(200) NOT NULL COMMENT '对接系统名称',
  integration_type VARCHAR(30) NOT NULL COMMENT 'api/database/file/mq/manual',
  integration_direction VARCHAR(20) DEFAULT NULL COMMENT 'inbound/outbound/bidirectional',
  content_desc MEDIUMTEXT NOT NULL COMMENT '对接内容说明',
  joint_status VARCHAR(20) NOT NULL COMMENT 'pending/testing/online/disabled',
  external_owner VARCHAR(100) DEFAULT NULL COMMENT '外部负责人',
  internal_owner_user_id BIGINT UNSIGNED DEFAULT NULL COMMENT '内部负责人',
  endpoint_desc VARCHAR(500) DEFAULT NULL COMMENT '端点或对象说明',
  remark_text TEXT DEFAULT NULL COMMENT '备注',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  is_deleted TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  PRIMARY KEY (id),
  UNIQUE KEY uk_integration_no (integration_no),
  KEY idx_integration_project (project_id),
  KEY idx_integration_status (joint_status),
  KEY idx_integration_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='外部系统对接表';

CREATE TABLE project_script_asset (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  asset_no VARCHAR(30) NOT NULL COMMENT '记录编号 AST-...',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  related_upgrade_id BIGINT UNSIGNED DEFAULT NULL COMMENT '预留，当前服务层固定写0',
  asset_name VARCHAR(200) NOT NULL COMMENT '名称',
  asset_type VARCHAR(30) NOT NULL COMMENT 'script/tool/patch/temp_program',
  deploy_path VARCHAR(500) DEFAULT NULL COMMENT '部署位置',
  purpose_desc VARCHAR(500) NOT NULL COMMENT '用途说明',
  execute_command TEXT DEFAULT NULL COMMENT '执行方式',
  rollback_method TEXT DEFAULT NULL COMMENT '回退方式',
  test_result TEXT DEFAULT NULL COMMENT '测试结果',
  changed_by BIGINT UNSIGNED NOT NULL COMMENT '修改人',
  changed_at DATETIME NOT NULL COMMENT '修改时间',
  remark_text TEXT DEFAULT NULL COMMENT '备注',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  is_deleted TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  PRIMARY KEY (id),
  UNIQUE KEY uk_asset_no (asset_no),
  KEY idx_asset_project (project_id),
  KEY idx_asset_type (asset_type),
  KEY idx_asset_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='脚本补丁工具表';

CREATE TABLE project_service_record (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  service_no VARCHAR(30) NOT NULL COMMENT '记录编号 SRV-...',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  related_upgrade_id BIGINT UNSIGNED DEFAULT NULL COMMENT '预留，当前服务层固定写0',
  service_type VARCHAR(30) NOT NULL COMMENT 'support/implementation/inspection/training/incident',
  service_mode VARCHAR(20) DEFAULT NULL COMMENT 'remote/onsite',
  service_date DATETIME NOT NULL COMMENT '服务或问题时间',
  summary VARCHAR(200) NOT NULL COMMENT '主题摘要',
  issue_version VARCHAR(50) DEFAULT NULL COMMENT '仅问题记录使用',
  problem_desc MEDIUMTEXT DEFAULT NULL COMMENT '问题描述或现场情况',
  process_desc MEDIUMTEXT DEFAULT NULL COMMENT '处理过程',
  result_desc MEDIUMTEXT DEFAULT NULL COMMENT '处理结果',
  next_action TEXT DEFAULT NULL COMMENT '后续动作',
  owner_user_id BIGINT UNSIGNED NOT NULL COMMENT '负责人',
  remark_text TEXT DEFAULT NULL COMMENT '备注',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  is_deleted TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  PRIMARY KEY (id),
  UNIQUE KEY uk_service_no (service_no),
  KEY idx_service_project (project_id),
  KEY idx_service_type (service_type),
  KEY idx_service_issue_version (issue_version),
  KEY idx_service_date (service_date),
  KEY idx_service_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='服务与问题统一记录表';

CREATE TABLE project_attachment (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  project_id BIGINT UNSIGNED NOT NULL COMMENT '项目ID',
  ref_type VARCHAR(30) DEFAULT 'project' COMMENT 'project/upgrade/config/sql/integration/asset/service',
  ref_id BIGINT UNSIGNED DEFAULT NULL COMMENT '关联对象ID',
  title VARCHAR(200) NOT NULL COMMENT '附件标题',
  doc_category VARCHAR(30) NOT NULL COMMENT 'manual/deploy_doc/delivery_doc/arch_diagram/deploy_diagram/topology/screenshot/other',
  file_name VARCHAR(255) NOT NULL COMMENT '存储文件名',
  original_name VARCHAR(255) NOT NULL COMMENT '原始文件名',
  file_ext VARCHAR(20) NOT NULL COMMENT '扩展名',
  mime_type VARCHAR(100) DEFAULT NULL COMMENT 'MIME类型',
  file_size BIGINT UNSIGNED NOT NULL COMMENT '文件大小',
  storage_type VARCHAR(20) NOT NULL DEFAULT 'local' COMMENT '当前仅 local',
  relative_path VARCHAR(500) NOT NULL COMMENT '相对存储路径',
  thumbnail_path VARCHAR(500) DEFAULT NULL COMMENT '缩略图路径',
  tags VARCHAR(200) DEFAULT NULL COMMENT '标签，逗号分隔',
  description TEXT DEFAULT NULL COMMENT '文件说明',
  uploaded_by BIGINT UNSIGNED NOT NULL COMMENT '上传人',
  uploaded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '上传时间',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  is_deleted TINYINT NOT NULL DEFAULT 0 COMMENT '逻辑删除',
  PRIMARY KEY (id),
  KEY idx_attachment_project (project_id),
  KEY idx_attachment_ref (ref_type, ref_id),
  KEY idx_attachment_category (doc_category),
  KEY idx_attachment_uploaded_at (uploaded_at),
  KEY idx_attachment_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='附件表';

CREATE TABLE sys_audit_log (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  project_id BIGINT UNSIGNED DEFAULT NULL COMMENT '所属项目ID',
  object_type VARCHAR(30) NOT NULL COMMENT 'project/upgrade/config_change/sql_change/integration/asset/service_record/attachment/user',
  object_id BIGINT UNSIGNED NOT NULL COMMENT '对象ID',
  operation_type VARCHAR(30) NOT NULL COMMENT 'create/update/delete',
  operation_summary VARCHAR(500) NOT NULL COMMENT '操作摘要',
  before_snapshot JSON DEFAULT NULL COMMENT '操作前快照',
  after_snapshot JSON DEFAULT NULL COMMENT '操作后快照',
  operator_user_id BIGINT UNSIGNED NOT NULL COMMENT '操作人ID',
  operator_user_name VARCHAR(100) DEFAULT NULL COMMENT '操作人真实姓名',
  operated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '操作时间',
  ip_address VARCHAR(64) DEFAULT NULL COMMENT 'IP地址',
  user_agent VARCHAR(500) DEFAULT NULL COMMENT '浏览器信息',
  PRIMARY KEY (id),
  KEY idx_audit_project (project_id),
  KEY idx_audit_object (object_type, object_id),
  KEY idx_audit_operator (operator_user_id),
  KEY idx_audit_operated_at (operated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='审计日志表';

INSERT INTO sys_role (role_code, role_name, role_desc, status, is_system)
VALUES
('admin', '系统管理员', '平台与用户管理', 1, 1),
('delivery_manager', '交付负责人', '项目主档与关键记录维护', 1, 1),
('engineer', '实施运维工程师', '现场记录维护', 1, 1),
('rd_support', '研发支持', '技术支撑补录', 1, 1),
('viewer', '只读访客', '仅查看', 1, 1);

SET FOREIGN_KEY_CHECKS = 1;
