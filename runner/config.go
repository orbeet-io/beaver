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
	Type   string      `mapstructure:"type"`
	Name   string      `mapstructure:"name"`
	Values interface{} `mapstructure:"values"`
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

// MergeVariables takes another config and will import those variables into
// the current config by replacing old ones and adding the new ones
func (c *Config) MergeVariables(other *Config) {
	for _, variable := range other.Spec.Variables {
		c.overlayVariable(variable)
	}
}

// overlayVariable takes a variable in and either replaces an existing variable
// of the same name or create a new variable in the config if no matching name
// is found
func (c *Config) overlayVariable(v Variable) {
	// find same variable by name and replace is value
	// if not found then create the variable
	for index, originalVariable := range c.Spec.Variables {
		if originalVariable.Name == v.Name {
			c.Spec.Variables[index].Value = v.Value
			return
		}
	}
	c.Spec.Variables = append(c.Spec.Variables, v)
}
