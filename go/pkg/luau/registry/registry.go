// Package registry provides Luau package management with version resolution,
// caching, and require() function injection.
//
// The registry supports:
// - Package storage and retrieval with version constraints
// - Cascading lookups through multiple upstreams
// - Cycle detection in require() calls
// - Checksum verification
package registry

import (
	"errors"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

var (
	// ErrPackageNotFound is returned when a package cannot be found.
	ErrPackageNotFound = errors.New("package not found")
	// ErrVersionNotFound is returned when a specific version cannot be found.
	ErrVersionNotFound = errors.New("version not found")
	// ErrInvalidPackage is returned when a package is malformed.
	ErrInvalidPackage = errors.New("invalid package")
	// ErrCyclicDependency is returned when a cyclic require is detected.
	ErrCyclicDependency = errors.New("cyclic dependency detected")
	// ErrChecksumMismatch is returned when checksum verification fails.
	ErrChecksumMismatch = errors.New("checksum mismatch")
)

// Registry manages Luau packages with version resolution and caching.
type Registry interface {
	// ListPackages returns all available package names.
	ListPackages() ([]string, error)

	// ListVersions returns all available versions for a package.
	ListVersions(name string) ([]Version, error)

	// GetMeta returns metadata for a specific package version.
	// Use "latest" for version to get the latest available version.
	GetMeta(name, version string) (*PackageMeta, error)

	// Resolve finds and loads a package matching the constraint.
	// Constraint can be:
	// - Empty or "latest" for the latest version
	// - Exact version like "1.0.0"
	// - Semver constraint like "^1.0.0", "~1.2.0", ">=1.0.0 <2.0.0"
	Resolve(name, constraint string) (*Package, error)

	// Store adds or updates a package in the registry.
	Store(pkg *Package) error

	// Delete removes a package version from the registry.
	Delete(name, version string) error

	// RequireFunc returns a Luau function that implements require().
	// This function handles:
	// - Module name parsing
	// - Version constraint resolution
	// - Cycle detection
	// - Module execution and caching
	RequireFunc(state *luau.State) luau.GoFunc

	// Close releases any resources held by the registry.
	Close() error
}

// PackageType defines the type of a package.
type PackageType string

const (
	// PackageTypeLib is a library package (provides functions/utilities).
	PackageTypeLib PackageType = "lib"
	// PackageTypeAgent is an agent package (recv/emit pattern).
	PackageTypeAgent PackageType = "agent"
	// PackageTypeTool is a tool package (input/output pattern).
	PackageTypeTool PackageType = "tool"
)

// PackageMeta holds metadata about a package.
type PackageMeta struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Type         PackageType       `json:"type"`
	Entry        string            `json:"entry,omitempty"`        // Default: "init.luau"
	Dependencies map[string]string `json:"dependencies,omitempty"` // name -> version constraint
	Checksum     string            `json:"checksum,omitempty"`     // SHA256
	Size         int64             `json:"size,omitempty"`
	Description  string            `json:"description,omitempty"`
	Author       string            `json:"author,omitempty"`
	License      string            `json:"license,omitempty"`
	Repository   string            `json:"repository,omitempty"`
}

// EntryFile returns the entry file path, defaulting to "init.luau".
func (m *PackageMeta) EntryFile() string {
	if m.Entry == "" {
		return "init.luau"
	}
	return m.Entry
}

// Package represents a complete Luau package with source files.
type Package struct {
	Meta  *PackageMeta
	Entry []byte            // Entry file source
	Files map[string][]byte // All source files (path -> content)
}

// GetFile returns a source file by path.
func (p *Package) GetFile(path string) ([]byte, bool) {
	if path == p.Meta.EntryFile() || path == "" {
		return p.Entry, len(p.Entry) > 0
	}
	content, ok := p.Files[path]
	return content, ok
}

// RegistryConfig holds configuration for registry creation.
type RegistryConfig struct {
	// Upstreams defines upstream registries to query (in order).
	Upstreams []Upstream

	// MaxCacheSize limits the in-memory cache size (bytes).
	// 0 means no limit.
	MaxCacheSize int64

	// MaxPackageSize limits the maximum package size (bytes).
	// 0 means no limit.
	MaxPackageSize int64
}
