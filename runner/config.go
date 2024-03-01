package runner

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Sha define sha feature parameter
type Sha struct {
	// Key is used to generate beaver variable
	// beaver variable will be `sha.<Key>`
	Key string
	// Resource file from which we should compute the sha256
	// Same format as beaver output:
	// <kind>.<apiVersion>.<name>.yaml
	Resource string
}

// Chart define a chart to compile
type Chart struct {
	// Type: chart type, can be either `ytt` or `helm`
	Type string
	// Path: relative path to the chart itself
	Path string
	// Name: overwrite helm application name
	Name string
	// Namespace: will be pass to helm as parameter (helm only)
	Namespace string
	// Disabled: disable this chart
	// This can be useful when inheriting the chart
	// must be castable to bool (0,1,true,false)
	Disabled string
}

// Arg define command line arguments
type Arg struct {
	// Flag: is a CLI flag, eg. `--from-file`
	Flag string
	// Value: is the value for this flag, eg. `path/to/my/files`
	Value string
}

// Create define kubectl create command invocation
type Create struct {
	// Type: of resource you want to create with kubectl, eg. configmap
	Type string
	// Name: resource name
	Name string
	// Args: list of Arg pass to kubectl create command
	Args []Arg `yaml:",flow"`
}

// Config represent the beaver.yaml config file
type Config struct {
	// Inherit: relative path to another beaver project
	// a beaver project is a folder with a beaver.yaml file
	Inherit string
	// BeaverVersion: the beaver version this config is supposed to work with.
	BeaverVersion string
	// NameSpace: a kubernetes Namespace, shouldn't be mandatory
	NameSpace string
	// Inherits: list of relative path to other beaver projects
	Inherits []string `yaml:",flow"`
	// Variables: list of beaver variables
	Variables Variables `yaml:",flow"`
	// Sha: list of Sha
	Sha []Sha `yaml:",flow"`
	// Charts: map of charts definitions, where the key is the chart name for beaver values.
	// eg. beaver will use `foo.yaml` for a chart with key `foo`
	Charts map[string]Chart `yaml:",flow"`
	// Creates: list of kubectl create commands
	Creates []Create `yaml:"create,flow"`
	// Dir: internal use
	Dir string // the directory in which we found the config file
}

// Absolutize makes all chart paths absolute
func (c *Config) Absolutize(dir string) error {
	for name, chart := range c.Charts {
		resolvedChartPath := filepath.Join(dir, chart.Path)
		absChartPath, err := filepath.Abs(resolvedChartPath)
		if err != nil {
			return fmt.Errorf("failed to find abs() for %s: %w", resolvedChartPath, err)
		}

		chart.Path = absChartPath
		c.Charts[name] = chart
	}
	return nil
}

// NewConfig returns a *Config
func NewConfig(configDir string) (*Config, error) {
	var configName = "beaver"
	config := Config{}

	for _, ext := range []string{"yaml", "yml"} {
		configPath := filepath.Join(configDir, fmt.Sprintf("%s.%s", configName, ext))
		configInfo, err := os.Stat(configPath)
		if err != nil || configInfo.IsDir() {
			continue
		}
		configFile, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("fail to read config file: %s - %w", configPath, err)
		}
		err = yaml.Unmarshal(configFile, &config)
		if err != nil {
			return nil, fmt.Errorf("fail unmarshal config file: %s - %w", configPath, err)
		}
		return &config, nil
	}

	return nil, fmt.Errorf("no beaver file found in %s", configDir)
}
