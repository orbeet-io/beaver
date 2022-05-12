package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"orus.io/cloudcrane/beaver/testutils"
)

func TestConfig(t *testing.T) {
	logger := testutils.GetLogger(t)
	configDir := "fixtures/"
	config, err := NewConfig(logger, configDir, "ns1")
	require.NoError(t, err)
	// first config.spec.variables entry name should be VAULT_KV in our test file
	assert.Equal(t, "VAULT_KV", config.Spec.Variables[0].Name)
	// the postgres chart should have been expanded with our variables
	assert.Equal(
		t,
		`config:
  datasource:
    password: <path:cnpp.k8s.cloudcrane.io/data/ns1/postgres#password>
fullnameoverride: pg-exporter-ns1
`,
		config.Spec.Charts["postgres"].Values,
	)
}
