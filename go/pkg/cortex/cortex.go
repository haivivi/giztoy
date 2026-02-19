package cortex

import (
	"context"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/haivivi/giztoy/go/pkg/kv"
)

// Cortex is the unified runtime for giztoy. It opens KV from ctx config
// and provides Apply/Get/List/Delete for all resources with schema validation.
type Cortex struct {
	config   *ConfigStore
	kv       kv.Store
	schemas  *SchemaRegistry
	ownsKV   bool // true if Cortex opened the KV (should close it)
}

// Option configures Cortex creation.
type Option func(*options)

type options struct {
	kv kv.Store
}

// WithKV injects a KV store (for testing with kv.Memory).
func WithKV(store kv.Store) Option {
	return func(o *options) { o.kv = store }
}

// New creates a Cortex by reading the current ctx config and opening KV.
// Use WithKV to inject a test KV store instead of opening from ctx config.
func New(ctx context.Context, cfg *ConfigStore, opts ...Option) (*Cortex, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	kvStore := o.kv
	ownsKV := false
	if kvStore == nil {
		store, err := openKVFromConfig(cfg)
		if err != nil {
			return nil, err
		}
		kvStore = store
		ownsKV = true
	}

	return &Cortex{
		config:  cfg,
		kv:      kvStore,
		schemas: NewSchemaRegistry(),
		ownsKV:  ownsKV,
	}, nil
}

// Config returns the underlying ConfigStore.
func (c *Cortex) Config() *ConfigStore { return c.config }

// KV returns the underlying KV store.
func (c *Cortex) KV() kv.Store { return c.kv }

// Close releases all resources. If the KV was injected via WithKV,
// it is NOT closed (the caller owns it).
func (c *Cortex) Close() error {
	if c.ownsKV && c.kv != nil {
		return c.kv.Close()
	}
	return nil
}

// Apply validates and writes documents to KV. This is the single write entry
// point — all CLI commands and API handlers use this.
func (c *Cortex) Apply(ctx context.Context, docs []Document) ([]ApplyResult, error) {
	var results []ApplyResult
	for i, doc := range docs {
		result, err := c.applyOne(ctx, doc)
		if err != nil {
			return results, fmt.Errorf("document %d (kind=%s): %w", i, doc.Kind, err)
		}
		results = append(results, result)
	}
	return results, nil
}

func (c *Cortex) applyOne(ctx context.Context, doc Document) (ApplyResult, error) {
	schema := c.schemas.Get(doc.Kind)
	if schema == nil {
		return ApplyResult{}, fmt.Errorf("unknown kind %q", doc.Kind)
	}

	if err := schema.Validate(doc.Fields); err != nil {
		return ApplyResult{}, err
	}

	key := schema.Key(doc.Fields)

	// Check if already exists (for status reporting).
	_, existErr := c.kv.Get(ctx, key)
	status := "created"
	if existErr == nil {
		status = "updated"
	}

	data, err := yaml.Marshal(doc.Fields)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("marshal: %w", err)
	}

	if err := c.kv.Set(ctx, key, data); err != nil {
		return ApplyResult{}, fmt.Errorf("write KV: %w", err)
	}

	return ApplyResult{
		Kind:   doc.Kind,
		Name:   doc.Name(),
		Key:    key.String(),
		Status: status,
	}, nil
}

// Get retrieves a single document by its full KV name (e.g. "creds:openai:qwen").
func (c *Cortex) Get(ctx context.Context, fullName string) (*Document, error) {
	key := parseFullName(fullName)

	data, err := c.kv.Get(ctx, key)
	if err != nil {
		if err == kv.ErrNotFound {
			return nil, fmt.Errorf("not found: %s", fullName)
		}
		return nil, fmt.Errorf("get %s: %w", fullName, err)
	}

	var fields map[string]any
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("parse %s: %w", fullName, err)
	}

	kind := inferKind(key)
	return &Document{Kind: kind, Fields: fields}, nil
}

// List returns documents matching a prefix pattern (e.g. "creds:*", "genx:generator:*").
// The pattern must end with "*".
func (c *Cortex) List(ctx context.Context, pattern string, opts ListOpts) ([]Document, error) {
	if !strings.HasSuffix(pattern, "*") {
		return nil, fmt.Errorf("list pattern must end with '*', got %q", pattern)
	}
	prefix := strings.TrimSuffix(pattern, "*")
	prefix = strings.TrimSuffix(prefix, ":") // "creds:*" → prefix="creds"

	key := parseFullName(prefix)

	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}
	if opts.All {
		limit = -1
	}

	var docs []Document
	count := 0
	pastFrom := opts.From == ""

	for entry, err := range c.kv.List(ctx, key) {
		if err != nil {
			return nil, fmt.Errorf("list: %w", err)
		}

		entryName := entry.Key.String()

		if !pastFrom {
			if entryName == opts.From {
				pastFrom = true
			}
			continue
		}

		var fields map[string]any
		if err := yaml.Unmarshal(entry.Value, &fields); err != nil {
			continue
		}

		kind := inferKind(entry.Key)
		docs = append(docs, Document{Kind: kind, Fields: fields})
		count++

		if limit > 0 && count >= limit {
			break
		}
	}

	return docs, nil
}

// Delete removes a single document by its full KV name.
func (c *Cortex) Delete(ctx context.Context, fullName string) error {
	key := parseFullName(fullName)

	_, err := c.kv.Get(ctx, key)
	if err != nil {
		if err == kv.ErrNotFound {
			return fmt.Errorf("not found: %s", fullName)
		}
		return fmt.Errorf("delete %s: %w", fullName, err)
	}

	if err := c.kv.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete %s: %w", fullName, err)
	}
	return nil
}

// Schemas returns the schema registry.
func (c *Cortex) Schemas() *SchemaRegistry { return c.schemas }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseFullName splits "creds:openai:qwen" → kv.Key{"creds", "openai", "qwen"}
func parseFullName(name string) kv.Key {
	parts := strings.Split(name, ":")
	return kv.Key(parts)
}

// inferKind reconstructs the kind from a KV key.
// "creds:openai:qwen" → "creds/openai"
// "genx:generator:qwen/turbo" → "genx/generator"
func inferKind(key kv.Key) string {
	if len(key) < 2 {
		return strings.Join(key, "/")
	}
	switch key[0] {
	case "creds":
		return "creds/" + key[1]
	case "genx":
		return "genx/" + key[1]
	default:
		return key[0]
	}
}

// openKVFromConfig reads the current ctx config and opens the KV store.
func openKVFromConfig(cfg *ConfigStore) (kv.Store, error) {
	_, ctxCfg, err := cfg.CtxShow("")
	if err != nil {
		return nil, fmt.Errorf("open KV: %w", err)
	}
	if ctxCfg.KV == "" {
		return nil, fmt.Errorf("open KV: no 'kv' configured in current context; use 'ctx config set kv <url>'")
	}
	return openKVByURL(ctxCfg.KV)
}

// openKVByURL opens a KV store from a URL like "badger:///path" or "memory://".
func openKVByURL(url string) (kv.Store, error) {
	switch {
	case strings.HasPrefix(url, "badger://"):
		path := strings.TrimPrefix(url, "badger://")
		return kv.NewBadger(kv.BadgerOptions{
			Dir:     path,
			Options: &kv.Options{},
			Logger:  silentBadgerLogger{},
		})
	case url == "memory://":
		return kv.NewMemory(nil), nil
	default:
		return nil, fmt.Errorf("unsupported KV URL scheme: %s", url)
	}
}

type silentBadgerLogger struct{}

func (silentBadgerLogger) Errorf(string, ...any)   {}
func (silentBadgerLogger) Warningf(string, ...any) {}
func (silentBadgerLogger) Infof(string, ...any)    {}
func (silentBadgerLogger) Debugf(string, ...any)   {}
