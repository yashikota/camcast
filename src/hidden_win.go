//go:build windows

package main

import (
	"log"
	"runtime"
	"syscall"
)

func setHiddenAttribute(dir string) {
	if runtime.GOOS == "windows" {
		dirName, err := syscall.UTF16PtrFromString(dir)
		if err != nil {
			log.Fatalf("Failed to convert directory name to UTF-16: %v", err)
		}
		if err := syscall.SetFileAttributes(dirName, syscall.FILE_ATTRIBUTE_HIDDEN); err != nil {
			log.Fatalf("Failed to set hidden directory attribute: %v", err)
		}
	}
}
