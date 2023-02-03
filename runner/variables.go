package runner

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Variable ...
type Variable struct {
	Name  string
	Value interface{}
}

type Variables []Variable

func (v *Variables) UnmarshalYAML(node *yaml.Node) error {
	if err := node.Decode((*[]Variable)(v)); err == nil {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expects a mapping, got: %d", node.Kind)
	}
	*v = make(Variables, 0, len(node.Content)/2)
	var next Variable
	for i, content := range node.Content {
		if i%2 == 0 {
			content.Decode(&next.Name)
		} else {
			content.Decode(&next.Value)
			*v = append(*v, next)
		}
	}
	return nil
}

func lookupVariable(variables map[string]interface{}, name string) (interface{}, bool) {
	path := strings.Split(name, ".")

	var v interface{} = variables
	var ok bool
	for _, key := range path {
		v, ok = lookupVariableHelper(v, key)
		if !ok {
			return nil, false
		}
	}
	return v, true
}

func lookupVariableHelper(v interface{}, key string) (interface{}, bool) {
	switch t := v.(type) {
	case map[string]interface{}:
		ret, ok := t[key]
		return ret, ok
	case map[interface{}]interface{}:
		ret, ok := t[key]
		return ret, ok
	case []interface{}:
		index, err := strconv.Atoi(key)
		if err != nil {
			return nil, false
		}
		if index >= len(t) || index < 0 {
			return nil, false
		}
		return t[index], true
	default:
		return nil, false
	}
}
