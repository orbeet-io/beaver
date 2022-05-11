package runner

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
