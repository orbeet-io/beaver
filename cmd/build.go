package cmd

import "orus.io/cloudcrane/beaver/runner"

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
	if err := config.Initialize(); err != nil {
		Logger.Err(err).Msg("failed to prepare config")
	}
	r := runner.NewRunner(config)
	return r.Build()
}

func init() {
	buildCmd := NewBuildCmd()
	_, err := parser.AddCommand("build", "Build new environment", "", buildCmd)
	if err != nil {
		Logger.Fatal().Msg(err.Error())
	}
}
