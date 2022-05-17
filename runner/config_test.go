package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	configDir := "fixtures/"
	config, err := NewConfig(configDir)
	require.NoError(t, err)
	// first config.spec.variables entry name should be VAULT_KV in our test file
	assert.Equal(t, "VAULT_KV", config.Spec.Variables[0].Name)
	assert.Equal(t, "orus.io", config.Spec.Variables[0].Value)
	assert.Equal(t, "vendor/helm/postgresql", config.Spec.Charts.Helm["postgres"].Path)
	assert.Equal(t, "vendor/ytt/odoo", config.Spec.Charts.Ytt["odoo"].Path)
}
