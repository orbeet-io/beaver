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
	cmdConfig := &CmdConfig{}
	cmdConfig.Spec.Charts.Helm = make(map[string]CmdHelmChart)
	cmdConfig.Spec.Charts.Ytt = make(map[string]CmdYttChart)
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

	// first "import" all variables from baseCfg
	cmdConfig.Spec.Variables = baseCfg.Spec.Variables
	// then merge in all variables from the nsCfg
	cmdConfig.MergeVariables(nsCfg)

	// TODO:
	// - merge baseCfg & nsCfg charts
	if err := cmdConfig.importCharts(baseCfg); err != nil {
		return nil, err
	}
	if err := cmdConfig.importCharts(nsCfg); err != nil {
		return nil, err
	}

	// - hydrate
	if err := cmdConfig.hydrate(); err != nil {
		return nil, err
	}

	return cmdConfig, nil
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
			valueTmpl, err := template.New("chart").Parse(value)
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

// MergeVariables takes a config (from a file, not a cmd one) and import its
// variables into the current cmdconfig by replacing old ones
// and adding the new ones
func (c *CmdConfig) MergeVariables(other *Config) {
	for _, variable := range other.Spec.Variables {
		c.overlayVariable(variable)
	}
}

// overlayVariable takes a variable in and either replaces an existing variable
// of the same name or create a new variable in the config if no matching name
// is found
func (c *CmdConfig) overlayVariable(v Variable) {
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

func (c *CmdConfig) importCharts(other *Config) error {
	if err := c.importHelmCharts(other.Spec.Charts.Helm); err != nil {
		return nil
	}
	if err := c.importYttCharts(other.Spec.Charts.Ytt); err != nil {
		return nil
	}
	return nil
}

func (c *CmdConfig) importHelmCharts(helmCharts map[string]HelmChart) error {
	for id, chart := range helmCharts {
		convertedChart, err := cmdHelmChartFromHelmChart(chart)
		if err != nil {
			return err
		}
		_, ok := c.Spec.Charts.Helm[id]
		if !ok {
			// we have no chart by that name yet...
			// create one
			c.Spec.Charts.Helm[id] = *convertedChart
			continue
		}
		// else just append values to existing one
		convertedChart.Values = append(c.Spec.Charts.Helm[id].Values, convertedChart.Values...)
		c.Spec.Charts.Helm[id] = *convertedChart
	}
	return nil
}

func cmdHelmChartFromHelmChart(c HelmChart) (*CmdHelmChart, error) {
	strValues, err := yaml.Marshal(c.Values)
	if err != nil {
		return nil, err
	}
	return &CmdHelmChart{
		Type:   c.Type,
		Name:   c.Name,
		Values: []string{string(strValues)},
	}, nil
}

func (c *CmdConfig) importYttCharts(yttCharts map[string]YttChart) error {
	return nil
}
