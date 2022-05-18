package runner

import (
	"github.com/go-cmd/cmd"
)

func RunCMD(c *cmd.Cmd) (err error, stdout, stderr []string) {
	statusChan := c.Start()
	status := <-statusChan
	if status.Error != nil {
		return err, status.Stdout, status.Stderr
	}
	stdout = status.Stdout
	stderr = status.Stderr
	return
}
