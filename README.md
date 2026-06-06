# 💬 Go 网络聊天室

一个基于 Go 语言原生 `net` 包实现的 TCP 聊天室，支持 MySQL 持久化存储和 Redis 缓存加速。

## ✨ 功能特点

- **用户认证** — 注册 / 登录，密码缓存至 Redis（5分钟过期）
- **公共聊天** — 消息实时广播给所有在线用户
- **私聊** — 输入 `/msg 用户名 消息内容` 发送私聊
- **在线用户** — 输入 `/online` 查看当前在线用户列表
- **历史消息** — 输入 `/history` 查看最近 20 条公共消息
- **私聊记录** — 输入 `/private` 查看自己的私聊历史
- **活跃度排行** — 输入 `/rank` 查看 TOP10 活跃用户
- **Redis Stream** — 消息异步持久化到 MySQL

## 🚀 快速开始

### 环境要求

- Go 1.16+
- MySQL 8.0+
- Redis

### 1. 克隆仓库

```bash
git clone https://github.com/qianqiang-del/chat-room.git
cd chat-room
```

### 2. 建表

在 MySQL 中创建数据库和表：

```sql
CREATE DATABASE IF NOT EXISTS chatroom;
USE chatroom;

CREATE TABLE users (
    nickname VARCHAR(50) PRIMARY KEY,
    password VARCHAR(255) NOT NULL,
    last_active DATETIME
);

CREATE TABLE messages (
    id INT AUTO_INCREMENT PRIMARY KEY,
    sender VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    msg_type ENUM('public','private') NOT NULL,
    target VARCHAR(50),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 3. 启动服务端

```bash
go run ./cmd/server/
```

服务端默认监听 `localhost:8080`，启动后会自动连接本地 MySQL 和 Redis。

### 4. 启动客户端

```bash
go run ./cmd/client/
```

按菜单提示选择登录或注册即可进入聊天室。

## 📁 项目结构

```
├── cmd/
│   ├── client/          # 客户端入口（菜单式交互）
│   └── server/          # 服务端入口（TCP 监听）
├── database/
│   ├── db.go            # MySQL 数据库操作
│   └── redis.go         # Redis 缓存 & Stream 消息队列
├── go.mod / go.sum      # Go 模块依赖
└── .gitignore
```

## 🔧 技术栈

| 技术 | 用途 |
|------|------|
| Go 标准库 `net` | TCP 网络通信 |
| MySQL | 用户数据 & 消息持久化 |
| Redis (go-redis) | 密码缓存、活跃度排行、Stream 消息队列 |

## 📝 客户端命令

| 命令 | 功能 |
|------|------|
| `/online` | 查看在线用户 |
| `/history` | 查看最近 20 条历史消息 |
| `/private` | 查看私聊记录 |
| `/rank` | 查看活跃度排行榜 |
| `/msg <用户> <内容>` | 发送私聊 |
| `/back` | 返回主菜单（聊天模式中） |
