package runner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"orus.io/orus-io/beaver/runner"
	"orus.io/orus-io/beaver/testutils"
)

func TestHelmDependencyBuild(t *testing.T) {
	fixtures = "fixtures/f4"
	tl := testutils.NewTestLogger(t)

	absConfigDir, err := filepath.Abs(fixtures)
	require.NoError(t, err)

	tmpDir, err := os.MkdirTemp(os.TempDir(), "beaver-")
	require.NoError(t, err)

	c := runner.NewCmdConfig(tl.Logger(), absConfigDir, "base", false, false, "", "")
	require.NoError(t, c.Initialize(tmpDir))

	chartsPaths, err := c.HelmChartsPaths()
	require.NoError(t, err)
	require.Equal(t, 3, len(chartsPaths))
	assert.True(t, strings.HasSuffix(chartsPaths[0], "hcl3"))
	assert.True(t, strings.HasSuffix(chartsPaths[1], "hcl2"))
	assert.True(t, strings.HasSuffix(chartsPaths[2], "hcl1"))

	buildDir := filepath.Join(fixtures, "build")
	defer func() {
		require.NoError(t, runner.CleanDir(buildDir))
	}()
	r := runner.NewRunner(c)
	require.NoError(t, r.Build(tmpDir))
}
