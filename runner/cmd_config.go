package runner

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"

	beaver "orus.io/orus-io/beaver/lib"
)

const (
	HelmType = "helm"
	YttType  = "ytt"
)

// Ytt is a type alias describing ytt arguments
type Ytt []string

type CmdSpec struct {
	Variables Variables
	Shas      []*CmdSha
	Charts    CmdCharts
	Ytt       Ytt
	Creates   map[CmdCreateKey]CmdCreate
}

type CmdCreateKey struct {
	Type string
	Name string
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

type CmdCreate struct {
	Dir  string
	Args []Arg
}

type CmdSha struct {
	Key      string
	Resource string
	Sha      string
}

type CmdConfig struct {
	Spec      CmdSpec
	RootDir   string
	Layers    []string
	Namespace string
	Logger    zerolog.Logger
	DryRun    bool
	Output    string
}

func NewCmdConfig(logger zerolog.Logger, rootDir, configDir string, dryRun bool, output string, namespace string) *CmdConfig {
	cmdConfig := &CmdConfig{}
	cmdConfig.DryRun = dryRun
	cmdConfig.Output = output
	cmdConfig.RootDir = rootDir
	cmdConfig.Layers = append(cmdConfig.Layers, configDir)
	cmdConfig.Spec.Charts = make(map[string]CmdChart)
	cmdConfig.Spec.Creates = make(map[CmdCreateKey]CmdCreate)
	cmdConfig.Spec.Shas = []*CmdSha{}
	cmdConfig.Namespace = namespace
	cmdConfig.Logger = logger
	return cmdConfig
}

func (c *CmdConfig) Initialize(tmpDir string) error {
	if len(c.Layers) != 1 {
		return fmt.Errorf("you must only have one layer when calling Initialize, found: %d", len(c.Layers))
	}
	var (
		configLayers []*Config
	)

	resolvedConfigDir := filepath.Join(c.RootDir, c.Layers[0])
	absConfigDir, err := filepath.Abs(resolvedConfigDir)
	if err != nil {
		return fmt.Errorf("failed to find abs() for %s: %w", resolvedConfigDir, err)
	}
	dirs := []string{absConfigDir}
	dirMap := make(map[string]interface{})

	// otherwise, first layer will be present twice
	c.Layers = []string{}

	for len(dirs) > 0 {
		var newDirs []string
		for _, dir := range dirs {
			dirs, cl, err := c.addConfDir(dir, dirMap, configLayers)
			if err != nil {
				return err
			}
			configLayers = cl
			newDirs = append(newDirs, dirs...)
		}
		dirs = newDirs
	}

	// reverse our layers list
	for i, j := 0, len(configLayers)-1; i < j; i, j = i+1, j-1 {
		configLayers[i], configLayers[j] = configLayers[j], configLayers[i]
	}

	for _, config := range configLayers {
		if c.Namespace == "" {
			c.Namespace = config.NameSpace
		}
		c.MergeVariables(config)

		for k, chart := range config.Charts {
			c.Spec.Charts[k] = CmdChartFromChart(chart)
		}

		for _, k := range config.Creates {
			cmdCreate := CmdCreateKey{Type: k.Type, Name: k.Name}
			c.Spec.Creates[cmdCreate] = CmdCreate{
				Dir:  config.Dir,
				Args: k.Args,
			}
		}
		for _, sha := range config.Sha {
			cmdSha := CmdSha{Key: sha.Key, Resource: sha.Resource}
			c.Spec.Shas = append(c.Spec.Shas, &cmdSha)
		}
	}

	for i, j := 0, len(c.Layers)-1; i < j; i, j = i+1, j-1 {
		c.Layers[i], c.Layers[j] = c.Layers[j], c.Layers[i]
	}
	c.populate()
	if err := c.hydrate(tmpDir, false); err != nil {
		return fmt.Errorf("failed to hydrate tmpDir (%s): %w", tmpDir, err)
	}
	return nil
}

func (c *CmdConfig) addConfDir(dir string, dirMap map[string]interface{}, configLayers []*Config) ([]string, []*Config, error) {
	// guard against recursive inherit loops
	_, present := dirMap[dir]
	if present {
		var dirList []string
		for k := range dirMap {
			dirList = append(dirList, k)
		}
		return nil, configLayers, fmt.Errorf("recursive inherit loop detected: dirs %s->%s", strings.Join(dirList, "->"), dir)
	}

	config, err := c.newConfigFromDir(dir)
	if err != nil {
		return nil, configLayers, fmt.Errorf("failed to create config from %s: %w", dir, err)
	}
	if config == nil {
		if len(c.Layers) == 1 {
			return nil, configLayers, fmt.Errorf("beaver file not found in directory: %s", dir)
		}
		return nil, configLayers, nil
	}
	config.Dir = dir
	if err := config.Absolutize(dir); err != nil {
		return nil, configLayers, fmt.Errorf("failed to absolutize config from dir: %s, %w", dir, err)
	}
	// first config dir must return a real config...
	// others can be skipped
	if config == nil && len(configLayers) == 0 {
		return nil, configLayers, fmt.Errorf("failed to find config in dir: %s", dir)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, configLayers, fmt.Errorf("failed to find abs() for %s: %w", dir, err)
	}

	c.Layers = append(c.Layers, absDir)
	if config.BeaverVersion != "" && beaver.Version() != "" {
		if err := beaver.ControlVersions(config.BeaverVersion, beaver.Version()); err != nil {
			return nil, nil, err
		}
	}
	configLayers = append(configLayers, config)

	if config == nil || (len(config.Inherits) == 0 && config.Inherit == "") {
		// weNeedToGoDeeper = false
		return nil, configLayers, nil
	}
	var newDirs []string
	for _, inherit := range config.Inherits {
		resolvedDir := filepath.Join(absDir, inherit)
		newDir, err := filepath.Abs(resolvedDir)
		if err != nil {
			return nil, configLayers, fmt.Errorf("failed to find abs() for %s: %w", resolvedDir, err)
		}
		newDirs = append(newDirs, newDir)
	}
	if config.Inherit != "" {
		resolvedDir := filepath.Join(absDir, config.Inherit)
		newDir, err := filepath.Abs(resolvedDir)
		if err != nil {
			return nil, configLayers, fmt.Errorf("failed to find abs() for %s: %w", resolvedDir, err)
		}
		newDirs = append(newDirs, newDir)
	}
	return newDirs, configLayers, nil
}

func (c *CmdConfig) newConfigFromDir(dir string) (*Config, error) {
	cfg, err := NewConfig(dir)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *CmdConfig) HasShas() bool {
	return len(c.Spec.Shas) > 0
}

func (c *CmdConfig) SetShas(buildDir string) error {
	for _, sha := range c.Spec.Shas {
		if err := sha.SetSha(buildDir); err != nil {
			return err
		}
	}
	return nil
}

func (s *CmdSha) SetSha(buildDir string) error {
	fPath := filepath.Join(buildDir, s.Resource)
	f, err := os.Open(fPath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", fPath, err)
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return fmt.Errorf("failed to read %s: %w", fPath, err)
	}
	s.Sha = fmt.Sprintf("%x", hash.Sum(nil))
	return nil
}

func (c *CmdConfig) BuildYttArgs(paths, compiled []string) []string {
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
	Type      string
	Path      string
	Name      string
	Namespace string
	// Must be castable into bool (0,1,true,false)
	Disabled        string
	ValuesFileNames []string
}

// BuildArgs is in charge of producing the argument list to be provided
// to the cmd
func (c CmdChart) BuildArgs(n, ns string) ([]string, error) {
	var name string
	var namespace string
	var args []string
	if c.Name != "" {
		name = c.Name
	} else {
		name = n
	}
	if c.Namespace != "" {
		namespace = c.Namespace
	} else {
		namespace = ns
	}
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

func CmdChartFromChart(c Chart) CmdChart {
	return CmdChart{
		Type:            c.Type,
		Path:            c.Path,
		Name:            c.Name,
		Namespace:       c.Namespace,
		Disabled:        c.Disabled,
		ValuesFileNames: nil,
	}
}

func (c *CmdConfig) prepareVariables(doSha bool) (map[string]interface{}, error) {
	variables := make(map[string]interface{})
	for _, variable := range c.Spec.Variables {
		variables[variable.Name] = variable.Value
	}
	variables["namespace"] = c.Namespace
	shavars := map[string]interface{}{}
	for _, sha := range c.Spec.Shas {
		if doSha {
			if sha.Sha != "" {
				shavars[sha.Key] = sha.Sha
			} else {
				return nil, fmt.Errorf("SHA not found for %s", sha.Key)
			}
		} else {
			shavars[sha.Key] = fmt.Sprintf("<[sha.%s]>", sha.Key)
		}
	}
	variables["sha"] = shavars
	return variables, nil
}

// MergeVariables takes a config (from a file, not a cmd one) and import its
// variables into the current cmdconfig by replacing old ones
// and adding the new ones
func (c *CmdConfig) MergeVariables(other *Config) {
	c.Spec.Variables.Overlay(other.Variables...)
}

// hydrate expands templated variables in our config with concrete values
func (c *CmdConfig) hydrate(dirName string, doSha bool) error {
	variables, err := c.prepareVariables(doSha)
	if err != nil {
		return fmt.Errorf("cannot prepare variables %w", err)
	}

	for key, chart := range c.Spec.Charts {
		paths, err := hydrateFiles(dirName, variables, chart.ValuesFileNames)
		if err != nil {
			return err
		}
		chart.ValuesFileNames = paths
		c.Spec.Charts[key] = chart
	}
	paths, err := hydrateFiles(dirName, variables, c.Spec.Ytt)
	if err != nil {
		return err
	}
	c.Spec.Ytt = paths
	return nil
}
