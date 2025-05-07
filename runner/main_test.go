package runner_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"orus.io/orus-io/beaver/runner"
)

type ToBoolTestCase struct {
	Input       string
	Output      bool
	ExpectError bool // if you expect an error put true here
}

func TestToBool(t *testing.T) {
	tCases := []ToBoolTestCase{
		{
			Input:       "1",
			Output:      true,
			ExpectError: false,
		},
		{
			Input:       "0",
			Output:      false,
			ExpectError: false,
		},
		{
			Input:       "True",
			Output:      true,
			ExpectError: false,
		},
		{
			Input:       "true",
			Output:      true,
			ExpectError: false,
		},
		{
			Input:       "False",
			Output:      false,
			ExpectError: false,
		},
		{
			Input:       "false",
			Output:      false,
			ExpectError: false,
		},
		{
			Input:       "flase",
			Output:      false,
			ExpectError: true,
		},
	}
	for _, tCase := range tCases {
		res, err := runner.ToBool(tCase.Input)
		if tCase.ExpectError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tCase.Output, res)
		}
	}
}
