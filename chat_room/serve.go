package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
)

// 创建一个map
var onlineUsers = make(map[string]net.Conn)
var mu sync.Mutex
var broadcastChan = make(chan broadcastNews, 100)

type broadcastNews struct {
	News string //发送者的消息
	Name string //发送者的昵称
}

//现在要写一个函数进行消息注册

func Register(name string, conn net.Conn) bool {
	//客户端传入了一个数据,因为是协程所以要对map的使用进行加锁，因为你不能同时对map进行读取和写入
	//关于这个重复网络可以这样处理，你的得到了一个网名然后对应的是一个客户端
	//对map进行查找的方法查找到主键（这个主键对应的是网名）相同的时候就失败
	mu.Lock()
	defer mu.Unlock()
	if _, ok := onlineUsers[name]; ok {
		return false
	}
	onlineUsers[name] = conn
	return true
}

// 注销用户
func unregister(name string) {
	mu.Lock()
	defer mu.Unlock()
	delete(onlineUsers, name)
	broadcast(name+" 离开了聊天室", name)
	fmt.Printf("用户 %s 已退出聊天室\n", name)
}

// 这个广播函数只需要把消息放进管道中
func broadcast(news string, name string) {
	//现在把消息放进管道中
	broadcastChan <- broadcastNews{News: news, Name: name}
}

func broadcastSender() {
	// 广播把消息发送给除自己外的所有的客户端
	for v := range broadcastChan {
		mu.Lock()
		//当从客户端接收到一个消息，现在要把这个消息发送给所有的客户端
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

// 创建一个函数进行接收客户端发送的信息
// 首先最重要的就是要拿到这个连接客户端发送的
func process(conn net.Conn) {
	//处理完这个数据之后要进行关闭这个连接
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			fmt.Printf("关闭连接失败 err = %v \n", err)
		}
	}(conn)

	//此时从客户端读取到网名
	//使用进行正行读取
	//先把连接中的数据读取到缓冲区中
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	var nickname string
	for {
		str, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		//将空格和换行符进行清除
		nickname = strings.TrimSpace(str)
		if nickname == "" {
			_, err = writer.WriteString("输入的网名不能为空 \n")
			err = writer.Flush()
			continue
		}
		if Register(nickname, conn) {
			_, err = writer.WriteString("欢迎进入聊天室 \n")
			err = writer.Flush()
			break
		} else {
			_, err = writer.WriteString("输入的网名重复 \n")
			err = writer.Flush()
		}
	}
	//当该用户退出的时候进行注销
	defer unregister(nickname)
	broadcast(nickname+" 加入了聊天室", nickname)
	//现在从客户端接收消息跟接收网名的信息一样 循环接收
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		news := strings.TrimSpace(line)
		if news == "" {
			continue
		}
		broadcast(nickname+": "+news, nickname)
	}
}

func main() {
	//使用到net包下的listen函数，返回值是一个listener的结构体和错误
	//首先这个地址是自己的ip和端口的不是客户端的ip和地址
	//提供一个固定的“服务入口”；客户端的 IP 和端口是在连接建立后得知的，而不是用来监听的。
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
	//现在这个服务器端不能只接收一个客户端的发送
	//客户端往这个端口发送数据，服务端接收到，不能只接受一个客户端的数据，就是我这个监听持续监听
	go broadcastSender()

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
