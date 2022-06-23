package runner

import (
	"fmt"

	"github.com/go-cmd/cmd"
)

// RunCMD runs the given cmd and returns its stdout, stderr and
// an eventual error
func RunCMD(c *cmd.Cmd) (stdout, stderr []string, err error) {
	statusChan := c.Start()
	status := <-statusChan
	if status.Error != nil || status.Exit > 0 {
		return status.Stdout, status.Stderr, fmt.Errorf("cannot execute command: %w", err)
	}
	stdout = status.Stdout
	stderr = status.Stderr
	return
}
