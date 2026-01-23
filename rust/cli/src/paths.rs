//! Path utilities for giztoy applications.

use std::io;
use std::path::PathBuf;

/// Default base configuration directory name.
pub const DEFAULT_BASE_DIR: &str = ".giztoy";

/// Default configuration filename.
pub const DEFAULT_CONFIG_FILE: &str = "config.yaml";

/// Provides access to giztoy directory structure.
#[derive(Debug, Clone)]
pub struct Paths {
    /// Application name.
    pub app_name: String,
    /// User's home directory.
    pub home_dir: PathBuf,
}

impl Paths {
    /// Creates a new Paths instance for the given app.
    pub fn new(app_name: impl Into<String>) -> io::Result<Self> {
        let home_dir = dirs::home_dir().ok_or_else(|| {
            io::Error::new(io::ErrorKind::NotFound, "could not find home directory")
        })?;
        Ok(Self {
            app_name: app_name.into(),
            home_dir,
        })
    }

    /// Returns the base giztoy directory (~/.giztoy).
    pub fn base_dir(&self) -> PathBuf {
        self.home_dir.join(DEFAULT_BASE_DIR)
    }

    /// Returns the app-specific directory (~/.giztoy/<app>).
    pub fn app_dir(&self) -> PathBuf {
        self.base_dir().join(&self.app_name)
    }

    /// Returns the config file path (~/.giztoy/<app>/config.yaml).
    pub fn config_file(&self) -> PathBuf {
        self.app_dir().join(DEFAULT_CONFIG_FILE)
    }

    /// Returns the cache directory (~/.giztoy/<app>/cache).
    pub fn cache_dir(&self) -> PathBuf {
        self.app_dir().join("cache")
    }

    /// Returns the log directory (~/.giztoy/<app>/logs).
    pub fn log_dir(&self) -> PathBuf {
        self.app_dir().join("logs")
    }

    /// Returns the data directory (~/.giztoy/<app>/data).
    pub fn data_dir(&self) -> PathBuf {
        self.app_dir().join("data")
    }

    /// Creates the app directory if it doesn't exist.
    pub fn ensure_app_dir(&self) -> io::Result<()> {
        std::fs::create_dir_all(self.app_dir())
    }

    /// Creates the cache directory if it doesn't exist.
    pub fn ensure_cache_dir(&self) -> io::Result<()> {
        std::fs::create_dir_all(self.cache_dir())
    }

    /// Creates the log directory if it doesn't exist.
    pub fn ensure_log_dir(&self) -> io::Result<()> {
        std::fs::create_dir_all(self.log_dir())
    }

    /// Creates the data directory if it doesn't exist.
    pub fn ensure_data_dir(&self) -> io::Result<()> {
        std::fs::create_dir_all(self.data_dir())
    }

    /// Returns a path within the cache directory.
    pub fn cache_path(&self, name: &str) -> PathBuf {
        self.cache_dir().join(name)
    }

    /// Returns a path within the log directory.
    pub fn log_path(&self, name: &str) -> PathBuf {
        self.log_dir().join(name)
    }

    /// Returns a path within the data directory.
    pub fn data_path(&self, name: &str) -> PathBuf {
        self.data_dir().join(name)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_paths_new() {
        let paths = Paths::new("testapp").unwrap();
        assert_eq!(paths.app_name, "testapp");
        assert!(!paths.home_dir.as_os_str().is_empty());
    }

    #[test]
    fn test_paths_structure() {
        let paths = Paths::new("testapp").unwrap();

        assert!(paths.base_dir().ends_with(".giztoy"));
        assert!(paths.app_dir().ends_with("testapp"));
        assert!(paths.config_file().ends_with("config.yaml"));
        assert!(paths.cache_dir().ends_with("cache"));
        assert!(paths.log_dir().ends_with("logs"));
        assert!(paths.data_dir().ends_with("data"));
    }

    #[test]
    fn test_paths_subpaths() {
        let paths = Paths::new("testapp").unwrap();

        assert!(paths.cache_path("file.txt").ends_with("file.txt"));
        assert!(paths.log_path("app.log").ends_with("app.log"));
        assert!(paths.data_path("data.db").ends_with("data.db"));
    }
}
