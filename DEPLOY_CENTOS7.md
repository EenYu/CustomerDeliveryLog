# CentOS 7.9 部署说明

## 1. 适用范围

本文适用于以下环境：

1. Linux `x86_64`
2. CentOS `7.9`
3. 应用通过 `start.sh / stop.sh / restart.sh` 运维，不依赖 `systemd`
4. Nginx、MySQL 由运维单独安装和启动

## 2. 发布包目录结构

解压后的推荐目录如下：

```text
/home/gxp/customer-delivery-log
├── bin/
│   └── customer-delivery-log
├── config/
│   ├── app.env.example
│   ├── nginx/
│   │   └── customer-delivery-log.conf
│   └── sql/
│       └── database-schema.sql
├── docs/
├── logs/
├── run/
├── uploads/
├── web/
├── start.sh
├── stop.sh
└── restart.sh
```

说明：

1. `bin/` 放可执行程序
2. `config/` 放环境变量、Nginx 配置和数据库初始化 SQL
3. `logs/` 放运行日志
4. `run/` 放 PID 文件
5. `uploads/` 放本地上传附件
6. `web/` 放前端静态页面

## 3. MySQL 准备

### 3.1 创建数据库

```sql
CREATE DATABASE IF NOT EXISTS customer_delivery_log
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_general_ci;
```

### 3.2 创建账号并授权

```sql
CREATE USER IF NOT EXISTS 'app_user'@'%' IDENTIFIED BY 'app_password';
GRANT ALL PRIVILEGES ON customer_delivery_log.* TO 'app_user'@'%';
FLUSH PRIVILEGES;
```

### 3.3 导入表结构

```bash
mysql -u app_user -p customer_delivery_log < /home/gxp/customer-delivery-log/config/sql/database-schema.sql
```

## 4. 应用配置

复制环境变量模板：

```bash
cd /home/gxp/customer-delivery-log
cp config/app.env.example config/app.env
```

重点修改以下配置：

```bash
LISTEN_ADDR=:8080
STORAGE_BACKEND=mysql
MYSQL_DSN="app_user:app_password@tcp(127.0.0.1:3306)/customer_delivery_log?charset=utf8mb4&parseTime=true&loc=Local"
TOKEN_SECRET=请改成随机密钥
SEED_ADMIN_USERNAME=admin
SEED_ADMIN_PASSWORD=请改成你的管理员密码
UPLOAD_DIR=./uploads
WEB_DIR=./web
```

注意：

1. `MYSQL_DSN` 必须保留双引号
2. 这是因为 `source config/app.env` 时，DSN 中的括号和 `&` 如果不加引号，bash 可能误解析

## 5. 启动应用

```bash
cd /home/gxp/customer-delivery-log
chmod +x start.sh stop.sh restart.sh
chmod +x bin/customer-delivery-log
./start.sh
```

启动成功后：

1. 日志文件：`logs/app.out`
2. PID 文件：`run/app.pid`
3. 后端监听：`127.0.0.1:8080` 或 `0.0.0.0:8080`，取决于 `LISTEN_ADDR`

查看日志：

```bash
tail -f /home/gxp/customer-delivery-log/logs/app.out
```

停止：

```bash
cd /home/gxp/customer-delivery-log
./stop.sh
```

重启：

```bash
cd /home/gxp/customer-delivery-log
./restart.sh
```

如需从 Windows 统一执行远端运维，可使用：

```powershell
.\scripts\remote-ops.ps1 -Action restart -RemoteHost 192.168.203.131 -User root -Password root -RunAs gxp
```

## 6. Nginx 反向代理

将包内示例配置复制到 Nginx：

```bash
cp /home/gxp/customer-delivery-log/config/nginx/customer-delivery-log.conf /etc/nginx/conf.d/customer-delivery-log.conf
```

然后按实际目录修改 `root`，例如：

```nginx
server {
    listen 80;
    server_name _;

    root /home/gxp/customer-delivery-log/web;
    index index.html;

    client_max_body_size 100m;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

检查并重载：

```bash
nginx -t
nginx -s reload
```

## 7. 验证方式

### 7.1 后端健康检查

```bash
curl http://127.0.0.1:8080/api/v1/health
```

期望返回：

```json
{"code":0,"data":{"status":"ok"},"message":"success"}
```

### 7.2 页面访问

浏览器访问：

```text
http://服务器IP/login
```

登录账号使用 `config/app.env` 中设置的初始化管理员账号密码。

### 7.3 功能联调清单

部署后建议至少验证以下流程：

1. 顶部菜单可正常进入“项目档案”“看板”“问题汇总”“用户管理”
2. 在项目详情的“服务与问题 -> 问题记录”新增问题后，“问题汇总”能自动看到该记录
3. “问题汇总”支持按关键字、问题版本筛选
4. 从“问题汇总”点击“查看项目”可直接回到对应项目详情
5. 看板可看到按月份统计的问题数、服务数、升级次数和问题版本分布

## 8. 常见问题

### 8.1 页面能打开，但接口 502

排查：

1. `./start.sh` 或 `./restart.sh` 是否已成功启动
2. `logs/app.out` 是否有报错
3. Nginx `proxy_pass` 是否指向正确的 Go 监听地址

### 8.2 登录失败

排查：

1. 数据库表是否已导入
2. `STORAGE_BACKEND` 是否为 `mysql`
3. `MYSQL_DSN` 是否可连通
4. `SEED_ADMIN_PASSWORD` 是否与你登录密码一致

### 8.3 附件上传失败

排查：

1. `uploads/` 目录是否存在
2. 启动用户是否有写权限
3. Nginx `client_max_body_size` 是否足够大
