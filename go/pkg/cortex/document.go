package cortex

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/goccy/go-yaml"
)

// Document is the universal resource representation. Every apply/get/list/delete
// operation works with Documents. The Kind field determines the schema used for
// validation and the KV key layout.
type Document struct {
	Kind   string         `yaml:"kind" json:"kind"`
	Fields map[string]any `yaml:",inline" json:",inline"`
}

// Name returns the "name" field from the document, or empty string.
func (d *Document) Name() string {
	if v, ok := d.Fields["name"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// FullName returns the KV-style full name: "kind:...additional segments".
// The exact format depends on the kind's schema KeyFunc.
// This is a display helper; actual KV keys are computed by Schema.Key().
func (d *Document) FullName() string {
	return d.Kind + ":" + d.Name()
}

// GetString returns a string field or empty string.
func (d *Document) GetString(key string) string {
	if v, ok := d.Fields[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetFloat returns a float64 field or 0.
func (d *Document) GetFloat(key string) float64 {
	if v, ok := d.Fields[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		case int64:
			return float64(n)
		}
	}
	return 0
}

// GetInt returns an int field or 0.
func (d *Document) GetInt(key string) int {
	return toInt(d.Fields[key])
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// ApplyResult describes the outcome of applying one document.
type ApplyResult struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Key     string `json:"key"`
	Status  string `json:"status"` // "created", "updated"
	Message string `json:"message,omitempty"`
}

// ListOpts controls list pagination.
type ListOpts struct {
	Limit int    // max items (default 10)
	From  string // start after this key
	All   bool   // ignore limit, return all
}

// ParseDocuments parses a multi-document YAML stream (--- separated).
func ParseDocuments(data []byte) ([]Document, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var docs []Document
	for {
		var raw map[string]any
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("parse YAML: %w", err)
		}
		if raw == nil {
			continue
		}
		kind, _ := raw["kind"].(string)
		if kind == "" {
			return nil, fmt.Errorf("document missing 'kind' field")
		}
		delete(raw, "kind")
		docs = append(docs, Document{Kind: kind, Fields: raw})
	}
	return docs, nil
}

// ParseDocumentsFromFile reads and parses a YAML file. Use "-" for stdin.
func ParseDocumentsFromFile(path string) ([]Document, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseDocuments(data)
}
