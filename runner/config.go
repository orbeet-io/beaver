package runner

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"github.com/valyala/fasttemplate"
	"gopkg.in/yaml.v3"
)

type Sha struct {
	Key      string
	Resource string
}

type Chart struct {
	Type     string
	Path     string
	Name     string
	Disabled bool
}

type Arg struct {
	Flag  string
	Value string
}

type Create struct {
	Type string
	Name string
	Args []Arg `yaml:",flow"`
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

// Config ...
type Config struct {
	Inherit   string
	NameSpace string
	Inherits  []string         `yaml:",flow"`
	Variables Variables        `yaml:",flow"`
	Sha       []Sha            `yaml:",flow"`
	Charts    map[string]Chart `yaml:",flow"`
	Creates   []Create         `yaml:"create,flow"`
	Dir       string           // the directory in which we found the config file
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

func NewCmdConfig(logger zerolog.Logger, rootDir, configDir string, dryRun bool, output string) *CmdConfig {
	cmdConfig := &CmdConfig{}
	cmdConfig.DryRun = dryRun
	cmdConfig.Output = output
	cmdConfig.RootDir = rootDir
	cmdConfig.Layers = append(cmdConfig.Layers, configDir)
	cmdConfig.Spec.Charts = make(map[string]CmdChart)
	cmdConfig.Spec.Creates = make(map[CmdCreateKey]CmdCreate)
	cmdConfig.Spec.Shas = []*CmdSha{}
	cmdConfig.Namespace = ""
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
		c.Namespace = config.NameSpace
		c.MergeVariables(config)

		for k, chart := range config.Charts {
			c.Spec.Charts[k] = cmdChartFromChart(chart)
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

type CmdCreateKey struct {
	Type string
	Name string
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

type CmdSpec struct {
	Variables Variables
	Shas      []*CmdSha
	Charts    CmdCharts
	Ytt       Ytt
	Creates   map[CmdCreateKey]CmdCreate
}

type Ytt []string

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
	Type            string
	Path            string
	Name            string
	Disabled        bool
	ValuesFileNames []string
}

const (
	HelmType = "helm"
	YttType  = "ytt"
)

// BuildArgs is in charge of producing the argument list to be provided
// to the cmd
func (c CmdChart) BuildArgs(n, namespace string) ([]string, error) {
	var name string
	var args []string
	if c.Name != "" {
		name = c.Name
	} else {
		name = n
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

func cmdChartFromChart(c Chart) CmdChart {
	return CmdChart{
		Type:            c.Type,
		Path:            c.Path,
		Name:            c.Name,
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

func (c *CmdConfig) populate() {
	c.Spec.Charts = FindFiles(c.Layers, c.Spec.Charts)
	c.Spec.Ytt = findYtts(c.Layers)
}

func findYtts(layers []string) []string {
	var result []string

	// we cannot use findYaml here because the order matters
	for _, layer := range layers {
		yttDirPath := filepath.Join(layer, "ytt")
		yttDirInfo, err := os.Stat(yttDirPath)
		if err == nil && yttDirInfo.IsDir() {
			result = append(result, yttDirPath)
		}

		for _, ext := range []string{"yaml", "yml"} {
			yttFilePath := filepath.Join(layer, fmt.Sprintf("ytt.%s", ext))
			yttFileInfo, err := os.Stat(yttFilePath)
			if err == nil && !yttFileInfo.IsDir() {
				result = append(result, yttFilePath)
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

func hydrateString(input string, output io.Writer, variables map[string]interface{}) error {
	t, err := fasttemplate.NewTemplate(input, "<[", "]>")
	if err != nil {
		return fmt.Errorf("unexpected error when parsing template: %w", err)
	}
	s, err := t.ExecuteFuncStringWithErr(func(w io.Writer, tag string) (int, error) {
		val, ok := lookupVariable(variables, tag)
		if !ok {
			return 0, fmt.Errorf("tag not found: %s", tag)
		}
		switch v := val.(type) {
		case string:
			return w.Write([]byte(v))
		default:
			e := yaml.NewEncoder(w)
			err := e.Encode(val)
			return 0, err
		}
	})
	if err != nil {
		return err
	}
	if _, err := output.Write([]byte(s)); err != nil {
		return fmt.Errorf("failed to template: %w", err)
	}
	return nil
}

// ^<\[ starts with literal <[
// ([^<\[\]>]*) a capturing group that matches any 0 or more (due to the * quantifier)
// characters other than <, [ and ],>
// ([^...] is a negated character class matching any char but the one(s) specified between [^ and ])
// \]>$ ends with literal ]>
var re = regexp.MustCompile(`^<\[([^<\[\]>]*)\]>$`)

func hydrateScalarNode(node *yaml.Node, variables map[string]interface{}) error {
	input := node.Value

	// find all matches
	matches := re.FindAllStringSubmatch(input, -1)

	if len(matches) == 1 {
		// first match, then first extracted data (in position 1)
		tag := matches[0][1]
		var ok bool
		output, ok := lookupVariable(variables, tag)
		if !ok {
			return fmt.Errorf("tag not found: %s", tag)
		}
		// preserve comments
		hc := node.HeadComment
		lc := node.LineComment
		fc := node.FootComment
		if err := node.Encode(output); err != nil {
			return err
		}
		node.HeadComment = hc
		node.LineComment = lc
		node.FootComment = fc
	} else {
		buf := bytes.NewBufferString("")
		if err := hydrateString(input, buf, variables); err != nil {
			return err
		}
		node.Value = buf.String()
	}
	return nil
}

func hydrateYamlNodes(nodes []*yaml.Node, variables map[string]interface{}) error {
	for _, node := range nodes {
		if node.Kind == yaml.ScalarNode {
			if err := hydrateScalarNode(node, variables); err != nil {
				fmt.Printf("node: %+v, variables: %+v\n", node, variables)
				return fmt.Errorf("failed to parse scalar: %w", err)
			}
		} else {
			if err := hydrateYamlNodes(node.Content, variables); err != nil {
				return fmt.Errorf("failed to hydrate content: %w", err)
			}
		}
	}
	return nil
}

func hydrateYaml(root *yaml.Node, variables map[string]interface{}) error {
	err := hydrateYamlNodes(root.Content, variables)
	return err
}

func documentSplitter(input io.Reader) [][]byte {
	var output [][]byte
	var tmpOut []byte
	scanner := bufio.NewScanner(input)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		text := scanner.Bytes()
		// if sep is found first flush our buffer
		if string(text) == "---" {
			// flush buffer
			output = append(output, tmpOut)
			// initialize new buffer
			tmpOut = []byte{}
		}
		tmpOut = append(tmpOut, text...)
		tmpOut = append(tmpOut, []byte("\n")...)
	}
	output = append(output, tmpOut)
	return output
}

func Hydrate(input []byte, output io.Writer, variables map[string]interface{}) error {
	// documents := bytes.Split(input, []byte("---\n"))
	documents := documentSplitter(bytes.NewReader(input))
	// yaml lib ignore leading '---'
	// see: https://github.com/go-yaml/yaml/issues/749
	// which is an issue for ytt value files
	// this is why we loop over documents in the same file
	for i, doc := range documents {
		var node yaml.Node
		if err := yaml.Unmarshal(doc, &node); err != nil || len(node.Content) == 0 {
			// not a yaml template, fallback to raw template method
			// ...maybe a ytt header or a frontmatter
			template := string(doc)
			if err := hydrateString(template, output, variables); err != nil {
				return err
			}
		} else {
			// FIXME: do not call this method when hydrating only for sha,
			// could be quite expensive

			// yaml template method
			err := hydrateYaml(&node, variables)
			if err != nil {
				return fmt.Errorf("failed to hydrate yaml: %w", err)
			}
			o, err := yaml.Marshal(node.Content[0])
			if err != nil {
				return fmt.Errorf("failed to marshal yaml: %w", err)
			}
			_, err = output.Write(o)
			if err != nil {
				return err
			}
		}
		if i != len(documents)-1 {
			_, err := output.Write([]byte("---\n"))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func hydrate(input string, output io.Writer, variables map[string]interface{}) error {
	byteTemplate, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", input, err)
	}
	return Hydrate(byteTemplate, output, variables)
}

func hydrateFiles(tmpDir string, variables map[string]interface{}, paths []string) ([]string, error) {
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

		ext := filepath.Ext(path)
		tmpFile, err := os.CreateTemp(tmpDir, fmt.Sprintf("%s-*%s", strings.TrimSuffix(filepath.Base(path), ext), ext))
		if err != nil {
			return nil, fmt.Errorf("hydrateFiles failed to create tempfile: %w", err)
		}
		defer func() {
			_ = tmpFile.Close()
		}()
		if err := hydrate(path, tmpFile, variables); err != nil {
			return nil, fmt.Errorf("failed to hydrate: %w", err)
		}
		result = append(result, tmpFile.Name())
	}
	return result, nil
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

// MergeVariables takes a config (from a file, not a cmd one) and import its
// variables into the current cmdconfig by replacing old ones
// and adding the new ones
func (c *CmdConfig) MergeVariables(other *Config) {
	c.Spec.Variables.Overlay(other.Variables...)
}
