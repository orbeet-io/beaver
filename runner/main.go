package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-cmd/cmd"
	"github.com/go-yaml/yaml"
)

var (
	defaultFileMod os.FileMode = 0600
	defaultDirMod  os.FileMode = 0700
)

type Runner struct {
	config *CmdConfig
}

// NewRunner ...
func NewRunner(cfg *CmdConfig) *Runner {
	return &Runner{
		config: cfg,
	}
}

// Build is in charge of applying commands based on the config data
func (r *Runner) Build(tmpDir string) error {
	// TODO: find command full path
	var yttCmd = "ytt"
	var helmCmd = "helm"
	var kubectlCmd = "kubectl"
	// create helm commands
	// create ytt chart commands
	cmds := make(map[string]*cmd.Cmd)
	for name, chart := range r.config.Spec.Charts {
		args, err := chart.BuildArgs(name, r.config.Namespace)
		if err != nil {
			return fmt.Errorf("build: failed to build args %w", err)
		}
		switch chart.Type {
		case HelmType:
			cmds[name] = cmd.NewCmd(helmCmd, args...)
		case YttType:
			cmds[name] = cmd.NewCmd(yttCmd, args...)
		default:
			return fmt.Errorf("unsupported chart %s type: %q", chart.Path, chart.Type)
		}
	}

	for key, create := range r.config.Spec.Creates {
		strArgs := key.BuildArgs(r.config.Namespace, create.Args)
		name := fmt.Sprintf("%s_%s", key.Type, key.Name)
		c := cmd.NewCmd(kubectlCmd, strArgs...)
		c.Dir = create.Dir
		cmds[name] = c
	}

	// run commands or print them
	var compiled []string
	if r.config.DryRun {
		for _, cmd := range cmds {
			r.config.Logger.Info().
				Str("command", cmd.Name).
				Str("args", strings.Join(cmd.Args, " ")).
				Msg("would run command")
		}
	} else {
		for name, cmd := range cmds {
			stdOut, stdErr, err := RunCMD(cmd)
			if err != nil {
				r.config.Logger.Err(err).
					Str("command", cmd.Name).
					Str("args", strings.Join(cmd.Args, " ")).
					Str("sdtout", strings.Join(stdOut, "\n")).
					Str("stderr", strings.Join(stdErr, "\n")).
					Msg("failed to run command")

				// TODO: print error to stderr
				// Error must be pretty printed to end users /!\
				fmt.Printf("\n%s\n\n", strings.Join(stdErr, "\n"))
				return fmt.Errorf("failed to run command: %w", err)
			}
			tmpFile, err := ioutil.TempFile(tmpDir, fmt.Sprintf("compiled-%s-*.yaml", name))
			if err != nil {
				return fmt.Errorf("cannot create compiled file: %w", err)
			}
			defer func() {
				if err := tmpFile.Close(); err != nil {
					r.config.Logger.
						Err(err).
						Str("temp file", tmpFile.Name()).
						Msg("failed to close temp file")
				}
			}()
			if _, err := tmpFile.WriteString(strings.Join(stdOut, "\n")); err != nil {
				return fmt.Errorf("cannot write compiled file: %w", err)
			}
			compiled = append(compiled, tmpFile.Name())
		}
	}

	// create ytt additional command
	args, err := r.config.PrepareYttArgs(tmpDir, r.config.Layers, compiled)
	if err != nil {
		return fmt.Errorf("cannot prepare ytt args: %w", err)
	}

	yttExtraCmd := cmd.NewCmd(yttCmd, args...)
	if r.config.DryRun {
		r.config.Logger.Info().
			Str("command", yttExtraCmd.Name).
			Str("args", strings.Join(yttExtraCmd.Args, " ")).
			Msg("would run command")
		return nil
	}
	stdOut, stdErr, err := RunCMD(yttExtraCmd)
	if err != nil {
		r.config.Logger.Err(err).
			Str("command", yttExtraCmd.Name).
			Str("args", strings.Join(yttExtraCmd.Args, " ")).
			Str("sdtout", strings.Join(stdOut, "\n")).
			Str("stderr", strings.Join(stdErr, "\n")).
			Msg("failed to run command")

		// TODO: print error to stderr
		// Error message must be pretty printed to end users
		fmt.Printf("\n%s\n\n", strings.Join(stdErr, "\n"))
		return fmt.Errorf("failed to run command: %w", err)
	}
	tmpFile, err := ioutil.TempFile(tmpDir, "fully-compiled-")
	if err != nil {
		return fmt.Errorf("cannot create fully compiled file: %w", err)
	}
	defer func() {
		if err := tmpFile.Close(); err != nil {
			r.config.Logger.Err(err).
				Str("tmp file", tmpFile.Name()).
				Msg("failed to close temp file")
		}
	}()
	if _, err := tmpFile.WriteString(strings.Join(stdOut, "\n")); err != nil {
		return fmt.Errorf("cannot write full compiled file: %w", err)
	}
	outputDir := filepath.Join(r.config.RootDir, "build", r.config.Namespace)
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("cannot cleanup output directory: %w", err)
	}
	if err := os.MkdirAll(outputDir, defaultDirMod); err != nil {
		return fmt.Errorf("cannot create output directory: %w", err)
	}
	if _, err := YamlSplit(outputDir, tmpFile.Name()); err != nil {
		return fmt.Errorf("cannot split full compiled file: %w", err)
	}

	return nil
}

func YamlSplit(buildDir, inputFile string) ([]string, error) {
	var splitted []string
	var allResources []map[string]interface{}
	input, err := os.ReadFile(inputFile)
	if err != nil {
		return nil, err
	}
	if err := UnmarshalAllResources(input, &allResources); err != nil {
		return nil, err
	}
	for _, resource := range allResources {
		apiVersion, ok := resource["apiVersion"].(string)
		if !ok {
			return nil, fmt.Errorf("fail to type assert apiVersion from: %+v", resource)
		}
		kind, ok := resource["kind"].(string)
		if !ok {
			return nil, fmt.Errorf("kind missing from: %+v", resource)
		}
		metadata, ok := resource["metadata"].(map[interface{}]interface{})
		if !ok {
			return nil, fmt.Errorf("fail to type assert metadata from: %+v", resource)
		}
		name, ok := metadata["name"].(string)
		if !ok {
			return nil, fmt.Errorf("fail to type assert metadata.name from: %+v", resource)
		}
		filename := fmt.Sprintf("%s.%s.%s.yaml", kind, strings.ReplaceAll(apiVersion, "/", "_"), name)
		fPath := filepath.Join(buildDir, filename)

		out, err := yaml.Marshal(resource)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal resource: %w", err)
		}
		if err := os.MkdirAll(buildDir, defaultDirMod); err != nil {
			return nil, fmt.Errorf("cannot create build directory: %w", err)
		}
		content := append([]byte("---\n"), out...)
		if err := os.WriteFile(fPath, content, defaultFileMod); err != nil {
			return nil, fmt.Errorf("cannot write resource: %w", err)
		}
		splitted = append(splitted, fPath)
	}

	return splitted, nil
}

func UnmarshalAllResources(in []byte, out *[]map[string]interface{}) error {
	r := bytes.NewReader(in)
	decoder := yaml.NewDecoder(r)
	for {
		res := make(map[string]interface{})
		if err := decoder.Decode(&res); err != nil {
			// Break when there are no more documents to decode
			if !errors.Is(err, io.EOF) {
				return err
			}
			break
		}
		*out = append(*out, res)
	}
	return nil
}
