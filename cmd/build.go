package cmd

import (
	"fmt"
	"os"

	"orus.io/cloudcrane/beaver/runner"
)

type BuildCmd struct {
	Args struct {
		DryRun bool `short:"d" long:"dry-run" description:"if set only prints commands but do not run them"`
	}
	PositionnalArgs struct {
		Namespace string `required:"yes" positional-arg-name:"namespace"`
	} `positional-args:"yes"`
}

// NewBuildCmd ...
func NewBuildCmd() *BuildCmd {

	cmd := BuildCmd{}

	return &cmd
}

// Execute ...
func (cmd *BuildCmd) Execute([]string) error {
	Logger.Info().Str("namespace", cmd.PositionnalArgs.Namespace).Msg("starting beaver")

	config := runner.NewCmdConfig(Logger, ".", cmd.PositionnalArgs.Namespace, cmd.Args.DryRun)

	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	if !cmd.Args.DryRun {
		defer func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				Logger.Err(err).Str("tempdir", tmpDir).Msg("failed to remove temp dir")
			}
		}()
	}

	if err := config.Initialize(tmpDir); err != nil {
		Logger.Err(err).Msg("failed to prepare config")
	}
	r := runner.NewRunner(config)
	return r.Build(tmpDir)
}

func init() {
	buildCmd := NewBuildCmd()
	_, err := parser.AddCommand("build", "Build new environment", "", buildCmd)
	if err != nil {
		Logger.Fatal().Msg(err.Error())
	}
}
