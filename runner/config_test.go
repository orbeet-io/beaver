package runner_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-yaml/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"orus.io/orus-io/beaver/runner"
	"orus.io/orus-io/beaver/testutils"
)

var (
	fixtures    = "fixtures/f1"
	shaFixtures = "fixtures/f2"
)

func TestConfig(t *testing.T) {
	config, err := runner.NewConfig(filepath.Join(fixtures, "base"))
	require.NoError(t, err)
	// first config.spec.variables entry name should be VAULT_KV in our test file
	assert.Equal(t, "VAULT_KV", config.Variables[0].Name)
	assert.Equal(t, "orus.io", config.Variables[0].Value)
	assert.Equal(t, "../vendor/helm/postgresql", config.Charts["postgres"].Path)
	assert.Equal(t, "../vendor/ytt/odoo", config.Charts["odoo"].Path)
}

func TestYttBuildArgs(t *testing.T) {
	tl := testutils.NewTestLogger(t)
	testNS := "environments/ns1"
	absConfigDir, err := filepath.Abs(fixtures)
	require.NoError(t, err)
	c := runner.NewCmdConfig(tl.Logger(), absConfigDir, testNS, false, "")
	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	require.NoError(t, c.Initialize(tmpDir))

	args := c.BuildYttArgs(c.Spec.Ytt, []string{"/tmp/postgres.1234.yaml", "/tmp/odoo.5678.yaml"})
	require.NoError(t, err)
	assert.Equal(
		t,
		// []string{
		// 	"-f", "/tmp/postgres.1234.yaml", "--file-mark=postgres.1234.yaml:type=yaml-plain",
		// 	"-f", "/tmp/odoo.5678.yaml", "--file-mark=odoo.5678.yaml:type=yaml-plain",
		// 	"-f", filepath.Join(fixtures, "base/ytt"),
		// 	"-f", filepath.Join(fixtures, "base/ytt.yml"),
		// 	"-f", filepath.Join(fixtures, "environments/ns1/ytt"),
		// 	"-f", filepath.Join(fixtures, "environments/ns1/ytt.yaml"),
		// },
		14,
		len(args),
	)
}

func TestCreateConfig(t *testing.T) {
	tl := testutils.NewTestLogger(t)
	testNS := "environments/ns1"
	absConfigDir, err := filepath.Abs(fixtures)
	require.NoError(t, err)
	c := runner.NewCmdConfig(tl.Logger(), absConfigDir, testNS, false, "")
	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	require.NoError(t, c.Initialize(tmpDir))
	require.Equal(t, 1, len(c.Spec.Creates))

	assert.True(t, filepath.IsAbs(c.Spec.Charts["postgres"].Path))
	crKey := runner.CmdCreateKey{
		Type: "configmap",
		Name: "xbus-pipelines",
	}
	cr, ok := c.Spec.Creates[crKey]
	assert.True(t, ok)
	require.Equal(t, 1, len(cr.Args))

	for k, create := range c.Spec.Creates {
		args := k.BuildArgs(c.Namespace, create.Args)
		assert.Equal(
			t,
			[]string{
				"-n", c.Namespace,
				"create",
				"configmap", "xbus-pipelines",
				"--dry-run=client", "-o", "yaml",
				"--from-file", "environments/ns1/pipelines",
			},
			args,
		)
	}
}

func TestSha(t *testing.T) {
	shaValue := "2145bea9e32804c65d960e6d4af1c87f95ccc39fad7df5eec2f3925a193112ab"
	tl := testutils.NewTestLogger(t)
	absConfigDir, err := filepath.Abs(shaFixtures)
	require.NoError(t, err)
	c := runner.NewCmdConfig(tl.Logger(), absConfigDir, "base", false, "")
	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	require.NoError(t, c.Initialize(tmpDir))
	r := runner.NewRunner(c)
	require.NoError(t, r.Build(tmpDir))

	buildDir := filepath.Join(shaFixtures, "build", "example")

	configMap := filepath.Join(buildDir, "ConfigMap.v1.demo.yaml")
	cm,  err := parseFile(configMap)
	require.NoError(t, err)
	sha, err := getLabel(cm, "mysha")
	require.NoError(t, err)
	require.Equal(t, shaValue, sha)

	deployment := filepath.Join(buildDir, "Deployment.apps_v1.nginx.yaml")
	deploy, err := parseFile(deployment)
	require.NoError(t, err)
	sha, err = getLabel(deploy, "config.sha")
	require.NoError(t, err)
	require.Equal(t, shaValue, sha)
}

func getLabel(resource map[string]interface{}, label string) (string, error) {
	metadata, ok := resource["metadata"].(map[interface{}]interface{})
	if !ok {
		return "", fmt.Errorf("fail to get label")
	}
	labels, ok := metadata["labels"].(map[interface{}]interface{})
	if !ok {
		return "", fmt.Errorf("fail to get label")
	}
	result, ok := labels[label].(string)
	if !ok {
		return "", fmt.Errorf("fail to get label")
	}
	return result, nil
}

func parseFile(input string) (map[string]interface{}, error) {
	resource := make(map[string]interface{})

	content, err := os.ReadFile(input)
	if err != nil {
		return nil, fmt.Errorf("fail to parse: %w", err)
	}

	byteContent := bytes.NewReader(content)
	decoder := yaml.NewDecoder(byteContent)

	err = decoder.Decode(&resource)
	if err != nil {
		return nil, fmt.Errorf("fail to parse: %w", err)
	}

	return resource, nil
}
