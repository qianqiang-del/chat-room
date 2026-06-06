package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

func showMainMenu() {
	fmt.Println("\n========== 聊天室主菜单 ==========")
	fmt.Println("1. 查看在线用户")
	fmt.Println("2. 查看历史聊天记录")
	fmt.Println("3. 退出聊天室")
	fmt.Println("4. 发送消息（进入聊天模式）")
	fmt.Println("5. 查看私聊信息")
	fmt.Println("6. 查看活跃度排行榜")
	fmt.Println("==================================")
}

// 与服务器进行登录/注册交互，直到登录成功，返回 true
func doAuth(conn net.Conn, nameReader, serverReader *bufio.Reader) bool {
	for {
		fmt.Println("========== 欢迎来到聊天室 ==========")
		fmt.Print("请选择: 1 登录  2 注册 : ")
		choice, _ := nameReader.ReadString('\n')
		choice = strings.TrimSpace(choice)
		if choice != "1" && choice != "2" {
			fmt.Println("无效选择，请重新输入")
			continue
		}
		_, err := conn.Write([]byte(choice + "\n"))
		if err != nil {
			return false
		}
		// 输入网名（非空校验）
		var nickname string
		for {
			fmt.Print("请输入网名: ")
			nickname, _ = nameReader.ReadString('\n')
			nickname = strings.TrimSpace(nickname)
			if nickname == "" {
				fmt.Println("网名不能为空，请重新输入")
				continue
			}
			break
		}
		_, err = conn.Write([]byte(nickname + "\n"))
		if err != nil {
			return false
		}

		// 输入密码（非空校验）
		var password string
		for {
			fmt.Print("请输入密码: ")
			password, _ = nameReader.ReadString('\n')
			password = strings.TrimSpace(password)
			if password == "" {
				fmt.Println("密码不能为空，请重新输入")
				continue
			}
			break
		}
		_, err = conn.Write([]byte(password + "\n"))
		if err != nil {
			return false
		}

		// 读取服务端响应
		response, err := serverReader.ReadString('\n')
		if err != nil {
			fmt.Println("读取响应失败:", err)
			return false
		}
		response = strings.TrimSpace(response)

		if strings.Contains(response, "登录成功") {
			fmt.Println(response)
			return true
		} else {
			fmt.Println(response)
			// 其他错误（用户不存在、密码错误、注册成功等）继续循环
		}
	}
}

// 创建一个发送消息的函数
func enterChatMode(conn net.Conn, reader *bufio.Reader) {
	fmt.Println("已进入聊天模式，输入消息即可发送，输入 /back 返回主菜单")
	fmt.Println("提示：输入 /msg 用户名 消息内容 可发送私聊消息")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("消息输入错误")
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "/back" {
			fmt.Println("返回主菜单")
			return
		}
		if line == "/online" || line == "/history" || line == "/private" || line == "/rank" {
			fmt.Println("【提示】该命令不能作为聊天消息发送，请重新输入")
			continue
		}
		_, err = conn.Write([]byte(line + "\n"))
		if err != nil {
			fmt.Println("客户端写入数据出错 err = ", err)
			break
		}
	}
}

func runMainMenu(conn net.Conn, nameReader *bufio.Reader) {
	for {
		fmt.Println()  // 先输出一个空行，与之前的消息隔开
		showMainMenu() // 显示菜单
		fmt.Println("请输入数字: ")
		choiceStr, _ := nameReader.ReadString('\n')
		choiceStr = strings.TrimSpace(choiceStr)
		switch choiceStr {
		case "1":
			_, err := conn.Write([]byte("/online\n"))
			if err != nil {
				fmt.Println("发送命令失败:", err)
			}
			fmt.Println("按回车键返回主菜单")
			_, err = nameReader.ReadString('\n')
			if err != nil {
				fmt.Printf("查看在线用户失败 err = %v \n", err)
				return
			} // 等待用户按回车
		case "2":
			_, err := conn.Write([]byte("/history\n"))
			if err != nil {
				fmt.Printf("发送命令失败 err = %v \n", err)
				return
			}
			fmt.Println("按回车键返回主菜单...")
			_, err = nameReader.ReadString('\n')
			if err != nil {
				fmt.Printf("查看历史信息失败 err = %v \n", err)
				return
			}
		case "3":
			fmt.Println("退出聊天室")
			err := conn.Close()
			if err != nil {
				return
			}
			// 退出程序
			os.Exit(0)
		case "4":
			enterChatMode(conn, nameReader)
		case "5":
			_, err := conn.Write([]byte("/private\n"))
			if err != nil {
				fmt.Println("发送命令失败:", err)
			}
			fmt.Println("按回车键返回主菜单")
			_, err = nameReader.ReadString('\n')
			if err != nil {
				fmt.Printf("查看私聊信息失败 err = %v \n", err)
				return
			} // 等待用户按回车
		case "6":
			_, err := conn.Write([]byte("/rank\n"))
			if err != nil {
				fmt.Println("发送命令失败:", err)
			}
			fmt.Println("按回车键返回主菜单")
			_, err = nameReader.ReadString('\n')
			if err != nil {
				fmt.Printf("查看排行榜失败 err = %v \n", err)
				return
			}
		default:
			fmt.Println("无效输入，请重新选择（1-6）")
		}
	}
}

func main() {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Printf("建立连接出错，err = %v \n", err)
		return
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			fmt.Println("关闭连接失败")
		}
	}()

	//利用键盘输入首先输入网名 创建一个封装函数
	nameReader := bufio.NewReader(os.Stdin)
	//接收服务器端的响应也是写进连接中
	serverReader := bufio.NewReader(conn)
	// 进行身份验证，直到登录成功
	if !doAuth(conn, nameReader, serverReader) {
		fmt.Println("身份验证失败，退出")
		return
	}
	//网名发送成功之后该发送消息
	// 启动接收广播消息的 goroutine（必须在进入菜单前启动）
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			news, err := serverReader.ReadString('\n')
			if err != nil {
				fmt.Println("与服务器断开连接")
				os.Exit(0)
			}
			fmt.Print(news)
		}
	}()
	// 运行主菜单（用户选择功能）
	runMainMenu(conn, nameReader)
	// 等待接收 goroutine 结束（当连接关闭时自然会退出）
	wg.Wait()
}
