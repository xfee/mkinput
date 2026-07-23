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

// SendText 将 text 逐字符通过 SendInput 模拟键盘输入到当前焦点窗口。
// 每个字符先发送 KEYDOWN（UNICODE），再发送 KEYUP。
// 返回首个出错字符的 error，全部成功则返回 nil。
func SendText(text string) error {
	runes := []rune(text)
	for i, r := range runes {
		inp := input{
			_type: inputKeyboard,
			ki: keybdInput{
				wScan:   uint16(r),
				dwFlags: keyeventfUnicode,
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
			return fmt.Errorf("KEYDOWN 失败 [%d/%d] char=U+%04X  GetLastError=%d  size=%d",
				i+1, len(runes), r, errCode, unsafe.Sizeof(inp))
		}

		// KEYUP
		inp.ki.dwFlags = keyeventfUnicode | keyeventfKeyUp
		ret, _, _ = sendInputProc.Call(
			uintptr(1),
			uintptr(unsafe.Pointer(&inp)),
			uintptr(unsafe.Sizeof(inp)),
		)
		if ret == 0 {
			errCode, _, _ := lastErrorProc.Call()
			return fmt.Errorf("KEYUP 失败 [%d/%d] char=U+%04X  GetLastError=%d",
				i+1, len(runes), r, errCode)
		}
	}
	return nil
}

// SendBackspace 模拟 n 次退格键（删除 count 个字符）。
// 每按一次发送 KEYDOWN + KEYUP，使用虚拟键码 VK_BACK（0x08）。
func SendBackspace(count int) error {
	for i := 0; i < count; i++ {
		// KEYDOWN
		inp := input{
			_type: inputKeyboard,
			ki: keybdInput{
				wVk:    vkBack,
				dwFlags: 0,
			},
		}
		ret, _, _ := sendInputProc.Call(
			uintptr(1),
			uintptr(unsafe.Pointer(&inp)),
			uintptr(unsafe.Sizeof(inp)),
		)
		if ret == 0 {
			errCode, _, _ := lastErrorProc.Call()
			return fmt.Errorf("BACKSPACE KEYDOWN 失败 [%d/%d] GetLastError=%d",
				i+1, count, errCode)
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
			return fmt.Errorf("BACKSPACE KEYUP 失败 [%d/%d] GetLastError=%d",
				i+1, count, errCode)
		}
	}
	return nil
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
