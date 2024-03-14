package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-cmd/cmd"
	"gopkg.in/yaml.v3"
)

type HelmDependency struct {
	Name       string
	Repository string
}

type HelmChart struct {
	Dependencies []HelmDependency
}

func (c CmdConfig) HelmDependencyBuild() error {
	paths, err := c.HelmChartsPaths()
	if err != nil {
		return err
	}
	c.Logger.Debug().Strs("paths", paths).Msg("found helm dependencies")
	for _, p := range paths {
		if err := c.HelmBuildDependency(p); err != nil {
			return err
		}
	}

	return nil
}

func (c CmdConfig) HelmBuildDependency(path string) error {
	args := []string{"dependency", "build", path}
	apiCmd := cmd.NewCmd(helmCmd, args...)
	stdOut, stdErr, err := RunCMD(apiCmd)
	if err != nil {
		c.Logger.Err(err).
			Str("command", helmCmd).
			Str("args", strings.Join(args, " ")).
			Str("sdtout", strings.Join(stdOut, "\n")).
			Str("stderr", strings.Join(stdErr, "\n")).
			Msg("failed to run command")

		// Error must be pretty printed to end users /!\
		fmt.Printf("\n%s\n\n", strings.Join(stdErr, "\n"))
		return fmt.Errorf("failed to run command: %w", err)
	}
	c.Logger.Debug().
		Strs("stdout", stdOut).
		Str("path", path).
		Msg("helm dependencies successfully built")
	return nil
}

func (c CmdConfig) HelmChartsPaths() ([]string, error) {
	var allPaths []string
	for name, chart := range c.Spec.Charts {
		if chart.Type == "helm" {
			c.Logger.Debug().
				Str("chart", name).
				Str("type", chart.Type).
				Str("path", chart.Path).
				Msg("search helm dependencies for")
			paths, err := c.pathsByChart(chart.Path)
			if err != nil {
				return nil, err
			}
			for _, p := range paths {
				// Avoid infinite loop with circular dependencies.
				// Also improve the performance by templating only
				// once any given chart in case the dependency is
				// used multiple times.
				if !contains(allPaths, p) {
					allPaths = append(allPaths, p)
				}
			}
		}
	}
	return allPaths, nil
}

func (c CmdConfig) pathsByChart(path string) ([]string, error) {
	var allPaths []string
	helmChart, err := getHelmChart(path)
	if err != nil {
		return nil, err
	}
	for _, dependency := range helmChart.Dependencies {
		c.Logger.Debug().
			Str("chart", dependency.Name).
			Str("repository", dependency.Repository).
			Msg("found helm dependency")

		if strings.HasPrefix(dependency.Repository, "file://") {
			subChartPath := filepath.Join(
				path, strings.TrimPrefix(dependency.Repository, "file://"))

			subChartsDependenciesPaths, err := c.pathsByChart(subChartPath)
			if err != nil {
				return nil, err
			}

			allPaths = append(allPaths, subChartsDependenciesPaths...)
		}
	}
	allPaths = append(allPaths, path)
	return allPaths, nil
}

func getHelmChart(path string) (*HelmChart, error) {
	helmChart := HelmChart{}
	for _, ext := range []string{"yaml", "yml"} {
		helmChartFile := filepath.Join(path, fmt.Sprintf("%s.%s", "Chart", ext))
		fileInfo, err := os.Stat(helmChartFile)
		if err != nil || fileInfo.IsDir() {
			continue
		}
		helmChartContent, err := os.ReadFile(helmChartFile)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(helmChartContent, &helmChart)
		if err != nil {
			return nil, err
		}
		return &helmChart, nil
	}
	return nil, fmt.Errorf("helm Chart.yaml file not found in %s", path)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
