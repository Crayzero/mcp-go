//go:build !windows
// +build !windows

package transport

import (
	"os"
	"os/exec"
	"syscall"
)

// killprocess kills the process on non-windows platforms.
func killProcess(proc *os.Process) error {
	// if the process is still running, kill it
	return syscall.Kill(-proc.Pid, syscall.SIGKILL)
}

func setProcessAttributes(cmd *exec.Cmd) {
	// Set the process attributes to inherit the job object
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func assignJob(cmd *exec.Cmd) error {
	return nil
}
