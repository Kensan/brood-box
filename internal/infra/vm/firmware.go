// SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gofrs/flock"
)

const (
	firmwareSourceDownload = "download"
	firmwareSourceSystem   = "system"
	// maxFirmwareArchiveSize caps firmware downloads to 64 MiB.
	maxFirmwareArchiveSize = 64 << 20
	// maxFirmwareExtractSize caps extracted firmware to 128 MiB.
	maxFirmwareExtractSize = 128 << 20
	// maxFirmwareEntries caps the number of tar entries to prevent inode exhaustion.
	maxFirmwareEntries = 1000
)

type FirmwareResolution struct {
	Dir       string
	Version   string
	OS        string
	Arch      string
	Source    string
	URL       string
	Timestamp time.Time
}

type FirmwareManifest struct {
	Version     string    `json:"version"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	Source      string    `json:"source"`
	URL         string    `json:"url,omitempty"`
	LibraryHash string    `json:"library_hash,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

type FirmwareReference struct {
	Version   string    `json:"version"`
	Source    string    `json:"source"`
	Path      string    `json:"path"`
	URL       string    `json:"url,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type FirmwareResolveOpts struct {
	CacheDir        string
	Version         string
	OS              string
	Arch            string
	DownloadEnabled bool
	Logger          *slog.Logger
}

func ResolveFirmware(ctx context.Context, opts FirmwareResolveOpts) (FirmwareResolution, error) {
	version := opts.Version
	if version == "" {
		return FirmwareResolution{}, errors.New("firmware version is required")
	}

	osName := opts.OS
	if osName == "" {
		osName = runtime.GOOS
	}
	arch := opts.Arch
	if arch == "" {
		arch = runtime.GOARCH
	}

	var downloadErr error
	if opts.DownloadEnabled {
		cacheDir := opts.CacheDir
		if cacheDir == "" {
			return FirmwareResolution{}, errors.New("firmware cache directory is required")
		}
		res, err := downloadFirmware(ctx, cacheDir, version, osName, arch)
		if err == nil {
			return res, nil
		}
		downloadErr = err
		if opts.Logger != nil {
			opts.Logger.Warn("firmware download failed, falling back to system", "error", err)
		}
	}

	res, err := findSystemFirmware(version, osName, arch)
	if err == nil {
		return res, nil
	}
	if downloadErr != nil {
		return FirmwareResolution{}, fmt.Errorf("resolve firmware: download failed: %w; system lookup failed: %v", downloadErr, err)
	}
	return FirmwareResolution{}, fmt.Errorf("resolve firmware: system lookup failed: %w", err)
}

func WriteFirmwareReference(path string, ref FirmwareReference) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create firmware ref directory: %w", err)
	}
	data, err := json.MarshalIndent(ref, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal firmware ref: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write firmware ref: %w", err)
	}
	return nil
}

func downloadFirmware(ctx context.Context, cacheRoot, version, osName, arch string) (FirmwareResolution, error) {
	url := firmwareURL(version, osName, arch)
	checksumURL := url + ".sha256"
	cacheDir := filepath.Join(cacheRoot, version, fmt.Sprintf("%s-%s", osName, arch))
	manifestPath := filepath.Join(cacheDir, "firmware.json")

	lock := flock.New(filepath.Join(cacheRoot, ".firmware.lock"))
	if err := ensureSecureCacheRoot(cacheRoot); err != nil {
		return FirmwareResolution{}, err
	}
	if err := lock.Lock(); err != nil {
		return FirmwareResolution{}, fmt.Errorf("acquire firmware lock: %w", err)
	}
	defer func() { _ = lock.Unlock() }()

	if manifest, ok := readFirmwareManifestRaw(manifestPath); ok && manifest.LibraryHash != "" {
		if fwPath, err := findFirmwareFile(cacheDir, osName); err == nil {
			if fileHash, hashErr := hashFile(fwPath); hashErr == nil && fileHash == manifest.LibraryHash {
				return manifestToResolution(manifestPath, manifest), nil
			}
		}
		// Hash mismatch, missing file, or legacy manifest — invalidate and re-download.
	}

	if err := os.RemoveAll(cacheDir); err != nil {
		return FirmwareResolution{}, fmt.Errorf("clear firmware cache: %w", err)
	}

	tmpArchive, err := os.CreateTemp(cacheRoot, "firmware-*.tar.gz")
	if err != nil {
		return FirmwareResolution{}, fmt.Errorf("create firmware temp archive: %w", err)
	}
	tmpArchivePath := tmpArchive.Name()
	defer func() {
		_ = tmpArchive.Close()
		_ = os.Remove(tmpArchivePath)
	}()

	checksum, err := downloadChecksum(ctx, checksumURL)
	if err != nil {
		return FirmwareResolution{}, err
	}
	archiveHash, err := downloadToFile(ctx, url, tmpArchive, maxFirmwareArchiveSize)
	if err != nil {
		return FirmwareResolution{}, err
	}
	if !strings.EqualFold(archiveHash, checksum) {
		return FirmwareResolution{}, fmt.Errorf("firmware checksum mismatch: expected %s got %s", checksum, archiveHash)
	}
	if err := tmpArchive.Close(); err != nil {
		return FirmwareResolution{}, fmt.Errorf("close firmware archive: %w", err)
	}

	tmpDir, err := os.MkdirTemp(cacheRoot, "firmware-extract-")
	if err != nil {
		return FirmwareResolution{}, fmt.Errorf("create firmware temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := extractTarGz(tmpArchivePath, tmpDir, maxFirmwareExtractSize); err != nil {
		return FirmwareResolution{}, fmt.Errorf("extract firmware archive: %w", err)
	}
	fwPath, err := findFirmwareFile(tmpDir, osName)
	if err != nil {
		return FirmwareResolution{}, errors.New("firmware archive missing libkrunfw")
	}
	fwHash, err := hashFile(fwPath)
	if err != nil {
		return FirmwareResolution{}, fmt.Errorf("hash firmware library: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(cacheDir), 0o700); err != nil {
		return FirmwareResolution{}, fmt.Errorf("create firmware parent: %w", err)
	}
	if err := os.Rename(tmpDir, cacheDir); err != nil {
		return FirmwareResolution{}, fmt.Errorf("finalize firmware cache: %w", err)
	}

	manifest := FirmwareManifest{
		Version:     version,
		OS:          osName,
		Arch:        arch,
		Source:      firmwareSourceDownload,
		URL:         url,
		LibraryHash: fwHash,
		Timestamp:   time.Now().UTC(),
	}
	if err := writeFirmwareManifest(manifestPath, manifest); err != nil {
		return FirmwareResolution{}, err
	}

	return FirmwareResolution{
		Dir:       cacheDir,
		Version:   version,
		OS:        osName,
		Arch:      arch,
		Source:    firmwareSourceDownload,
		URL:       url,
		Timestamp: manifest.Timestamp,
	}, nil
}

func findSystemFirmware(version, osName, arch string) (FirmwareResolution, error) {
	path, err := findFirmwareInDirs(systemFirmwareDirs(), firmwareLibNames(osName))
	if err != nil {
		return FirmwareResolution{}, errors.New("libkrunfw not found on system")
	}
	return FirmwareResolution{
		Dir:       filepath.Dir(path),
		Version:   version,
		OS:        osName,
		Arch:      arch,
		Source:    firmwareSourceSystem,
		Timestamp: time.Now().UTC(),
	}, nil
}

// findFirmwareInDirs checks for firmware files by direct os.Stat in each
// directory. Returns the first match. This avoids recursive WalkDir on
// large system directories like /usr/lib.
func findFirmwareInDirs(dirs, names []string) (string, error) {
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		for _, name := range names {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}
	return "", errors.New("libkrunfw not found")
}

func firmwareURL(version, osName, arch string) string {
	return fmt.Sprintf("https://github.com/stacklok/propolis/releases/download/%s/propolis-firmware-%s-%s.tar.gz", version, osName, arch)
}

func downloadToFile(ctx context.Context, url string, dst *os.File, maxBytes int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create firmware request: %w", err)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download firmware: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download firmware: unexpected status %s", resp.Status)
	}
	hasher := sha256.New()
	lr := io.LimitReader(resp.Body, maxBytes+1)
	n, err := io.Copy(io.MultiWriter(dst, hasher), lr)
	if err != nil {
		return "", fmt.Errorf("download firmware: %w", err)
	}
	if n > maxBytes {
		return "", fmt.Errorf("download firmware: exceeded %d bytes", maxBytes)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func extractTarGz(archivePath, destDir string, maxBytes int64) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open firmware archive: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer func() {
		_ = gz.Close()
	}()

	tr := tar.NewReader(gz)
	var extracted int64
	var entries int
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}

		entries++
		if entries > maxFirmwareEntries {
			return fmt.Errorf("extract firmware: exceeded %d entries", maxFirmwareEntries)
		}

		if hdr.Name == "" {
			continue
		}
		cleanName := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return fmt.Errorf("invalid firmware entry: %s", hdr.Name)
		}
		targetPath := filepath.Join(destDir, cleanName)
		if !strings.HasPrefix(targetPath, destDir+string(filepath.Separator)) && targetPath != destDir {
			return fmt.Errorf("invalid firmware entry path: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create dir: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create parent dir: %w", err)
			}
			mode := safeFileMode(hdr.Mode)
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}
			remaining := maxBytes - extracted
			if remaining <= 0 {
				_ = out.Close()
				return fmt.Errorf("extract firmware: exceeded %d bytes", maxBytes)
			}
			lr := io.LimitReader(tr, remaining+1)
			written, err := io.Copy(out, lr)
			extracted += written
			if err != nil {
				_ = out.Close()
				return fmt.Errorf("write file: %w", err)
			}
			if written > remaining {
				_ = out.Close()
				return fmt.Errorf("extract firmware: exceeded %d bytes", maxBytes)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("close file: %w", err)
			}
		default:
			return fmt.Errorf("unsupported firmware entry type: %d", hdr.Typeflag)
		}
	}
	return nil
}

// findFirmwareFile returns the full path to the libkrunfw library within dir.
// Uses WalkDir for cache directories where tar extraction may produce
// unpredictable subdirectory structures.
func findFirmwareFile(dir, osName string) (string, error) {
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("firmware directory not found: %w", err)
	}
	var found string
	var errFound = errors.New("firmware-found")
	walkErr := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		for _, name := range firmwareLibNames(osName) {
			if entry.Name() == name {
				found = path
				return errFound
			}
		}
		if strings.HasPrefix(entry.Name(), "libkrunfw.") {
			found = path
			return errFound
		}
		return nil
	})
	if errors.Is(walkErr, errFound) {
		return found, nil
	}
	if walkErr != nil {
		return "", fmt.Errorf("search firmware dir: %w", walkErr)
	}
	return "", errors.New("libkrunfw not found")
}

// hashFile computes the SHA-256 hex digest of the file at path.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for hashing: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readFirmwareManifestRaw(path string) (FirmwareManifest, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FirmwareManifest{}, false
	}
	var manifest FirmwareManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return FirmwareManifest{}, false
	}
	return manifest, true
}

func manifestToResolution(manifestPath string, m FirmwareManifest) FirmwareResolution {
	return FirmwareResolution{
		Dir:       filepath.Dir(manifestPath),
		Version:   m.Version,
		OS:        m.OS,
		Arch:      m.Arch,
		Source:    m.Source,
		URL:       m.URL,
		Timestamp: m.Timestamp,
	}
}

func writeFirmwareManifest(path string, manifest FirmwareManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal firmware manifest: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write firmware manifest: %w", err)
	}
	return nil
}

func systemFirmwareDirs() []string {
	if runtime.GOOS == "darwin" {
		return []string{"/opt/homebrew/lib", "/usr/local/lib"}
	}
	return []string{"/usr/lib", "/usr/local/lib", "/lib", "/lib64", "/usr/lib64"}
}

func firmwareLibNames(osName string) []string {
	if osName == "darwin" {
		return []string{"libkrunfw.dylib"}
	}
	return []string{"libkrunfw.so.5"}
}

func downloadChecksum(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create firmware checksum request: %w", err)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download firmware checksum: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download firmware checksum: unexpected status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("read firmware checksum: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "", errors.New("firmware checksum is empty")
	}
	checksum := fields[0]
	if len(checksum) != 64 {
		return "", fmt.Errorf("invalid firmware checksum length: %d", len(checksum))
	}
	if _, err := hex.DecodeString(checksum); err != nil {
		return "", fmt.Errorf("invalid firmware checksum: %w", err)
	}
	return checksum, nil
}

func safeFileMode(mode int64) os.FileMode {
	if mode&0o111 != 0 {
		return 0o755
	}
	return 0o644
}

func ensureSecureCacheRoot(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create firmware cache root: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat firmware cache root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("firmware cache root must not be a symlink")
	}
	if !info.IsDir() {
		return errors.New("firmware cache root is not a directory")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("firmware cache root permissions too open: %v", info.Mode().Perm())
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		if stat.Uid != uint32(os.Getuid()) {
			return fmt.Errorf("firmware cache root not owned by current user")
		}
	}
	return nil
}
