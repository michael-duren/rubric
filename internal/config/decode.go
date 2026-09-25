package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"

	"go.yaml.in/yaml/v3"
)

var itemFields = map[kind][]string{
	kindEntryPoints: {"name", "dir"},
	kindCommands:    {"name", "dir", "argv", "env"},
	kindEvidence:    {"field", "value", "source"},
}

// Decode parses exactly one YAML document strictly against the schema, keeping its syntax tree and explicit values.
func Decode(data []byte) (Document, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var root yaml.Node
	if err := dec.Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			return Document{Values: Patch{}}, nil
		}
		return Document{}, fmt.Errorf("invalid yaml: %w", err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return Document{}, fmt.Errorf("invalid yaml: %w", err)
		}
		return Document{}, fmt.Errorf("line %d: expected a single YAML document", extra.Line)
	}
	values := Patch{}
	if len(root.Content) == 0 {
		return Document{Node: &root, Values: values}, nil
	}
	w := walker{visiting: map[*yaml.Node]bool{}}
	top := root.Content[0]
	err := w.deref(top, func(n *yaml.Node) error {
		if n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null" {
			return nil
		}
		return w.section(n, "", values)
	})
	if err != nil {
		return Document{}, err
	}
	return Document{Node: &root, Values: values}, nil
}

type walker struct {
	visiting map[*yaml.Node]bool
}

func (w walker) deref(n *yaml.Node, fn func(*yaml.Node) error) error {
	target := n
	if n.Kind == yaml.AliasNode {
		target = n.Alias
	}
	if w.visiting[target] {
		return fmt.Errorf("line %d: alias *%s forms a cycle", n.Line, n.Value)
	}
	w.visiting[target] = true
	defer delete(w.visiting, target)
	return fn(target)
}

func (w walker) section(n *yaml.Node, prefix string, values Patch) error {
	name := prefix
	if name == "" {
		name = "document"
	}
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: %s: expected a mapping", n.Line, name)
	}
	return w.pairs(n, name, func(key string, value *yaml.Node) error {
		path := prefix + key
		f, ok := fields[path]
		if !ok {
			return fmt.Errorf("line %d: %s: unknown key", value.Line, path)
		}
		return w.deref(value, func(v *yaml.Node) error {
			if f.kind == kindSection {
				return w.section(v, path+".", values)
			}
			decoded, err := w.value(v, path, f.kind)
			if err != nil {
				return err
			}
			values[path] = decoded
			return nil
		})
	})
}

func (w walker) pairs(n *yaml.Node, name string, fn func(string, *yaml.Node) error) error {
	seen := map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		keyNode := n.Content[i]
		var key string
		err := w.deref(keyNode, func(k *yaml.Node) error {
			if k.Kind != yaml.ScalarNode || k.ShortTag() != "!!str" {
				return fmt.Errorf("line %d: %s: keys must be strings", k.Line, name)
			}
			key = k.Value
			return nil
		})
		if err != nil {
			return err
		}
		if seen[key] {
			return fmt.Errorf("line %d: %s: duplicate key %q", keyNode.Line, name, key)
		}
		seen[key] = true
		if err := fn(key, n.Content[i+1]); err != nil {
			return err
		}
	}
	return nil
}

func (w walker) value(n *yaml.Node, path string, k kind) (any, error) {
	switch k {
	case kindString:
		return scalar(n, path, "!!str", "string", func(s string) (string, error) { return s, nil })
	case kindBool:
		return scalar(n, path, "!!bool", "boolean", strconv.ParseBool)
	case kindInt:
		return scalar(n, path, "!!int", "integer", strconv.Atoi)
	case kindStringMap:
		return w.stringMap(n, path)
	case kindSkills:
		if n.Kind == yaml.ScalarNode {
			return scalar(n, path, "!!bool", "list of skill groups", strconv.ParseBool)
		}
		return w.stringList(n, path)
	case kindEntryPoints:
		return decodeItems(w, n, path, k, func(m map[string]any) EntryPoint {
			return EntryPoint{Name: str(m["name"]), Dir: str(m["dir"])}
		})
	case kindCommands:
		return decodeItems(w, n, path, k, func(m map[string]any) Command {
			return Command{Name: str(m["name"]), Dir: str(m["dir"]), Argv: strs(m["argv"]), Env: strs(m["env"])}
		})
	case kindEvidence:
		return decodeItems(w, n, path, k, func(m map[string]any) Evidence {
			return Evidence{Field: str(m["field"]), Value: str(m["value"]), Source: str(m["source"])}
		})
	}
	return nil, fmt.Errorf("%s: unsupported schema kind", path)
}

func scalar[T any](n *yaml.Node, path, tag, name string, parse func(string) (T, error)) (T, error) {
	var zero T
	if n.Kind != yaml.ScalarNode || n.ShortTag() != tag {
		return zero, fmt.Errorf("line %d: %s: expected %s", n.Line, path, name)
	}
	v, err := parse(n.Value)
	if err != nil {
		return zero, fmt.Errorf("line %d: %s: expected %s", n.Line, path, name)
	}
	return v, nil
}

func (w walker) stringMap(n *yaml.Node, path string) (map[string]string, error) {
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: %s: expected a mapping", n.Line, path)
	}
	out := map[string]string{}
	err := w.pairs(n, path, func(key string, value *yaml.Node) error {
		return w.deref(value, func(v *yaml.Node) error {
			s, err := scalar(v, path+"."+key, "!!str", "string", func(s string) (string, error) { return s, nil })
			out[key] = s
			return err
		})
	})
	return out, err
}

func (w walker) stringList(n *yaml.Node, path string) ([]string, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("line %d: %s: expected a list", n.Line, path)
	}
	out := make([]string, 0, len(n.Content))
	for _, item := range n.Content {
		err := w.deref(item, func(v *yaml.Node) error {
			s, err := scalar(v, path, "!!str", "string", func(s string) (string, error) { return s, nil })
			out = append(out, s)
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func decodeItems[T any](w walker, n *yaml.Node, path string, k kind, build func(map[string]any) T) ([]T, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("line %d: %s: expected a list", n.Line, path)
	}
	allowed := map[string]bool{}
	for _, name := range itemFields[k] {
		allowed[name] = true
	}
	out := make([]T, 0, len(n.Content))
	for i, item := range n.Content {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		m := map[string]any{}
		err := w.deref(item, func(v *yaml.Node) error {
			if v.Kind != yaml.MappingNode {
				return fmt.Errorf("line %d: %s: expected a mapping", v.Line, itemPath)
			}
			return w.pairs(v, itemPath, func(key string, value *yaml.Node) error {
				if !allowed[key] {
					return fmt.Errorf("line %d: %s.%s: unknown key", value.Line, itemPath, key)
				}
				return w.deref(value, func(fv *yaml.Node) error {
					var err error
					if key == "argv" || key == "env" {
						m[key], err = w.stringList(fv, itemPath+"."+key)
					} else {
						m[key], err = scalar(fv, itemPath+"."+key, "!!str", "string", func(s string) (string, error) { return s, nil })
					}
					return err
				})
			})
		})
		if err != nil {
			return nil, err
		}
		out = append(out, build(m))
	}
	return out, nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func strs(v any) []string {
	s, _ := v.([]string)
	if s == nil {
		return []string{}
	}
	return s
}
