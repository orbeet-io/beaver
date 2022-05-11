package cmd

type BuildCmd struct {
	Args struct {
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
	Logger.Info().Str("namespace", cmd.Args.Namespace).Msg("Welcome buddy")
	return nil
}

func init() {
	buildCmd := NewBuildCmd()
	_, err := parser.AddCommand("build", "Build new environment", "", buildCmd)
	if err != nil {
		Logger.Fatal().Msg(err.Error())
	}
}
