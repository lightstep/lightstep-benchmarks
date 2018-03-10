package clientlib

import (
	"fmt"
	"os"
	"os/exec"
)

type ProcessClient struct {
	Args      []string
	stoppedCh chan bool
	cmd       *exec.Cmd
}

func CreateProcessClient(args []string) TestClient {
	return &ProcessClient{
		Args: args,
	}
}

func (c *ProcessClient) WaitForExit() {
	<-c.stoppedCh
}

func (c *ProcessClient) Pid() int {
	return c.cmd.Process.Pid
}

func (c *ProcessClient) Start() error {
	c.stoppedCh = make(chan bool)

	c.cmd = exec.Command(c.Args[0], c.Args[1:]...)
	c.cmd.Stderr = os.Stderr
	c.cmd.Stdout = os.Stdout
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("Could not start client: %v", err)
	}
	// Start watch goroutine
	go func() {
		if err := c.cmd.Wait(); err != nil {
			perr, ok := err.(*exec.ExitError)
			if !ok {
				panic(fmt.Errorf("Could not await client: %v", err))
			}
			if !perr.Exited() {
				panic(fmt.Errorf("Client did not exit: %v", err))
			}
			if !perr.Success() {
				panic(fmt.Errorf("Client failed: %v", string(perr.Stderr)))
			}
		}
		c.stoppedCh <- true
	}()
	return nil
}
