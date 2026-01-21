package playground

import (
	"encoding/json"
	"io/fs"
	"maps"
	"path"
	"strings"

	"github.com/goccy/go-yaml"
)

// Loader is a function that unmarshals data into a map.
type Loader func(data []byte) (map[string]any, error)

// JSONLoader is the default JSON loader.
var JSONLoader Loader = func(data []byte) (map[string]any, error) {
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// YAMLLoader is the default YAML loader.
var YAMLLoader Loader = func(data []byte) (map[string]any, error) {
	var v map[string]any
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// DefaultLoaders returns the default loaders for JSON and YAML.
func DefaultLoaders() map[string]Loader {
	return map[string]Loader{
		".json": JSONLoader,
		".yaml": YAMLLoader,
		".yml":  YAMLLoader,
	}
}

// ReadonlyLayer represents a readonly layer of key-value storage.
type ReadonlyLayer struct {
	Name string
	Data map[string]map[string]any
}

// WritableLayer represents the writable layer with delete tracking.
type WritableLayer struct {
	Data    map[string]map[string]any
	Deleted map[string]bool // tracks deleted keys
}

func newWritableLayer() *WritableLayer {
	return &WritableLayer{
		Data:    make(map[string]map[string]any),
		Deleted: make(map[string]bool),
	}
}

// Store is a layered key-value store.
// It has multiple readonly layers at the bottom and a writable layer on top.
// When getting a value, it merges from bottom layers up, with upper layers overriding lower ones.
type Store struct {
	loaders        map[string]Loader
	readonlyLayers []*ReadonlyLayer
	writable       *WritableLayer
}

// NewStore creates a new layered key-value store with the given loaders.
// Loaders map file extensions (e.g. ".json") to unmarshal functions.
// If loaders is nil, DefaultLoaders() is used.
func NewStore(loaders map[string]Loader) *Store {
	if loaders == nil {
		loaders = DefaultLoaders()
	}
	return &Store{
		loaders:        loaders,
		readonlyLayers: nil,
		writable:       newWritableLayer(),
	}
}

// Writable returns the writable layer.
func (s *Store) Writable() *WritableLayer {
	return s.writable
}

// Get retrieves a value by key, merging all layers from bottom to top.
// Returns the merged value and whether the key exists.
func (s *Store) Get(key string) (map[string]any, bool) {
	// Check if deleted in writable layer
	if s.writable.Deleted[key] {
		return nil, false
	}

	// Start with empty result
	var result map[string]any
	found := false

	// Merge from bottom readonly layers up
	for _, layer := range s.readonlyLayers {
		if v, ok := layer.Data[key]; ok {
			if result == nil {
				result = make(map[string]any)
			}
			maps.Copy(result, v)
			found = true
		}
	}

	// Merge writable layer on top
	if v, ok := s.writable.Data[key]; ok {
		if result == nil {
			result = make(map[string]any)
		}
		maps.Copy(result, v)
		found = true
	}

	return result, found
}

// Set sets a value in the writable layer.
func (s *Store) Set(key string, value map[string]any) {
	s.writable.Data[key] = value
	delete(s.writable.Deleted, key)
}

// Delete marks a key as deleted in the writable layer.
// The key will not appear in Get results even if it exists in lower layers.
func (s *Store) Delete(key string) {
	s.writable.Deleted[key] = true
	delete(s.writable.Data, key)
}

// AddReadonlyLayer adds a readonly layer with the given name and data.
// Newer layers are added on top (higher priority in merge).
func (s *Store) AddReadonlyLayer(name string, data map[string]map[string]any) {
	layer := &ReadonlyLayer{
		Name: name,
		Data: data,
	}
	s.readonlyLayers = append(s.readonlyLayers, layer)
}

// LoadReadonlyLayer recursively loads files from a fs.FS as a readonly layer.
// Only files with extensions matching the configured loaders are loaded.
// The key for each file is its relative path without the extension.
// For example, "foo/bar.json" becomes key "foo/bar".
func (s *Store) LoadReadonlyLayer(name string, fsys fs.FS) error {
	layer := &ReadonlyLayer{
		Name: name,
		Data: make(map[string]map[string]any),
	}

	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(path.Ext(p))
		loader, ok := s.loaders[ext]
		if !ok {
			return nil // skip unsupported extensions
		}

		// Remove extension to get key
		key := strings.TrimSuffix(p, path.Ext(p))

		// Read and parse file
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}

		value, err := loader(data)
		if err != nil {
			return err
		}

		layer.Data[key] = value
		return nil
	})

	if err != nil {
		return err
	}

	s.readonlyLayers = append(s.readonlyLayers, layer)
	return nil
}

// ReadonlyLayerCount returns the number of readonly layers.
func (s *Store) ReadonlyLayerCount() int {
	return len(s.readonlyLayers)
}

// Clear removes all data from all layers.
func (s *Store) Clear() {
	s.readonlyLayers = nil
	s.writable = newWritableLayer()
}
