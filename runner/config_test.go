package runner_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"orus.io/orus-io/beaver/runner"
	"orus.io/orus-io/beaver/testutils"
)

const (
	fixtures              = "fixtures/f1"
	shaFixtures           = "fixtures/f2"
	helmNamespaceFixtures = "fixtures/f3"
	disabledAsVar         = "fixtures/fDisabledAsVar"
	namespaceAsVar        = "fixtures/fNamespaceAsVar"
	versionTest           = "fixtures/versionTest"
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

func TestConfigVersionValid(t *testing.T) {
	config, err := runner.NewConfig(filepath.Join(versionTest, "validbase"))
	require.NoError(t, err)
	err = runner.ControlVersions(config.BeaverVersion, "3.2.3")
	require.NoError(t, err)
}

func TestConfigVersionInValid(t *testing.T) {
	config, err := runner.NewConfig(filepath.Join(versionTest, "invalidbase"))
	require.NoError(t, err)
	err = runner.ControlVersions(config.BeaverVersion, "3.2.3")
	require.Error(t, err)
}

func TestBuildArgs(t *testing.T) {
	config, err := runner.NewConfig(filepath.Join(helmNamespaceFixtures, "base"))
	require.NoError(t, err)

	for _, chart := range config.Charts {
		cmdChart := runner.CmdChartFromChart(chart)
		result, err := cmdChart.BuildArgs("my-name", "my-namespace")
		require.NoError(t, err)
		if cmdChart.Namespace != "" {
			assert.Equal(t, cmdChart.Namespace, result[4])
		} else {
			assert.Equal(t, "my-namespace", result[4])
		}
	}
}

func TestHydrate(t *testing.T) {
	rawVariables := []byte(`
#@data/values
---
foo: |
    a multi
    line string
bar:
    port: 5432
    simple: interface
    with:
        - some
        - content
baz: |
    only one line in multiline mode
boo: a simple joke line
voo: 33
`)
	variables := make(map[string]interface{})

	byteContent := bytes.NewReader(rawVariables)
	decoder := yaml.NewDecoder(byteContent)

	require.NoError(t, decoder.Decode(&variables))
	input := `
#@data/values
---
foo: <[foo]>
bar: <[bar]>
baz: <[baz]>
boo: <[boo]>
voo: <[voo]>
barport: <[bar.port]>
barwith: <[bar.with.1]>
`

	buf := bytes.NewBufferString("")
	require.NoError(t, runner.Hydrate([]byte(input), buf, variables))
	assert.Equal(
		t,
		`
#@data/values
---
foo: |
    a multi
    line string
bar:
    port: 5432
    simple: interface
    with:
        - some
        - content
baz: |
    only one line in multiline mode
boo: a simple joke line
voo: 33
barport: 5432
barwith: content
`,
		buf.String(),
	)
}

func TestHydrateSideEffect(t *testing.T) {
	rawVariables := []byte(`
---
foo: 33
`)
	variables := make(map[string]interface{})

	byteContent := bytes.NewReader(rawVariables)
	decoder := yaml.NewDecoder(byteContent)

	require.NoError(t, decoder.Decode(&variables))
	input := `
---
foo: <[foo]>
bar: |
  [section1]
  value1 = 1
  # ----------
  # some comment
  # ----------
  value2 = 2
replicas: 1
`
	expected := `
---
foo: 33
bar: |
    [section1]
    value1 = 1
    # ----------
    # some comment
    # ----------
    value2 = 2
replicas: 1
`

	buf := bytes.NewBufferString("")
	require.NoError(t, runner.Hydrate([]byte(input), buf, variables))
	assert.Equal(
		t,
		expected,
		buf.String(),
	)
}

func TestYttBuildArgs(t *testing.T) {
	tl := testutils.NewTestLogger(t)
	testNS := "environments/ns1"
	absConfigDir, err := filepath.Abs(fixtures)
	require.NoError(t, err)
	c := runner.NewCmdConfig(tl.Logger(), absConfigDir, testNS, false, "", "")
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

func TestInheritVariables(t *testing.T) {
	tl := testutils.NewTestLogger(t)
	testNS := "environments/ns1"
	absConfigDir, err := filepath.Abs(fixtures)
	require.NoError(t, err)
	c := runner.NewCmdConfig(tl.Logger(), absConfigDir, testNS, false, "", "")
	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	require.NoError(t, c.Initialize(tmpDir))
	assert.Equal(t, "another value", c.Spec.Variables.GetD("test-nested.nested-value1", nil))
	assert.Equal(t, "Value2", c.Spec.Variables.GetD("test-nested.nested-value2", nil))
}

func TestCreateConfig(t *testing.T) {
	tl := testutils.NewTestLogger(t)
	testNS := "environments/ns1"
	absConfigDir, err := filepath.Abs(fixtures)
	require.NoError(t, err)
	c := runner.NewCmdConfig(tl.Logger(), absConfigDir, testNS, false, "", "")
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
				"--from-file", "pipelines",
			},
			args,
		)
	}
}

type disabledTCase struct {
	Name        string
	TestPath    string
	FilePresent bool
}

func TestDisabledAsVariable(t *testing.T) {
	tCases := []disabledTCase{
		{
			Name:        "disabled",
			TestPath:    "noconfigmap",
			FilePresent: false,
		},
		{
			Name:        "enabled",
			TestPath:    "configmapenabled",
			FilePresent: true,
		},
	}
	for _, tCase := range tCases {
		t.Run(tCase.Name, func(t *testing.T) {
			runTestDisabledAsVar(t, tCase)
		})
	}
}

func runTestDisabledAsVar(t *testing.T, tCase disabledTCase) {
	t.Helper()
	buildDir := filepath.Join(disabledAsVar, "build", "example")
	defer func() {
		require.NoError(t, runner.CleanDir(buildDir))
	}()

	tl := testutils.NewTestLogger(t)
	absRootDir, err := filepath.Abs(disabledAsVar)
	require.NoError(t, err)
	c := runner.NewCmdConfig(tl.Logger(), absRootDir, tCase.TestPath, false, "", "")
	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	require.NoError(t, c.Initialize(tmpDir))
	r := runner.NewRunner(c)
	require.NoError(t, r.Build(tmpDir))

	outPutConfigMapName := filepath.Join(buildDir, "ConfigMap.v1.demo.yaml")
	if !tCase.FilePresent {
		_, err = os.Stat(outPutConfigMapName)
		require.Error(t, err)
		// file should not exist since it is disabled by a variable in the ns2/beaver.yaml
		require.True(t, errors.Is(err, os.ErrNotExist))
	} else {
		_, err = os.Stat(outPutConfigMapName)
		// file should exist
		require.NoError(t, err)
	}
}

type namespaceTCase struct {
	Name             string
	TestPath         string
	ExpectedBuildDir string
	FilePresent      bool
}

func TestNamaspaceAsVariable(t *testing.T) {
	tCases := []namespaceTCase{
		{
			Name:             "ns1yo",
			TestPath:         "ns1",
			ExpectedBuildDir: filepath.Join(namespaceAsVar, "build", "ns1yo"),
			FilePresent:      true,
		},
		{
			Name:             "ns2yo",
			TestPath:         "ns2",
			ExpectedBuildDir: filepath.Join(namespaceAsVar, "build", "ns2yo"),
			FilePresent:      true,
		},
	}
	for _, tCase := range tCases {
		t.Run(tCase.Name, func(t *testing.T) {
			runTestBeaverNamespaceAsVariable(t, tCase)
		})
	}
}

func runTestBeaverNamespaceAsVariable(t *testing.T, tCase namespaceTCase) {
	t.Helper()
	defer func() {
		require.NoError(t, runner.CleanDir(tCase.ExpectedBuildDir))
		require.NoError(t, os.RemoveAll(tCase.ExpectedBuildDir))
	}()

	tl := testutils.NewTestLogger(t)
	absRootDir, err := filepath.Abs(namespaceAsVar)
	require.NoError(t, err)
	c := runner.NewCmdConfig(tl.Logger(), absRootDir, tCase.TestPath, false, "", "")
	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	require.NoError(t, c.Initialize(tmpDir))
	r := runner.NewRunner(c)
	require.NoError(t, r.Build(tmpDir))

	// the output file should be in a directory that matches the variable namespace
	outPutConfigMapName := filepath.Join(tCase.ExpectedBuildDir, "ConfigMap.v1.demo.yaml")
	if !tCase.FilePresent {
		_, err = os.Stat(outPutConfigMapName)
		require.Error(t, err)
		// file should not exist since it is disabled by a variable in the ns2/beaver.yaml
		require.True(t, errors.Is(err, os.ErrNotExist))
	} else {
		_, err = os.Stat(outPutConfigMapName)
		// file should exist
		require.NoError(t, err)
	}
}

func TestSha(t *testing.T) {
	shaValue := "2145bea9e32804c65d960e6d4af1c87f95ccc39fad7df5eec2f3925a193112ab"
	buildDir := filepath.Join(shaFixtures, "build", "example")
	defer func() {
		require.NoError(t, runner.CleanDir(buildDir))
	}()

	tl := testutils.NewTestLogger(t)
	absConfigDir, err := filepath.Abs(shaFixtures)
	require.NoError(t, err)
	c := runner.NewCmdConfig(tl.Logger(), absConfigDir, "base", false, "", "")
	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	require.NoError(t, c.Initialize(tmpDir))
	r := runner.NewRunner(c)
	require.NoError(t, r.Build(tmpDir))

	configMap := filepath.Join(buildDir, "ConfigMap.v1.demo.yaml")
	cm, err := parseFile(configMap)
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
	metadata, ok := resource["metadata"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("fail to get label: metadata on %+v", resource["metadata"])
	}
	labels, ok := metadata["labels"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("fail to get label: labels on %+v", metadata["labels"])
	}
	result, ok := labels[label].(string)
	if !ok {
		return "", fmt.Errorf("fail to get label: result")
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
