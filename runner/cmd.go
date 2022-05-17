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
	cmdConfig.RootDir = configDir
	cmdConfig.Spec.Charts = make(map[string]CmdChart)
	cmdConfig.Namespace = namespace
	cmdConfig.Logger = logger

	baseCfg, err := NewConfig(configDir)
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

	for k, c := range baseCfg.Spec.Charts {
		cmdConfig.Spec.Charts[k] = NewCmdChartFromChart(c)
	}

	cmdConfig.populate()

	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// - hydrate
	if err := cmdConfig.hydrate(tmpDir); err != nil {
		return nil, err
	}

	return cmdConfig, nil
}

type CmdConfig struct {
	Spec      CmdSpec
	RootDir   string
	Namespace string
	Logger    zerolog.Logger
}

type CmdSpec struct {
	Variables []Variable
	Charts    CmdCharts
}

type CmdCharts map[string]CmdChart

type CmdChart struct {
	Type            string
	Path            string
	ValuesFileNames []string
}

func NewCmdChartFromChart(c Chart) CmdChart {
	return CmdChart{
		Path:            c.Path,
		ValuesFileNames: nil,
	}
}

// hydrate expands templated variables in our config with concrete values
func (c *CmdConfig) hydrate(dirName string) error {
	c.Logger.Debug().Str("charts", fmt.Sprintf("%+v\n", c.Spec.Charts)).Msg("charts before hydrate")
	if err := c.hydrateFiles(dirName); err != nil {
		return err
	}
	c.Logger.Debug().Str("charts", fmt.Sprintf("%+v\n", c.Spec.Charts)).Msg("charts after hydrate")
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
	c.Spec.Charts = findFiles(c.RootDir, c.Namespace, c.Spec.Charts)
}

func findFiles(rootdir, namespace string, charts map[string]CmdChart) map[string]CmdChart {
	for name, chart := range charts {
		var files []string
		for _, folder := range []string{"base", filepath.Join("environments", namespace)} {
			for _, ext := range []string{"yaml", "yml"} {
				fpath := filepath.Join(rootdir, folder, fmt.Sprintf("%s.%s", name, ext))
				if _, err := os.Stat(fpath); err == nil {
					files = append(files, fpath)
				}
			}
		}
		chart.ValuesFileNames = append(chart.ValuesFileNames, files...)
		charts[name] = chart
	}
	return charts
}

func (c *CmdChart) hydrateFiles(dirName string, variables map[string]string) ([]string, error) {
	var hydratedFiles []string
	for _, file := range c.ValuesFileNames {
		if tmpl, err := template.New(filepath.Base(file)).ParseFiles(file); err != nil {
			return nil, err
		} else {
			if tmpFile, err := ioutil.TempFile(dirName, fmt.Sprintf("%s-", filepath.Base(file))); err != nil {
				return nil, fmt.Errorf("hydrateFiles failed to create tempfile: %w", err)
			} else {
				defer func() {
					_ = tmpFile.Close()
				}()
				if err := tmpl.Execute(tmpFile, variables); err != nil {
					return nil, fmt.Errorf("hydrateFiles failed to execute template: %w", err)
				}
				hydratedFiles = append(hydratedFiles, tmpFile.Name())
			}
		}
	}
	return hydratedFiles, nil
}

func (c *CmdConfig) hydrateFiles(dirName string) error {
	variables := c.prepareVariables(c.Spec.Variables)

	for key, chart := range c.Spec.Charts {
		if files, err := chart.hydrateFiles(dirName, variables); err != nil {
			return err
		} else {
			chart.ValuesFileNames = files
			c.Spec.Charts[key] = chart
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
