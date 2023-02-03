package runner

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestVariablesUnmarshal(t *testing.T) {
	for _, tt := range []struct {
		name     string
		yaml     string
		expected Variables
		err      string
	}{
		{"legacy", `- name: v1
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
			Variables{
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
		{"dict", `v1: value1
v2: 54.3
nested:
  - attr1: attr1value
    attr2: 2
  - attr1: otherattr1value
    attr2: 4
`,
			Variables{
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
			var actual Variables
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
