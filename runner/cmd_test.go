package runner

import (
	"fmt"
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
	c, err := NewCmdConfig(tl.Logger(), "fixtures/", testNS)
	require.NoError(t, err)

	pgHelmChart, ok := c.Spec.Charts.Helm["postgres"]
	require.True(t, ok, "we should have a postgres helm chart in our cmdConfig")

	require.NotEqual(t, 0, len(pgHelmChart.Files))

	/*
			assert.Equal(t, []string{
				`config:
		  datasource:
		    password: <path:cnpp.k8s.cloudcrane.io/data/ns1/postgres#password>
		  role: 'admin'
		fullnameoverride: pg-exporter-ns1
		`},
				c.Spec.Charts.Helm["postgres"].Files[0],
			)
	*/
}
