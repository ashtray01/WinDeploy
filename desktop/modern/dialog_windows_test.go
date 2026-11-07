package main

import (
	"encoding/binary"
	"testing"
	"unsafe"
)

func TestTaskDialogButtonsAreOneBytePacked(t *testing.T) {
	ptrSize := int(unsafe.Sizeof(uintptr(0)))
	buttons := encodeTaskDialogButtons(ptrSize, []int32{101, 102}, []uintptr{0x1111, 0x2222})
	if len(buttons) != 2*(4+ptrSize) {
		t.Fatalf("unexpected button array size: %d", len(buttons))
	}
	if binary.LittleEndian.Uint32(buttons[0:]) != 101 || binary.LittleEndian.Uint32(buttons[4:]) != 0x1111 {
		t.Fatalf("first button layout is wrong: %x", buttons)
	}
	second := 4 + ptrSize
	if binary.LittleEndian.Uint32(buttons[second:]) != 102 {
		t.Fatalf("second button ID is wrong: %x", buttons)
	}
}

func TestTaskDialogConfigLayout(t *testing.T) {
	ptrSize := int(unsafe.Sizeof(uintptr(0)))
	buttons := make([]byte, 4+ptrSize)
	title, instruction, content := uint16(1), uint16(2), uint16(3)
	config := encodeTaskDialogConfig(ptrSize, taskDialogValues{
		Flags:           tdfAllowDialogCancellation,
		Title:           &title,
		MainInstruction: &instruction,
		Content:         &content,
		Buttons:         buttons,
		DefaultButton:   101,
	})
	if len(config) != 160 {
		t.Fatalf("unexpected amd64 config size: %d", len(config))
	}
	if binary.LittleEndian.Uint32(config) != 160 {
		t.Fatalf("cbSize is wrong: %d", binary.LittleEndian.Uint32(config))
	}
	if binary.LittleEndian.Uint32(config[20:]) != tdfAllowDialogCancellation {
		t.Fatalf("flags offset is wrong: %x", config[20:24])
	}
	titleAddress := uint64(uintptr(unsafe.Pointer(&title)))
	instructionAddress := uint64(uintptr(unsafe.Pointer(&instruction)))
	contentAddress := uint64(uintptr(unsafe.Pointer(&content)))
	if binary.LittleEndian.Uint64(config[28:]) != titleAddress ||
		binary.LittleEndian.Uint64(config[44:]) != instructionAddress ||
		binary.LittleEndian.Uint64(config[52:]) != contentAddress {
		t.Fatalf("pointer offsets are wrong: %x", config)
	}
	if binary.LittleEndian.Uint32(config[72:]) != 101 {
		t.Fatalf("default button offset is wrong: %x", config[72:76])
	}
}
