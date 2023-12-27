package runner_test

import (
	"fmt"
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
	stdout, stderr, err := runner.RunCMD(c)
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
	c := runner.NewCmdConfig(tl.Logger(), absConfigDir, testPath, false, false, "", "")
	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	require.NoError(t, c.Initialize(tmpDir))

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

	t.Run("build", func(t *testing.T) {
		buildDir := filepath.Join(fixtures, "build", "ns1")
		defer func() {
			require.NoError(t, runner.CleanDir(buildDir))
		}()
		r := runner.NewRunner(c)
		require.NoError(t, r.Build(tmpDir))

		deployment := filepath.Join(buildDir, "Deployment.apps_v1.postgres.yaml")
		deploy, err := parseFile(deployment)
		require.NoError(t, err)

		odooConf := filepath.Join(buildDir, "Secret.v1.odoo_conf.yaml")
		_, err = parseFile(odooConf)
		require.NoError(t, err, "we produced an invalid yaml resource")

		envVars, err := getEnvVars(deploy)
		require.NoError(t, err)

		pguser, ok := envVars["PGUSER"]
		require.True(t, ok)
		assert.Equal(t, "<path:k8s.orus.io/data/ns1/postgres#username>", pguser)

		pgdatabase, ok := envVars["PGDATABASE"]
		require.True(t, ok)
		assert.Equal(t, "<path:k8s.orus.io/data/ns1/postgres#database>", pgdatabase)
	})
}

func getEnvVars(resource map[string]interface{}) (map[string]string, error) {
	result := make(map[string]string)
	spec, ok := resource["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("fail to env var: spec")
	}
	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("fail to env var: template")
	}
	containersSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("fail to env var: containersSpec")
	}
	containers, ok := containersSpec["containers"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("fail to env var: containers")
	}
	container, ok := containers[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("fail to env var: container")
	}
	env, ok := container["env"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("fail to env var: env")
	}
	for _, item := range env {
		e, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("fail to env var: env var: %v", item)
		}
		name, ok := e["name"].(string)
		if !ok {
			return nil, fmt.Errorf("fail to env var: env var name: %v", item)
		}
		value, ok := e["value"].(string)
		if !ok {
			return nil, fmt.Errorf("fail to env var: env var value: %v", item)
		}
		result[name] = value
	}
	return result, nil
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
	compiled := "test_split_input.yaml"
	buildDir := filepath.Join(fixtures, "build", namespace)
	compiledFiles, err := runner.YamlSplit(buildDir, filepath.Join(fixtures, compiled))
	require.NoError(t, err)
	require.Equal(t, 4, len(compiledFiles))
	for _, filePath := range compiledFiles {
		fileName := filepath.Base(filePath)
		tokens := strings.Split(fileName, ".")
		require.Equal(t, 4, len(tokens))

		// <kind>.<apiVersion>.<name>.yaml
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
