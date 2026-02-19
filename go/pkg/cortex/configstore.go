// Package cortex provides the unified runtime for giztoy.
//
// Architecture:
//
//   - ConfigStore: Pure file operations for ctx config (bootstrap — tells Cortex where KV is).
//   - Cortex: Opens KV from ctx config, provides Apply/Get/List/Delete with schema validation.
//   - Server: Long-running service managing device sessions.
package cortex

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// ConfigStore provides pure file-system operations for managing giztoy
// context configuration. It only handles ctx CRUD — telling Cortex where
// the KV store is. All other data (creds, genx, memory) lives in KV.
type ConfigStore struct {
	dir string
}

// OpenConfigStore opens the default configuration directory.
func OpenConfigStore() (*ConfigStore, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("cortex: cannot determine config directory: %w", err)
	}
	return OpenConfigStoreAt(filepath.Join(base, "giztoy"))
}

// OpenConfigStoreAt opens a configuration directory at the given path.
func OpenConfigStoreAt(dir string) (*ConfigStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("cortex: create config dir: %w", err)
	}
	return &ConfigStore{dir: dir}, nil
}

// Dir returns the root configuration directory path.
func (s *ConfigStore) Dir() string { return s.dir }

// ---------------------------------------------------------------------------
// Ctx CRUD
// ---------------------------------------------------------------------------

// CtxConfig holds the storage backend configuration for a context.
type CtxConfig struct {
	KV       string `yaml:"kv,omitempty" json:"kv,omitempty"`
	Storage  string `yaml:"storage,omitempty" json:"storage,omitempty"`
	VecStore string `yaml:"vecstore,omitempty" json:"vecstore,omitempty"`
	Embed    string `yaml:"embed,omitempty" json:"embed,omitempty"`
}

// CtxInfo describes a context in list output.
type CtxInfo struct {
	Name    string `json:"name"`
	Current bool   `json:"current"`
}

var validCtxConfigKeys = map[string]string{
	"kv":       "KV store (badger/redis/etcd)",
	"storage":  "File storage (local/s3/oss)",
	"vecstore": "Vector index (hnsw/milvus/qdrant)",
	"embed":    "Embedding service (dashscope/openai)",
}

// ConfigKeyInfo describes a supported config key.
type ConfigKeyInfo struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

func (s *ConfigStore) ctxDir(name string) string {
	return filepath.Join(s.dir, "contexts", name)
}

func (s *ConfigStore) ctxConfigPath(name string) string {
	return filepath.Join(s.ctxDir(name), "ctx.yaml")
}

func (s *ConfigStore) currentCtxPath() string {
	return filepath.Join(s.dir, "current-context")
}

// CtxAdd creates a new empty context.
func (s *ConfigStore) CtxAdd(name string) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("ctx add: %w", err)
	}
	dir := s.ctxDir(name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("ctx add: context %q already exists", name)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("ctx add: %w", err)
	}
	return nil
}

// CtxRemove deletes a context. It refuses to delete the current context.
func (s *ConfigStore) CtxRemove(name string) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("ctx remove: %w", err)
	}
	cur, _ := s.CtxCurrent()
	if cur == name {
		return fmt.Errorf("ctx remove: cannot remove current context %q; switch first with 'ctx use'", name)
	}
	dir := s.ctxDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("ctx remove: context %q not found", name)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("ctx remove: %w", err)
	}
	return nil
}

// CtxUse switches the current context.
func (s *ConfigStore) CtxUse(name string) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("ctx use: %w", err)
	}
	dir := s.ctxDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("ctx use: context %q not found", name)
	}
	return writeFile(s.currentCtxPath(), []byte(name+"\n"))
}

// CtxCurrent returns the name of the current context.
func (s *ConfigStore) CtxCurrent() (string, error) {
	data, err := os.ReadFile(s.currentCtxPath())
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no current context set; use 'ctx use <name>'")
		}
		return "", fmt.Errorf("ctx current: %w", err)
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "", fmt.Errorf("no current context set; use 'ctx use <name>'")
	}
	return name, nil
}

// CtxList returns all context names, sorted alphabetically.
func (s *ConfigStore) CtxList() ([]CtxInfo, error) {
	ctxsDir := filepath.Join(s.dir, "contexts")
	entries, err := os.ReadDir(ctxsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("ctx list: %w", err)
	}
	cur, _ := s.CtxCurrent()
	var infos []CtxInfo
	for _, e := range entries {
		if e.IsDir() {
			infos = append(infos, CtxInfo{
				Name:    e.Name(),
				Current: e.Name() == cur,
			})
		}
	}
	return infos, nil
}

// CtxShow returns the configuration of a context.
// If name is empty, uses the current context.
func (s *ConfigStore) CtxShow(name string) (string, *CtxConfig, error) {
	if name == "" {
		var err error
		name, err = s.CtxCurrent()
		if err != nil {
			return "", nil, err
		}
	}
	path := s.ctxConfigPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return name, &CtxConfig{}, nil
		}
		return "", nil, fmt.Errorf("ctx show: %w", err)
	}
	var cfg CtxConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", nil, fmt.Errorf("ctx show: parse %s: %w", path, err)
	}
	return name, &cfg, nil
}

// CtxConfigSet sets a config key on the current context.
func (s *ConfigStore) CtxConfigSet(key, value string) error {
	if _, ok := validCtxConfigKeys[key]; !ok {
		return fmt.Errorf("ctx config set: unknown key %q; valid keys: %s", key, ctxConfigKeyNames())
	}
	name, err := s.CtxCurrent()
	if err != nil {
		return err
	}
	_, cfg, err := s.CtxShow(name)
	if err != nil {
		return err
	}
	switch key {
	case "kv":
		cfg.KV = value
	case "storage":
		cfg.Storage = value
	case "vecstore":
		cfg.VecStore = value
	case "embed":
		cfg.Embed = value
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("ctx config set: marshal: %w", err)
	}
	return writeFile(s.ctxConfigPath(name), data)
}

// CtxConfigList returns all supported config keys with descriptions.
func (s *ConfigStore) CtxConfigList() []ConfigKeyInfo {
	keys := make([]ConfigKeyInfo, 0, len(validCtxConfigKeys))
	for k, desc := range validCtxConfigKeys {
		keys = append(keys, ConfigKeyInfo{Key: k, Description: desc})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Key < keys[j].Key })
	return keys
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("name %q must not contain path separators", name)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("name %q must not start with '.'", name)
	}
	return nil
}

func writeFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func ctxConfigKeyNames() string {
	keys := make([]string, 0, len(validCtxConfigKeys))
	for k := range validCtxConfigKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
