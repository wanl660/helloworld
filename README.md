# helloworld

基于 Go 标准库 `net/http` 的 HTTP API 示例项目，使用 PostgreSQL 持久化用户数据。

## 技术栈

- Go 1.22+
- `net/http`（标准库路由）
- PostgreSQL 16
- [pgx/v5](https://github.com/jackc/pgx) 数据库驱动与连接池

## 项目结构

```
.
├── main.go                 # 服务入口、路由与 HTTP 处理器
├── config.yaml             # 默认配置文件
├── config.example.yaml     # 配置文件示例
├── internal/
│   ├── config/config.go    # 配置加载
│   ├── db/db.go            # 数据库连接与表迁移
│   └── store/user.go       # 用户数据访问层
├── docker-compose.yml      # 本地 PostgreSQL 环境
├── go.mod
└── go.sum
```

## 快速开始

### 前置要求

- Go 1.22 或更高版本
- Docker（用于启动 PostgreSQL）

### 1. 启动数据库

```bash
docker compose up -d
```

默认会创建数据库 `helloworld`，账号密码均为 `postgres`。

### 2. 配置服务

复制并编辑配置文件（可选，仓库已包含默认 `config.yaml`）：

```bash
cp config.example.yaml config.yaml
```

`config.yaml` 示例：

```yaml
server:
  port: 8080

postgres:
  host: localhost
  port: 5432
  user: postgres
  password: postgres
  database: helloworld
  sslmode: disable
```

### 3. 启动 API 服务

```bash
go run .
```

指定配置文件：

```bash
go run . -config /path/to/config.yaml
```

服务默认监听 `http://localhost:8080`。

### 4. 验证接口

```bash
# 健康检查
curl http://localhost:8080/health

# 问候接口
curl "http://localhost:8080/api/hello?name=Go"

# 创建用户
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com"}'

# 查询用户列表
curl http://localhost:8080/api/users
```

## 配置说明

配置文件为 YAML 格式，默认读取项目根目录下的 `config.yaml`。

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `server.port` | HTTP 服务监听端口 | `8080` |
| `postgres.host` | 数据库主机 | `localhost` |
| `postgres.port` | 数据库端口 | `5432` |
| `postgres.user` | 数据库用户名 | `postgres` |
| `postgres.password` | 数据库密码 | 空 |
| `postgres.database` | 数据库名 | `helloworld` |
| `postgres.sslmode` | SSL 模式 | `disable` |

也可通过以下方式指定配置文件路径：

- 命令行参数：`-config /path/to/config.yaml`
- 环境变量：`CONFIG_PATH=/path/to/config.yaml`

### 环境变量覆盖

以下环境变量可覆盖配置文件中的对应项（便于容器部署）：

| 变量 | 说明 |
|------|------|
| `PORT` | 覆盖 `server.port` |
| `DATABASE_URL` | 覆盖 PostgreSQL 连接串（优先级高于 `postgres.*` 字段） |

示例：

```bash
export PORT=9090
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/helloworld?sslmode=disable"
go run .
```

## API 文档

### 统一响应格式

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

### 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查，含数据库连通性检测 |
| GET | `/api/hello?name=xxx` | 问候接口，`name` 默认为 `World` |
| GET | `/api/users` | 获取用户列表 |
| POST | `/api/users` | 创建用户 |

### POST /api/users

请求体：

```json
{
  "name": "Alice",
  "email": "alice@example.com"
}
```

成功响应（201）：

```json
{
  "code": 0,
  "message": "created",
  "data": {
    "id": 1,
    "name": "Alice",
    "email": "alice@example.com",
    "created_at": "2026-06-30T07:06:56.231473Z"
  }
}
```

常见错误：

| HTTP 状态码 | 说明 |
|-------------|------|
| 400 | 请求体无效，或 `name` / `email` 缺失 |
| 409 | 邮箱已存在 |
| 500 | 服务器内部错误 |
| 503 | 数据库不可用（仅 `/health`） |

## 数据库

服务启动时会自动执行迁移，创建 `users` 表：

```sql
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## 构建与运行

```bash
# 编译
go build -o server .

# 运行二进制
./server
```

服务支持 `SIGINT` / `SIGTERM` 优雅关闭。

## 停止服务

```bash
# 停止 PostgreSQL
docker compose down

# 停止并删除数据卷
docker compose down -v
```
