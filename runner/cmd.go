package runner

import (
	"fmt"
	"strings"

	"github.com/go-cmd/cmd"
)

// RunCMD runs the given cmd and returns its stdout, stderr and
// an eventual error.
func RunCMD(c *cmd.Cmd) ([]string, []string, error) {
	statusChan := c.Start()

	status := <-statusChan
	if status.Error != nil || status.Exit > 0 {
		return status.Stdout, status.Stderr, fmt.Errorf(
			"cannot execute command: %s with output: %s",
			c.Name,
			strings.Join(status.Stdout, " "))
	}

	return status.Stdout, status.Stderr, nil
}
