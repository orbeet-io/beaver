package runner_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-cmd/cmd"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"orus.io/orus-io/beaver/runner"
	"orus.io/orus-io/beaver/testutils"
)

func TestRunCMD(t *testing.T) {
	c := cmd.NewCmd("echo", "p00f")
	err, stdout, stderr := runner.RunCMD(c)
	require.NoError(t, err)
	for _, out := range stdout {
		assert.Equal(t, "p00f", out)
		fmt.Println(out)
	}
	for _, errMsg := range stderr {
		fmt.Println(errMsg)
	}
}

func TestCmdConfig(t *testing.T) {
	tl := testutils.NewTestLogger(t)
	testPath := filepath.Join("environments", "ns1")
	absConfigDir, err := filepath.Abs(fixtures)
	require.NoError(t, err)
	c := runner.NewCmdConfig(tl.Logger(), absConfigDir, testPath, false)
	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	require.NoError(t, c.Initialize(tmpDir))

	t.Run("helmCharts", func(t *testing.T) {
		pgHelmChart, ok := c.Spec.Charts["postgres"]
		require.True(t, ok, "we should have a postgres helm chart in our cmdConfig")

		require.Equal(t, 2, len(pgHelmChart.ValuesFileNames))
		file1Content, err := ioutil.ReadFile(pgHelmChart.ValuesFileNames[0])
		require.NoError(t, err)

		assert.Equal(t, `persistence:
  storageClass: huawei-iscsi

initdbScripts:
  create.sql: |
    CREATE EXTENSION IF NOT EXISTS unaccent;

postgresqlUsername: "<path:k8s.orus.io/data/ns1/postgres#username>"
postgresqlDatabase: "<path:k8s.orus.io/data/ns1/postgres#database>"
`, string(file1Content))

		file2Content, err := ioutil.ReadFile(pgHelmChart.ValuesFileNames[1])
		require.NoError(t, err)
		assert.Equal(t, `image:
  tag: 14
`, string(file2Content))
	})

	t.Run("yttCharts", func(t *testing.T) {
		odooYttChart, ok := c.Spec.Charts["odoo"]
		require.True(t, ok, "we should have an odoo ytt chart in our cmdConfig")
		assert.Equal(t, 2, len(odooYttChart.ValuesFileNames))
	})

	t.Run("yttPatches", func(t *testing.T) {
		yttPatches := c.Spec.Ytt
		l := tl.Logger()
		logger := &l
		logger.Debug().Str("patches", fmt.Sprintf("%+v", yttPatches)).Msg("found patches")
		require.Equal(t, 4, len(yttPatches))
	})
}

func TestFindFiles(t *testing.T) {
	namespace := "ns1"

	charts := map[string]runner.CmdChart{
		"postgres": {
			Path:            "postgres",
			ValuesFileNames: nil,
		},
	}
	layers := []string{
		fmt.Sprintf("fixtures/f1/environments/%s", namespace),
		"fixtures/f1/base",
	}

	newCharts := runner.FindFiles(layers, charts)
	require.Equal(t, 2, len(newCharts["postgres"].ValuesFileNames))
}

func TestYamlSplit(t *testing.T) {
	namespace := "ns1"
	compiled := "output.yaml"
	buildDir := filepath.Join(fixtures, "build", namespace)
	compiledFiles, err := runner.YamlSplit(buildDir, filepath.Join(fixtures, compiled))
	require.NoError(t, err)
	require.Equal(t, 3, len(compiledFiles))
	for _, filePath := range compiledFiles {
		fileName := filepath.Base(filePath)
		tokens := strings.Split(fileName, ".")
		require.Equal(t, 4, len(tokens))

		// <apiVersion>.<kind>.<name>.yaml
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "---", string(content)[:3])

		v := viper.New()
		v.SetConfigName(strings.TrimSuffix(fileName, path.Ext(fileName)))
		v.AddConfigPath(strings.TrimSuffix(filePath, fileName))
		require.NoError(t, v.ReadInConfig())

		resource := make(map[string]interface{})
		require.NoError(t, v.Unmarshal(&resource))

		kind, ok := resource["kind"].(string)
		require.True(t, ok)
		assert.Equal(t, tokens[0], kind)

		apiVersion, ok := resource["apiversion"].(string)
		require.True(t, ok)
		assert.Equal(t, tokens[1], apiVersion)

		metadata, ok := resource["metadata"].(map[string]interface{})
		require.True(t, ok)
		name, ok := metadata["name"].(string)
		require.True(t, ok)
		assert.Equal(t, tokens[2], name)
	}
}
