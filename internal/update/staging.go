package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

const (
	applicationName     = "Fallout Terminal"
	artifactManifest    = "artifact-manifest.json"
	stagingCopyFileMode = 0o600
)

type stageRequest struct {
	AttemptID          string
	Version            string
	Target             Target
	SourceUnit         string
	InstalledUnit      string
	LaunchRelativePath string
}

type stagingDependencies struct {
	copyTree  func(context.Context, string, string) error
	syncTree  func(context.Context, string) error
	removeAll func(string) error
}

// PrepareApplicationUnitRequest supplies the private, already authenticated
// release identity and the two filesystem roots needed to construct an
// adjacent replacement unit. Paths remain backend-only.
type PrepareApplicationUnitRequest struct {
	Candidate                   UpdateCandidate
	AttemptID                   string
	ExtractedRoot               string
	InstalledUnit               string
	InstalledLaunchRelativePath string
}

// PrepareApplicationUnit validates the extracted package identity and exact
// manifest inventory before copying its complete application unit beside the
// installed unit. The returned unit is durable and ready for helper handoff.
func PrepareApplicationUnit(
	ctx context.Context,
	request PrepareApplicationUnitRequest,
) (PreparedApplicationUnit, error) {
	if ctx == nil {
		return PreparedApplicationUnit{}, errors.New("application update preparation context is required")
	}
	if err := ctx.Err(); err != nil {
		return PreparedApplicationUnit{}, err
	}
	if err := validateExtractedManifest(ctx, request.ExtractedRoot, request.Candidate); err != nil {
		return PreparedApplicationUnit{}, fmt.Errorf("validate extracted application update: %w", err)
	}
	sourceUnit, launchRelativePath, err := selectReplacementUnit(
		request.ExtractedRoot,
		request.Candidate.Artifact.Target,
	)
	if err != nil {
		return PreparedApplicationUnit{}, err
	}
	if launchRelativePath != request.InstalledLaunchRelativePath {
		return PreparedApplicationUnit{}, errors.New("replacement and installed application launch paths disagree")
	}
	return stageApplicationUnit(ctx, stageRequest{
		AttemptID:          request.AttemptID,
		Version:            request.Candidate.Version,
		Target:             request.Candidate.Artifact.Target,
		SourceUnit:         sourceUnit,
		InstalledUnit:      request.InstalledUnit,
		LaunchRelativePath: launchRelativePath,
	}, stagingDependencies{})
}

type extractedArtifactManifest struct {
	SchemaVersion  int                             `json:"schemaVersion"`
	Product        string                          `json:"product"`
	Version        string                          `json:"version"`
	SourceRevision string                          `json:"sourceRevision"`
	Target         extractedArtifactManifestTarget `json:"target"`
	Runtime        string                          `json:"runtime"`
	Files          []extractedArtifactManifestFile `json:"files"`
}

type extractedArtifactManifestTarget struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type extractedArtifactManifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
}

type extractedApplicationFile struct {
	size   int64
	mode   string
	sha256 string
}

func validateExtractedManifest(ctx context.Context, root string, candidate UpdateCandidate) error {
	if err := validateTarget(candidate.Artifact.Target); err != nil {
		return err
	}
	if candidate.Version == "" {
		return errors.New("candidate version is empty")
	}
	if err := validateAbsoluteDirectory("extracted root", root); err != nil {
		return err
	}
	if err := validateRegularTree(root); err != nil {
		return fmt.Errorf("validate extracted root: %w", err)
	}

	manifest, err := loadExtractedArtifactManifest(root)
	if err != nil {
		return err
	}
	if err := validateExtractedManifestIdentity(manifest, candidate); err != nil {
		return err
	}

	actual, err := inspectExtractedApplicationFiles(ctx, root, candidate.Artifact.Target.OS == "windows")
	if err != nil {
		return err
	}
	required := requiredExtractedApplicationFiles(candidate.Artifact.Target)
	if err := validateExtractedApplicationShape(manifest, actual, required, candidate.Artifact.Target.OS == "windows"); err != nil {
		return err
	}
	return validateExtractedManifestFiles(manifest.Files, actual, candidate.Artifact.Target.OS == "windows")
}

func loadExtractedArtifactManifest(root string) (extractedArtifactManifest, error) {
	manifestPath := filepath.Join(root, artifactManifest)
	manifestInfo, err := os.Lstat(manifestPath)
	if err != nil || !manifestInfo.Mode().IsRegular() || manifestInfo.Mode().Perm() != 0o444 || manifestInfo.Size() > 1<<20 {
		return extractedArtifactManifest{}, errors.New("artifact manifest is unavailable")
	}
	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		return extractedArtifactManifest{}, errors.New("read artifact manifest")
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var manifest extractedArtifactManifest
	if err := decoder.Decode(&manifest); err != nil {
		return extractedArtifactManifest{}, errors.New("decode artifact manifest")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return extractedArtifactManifest{}, errors.New("decode artifact manifest")
	}
	return manifest, nil
}

func validateExtractedManifestIdentity(manifest extractedArtifactManifest, candidate UpdateCandidate) error {
	if manifest.SchemaVersion != 2 || manifest.Product != applicationName ||
		manifest.Version != candidate.Version ||
		manifest.Target.OS != candidate.Artifact.Target.OS ||
		manifest.Target.Arch != candidate.Artifact.Target.Arch ||
		manifest.Runtime != expectedApplicationRuntime(candidate.Artifact.Target) ||
		!validSourceRevision(manifest.SourceRevision) {
		return errors.New("artifact manifest identity does not match the selected release")
	}
	return nil
}

func validateExtractedApplicationShape(
	manifest extractedArtifactManifest,
	actual map[string]extractedApplicationFile,
	required map[string]string,
	foldCase bool,
) error {
	if len(manifest.Files) != len(actual) || len(actual) != len(required) {
		return errors.New("artifact manifest inventory does not match the extracted package")
	}
	for path, mode := range required {
		key := path
		if foldCase {
			key = strings.ToLower(key)
		}
		file, ok := actual[key]
		if !ok || file.size <= 0 || file.mode != mode {
			return errors.New("artifact package shape does not match the release contract")
		}
	}
	return nil
}

func validateExtractedManifestFiles(
	files []extractedArtifactManifestFile,
	actual map[string]extractedApplicationFile,
	foldCase bool,
) error {
	seen := make(map[string]struct{}, len(files))
	previous := ""
	for _, record := range files {
		if record.Path == "" || strings.ContainsAny(record.Path, "\\\x00") ||
			record.Path != filepath.ToSlash(filepath.Clean(record.Path)) ||
			record.Path == "." || record.Path == ".." || strings.HasPrefix(record.Path, "../") ||
			filepath.IsAbs(record.Path) || record.Size < 0 || !validManifestMode(record.Mode) ||
			!validLowerHex(record.SHA256, sha256.Size) ||
			foldCase && strings.Contains(record.Path, ":") {
			return errors.New("artifact manifest contains an invalid file record")
		}
		key := record.Path
		if foldCase {
			key = strings.ToLower(key)
		}
		if _, duplicate := seen[key]; duplicate || previous != "" && record.Path < previous {
			return errors.New("artifact manifest file inventory is duplicated or unsorted")
		}
		seen[key] = struct{}{}
		previous = record.Path
		file, ok := actual[key]
		if !ok || file.size != record.Size || file.mode != record.Mode || file.sha256 != record.SHA256 {
			return errors.New("artifact manifest file evidence does not match the extracted package")
		}
	}
	return nil
}

func inspectExtractedApplicationFiles(
	ctx context.Context,
	root string,
	foldCase bool,
) (map[string]extractedApplicationFile, error) {
	files := make(map[string]extractedApplicationFile)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == artifactManifest {
			return nil
		}
		key := relative
		if foldCase {
			key = strings.ToLower(key)
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("extracted application contains an unsupported file type")
		}
		file, err := os.Open(path)
		if err != nil {
			return errors.New("read extracted application file")
		}
		digest := sha256.New()
		_, copyErr := io.Copy(digest, &contextReader{ctx: ctx, reader: file})
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			return errors.New("hash extracted application file")
		}
		if _, duplicate := files[key]; duplicate {
			return errors.New("extracted application contains colliding file paths")
		}
		files[key] = extractedApplicationFile{
			size: info.Size(), mode: fmt.Sprintf("%04o", info.Mode().Perm()),
			sha256: hex.EncodeToString(digest.Sum(nil)),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

func expectedApplicationRuntime(target Target) string {
	switch target.OS {
	case "windows":
		return "WebView2"
	case "linux":
		return "GTK4/WebKitGTK 6.0 and Secret Service"
	case "darwin":
		return "WebKit and Apple Keychain"
	default:
		return ""
	}
}

func requiredExtractedApplicationFiles(target Target) map[string]string {
	if target.OS == "darwin" {
		return map[string]string{
			"Fallout Terminal.app/Contents/Info.plist":                           "0444",
			"Fallout Terminal.app/Contents/MacOS/Fallout Terminal":               "0755",
			"Fallout Terminal.app/Contents/Resources/THIRD_PARTY_NOTICES.md":     "0444",
			"Fallout Terminal.app/Contents/Resources/icon.icns":                  "0444",
			"Fallout Terminal.app/Contents/Resources/sessions/demo-players.json": "0444",
			"Fallout Terminal.app/Contents/Resources/sessions/demo.json":         "0444",
		}
	}
	executable := "Fallout Terminal"
	if target.OS == "windows" {
		executable += ".exe"
	}
	return map[string]string{
		executable:                             "0755",
		"resources/THIRD_PARTY_NOTICES.md":     "0444",
		"resources/appicon.png":                "0444",
		"resources/sessions/demo-players.json": "0444",
		"resources/sessions/demo.json":         "0444",
	}
}

func validSourceRevision(value string) bool { return validLowerHex(value, 20) }

func validLowerHex(value string, bytes int) bool {
	if len(value) != bytes*2 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func validManifestMode(value string) bool {
	if len(value) != 4 || value[0] != '0' {
		return false
	}
	for _, character := range value[1:] {
		if character < '0' || character > '7' {
			return false
		}
	}
	return true
}

func selectReplacementUnit(extractedRoot string, target Target) (string, string, error) {
	if err := validateTarget(target); err != nil {
		return "", "", err
	}
	if err := validateAbsoluteDirectory("extracted root", extractedRoot); err != nil {
		return "", "", err
	}
	if err := validateRegularTree(extractedRoot); err != nil {
		return "", "", fmt.Errorf("validate extracted root: %w", err)
	}
	if err := validateRegularFile(filepath.Join(extractedRoot, artifactManifest)); err != nil {
		return "", "", fmt.Errorf("validate artifact manifest: %w", err)
	}

	var unit, launchRelativePath string
	switch target.OS {
	case "windows":
		unit = extractedRoot
		launchRelativePath = applicationName + ".exe"
	case "linux":
		unit = extractedRoot
		launchRelativePath = applicationName
	case "darwin":
		unit = filepath.Join(extractedRoot, applicationName+".app")
		launchRelativePath = filepath.Join("Contents", "MacOS", applicationName)
	default:
		return "", "", fmt.Errorf("unsupported update operating system %q", target.OS)
	}

	if err := validateAbsoluteDirectory("replacement unit", unit); err != nil {
		return "", "", err
	}
	if err := validateLaunchPath(unit, launchRelativePath); err != nil {
		return "", "", err
	}
	return unit, launchRelativePath, nil
}

func stageApplicationUnit(
	ctx context.Context,
	request stageRequest,
	dependencies stagingDependencies,
) (PreparedApplicationUnit, error) {
	if ctx == nil {
		return PreparedApplicationUnit{}, errors.New("staging context is required")
	}
	if err := ctx.Err(); err != nil {
		return PreparedApplicationUnit{}, err
	}
	if err := validateStageRequest(request); err != nil {
		return PreparedApplicationUnit{}, err
	}

	dependencies = withDefaultStagingDependencies(dependencies)
	if dependencies.copyTree == nil || dependencies.syncTree == nil || dependencies.removeAll == nil {
		return PreparedApplicationUnit{}, errors.New("staging dependencies are incomplete")
	}

	stagedUnit, err := os.MkdirTemp(
		filepath.Dir(request.InstalledUnit),
		"."+filepath.Base(request.InstalledUnit)+".update-"+request.AttemptID+"-",
	)
	if err != nil {
		return PreparedApplicationUnit{}, fmt.Errorf("create adjacent update stage: %w", err)
	}

	cleanupFailure := func(stageErr error) (PreparedApplicationUnit, error) {
		if cleanupErr := dependencies.removeAll(stagedUnit); cleanupErr != nil {
			stageErr = errors.Join(stageErr, fmt.Errorf("remove incomplete update stage: %w", cleanupErr))
		}
		return PreparedApplicationUnit{}, stageErr
	}

	if err := dependencies.copyTree(ctx, request.SourceUnit, stagedUnit); err != nil {
		return cleanupFailure(fmt.Errorf("copy application unit: %w", err))
	}
	if err := validateRegularTree(stagedUnit); err != nil {
		return cleanupFailure(fmt.Errorf("validate staged application unit: %w", err))
	}
	if err := validateLaunchPath(stagedUnit, request.LaunchRelativePath); err != nil {
		return cleanupFailure(fmt.Errorf("validate staged application launch path: %w", err))
	}
	if err := dependencies.syncTree(ctx, stagedUnit); err != nil {
		return cleanupFailure(fmt.Errorf("sync staged application unit: %w", err))
	}

	return PreparedApplicationUnit{
		AttemptID:          request.AttemptID,
		Version:            request.Version,
		Target:             request.Target,
		InstalledUnit:      request.InstalledUnit,
		StagedUnit:         stagedUnit,
		LaunchRelativePath: request.LaunchRelativePath,
	}, nil
}

func validateStageRequest(request stageRequest) error {
	if err := validateAttemptID(request.AttemptID); err != nil {
		return err
	}
	if strings.TrimSpace(request.Version) == "" {
		return errors.New("update version is empty")
	}
	if err := validateTarget(request.Target); err != nil {
		return err
	}
	if err := validateAbsoluteDirectory("source application unit", request.SourceUnit); err != nil {
		return err
	}
	if err := validateRegularTree(request.SourceUnit); err != nil {
		return fmt.Errorf("validate source application unit: %w", err)
	}
	if err := validateAbsoluteDirectory("installed application unit", request.InstalledUnit); err != nil {
		return err
	}
	if samePath(request.SourceUnit, request.InstalledUnit) ||
		pathContains(request.SourceUnit, request.InstalledUnit) ||
		pathContains(request.InstalledUnit, request.SourceUnit) {
		return errors.New("source and installed application units must be separate")
	}
	if err := validateLaunchPath(request.SourceUnit, request.LaunchRelativePath); err != nil {
		return err
	}
	return nil
}

func validateTarget(target Target) error {
	switch target.OS {
	case "windows", "linux":
		if target.Arch == "amd64" || target.Arch == "arm64" {
			return nil
		}
	case "darwin":
		if target.Arch == "arm64" {
			return nil
		}
	default:
		return fmt.Errorf("unsupported update operating system %q", target.OS)
	}
	return fmt.Errorf("unsupported update target %s/%s", target.OS, target.Arch)
}

func validateAttemptID(attemptID string) error {
	if attemptID == "" {
		return errors.New("update attempt identifier is empty")
	}
	for _, character := range attemptID {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' {
			continue
		}
		return errors.New("update attempt identifier contains an unsafe character")
	}
	return nil
}

func validateAbsoluteDirectory(name, path string) error {
	if path == "" {
		return fmt.Errorf("%s is empty", name)
	}
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return fmt.Errorf("%s must be absolute", name)
	}
	if isFilesystemRoot(cleaned) {
		return fmt.Errorf("%s must not be a filesystem root", name)
	}
	if cleaned != path {
		return fmt.Errorf("%s must be clean", name)
	}
	info, err := os.Lstat(cleaned)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s must be a directory and not a symbolic link", name)
	}
	return nil
}

func validateRegularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("path must identify a regular file")
	}
	return nil
}

func validateRegularTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect tree entry: %w", err)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("tree entry %q is not a regular file or directory", filepath.Base(path))
		}
		return nil
	})
}

func validateLaunchPath(unit, launchRelativePath string) error {
	if launchRelativePath == "" {
		return errors.New("application launch path is empty")
	}
	cleaned := filepath.Clean(launchRelativePath)
	if cleaned != launchRelativePath || cleaned == "." || filepath.IsAbs(cleaned) {
		return errors.New("application launch path must be a clean relative path")
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return errors.New("application launch path must not traverse its application unit")
	}
	return validateRegularFile(filepath.Join(unit, cleaned))
}

func isFilesystemRoot(path string) bool {
	volume := filepath.VolumeName(path)
	return filepath.Clean(path) == filepath.Clean(volume+string(filepath.Separator))
}

func pathContains(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != "." && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func samePath(first, second string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(first), filepath.Clean(second))
	}
	return filepath.Clean(first) == filepath.Clean(second)
}

func withDefaultStagingDependencies(dependencies stagingDependencies) stagingDependencies {
	if dependencies.copyTree == nil {
		dependencies.copyTree = copyApplicationTree
	}
	if dependencies.syncTree == nil {
		dependencies.syncTree = syncApplicationTree
	}
	if dependencies.removeAll == nil {
		dependencies.removeAll = os.RemoveAll
	}
	return dependencies
}

func copyApplicationTree(ctx context.Context, source, destination string) error {
	type directoryMode struct {
		path string
		mode fs.FileMode
	}
	directories := make([]directoryMode, 0, 8)
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect application entry: %w", err)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("resolve application entry: %w", err)
		}
		target := destination
		if relative != "." {
			target = filepath.Join(destination, relative)
		}
		switch {
		case info.IsDir():
			if relative == "." {
				directories = append(directories, directoryMode{path: target, mode: info.Mode().Perm()})
				return nil
			}
			if err := os.Mkdir(target, 0o700); err != nil {
				return err
			}
			directories = append(directories, directoryMode{path: target, mode: info.Mode().Perm()})
			return nil
		case info.Mode().IsRegular():
			return copyApplicationFile(ctx, path, target, info.Mode().Perm())
		default:
			return errors.New("application unit contains a non-regular entry")
		}
	})
	if err != nil {
		return err
	}
	for index := len(directories) - 1; index >= 0; index-- {
		if err := os.Chmod(directories[index].path, directories[index].mode); err != nil {
			return err
		}
	}
	return nil
}

func copyApplicationFile(ctx context.Context, source, destination string, mode fs.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, stagingCopyFileMode)
	if err != nil {
		_ = input.Close()
		return err
	}

	buffer := make([]byte, 128*1024)
	for {
		if err := ctx.Err(); err != nil {
			_ = output.Close()
			_ = input.Close()
			return err
		}
		read, readErr := input.Read(buffer)
		if read > 0 {
			if _, err := output.Write(buffer[:read]); err != nil {
				_ = output.Close()
				_ = input.Close()
				return err
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				_ = output.Close()
				_ = input.Close()
				return readErr
			}
			break
		}
	}
	if err := input.Close(); err != nil {
		_ = output.Close()
		return err
	}
	if err := output.Chmod(mode); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func syncApplicationTree(ctx context.Context, root string) error {
	directories := make([]string, 0, 8)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			directories = append(directories, path)
			return nil
		}
		return syncApplicationPath(path)
	})
	if err != nil {
		return err
	}
	for index := len(directories) - 1; index >= 0; index-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := syncApplicationPath(directories[index]); err != nil && !syncUnsupported(err) {
			return err
		}
	}
	return nil
}

func syncApplicationPath(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	return errors.Join(syncErr, closeErr)
}

func syncUnsupported(err error) bool {
	return errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.ENOSYS)
}
