//go:build windows

package services

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr 设置 Windows 进程属性（隐藏窗口）
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
}
