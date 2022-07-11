package cmd

import (
	"fmt"
	"os"

	"orus.io/orus-io/beaver/runner"
)

type BuildCmd struct {
	Args struct {
		DryRun bool   `short:"d" long:"dry-run" description:"if set only prints commands but do not run them"`
		Keep   bool   `short:"k" long:"keep" descriptions:"Keep the temporary files"`
		Output string `short:"o" long:"output" descriptions:"output directory"`
	}
	PositionnalArgs struct {
		DirName string `required:"yes" positional-arg-name:"directory"`
	} `positional-args:"yes"`
}

// NewBuildCmd ...
func NewBuildCmd() *BuildCmd {
	cmd := BuildCmd{}
	return &cmd
}

// Execute ...
func (cmd *BuildCmd) Execute([]string) error {
	Logger.Info().Str("directory", cmd.PositionnalArgs.DirName).Msg("starting beaver")

	config := runner.NewCmdConfig(Logger, ".", cmd.PositionnalArgs.DirName, cmd.Args.DryRun, cmd.Args.Output)

	path, err := os.Getwd()
	if err != nil {
		Logger.Fatal().Err(err).Msg("cannot get current working directory")
	}

	tmpDir, err := os.MkdirTemp(path, ".beaver-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	if !cmd.Args.Keep {
		defer func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				Logger.Err(err).Str("tempdir", tmpDir).Msg("failed to remove temp dir")
			}
		}()
	}

	if err := config.Initialize(tmpDir); err != nil {
		return fmt.Errorf("failed to prepare config: %w", err)
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
