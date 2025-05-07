package runner_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"orus.io/orus-io/beaver/runner"
	"orus.io/orus-io/beaver/testutils"
)

const (
	nestedVars = "fixtures/f5"
	envNS1     = "environments/ns1"
)

func TestConfigNestedVar(t *testing.T) {
	tl := testutils.NewTestLogger(t)
	absConfigDir, err := filepath.Abs(nestedVars)
	require.NoError(t, err)

	testNS := envNS1

	c := runner.NewCmdConfig(tl.Logger(), absConfigDir, testNS, false, false, "", "")

	tmpDir := t.TempDir()

	require.NoError(t, c.Initialize(tmpDir))

	v, success := c.Spec.Variables.Get("common.port")
	require.True(t, success)

	port, ok := v.(int)
	require.True(t, ok)

	assert.Equal(t, 443, port)

	r := runner.NewRunner(c)
	require.NoError(t, r.Build(tmpDir))

	buildDir := filepath.Join(nestedVars, "build", "ns1")
	outPutConfigMapName := filepath.Join(buildDir, "ConfigMap.v1.test-configmap.yaml")

	f, err := os.Open(outPutConfigMapName)
	require.NoError(t, err)

	content, err := io.ReadAll(f)
	require.NoError(t, err)

	expected := `---
apiVersion: v1
data:
    data-1: |-
        port=443
        data=value
kind: ConfigMap
metadata:
    name: test-configmap
`

	assert.Equal(t, expected, string(content))
}
