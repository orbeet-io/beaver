package runner

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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

type Arg struct {
	Flag  string `mapstructure:"flag"`
	Value string `mapstructure:"value"`
}

type Create struct {
	Type string `mapstructure:"type"`
	Name string `mapstructure:"name"`
	Args []Arg  `mapstructure:"args"`
}

func (k CmdCreateKey) BuildArgs(namespace string, args []Arg) []string {
	output := []string{
		"-n", namespace,
		"create",
		k.Type,
		k.Name,
		"--dry-run=client",
		"-o", "yaml"}

	for _, arg := range args {
		output = append(output, arg.Flag, arg.Value)
	}
	return output
}

// Spec ...
type Spec struct {
	Inherit   string           `mapstructure:"inherit"`
	NameSpace string           `mapstructure:"namespace"`
	Variables []Variable       `mapstructure:"variables"`
	Charts    map[string]Chart `mapstructure:"charts"`
	Creates   []Create         `mapstructure:"create"`
}

// Config is the configuration we get after parsing our beaver.yml file
type Config struct {
	APIVersion string `mapstructure:"apiVersion"`
	Kind       string `mapstructure:"kind"`
	Spec       Spec   `mapstructure:"spec"`
	Dir        string // the directory in which we found the config file
}

// Absolutize makes all chart paths absolute
func (c *Config) Absolutize(dir string) error {
	for name, chart := range c.Spec.Charts {
		resolvedChartPath := filepath.Join(dir, chart.Path)
		absChartPath, err := filepath.Abs(resolvedChartPath)
		if err != nil {
			return fmt.Errorf("failed to find abs() for %s: %w", resolvedChartPath, err)
		}

		chart.Path = absChartPath
		c.Spec.Charts[name] = chart
	}
	return nil
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

func NewCmdConfig(logger zerolog.Logger, rootDir, configDir string, dryRun bool) *CmdConfig {
	cmdConfig := &CmdConfig{}
	cmdConfig.DryRun = dryRun
	cmdConfig.RootDir = rootDir
	cmdConfig.Layers = append(cmdConfig.Layers, configDir)
	cmdConfig.Spec.Charts = make(map[string]CmdChart)
	cmdConfig.Spec.Creates = make(map[CmdCreateKey]CmdCreate)
	cmdConfig.Namespace = ""
	cmdConfig.Logger = logger
	return cmdConfig
}

func (c *CmdConfig) Initialize(tmpDir string) error {
	if len(c.Layers) != 1 {
		return fmt.Errorf("you must only have one layer when calling Initialize, found: %d", len(c.Layers))
	}
	var (
		weNeedToGoDeeper = true
		configLayers     []*Config
	)

	resolvedConfigDir := filepath.Join(c.RootDir, c.Layers[0])
	absConfigDir, err := filepath.Abs(resolvedConfigDir)
	if err != nil {
		return fmt.Errorf("failed to find abs() for %s: %w", resolvedConfigDir, err)
	}
	dir := absConfigDir

	for weNeedToGoDeeper {
		config, err := c.newConfigFromDir(dir)
		if err != nil {
			return fmt.Errorf("failed to create config from %s: %w", dir, err)
		}
		config.Dir = dir
		if err := config.Absolutize(dir); err != nil {
			return fmt.Errorf("failed to absolutize config from dir: %s, %w", dir, err)
		}
		// first config dir must return a real config...
		// others can be skipped
		if config == nil && len(configLayers) == 0 {
			return fmt.Errorf("failed to find config in dir: %s", dir)
		}

		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("failed to find abs() for %s: %w", dir, err)
		}

		c.Layers = append(c.Layers, absDir)
		configLayers = append(configLayers, config)

		if config == nil || config.Spec.Inherit == "" {
			weNeedToGoDeeper = false
		} else {
			resolvedDir := filepath.Join(absDir, config.Spec.Inherit)
			newDir, err := filepath.Abs(resolvedDir)
			if err != nil {
				return fmt.Errorf("failed to find abs() for %s: %w", resolvedDir, err)
			}
			dir = newDir
		}
	}

	// reverse our layers list
	for i, j := 0, len(configLayers)-1; i < j; i, j = i+1, j-1 {
		configLayers[i], configLayers[j] = configLayers[j], configLayers[i]
	}

	for _, config := range configLayers {
		c.Namespace = config.Spec.NameSpace
		c.MergeVariables(config)

		for k, chart := range config.Spec.Charts {
			c.Spec.Charts[k] = cmdChartFromChart(chart)
		}

		for _, k := range config.Spec.Creates {
			cmdCreate := CmdCreateKey{Type: k.Type, Name: k.Name}
			c.Spec.Creates[cmdCreate] = CmdCreate{
				Dir:  config.Dir,
				Args: k.Args,
			}
		}
	}

	for i, j := 0, len(c.Layers)-1; i < j; i, j = i+1, j-1 {
		c.Layers[i], c.Layers[j] = c.Layers[j], c.Layers[i]
	}
	c.populate()
	if err := c.hydrate(tmpDir); err != nil {
		return fmt.Errorf("failed to hydrate tmpDir (%s): %w", tmpDir, err)
	}
	return nil
}

func (c *CmdConfig) newConfigFromDir(dir string) (*Config, error) {
	cfg, err := NewConfig(dir)
	var cfgNotFound bool
	if err != nil {
		_, cfgNotFound = err.(viper.ConfigFileNotFoundError)
		if !cfgNotFound {
			return nil, err
		}
	}
	return cfg, nil
}

type CmdCreateKey struct {
	Type string `mapstructure:"type"`
	Name string `mapstructure:"name"`
}

type CmdCreate struct {
	Dir  string
	Args []Arg
}

type CmdConfig struct {
	Spec      CmdSpec
	RootDir   string
	Layers    []string
	Namespace string
	Logger    zerolog.Logger
	DryRun    bool
}

type CmdSpec struct {
	Variables []Variable
	Charts    CmdCharts
	Ytt       Ytt
	Creates   map[CmdCreateKey]CmdCreate
}

type Ytt []string

func (c CmdConfig) PrepareYttArgs(tmpDir string, layers, compiled []string) ([]string, error) {
	var paths []string
	variables := c.prepareVariables(c.Spec.Variables)

	for _, layer := range layers {
		for _, ext := range []string{"", ".yaml", ".yml"} {
			entry := fmt.Sprintf("ytt%s", ext)
			entryPath := filepath.Join(layer, entry)

			if stat, err := os.Stat(entryPath); !os.IsNotExist(err) {
				if !stat.IsDir() {
					hydratedPaths, err := hydrateFiles(tmpDir, variables, []string{entryPath})
					if err != nil {
						return nil, fmt.Errorf("failed to hydrate %s: %w", entryPath, err)
					}
					entryPath = hydratedPaths[0]
				}
				paths = append(paths, entryPath)
			}
		}
	}

	args := c.BuildYttArgs(paths, compiled)

	return args, nil
}

func (c CmdConfig) BuildYttArgs(paths, compiled []string) []string {
	// ytt -f $chartsTmpFile --file-mark "$(basename $chartsTmpFile):type=yaml-plain"\
	//   -f base/ytt/ -f base/ytt.yml -f ns1/ytt/ -f ns1/ytt.yml
	var args []string
	for _, c := range compiled {
		args = append(args, "-f", c, fmt.Sprintf("--file-mark=%s:type=yaml-plain", filepath.Base(c)))
	}
	for _, path := range paths {
		args = append(args, "-f", path)
	}
	return args
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
		// helm template name vendor/helm/mychart/ --namespace ns1 -f base.values.yaml -f ns.yaml -f ns.values.yaml
		args = append(args, "template", name, c.Path, "--namespace", namespace)
	case YttType:
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
func (c *CmdConfig) hydrate(tmpDir string) error {
	c.Logger.Debug().Str("charts", fmt.Sprintf("%+v\n", c.Spec.Charts)).Msg("before hydrate")
	if err := c.hydrateFiles(tmpDir); err != nil {
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
	c.Spec.Charts = FindFiles(c.Layers, c.Spec.Charts)
	c.Spec.Ytt = findYttFiles(c.Layers)
}

func findYttFiles(layers []string) []string {
	var result []string

	for _, layer := range layers {
		fPath := filepath.Join(layer, "ytt")
		baseYttDirInfo, err := os.Stat(fPath)
		if err == nil && baseYttDirInfo.IsDir() {
			result = append(result, fPath)
		}

		for _, ext := range []string{"yaml", "yml"} {
			fPath := filepath.Join(layer, fmt.Sprintf("ytt.%s", ext))
			baseYttFileInfo, err := os.Stat(fPath)
			if err == nil && !baseYttFileInfo.IsDir() {
				result = append(result, fPath)
			}
		}
	}
	return result
}

func FindFiles(layers []string, charts map[string]CmdChart) map[string]CmdChart {
	for name, chart := range charts {
		files := findYaml(layers, name)
		chart.ValuesFileNames = append(chart.ValuesFileNames, files...)
		charts[name] = chart
	}
	return charts
}

func findYaml(layers []string, name string) []string {
	var files []string
	for _, layer := range layers {
		for _, ext := range []string{"yaml", "yml"} {
			fpath := filepath.Join(layer, fmt.Sprintf("%s.%s", name, ext))
			if _, err := os.Stat(fpath); err == nil {
				files = append(files, fpath)
			}
		}
	}
	return files
}

func hydrateFiles(tmpDir string, variables map[string]string, paths []string) ([]string, error) {
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
			ext := filepath.Ext(path)
			if tmpFile, err := ioutil.TempFile(tmpDir, fmt.Sprintf("%s-*%s", strings.TrimSuffix(filepath.Base(path), ext), ext)); err != nil {
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
