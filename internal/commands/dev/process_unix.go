//go:build darwin || linux

package dev

import (
	"os/exec"
	"syscall"
)

func prepareProcess(process *exec.Cmd) {
	process.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func interruptProcess(process *exec.Cmd) {
	if process.Process != nil {
		_ = syscall.Kill(-process.Process.Pid, syscall.SIGINT)
	}
}

func killProcess(process *exec.Cmd) {
	if process.Process != nil {
		_ = syscall.Kill(-process.Process.Pid, syscall.SIGKILL)
	}
}
