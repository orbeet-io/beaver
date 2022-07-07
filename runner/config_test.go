package runner_test

import (
	"os"
	"path/filepath"
	"testing"

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

/*
func TestSha(t *testing.T) {
	tl := testutils.NewTestLogger(t)
	absConfigDir, err := filepath.Abs(shaFixtures)
	require.NoError(t, err)
	c := runner.NewCmdConfig(tl.Logger(), absConfigDir, "base", false, "")
	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	require.NoError(t, err)
	// defer func() {
	// 	assert.NoError(t, os.RemoveAll(tmpDir))
	// }()
	require.NoError(t, c.Initialize(tmpDir))
	r := runner.NewRunner(c)
	require.NoError(t, r.Build(tmpDir))
}
*/
