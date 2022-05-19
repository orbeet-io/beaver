package runner

import (
	"fmt"
	"io/ioutil"
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
func (r *Runner) Build(tmpDir string) error {
	// create helm commands
	// create ytt chart commands
	var cmds map[string]*cmd.Cmd
	for name, chart := range r.config.Spec.Charts {
		args, err := chart.BuildArgs(name, r.config.Namespace)
		if err != nil {
			return fmt.Errorf("build: failed to build args %w", err)
		}
		switch chart.Type {
		case HelmType:
			cmds[name] = cmd.NewCmd("/path/to/helm", args...)
		case YttType:
			cmds[name] = cmd.NewCmd("/path/to/ytt", args...)
		default:
			return fmt.Errorf("unsupported chart %s type: %q", chart.Path, chart.Type)
		}
	}

	// run commands or print them
	var compiled []string
	if r.config.DryRun {
		for _, helmCmd := range cmds {
			r.config.Logger.Info().
				Str("command", helmCmd.Name).
				Str("args", strings.Join(helmCmd.Args, " ")).
				Msg("would run command")
		}
	} else {
		for name, cmd := range cmds {
			err, stdOut, stdErr := RunCMD(cmd)
			if err != nil {
				r.config.Logger.Err(err).
					Str("command", cmd.Name).
					Str("args", strings.Join(cmd.Args, " ")).
					Str("sdtout", strings.Join(stdOut, "\n")).
					Str("stderr", strings.Join(stdErr, "\n")).
					Msg("failed to run command")

				return fmt.Errorf("failed to run command: %w", err)
			}
			if tmpFile, err := ioutil.TempFile(tmpDir, fmt.Sprintf("compiled-%s-", name)); err != nil {
				return fmt.Errorf("cannot create compiled file: %w", err)
			} else {
				defer tmpFile.Close()
				if _, err := tmpFile.WriteString(strings.Join(stdOut, "\n")); err != nil {
					return fmt.Errorf("cannot write compiled file: %w", err)
				}
				compiled = append(compiled, tmpFile.Name())
			}
		}
	}

	// create ytt additional command
	args := r.config.Spec.Ytt.BuildArgs(r.config.Namespace, compiled)

	cmd := cmd.NewCmd("/path/to/ytt", args...)
	err, stdOut, stdErr := RunCMD(cmd)
	if err != nil {
		r.config.Logger.Err(err).
			Str("command", cmd.Name).
			Str("args", strings.Join(cmd.Args, " ")).
			Str("sdtout", strings.Join(stdOut, "\n")).
			Str("stderr", strings.Join(stdErr, "\n")).
			Msg("failed to run command")

		return fmt.Errorf("failed to run command: %w", err)
	}

	// TODO read and split resources on stdout and
	// then write those to build/<namespace>/<apiVersion>.<kind>.<name>.yaml

	return nil
}
