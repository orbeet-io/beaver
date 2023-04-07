package runner

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/valyala/fasttemplate"
	"gopkg.in/yaml.v3"
)

// ^<\[ starts with literal <[
// ([^<\[\]>]*) a capturing group that matches any 0 or more (due to the * quantifier)
// characters other than <, [ and ],>
// ([^...] is a negated character class matching any char but the one(s) specified between [^ and ])
// \]>$ ends with literal ]>
var beaverVariableRe = regexp.MustCompile(`^<\[([^<\[\]>]*)\]>$`)

// hydrateString will replace all instance of beaver variables in a given string
func hydrateString(input string, output io.Writer, variables map[string]interface{}) error {
	t, err := fasttemplate.NewTemplate(input, "<[", "]>")
	if err != nil {
		return fmt.Errorf("unexpected error when parsing template: %w", err)
	}
	s, err := t.ExecuteFuncStringWithErr(func(w io.Writer, tag string) (int, error) {
		val, ok := lookupVariable(variables, tag)
		if !ok {
			return 0, fmt.Errorf("tag not found: %s", tag)
		}
		switch v := val.(type) {
		case string:
			return w.Write([]byte(v))
		default:
			e := yaml.NewEncoder(w)
			err := e.Encode(val)
			return 0, err
		}
	})
	if err != nil {
		return err
	}
	if _, err := output.Write([]byte(s)); err != nil {
		return fmt.Errorf("failed to template: %w", err)
	}
	return nil
}

// hydrateScalarNode a yaml node
func hydrateScalarNode(node *yaml.Node, variables map[string]interface{}) error {
	input := node.Value

	// find all matches
	matches := beaverVariableRe.FindAllStringSubmatch(input, -1)

	if len(matches) == 1 {
		// first match, then first extracted data (in position 1)
		tag := matches[0][1]
		var ok bool
		output, ok := lookupVariable(variables, tag)
		if !ok {
			return fmt.Errorf("tag not found: %s", tag)
		}
		// preserve comments
		hc := node.HeadComment
		lc := node.LineComment
		fc := node.FootComment
		if err := node.Encode(output); err != nil {
			return err
		}
		node.HeadComment = hc
		node.LineComment = lc
		node.FootComment = fc
	} else {
		buf := bytes.NewBufferString("")
		if err := hydrateString(input, buf, variables); err != nil {
			return err
		}
		node.Value = buf.String()
	}
	return nil
}

// hydrateYamlNodes ...
func hydrateYamlNodes(nodes []*yaml.Node, variables map[string]interface{}) error {
	for _, node := range nodes {
		if node.Kind == yaml.ScalarNode {
			if err := hydrateScalarNode(node, variables); err != nil {
				fmt.Printf("node: %+v, variables: %+v\n", node, variables)
				return fmt.Errorf("failed to parse scalar: %w", err)
			}
		} else {
			if err := hydrateYamlNodes(node.Content, variables); err != nil {
				return fmt.Errorf("failed to hydrate content: %w", err)
			}
		}
	}
	return nil
}

// hydrateYaml hydrate a yaml document
func hydrateYaml(root *yaml.Node, variables map[string]interface{}) error {
	err := hydrateYamlNodes(root.Content, variables)
	return err
}

// Hydrate []byte
func Hydrate(input []byte, output io.Writer, variables map[string]interface{}) error {
	// documents := bytes.Split(input, []byte("---\n"))
	documents := documentSplitter(bytes.NewReader(input))
	// yaml lib ignore leading '---'
	// see: https://github.com/go-yaml/yaml/issues/749
	// which is an issue for ytt value files
	// this is why we loop over documents in the same file
	for i, doc := range documents {
		var node yaml.Node
		if err := yaml.Unmarshal(doc, &node); err != nil || len(node.Content) == 0 {
			// not a yaml template, fallback to raw template method
			// ...maybe a ytt header or a frontmatter
			template := string(doc)
			if err := hydrateString(template, output, variables); err != nil {
				return err
			}
		} else {
			// FIXME: do not call this method when hydrating only for sha,
			// could be quite expensive

			// yaml template method
			err := hydrateYaml(&node, variables)
			if err != nil {
				return fmt.Errorf("failed to hydrate yaml: %w", err)
			}
			o, err := yaml.Marshal(node.Content[0])
			if err != nil {
				return fmt.Errorf("failed to marshal yaml: %w", err)
			}
			_, err = output.Write(o)
			if err != nil {
				return err
			}
		}
		if i != len(documents)-1 {
			_, err := output.Write([]byte("---\n"))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// hydrate a given file
func hydrate(input string, output io.Writer, variables map[string]interface{}) error {
	byteTemplate, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", input, err)
	}
	return Hydrate(byteTemplate, output, variables)
}

// hydrateFiles in a given directory
func hydrateFiles(tmpDir string, variables map[string]interface{}, paths []string) ([]string, error) {
	var result []string
	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("hydrateFiles could not stat file or dir %s: %w", path, err)
		}
		if fileInfo.IsDir() {
			result = append(result, path)
			continue
		}

		ext := filepath.Ext(path)
		tmpFile, err := os.CreateTemp(tmpDir, fmt.Sprintf("%s-*%s", strings.TrimSuffix(filepath.Base(path), ext), ext))
		if err != nil {
			return nil, fmt.Errorf("hydrateFiles failed to create tempfile: %w", err)
		}
		defer func() {
			_ = tmpFile.Close()
		}()
		if err := hydrate(path, tmpFile, variables); err != nil {
			return nil, fmt.Errorf("failed to hydrate: %w", err)
		}
		result = append(result, tmpFile.Name())
	}
	return result, nil
}

// documentSplitter will split yaml documents
func documentSplitter(input io.Reader) [][]byte {
	var output [][]byte
	var tmpOut []byte
	scanner := bufio.NewScanner(input)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		text := scanner.Bytes()
		// if sep is found first flush our buffer
		if string(text) == "---" {
			// flush buffer
			output = append(output, tmpOut)
			// initialize new buffer
			tmpOut = []byte{}
		}
		tmpOut = append(tmpOut, text...)
		tmpOut = append(tmpOut, []byte("\n")...)
	}
	output = append(output, tmpOut)
	return output
}
