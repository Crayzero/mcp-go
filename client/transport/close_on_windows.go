//go:build windows
// +build windows

package transport

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/windows"
)

// killByPid kills the process by pid on windows.
// It kills all subprocesses recursively.
func killByPid(pid int) error {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return err
	}
	// get all subprocess recursively
	children, err := proc.Children()
	if err == nil {
		for _, child := range children {
			err = killByPid(int(child.Pid)) // kill all subprocesses
			if err != nil {
				fmt.Printf("Failed to kill pid %d: %v\n", child.Pid, err)
			}
		}
	}

	// kill current process
	p, err := os.FindProcess(int(pid))
	if err == nil {
		// windows does not support SIGTERM, so we just use Kill()
		err = p.Kill()
		if err != nil {
			fmt.Printf("Failed to kill pid %d: %v\n", pid, err)
		}
	}
	return err
}

// KillProcess kills the process on windows.
func killProcess(p *os.Process) error {
	if p == nil {
		return nil
	}
	return killByPid(p.Pid)
}

func CreateJobObject() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}

	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

	ret, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		return 0, err
	}
	if ret == 0 {
		return 0, errors.New("SetInformationJobObject failed")
	}

	return job, nil
}

func CloseJob(job windows.Handle) error {
	return windows.CloseHandle(job)
}

func AssignProcessToJob(job windows.Handle, pid uint32) error {
	hProcess, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, pid)
	if err != nil {
		return err
	}
	defer func() {
		_ = windows.CloseHandle(hProcess)
	}()

	err = windows.AssignProcessToJobObject(job, hProcess)
	if err != nil {
		return err
	}
	return nil
}

func AssignAllProcessToJob(job windows.Handle, pid uint32) error {
	// 首先将当前进程加入作业
	hProcess, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, pid)
	if err != nil {
		return err
	}
	defer func() {
		_ = windows.CloseHandle(hProcess)
	}()

	err = windows.AssignProcessToJobObject(job, hProcess)
	if err != nil {
		return err
	}

	// 递归获取子进程
	hSnapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer func() {
		_ = windows.CloseHandle(hProcess)
	}()

	var pe32 windows.ProcessEntry32
	pe32.Size = uint32(unsafe.Sizeof(pe32))

	err = windows.Process32First(hSnapshot, &pe32)
	if err != nil {
		return err
	}

	for {
		if pe32.ParentProcessID == pid {
			// 递归处理子进程
			err = AssignAllProcessToJob(job, pe32.ProcessID)
			if err != nil {
				return err
			}
		}

		err = windows.Process32Next(hSnapshot, &pe32)
		if err != nil {
			break
		}
	}

	return nil
}

func setProcessAttributes(cmd *exec.Cmd) {
	// Set the process attributes to inherit the job object
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow: true,
		// Set the job object to inherit the process
		CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB,
	}
}

var jobHandle windows.Handle

func init() {
	// Create a job object for the process
	var err error
	jobHandle, err = CreateJobObject()
	if err != nil {
		fmt.Printf("Failed to create job object: %v\n", err)
	}
}

// GetGlobalJobHandle returns the global job handle.
func GetGlobalJobHandle() windows.Handle {
	return jobHandle
}

func assignJob(cmd *exec.Cmd) error {
	if jobHandle == 0 {
		return errors.New("job handle is not valid")
	}
	err := AssignAllProcessToJob(jobHandle, uint32(cmd.Process.Pid))
	if err != nil {
		return fmt.Errorf("failed to assign process to job: %v", err)
	}
	return nil
}
