package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestConfig(t *testing.T) {
	configDir := "fixtures/"
	config, err := NewConfig(configDir)
	require.NoError(t, err)
	// first config.spec.variables entry name should be VAULT_KV in our test file
	assert.Equal(t, "VAULT_KV", config.Spec.Variables[0].Name)

	dumped, err := yaml.Marshal(config.Spec.Charts.Helm["postgres"].Values)
	require.NoError(t, err)
	// this config entry is just read from the base file, and not yet hydrated
	assert.Equal(t, `config:
  datasource:
    password: <path:{{.VAULT_KV}}/data/{{.namespace}}/postgres#password>
  role: '{{.ROLE}}'
fullnameoverride: pg-exporter-{{.namespace}}
`,
		string(dumped))

	/*
		// verify ytt entries
		assert.Equal(t, "cnpp.k8s.cloudcrane.io", config.Spec.Charts.Ytt["odoo"].Values[0].Value)
		assert.Equal(t, testNS, config.Spec.Charts.Ytt["odoo"].Values[1].Value)

		// yaml support should let toto entry use DEFAULT_VALUES defined in odoo
		assert.Equal(t, "cnpp.k8s.cloudcrane.io", config.Spec.Charts.Ytt["toto"].Values[0].Value)
		assert.Equal(t, testNS, config.Spec.Charts.Ytt["toto"].Values[1].Value)

		// verify variables overwrite
		assert.Equal(t, "admin", config.Spec.Charts.Ytt["odoo"].Values[2].Value)
	*/
}
