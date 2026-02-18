// Package cortex provides the unified runtime for giztoy.
//
// Architecture:
//
//   - ConfigStore (Layer 1): Pure file operations for ctx/app/genx config CRUD.
//   - Cortex (Layer 2): Initialized runtime with storage backends and sub-system *Tex accessors.
//   - Server (Layer 3): Long-running service managing device sessions (Atom).
package cortex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// ConfigStore provides pure file-system operations for managing giztoy
// configuration: contexts, service apps, and genx model configs.
//
// It does not initialize any storage backends or network connections.
// All methods are safe for concurrent use from multiple processes
// (they operate on independent files with atomic writes).
type ConfigStore struct {
	dir string
}

// OpenConfigStore opens the default configuration directory.
//
// Layout:
//
//	~/.config/giztoy/              (Linux)
//	~/Library/Application Support/giztoy/  (macOS)
//	%AppData%/giztoy/              (Windows)
func OpenConfigStore() (*ConfigStore, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("cortex: cannot determine config directory: %w", err)
	}
	return OpenConfigStoreAt(filepath.Join(base, "giztoy"))
}

// OpenConfigStoreAt opens a configuration directory at the given path.
// The directory is created if it does not exist.
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
	Name    string
	Current bool
}

var validCtxConfigKeys = map[string]string{
	"kv":       "KV store (badger/redis/etcd)",
	"storage":  "File storage (local/s3/oss)",
	"vecstore": "Vector index (hnsw/milvus/qdrant)",
	"embed":    "Embedding service (dashscope/openai)",
}

// ConfigKeyInfo describes a supported config key.
type ConfigKeyInfo struct {
	Key         string
	Description string
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
// Returns an empty string and an error if no context is set.
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
// App CRUD (shared by minimax, doubaospeech, dashscope)
// ---------------------------------------------------------------------------

// AppInfo describes an app in list output.
type AppInfo struct {
	Name    string
	Current bool
}

func (s *ConfigStore) serviceDir(service string) (string, error) {
	name, err := s.CtxCurrent()
	if err != nil {
		return "", err
	}
	return filepath.Join(s.ctxDir(name), service), nil
}

func (s *ConfigStore) appPath(service, appName string) (string, error) {
	svcDir, err := s.serviceDir(service)
	if err != nil {
		return "", err
	}
	return filepath.Join(svcDir, appName+".yaml"), nil
}

func (s *ConfigStore) currentAppPath(service string) (string, error) {
	svcDir, err := s.serviceDir(service)
	if err != nil {
		return "", err
	}
	return filepath.Join(svcDir, "current-app"), nil
}

// AppAdd adds a named app configuration for a service.
func (s *ConfigStore) AppAdd(service, name string, cfg map[string]any) error {
	if err := validateName(service); err != nil {
		return fmt.Errorf("app add: invalid service: %w", err)
	}
	if err := validateName(name); err != nil {
		return fmt.Errorf("app add: invalid name: %w", err)
	}
	svcDir, err := s.serviceDir(service)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(svcDir, 0755); err != nil {
		return fmt.Errorf("app add: %w", err)
	}
	path := filepath.Join(svcDir, name+".yaml")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("app add: app %q already exists for service %q", name, service)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("app add: marshal: %w", err)
	}
	return writeFile(path, data)
}

// AppRemove removes an app configuration.
func (s *ConfigStore) AppRemove(service, name string) error {
	path, err := s.appPath(service, name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("app remove: app %q not found for service %q", name, service)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("app remove: %w", err)
	}
	// Clear current-app if it was the removed one.
	cur, _ := s.AppCurrent(service)
	if cur == name {
		capPath, _ := s.currentAppPath(service)
		os.Remove(capPath)
	}
	return nil
}

// AppUse switches the current app for a service.
func (s *ConfigStore) AppUse(service, name string) error {
	path, err := s.appPath(service, name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("app use: app %q not found for service %q", name, service)
	}
	capPath, err := s.currentAppPath(service)
	if err != nil {
		return err
	}
	return writeFile(capPath, []byte(name+"\n"))
}

// AppCurrent returns the current app name for a service.
func (s *ConfigStore) AppCurrent(service string) (string, error) {
	capPath, err := s.currentAppPath(service)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(capPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no current app set for %s; use '%s app use <name>'", service, service)
		}
		return "", fmt.Errorf("app current: %w", err)
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "", fmt.Errorf("no current app set for %s; use '%s app use <name>'", service, service)
	}
	return name, nil
}

// AppList returns all app names for a service in the current context.
func (s *ConfigStore) AppList(service string) ([]AppInfo, error) {
	svcDir, err := s.serviceDir(service)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(svcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("app list: %w", err)
	}
	cur, _ := s.AppCurrent(service)
	var infos []AppInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		base := name[:len(name)-len(ext)]
		if base == "current-app" {
			continue
		}
		infos = append(infos, AppInfo{
			Name:    base,
			Current: base == cur,
		})
	}
	return infos, nil
}

// AppShow returns the raw config for an app.
func (s *ConfigStore) AppShow(service, name string) (map[string]any, error) {
	path, err := s.appPath(service, name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("app show: app %q not found for service %q", name, service)
		}
		return nil, fmt.Errorf("app show: %w", err)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("app show: parse: %w", err)
	}
	return cfg, nil
}

// AppLoad loads an app config into a typed struct.
func AppLoad[T any](s *ConfigStore, service, name string) (*T, error) {
	path, err := s.appPath(service, name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("app load: app %q not found for service %q", name, service)
		}
		return nil, fmt.Errorf("app load: %w", err)
	}
	var cfg T
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("app load: parse: %w", err)
	}
	return &cfg, nil
}

// ---------------------------------------------------------------------------
// GenX CRUD
// ---------------------------------------------------------------------------

// GenXInfo describes a genx config entry.
type GenXInfo struct {
	Pattern string
	Type    string
	Schema  string
	File    string
}

func (s *ConfigStore) genxDir() (string, error) {
	name, err := s.CtxCurrent()
	if err != nil {
		return "", err
	}
	return filepath.Join(s.ctxDir(name), "genx"), nil
}

// GenXAdd copies a config file into the current context's genx/ directory.
func (s *ConfigStore) GenXAdd(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("genx add: %w", err)
	}
	gDir, err := s.genxDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(gDir, 0755); err != nil {
		return fmt.Errorf("genx add: %w", err)
	}
	dest := filepath.Join(gDir, filepath.Base(configPath))
	return writeFile(dest, data)
}

// GenXRemove removes a genx config file containing the given pattern.
func (s *ConfigStore) GenXRemove(pattern string) error {
	gDir, err := s.genxDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(gDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("genx remove: pattern %q not found", pattern)
		}
		return fmt.Errorf("genx remove: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(gDir, e.Name())
		infos, err := parseGenXFile(path)
		if err != nil {
			continue
		}
		for _, info := range infos {
			if info.Pattern == pattern {
				return os.Remove(path)
			}
		}
	}
	return fmt.Errorf("genx remove: pattern %q not found", pattern)
}

// GenXList lists all genx config entries in the current context.
func (s *ConfigStore) GenXList(filterType string) ([]GenXInfo, error) {
	gDir, err := s.genxDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(gDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("genx list: %w", err)
	}
	var all []GenXInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(gDir, e.Name())
		infos, err := parseGenXFile(path)
		if err != nil {
			continue
		}
		for _, info := range infos {
			if filterType == "" || info.Type == filterType {
				all = append(all, info)
			}
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Type != all[j].Type {
			return all[i].Type < all[j].Type
		}
		return all[i].Pattern < all[j].Pattern
	})
	return all, nil
}

// GenXDir returns the genx config directory path for the current context.
// Used by Cortex Layer 2 to call modelloader.LoadFromDir.
func (s *ConfigStore) GenXDir() (string, error) {
	return s.genxDir()
}

// parseGenXFile extracts pattern/type/schema from a genx config file.
func parseGenXFile(path string) ([]GenXInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Schema string `json:"schema" yaml:"schema"`
		Type   string `json:"type" yaml:"type"`
		Kind   string `json:"kind" yaml:"kind"`
		Models []struct {
			Name string `json:"name" yaml:"name"`
		} `json:"models" yaml:"models"`
		Voices []struct {
			Name string `json:"name" yaml:"name"`
		} `json:"voices" yaml:"voices"`
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, err
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported extension: %s", ext)
	}

	typ := raw.Type
	if typ == "" {
		typ = raw.Kind
	}

	fileName := filepath.Base(path)
	var infos []GenXInfo

	for _, m := range raw.Models {
		if m.Name != "" {
			infos = append(infos, GenXInfo{
				Pattern: m.Name,
				Type:    typ,
				Schema:  raw.Schema,
				File:    fileName,
			})
		}
	}
	for _, v := range raw.Voices {
		if v.Name != "" {
			infos = append(infos, GenXInfo{
				Pattern: v.Name,
				Type:    typ,
				Schema:  raw.Schema,
				File:    fileName,
			})
		}
	}

	return infos, nil
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
