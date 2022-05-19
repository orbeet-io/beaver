package runner

import (
	"fmt"
	"strings"

	"github.com/go-cmd/cmd"
)

type Runner struct {
	config *CmdConfig
}

// NewRunner ...
func NewRunner(cfg *CmdConfig) *Runner {
	return &Runner{
		config: cfg,
	}
}

// Build is in charge of applying commands based on the config data
func (r *Runner) Build() error {
	// create helm commands
	// create ytt chart commands
	var cmds []*cmd.Cmd
	for name, chart := range r.config.Spec.Charts {
		args, err := chart.BuildArgs(name, r.config.Namespace)
		if err != nil {
			return fmt.Errorf("build: failed to build args %w", err)
		}
		switch chart.Type {
		case HelmType:
			cmds = append(cmds, cmd.NewCmd("/path/to/helm", args...))
		case YttType:
			cmds = append(cmds, cmd.NewCmd("/path/to/ytt", args...))
		default:
			return fmt.Errorf("unsupported chart %s type: %q", chart.Path, chart.Type)
		}
	}

	// create ytt additional command

	// run commands or print them
	if r.config.DryRun {
		for _, helmCmd := range cmds {
			r.config.Logger.Info().
				Str("command", helmCmd.Name).
				Str("args", strings.Join(helmCmd.Args, " ")).
				Msg("would run command")
		}
	} else {
		for _, helmCmd := range cmds {
			err, sdtOut, stdErr := RunCMD(helmCmd)
			if err != nil {
				r.config.Logger.Err(err).
					Str("command", helmCmd.Name).
					Str("args", strings.Join(helmCmd.Args, " ")).
					Str("sdtout", strings.Join(sdtOut, "\n")).
					Str("stderr", strings.Join(stdErr, "\n")).
					Msg("failed to run command")

				return fmt.Errorf("failed to run command: %w", err)
			}
		}
	}

	return nil
}
