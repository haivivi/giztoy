package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// ParseManifest parses a pkg.json file into PackageMeta.
func ParseManifest(data []byte) (*PackageMeta, error) {
	var meta PackageMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse pkg.json: %w", err)
	}

	// Validate required fields
	if meta.Name == "" {
		return nil, fmt.Errorf("%w: missing name", ErrInvalidPackage)
	}
	if meta.Version == "" {
		return nil, fmt.Errorf("%w: missing version", ErrInvalidPackage)
	}

	// Set defaults
	if meta.Type == "" {
		meta.Type = PackageTypeLib
	}
	if meta.Entry == "" {
		meta.Entry = "init.luau"
	}

	return &meta, nil
}

// MarshalManifest serializes PackageMeta to JSON.
func MarshalManifest(meta *PackageMeta) ([]byte, error) {
	return json.MarshalIndent(meta, "", "  ")
}

// ParseTarball extracts a package from a gzipped tarball.
func ParseTarball(data []byte) (*Package, error) {
	// Decompress gzip
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decompress: %w", err)
	}
	defer gzr.Close()

	// Read tar
	tr := tar.NewReader(gzr)

	pkg := &Package{
		Files: make(map[string][]byte),
	}

	var manifestData []byte

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Normalize path (remove leading ./ or package name prefix)
		name := normalizePath(header.Name)
		if name == "" {
			continue
		}

		// Read file content
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}

		// Handle pkg.json
		if name == "pkg.json" {
			manifestData = content
			continue
		}

		// Store file
		pkg.Files[name] = content
	}

	// Parse manifest
	if manifestData == nil {
		return nil, fmt.Errorf("%w: missing pkg.json", ErrInvalidPackage)
	}

	meta, err := ParseManifest(manifestData)
	if err != nil {
		return nil, err
	}
	pkg.Meta = meta

	// Extract entry file
	entryPath := meta.EntryFile()
	if entry, ok := pkg.Files[entryPath]; ok {
		pkg.Entry = entry
		delete(pkg.Files, entryPath)
	} else {
		return nil, fmt.Errorf("%w: missing entry file %s", ErrInvalidPackage, entryPath)
	}

	return pkg, nil
}

// CreateTarball creates a gzipped tarball from a package.
func CreateTarball(pkg *Package) ([]byte, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	// Write pkg.json
	manifestData, err := MarshalManifest(pkg.Meta)
	if err != nil {
		return nil, err
	}
	if err := writeToTar(tw, "pkg.json", manifestData); err != nil {
		return nil, err
	}

	// Write entry file
	entryPath := pkg.Meta.EntryFile()
	if err := writeToTar(tw, entryPath, pkg.Entry); err != nil {
		return nil, err
	}

	// Write other files
	for path, content := range pkg.Files {
		if err := writeToTar(tw, path, content); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// writeToTar writes a file to the tar archive.
func writeToTar(tw *tar.Writer, name string, content []byte) error {
	header := &tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(content)
	return err
}

// normalizePath normalizes a tar entry path.
func normalizePath(p string) string {
	// Remove leading ./
	p = strings.TrimPrefix(p, "./")

	// Remove leading package/ or similar prefix
	parts := strings.SplitN(p, "/", 2)
	if len(parts) == 2 && !strings.HasSuffix(parts[0], ".luau") {
		// Likely a directory prefix like "package/"
		p = parts[1]
	}

	// Clean the path
	p = filepath.Clean(p)

	// Ignore hidden files and directories
	for _, part := range strings.Split(p, "/") {
		if strings.HasPrefix(part, ".") {
			return ""
		}
	}

	return p
}

// ComputeChecksum computes the SHA256 checksum of data.
func ComputeChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// VerifyChecksum verifies that data matches the expected checksum.
func VerifyChecksum(data []byte, expected string) error {
	actual := ComputeChecksum(data)
	if actual != expected {
		return fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, expected, actual)
	}
	return nil
}

// PackageKey returns a unique key for a package name and version.
func PackageKey(name, version string) string {
	return name + "@" + version
}

// ParsePackageKey parses a package key into name and version.
func ParsePackageKey(key string) (name, version string) {
	idx := strings.LastIndex(key, "@")
	if idx == -1 {
		return key, ""
	}
	return key[:idx], key[idx+1:]
}

// ParseRequireName parses a require name like "@scope/pkg" or "@scope/pkg@^1.0.0".
func ParseRequireName(s string) (name, constraint string) {
	s = strings.TrimSpace(s)

	// Check for version constraint
	// @scope/pkg@^1.0.0 or pkg@>=1.0.0
	idx := strings.LastIndex(s, "@")
	if idx > 0 {
		// Make sure it's not just the scope @
		beforeAt := s[:idx]
		// If beforeAt contains another @, the last @ is the version separator
		if strings.Contains(beforeAt, "@") || !strings.HasPrefix(beforeAt, "@") {
			return beforeAt, s[idx+1:]
		}
	}

	return s, ""
}
