package config

import (
	"bytes"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// Encode writes cfg into a copy of doc's syntax tree, changing only differing values so comments survive.
func Encode(doc Document, cfg Config) ([]byte, error) {
	var fresh yaml.Node
	if err := fresh.Encode(cfg); err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	root := copyNode(doc.Node, map[*yaml.Node]*yaml.Node{})
	if root == nil || len(root.Content) == 0 {
		head := ""
		if root != nil {
			head = root.HeadComment
		}
		root = &yaml.Node{Kind: yaml.DocumentNode, HeadComment: head, Content: []*yaml.Node{&fresh}}
	} else if top := root.Content[0]; top.Kind == yaml.MappingNode {
		merge(top, &fresh)
	} else {
		root.Content[0] = &fresh
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return buf.Bytes(), nil
}

func copyNode(n *yaml.Node, seen map[*yaml.Node]*yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	if c, ok := seen[n]; ok {
		return c
	}
	c := *n
	seen[n] = &c
	c.Content = make([]*yaml.Node, len(n.Content))
	for i, child := range n.Content {
		c.Content[i] = copyNode(child, seen)
	}
	c.Alias = copyNode(n.Alias, seen)
	return &c
}

func merge(old, fresh *yaml.Node) {
	if equalNodes(old, fresh) {
		return
	}
	switch {
	case old.Kind == yaml.MappingNode && fresh.Kind == yaml.MappingNode:
		for i := 0; i+1 < len(fresh.Content); i += 2 {
			key, value := fresh.Content[i], fresh.Content[i+1]
			if existing := lookup(old, key.Value); existing != nil {
				merge(existing, value)
				continue
			}
			old.Content = append(old.Content, key, value)
		}
		kept := old.Content[:0]
		for i := 0; i+1 < len(old.Content); i += 2 {
			if lookup(fresh, old.Content[i].Value) != nil {
				kept = append(kept, old.Content[i], old.Content[i+1])
			}
		}
		old.Content = kept
	case old.Kind == yaml.ScalarNode && fresh.Kind == yaml.ScalarNode:
		old.Value = fresh.Value
		old.Tag = fresh.Tag
		if old.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
			old.Style = fresh.Style
		}
	default:
		replace(old, fresh)
	}
}

func replace(old, fresh *yaml.Node) {
	head, line, foot := old.HeadComment, old.LineComment, old.FootComment
	*old = *fresh
	old.HeadComment, old.LineComment, old.FootComment = head, line, foot
}

func lookup(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func equalNodes(a, b *yaml.Node) bool {
	for a.Kind == yaml.AliasNode {
		a = a.Alias
	}
	for b.Kind == yaml.AliasNode {
		b = b.Alias
	}
	if a.Kind != b.Kind || len(a.Content) != len(b.Content) {
		return false
	}
	if a.Kind == yaml.ScalarNode && (a.Value != b.Value || a.ShortTag() != b.ShortTag()) {
		return false
	}
	for i := range a.Content {
		if !equalNodes(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}
