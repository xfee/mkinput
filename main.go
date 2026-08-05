package main

import (
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/getlantern/systray"
	"github.com/gorilla/websocket"
	"github.com/xfee/mkinput/internal/keyboard"
	"golang.org/x/sys/windows/registry"
)

// 将前端页面嵌入到编译后的二进制文件中，无需额外分发 HTML 文件
//
//go:embed web/index.html assets/icon.ico
var embeddedFiles embed.FS

// WebSocket 升级器，允许跨域连接（手机浏览器访问电脑服务）
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// 当前连接的客户端数量，使用原子操作确保并发安全
var clientCount int64

// 用于将客户端数量变化通知到系统托盘菜单的通道
var clientInfoCh = make(chan int64, 8)

// 服务端口号
const port = "9999"

// 程序入口：启动 HTTP 服务 + 系统托盘图标
func main() {
	// 注册路由：首页 + WebSocket
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/ws", wsHandler)

	// 在协程中启动 HTTP 服务器，不阻塞主线程
	go startHTTPServer()

	// 启动系统托盘（会阻塞主线程，直到用户点击退出）
	systray.Run(onSystrayReady, onSystrayExit)
}

// startHTTPServer 在后台启动 HTTP 监听，打印访问地址
func startHTTPServer() {
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("  秒开输入 — 手机输入实时同步到电脑")
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("  HTTP 服务已启动,请在同网络下的手机浏览器中访问:")
	for _, ip := range getLocalIPs() {
		log.Printf("    → http://%s:%s", ip, port)
	}
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

// getLocalIPs 获取本机所有活跃网卡的 IPv4 地址，
// 排除回环地址（127.0.0.1）和未启用的网卡
func getLocalIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		// 跳过未启用的网卡和回环地址
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			// 只保留 IPv4 地址
			if ip4 := ip.To4(); ip4 != nil {
				ips = append(ips, ip4.String())
			}
		}
	}
	return ips
}

// ── 系统托盘 ─────────────────────────────────────────────

// onSystrayReady 是系统托盘初始化完成后的回调，
// 在此创建右键菜单项，设置图标
func onSystrayReady() {
	// 从嵌入式文件系统加载图标
	iconData, err := embeddedFiles.ReadFile("assets/icon.ico")
	if err != nil {
		log.Printf("⚠️ 加载图标失败: %v", err)
	} else {
		systray.SetIcon(iconData)
	}
	systray.SetTitle("秒开输入")
	systray.SetTooltip("秒开输入 - 手机输入实时同步到电脑")

	// ── 菜单顶部标题 ──
	titleItem := systray.AddMenuItem("秒开输入", "手机输入实时同步到电脑")
	titleItem.Disable()

	systray.AddSeparator()

	// ── 服务地址（子菜单，点击复制） ──
	addrHeader := systray.AddMenuItem("服务地址", "所有可用 IP 地址，点击复制")
	ips := getLocalIPs()

	type addrPair struct {
		item *systray.MenuItem
		url  string
	}
	var addrItems []addrPair
	if len(ips) == 0 {
		noIP := addrHeader.AddSubMenuItem("无网络连接", "")
		noIP.Disable()
	} else {
		for _, ip := range ips {
			url := fmt.Sprintf("http://%s:%s", ip, port)
			item := addrHeader.AddSubMenuItem(url+"  复制", "点击复制到剪贴板")
			addrItems = append(addrItems, addrPair{item, url})
		}
	}

	// 启动弹窗：告知用户托盘位置
	go showStartupMessage(ips)

	// ── 连接客户端数量 ──
	systray.AddSeparator()
	clientItem := systray.AddMenuItem("连接客户端: 0", "当前连接的客户端数量")
	clientItem.Disable()

	systray.AddSeparator()

	// ── 开机自动运行 ──
	autoStart := isAutoStartEnabled()
	autoStartItem := systray.AddMenuItemCheckbox("开机自动运行", "开机时自动启动", autoStart)

	// ── 项目链接 ──
	projectItem := systray.AddMenuItem("项目地址", "在浏览器中打开项目主页")
	authorItem := systray.AddMenuItem("作者地址", "在浏览器中打开作者主页")
	updateItem := systray.AddMenuItem("检查更新", "在浏览器中查看发布页面")

	systray.AddSeparator()

	// ── 退出 ──
	quitItem := systray.AddMenuItem("退出", "退出程序")

	// ── 事件处理 ──

	// 客户端数量实时更新
	go func() {
		for count := range clientInfoCh {
			clientItem.SetTitle(fmt.Sprintf("连接客户端: %d", count))
		}
	}()

	// IP 子菜单点击 → 复制到剪贴板
	go func() {
		for _, pair := range addrItems {
			go func(item *systray.MenuItem, url string) {
				for range item.ClickedCh {
					if err := copyToClipboard(url); err != nil {
						log.Printf("⚠️ 复制失败: %v", err)
						showMessage("复制失败", err.Error())
					} else {
						showMessage("秒开输入", "复制 "+url+" 成功")
					}
				}
			}(pair.item, pair.url)
		}
	}()

	// 其他菜单点击
	go func() {
		for {
			select {
			case <-autoStartItem.ClickedCh:
				enabled := !autoStartItem.Checked()
				setAutoStart(enabled)
				if enabled {
					autoStartItem.Check()
				} else {
					autoStartItem.Uncheck()
				}
			case <-projectItem.ClickedCh:
				_ = openURL("https://github.com/xfee/mkinput")
			case <-authorItem.ClickedCh:
				_ = openURL("https://github.com/xfee")
			case <-updateItem.ClickedCh:
				_ = openURL("https://github.com/xfee/mkinput/releases")
			case <-quitItem.ClickedCh:
				systray.Quit()
				os.Exit(0)
			}
		}
	}()
}

// openURL 在默认浏览器中打开链接（Windows）
func openURL(url string) error {
	return runHidden("cmd", "/c", "start", url)
}

// runHidden 执行命令并隐藏窗口（避免弹 PowerShell 蓝框）
func runHidden(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}

// showMessage 弹出 2 秒自动关闭的提示框（PowerShell WScript.Shell Popup）
func showMessage(title, text string) {
	// 对 PowerShell 来说单引号内的单引号需要转义
	safeText := strings.ReplaceAll(text, "'", "''")
	safeTitle := strings.ReplaceAll(title, "'", "''")
	// timeout=2 秒, 0x40=信息图标, 不阻塞后续操作
	cmd := fmt.Sprintf("(New-Object -ComObject WScript.Shell).Popup('%s',3,'%s',0x40)", safeText, safeTitle)
	runHidden("powershell", "-command", cmd)
}

// showStartupMessage 程序启动后弹出 1.5 秒提示，告知用户托盘位置
func showStartupMessage(ips []string) {
	msg := "秒开输入已启动\n请在右下角托盘中进行操作\n\n"
	if len(ips) > 0 {
		msg += "服务地址：\n"
		for _, ip := range ips {
			msg += fmt.Sprintf("  http://%s:%s\n", ip, port)
		}
	}
	safeMsg := strings.ReplaceAll(msg, "'", "''")
	// timeout=1.5 秒, 0x40=信息图标
	cmd := fmt.Sprintf("(New-Object -ComObject WScript.Shell).Popup('%s',1.5,'秒开输入',0x40)", safeMsg)
	runHidden("powershell", "-command", cmd)
}

// copyToClipboard 使用 PowerShell 将文字写入 Windows 剪贴板
func copyToClipboard(text string) error {
	// 对单引号转义
	safeText := strings.ReplaceAll(text, "'", "''")
	return runHidden("powershell", "-command", fmt.Sprintf("Set-Clipboard -Value '%s'", safeText))
}

// onSystrayExit 是系统托盘退出时的清理回调
func onSystrayExit() {
	// 可在此处关闭资源（当前无需清理）
}

// ── 开机自启动（Windows 注册表） ─────────────────────────

const autoStartKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const autoStartName = "mkinput"

// isAutoStartEnabled 检查当前是否已注册开机自启动
func isAutoStartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, autoStartKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue(autoStartName)
	return err == nil
}

// setAutoStart 设置或取消开机自启动
// 原理：在 HKCU\...\Run 下写入或删除本程序的路径
func setAutoStart(enabled bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, autoStartKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		log.Printf("⚠️ 无法打开注册表: %v", err)
		return
	}
	defer k.Close()

	if enabled {
		exe, err := os.Executable()
		if err != nil {
			log.Printf("⚠️ 获取程序路径失败: %v", err)
			return
		}
		
		// 检查是否为临时路径（IDE 调试时 os.Executable() 返回临时目录）
		lowerExe := strings.ToLower(exe)
		if strings.Contains(lowerExe, "\\tmp\\") || 
		   strings.Contains(lowerExe, "\\go-build") ||
		   strings.Contains(lowerExe, "\\_go_build") {
			log.Printf("⚠️ 当前运行路径为临时目录，无法设置开机自启动")
			log.Printf("   当前路径: %s", exe)
			log.Printf("   请先编译并部署到正式位置（如 D:\\Program Files\\mkinput）")
			showMessage("无法设置自启动", "请先编译并部署到正式位置再启用开机自启动")
			return
		}
		
		if err := k.SetStringValue(autoStartName, exe); err != nil {
			log.Printf("⚠️ 设置开机自启动失败: %v", err)
			showMessage("设置失败", "设置开机自启动时发生错误")
		} else {
			log.Printf("✅ 开机自启动已启用: %s", exe)
			msg := fmt.Sprintf("开机自启动设置成功\n\n注册表路径:\n%s\n\n程序路径:\n%s", autoStartKey+"\\"+autoStartName, exe)
			showMessage("设置成功", msg)
		}
	} else {
		if err := k.DeleteValue(autoStartName); err != nil {
			log.Printf("⚠️ 取消开机自启动失败: %v", err)
			showMessage("取消失败", "取消开机自启动时发生错误")
		} else {
			log.Printf("✅ 开机自启动已禁用")
			showMessage("已取消", "开机自启动已取消")
		}
	}
}

// ── HTTP 路由处理 ────────────────────────────────────────

// homeHandler 返回嵌入式的前端页面
func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	data, _ := embeddedFiles.ReadFile("web/index.html")
	_, _ = w.Write(data)
}

// wsHandler 处理 WebSocket 连接，接收手机端输入的文本。
// 原则：以追加输入（文本 / 回车）为主；删除电脑文本仅通过用户显式点击"退格"触发，
// 程序不会自动删除或整体替换电脑已有内容。
func wsHandler(w http.ResponseWriter, r *http.Request) {
	// 将 HTTP 连接升级为 WebSocket 长连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}

	// 原子递增客户端计数，记录此连接的序号
	count := atomic.AddInt64(&clientCount, 1)
	clientIP := r.RemoteAddr
	log.Printf("🟢 客户端已连接 [#%d] %s (当前在线: %d)", count, clientIP, count)
	clientInfoCh <- count

	// 连接断开时自动清理
	defer func() {
		conn.Close()
		current := atomic.AddInt64(&clientCount, -1)
		log.Printf("🔴 客户端已断开 [#%d] %s (当前在线: %d)", count, clientIP, current)
		clientInfoCh <- current
	}()

	// lastInput 记录该客户端上一次接收的文本，仅用于去重
	var lastInput string

	// 持续读取 WebSocket 消息
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
			}
			text := string(message)

			// 用户显式点击"退格"：向电脑发送一次退格（删除光标前一个字符）
			if text == "__BACKSPACE__" {
				if err := keyboard.SendBackspace(); err != nil {
					log.Printf("⚠️ 退格失败: %v", err)
				}
				r := []rune(lastInput)
				if len(r) > 0 {
					lastInput = string(r[:len(r)-1])
				}
				continue
			}
			// 处理特殊指令
			if text == "__ENTER__" {
			// 发送回车键
			if err := keyboard.SendEnter(); err != nil {
				log.Printf("⚠️ 回车失败: %v", err)
			}
			lastInput = ""
			continue
		}
		if text == "__CLEARED__" {
			// 手机输入框已清空：重置去重基准，允许相同内容再次发送
			lastInput = ""
			continue
		}
		// 去重：内容和上次一样则不处理
		if text == lastInput {
			continue
		}
		log.Printf("📩 收到输入 [#%d]: %s", count, text)

		// 纯追加：把完整文本输入到电脑光标处，不做任何删除/退格/替换
		if err := keyboard.SendText(text); err != nil {
			log.Printf("⚠️ 输入失败: %v", err)
		}
		lastInput = text
	}
}
