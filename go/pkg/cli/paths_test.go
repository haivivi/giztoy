package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewPaths(t *testing.T) {
	paths, err := NewPaths("testapp")
	if err != nil {
		t.Fatalf("NewPaths error: %v", err)
	}

	if paths.AppName != "testapp" {
		t.Errorf("AppName = %q, want %q", paths.AppName, "testapp")
	}

	if paths.HomeDir == "" {
		t.Error("HomeDir should not be empty")
	}
}

func TestPaths_BaseDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	paths := &Paths{AppName: "testapp", HomeDir: home}

	baseDir := paths.BaseDir()
	expected := filepath.Join(home, DefaultBaseDir)

	if baseDir != expected {
		t.Errorf("BaseDir() = %q, want %q", baseDir, expected)
	}
}

func TestPaths_AppDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	paths := &Paths{AppName: "testapp", HomeDir: home}

	appDir := paths.AppDir()
	expected := filepath.Join(home, DefaultBaseDir, "testapp")

	if appDir != expected {
		t.Errorf("AppDir() = %q, want %q", appDir, expected)
	}
}

func TestPaths_ConfigFile(t *testing.T) {
	home, _ := os.UserHomeDir()
	paths := &Paths{AppName: "testapp", HomeDir: home}

	configFile := paths.ConfigFile()
	expected := filepath.Join(home, DefaultBaseDir, "testapp", DefaultConfigFile)

	if configFile != expected {
		t.Errorf("ConfigFile() = %q, want %q", configFile, expected)
	}
}

func TestPaths_CacheDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	paths := &Paths{AppName: "testapp", HomeDir: home}

	cacheDir := paths.CacheDir()

	if !strings.HasSuffix(cacheDir, "cache") {
		t.Errorf("CacheDir() = %q, should end with 'cache'", cacheDir)
	}
}

func TestPaths_LogDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	paths := &Paths{AppName: "testapp", HomeDir: home}

	logDir := paths.LogDir()

	if !strings.HasSuffix(logDir, "logs") {
		t.Errorf("LogDir() = %q, should end with 'logs'", logDir)
	}
}

func TestPaths_DataDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	paths := &Paths{AppName: "testapp", HomeDir: home}

	dataDir := paths.DataDir()

	if !strings.HasSuffix(dataDir, "data") {
		t.Errorf("DataDir() = %q, should end with 'data'", dataDir)
	}
}

func TestPaths_CachePath(t *testing.T) {
	home, _ := os.UserHomeDir()
	paths := &Paths{AppName: "testapp", HomeDir: home}

	cachePath := paths.CachePath("file.txt")
	expected := filepath.Join(paths.CacheDir(), "file.txt")

	if cachePath != expected {
		t.Errorf("CachePath() = %q, want %q", cachePath, expected)
	}
}

func TestPaths_LogPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	paths := &Paths{AppName: "testapp", HomeDir: home}

	logPath := paths.LogPath("app.log")
	expected := filepath.Join(paths.LogDir(), "app.log")

	if logPath != expected {
		t.Errorf("LogPath() = %q, want %q", logPath, expected)
	}
}

func TestPaths_DataPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	paths := &Paths{AppName: "testapp", HomeDir: home}

	dataPath := paths.DataPath("db.sqlite")
	expected := filepath.Join(paths.DataDir(), "db.sqlite")

	if dataPath != expected {
		t.Errorf("DataPath() = %q, want %q", dataPath, expected)
	}
}

func TestPaths_EnsureAppDir(t *testing.T) {
	// Use temp directory to avoid polluting user's home
	tmpDir := t.TempDir()
	paths := &Paths{AppName: "testapp", HomeDir: tmpDir}

	err := paths.EnsureAppDir()
	if err != nil {
		t.Fatalf("EnsureAppDir error: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(paths.AppDir())
	if err != nil {
		t.Fatalf("AppDir not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("AppDir should be a directory")
	}
}

func TestPaths_EnsureCacheDir(t *testing.T) {
	tmpDir := t.TempDir()
	paths := &Paths{AppName: "testapp", HomeDir: tmpDir}

	err := paths.EnsureCacheDir()
	if err != nil {
		t.Fatalf("EnsureCacheDir error: %v", err)
	}

	info, err := os.Stat(paths.CacheDir())
	if err != nil {
		t.Fatalf("CacheDir not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("CacheDir should be a directory")
	}
}

func TestPaths_EnsureLogDir(t *testing.T) {
	tmpDir := t.TempDir()
	paths := &Paths{AppName: "testapp", HomeDir: tmpDir}

	err := paths.EnsureLogDir()
	if err != nil {
		t.Fatalf("EnsureLogDir error: %v", err)
	}

	info, err := os.Stat(paths.LogDir())
	if err != nil {
		t.Fatalf("LogDir not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("LogDir should be a directory")
	}
}

func TestPaths_EnsureDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	paths := &Paths{AppName: "testapp", HomeDir: tmpDir}

	err := paths.EnsureDataDir()
	if err != nil {
		t.Fatalf("EnsureDataDir error: %v", err)
	}

	info, err := os.Stat(paths.DataDir())
	if err != nil {
		t.Fatalf("DataDir not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("DataDir should be a directory")
	}
}
