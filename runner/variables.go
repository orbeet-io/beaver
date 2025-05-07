package runner

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Variable ...
type Variable struct {
	Name  string      `yaml:"name"`
	Value interface{} `yaml:"value"`
}

type Variables []Variable

func (v *Variables) Get(path string) (interface{}, bool) {
	sp := strings.Split(path, ".")
	head := sp[0]
	tail := sp[1:]

	for _, variable := range *v {
		if variable.Name == head {
			if len(tail) == 0 {
				return variable.Value, true
			}

			return LookupVariable(variable.Value, strings.Join(tail, "."))
		}
	}

	return nil, false
}

func (v *Variables) GetD(path string, defaultValue interface{}) interface{} {
	ret, ok := v.Get(path)
	if ok {
		return ret
	}

	return defaultValue
}

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
			if err := content.Decode(&next.Name); err != nil {
				return err
			}
		} else {
			if err := content.Decode(&next.Value); err != nil {
				return err
			}

			*v = append(*v, next)
		}
	}

	return nil
}

func (v *Variables) Overlay(variables ...Variable) {
	newVariables := Variables{}

	for _, inputVar := range variables {
		path := strings.Split(inputVar.Name, ".")
		head := path[0]
		tail := path[1:]

		for i := range *v {
			if (*v)[i].Name == head {
				if len(tail) == 0 {
					(*v)[i].Value = inputVar.Value
				} else {
					SetVariable((*v)[i].Value, tail, inputVar.Value)
				}

				continue
			}
		}

		newVariables = append(newVariables, inputVar)
	}

	*v = append(*v, newVariables...)
}

func SetVariable(v interface{}, path []string, value interface{}) {
	head := path[0]
	tail := path[1:]
	hasTail := len(tail) != 0

	switch t := v.(type) {
	case map[string]interface{}:
		if hasTail {
			nextValue, ok := t[head]
			if !ok {
				return
			}

			SetVariable(nextValue, tail, value)
		} else {
			t[head] = value
		}

	case map[interface{}]interface{}:
		if hasTail {
			nextValue, ok := t[head]
			if !ok {
				return
			}

			SetVariable(nextValue, tail, value)
		} else {
			t[head] = value
		}

	case []interface{}:
		index, err := strconv.Atoi(head)
		if err != nil {
			return
		}

		if index >= len(t) || index < 0 {
			return
		}

		if hasTail {
			SetVariable(t[index], tail, value)
		} else {
			t[index] = value
		}

	default:
	}
}

func LookupVariable(variables interface{}, name string) (interface{}, bool) {
	v := variables

	path := strings.Split(name, ".")

	for _, key := range path {
		var ok bool

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
