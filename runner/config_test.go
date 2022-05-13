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
	testNS := "ns1"
	config, err := NewConfig(logger, configDir, testNS)
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
		config.Spec.Charts.Helm["postgres"].Values,
	)

	// verify ytt entries
	assert.Equal(t, "cnpp.k8s.cloudcrane.io", config.Spec.Charts.Ytt["odoo"].Values[0].Value)
	assert.Equal(t, testNS, config.Spec.Charts.Ytt["odoo"].Values[1].Value)

	// yaml support should let toto entry use DEFAULT_VALUES defined in odoo
	assert.Equal(t, "cnpp.k8s.cloudcrane.io", config.Spec.Charts.Ytt["toto"].Values[0].Value)
	assert.Equal(t, testNS, config.Spec.Charts.Ytt["toto"].Values[1].Value)
}
