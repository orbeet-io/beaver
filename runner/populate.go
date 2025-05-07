package runner

import (
	"fmt"
	"os"
	"path/filepath"
)

func (c *CmdConfig) populate() {
	c.Spec.Charts = FindFiles(c.Layers, c.Spec.Charts)
	c.Spec.Ytt = findYtts(c.Layers)
}

// findYtts looks for `ytt` folder and/or `ytt.y[a]ml` file in beaver projects.
func findYtts(layers []string) []string {
	var result []string

	// we cannot use findYaml here because the order matters.
	for i := len(layers); i != 0; i-- {
		layer := layers[i-1]
		yttDirPath := filepath.Join(layer, "ytt")

		yttDirInfo, err := os.Stat(yttDirPath)
		if err == nil && yttDirInfo.IsDir() {
			result = append(result, yttDirPath)
		}

		for _, ext := range []string{"yaml", "yml"} {
			yttFilePath := filepath.Join(layer, "ytt."+ext)

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
