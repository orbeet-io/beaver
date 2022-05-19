package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"orus.io/cloudcrane/beaver/testutils"
)

func TestConfig(t *testing.T) {
	configDir := "fixtures/"
	config, err := NewConfig(configDir)
	require.NoError(t, err)
	// first config.spec.variables entry name should be VAULT_KV in our test file
	assert.Equal(t, "VAULT_KV", config.Spec.Variables[0].Name)
	assert.Equal(t, "orus.io", config.Spec.Variables[0].Value)
	assert.Equal(t, "vendor/helm/postgresql", config.Spec.Charts["postgres"].Path)
	assert.Equal(t, "vendor/ytt/odoo", config.Spec.Charts["odoo"].Path)
}

func TestYttBuildArgs(t *testing.T) {
	tl := testutils.NewTestLogger(t)
	testNS := "ns1"
	absConfigDir, err := filepath.Abs("fixtures/")
	require.NoError(t, err)
	c := NewCmdConfig(tl.Logger(), absConfigDir, testNS, false)
	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}()
	require.NoError(t, c.Initialize(tmpDir))

	args := c.Spec.Ytt.BuildArgs(testNS, []string{"/tmp/postgres.1234.yaml", "/tmp/odoo.5678.yaml"})
	assert.Equal(
		t,
		args,
		[]string{
			"-f", "/tmp/postgres.1234.yaml", "--file-mark", "postgres.1234.yaml",
			"-f", "/tmp/odoo.5678.yaml", "--file-mark", "odoo.5678.yaml",
			"-f", "base/ytt",
			"-f", "base/ytt.yaml",
			"-f", "environments/ns1/ytt",
			"-f", "environments/ns1/ytt.yaml",
		},
	)
}
