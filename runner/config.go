package runner

import (
	"github.com/spf13/viper"
)

// Variable ...
type Variable struct {
	Name  string `mapstructure:"name"`
	Value string `mapstructure:"value"`
}

// Value ...
type Value struct {
	Key   string `mapstructure:"key"`
	Value string `mapstructure:"value"`
}

type HelmChart struct {
	Type   string                 `mapstructure:"type"`
	Name   string                 `mapstructure:"name"`
	Values map[string]interface{} `mapstructure:"values"`
}

type YttChart struct {
	Type   string  `mapstructure:"type"`
	Name   string  `mapstructure:"name"`
	Values []Value `mapstructure:"values"`
}

type Charts struct {
	Helm map[string]HelmChart `mapstructure:"helm"`
	Ytt  map[string]YttChart  `mapstructure:"ytt"`
}

// Spec ...
type Spec struct {
	Variables []Variable `mapstructure:"variables"`
	Charts    Charts     `mapstructure:"charts"`
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
