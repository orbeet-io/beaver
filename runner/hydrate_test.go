package runner_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"orus.io/orus-io/beaver/runner"
)

type hydrateTestCase struct {
	Name           string
	InputYaml      string
	InputVars      map[string]interface{}
	Success        bool
	ExpectedResult string
}

func TestHydrateScalarNode(t *testing.T) {
	testCases := []hydrateTestCase{
		{
			Name:           "simpleValue",
			InputYaml:      "<[namespace]>",
			InputVars:      map[string]interface{}{"namespace": "ns1"},
			Success:        true,
			ExpectedResult: "ns1",
		},
		{
			Name:           "two-on-one-line",
			InputYaml:      "<[beaver_image]>:<[beaver_tag]>",
			InputVars:      map[string]interface{}{"beaver_image": "img1", "beaver_tag": "3.1.4"},
			Success:        true,
			ExpectedResult: "img1:3.1.4",
		},
		{
			Name:           "two-on-one-line-with-int",
			InputYaml:      "<[host]>:<[port]>",
			InputVars:      map[string]interface{}{"host": "toot", "port": 443},
			Success:        true,
			ExpectedResult: "toot:443",
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.Name, func(t *testing.T) {
			var node yaml.Node

			require.NoError(t, yaml.Unmarshal([]byte(tcase.InputYaml), &node))

			// hydrate must work
			require.NoError(t, runner.HydrateScalarNode(node.Content[0], tcase.InputVars))

			assert.Equal(t, tcase.ExpectedResult, node.Content[0].Value)
		})
	}
}

func TestHydrateString(t *testing.T) {
	testCases := []hydrateTestCase{
		{
			Name:           "two-on-one-line-with-int",
			InputYaml:      "<[host]>:<[port]>",
			InputVars:      map[string]interface{}{"host": "toot", "port": 443},
			Success:        true,
			ExpectedResult: "toot:443",
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.Name, func(t *testing.T) {
			b := []byte{}
			buf := bytes.NewBuffer(b)
			// hydrate must work
			require.NoError(t, runner.HydrateString(tcase.InputYaml, buf, tcase.InputVars))

			assert.Equal(t, tcase.ExpectedResult, buf.String())
		})
	}
}
