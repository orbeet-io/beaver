package runner

import (
	"fmt"

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
