package main

import (
	"os/exec"
	"runtime"
)

func openBrowser() {
	switch runtime.GOOS {
	case "windows":
		runCommand("rundll32.exe", "url.dll,FileProtocolHandler")
	case "darwin":
		runCommand("open")
	case "linux":
		runCommand("xdg-open")
	}
}

func runCommand(cmd string, args ...string) {
	args = append(args, "http://localhost:"+getPortNumber("http")+"/mystream/publish")
	err := exec.Command(cmd, args...).Start()
	if err != nil {
		panic(err)
	}
}
