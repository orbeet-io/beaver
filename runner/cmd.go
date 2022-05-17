package runner

import (
	"github.com/go-cmd/cmd"
)

func RunCMD(name string, args ...string) (err error, stdout, stderr []string) {
	c := cmd.NewCmd(name, args...)
	statusChan := c.Start()
	status := <-statusChan
	if status.Error != nil {
		return err, status.Stdout, status.Stderr
	}
	stdout = status.Stdout
	stderr = status.Stderr
	return
}
