# 秒开输入（mkinput）

手机语音输入 → 实时同步到电脑


## 产品理念

手机麦克风的收音质量和输入法的语音识别准确率都比电脑高。电脑端的语音输入，例如微信输入法和千问，都有各种各样的问题，于是有了这个项目，用手机做输入设备，通过 websocket 直接传输到电脑。

## 核心原理

电脑运行软件后，会在后台启动一个Web服务（例如192.168.1.60:9999），手机浏览器打开链接，输入的文字（包括语音转文字）会实时输入到电脑当前焦点窗口中。手机以追加输入为主（文字/回车），仅"退格"按钮可显式删除电脑末尾字符，不做任何自动删除或整体替换，发送后手机输入框自动清空



## 使用方法

1. **打开程序**：双击 `mkinput.exe`，自动隐藏到右下角**系统托盘**
2. **复制地址**：右键托盘图标 → **服务地址** → 点击 IP 复制
3. **手机访问**：同一 WiFi 下，手机浏览器打开复制的网址
4. **开始输入**：在手机页面中打字或语音输入，文字实时出现在电脑上


## 功能介绍

- **两种模式**：手机端可切换"自动发送后清屏"与"手动发送后清屏"，两种模式发送后都会自动清空输入框
- **自动发送**：输入/语音识别内容自动上屏，发送后清空输入框
- **手动发送**：点击"发送"按钮才上屏，发送后清空输入框
- **退格**：唯一显式删除操作——点击后删除电脑光标前一个字符，并同步删除手机输入框最后一个字符
- **回车**：发送回车键
- **安全原则**：除显式点击"退格"外，不做任何自动删除、退格或整体替换

## 页面截图

![手机端页面](assets/UI.jpg)


## 项目结构

```
mkinput/
├── main.go                        # 主程序：HTTP + WebSocket + 全文输入 + 系统托盘
├── web/index.html                 # 手机端页面（//go:embed 嵌入到二进制）
├── internal/keyboard/sendinput.go # Windows SendInput API syscall 封装
├── assets/
│   ├── architecture.svg           # 架构图
│   ├── icon.ico                   # 托盘图标
│   └── UI.jpg                     # 手机端页面截图
├── png2ico.py                     # PNG 转 ICO 工具
├── go.mod / go.sum                # Go 模块依赖
└── README.md
```



## 打包流程

```bash
# 开发调试（有控制台日志）
go run .

# 热更新
 go install github.com/air-verse/air@latest 
 air
 

# 打包
go build -ldflags -H=windowsgui -o build/mkinput.exe .

# 打tag
git push origin v1.2.0

# 发布release
gh release create v1.2.0 build/mkinput.exe -t "v1.2.0"




```

***

## 技术栈

- **后端**：Go `net/http` + `gorilla/websocket` + `getlantern/systray`
- **键盘模拟**：Windows `user32.dll!SendInput`（直接 syscall，无 cgo）
- **前端**：原生 HTML + CSS + JS（嵌入式）
- **系统要求**：Windows 10/11 x64，Go 1.23+（编译），同一局域网

***

## License

MIT
