package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Printf("建立连接出错，err = %v \n", err)
		return
	}
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {

		}
	}(conn)

	reader1 := bufio.NewReader(conn)
	nameReader := bufio.NewReader(os.Stdin)

	// 注册昵称
	for {
		fmt.Print("请输入网名:")
		nickname, _ := nameReader.ReadString('\n')
		nickname = strings.TrimSpace(nickname)
		if nickname == "" {
			fmt.Println("网名不能为空")
			continue
		}
		_, err = conn.Write([]byte(nickname + "\n"))
		if err != nil {
			fmt.Println("发送网名失败:", err)
			return
		}
		response, err := reader1.ReadString('\n')
		if err != nil {
			fmt.Printf("获取服务端响应失败 err = %v\n", err)
			return
		}
		response = strings.TrimSpace(response)
		if strings.Contains(response, "欢迎") {
			fmt.Println(response)
			break
		} else {
			fmt.Println(response)
		}
	}

	// 接收广播消息
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			news, err := reader1.ReadString('\n')
			if err != nil {
				fmt.Println("与服务器断开连接")
				return
			}
			fmt.Print(news)
		}
	}()

	// 发送消息
	fmt.Println("请发送消息:")
	for {
		line, err := nameReader.ReadString('\n')
		if err != nil {
			fmt.Println("读取数据失败 err = ", err)
			break
		}
		line = strings.TrimSpace(line)
		if line == "exit" {
			fmt.Println("客户端退出")
			break
		}
		_, err = conn.Write([]byte(line + "\n"))
		if err != nil {
			fmt.Println("客户端写入数据出错 err = ", err)
			break
		}
	}
	err = conn.Close()
	if err != nil {
		return
	}
	wg.Wait()
}
