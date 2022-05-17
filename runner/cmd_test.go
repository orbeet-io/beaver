package runner

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"orus.io/cloudcrane/beaver/testutils"
)

func TestRunCMD(t *testing.T) {
	err, stdout, stderr := RunCMD("echo", "p00f")
	require.NoError(t, err)
	for _, out := range stdout {
		assert.Equal(t, "p00f", out)
		fmt.Println(out)
	}
	for _, errMsg := range stderr {
		fmt.Println(errMsg)
	}
}

func TestCmdConfig(t *testing.T) {
	tl := testutils.NewTestLogger(t)
	testNS := "ns1"
	absConfigDir, err := filepath.Abs("fixtures/")
	require.NoError(t, err)
	c := NewCmdConfig(tl.Logger(), absConfigDir, testNS)
	require.NoError(t, c.Initialize())

	t.Run("helmCharts", func(t *testing.T) {
		pgHelmChart, ok := c.Spec.Charts["postgres"]
		require.True(t, ok, "we should have a postgres helm chart in our cmdConfig")

		require.Equal(t, 2, len(pgHelmChart.ValuesFileNames))
		file1Content, err := ioutil.ReadFile(pgHelmChart.ValuesFileNames[0])
		require.NoError(t, err)

		assert.Equal(t, `persistence:
  storageClass: huawei-iscsi

initdbScripts:
  create.sql: |
    CREATE EXTENSION IF NOT EXISTS unaccent;

postgresqlUsername: "<path:k8s.orus.io/data/ns1/postgres#username>"
postgresqlDatabase: "<path:k8s.orus.io/data/ns1/postgres#database>"
`, string(file1Content))

		file2Content, err := ioutil.ReadFile(pgHelmChart.ValuesFileNames[1])
		require.NoError(t, err)
		assert.Equal(t, `image:
  tag: 14
`, string(file2Content))
	})

	t.Run("yttCharts", func(t *testing.T) {
		odooYttChart, ok := c.Spec.Charts["odoo"]
		require.True(t, ok, "we should have an odoo ytt chart in our cmdConfig")
		assert.Equal(t, 2, len(odooYttChart.ValuesFileNames))
	})

	t.Run("yttPatches", func(t *testing.T) {
		yttPatches := c.Spec.Ytt
		l := tl.Logger()
		logger := &l
		logger.Debug().Str("patches", fmt.Sprintf("%+v", yttPatches)).Msg("found patches")
		require.Equal(t, 4, len(yttPatches))
	})
}

func TestFindFiles(t *testing.T) {
	rootdir := "fixtures/"
	namespace := "ns1"

	charts := map[string]CmdChart{
		"postgres": CmdChart{
			Path:            "postgres",
			ValuesFileNames: nil,
		},
	}

	newCharts := findFiles(rootdir, namespace, charts)
	require.Equal(t, 2, len(newCharts["postgres"].ValuesFileNames))

}
