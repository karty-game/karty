//go:build windows

package dev

import "os/exec"

func prepareProcess(*exec.Cmd) {}

func interruptProcess(process *exec.Cmd) {
	if process.Process != nil {
		_ = process.Process.Kill()
	}
}

func killProcess(process *exec.Cmd) {
	if process.Process != nil {
		_ = process.Process.Kill()
	}
}
