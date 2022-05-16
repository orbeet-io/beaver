package runner

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/go-cmd/cmd"
	"github.com/rs/zerolog"
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
	cmdConfig.Spec.Charts.Helm = make(map[string]CmdChart)
	cmdConfig.Spec.Charts.Ytt = make(map[string]CmdChart)
	cmdConfig.Namespace = namespace
	cmdConfig.Logger = logger

	baseCfg, err := NewConfig(configDir)
	fmt.Printf(">>> %+v\n", baseCfg)
	if err != nil {
		return nil, err
	}

	nsCfgDir := filepath.Join(configDir, "environments", namespace)
	nsCfg, err := NewConfig(nsCfgDir)
	if err != nil && err != os.ErrNotExist {
		return nil, err
	}

	// first "import" all variables from baseCfg
	cmdConfig.Spec.Variables = baseCfg.Spec.Variables
	// then merge in all variables from the nsCfg
	cmdConfig.MergeVariables(nsCfg)

	// TODO: merge baseCfg & nsCfg charts into cmdConfig

	cmdConfig.populate()

	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	if err != nil {
		return nil, err
	}

	// - hydrate
	if err := cmdConfig.hydrate(tmpDir); err != nil {
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
	Charts    CmdCharts
}

type CmdCharts struct {
	Helm map[string]CmdChart
	Ytt  map[string]CmdChart
}

type CmdChart struct {
	Name  string
	Files []string
}

// hydrate expands templated variables in our config with concrete values
func (c *CmdConfig) hydrate(dirName string) error {
	if err := c.hydrateFiles(dirName); err != nil {
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

func (c *CmdConfig) populate() {
	c.Spec.Charts.Helm = findFiles(c.Namespace, c.Spec.Charts.Helm)
	c.Spec.Charts.Ytt = findFiles(c.Namespace, c.Spec.Charts.Ytt)
}

func findFiles(namespace string, charts map[string]CmdChart) map[string]CmdChart {
	var fpath string
	var files []string
	for name, chart := range charts {
		for _, folder := range []string{"base", filepath.Join("environments", namespace)} {
			for _, ext := range []string{"yaml", "yml"} {
				fpath = filepath.Join(folder, fmt.Sprintf("%s.%s", name, ext))
				if _, err := os.Stat(fpath); err == nil {
					files = append(files, fpath)
					chart.Files = append(chart.Files, fpath)
					charts[name] = chart
				}

			}
		}
	}
	return charts
}

func (c *CmdChart) hydrateFiles(dirName string, variables map[string]string) ([]string, error) {
	var hydratedFiles []string
	for _, file := range c.Files {
		if tmpl, err := template.New(file).ParseFiles(file); err != nil {
			return nil, err
		} else {
			if tmpFile, err := ioutil.TempFile(dirName, fmt.Sprintf("%s-", file)); err != nil {
				return nil, err
			} else {
				if err := tmpl.Execute(tmpFile, variables); err != nil {
					return nil, err
				}
				hydratedFiles = append(hydratedFiles, tmpFile.Name())
			}
		}
	}
	return hydratedFiles, nil
}

func (c *CmdConfig) hydrateFiles(dirName string) error {
	variables := c.prepareVariables(c.Spec.Variables)

	for key, helmChart := range c.Spec.Charts.Helm {
		if files, err := helmChart.hydrateFiles(dirName, variables); err != nil {
			return err
		} else {
			helmChart.Files = files
			c.Spec.Charts.Helm[key] = helmChart
		}
	}
	// FIXME: use generic to avoid repetition
	for key, yttChart := range c.Spec.Charts.Ytt {
		if files, err := yttChart.hydrateFiles(dirName, variables); err != nil {
			return err
		} else {
			yttChart.Files = files
			c.Spec.Charts.Ytt[key] = yttChart
		}
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
