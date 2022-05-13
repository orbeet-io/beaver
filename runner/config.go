package runner

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"text/template"

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

func checkExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}

// hydrate expands templated variables in our config with concrete values
func (c *Config) hydrate() error {
	if err := c.hydrateHelmCharts(); err != nil {
		return err
	}
	if err := c.hydrateYttCharts(); err != nil {
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

func (c *Config) hydrateYttCharts() error {
	for entryFileName, entry := range c.Spec.Charts.Ytt {
		for valIndex, val := range entry.Values {
			valueTmpl, err := template.New("ytt entry value").Parse(val.Value)
			if err != nil {
				return fmt.Errorf("failed to parse ytt entry value as template: %q, %w", val.Value, err)
			}
			buf := new(bytes.Buffer)
			if err := valueTmpl.Execute(buf, c.prepareVariables(c.Spec.Variables)); err != nil {
				return fmt.Errorf("failed to hydrate ytt entry: %q, %w", val.Value, err)
			}
			// replace original content with hydrated version
			c.Spec.Charts.Ytt[entryFileName].Values[valIndex].Value = buf.String()
		}
	}
	return nil
}

func (c *Config) hydrateHelmCharts() error {
	for name, chart := range c.Spec.Charts.Helm {
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
		c.Spec.Charts.Helm[name] = hydratedChart
	}
	return nil
}
