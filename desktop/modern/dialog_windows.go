package main

import (
	"encoding/binary"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TaskDialogIndirect lets the startup recovery dialog use explicit "continue" and
// "exit" labels. It never requests elevation. comctl32 declares
// TASKDIALOGCONFIG inside pshpack1.h, so it is serialized byte-for-byte instead of
// mapped to a naturally aligned Go struct.
const (
	tdfAllowDialogCancellation = 0x00000008
	fallbackButtonContinue     = 101
	fallbackButtonExit         = 102
)

var procTaskDialogIndirect = windows.NewLazySystemDLL("comctl32.dll").NewProc("TaskDialogIndirect")

func showFallbackChoice(fallbackDir string) bool {
	title, _ := windows.UTF16PtrFromString("WinDeploy — WebView2")
	instruction, _ := windows.UTF16PtrFromString("Используется резервный каталог WebView2")
	content, _ := windows.UTF16PtrFromString(
		"Основной каталог данных WebView2 недоступен для записи.\n\n" +
			fallbackDir + "\n\n" +
			"Это каталог данных WebView2, а не переносимая копия Runtime.\n" +
			"Можно продолжить с ним без повышения прав. Существующие данные\n" +
			"не удаляются, системные настройки не изменяются.")
	buttonContinue, _ := windows.UTF16PtrFromString("Продолжить с резервным каталогом")
	buttonExit, _ := windows.UTF16PtrFromString("Выход")

	ptrSize := int(unsafe.Sizeof(uintptr(0)))
	labels := []uintptr{
		uintptr(unsafe.Pointer(buttonContinue)),
		uintptr(unsafe.Pointer(buttonExit)),
	}
	ids := []int32{fallbackButtonContinue, fallbackButtonExit}
	buttons := encodeTaskDialogButtons(ptrSize, ids, labels)
	config := encodeTaskDialogConfig(ptrSize, taskDialogValues{
		Flags:           tdfAllowDialogCancellation,
		Title:           title,
		MainInstruction: instruction,
		Content:         content,
		Buttons:         buttons,
		DefaultButton:   fallbackButtonContinue,
	})

	if err := procTaskDialogIndirect.Find(); err != nil {
		return fallbackMessageBox(title, fallbackDir)
	}
	var selected int32
	hr, _, _ := procTaskDialogIndirect.Call(
		uintptr(unsafe.Pointer(&config[0])),
		uintptr(unsafe.Pointer(&selected)),
		0,
		0,
	)
	runtime.KeepAlive(title)
	runtime.KeepAlive(instruction)
	runtime.KeepAlive(content)
	runtime.KeepAlive(buttonContinue)
	runtime.KeepAlive(buttonExit)
	runtime.KeepAlive(buttons)
	runtime.KeepAlive(config)
	if int32(hr) < 0 {
		logStation("TaskDialogIndirect недоступен: HRESULT 0x%08X", uint32(hr))
		return fallbackMessageBox(title, fallbackDir)
	}
	return selected == fallbackButtonContinue
}

func fallbackMessageBox(title *uint16, fallbackDir string) bool {
	fallback, _ := windows.UTF16PtrFromString(
		"WinDeploy использует резервный каталог WebView2:\n" + fallbackDir + "\n\n" +
			"Это каталог данных, а не копия Runtime. Повышение прав не требуется.\n\n" +
			"ОК — продолжить; Отмена — выход.")
	result, _ := windows.MessageBox(0, fallback, title, windows.MB_OKCANCEL|windows.MB_ICONWARNING|windows.MB_DEFBUTTON1)
	return result == 1 // IDOK
}

type taskDialogValues struct {
	Flags           uint32
	Title           *uint16
	MainInstruction *uint16
	Content         *uint16
	Buttons         []byte
	DefaultButton   int32
}

func encodeTaskDialogButtons(ptrSize int, ids []int32, labels []uintptr) []byte {
	size := 4 + ptrSize
	buffer := make([]byte, size*len(ids))
	for i := range ids {
		offset := i * size
		binary.LittleEndian.PutUint32(buffer[offset:], uint32(ids[i]))
		putPointer(buffer[offset+4:], ptrSize, labels[i])
	}
	return buffer
}

func encodeTaskDialogConfig(ptrSize int, values taskDialogValues) []byte {
	var buffer []byte
	u32 := func(value uint32) { buffer = binary.LittleEndian.AppendUint32(buffer, value) }
	ptr := func(value uintptr) {
		part := make([]byte, ptrSize)
		putPointer(part, ptrSize, value)
		buffer = append(buffer, part...)
	}
	u32(0) // cbSize patched below
	ptr(0) // hwndParent
	ptr(0) // hInstance
	u32(values.Flags)
	u32(0) // dwCommonButtons
	ptr(uintptr(unsafe.Pointer(values.Title)))
	ptr(0) // pszMainIcon
	ptr(uintptr(unsafe.Pointer(values.MainInstruction)))
	ptr(uintptr(unsafe.Pointer(values.Content)))
	u32(uint32(len(values.Buttons) / (4 + ptrSize)))
	if len(values.Buttons) > 0 {
		ptr(uintptr(unsafe.Pointer(&values.Buttons[0])))
	} else {
		ptr(0)
	}
	u32(uint32(values.DefaultButton))
	u32(0) // cRadioButtons
	ptr(0) // pRadioButtons
	u32(0) // nDefaultRadioButton
	ptr(0) // pszVerificationText
	ptr(0) // pszExpandedInformation
	ptr(0) // pszExpandedControlText
	ptr(0) // pszCollapsedControlText
	ptr(0) // hFooterIcon
	ptr(0) // pszFooter
	ptr(0) // pfCallback
	ptr(0) // lpCallbackData
	u32(0) // cxWidth
	binary.LittleEndian.PutUint32(buffer, uint32(len(buffer)))
	return buffer
}

func putPointer(buffer []byte, ptrSize int, value uintptr) {
	if ptrSize == 8 {
		binary.LittleEndian.PutUint64(buffer, uint64(value))
		return
	}
	binary.LittleEndian.PutUint32(buffer, uint32(value))
}
