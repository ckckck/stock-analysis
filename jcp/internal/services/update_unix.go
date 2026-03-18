//go:build !windows

package services

import "os/exec"

// setSysProcAttr Unix 系统不需要特殊处理
func setSysProcAttr(cmd *exec.Cmd) {
	// Unix 系统无需特殊设置
}
