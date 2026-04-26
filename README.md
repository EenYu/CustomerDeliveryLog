# 客户现场定制化记录系统

## 项目说明

这是一个面向软件交付、实施、运维和研发支持团队的内部管理后台，用于统一沉淀客户现场项目档案、版本升级、配置变更、SQL 变更、外部系统对接、脚本补丁、附件资料、服务记录和问题记录。

## 当前功能范围

1. 账号密码登录、用户管理
2. 项目档案列表、搜索、详情、删除
3. 升级记录 CRUD，并自动同步项目当前版本
4. 配置变更 CRUD
5. SQL 变更 CRUD
6. 外部系统对接 CRUD
7. 脚本 / 补丁 / 工具记录 CRUD
8. 服务记录 CRUD
9. 问题记录 CRUD，支持可选“问题发生版本”
10. 顶部“问题汇总”模块，自动汇总所有项目内的问题记录
11. 看板模块，支持按月份查看服务数、问题数、升级次数、现场版本分布、问题版本分布
12. 文档资料上传、预览、下载、删除
13. 备注区域支持直接粘贴截图，保存后自动归档到附件中
14. 操作日志留痕，记录真实姓名、中文对象类型、中文操作类型

## 技术栈

1. 后端：Go
2. 前端：HTML + CSS + 原生 JavaScript
3. 反向代理：Nginx
4. 数据库：MySQL 5.7+ / 8.0+
5. 文件存储：本地文件目录

## 本地运行

```powershell
$env:GOCACHE='D:\xiangmu\CustomerDeliveryLog\.gocache'
$env:GOMODCACHE='D:\xiangmu\CustomerDeliveryLog\.gomodcache'
$env:GOPATH='D:\xiangmu\CustomerDeliveryLog\.gopath'
& 'C:\Program Files\Go\bin\go.exe' run .\cmd\server
```

默认地址：

```text
http://127.0.0.1:8080
```

初始化管理员账号由环境变量控制：

1. `SEED_ADMIN_USERNAME`
2. `SEED_ADMIN_PASSWORD`

登录页不会默认展示或回填账号密码。

## 配置说明

1. 本地开发示例见 [app.env.example](/D:/xiangmu/CustomerDeliveryLog/app.env.example)
2. Linux 部署示例见 [deploy/linux/app.env.example](/D:/xiangmu/CustomerDeliveryLog/deploy/linux/app.env.example)
3. 使用 MySQL 时需要配置：

```bash
STORAGE_BACKEND=mysql
MYSQL_DSN="user:password@tcp(host:3306)/db?charset=utf8mb4&parseTime=true&loc=Local"
```

注意：

1. `MYSQL_DSN` 建议始终加双引号
2. 这样在 `bash` 中 `source` 配置文件时，不会因为括号或 `&` 被错误解析

## 测试

```powershell
.\scripts\test.ps1
```

本次“问题汇总”相关改动已覆盖：

1. 服务层问题记录汇总查询
2. HTTP 接口 `GET /api/v1/issues`
3. 问题版本筛选
4. 项目名称 / 客户名称透出

## 打包

Linux x86_64 / CentOS 7.9 发布包：

```powershell
.\scripts\build-linux.ps1
```

输出文件：

1. `dist/customer-delivery-log-linux-amd64.tar.gz`
2. `dist/customer-delivery-log-linux-amd64/customer-delivery-log/`

发布包目录结构：

```text
customer-delivery-log/
├── bin/
├── config/
├── docs/
├── logs/
├── run/
├── uploads/
├── web/
├── start.sh
├── stop.sh
└── restart.sh
```

## 远端运维

Windows 本地可直接使用仓库脚本统一执行远端启停、健康检查和 Nginx 重载：

```powershell
.\scripts\remote-ops.ps1 -Action sync-scripts -RemoteHost 192.168.203.131 -User root -Password root -RunAs gxp
.\scripts\remote-ops.ps1 -Action restart -RemoteHost 192.168.203.131 -User root -Password root -RunAs gxp
.\scripts\remote-ops.ps1 -Action health -RemoteHost 192.168.203.131 -User root -Password root -RunAs gxp
.\scripts\remote-ops.ps1 -Action nginx-reload -RemoteHost 192.168.203.131 -User root -Password root -RunAs gxp
```

## 相关文档

1. [产品文档目录](/D:/xiangmu/CustomerDeliveryLog/customer-delivery-record-system-docs/README.md)
2. [产品白皮书](/D:/xiangmu/CustomerDeliveryLog/customer-delivery-record-system-docs/01-product-whitepaper.md)
3. [PRD](/D:/xiangmu/CustomerDeliveryLog/customer-delivery-record-system-docs/02-prd.md)
4. [数据库设计 SQL](/D:/xiangmu/CustomerDeliveryLog/customer-delivery-record-system-docs/03-database-schema.sql)
5. [Go 后端接口清单](/D:/xiangmu/CustomerDeliveryLog/customer-delivery-record-system-docs/04-backend-api-go.md)
6. [页面原型说明](/D:/xiangmu/CustomerDeliveryLog/customer-delivery-record-system-docs/05-frontend-wireframes.md)
7. [CentOS 7.9 部署说明](/D:/xiangmu/CustomerDeliveryLog/DEPLOY_CENTOS7.md)
