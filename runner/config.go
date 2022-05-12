package runner

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
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

type Chart struct {
	Type   string      `mapstructure:"type"`
	Name   string      `mapstructure:"name"`
	Values interface{} `mapstructure:"values"`
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
	Namespace  string
	Logger     zerolog.Logger
}

// NewConfig returns a *Config
func NewConfig(logger zerolog.Logger, configDir string, namespace string) (*Config, error) {
	// we ONLY search for files named beaver.yml
	viper.SetConfigName("beaver")
	viper.AddConfigPath(configDir)
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	var config Config
	cfg := &config
	cfg.Namespace = namespace
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := cfg.hydrate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// hydrate expands templated variables in our config with concrete values
func (c *Config) hydrate() error {
	if err := c.hydrateCharts(); err != nil {
		return err
	}
	return nil
}

func (c *Config) prepareVariables(v []Variable) map[string]string {
	variables := make(map[string]string)
	for _, variable := range v {
		variables[variable.Name] = variable.Value
	}
	variables["namespace"] = c.Namespace
	return variables
}

/*
func (c *Config) hydrateYtt() error {
	for index, entry := range c.Spec.Ytt {
		valueTmpl, err := template.New("entry").Parse(entry.Value)
		if err != nil {
			return fmt.Errorf("failed to parse ytt entry value as template: %q, %w", entry.Value, err)
		}
		buf := new(bytes.Buffer)
		if err := valueTmpl.Execute(buf, c.prepareVariables(c.Spec.Variables)); err != nil {
			return fmt.Errorf("failed to hydrate ytt entry: %q, %w", entry.Value, err)
		}
		// replace original content with hydrated version
		c.Spec.Ytt[index].Value = buf.String()
	}
	return nil
}
*/

func (c *Config) hydrateCharts() error {
	for name, chart := range c.Spec.Charts {
		rawChartValues, err := yaml.Marshal(chart.Values)
		if err != nil {
			return fmt.Errorf("failed to get chart values as string: %w", err)
		}
		valueTmpl, err := template.New("chart").Parse(string(rawChartValues))
		if err != nil {
			return fmt.Errorf("failed to parse chart values as template: %q, %w", chart.Values, err)
		}
		buf := new(bytes.Buffer)
		if err := valueTmpl.Execute(buf, c.prepareVariables(c.Spec.Variables)); err != nil {
			return fmt.Errorf("failed to hydrate chart values entry: %q, %w", chart.Values, err)
		}
		// replace original content with hydrated version
		hydratedChart := chart
		hydratedChart.Values = buf.String()
		c.Spec.Charts[name] = hydratedChart
	}
	return nil
}
