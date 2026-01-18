package cli

import (
	"os"
	"path/filepath"
)

// Paths provides access to giztoy directory structure
type Paths struct {
	// AppName is the application name
	AppName string

	// HomeDir is the user's home directory
	HomeDir string
}

// NewPaths creates a new Paths instance for the given app
func NewPaths(appName string) (*Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &Paths{
		AppName: appName,
		HomeDir: home,
	}, nil
}

// BaseDir returns the base giztoy directory (~/.giztoy)
func (p *Paths) BaseDir() string {
	return filepath.Join(p.HomeDir, DefaultBaseDir)
}

// AppDir returns the app-specific directory (~/.giztoy/<app>)
func (p *Paths) AppDir() string {
	return filepath.Join(p.BaseDir(), p.AppName)
}

// ConfigFile returns the config file path (~/.giztoy/<app>/config.yaml)
func (p *Paths) ConfigFile() string {
	return filepath.Join(p.AppDir(), DefaultConfigFile)
}

// CacheDir returns the cache directory (~/.giztoy/<app>/cache)
func (p *Paths) CacheDir() string {
	return filepath.Join(p.AppDir(), "cache")
}

// LogDir returns the log directory (~/.giztoy/<app>/logs)
func (p *Paths) LogDir() string {
	return filepath.Join(p.AppDir(), "logs")
}

// DataDir returns the data directory (~/.giztoy/<app>/data)
func (p *Paths) DataDir() string {
	return filepath.Join(p.AppDir(), "data")
}

// EnsureAppDir creates the app directory if it doesn't exist
func (p *Paths) EnsureAppDir() error {
	return os.MkdirAll(p.AppDir(), 0755)
}

// EnsureCacheDir creates the cache directory if it doesn't exist
func (p *Paths) EnsureCacheDir() error {
	return os.MkdirAll(p.CacheDir(), 0755)
}

// EnsureLogDir creates the log directory if it doesn't exist
func (p *Paths) EnsureLogDir() error {
	return os.MkdirAll(p.LogDir(), 0755)
}

// EnsureDataDir creates the data directory if it doesn't exist
func (p *Paths) EnsureDataDir() error {
	return os.MkdirAll(p.DataDir(), 0755)
}

// CachePath returns a path within the cache directory
func (p *Paths) CachePath(name string) string {
	return filepath.Join(p.CacheDir(), name)
}

// LogPath returns a path within the log directory
func (p *Paths) LogPath(name string) string {
	return filepath.Join(p.LogDir(), name)
}

// DataPath returns a path within the data directory
func (p *Paths) DataPath(name string) string {
	return filepath.Join(p.DataDir(), name)
}
