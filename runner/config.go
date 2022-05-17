package runner

import (
	"github.com/spf13/viper"
)

// Variable ...
type Variable struct {
	Name  string `mapstructure:"name"`
	Value string `mapstructure:"value"`
}

type Chart struct {
	Type string `mapstructure:"type"`
	Path string `mapstructure:"path"`
}

// Spec ...
type Spec struct {
	Variables []Variable       `mapstructure:"variables"`
	Charts    map[string]Chart `mapstructure:"charts"`
}

// Config is the configuration we get after parsing our beaver.yml file
type Config struct {
	APIVersion string `mapstructure:"apiVersion"`
	Kind       string `mapstructure:"kind"`
	Spec       Spec   `mapstructure:"spec"`
}

// NewConfig returns a *Config
func NewConfig(configDir string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("beaver")
	v.AddConfigPath(configDir)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var config Config
	cfg := &config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
