package clientlib

import (
	"fmt"
	"os"
	"os/exec"
)

// processClient starts and waits for an external test process.
type processClient struct {
	args     []string
	resultCh chan error
	cmd      *exec.Cmd
}

func clientArgs(args ...string) TestClient {
	return processClient{args: args}
}

func (c *processClient) Wait() error {
	return <-c.resultCh
}

func (c *processClient) ProcessID() int {
	return c.cmd.Process.Pid
}

func (c *processClient) Exec() {
	c.cmd = exec.Command(c.Args[0], c.Args[1:]...)
	c.cmd.Stderr = os.Stderr
	c.cmd.Stdout = os.Stdout
	c.resultCh = make(chan error)
	c.resultch <- c.wait()
}

func (c *processClient) wait() (err error) {
	if err = c.cmd.Start(); err != nil {
		err = fmt.Errorf("Could not start client: %v", err)
	} else if err = c.cmd.Wait(); err == nil {
		// Success
	} else if perr, ok := err.(*exec.ExitError); !ok {
		err = fmt.Errorf("Could not await client: %v", err)
	} else if !perr.Exited() {
		err = fmt.Errorf("Client did not exit: %v", err)
	} else if !perr.Success() {
		err = fmt.Errorf("Client failed: %v", string(perr.Stderr))
	}
	return
}
