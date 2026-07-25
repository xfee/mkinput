# 秒开输入（mkinput）

手机语音输入 → 实时同步到电脑


## 产品理念

手机麦克风的收音质量和输入法的语音识别准确率都比电脑高。电脑端的语音输入，例如微信输入法和千问，都有各种各样的问题，于是有了这个项目，用手机做输入设备，通过 websocket 直接传输到电脑。

## 核心原理

电脑运行软件后，会在后台启动一个Web服务（例如192.168.1.60:9999），手机浏览器打开链接，输入的文字（包括语音转文字）会实时发送到电脑当前焦点窗口中



## 使用方法

1. **打开程序**：双击 `mkinput.exe`，自动隐藏到右下角**系统托盘**
2. **复制地址**：右键托盘图标 → **服务地址** → 点击 IP 复制
3. **手机访问**：同一 WiFi 下，手机浏览器打开复制的网址
4. **开始输入**：在手机页面中打字或语音输入，文字实时出现在电脑上


## 功能介绍

- **输入框打字/语音识别**： 开启自动发送时，内容自动同步到电脑 
- **重新上屏**：手动发送完整内容到电脑       
- **发送退格** 发送退格键             
- **发送回车** 发送回车键             
- **清空**    清空输入框             




## 项目结构

```
mkinput/
├── main.go                        # 主程序：HTTP + WebSocket + 差异比较 + 系统托盘
├── web/index.html                 # 手机端页面（//go:embed 嵌入到二进制）
├── internal/keyboard/sendinput.go # Windows SendInput API syscall 封装
├── assets/
│   ├── architecture.svg           # 架构图
│   └── icon.ico                   # 托盘图标
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

暂不构建macOS， 因为Windows没有豆包输入法
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
