//go:build windows

package i18n

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

func detectPlatformLocales() []string {
	var candidates []string
	if value := localeNameFromProc("GetUserDefaultLocaleName"); value != "" {
		candidates = append(candidates, value)
	}
	if value := localeNameFromProc("GetSystemDefaultLocaleName"); value != "" {
		candidates = append(candidates, value)
	}
	return candidates
}

func localeNameFromProc(name string) string {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	proc := kernel32.NewProc(name)
	buf := make([]uint16, 85)
	size, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if size == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}
