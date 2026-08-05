// Package keyboard 封装 Windows SendInput API，将文字模拟键盘输入到当前焦点窗口。
//
// 直接从 simple-demo.go 提取，经测试验证可行。
// 关键修复：x64 上 sizeof(INPUT) = 40，不是 KEYBDINPUT 的完整大小。
package keyboard

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	inputKeyboard    = 1
	keyeventfUnicode = 0x0004
	keyeventfKeyUp   = 0x0002
	vkBack           = 0x08
	vkReturn         = 0x0D
)

// keybdInput 与 Windows KEYBDINPUT 结构一致，x64 上 sizeof = 24
type keybdInput struct {
	wVk         uint16  // offset 0
	wScan       uint16  // offset 2
	dwFlags     uint32  // offset 4
	time        uint32  // offset 8
	_           [4]byte // padding: offset 12 → 16（对齐 ULONG_PTR）
	dwExtraInfo uintptr // offset 16, size 8
}

// input 与 Windows INPUT 结构一致。
// x64 上 sizeof(INPUT) = 40，因为 union 中最大成员是 MOUSEINPUT（32 bytes）
// 仅 keybdInput 只占 24 bytes，所以末尾再补 8 bytes。
type input struct {
	_type uint32      // offset 0
	_     [4]byte     // offset 4: 对齐 union 到 8
	ki    keybdInput  // offset 8, size 24
	_     [8]byte     // offset 32: union 填充
}

var (
	user32        = syscall.NewLazyDLL("user32.dll")
	sendInputProc = user32.NewProc("SendInput")
	kernel32      = syscall.NewLazyDLL("kernel32.dll")
	lastErrorProc = kernel32.NewProc("GetLastError")
)

// sendBatch 一次 SendInput 调用批量提交一组 INPUT 事件。
func sendBatch(events []input) error {
	if len(events) == 0 {
		return nil
	}
	ret, _, _ := sendInputProc.Call(
		uintptr(len(events)),
		uintptr(unsafe.Pointer(&events[0])),
		uintptr(unsafe.Sizeof(input{})),
	)
	if ret == 0 {
		errCode, _, _ := lastErrorProc.Call()
		return fmt.Errorf("SendInput 失败: count=%d GetLastError=%d", len(events), errCode)
	}
	return nil
}

// SendText 将 text 通过一次 SendInput 调用批量输入到当前焦点窗口。
// 每个字符对应 KEYDOWN（UNICODE）+ KEYUP 两个 INPUT 事件，
// 全部打包成一个数组一次性提交，不做逐字符模拟，避免中途被打断。
func SendText(text string) error {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	// 每个字符需要 KEYDOWN + KEYUP 两个事件
	events := make([]input, 0, len(runes)*2)
	for _, r := range runes {
		events = append(events, input{
			_type: inputKeyboard,
			ki:    keybdInput{wScan: uint16(r), dwFlags: keyeventfUnicode},
		})
		events = append(events, input{
			_type: inputKeyboard,
			ki:    keybdInput{wScan: uint16(r), dwFlags: keyeventfUnicode | keyeventfKeyUp},
		})
	}
	return sendBatch(events)
}

// SendBackspace 向当前焦点窗口发送一次退格键（VK_BACK），删除光标前一个字符。
// 仅在用户显式点击手机端"退格"按钮时调用，程序不会自动删除电脑文本。
func SendBackspace() error {
	return sendBatch([]input{
		{_type: inputKeyboard, ki: keybdInput{wVk: vkBack, dwFlags: 0}},
		{_type: inputKeyboard, ki: keybdInput{wVk: vkBack, dwFlags: keyeventfKeyUp}},
	})
}

// SendEnter 模拟一次回车键（VK_RETURN）。
func SendEnter() error {
	inp := input{
		_type: inputKeyboard,
		ki: keybdInput{
			wVk:    vkReturn,
			dwFlags: 0,
		},
	}
	// KEYDOWN
	ret, _, _ := sendInputProc.Call(
		uintptr(1),
		uintptr(unsafe.Pointer(&inp)),
		uintptr(unsafe.Sizeof(inp)),
	)
	if ret == 0 {
		errCode, _, _ := lastErrorProc.Call()
		return fmt.Errorf("ENTER KEYDOWN 失败 GetLastError=%d", errCode)
	}
	// KEYUP
	inp.ki.dwFlags = keyeventfKeyUp
	ret, _, _ = sendInputProc.Call(
		uintptr(1),
		uintptr(unsafe.Pointer(&inp)),
		uintptr(unsafe.Sizeof(inp)),
	)
	if ret == 0 {
		errCode, _, _ := lastErrorProc.Call()
		return fmt.Errorf("ENTER KEYUP 失败 GetLastError=%d", errCode)
	}
	return nil
}

// StructSize 返回 INPUT / KEYBDINPUT 结构体大小，供调试验证
func StructSize() (inputSize, keybdSize int) {
	return int(unsafe.Sizeof(input{})), int(unsafe.Sizeof(keybdInput{}))
}
