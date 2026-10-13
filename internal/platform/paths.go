// Package platform contains operating-system integration that is kept outside
// the domain and persistence packages.
package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	documentsDirectoryName            = "Documents"
	sessionsDirectoryName             = "Sessions"
	applicationSupportName            = "Application Support"
	applicationIdentifier             = "com.vaulttec.fallout-terminal"
	productDirectoryName              = "Fallout Terminal"
	bundledSessionsDirectory          = "sessions"
	bundledDemoFilename               = "demo.json"
	publicAccessFilename              = "public-access.json"
	applicationUpdateRecoveryFilename = "application-update-recovery.json"
	applicationLogsDirectoryName      = "logs"
)

// SessionLocations separates user-owned session documents, the immutable
// bundled sample, and private application metadata.
//
// Resolving these locations has no filesystem side effects. In particular,
// DocumentsDefault is created only after a native save dialog is confirmed.
type SessionLocations struct {
	DocumentsDefault   string
	BundledDemo        string
	ApplicationSupport string
}

// ApplicationLogDirectory resolves the private retained-log directory without
// touching the filesystem.
func ApplicationLogDirectory(applicationSupportDirectory string) (string, error) {
	directory, err := cleanAbsolutePath("application support directory", applicationSupportDirectory)
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, applicationLogsDirectoryName), nil
}

// directoryProvider isolates operating-system directory discovery from path
// policy so redirected and unavailable native roots can be handled without
// consulting the current working directory.
type directoryProvider interface {
	HomeDirectory() (string, error)
	DocumentsDirectory() (string, error)
	ApplicationDataDirectory() (string, error)
}

type homeDirectoryProvider struct {
	home string
}

func (p homeDirectoryProvider) HomeDirectory() (string, error) {
	return p.home, nil
}

func (homeDirectoryProvider) DocumentsDirectory() (string, error) {
	return "", nil
}

func (homeDirectoryProvider) ApplicationDataDirectory() (string, error) {
	return "", nil
}

// PublicAccessSettingsPath resolves the separate version-1 non-secret settings file.
// It has no filesystem side effects and never points into session or player-config storage.
func PublicAccessSettingsPath(applicationSupportDirectory string) (string, error) {
	directory, err := cleanAbsolutePath("application support directory", applicationSupportDirectory)
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, publicAccessFilename), nil
}

// ApplicationUpdateRecoveryPath resolves the private non-user recovery
// journal. It has no filesystem side effects and remains separate from session
// documents and bundled application resources.
func ApplicationUpdateRecoveryPath(applicationSupportDirectory string) (string, error) {
	directory, err := cleanAbsolutePath("application support directory", applicationSupportDirectory)
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, applicationUpdateRecoveryFilename), nil
}

// InstalledApplicationUnit resolves the complete package unit containing the
// running executable and the executable path relative to that unit. It has no
// filesystem side effects.
func InstalledApplicationUnit() (unit, launchRelativePath string, err error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("resolve application executable: %w", err)
	}
	return installedApplicationUnitFor(runtime.GOOS, executablePath)
}

func installedApplicationUnitFor(goos, executablePath string) (unit, launchRelativePath string, err error) {
	executablePath, err = cleanAbsolutePath("application executable", executablePath)
	if err != nil {
		return "", "", err
	}
	resolvedExecutable, err := filepath.EvalSymlinks(executablePath)
	if err != nil {
		return "", "", fmt.Errorf("resolve application executable links: %w", err)
	}
	info, err := os.Lstat(resolvedExecutable)
	if err != nil {
		return "", "", fmt.Errorf("inspect application executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("application executable must be a regular file")
	}

	switch goos {
	case "windows":
		if filepath.Base(resolvedExecutable) != productDirectoryName+".exe" {
			return "", "", fmt.Errorf("unexpected Windows application executable")
		}
		unit = filepath.Dir(resolvedExecutable)
		launchRelativePath = productDirectoryName + ".exe"
	case "linux":
		if filepath.Base(resolvedExecutable) != productDirectoryName {
			return "", "", fmt.Errorf("unexpected Linux application executable")
		}
		unit = filepath.Dir(resolvedExecutable)
		launchRelativePath = productDirectoryName
	case "darwin":
		macOSDirectory := filepath.Dir(resolvedExecutable)
		contentsDirectory := filepath.Dir(macOSDirectory)
		unit = filepath.Dir(contentsDirectory)
		if filepath.Base(resolvedExecutable) != productDirectoryName ||
			filepath.Base(macOSDirectory) != "MacOS" ||
			filepath.Base(contentsDirectory) != "Contents" ||
			filepath.Ext(unit) != ".app" {
			return "", "", fmt.Errorf("application executable is outside a supported macOS bundle")
		}
		launchRelativePath = filepath.Join("Contents", "MacOS", productDirectoryName)
	default:
		return "", "", fmt.Errorf("unsupported operating system %q", goos)
	}

	unitInfo, err := os.Lstat(unit)
	if err != nil {
		return "", "", fmt.Errorf("inspect installed application unit: %w", err)
	}
	if unitInfo.Mode()&os.ModeSymlink != 0 || !unitInfo.IsDir() || filepath.Dir(unit) == unit {
		return "", "", fmt.Errorf("installed application unit must be a non-root directory and not a symbolic link")
	}
	return unit, launchRelativePath, nil
}

// DefaultSessionLocations resolves locations for the current user beneath
// resourceRoot. In a packaged build resourceRoot is the app's Contents/Resources
// directory; during development it is the repository root.
func DefaultSessionLocations(resourceRoot string) (SessionLocations, error) {
	return sessionLocationsFor(runtime.GOOS, nativeDirectoryProvider{}, resourceRoot)
}

// NewSessionLocations resolves deterministic session paths without touching
// the filesystem. Both inputs must be absolute so a later working-directory
// change cannot redirect user data or the bundled read-only sample.
func NewSessionLocations(homeDirectory, resourceRoot string) (SessionLocations, error) {
	return sessionLocationsFor("darwin", homeDirectoryProvider{home: homeDirectory}, resourceRoot)
}

func sessionLocationsFor(goos string, provider directoryProvider, resourceRoot string) (SessionLocations, error) {
	if provider == nil {
		return SessionLocations{}, fmt.Errorf("native directory provider is unavailable")
	}

	resourceRoot, err := cleanAbsolutePath("resource root", resourceRoot)
	if err != nil {
		return SessionLocations{}, err
	}

	documentsRoot, applicationDataRoot, err := storageRootsFor(goos, provider)
	if err != nil {
		return SessionLocations{}, err
	}

	locations := SessionLocations{
		DocumentsDefault:   filepath.Join(documentsRoot, productDirectoryName, sessionsDirectoryName),
		BundledDemo:        filepath.Join(resourceRoot, bundledSessionsDirectory, bundledDemoFilename),
		ApplicationSupport: filepath.Join(applicationDataRoot, applicationIdentifier),
	}
	if pathsOverlap(goos, resourceRoot, locations.DocumentsDefault) ||
		pathsOverlap(goos, resourceRoot, locations.ApplicationSupport) {
		return SessionLocations{}, fmt.Errorf("writable data must be separate from bundled resource root")
	}
	return locations, nil
}

func storageRootsFor(goos string, provider directoryProvider) (documents, applicationData string, err error) {
	switch goos {
	case "darwin":
		return darwinStorageRoots(provider)
	case "windows":
		return windowsStorageRoots(provider)
	case "linux":
		return linuxStorageRoots(provider)
	default:
		return "", "", fmt.Errorf("unsupported operating system %q", goos)
	}
}

func darwinStorageRoots(provider directoryProvider) (documents, applicationData string, err error) {
	homeDirectory, err := provider.HomeDirectory()
	if err != nil {
		return "", "", fmt.Errorf("resolve home directory: %w", err)
	}
	homeDirectory, err = cleanAbsolutePath("home directory", homeDirectory)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(homeDirectory, documentsDirectoryName),
		filepath.Join(homeDirectory, "Library", applicationSupportName), nil
}

func windowsStorageRoots(provider directoryProvider) (documents, applicationData string, err error) {
	documents, err = provider.DocumentsDirectory()
	if err != nil {
		return "", "", fmt.Errorf("resolve documents directory: %w", err)
	}
	documents, err = cleanAbsolutePath("documents directory", documents)
	if err != nil {
		return "", "", err
	}
	applicationData, err = provider.ApplicationDataDirectory()
	if err != nil {
		return "", "", fmt.Errorf("resolve application data directory: %w", err)
	}
	applicationData, err = cleanAbsolutePath("application data directory", applicationData)
	if err != nil {
		return "", "", err
	}
	return documents, applicationData, nil
}

func linuxStorageRoots(provider directoryProvider) (documents, applicationData string, err error) {
	documents, err = provider.DocumentsDirectory()
	if err != nil {
		return "", "", fmt.Errorf("resolve documents directory: %w", err)
	}
	applicationData, err = provider.ApplicationDataDirectory()
	if err != nil {
		return "", "", fmt.Errorf("resolve application data directory: %w", err)
	}

	if documents == "" || applicationData == "" {
		homeDirectory, homeErr := provider.HomeDirectory()
		if homeErr != nil {
			return "", "", fmt.Errorf("resolve home directory: %w", homeErr)
		}
		homeDirectory, homeErr = cleanAbsolutePath("home directory", homeDirectory)
		if homeErr != nil {
			return "", "", homeErr
		}
		if documents == "" {
			documents = filepath.Join(homeDirectory, documentsDirectoryName)
		}
		if applicationData == "" {
			applicationData = filepath.Join(homeDirectory, ".config")
		}
	}
	documents, err = cleanAbsolutePath("documents directory", documents)
	if err != nil {
		return "", "", err
	}
	applicationData, err = cleanAbsolutePath("application data directory", applicationData)
	if err != nil {
		return "", "", err
	}
	return documents, applicationData, nil
}

func pathsOverlap(goos, first, second string) bool {
	return pathContains(goos, first, second) || pathContains(goos, second, first)
}

func pathContains(goos, root, candidate string) bool {
	if goos == "windows" {
		root = strings.ToLower(root)
		candidate = strings.ToLower(candidate)
	}
	relative, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." || relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func cleanAbsolutePath(name, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%s is empty", name)
	}
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("%s must be absolute", name)
	}
	return cleaned, nil
}

// dialogLocation resolves a suggested native-dialog location without creating
// directories. Missing paths fall back to the nearest existing ancestor.
func dialogLocation(path string, pathIncludesFilename bool) (directory, filename string) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return "", ""
	}
	if pathIncludesFilename {
		directory = filepath.Dir(path)
		filename = filepath.Base(path)
	} else {
		directory = path
	}
	for directory != "" && directory != "." {
		info, err := os.Stat(directory)
		if err == nil && info.IsDir() {
			return directory, filename
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
		directory = parent
	}
	return "", filename
}
