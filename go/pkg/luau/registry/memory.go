package registry

import (
	"context"
	"sort"
	"sync"

	"github.com/haivivi/giztoy/go/pkg/luau"
)

// MemoryRegistry is an in-memory implementation of Registry.
// Useful for testing and development.
type MemoryRegistry struct {
	mu        sync.RWMutex
	packages  map[string]map[string]*Package // name -> version -> package
	upstreams []Upstream
	ctx       context.Context
}

// NewMemoryRegistry creates a new in-memory registry.
func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{
		packages: make(map[string]map[string]*Package),
		ctx:      context.Background(),
	}
}

// NewMemoryRegistryWithConfig creates a new in-memory registry with config.
func NewMemoryRegistryWithConfig(cfg *RegistryConfig) *MemoryRegistry {
	r := NewMemoryRegistry()
	if cfg != nil {
		r.upstreams = cfg.Upstreams
	}
	return r
}

// SetContext sets the context for upstream operations.
func (r *MemoryRegistry) SetContext(ctx context.Context) {
	r.ctx = ctx
}

// ListPackages returns all package names.
func (r *MemoryRegistry) ListPackages() ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.packages))
	for name := range r.packages {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// ListVersions returns all versions for a package.
func (r *MemoryRegistry) ListVersions(name string) ([]Version, error) {
	r.mu.RLock()
	versions, ok := r.packages[name]
	r.mu.RUnlock()

	if !ok || len(versions) == 0 {
		// Try upstreams
		for _, upstream := range r.upstreams {
			client := NewUpstreamClient(&upstream)
			verStrs, err := client.ListVersions(r.ctx, name)
			if err == nil && len(verStrs) > 0 {
				var result []Version
				for _, vs := range verStrs {
					v, err := ParseVersion(vs)
					if err == nil {
						result = append(result, v)
					}
				}
				SortVersions(result)
				return result, nil
			}
		}
		return nil, ErrPackageNotFound
	}

	result := make([]Version, 0, len(versions))
	for vs := range versions {
		v, err := ParseVersion(vs)
		if err == nil {
			result = append(result, v)
		}
	}
	SortVersions(result)
	return result, nil
}

// GetMeta returns metadata for a specific version.
func (r *MemoryRegistry) GetMeta(name, version string) (*PackageMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, ok := r.packages[name]
	if !ok {
		return nil, ErrPackageNotFound
	}

	// Handle "latest"
	if version == "" || version == "latest" {
		var latest *Package
		var latestVer Version
		for vs, pkg := range versions {
			v, err := ParseVersion(vs)
			if err != nil {
				continue
			}
			if latest == nil || v.GreaterThan(latestVer) {
				latest = pkg
				latestVer = v
			}
		}
		if latest != nil {
			return latest.Meta, nil
		}
		return nil, ErrVersionNotFound
	}

	pkg, ok := versions[version]
	if !ok {
		return nil, ErrVersionNotFound
	}
	return pkg.Meta, nil
}

// Resolve finds and loads a package matching the constraint.
func (r *MemoryRegistry) Resolve(name, constraint string) (*Package, error) {
	// Parse constraint
	c, err := ParseConstraint(constraint)
	if err != nil {
		return nil, err
	}

	// Get available versions
	versions, err := r.ListVersions(name)
	if err != nil {
		return nil, err
	}

	// Find best match
	best := FindBestMatch(versions, c)
	if best == nil {
		return nil, ErrVersionNotFound
	}

	// Get package
	r.mu.RLock()
	pkgVersions, ok := r.packages[name]
	if ok {
		pkg, ok := pkgVersions[best.String()]
		r.mu.RUnlock()
		if ok {
			return pkg, nil
		}
	} else {
		r.mu.RUnlock()
	}

	// Try to fetch from upstream
	for _, upstream := range r.upstreams {
		client := NewUpstreamClient(&upstream)
		pkg, err := client.FetchPackage(r.ctx, name, best.String())
		if err == nil {
			// Cache it
			r.Store(pkg)
			return pkg, nil
		}
	}

	return nil, ErrVersionNotFound
}

// Store adds or updates a package.
func (r *MemoryRegistry) Store(pkg *Package) error {
	if pkg == nil || pkg.Meta == nil {
		return ErrInvalidPackage
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	versions, ok := r.packages[pkg.Meta.Name]
	if !ok {
		versions = make(map[string]*Package)
		r.packages[pkg.Meta.Name] = versions
	}

	versions[pkg.Meta.Version] = pkg
	return nil
}

// Delete removes a package version.
func (r *MemoryRegistry) Delete(name, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	versions, ok := r.packages[name]
	if !ok {
		return nil
	}

	delete(versions, version)
	if len(versions) == 0 {
		delete(r.packages, name)
	}
	return nil
}

// RequireFunc returns a Luau require function.
func (r *MemoryRegistry) RequireFunc(state *luau.State) luau.GoFunc {
	return newRequireFuncSimple(r, state)
}

// Close releases resources.
func (r *MemoryRegistry) Close() error {
	return nil
}

// AddPackageFromSource creates and stores a package from source code.
// This is a convenience method for testing.
func (r *MemoryRegistry) AddPackageFromSource(name, version string, source []byte) error {
	pkg := &Package{
		Meta: &PackageMeta{
			Name:    name,
			Version: version,
			Type:    PackageTypeLib,
			Entry:   "init.luau",
		},
		Entry: source,
		Files: make(map[string][]byte),
	}
	return r.Store(pkg)
}

// Ensure MemoryRegistry implements Registry.
var _ Registry = (*MemoryRegistry)(nil)
