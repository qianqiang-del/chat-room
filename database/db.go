// Package database 提供 MySQL 数据库操作
package database

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// 消息存储最大容量，超过时自动丢弃最旧消息
const maxMessages = 1000

// ErrUserNotFound 用户不存在的错误
var ErrUserNotFound = errors.New("用户不存在")

// ErrDBInternal 数据库内部错误
//var ErrDBInternal = errors.New("数据库内部错误")

// PublicMessage 公共消息记录
type PublicMessage struct {
	Sender  string
	Content string
}

// PrivateMessage 私聊消息记录
type PrivateMessage struct {
	Sender  string
	Target  string
	Content string
}

// InitDB 初始化 MySQL 连接
func InitDB() error {
	dsn := "root:123456@tcp(127.0.0.1:3306)/chatroom?charset=utf8mb4&parseTime=true"
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	return db.Ping()
}

// CloseDB 关闭 MySQL 连接
func CloseDB() {
	if db != nil {
		_ = db.Close()
	}
}

// QueryPassword 查询用户密码，用户不存在时返回 ErrUserNotFound
func QueryPassword(nickname string) (string, error) {
	var storedPassword string
	err := db.QueryRow("SELECT password FROM users WHERE nickname = ?", nickname).Scan(&storedPassword)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", fmt.Errorf("查询密码失败: %w", err)
	}
	return storedPassword, nil
}

// CheckUserExists 检查用户是否已存在
func CheckUserExists(nickname string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE nickname = ?", nickname).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("查询用户失败: %w", err)
	}
	return count > 0, nil
}

// CreateUser 注册新用户
func CreateUser(nickname, password string) error {
	_, err := db.Exec("INSERT INTO users (nickname, password) VALUES (?, ?)", nickname, password)
	if err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	return nil
}

// UpdateLastActive 更新用户最后活跃时间
func UpdateLastActive(nickname string) {
	_, err := db.Exec("UPDATE users SET last_active = NOW() WHERE nickname = ?", nickname)
	if err != nil {
		fmt.Printf("更新用户活跃时间失败: %v\n", err)
	}
}

// SavePublicMessage 保存公共消息到数据库
func SavePublicMessage(sender, content string) error {
	_, err := db.Exec(`
		INSERT INTO messages (sender, content, msg_type, target)
		VALUES (?, ?, 'public', NULL)
	`, sender, content)
	if err != nil {
		return fmt.Errorf("保存公共消息失败: %w", err)
	}
	return nil
}

// SavePrivateMessage 保存私聊消息到数据库
func SavePrivateMessage(sender, content, target string) error {
	_, err := db.Exec(`
		INSERT INTO messages (sender, content, msg_type, target)
		VALUES (?, ?, 'private', ?)
	`, sender, content, target)
	if err != nil {
		return fmt.Errorf("保存私聊消息失败: %w", err)
	}
	return nil
}

// GetPublicHistory 获取公共历史消息（按时间正序）
func GetPublicHistory(limit int) ([]PublicMessage, error) {
	rows, err := db.Query(`
		SELECT sender, content
		FROM messages
		WHERE msg_type = 'public'
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("查询公共历史失败: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	var records []PublicMessage
	for rows.Next() {
		var m PublicMessage
		if err := rows.Scan(&m.Sender, &m.Content); err != nil {
			continue
		}
		records = append(records, m)
	}

	// 反转顺序（变成时间正序：最早的消息在前）
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// GetPrivateHistory 获取当前用户的私聊记录（按时间正序）
func GetPrivateHistory(nickname string) ([]PrivateMessage, error) {
	rows, err := db.Query(`
		SELECT sender, target, content FROM messages
		WHERE msg_type = 'private' AND (sender = ? OR target = ?)
		ORDER BY created_at ASC
	`, nickname, nickname)
	if err != nil {
		return nil, fmt.Errorf("查询私聊记录失败: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)
	var messages []PrivateMessage
	for rows.Next() {
		var m PrivateMessage
		if err := rows.Scan(&m.Sender, &m.Target, &m.Content); err != nil {
			continue
		}
		messages = append(messages, m)
	}

	return messages, nil
}

// EnforceMessageLimit 检查消息总数，超过 maxMessages 则删除最旧的消息
func EnforceMessageLimit() {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count)
	if err != nil {
		fmt.Printf("获取消息数量失败: %v\n", err)
		return
	}
	if count > maxMessages {
		deleteCount := count - maxMessages
		_, err = db.Exec("DELETE FROM messages ORDER BY created_at ASC LIMIT ?", deleteCount)
		if err != nil {
			fmt.Printf("清理旧消息失败: %v\n", err)
		} else {
			fmt.Printf("已达到消息上限，已清理 %d 条旧消息\n", deleteCount)
		}
	}
}
