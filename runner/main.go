package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-cmd/cmd"
	"gopkg.in/yaml.v3"
)

var (
	defaultFileMod os.FileMode = 0600
	defaultDirMod  os.FileMode = 0700
	// TODO: find commands full path
	yttCmd     = "ytt"
	helmCmd    = "helm"
	kubectlCmd = "kubectl"
)

// Runner is the struct in charge of launching commands
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
	variables, err := r.config.prepareVariables(false)
	if err != nil {
		return fmt.Errorf("cannot prepare variables: %w", err)
	}
	var outputDir string
	if r.config.Output == "" {
		w := bytes.NewBuffer([]byte{})
		if err := hydrateString(r.config.Namespace, w, variables); err != nil {
			return err
		}
		r.config.Namespace = w.String()
		outputDir = filepath.Join(r.config.RootDir, "build", r.config.Namespace)
	} else {
		outputDir = r.config.Output
	}

	for name := range r.config.Spec.Charts {
		w := bytes.NewBuffer([]byte{})
		if err := hydrateString(r.config.Spec.Charts[name].Disabled, w, variables); err != nil {
			return err
		}
		chart := r.config.Spec.Charts[name]
		chart.Disabled = w.String()
		r.config.Spec.Charts[name] = chart
	}
	preBuildDir := filepath.Join(tmpDir, "pre-build")
	if err := r.DoBuild(tmpDir, preBuildDir); err != nil {
		return fmt.Errorf("failed to do pre-build: %w", err)
	}
	if err := r.config.SetShas(preBuildDir); err != nil {
		return fmt.Errorf("failed to set SHAs: %w", err)
	}
	files, err := os.ReadDir(preBuildDir)
	if err != nil {
		return fmt.Errorf("cannot list directory: %s - %w", preBuildDir, err)
	}
	if outputDir != "stdout" {
		if err := CleanDir(outputDir); err != nil {
			return fmt.Errorf("cannot clean dir: %s: %w", outputDir, err)
		}
	}
	variables, err = r.config.prepareVariables(true)
	if err != nil {
		return fmt.Errorf("cannot prepare variables: %w", err)
	}
	for _, file := range files {
		var outFilePath string
		var outFile *os.File
		inFilePath := filepath.Join(preBuildDir, file.Name())
		if outputDir == "stdout" {
			outFilePath = "stdout"
			outFile = os.Stdout
		} else {
			var outputFileName bytes.Buffer
			if err := Hydrate([]byte(file.Name()), &outputFileName, variables); err != nil {
				return fmt.Errorf("cannot hydrate file name: %w", err)
			}
			outFilePath = filepath.Join(outputDir, strings.TrimSuffix(outputFileName.String(), "\n"))
			outFile, err = os.Create(outFilePath)
			if err != nil {
				return fmt.Errorf("cannot open: %s - %w", outFilePath, err)
			}
			defer func() {
				if err := outFile.Close(); err != nil {
					r.config.Logger.Fatal().Err(err).Msg("cannot close hydrated file")
				}
			}()
		}
		if err := hydrate(inFilePath, outFile, variables, r.config.WithoutHydrate); err != nil {
			return fmt.Errorf("cannot hydrate: %s - %w", outFilePath, err)
		}
	}
	return nil
}

func (r *Runner) DoBuild(tmpDir, outputDir string) error {
	cmds, err := r.prepareCmds()
	if err != nil {
		return err
	}

	compiled, err := r.runCommands(tmpDir, cmds)
	if err != nil {
		return err
	}

	yttOutput, err := r.runYtt(tmpDir, compiled)
	if err != nil {
		return err
	}

	kustomizeOutput, err := r.kustomize(tmpDir, yttOutput)
	if err != nil {
		return err
	}

	if r.config.DryRun {
		return nil
	}

	if err := CleanDir(outputDir); err != nil {
		return fmt.Errorf("cannot clean dir: %s: %w", outputDir, err)
	}
	if _, err := YamlSplit(outputDir, kustomizeOutput.Name()); err != nil {
		return fmt.Errorf("cannot split full compiled file: %w", err)
	}

	return nil
}

func (r *Runner) kustomize(tmpDir string, input *os.File) (*os.File, error) {
	kustomizeFilePath := filepath.Join(tmpDir, "kustomization.yaml")
	f, err := os.Create(kustomizeFilePath)
	if err != nil {
		return nil, fmt.Errorf("fail to open %s: %w", kustomizeFilePath, err)
	}
	_, err = f.Write([]byte(fmt.Sprintf("resources: [%s]", filepath.Base(input.Name()))))
	if err != nil {
		return nil, fmt.Errorf("fail to write %s: %w", kustomizeFilePath, err)
	}

	variables, err := r.config.prepareVariables(false)
	if err != nil {
		return nil, fmt.Errorf("cannot prepare kustomize variables: %w", err)
	}

	var lastKustomizeFolder string

	for _, layer := range r.config.Layers {
		for _, ext := range []string{"yml", "yaml"} {
			fName := fmt.Sprintf("kustomization.%s", ext)
			fPath := filepath.Join(layer, "kustomize", fName)

			fStat, err := os.Stat(fPath)
			if err != nil || fStat.IsDir() {
				continue
			}
			backupFile := fmt.Sprintf("%s.back", fPath)
			if err := Copy(fPath, backupFile); err != nil {
				return nil, fmt.Errorf("cannot copy kustomization file: %w", err)
			}
			defer func(fPath string) {
				if err := Copy(backupFile, fPath); err != nil {
					r.config.Logger.Fatal().Err(err).Msg("cannot restore kustomization back file")
				}
				if err := os.Remove(backupFile); err != nil {
					r.config.Logger.Fatal().Err(err).Msg("cannot remove kustomization back file")
				}
			}(fPath)

			if err := os.Remove(fPath); err != nil {
				return nil, fmt.Errorf("cannot remove original kustomization file: %w", err)
			}
			outFile, err := os.Create(fPath)
			if err != nil {
				return nil, fmt.Errorf("cannot open: %s - %w", fPath, err)
			}
			defer func() {
				if err := outFile.Close(); err != nil {
					r.config.Logger.Fatal().Err(err).Msg("cannot close hydrated kustomization file")
				}
			}()

			// kustomize root cannot be absolute
			RelInputFilePath, err := filepath.Rel(filepath.Join(layer, "kustomize"), tmpDir)
			if err != nil {
				return nil, fmt.Errorf("cannot find relative path for: %s - %w", tmpDir, err)
			}
			variables["beaver"] = map[string]interface{}{
				"build": RelInputFilePath,
			}

			if err := hydrate(backupFile, outFile, variables, r.config.WithoutHydrate); err != nil {
				return nil, fmt.Errorf("cannot hydrate: %s - %w", fPath, err)
			}
			lastKustomizeFolder = filepath.Join(layer, "kustomize")
		}
	}

	// now run customize on the last layer with a kustomize folder
	if lastKustomizeFolder != "" {
		// now run customize on the last layer with a kustomize folder
		kustomizeCmd := cmd.NewCmd(kubectlCmd, []string{"kustomize", lastKustomizeFolder}...)
		return r.runCommand(tmpDir, "kustomize", kustomizeCmd)
	}

	return input, nil
}

func toBool(s string) (bool, error) {
	sLower := strings.ToLower(s)
	finalString := strings.TrimSuffix(sLower, "\n")
	switch finalString {
	case "0", "false", "":
		return false, nil
	case "1", "true":
		return true, nil
	default:
		return false, errors.New("cannot parse " + s + " as bool")
	}
}

func (r *Runner) prepareCmds() (map[string]*cmd.Cmd, error) {
	// create helm commands
	// create ytt chart commands
	cmds := make(map[string]*cmd.Cmd)
	for name, chart := range r.config.Spec.Charts {
		disabled, err := toBool(chart.Disabled)
		if err != nil {
			return nil, err
		}
		if disabled {
			continue
		}
		args, err := chart.BuildArgs(name, r.config.Namespace)
		if err != nil {
			return nil, fmt.Errorf("build: failed to build args %w", err)
		}
		switch chart.Type {
		case HelmType:
			cmds[name] = cmd.NewCmd(helmCmd, args...)
		case YttType:
			cmds[name] = cmd.NewCmd(yttCmd, args...)
		default:
			return nil, fmt.Errorf("unsupported chart %s type: %q", chart.Path, chart.Type)
		}
	}

	for key, create := range r.config.Spec.Creates {
		strArgs := key.BuildArgs(r.config.Namespace, create.Args)
		name := fmt.Sprintf("%s_%s", key.Type, key.Name)
		c := cmd.NewCmd(kubectlCmd, strArgs...)
		c.Dir = create.Dir
		cmds[name] = c
	}

	return cmds, nil
}
func (r *Runner) runCommand(tmpDir, name string, cmd *cmd.Cmd) (*os.File, error) {
	tmpFile, err := os.CreateTemp(tmpDir, fmt.Sprintf("compiled-%s-*.yaml", name))
	if err != nil {
		return nil, fmt.Errorf("cannot create compiled file: %w", err)
	}
	defer func() {
		if err := tmpFile.Close(); err != nil {
			r.config.Logger.
				Err(err).
				Str("temp file", tmpFile.Name()).
				Msg("failed to close temp file")
		}
	}()
	if r.config.DryRun {
		r.config.Logger.Info().
			Str("command", cmd.Name).
			Strs("args", cmd.Args).
			Msg("would run command")
		return tmpFile, nil
	}
	r.config.Logger.Debug().
		Str("command", cmd.Name).
		Strs("args", cmd.Args).
		Msg("running command")
	stdOut, stdErr, err := RunCMD(cmd)
	if err != nil {
		r.config.Logger.Err(err).
			Str("command", cmd.Name).
			Str("args", strings.Join(cmd.Args, " ")).
			Str("sdtout", strings.Join(stdOut, "\n")).
			Str("stderr", strings.Join(stdErr, "\n")).
			Msg("failed to run command")

		// Error must be pretty printed to end users /!\
		fmt.Printf("\n%s\n\n", strings.Join(stdErr, "\n"))
		return nil, fmt.Errorf("failed to run command: %w", err)
	}
	if _, err := tmpFile.WriteString(strings.Join(stdOut, "\n")); err != nil {
		return nil, fmt.Errorf("cannot write compiled file: %w", err)
	}
	return tmpFile, nil
}

func (r *Runner) runCommands(tmpDir string, cmds map[string]*cmd.Cmd) ([]string, error) {
	var compiled []string
	var wg sync.WaitGroup
	errors := make(chan error, len(cmds))
	results := make(chan string, len(cmds))
	for name, command := range cmds {
		wg.Add(1)
		go func(name string, c *cmd.Cmd) {
			defer wg.Done()
			f, err := r.runCommand(tmpDir, name, c)
			if err != nil {
				errors <- err
			}
			results <- f.Name()
		}(name, command)
	}
	wg.Wait()
	select {
	case err := <-errors:
		// return only the first error if any
		return nil, err
	default:
		close(results)
		for res := range results {
			compiled = append(compiled, res)
		}
		return compiled, nil
	}
}

func (r *Runner) runYtt(tmpDir string, compiled []string) (*os.File, error) {
	// create ytt additional command
	args := r.config.BuildYttArgs(r.config.Spec.Ytt, compiled)

	yttExtraCmd := cmd.NewCmd(yttCmd, args...)
	return r.runCommand(tmpDir, "ytt", yttExtraCmd)
}

func CleanDir(directory string) error {
	if err := os.RemoveAll(directory); err != nil {
		return fmt.Errorf("cannot cleanup output directory: %w", err)
	}
	if err := os.MkdirAll(directory, defaultDirMod); err != nil {
		return fmt.Errorf("cannot create output directory: %w", err)
	}
	return nil
}

func Copy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

// YamlSplit takes a buildDir and an inputFile.
// it returns a list of yaml documents and an eventual error
func YamlSplit(buildDir, inputFile string) ([]string, error) {
	var docs []string
	var allResources []map[string]interface{}
	input, err := os.ReadFile(inputFile)
	if err != nil {
		return nil, err
	}
	if err := unmarshalAllResources(input, &allResources); err != nil {
		return nil, err
	}
	for _, resource := range allResources {
		apiVersionData, ok := resource["apiVersion"]
		if !ok {
			return nil, fmt.Errorf("apiVersion not present in resource: %+v", resource)
		}
		apiVersion, ok := apiVersionData.(string)
		if !ok {
			return nil, fmt.Errorf("failed to type assert apiVersion to string from: %+v", apiVersionData)
		}
		kind, ok := resource["kind"].(string)
		if !ok {
			return nil, fmt.Errorf("kind missing from: %+v", resource)
		}
		metadata, ok := resource["metadata"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("fail to type assert metadata from: %+v", resource)
		}
		namespace, ok := metadata["namespace"]
		if !ok {
			namespace = ""
		}
		name, ok := metadata["name"]
		if !ok {
			return nil, fmt.Errorf("fail to type get metadata.name from: %+v", resource)
		}
		filename := ""
		if namespace != "" {
			filename = fmt.Sprintf("%s.%s.%s.%s.yaml", kind, strings.ReplaceAll(apiVersion, "/", "_"), namespace, name)
		} else {
			filename = fmt.Sprintf("%s.%s.%s.yaml", kind, strings.ReplaceAll(apiVersion, "/", "_"), name)
		}
		fPath := filepath.Join(buildDir, filename)

		buf := new(bytes.Buffer)
		encoder := yaml.NewEncoder(buf)
		encoder.SetIndent(2)
		if err := encoder.Encode(resource); err != nil {
			return nil, fmt.Errorf("cannot encode resource: %+v, %w", resource, err)
		}
		/*
			out, err := yaml.Marshal(resource)
			if err != nil {
				return nil, fmt.Errorf("cannot marshal resource: %w", err)
			}
		*/
		if err := os.MkdirAll(buildDir, defaultDirMod); err != nil {
			return nil, fmt.Errorf("cannot create build directory: %w", err)
		}
		content := append([]byte("---\n"), buf.Bytes()...)
		if err := os.WriteFile(fPath, content, defaultFileMod); err != nil {
			return nil, fmt.Errorf("cannot write resource: %w", err)
		}
		docs = append(docs, fPath)
	}

	return docs, nil
}

func unmarshalAllResources(in []byte, out *[]map[string]interface{}) error {
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
