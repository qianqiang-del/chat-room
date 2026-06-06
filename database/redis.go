// Package database 提供 MySQL 和 Redis 数据库操作
package database

import (
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strings"
	"time"
)

var rdb *redis.Client
var ctx = context.Background()

// ActivityEntry 活跃度排行条目
type ActivityEntry struct {
	Nickname string
	Score    int64
}

// InitRedis 初始化 Redis 连接
func InitRedis(addr string) error {
	rdb = redis.NewClient(&redis.Options{
		Addr: addr,
	})
	err := rdb.Ping(ctx).Err()
	if err != nil {
		return fmt.Errorf("redis 连接失败: %w", err)
	}
	fmt.Printf("Redis 连接成功 (%s)\n", addr)
	return nil
}

// CloseRedis 关闭 Redis
func CloseRedis() {
	if rdb != nil {
		_ = rdb.Close()
		fmt.Println("Redis 连接已关闭")
	}
}

// IncrActivity 为用户增加活跃度分数（Sorted Set: user:activity）
func IncrActivity(nickname string, points int64) error {
	if rdb == nil {
		return fmt.Errorf("redis 未初始化")
	}
	return rdb.ZIncrBy(ctx, "user:activity", float64(points), nickname).Err()
}

// GetTopActivity 获取活跃度排行前 limit 名
func GetTopActivity(limit int) ([]ActivityEntry, error) {
	if rdb == nil {
		return nil, fmt.Errorf("redis 未初始化")
	}
	result, err := rdb.ZRevRangeWithScores(ctx, "user:activity", 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("查询活跃排行失败: %w", err)
	}
	var entries []ActivityEntry
	for _, z := range result {
		entries = append(entries, ActivityEntry{
			Nickname: z.Member.(string),
			Score:    int64(z.Score),
		})
	}
	return entries, nil
}

// EnsureStreamGroup 创建Stream消费者组
func EnsureStreamGroup() error {
	if rdb == nil {
		return fmt.Errorf("redis 未初始化")
	}
	err := rdb.XGroupCreateMkStream(ctx, "messages:stream", "chat-group", "$").Err()
	if err != nil {
		// 组已存在 → 不是错误，忽略
		if strings.Contains(err.Error(), "BUSYGROUP") {
			return nil
		}
		return err
	}
	return nil
}

// PushMessageToStream 将消息推送到Stream中
func PushMessageToStream(sender, content, msgType, target string) error {
	if rdb == nil {
		return fmt.Errorf("redis 未初始化")
	}
	err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "messages:stream",
		Values: map[string]interface{}{
			"sender":   sender,
			"content":  content,
			"msg_type": msgType,
			"target":   target,
		},
	}).Err()
	if err != nil {
		return fmt.Errorf("推送消息到Stream失败: %w", err)
	}
	return nil
}

func StartMessageConsumer() {
	if rdb == nil {
		fmt.Println("Redis 未初始化，无法启动消息消费者")
		return
	}
	fmt.Println("消息消费者已启动")
	for {
		streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    "chat-group",
			Consumer: "consumer-1",
			Streams:  []string{"messages:stream", ">"},
			Count:    10,
			Block:    30 * time.Second,
		}).Result()

		if err != nil {
			continue // 超时或错误，继续循环
		}
		for _, msg := range streams[0].Messages {
			sender := msg.Values["sender"].(string)
			content := msg.Values["content"].(string)
			msgType := msg.Values["msg_type"].(string)
			target := msg.Values["target"].(string)

			var saveErr error
			if msgType == "public" {
				saveErr = SavePublicMessage(sender, content)
			} else {
				saveErr = SavePrivateMessage(sender, content, target)
			}
			if saveErr != nil {
				fmt.Printf("消费者持久化消息失败: %v\n", saveErr)
				// 不 return，继续处理下一条
			} else {
				EnforceMessageLimit()
			}
			rdb.XAck(ctx, "messages:stream", "chat-group", msg.ID)
		}
		// 每处理完一批消息，裁剪 Stream 只保留最近 5000 条，防止无限膨胀
		if len(streams[0].Messages) > 0 {
			rdb.XTrimMaxLenApprox(ctx, "messages:stream", 200, 0)
		}
	}
}

// CacheUserPassword 缓存用户的密码 Redis作为旁路缓存的功能
func CacheUserPassword(nickname, password string) error {
	if rdb == nil {
		return fmt.Errorf("redis 未初始化")
	}
	// SetEx：原子操作 SET + EXPIRE，5 分钟后自动过期
	return rdb.Set(ctx, "user:"+nickname, password, 5*time.Minute).Err()
}

// GetCachedPassword 从缓存获取密码
func GetCachedPassword(nickname string) (string, error) {
	if rdb == nil {
		return "", fmt.Errorf("redis 未初始化")
	}
	val, err := rdb.Get(ctx, "user:"+nickname).Result()
	if errors.Is(err, redis.Nil) {
		// 缓存未命中，不是错误
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("获取密码失败")
	}
	return val, nil
}
