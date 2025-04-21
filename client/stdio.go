package client

import (
	"context"
	"fmt"
	"io"

	"git.woa.com/copilot-chat/copilot_agent/mcp-go/client/transport"
)

// NewStdioMCPClient creates a new stdio-based MCP client that communicates with a subprocess.
// It launches the specified command with given arguments and sets up stdin/stdout pipes for communication.
// Returns an error if the subprocess cannot be started or the pipes cannot be created.
//
// NOTICE: NewStdioMCPClient will start the connection automatically. Don't call the Start method manually.
// This is for backward compatibility.
func NewStdioMCPClient(
	command string,
	env []string,
	args ...string,
) (*Client, error) {

	stdioTransport := transport.NewStdio(command, env, args...)
	err := stdioTransport.Start(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to start stdio transport: %w", err)
	}

	return NewClient(stdioTransport), nil
}

// Close shuts down the stdio client, closing the stdin pipe and waiting for the subprocess to exit.
// Returns an error if there are issues closing stdin or waiting for the subprocess to terminate.
func (c *StdioMCPClient) Close() error {
	close(c.done)
	if err := c.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}
	if err := c.stderr.Close(); err != nil {
		return fmt.Errorf("failed to close stderr: %w", err)
	}

	// Wait for the process to exit with a timeout
	errChan := make(chan error, 1)
	go func() {
		errChan <- c.cmd.Wait()
	}()

	select {
	case err := <-errChan:
		return err
	case <-time.After(3 * time.Second):
		if runtime.GOOS == "windows" {
			return killOnWindows(c.cmd.Process.Pid)
		}
		// Send SIGTERM if the process hasn't exited after 3 seconds
		if err := c.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to send SIGTERM: %w", err)
		}

		// Wait for another 1 second
		select {
		case err := <-errChan:
			return err
		case <-time.After(1 * time.Second):
			// Send SIGKILL if the process still hasn't exited
			if err := c.cmd.Process.Kill(); err != nil {
				return fmt.Errorf("failed to send SIGKILL: %w", err)
			}
			return <-errChan
		}
	}
}

// Stderr returns a reader for the stderr output of the subprocess.
// This can be used to capture error messages or logs from the subprocess.
//
// Note: This method only works with stdio transport, or it will panic.
func GetStderr(c *Client) io.Reader {
	t := c.GetTransport()
	stdio := t.(*transport.Stdio)
	return stdio.Stderr()
}

func killOnWindows(pid int) error {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return err
	}
	// 获取所有子进程（递归）
	children, err := proc.Children()
	if err == nil {
		for _, child := range children {
			err = killOnWindows(int(child.Pid)) // 递归杀子进程
			if err != nil {
				fmt.Printf("Failed to kill pid %d: %v\n", child.Pid, err)
			}
		}
	}

	// 杀掉当前进程
	p, err := os.FindProcess(int(pid))
	if err == nil {
		err = p.Kill()
		if err != nil {
			fmt.Printf("Failed to kill pid %d: %v\n", pid, err)
		}
	}
	return err
}
