package runner

import (
	"testing"
	"github.com/stretchr/testify/require"
)

type ToBoolTestCase struct {
	Input       string
	Ouput       bool
	ExpectError bool // if you expect an error put true here
}

func TestToBool(t *testing.T) {
	tCases := []ToBoolTestCase{
		{
			Input:       "1",
			Ouput:       true,
			ExpectError: false,
		},
		{
			Input:       "0",
			Ouput:       false,
			ExpectError: false,
		},
		{
			Input:       "True",
			Ouput:       true,
			ExpectError: false,
		},
		{
			Input:       "true",
			Ouput:       true,
			ExpectError: false,
		},
		{
			Input:       "False",
			Ouput:       false,
			ExpectError: false,
		},
		{
			Input:       "false",
			Ouput:       false,
			ExpectError: false,
		},
		{
			Input:       "flase",
			Ouput:       false,
			ExpectError: true,
		},
	}
	for _, tCase := range tCases {
		res, err := toBool(tCase.Input)
		if tCase.ExpectError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tCase.Ouput, res)
		}
	}
}
