package runner

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/go-cmd/cmd"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"
)

func RunCMD(name string, args ...string) (err error, stdout, stderr []string) {
	// helm template -f base.yaml -f base.values.yaml -f ns.yaml -f ns.values.yaml
	// ytt -f /chart-folder -f base.yaml -f ns.yaml -v ... -v ...
	c := cmd.NewCmd(name, args...)
	statusChan := c.Start()
	status := <-statusChan
	if status.Error != nil {
		return err, status.Stdout, status.Stderr
	}
	stdout = status.Stdout
	stderr = status.Stderr
	return
}

func NewCmdConfig(logger zerolog.Logger, configDir string, namespace string) (*CmdConfig, error) {
	var cmdConfig CmdConfig
	cmdConfig.Namespace = namespace
	cmdConfig.Logger = logger

	baseCfg, err := NewConfig(configDir)
	if err != nil {
		return nil, err
	}

	nsCfgDir := filepath.Join(configDir, namespace)
	nsCfg, err := NewConfig(nsCfgDir)
	if err != nil && err != os.ErrNotExist {
		return nil, err
	}

	// TODO:
	// - merge baseCfg & nsCfg according to magic
	// - hydrate
	return &cmdConfig, nil
}

type CmdConfig struct {
	Spec      CmdSpec
	Namespace string
	Logger    zerolog.Logger
}

type CmdSpec struct {
	Variables []Variable
	Charts    CmdChart
}

type CmdChart struct {
	Helm map[string]CmdHelmChart
	Ytt  map[string]CmdYttChart
}

type CmdHelmChart struct {
	Type   string
	Name   string
	Values []string
}
type CmdYttChart struct {
	Type   string
	Name   string
	Files  []string
	Values []Value
}

// hydrate expands templated variables in our config with concrete values
func (c *CmdConfig) hydrate() error {
	if err := c.hydrateHelmCharts(); err != nil {
		return err
	}
	if err := c.hydrateYttCharts(); err != nil {
		return err
	}
	return nil
}

func (c *CmdConfig) prepareVariables(v []Variable) map[string]string {
	variables := make(map[string]string)
	for _, variable := range v {
		variables[variable.Name] = variable.Value
	}
	variables["namespace"] = c.Namespace
	return variables
}

func (c *CmdConfig) hydrateYttCharts() error {
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

func (c *CmdConfig) hydrateHelmCharts() error {
	for name, chart := range c.Spec.Charts.Helm {
		var newVals []string
		for _, value := range chart.Values {
			rawChartValue, err := yaml.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to get chart values as string: %w", err)
			}
			valueTmpl, err := template.New("chart").Parse(string(rawChartValue))
			if err != nil {
				return fmt.Errorf("failed to parse chart values as template: %q, %w", chart.Values, err)
			}
			buf := new(bytes.Buffer)
			if err := valueTmpl.Execute(buf, c.prepareVariables(c.Spec.Variables)); err != nil {
				return fmt.Errorf("failed to hydrate chart values entry: %q, %w", chart.Values, err)
			}
			newVals = append(newVals, buf.String())
		}
		chart.Values = newVals
		c.Spec.Charts.Helm[name] = chart
	}
	return nil
}
