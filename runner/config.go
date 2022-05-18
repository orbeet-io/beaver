package runner

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/rs/zerolog"
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

func NewCmdConfig(logger zerolog.Logger, configDir string, namespace string, dryRun bool) *CmdConfig {
	cmdConfig := &CmdConfig{}
	cmdConfig.DryRun = dryRun
	cmdConfig.RootDir = configDir
	cmdConfig.Spec.Charts = make(map[string]CmdChart)
	cmdConfig.Namespace = namespace
	cmdConfig.Logger = logger
	return cmdConfig
}

func (c *CmdConfig) Initialize() error {
	baseCfg, err := NewConfig(c.RootDir)
	if err != nil {
		return err
	}

	nsCfgDir := filepath.Join(c.RootDir, "environments", c.Namespace)
	nsCfg, err := NewConfig(nsCfgDir)
	if err != nil && err != os.ErrNotExist {
		return err
	}

	// first "import" all variables from baseCfg
	c.Spec.Variables = baseCfg.Spec.Variables
	// then merge in all variables from the nsCfg
	c.MergeVariables(nsCfg)

	for k, chart := range baseCfg.Spec.Charts {
		c.Spec.Charts[k] = cmdChartFromChart(chart)
	}

	c.populate()

	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	// - hydrate
	if err := c.hydrate(tmpDir); err != nil {
		return err
	}

	return nil
}

type CmdConfig struct {
	Spec      CmdSpec
	RootDir   string
	Namespace string
	Logger    zerolog.Logger
	DryRun    bool
}

type CmdSpec struct {
	Variables []Variable
	Charts    CmdCharts
	Ytt       []string
}

type CmdCharts map[string]CmdChart

type CmdChart struct {
	Type            string
	Path            string
	ValuesFileNames []string
}

const (
	HelmType = "helm"
	YttType  = "ytt"
)

// BuildArgs is in charge of producing the argument list to be provided
// to the cmd
func (c CmdChart) BuildArgs(name, namespace string) ([]string, error) {
	var args []string
	switch c.Type {
	case HelmType:
		// helm template -f base.yaml -f base.values.yaml -f ns.yaml -f ns.values.yaml
		args = append(args, "template", name, c.Path, "--namespace", namespace)
	case YttType:
		// ytt -f $chartsTmpFile --file-mark "$(basename $chartsTmpFile):type=yaml-plain"\
		//   -f base/ytt/ -f base/ytt.yml -f ns1/ytt/ -f ns1/ytt.yml
		args = append(args, "-f", c.Path)
	default:
		return nil, fmt.Errorf("unsupported chart %s type: %q", c.Path, c.Type)
	}
	for _, vFile := range c.ValuesFileNames {
		args = append(args, "-f", vFile)
	}
	return args, nil
}

func cmdChartFromChart(c Chart) CmdChart {
	return CmdChart{
		Type:            c.Type,
		Path:            c.Path,
		ValuesFileNames: nil,
	}
}

// hydrate expands templated variables in our config with concrete values
func (c *CmdConfig) hydrate(dirName string) error {
	c.Logger.Debug().Str("charts", fmt.Sprintf("%+v\n", c.Spec.Charts)).Msg("before hydrate")
	if err := c.hydrateFiles(dirName); err != nil {
		return err
	}
	c.Logger.Debug().Str("charts", fmt.Sprintf("%+v\n", c.Spec.Charts)).Msg("after hydrate")
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
	c.Spec.Ytt = findYttFiles(c.RootDir, c.Namespace)
}

func findYttFiles(rootDir, namespace string) []string {
	var result []string
	for _, dir := range []string{"base", filepath.Join("environments", namespace)} {
		fPath := filepath.Join(rootDir, dir, "ytt")
		baseYttDirInfo, err := os.Stat(fPath)
		if err == nil && baseYttDirInfo.IsDir() {
			result = append(result, fPath)
		}

		for _, ext := range []string{"yaml", "yml"} {
			fPath := filepath.Join(rootDir, dir, fmt.Sprintf("ytt.%s", ext))
			baseYttFileInfo, err := os.Stat(fPath)
			if err == nil && !baseYttFileInfo.IsDir() {
				result = append(result, fPath)
			}
		}
	}
	return result
}

func findFiles(rootdir, namespace string, charts map[string]CmdChart) map[string]CmdChart {
	for name, chart := range charts {
		files := findYaml(rootdir, namespace, name)
		chart.ValuesFileNames = append(chart.ValuesFileNames, files...)
		charts[name] = chart
	}
	return charts
}

func findYaml(rootDir, namespace, name string) []string {
	var files []string
	for _, folder := range []string{"base", filepath.Join("environments", namespace)} {
		for _, ext := range []string{"yaml", "yml"} {
			fpath := filepath.Join(rootDir, folder, fmt.Sprintf("%s.%s", name, ext))
			if _, err := os.Stat(fpath); err == nil {
				files = append(files, fpath)
			}
		}
	}
	return files
}

func hydrateFiles(dirName string, variables map[string]string, paths []string) ([]string, error) {
	var result []string
	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("hydrateFiles could not stat file or dir %s: %w", path, err)
		}
		if fileInfo.IsDir() {
			result = append(result, path)
			continue
		}

		if tmpl, err := template.New(filepath.Base(path)).ParseFiles(path); err != nil {
			return nil, err
		} else {
			if tmpFile, err := ioutil.TempFile(dirName, fmt.Sprintf("%s-", filepath.Base(path))); err != nil {
				return nil, fmt.Errorf("hydrateFiles failed to create tempfile: %w", err)
			} else {
				defer func() {
					_ = tmpFile.Close()
				}()
				if err := tmpl.Execute(tmpFile, variables); err != nil {
					return nil, fmt.Errorf("hydrateFiles failed to execute template: %w", err)
				}
				result = append(result, tmpFile.Name())
			}
		}
	}
	return result, nil
}

func (c *CmdConfig) hydrateFiles(dirName string) error {
	variables := c.prepareVariables(c.Spec.Variables)

	for key, chart := range c.Spec.Charts {
		if paths, err := hydrateFiles(dirName, variables, chart.ValuesFileNames); err != nil {
			return err
		} else {
			chart.ValuesFileNames = paths
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
