package main

import (
	"bufio"
	"errors"
	"fmt"
	"go_chat_room/database"
	"net"
	"strings"
	"sync"
)

var onlineUsers = make(map[string]net.Conn)
var mu sync.Mutex
var broadcastChan = make(chan broadcastNews, 100)

type broadcastNews struct {
	News string //发送者的消息
	Name string //发送者的昵称
}

// 处理身份验证（登录/注册循环），直到登录成功才返回网名
func handleAuth(reader *bufio.Reader, writer *bufio.Writer) (string, bool) {

	for {
		// 1. 读取用户选择
		choiceLine, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("向客户端读取数据失败 err = %v \n", err)
			return "", false
		}
		//读取用户输入的账号和密码
		nickname, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("读取客户端输入账号出错 err = %v", err)
			return "", false
		}
		nickname = strings.TrimSpace(nickname)
		if nickname == "" {
			// 网名不能为空，提示后重新循环
			if _, err := writer.WriteString("网名不能为空\n"); err != nil {
				return "", false
			}
			_ = writer.Flush()
			continue
		}
		//读取用户输入的密码
		password, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("读取客户端输入密码出错 err = %v", err)
			return "", false
		}
		password = strings.TrimSpace(password)
		if password == "" {
			// 网名不能为空，提示后重新循环
			if _, err := writer.WriteString("密码不能为空\n"); err != nil {
				return "", false
			}
			_ = writer.Flush()
			continue
		}
		//对客户端读取到的消息进行空格处理
		choice := strings.TrimSpace(choiceLine)
		switch choice {
		case "1": //选择1进行登录的操作
			//首先登录查看密码是否在Redis缓存当中看是否命中
			cachedPassword, cacheErr := database.GetCachedPassword(nickname)
			if cachedPassword != "" && cacheErr == nil {
				//此时缓存命中
				if cachedPassword != password {
					_, err2 := writer.WriteString("密码错误\n")
					if err2 != nil {
						return "", false
					}
					err = writer.Flush()
					if err != nil {
						return "", false
					}
					continue
				}
				// 缓存命中且密码正确直接登录成功
				err = database.IncrActivity(nickname, 3)
				if err != nil {
					fmt.Printf("增加活跃度失败: %v\n", err)
				}
				database.UpdateLastActive(nickname)
				_, err = writer.WriteString("登录成功！\n")
				if err != nil {
					return "", false
				}
				err = writer.Flush()
				if err != nil {
					return "", false
				}

				return nickname, true
			}
			//下面是缓存没有命中去数据库中找
			storedPassword, err := database.QueryPassword(nickname)
			if errors.Is(err, database.ErrUserNotFound) {
				if _, err := writer.WriteString("用户不存在\n"); err != nil {
					return "", false
				}
				_ = writer.Flush()
				continue
			} else if err != nil {
				fmt.Printf("数据库查询失败: %v\n", err)
				if _, err := writer.WriteString("数据库内部错误，请稍后重试\n"); err != nil {
					return "", false
				}
				_ = writer.Flush()
				continue
			}
			if storedPassword != password {
				if _, err := writer.WriteString("密码错误\n"); err != nil {
					return "", false
				}
				_ = writer.Flush()
				continue
			}
			//每次登录成功的时候 就把这个账号中的密码存进缓存中
			if err = database.CacheUserPassword(nickname, storedPassword); err != nil {
				fmt.Printf("缓存用户信息失败: %v\n", err)
			}

			// 更新最后活跃时间
			database.UpdateLastActive(nickname)
			// 登录成功，活跃度 +3
			if err := database.IncrActivity(nickname, 3); err != nil {
				fmt.Printf("增加活跃度失败: %v\n", err)
			}

			if _, err := writer.WriteString("登录成功！\n"); err != nil {
				return "", false
			}
			if err := writer.Flush(); err != nil {
				return "", false
			}
			return nickname, true

		case "2": //选择2进行注册的操作
			exists, err := database.CheckUserExists(nickname)
			if err != nil {
				fmt.Printf("查询用户数量失败: %v\n", err)
				if _, err := writer.WriteString("数据库错误，无法注册\n"); err != nil {
					return "", false
				}
				_ = writer.Flush()
				continue
			}
			if exists {
				if _, err := writer.WriteString("网名已被注册\n"); err != nil {
					return "", false
				}
				_ = writer.Flush()
				continue
			}
			// 插入新用户（明文密码，仅演示）
			err = database.CreateUser(nickname, password)
			if err != nil {
				fmt.Printf("插入用户失败: %v\n", err)
				if _, err := writer.WriteString("注册失败，请稍后重试\n"); err != nil {
					return "", false
				}
				_ = writer.Flush()
				continue
			}
			if _, err := writer.WriteString("注册成功，请重新登录\n"); err != nil {
				return "", false
			}
			if err := writer.Flush(); err != nil {
				return "", false
			}
			// 注册成功后继续循环，让用户选择登录
			continue

		default:
			if _, err := writer.WriteString("无效选择，请输入 1 或 2\n"); err != nil {
				return "", false
			}
			_ = writer.Flush()
			continue
		}
	}
}

// 创建一个注销网名的操作
func unRegister(name string) {
	mu.Lock()
	delete(onlineUsers, name)
	mu.Unlock()
	broadcast(name+" 离开了聊天室", name)
	fmt.Printf("用户 %s 已退出聊天室\n", name)
}

// 这个广播函数只需要把消息放进管道中
func broadcast(news string, name string) {
	//现在把消息放进管道中
	broadcastChan <- broadcastNews{News: news, Name: name}
}

// 把管道中的消息发送给除自己外的所有的客户端
func broadcastSender() {
	//对管道的消息进行遍历
	for v := range broadcastChan {
		mu.Lock()
		for name, conn := range onlineUsers {
			if name == v.Name {
				continue
			}
			_, err := conn.Write([]byte(v.News + "\n"))
			if err != nil {
				fmt.Printf("广播给 %s 失败: %v\n", name, err)
			}
		}
		mu.Unlock()
	}
}

// 发送私聊消息：格式 /msg 目标 消息内容
func sendPrivateMessage(senderConn net.Conn, senderName, rawLine string) {
	// 去掉前导空格，检查前缀
	trimmed := strings.TrimSpace(rawLine)
	if !strings.HasPrefix(trimmed, "/msg ") {
		return
	}
	// 分割：第一部分命令，第二部分目标，第三部分开始是消息内容
	parts := strings.SplitN(trimmed, " ", 3)
	if len(parts) < 3 {
		_, err := senderConn.Write([]byte("【系统】私聊格式错误，正确格式: /msg 用户名 消息内容\n"))
		if err != nil {
			return
		}
		return
	}
	targetName := parts[1]
	content := parts[2]
	if content == "" {
		_, err := senderConn.Write([]byte("【系统】私聊消息不能为空\n"))
		if err != nil {
			return
		}
		return
	}
	if targetName == senderName {
		_, err := senderConn.Write([]byte("【系统】不能给自己发送私聊消息\n"))
		if err != nil {
			return
		}
		return
	}
	mu.Lock()
	targetConn, ok := onlineUsers[targetName]
	mu.Unlock()
	if !ok {
		_, err := senderConn.Write([]byte(fmt.Sprintf("【系统】用户 %s 当前不在线\n", targetName)))
		if err != nil {
			return
		}
		return
	}

	// ========== 插入私聊消息到数据库 ==========
	if err := database.PushMessageToStream(senderName, content, "private", targetName); err != nil {
		fmt.Printf("推送私聊消息到Stream失败: %v\n", err)
	}
	// =====================================
	// 私聊活跃度 +1
	if err := database.IncrActivity(senderName, 1); err != nil {
		fmt.Printf("增加活跃度失败: %v\n", err)
	}
	// 转发给目标
	_, err := targetConn.Write([]byte(fmt.Sprintf("【私聊】%s: %s\n", senderName, content)))
	if err != nil {
		_, err2 := senderConn.Write([]byte(fmt.Sprintf("【系统】发送给 %s 失败，对方可能已断开\n", targetName)))
		if err2 != nil {
			return
		}
		return
	}
	_, err = senderConn.Write([]byte(fmt.Sprintf("【系统】已发送私聊给 %s\n", targetName)))
	if err != nil {
		return
	}
}

// 查看私聊消息的函数
func sendPrivateHistoryFromDB(conn net.Conn, nickname string) {
	records, err := database.GetPrivateHistory(nickname)
	if err != nil {
		_, _ = conn.Write([]byte("【系统】查询私聊记录失败\n"))
		fmt.Printf("查询私聊记录出错: %v\n", err)
		return
	}

	if len(records) == 0 {
		_, _ = conn.Write([]byte("暂无私聊记录\n"))
		return
	}

	_, _ = conn.Write([]byte("===== 我的私聊记录 =====\n"))
	for _, r := range records {
		var line string
		if r.Sender == nickname {
			line = fmt.Sprintf("[私聊] 我 -> %s : %s", r.Target, r.Content)
		} else {
			line = fmt.Sprintf("[私聊] %s -> 我 : %s", r.Sender, r.Content)
		}
		_, _ = conn.Write([]byte(line + "\n"))
	}
	_, _ = conn.Write([]byte("========================\n"))
}

// 查看历史消息的函数
func sendHistoryToConn(conn net.Conn, limit int) {
	records, err := database.GetPublicHistory(limit)
	if err != nil {
		_, _ = conn.Write([]byte("查询历史消息失败\n"))
		fmt.Printf("查询公共历史出错: %v\n", err)
		return
	}

	if len(records) == 0 {
		_, _ = conn.Write([]byte("暂无历史消息\n"))
		return
	}

	// 输出，格式： "用户名: 内容"
	for _, r := range records {
		line := fmt.Sprintf("%s: %s\n", r.Sender, r.Content)
		_, _ = conn.Write([]byte(line))
	}
}

// 发送活跃度排行榜
func sendActivityRank(conn net.Conn) {
	entries, err := database.GetTopActivity(10)
	if err != nil {
		_, _ = conn.Write([]byte(fmt.Sprintf("查询排行榜失败: %v\n", err)))
		return
	}
	if len(entries) == 0 {
		_, _ = conn.Write([]byte("暂无活跃度数据\n"))
		return
	}
	_, err = conn.Write([]byte("===== 活跃度排行榜 TOP10 =====\n"))
	if err != nil {
		return
	}
	_, err = conn.Write([]byte(fmt.Sprintf("%-4s %-16s %s\n", "排名", "用户", "活跃度")))
	if err != nil {
		return
	}
	_, err = conn.Write([]byte("--------------------------------\n"))
	if err != nil {
		return
	}
	for i, e := range entries {
		_, err = conn.Write([]byte(fmt.Sprintf(" %-4d %-16s %d\n", i+1, e.Nickname, e.Score)))
		if err != nil {
			return
		}
	}
	_, err = conn.Write([]byte("================================\n"))
	if err != nil {
		return
	}
}

// 发送在线用户的函数
func sendOnlineList(conn net.Conn) {
	mu.Lock()
	defer mu.Unlock()
	var users []string
	for name := range onlineUsers {
		users = append(users, name)
	}
	_, err := conn.Write([]byte("当前在线用户: " + strings.Join(users, ", ") + "\n"))
	if err != nil {
		return
	}
}

// 创建一个函数来处理接收的网名和消息 建立一个协程
func process(conn net.Conn) {
	//处理完这个数据之后要进行关闭这个连接
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			fmt.Printf("关闭连接失败 err = %v \n", err)
		}
	}(conn)
	//首先创建都缓冲区
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	// 调用身份验证函数，直到登录成功
	nickname, ok := handleAuth(reader, writer)
	if !ok {
		fmt.Printf("用户 %s 身份验证失败，连接关闭\n", nickname)
		return
	}

	// 将用户加入在线列表（防止重复登录）
	mu.Lock()
	if _, exists := onlineUsers[nickname]; exists {
		mu.Unlock()
		_, err := writer.WriteString("该账号已在其他地方登录\n")
		if err != nil {
			return
		}
		err = writer.Flush()
		if err != nil {
			return
		}
		return
	}
	onlineUsers[nickname] = conn
	mu.Unlock()
	//当该用户退出的时候进行注销
	defer unRegister(nickname)
	broadcast(nickname+" 加入了聊天室", nickname)
	//网名创建完成之后开始发送消息
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		//对接收到的的消息进行空格处理
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "/online" {
			sendOnlineList(conn)
			continue
		}
		if line == "/history" {
			sendHistoryToConn(conn, 20) // 发送最近20条历史消息
			continue
		}
		if line == "/private" {
			sendPrivateHistoryFromDB(conn, nickname)
			continue
		}
		if line == "/rank" {
			sendActivityRank(conn)
			continue
		}
		// 私聊命令 /msg
		if strings.HasPrefix(line, "/msg ") {
			sendPrivateMessage(conn, nickname, line)
			continue
		}
		// 公共消息先推入到Stream消息队列中
		msgStr := nickname + ": " + line
		if err = database.PushMessageToStream(nickname, line, "public", ""); err != nil {
			fmt.Printf("推送消息到Stream失败: %v\n", err)
		}
		broadcast(msgStr, nickname)
		// 发送公共消息活跃度 +2
		if err := database.IncrActivity(nickname, 2); err != nil {
			fmt.Printf("增加活跃度失败: %v\n", err)
		}
	}
}
func main() {
	err := database.InitDB() // 调用初始化数据库的函数
	if err != nil {
		fmt.Printf("init db failed,err:%v\n", err)
		return
	}
	defer database.CloseDB()

	// 初始化 Redis（非必须，连接失败只打日志不退出）
	if err = database.InitRedis("localhost:6379"); err != nil {
		fmt.Printf("警告: %v，活跃度功能不可用\n", err)
	} else {
		defer database.CloseRedis()
		err = database.EnsureStreamGroup()
		if err != nil {
			fmt.Printf("警告: 创建Stream消费者组失败: %v\n", err)
		}
		go database.StartMessageConsumer()

	}
	//创建一个网络聊天室首先创建一个接口开始监听
	//使用到net包下的listen函数，返回值是一个listener的结构体和错误
	//首先这个地址是自己的ip和端口的不是客户端的ip和地址
	//提供一个固定的"服务入口"；客户端的 IP 和端口是在连接建立后得知的，而不是用来监听的
	fmt.Println("服务器开始监听")
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		fmt.Printf("监听失败 err= %v \n", err)
		return
	}
	//关闭这个接口listener
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			fmt.Printf("关闭这个接口失败 err = %v \n", err)
		}
	}(listener)
	go broadcastSender()
	//如果监听到一个客户端那么跟这个客户端建立连接
	for {
		//接受客户端连接请求的,它会阻塞（等待）,直到有客户端调用 net.Dial
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("这个客户端没有连接成功，err = ", err)
		} else {
			fmt.Printf("接收到这个客户端的连接了 conn = %v \n", conn)
		}
		go process(conn)
	}

}
