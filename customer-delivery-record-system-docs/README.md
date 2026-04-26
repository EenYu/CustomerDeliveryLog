# 现场交付档案中心文档索引

## 说明

本目录文档已按当前源码重写，基线来源如下：

1. Go 后端路由与服务实现：`internal/httpserver`、`internal/service`、`internal/store`
2. 前端单页后台实现：`web/app.js`、`web/styles.css`
3. 当前交付包使用的 MySQL 表结构：`dist/.../config/sql/database-schema.sql`

这套文档不再复述最初的理想化需求，而是聚焦“当前系统已经实现了什么、页面怎么组织、字段怎么流转、接口和数据层怎么落地、有哪些已知边界”。

## 当前系统一句话定义

`现场交付档案中心` 是一个面向交付、实施、运维和研发支持团队的内部管理后台，以“项目档案”为中心，把升级、配置、SQL、对接、脚本、文档、服务、问题和审计记录统一沉淀到一个可检索、可追溯的系统中。

## 当前源码已实现能力

1. 账号密码登录、刷新令牌、个人改密
2. 项目档案列表、搜索、详情、删除
3. 升级记录 CRUD，且项目当前版本自动回写
4. 配置变更 CRUD
5. SQL 变更 CRUD
6. 外部系统对接 CRUD
7. 脚本 / 补丁 / 工具记录 CRUD
8. 服务记录与问题记录 CRUD
9. 顶部问题汇总模块，汇总所有项目问题
10. 看板模块，支持月份统计与版本分布
11. 文档资料上传、预览、下载、删除
12. 备注区域直接粘贴截图，保存后自动转附件
13. 项目级 / 全局操作日志
14. 用户管理、登录日志

## 当前页面结构

顶部一级导航：

1. 看板
2. 项目档案
3. 问题汇总
4. 用户管理

项目详情页一级 Tab：

1. 概览
2. 变更中心
3. 服务与问题
4. 外部对接
5. 文档资料
6. 操作日志

局部二级分类：

1. 变更中心：升级记录 / 配置变更 / SQL 变更 / 脚本补丁
2. 服务与问题：服务记录 / 问题记录

## 当前实现边界

1. 前端当前默认对已登录用户展示大部分操作入口，但真正的增删改权限以后端角色校验为准。
2. 后端仍保留角色模型与角色校验，前端用户管理页当前不提供角色编辑入口。
3. 多个业务表保留了 `related_upgrade_id` 字段，但当前服务层会统一置为 `0`，即当前版本未启用“关联升级记录”能力。
4. 新建项目弹窗当前不展示“环境说明”，该字段只能在编辑项目时补录。
5. 部署环境在前端实际使用的是 `standalone / cluster`，文档已按现状描述。

## 文档清单

1. [01-product-whitepaper.md](/D:/xiangmu/CustomerDeliveryLog/customer-delivery-record-system-docs/01-product-whitepaper.md)
   产品定位、目标用户、业务价值、边界与已知差异。
2. [02-prd.md](/D:/xiangmu/CustomerDeliveryLog/customer-delivery-record-system-docs/02-prd.md)
   当前实现版 PRD，覆盖页面结构、模块、字段、业务规则、交互和权限。
3. [03-database-schema.sql](/D:/xiangmu/CustomerDeliveryLog/customer-delivery-record-system-docs/03-database-schema.sql)
   基于当前源码整理的核心表结构 SQL。
4. [04-backend-api-go.md](/D:/xiangmu/CustomerDeliveryLog/customer-delivery-record-system-docs/04-backend-api-go.md)
   Go 后端接口说明、鉴权、请求规则、权限矩阵。
5. [05-frontend-wireframes.md](/D:/xiangmu/CustomerDeliveryLog/customer-delivery-record-system-docs/05-frontend-wireframes.md)
   按当前界面实现整理的页面结构与草图说明。

## 推荐阅读顺序

1. 先读白皮书，理解产品边界和当前实现定位
2. 再读 PRD，理解页面与业务规则
3. 再读接口与数据结构，进入研发和联调阶段
4. 最后看前端草图，便于设计、验收或二次改版
