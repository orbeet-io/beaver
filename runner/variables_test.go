package runner_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"orus.io/orus-io/beaver/runner"
)

func TestVariablesUnmarshal(t *testing.T) {
	for _, tt := range []struct {
		name     string
		yaml     string
		expected runner.Variables
		err      string
	}{
		{
			"legacy",
			`- name: v1
  value: value1
- name: v2
  value: 54.3
- name: nested
  value:
  - attr1: attr1value
    attr2: 2
  - attr1: otherattr1value
    attr2: 4
`,
			runner.Variables{
				{Name: "v1", Value: "value1"},
				{Name: "v2", Value: 54.3},
				{Name: "nested", Value: []interface{}{
					map[string]interface{}{
						"attr1": "attr1value",
						"attr2": 2,
					},
					map[string]interface{}{
						"attr1": "otherattr1value",
						"attr2": 4,
					},
				}},
			},
			"",
		},
		{
			"dict",
			`v1: value1
v2: 54.3
nested:
  - attr1: attr1value
    attr2: 2
  - attr1: otherattr1value
    attr2: 4
`,
			runner.Variables{
				{Name: "v1", Value: "value1"},
				{Name: "v2", Value: 54.3},
				{Name: "nested", Value: []interface{}{
					map[string]interface{}{
						"attr1": "attr1value",
						"attr2": 2,
					},
					map[string]interface{}{
						"attr1": "otherattr1value",
						"attr2": 4,
					},
				}},
			},
			"",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var actual runner.Variables

			err := yaml.Unmarshal([]byte(tt.yaml), &actual)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, actual)
			}
		})
	}
}

func TestLookupVariable(t *testing.T) {
	variables := map[string]interface{}{
		"string": "a string",
		"int":    3,
		"map": map[interface{}]interface{}{
			"float": 12.3,
		},
		"list": []interface{}{
			map[interface{}]interface{}{
				"float": 12.3,
			},
		},
	}

	for _, tt := range []struct {
		name     string
		expected interface{}
	}{
		{"string", "a string"},
		{"int", 3},
		{"map.float", 12.3},
		{"list.0.float", 12.3},
		{"list.2.float", nil},
		{"list.-2.float", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			actual, ok := runner.LookupVariable(variables, tt.name)
			if tt.expected == nil {
				assert.False(t, ok)
			} else {
				assert.True(t, ok)
				assert.Equal(t, tt.expected, actual)
			}
		})
	}
}

func TestSetVariable(t *testing.T) {
	type V = map[string]interface{}

	variables := func(setters ...func(V)) V {
		ret := V{
			"string": "a string",
			"int":    3,
			"map": map[interface{}]interface{}{
				"float": 12.3,
			},
			"list": []interface{}{
				map[interface{}]interface{}{
					"float": 12.3,
				},
			},
		}
		for _, s := range setters {
			s(ret)
		}

		return ret
	}
	for _, tt := range []struct {
		name     string
		value    interface{}
		expected interface{}
	}{
		{"string", "new string", variables(func(v V) { v["string"] = "new string" })},
		{"int", "not an int anymore", variables(func(v V) { v["int"] = "not an int anymore" })},
		{
			"map.float",
			13.0,
			variables(
				func(v V) {
					v["map"].(map[interface{}]interface{})["float"] = 13.0 //nolint:forcetypeassert
				},
			),
		},
		{
			"list.0.float",
			15.0,
			variables(
				func(v V) {
					v["list"].([]interface{})[0].(map[interface{}]interface{})["float"] = 15.0 //nolint:forcetypeassert
				},
			),
		},
		{"list.2.float", nil, variables()},
		{"list.-2.float", nil, variables()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := variables()
			runner.SetVariable(v, strings.Split(tt.name, "."), tt.value)
			assert.Equal(t, tt.expected, v)
		})
	}
}
