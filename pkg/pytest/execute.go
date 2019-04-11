package pytest

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
	"time"

	xpytest_proto "github.com/chainer/xpytest/proto"
)

// Execute executes a command.
func Execute(
	ctx context.Context, args []string, deadline time.Duration, env []string,
) (*xpytest_proto.TestResult, error) {
	startTime := time.Now()
	result := &xpytest_proto.TestResult{}

	// Prepare a Cmd object.
	if len(args) == 0 {
		return nil, fmt.Errorf("# of args must be larger than 0")
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	// Open pipes.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %s", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %s", err)
	}

	// Set environment variables.
	if env == nil {
		env = []string{}
	}
	env = append(env, os.Environ()...)
	cmd.Env = env

	// Start the command.
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %s", err)
	}

	// Prepare a wait group to maintain threads.
	wg := sync.WaitGroup{}
	async := func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f()
		}()
	}

	// Run I/O threads.
	readAll := func(pipe io.Reader) string {
		b, err := ioutil.ReadAll(pipe)
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"[ERROR] failed to read from pipe: %s\n", err)
		}
		if b == nil {
			return ""
		}
		return string(b)
	}
	async(func() { result.Stdout = readAll(stdoutPipe) })
	async(func() { result.Stderr = readAll(stderrPipe) })

	// Run timer thread.
	var timeout bool
	cmdIsDone := make(chan struct{}, 1)
	async(func() {
		select {
		case <-cmdIsDone:
		case <-time.After(deadline):
			timeout = true
			cmd.Process.Kill()
		}
	})

	// Wait for the command.
	cmd.Wait()
	close(cmdIsDone)
	wg.Wait()

	// Get the last line.
	if timeout {
		result.Status = xpytest_proto.TestResult_TIMEOUT
	} else if cmd.ProcessState.Success() {
		result.Status = xpytest_proto.TestResult_SUCCESS
	} else {
		result.Status = xpytest_proto.TestResult_FAILED
	}

	result.Time = float32(time.Now().Sub(startTime)) / float32(time.Second)
	return result, nil
}
