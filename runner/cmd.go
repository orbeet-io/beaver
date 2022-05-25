package runner

import (
	"fmt"

	"github.com/go-cmd/cmd"
)

func RunCMD(c *cmd.Cmd) (err error, stdout, stderr []string) {
	statusChan := c.Start()
	status := <-statusChan
	if status.Error != nil || status.Exit > 0 {
		return fmt.Errorf("Cannot execute command: %w", err), status.Stdout, status.Stderr
	}
	stdout = status.Stdout
	stderr = status.Stderr
	return
}
