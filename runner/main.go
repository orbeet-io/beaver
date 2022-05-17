package runner

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

	// create ytt additional command
	return nil
}
