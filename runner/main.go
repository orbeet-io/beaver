package runner

type Runner struct {
	config *Config
}

// NewRunner ...
func NewRunner(cfg *Config) *Runner {
	return &Runner{
		config: cfg,
	}
}

// Build is in charge of applying commands based on the config data
func (r *Runner) Build() error {
	return nil
}
