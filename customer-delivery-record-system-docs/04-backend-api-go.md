# Go 后端接口说明

## 1. 技术基线

### 1.1 当前实现

1. 语言：Go
2. HTTP：标准库 `net/http`
3. 认证：JWT access token + refresh token
4. 数据存储：MySQL 或内存实现
5. 文件存储：本地文件
6. 接口前缀：`/api/v1`

### 1.2 当前代码结构

1. 路由与鉴权：`internal/httpserver/server.go`
2. 业务服务：`internal/service/service.go`
3. 存储接口：`internal/store/store.go`
4. MySQL 实现：`internal/store/mysql`
5. 内存实现：`internal/store/memory`
6. 模型定义：`internal/model/model.go`

## 2. 通用约定

### 2.1 返回包结构

成功：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

失败：

```json
{
  "code": 400,
  "message": "错误信息"
}
```

### 2.2 鉴权方式

1. 登录成功后返回 `access_token` 和 `refresh_token`
2. 受保护接口使用 Header：`Authorization: Bearer <token>`
3. 附件预览在前端也支持通过 query 透传 `access_token`

### 2.3 请求规则

1. JSON 接口使用严格解码
2. 后端启用了 `DisallowUnknownFields`
3. 请求体出现未知字段会直接报错
4. 分页列表默认使用 `page`、`page_size`

### 2.4 角色说明

后端仍保留角色模型：

1. `admin`
2. `delivery_manager`
3. `engineer`
4. `rd_support`
5. `viewer`

真正权限以后端 `authz` 校验为准。

## 3. 权限矩阵

| 模块 | 查看 | 新增 / 编辑 | 删除 |
| --- | --- | --- | --- |
| 看板 | 登录即可 | - | - |
| 问题汇总 | 登录即可 | - | - |
| 项目档案 | 登录即可 | `admin/delivery_manager/engineer` | `admin` |
| 升级记录 | 登录即可 | `admin/delivery_manager/engineer/rd_support` | `admin/delivery_manager` |
| 配置变更 | 登录即可 | `admin/delivery_manager/engineer/rd_support` | `admin/delivery_manager` |
| SQL 变更 | 登录即可 | `admin/delivery_manager/engineer/rd_support` | `admin/delivery_manager` |
| 外部对接 | 登录即可 | `admin/delivery_manager/engineer/rd_support` | `admin/delivery_manager` |
| 脚本资产 | 登录即可 | `admin/delivery_manager/engineer/rd_support` | `admin/delivery_manager` |
| 服务与问题 | 登录即可 | `admin/delivery_manager/engineer/rd_support` | `admin/delivery_manager` |
| 附件 | 登录即可查看/预览/下载 | `admin/delivery_manager/engineer/rd_support` | `admin/delivery_manager` |
| 项目审计日志 | `admin/delivery_manager` | - | - |
| 全局审计日志 | `admin/delivery_manager` | - | - |
| 用户管理 | `admin` | `admin` | `admin` |
| 登录日志 | `admin` | - | - |

## 4. 认证相关接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/v1/auth/login` | 登录 |
| POST | `/api/v1/auth/refresh` | 刷新 access token |
| GET | `/api/v1/auth/me` | 获取当前用户 |
| POST | `/api/v1/auth/change-password` | 修改本人密码 |
| GET | `/api/v1/health` | 健康检查 |

### 4.1 登录请求

```json
{
  "username": "admin",
  "password": "Admin@123456"
}
```

### 4.2 登录响应核心字段

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "user": {
    "id": 1,
    "username": "admin",
    "real_name": "管理员",
    "roles": ["admin"]
  }
}
```

## 5. 看板与汇总接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/dashboard/overview` | 看板汇总 |
| GET | `/api/v1/issues` | 全局问题汇总 |

### 5.1 看板查询参数

| 参数 | 说明 |
| --- | --- |
| `month` | `YYYY-MM`，影响升级次数、服务次数、问题次数 |

### 5.2 问题汇总查询参数

| 参数 | 说明 |
| --- | --- |
| `keyword` | 按项目名、客户名、问题标题模糊查询 |
| `issue_version` | 问题发生版本 |
| `page` | 页码 |
| `page_size` | 每页条数 |

## 6. 用户与日志接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/users` | 用户列表 |
| POST | `/api/v1/users` | 新增用户 |
| PUT | `/api/v1/users/{id}` | 编辑用户 |
| DELETE | `/api/v1/users/{id}` | 删除用户 |
| POST | `/api/v1/users/{id}/reset-password` | 重置密码 |
| GET | `/api/v1/login-logs` | 登录日志 |
| GET | `/api/v1/audit-logs` | 全局审计日志 |

### 6.1 用户请求体说明

新增用户：

```json
{
  "username": "engineer01",
  "real_name": "张三",
  "password": "Abc123456",
  "roles": ["engineer"],
  "status": true
}
```

说明：

1. 后端支持 `roles`
2. 当前前端页面没有角色编辑入口
3. 如果前端不传 `roles`，用户可能只拥有基础可见能力

## 7. 项目接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/projects` | 项目列表 |
| POST | `/api/v1/projects` | 新建项目 |
| GET | `/api/v1/projects/{id}` | 项目详情 |
| PUT | `/api/v1/projects/{id}` | 编辑项目 |
| DELETE | `/api/v1/projects/{id}` | 删除项目 |
| PATCH | `/api/v1/projects/{id}/archive` | 归档项目 |
| GET | `/api/v1/projects/{id}/overview` | 项目概览聚合 |

### 7.1 项目列表查询参数

| 参数 | 说明 |
| --- | --- |
| `keyword` | 项目名称 / 客户名称 |
| `customer_name` | 客户名称 |
| `project_status` | 项目状态 |
| `current_version` | 当前版本 |
| `page` | 页码 |
| `page_size` | 每页条数 |

### 7.2 新建项目最小请求体

```json
{
  "project_name": "客户A现场",
  "customer_name": "客户A",
  "project_status": "implementing",
  "implementation_date": "2026-04-01",
  "current_version": "V1.0.0",
  "deploy_mode": "standalone",
  "customer_contact": "李四",
  "remark_text": "初始化建档"
}
```

说明：

1. `project_name`、`customer_name`、`implementation_date`、`current_version` 必填
2. `owner_user_id` 默认由服务层取当前用户
3. `project_code` 自动生成

## 8. 项目子模块接口

### 8.1 升级记录

| 方法 | 路径 |
| --- | --- |
| GET | `/api/v1/projects/{projectId}/upgrades` |
| POST | `/api/v1/projects/{projectId}/upgrades` |
| GET | `/api/v1/upgrades/{id}` |
| PUT | `/api/v1/upgrades/{id}` |
| DELETE | `/api/v1/upgrades/{id}` |

### 8.2 配置变更

| 方法 | 路径 |
| --- | --- |
| GET | `/api/v1/projects/{projectId}/config-changes` |
| POST | `/api/v1/projects/{projectId}/config-changes` |
| GET | `/api/v1/config-changes/{id}` |
| PUT | `/api/v1/config-changes/{id}` |
| DELETE | `/api/v1/config-changes/{id}` |

### 8.3 SQL 变更

| 方法 | 路径 |
| --- | --- |
| GET | `/api/v1/projects/{projectId}/sql-changes` |
| POST | `/api/v1/projects/{projectId}/sql-changes` |
| GET | `/api/v1/sql-changes/{id}` |
| PUT | `/api/v1/sql-changes/{id}` |
| DELETE | `/api/v1/sql-changes/{id}` |

### 8.4 外部对接

| 方法 | 路径 |
| --- | --- |
| GET | `/api/v1/projects/{projectId}/integrations` |
| POST | `/api/v1/projects/{projectId}/integrations` |
| GET | `/api/v1/integrations/{id}` |
| PUT | `/api/v1/integrations/{id}` |
| DELETE | `/api/v1/integrations/{id}` |

### 8.5 脚本资产

| 方法 | 路径 |
| --- | --- |
| GET | `/api/v1/projects/{projectId}/assets` |
| POST | `/api/v1/projects/{projectId}/assets` |
| GET | `/api/v1/assets/{id}` |
| PUT | `/api/v1/assets/{id}` |
| DELETE | `/api/v1/assets/{id}` |

### 8.6 服务与问题

| 方法 | 路径 |
| --- | --- |
| GET | `/api/v1/projects/{projectId}/service-records` |
| POST | `/api/v1/projects/{projectId}/service-records` |
| GET | `/api/v1/service-records/{id}` |
| PUT | `/api/v1/service-records/{id}` |
| DELETE | `/api/v1/service-records/{id}` |

说明：

1. 问题记录不是独立接口
2. 通过 `service_type=incident` 写入同一资源
3. 非 `incident` 类型时，后端会强制清空 `issue_version`

### 8.7 项目附件

| 方法 | 路径 |
| --- | --- |
| GET | `/api/v1/projects/{projectId}/attachments` |
| POST | `/api/v1/projects/{projectId}/attachments` |
| GET | `/api/v1/attachments/{id}` |
| PUT | `/api/v1/attachments/{id}` |
| DELETE | `/api/v1/attachments/{id}` |
| GET | `/api/v1/attachments/{id}/preview` |
| GET | `/api/v1/attachments/{id}/download` |

#### 8.7.1 上传方式

`multipart/form-data`

表单字段：

1. `file`
2. `title`
3. `doc_category`
4. `ref_type`
5. `ref_id`
6. `tags`
7. `description`

说明：

1. 后端要求 `title` 与 `doc_category` 必填
2. 当前使用本地文件存储

## 9. 审计接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/projects/{projectId}/audit-logs` | 项目级审计日志 |
| GET | `/api/v1/audit-logs` | 全局审计日志 |

## 10. 当前重要后端规则

1. 升级记录新增、编辑、删除后会同步项目当前版本。
2. 配置、SQL、对接、脚本、服务问题、附件都会刷新项目最近变更时间。
3. 编号统一由服务层自动生成，不接受前端手工提交业务编号。
4. 审计日志由服务层统一记录，包含对象类型、操作类型、摘要、前后快照、操作人真实姓名。
5. 备注截图会作为附件单独上传，不走富文本存储。

## 11. 联调注意事项

1. 请求体字段名必须和后端结构体完全一致。
2. 不要向不支持的接口字段提交额外字段，否则会因严格解码报错。
3. `related_upgrade_id` 虽然在模型中存在，但当前服务层会写 `0`，前端无需提交。
4. 用户管理接口仍支持 `roles`，但当前前端并未暴露。
