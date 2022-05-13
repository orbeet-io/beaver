package runner

import (
	"os"
	"path/filepath"

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
