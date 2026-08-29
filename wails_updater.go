package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	updateservice "github.com/obalunenko/Fallout-Terminal/v2/internal/update"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

const (
	applicationUpdateStatusEvent        = "application-update-status"
	applicationUpdateRepository         = "obalunenko/Fallout-Terminal"
	applicationGitHubAPIBaseURL         = "https://api.github.com"
	applicationGitHubReleasesPerPage    = 100
	applicationGitHubReleasePageLimit   = 100
	applicationGitHubReleaseResponseMax = 8 << 20
)

var (
	errWailsUpdaterHostRequired    = errors.New("wails updater host is required")
	errWailsUpdaterBackendRequired = errors.New("wails updater backend is required")
)

var applicationReleaseAssets = map[string]applicationReleaseTarget{
	"Fallout-Terminal-windows-amd64.zip":  {platform: "windows", arch: "amd64"},
	"Fallout-Terminal-windows-arm64.zip":  {platform: "windows", arch: "arm64"},
	"Fallout-Terminal-linux-amd64.tar.gz": {platform: "linux", arch: "amd64"},
	"Fallout-Terminal-linux-arm64.tar.gz": {platform: "linux", arch: "arm64"},
	"Fallout-Terminal-darwin-arm64.zip":   {platform: "darwin", arch: "arm64"},
}

type applicationGitHubProviderConfig struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
}

type applicationGitHubProvider struct {
	baseURL string
	client  *http.Client
	token   string
}

type applicationGitHubRelease struct {
	TagName     string                   `json:"tag_name"`
	Name        string                   `json:"name"`
	Body        string                   `json:"body"`
	Draft       bool                     `json:"draft"`
	Prerelease  bool                     `json:"prerelease"`
	PublishedAt time.Time                `json:"published_at"`
	Assets      []applicationGitHubAsset `json:"assets"`
}

type applicationGitHubAsset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	State              string `json:"state"`
	ContentType        string `json:"content_type"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type applicationReleaseTarget struct {
	platform string
	arch     string
}

// applicationUpdateBackendError retains an internal cause for classification
// while ensuring callers, logs, and command plumbing only see a stable message.
type applicationUpdateBackendError struct {
	message string
	cause   error
}

func (err applicationUpdateBackendError) Error() string { return err.message }

func (err applicationUpdateBackendError) Unwrap() error { return err.cause }

func sanitizedApplicationUpdateBackendError(message string, cause error) error {
	if cause == nil {
		return nil
	}
	return applicationUpdateBackendError{message: message, cause: cause}
}

type applicationSemanticVersion struct {
	major      string
	minor      string
	patch      string
	prerelease []string
}

type applicationEligibleRelease struct {
	release *applicationGitHubRelease
	version applicationSemanticVersion
}

func newApplicationGitHubProvider(config applicationGitHubProviderConfig) (updater.Provider, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = applicationGitHubAPIBaseURL
	}
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil || parsedBaseURL.Scheme == "" || parsedBaseURL.Host == "" {
		return nil, errors.New("configure application update service: invalid base URL")
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &applicationGitHubProvider{
		baseURL: baseURL,
		client:  client,
		token:   config.Token,
	}, nil
}

func (provider *applicationGitHubProvider) Name() string { return "github" }

func (provider *applicationGitHubProvider) Check(
	ctx context.Context,
	request updater.CheckRequest,
) (*updater.Release, error) {
	current, err := parseApplicationSemanticVersion(request.CurrentVersion, false)
	if err != nil {
		return nil, errors.New("check application updates: installed version is invalid")
	}
	releases, err := provider.fetchApplicationGitHubReleases(ctx)
	if err != nil {
		return nil, err
	}

	allowPrerelease := len(current.prerelease) > 0
	var eligible []applicationEligibleRelease
	for index := range releases {
		release := &releases[index]
		if release.Draft {
			continue
		}
		version, parseErr := parseApplicationSemanticVersion(release.TagName, true)
		if parseErr != nil || release.Prerelease != (len(version.prerelease) > 0) || release.PublishedAt.IsZero() {
			return nil, errors.New("check application updates: release metadata is invalid")
		}
		if !allowPrerelease && release.Prerelease {
			continue
		}
		if compareApplicationSemanticVersions(version, current) <= 0 {
			continue
		}
		eligible = append(eligible, applicationEligibleRelease{release: release, version: version})
	}
	if len(eligible) == 0 {
		return nil, nil
	}
	slices.SortFunc(eligible, func(left, right applicationEligibleRelease) int {
		return compareApplicationSemanticVersions(right.version, left.version)
	})
	selected := eligible[0].release

	asset, digest, err := selectApplicationReleaseAsset(*selected, request)
	if err != nil {
		return nil, err
	}
	channel := "stable"
	if selected.Prerelease {
		channel = "prerelease"
	}
	return &updater.Release{
		Version:     strings.TrimPrefix(selected.TagName, "v"),
		Channel:     channel,
		Name:        selected.Name,
		Notes:       applicationReleaseChangelog(eligible),
		PublishedAt: selected.PublishedAt,
		Artifact: updater.Artifact{
			Filename: asset.Name,
			Filetype: applicationReleaseFiletype(asset.Name),
			Size:     asset.Size,
			Platform: request.Platform,
			Arch:     request.Arch,
		},
		Verification: &updater.Verification{DigestAlgo: "sha256", Digest: digest},
		Metadata: map[string]any{
			"github.asset.id":          asset.ID,
			"github.asset.contentType": asset.ContentType,
			"github.asset.url":         asset.BrowserDownloadURL,
			"github.release.tag":       selected.TagName,
		},
	}, nil
}

func applicationReleaseChangelog(releases []applicationEligibleRelease) string {
	var changelog strings.Builder
	for _, candidate := range releases {
		if changelog.Len() > 0 {
			changelog.WriteString("\n\n")
		}
		changelog.WriteString("## Версия ")
		changelog.WriteString(strings.TrimPrefix(candidate.release.TagName, "v"))
		if strings.TrimSpace(candidate.release.Body) == "" {
			continue
		}
		changelog.WriteString("\n\n")
		changelog.WriteString(candidate.release.Body)
	}
	return changelog.String()
}

func (provider *applicationGitHubProvider) Download(
	ctx context.Context,
	release *updater.Release,
	destination io.Writer,
	onProgress func(written, total int64),
) error {
	if release == nil || release.Metadata == nil {
		return errors.New("download application update: release metadata is invalid")
	}
	downloadURL, ok := release.Metadata["github.asset.url"].(string)
	if !ok || !validApplicationDownloadURL(downloadURL) {
		return errors.New("download application update: release metadata is invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return errors.New("download application update: create asset request")
	}
	request.Header.Set("Accept", "application/octet-stream")
	provider.authorizeApplicationGitHubRequest(request)
	response, err := provider.doApplicationGitHubDownload(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("download application update: %w", ctxErr)
		}
		return errors.New("download application update: release service unavailable")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("download application update: release service returned HTTP %d", response.StatusCode)
	}
	total := release.Artifact.Size
	if total <= 0 && response.ContentLength > 0 {
		total = response.ContentLength
	}
	written := int64(0)
	buffer := make([]byte, 64*1024)
	for {
		count, readErr := response.Body.Read(buffer)
		if count > 0 {
			if _, writeErr := destination.Write(buffer[:count]); writeErr != nil {
				return errors.New("download application update: write artifact")
			}
			written += int64(count)
			if onProgress != nil {
				onProgress(written, total)
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return fmt.Errorf("download application update: %w", ctxErr)
			}
			return errors.New("download application update: read artifact")
		}
	}
}

func (provider *applicationGitHubProvider) doApplicationGitHubDownload(request *http.Request) (*http.Response, error) {
	client := *provider.client
	previousRedirectPolicy := client.CheckRedirect
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) > 0 && !strings.EqualFold(via[len(via)-1].URL.Host, next.URL.Host) {
			next.Header.Del("Authorization")
		}
		if previousRedirectPolicy != nil {
			return previousRedirectPolicy(next, via)
		}
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		return nil
	}
	return client.Do(request)
}

func (provider *applicationGitHubProvider) authorizeApplicationGitHubRequest(request *http.Request) {
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if provider.token != "" {
		request.Header.Set("Authorization", "Bearer "+provider.token)
	}
}

func (provider *applicationGitHubProvider) fetchApplicationGitHubReleases(
	ctx context.Context,
) ([]applicationGitHubRelease, error) {
	var releases []applicationGitHubRelease
	for page := 1; page <= applicationGitHubReleasePageLimit; page++ {
		pageReleases, err := provider.fetchApplicationGitHubReleasePage(ctx, page)
		if err != nil {
			return nil, err
		}
		releases = append(releases, pageReleases...)
		if len(pageReleases) < applicationGitHubReleasesPerPage {
			return releases, nil
		}
	}
	return nil, errors.New("check application updates: release history is too large")
}

func (provider *applicationGitHubProvider) fetchApplicationGitHubReleasePage(
	ctx context.Context,
	page int,
) ([]applicationGitHubRelease, error) {
	endpoint := fmt.Sprintf(
		"%s/repos/%s/releases?per_page=%d&page=%d",
		provider.baseURL,
		applicationUpdateRepository,
		applicationGitHubReleasesPerPage,
		page,
	)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.New("check application updates: create release request")
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	provider.authorizeApplicationGitHubRequest(request)
	response, err := provider.client.Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("check application updates: %w", ctxErr)
		}
		return nil, errors.New("check application updates: release service unavailable")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("check application updates: release service returned HTTP %d", response.StatusCode)
	}
	var releases []applicationGitHubRelease
	decoder := json.NewDecoder(io.LimitReader(response.Body, applicationGitHubReleaseResponseMax))
	if err := decoder.Decode(&releases); err != nil {
		return nil, errors.New("check application updates: release metadata is unreadable")
	}
	return releases, nil
}

func selectApplicationReleaseAsset(
	release applicationGitHubRelease,
	request updater.CheckRequest,
) (applicationGitHubAsset, []byte, error) {
	if len(release.Assets) != len(applicationReleaseAssets) {
		return applicationGitHubAsset{}, nil, errors.New("check application updates: release asset inventory is invalid")
	}
	assets := make(map[string]applicationGitHubAsset, len(release.Assets))
	digests := make(map[string][]byte, len(release.Assets))
	for _, asset := range release.Assets {
		target, governed := applicationReleaseAssets[asset.Name]
		if !governed || asset.State != "uploaded" || asset.ID <= 0 || asset.Size <= 0 ||
			asset.ContentType == "" || !validApplicationDownloadURL(asset.BrowserDownloadURL) {
			return applicationGitHubAsset{}, nil, errors.New("check application updates: release asset inventory is invalid")
		}
		if _, duplicate := assets[asset.Name]; duplicate {
			return applicationGitHubAsset{}, nil, errors.New("check application updates: release asset inventory is invalid")
		}
		if target.platform == "" || target.arch == "" {
			return applicationGitHubAsset{}, nil, errors.New("check application updates: release asset inventory is invalid")
		}
		digest, err := decodeApplicationGitHubDigest(asset.Digest)
		if err != nil {
			return applicationGitHubAsset{}, nil, errors.New("check application updates: release asset digest is invalid")
		}
		assets[asset.Name] = asset
		digests[asset.Name] = digest
	}

	matchedName := ""
	for name, target := range applicationReleaseAssets {
		if target.platform == request.Platform && target.arch == request.Arch {
			if matchedName != "" {
				return applicationGitHubAsset{}, nil, errors.New("check application updates: runtime target is ambiguous")
			}
			matchedName = name
		}
	}
	if matchedName == "" {
		return applicationGitHubAsset{}, nil, errors.New("check application updates: runtime target is unsupported")
	}
	asset, present := assets[matchedName]
	if !present {
		return applicationGitHubAsset{}, nil, errors.New("check application updates: release asset inventory is invalid")
	}
	return asset, digests[matchedName], nil
}

func decodeApplicationGitHubDigest(value string) ([]byte, error) {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+sha256.Size*2 {
		return nil, errors.New("invalid GitHub SHA-256 digest")
	}
	digest, err := hex.DecodeString(value[len(prefix):])
	if err != nil || len(digest) != sha256.Size {
		return nil, errors.New("invalid GitHub SHA-256 digest")
	}
	return digest, nil
}

func validApplicationDownloadURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "https" || parsed.Scheme == "http")
}

func applicationReleaseFiletype(name string) string {
	if strings.HasSuffix(name, ".tar.gz") {
		return "tar.gz"
	}
	return strings.TrimPrefix(strings.ToLower(filepathExtension(name)), ".")
}

func filepathExtension(name string) string {
	index := strings.LastIndexByte(name, '.')
	if index < 0 {
		return ""
	}
	return name[index:]
}

func parseApplicationSemanticVersion(value string, requireTag bool) (applicationSemanticVersion, error) {
	if requireTag {
		if !strings.HasPrefix(value, "v") {
			return applicationSemanticVersion{}, errors.New("semantic version tag must start with v")
		}
		value = strings.TrimPrefix(value, "v")
	} else if after, ok := strings.CutPrefix(value, "v"); ok {
		value = after
	}
	if value == "" || strings.Contains(value, "+") {
		return applicationSemanticVersion{}, errors.New("semantic version is not canonical")
	}
	core, prerelease, hasPrerelease := strings.Cut(value, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return applicationSemanticVersion{}, errors.New("semantic version core is invalid")
	}
	for _, part := range parts {
		if !isCanonicalNumericIdentifier(part) {
			return applicationSemanticVersion{}, errors.New("semantic version core is invalid")
		}
	}
	version := applicationSemanticVersion{major: parts[0], minor: parts[1], patch: parts[2]}
	if !hasPrerelease {
		return version, nil
	}
	identifiers := strings.Split(prerelease, ".")
	for _, identifier := range identifiers {
		if !isApplicationPrereleaseIdentifier(identifier) {
			return applicationSemanticVersion{}, errors.New("semantic version prerelease is invalid")
		}
		if isAllASCIIDigits(identifier) && !isCanonicalNumericIdentifier(identifier) {
			return applicationSemanticVersion{}, errors.New("semantic version prerelease is invalid")
		}
	}
	version.prerelease = identifiers
	return version, nil
}

func isCanonicalNumericIdentifier(value string) bool {
	return value != "" && isAllASCIIDigits(value) && (len(value) == 1 || value[0] != '0')
}

func isAllASCIIDigits(value string) bool {
	if value == "" {
		return false
	}
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func isApplicationPrereleaseIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if character != '-' && (character < '0' || character > '9') &&
			(character < 'A' || character > 'Z') && (character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func compareApplicationSemanticVersions(left, right applicationSemanticVersion) int {
	for _, pair := range [][2]string{{left.major, right.major}, {left.minor, right.minor}, {left.patch, right.patch}} {
		if comparison := compareApplicationNumericIdentifiers(pair[0], pair[1]); comparison != 0 {
			return comparison
		}
	}
	if len(left.prerelease) == 0 && len(right.prerelease) == 0 {
		return 0
	}
	if len(left.prerelease) == 0 {
		return 1
	}
	if len(right.prerelease) == 0 {
		return -1
	}
	for index := 0; index < min(len(left.prerelease), len(right.prerelease)); index++ {
		comparison := compareApplicationPrereleaseIdentifiers(left.prerelease[index], right.prerelease[index])
		if comparison != 0 {
			return comparison
		}
	}
	if len(left.prerelease) < len(right.prerelease) {
		return -1
	}
	if len(left.prerelease) > len(right.prerelease) {
		return 1
	}
	return 0
}

func compareApplicationPrereleaseIdentifiers(left, right string) int {
	leftNumeric := isAllASCIIDigits(left)
	rightNumeric := isAllASCIIDigits(right)
	if leftNumeric && rightNumeric {
		return compareApplicationNumericIdentifiers(left, right)
	}
	if leftNumeric {
		return -1
	}
	if rightNumeric {
		return 1
	}
	return strings.Compare(left, right)
}

func compareApplicationNumericIdentifiers(left, right string) int {
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return strings.Compare(left, right)
}

// wailsUpdaterBackend is the narrow portion of the pinned Wails updater used
// by application-owned update composition. Release policy and portable-package
// replacement remain outside the framework adapter.
type wailsUpdaterBackend interface {
	Init(updater.Config) error
	Check(context.Context) (*updater.Release, error)
	DownloadAndInstall(context.Context) error
	DownloadedPath() string
	StopPeriodicCheck()
}

// wailsUpdaterAdapter keeps the framework-owned updater behind a root seam so
// internal/update never imports Wails. The application update manager remains
// the owner of user-visible state and consent decisions.
type wailsUpdaterAdapter struct {
	backend wailsUpdaterBackend
	events  *applicationUpdaterHost
}

// newHeadlessWailsUpdater configures the updater owned by the Wails host. A
// zero CheckInterval disables periodic work and WindowNone prevents Wails from
// creating a second update UI.
func newHeadlessWailsUpdater(
	host *application.App,
	currentVersion string,
	providers ...updater.Provider,
) (*wailsUpdaterAdapter, error) {
	if host == nil {
		return nil, errWailsUpdaterHostRequired
	}
	return newHeadlessWailsUpdaterAdapter(host.Updater, currentVersion, providers...)
}

func newHeadlessWailsUpdaterAdapter(
	backend wailsUpdaterBackend,
	currentVersion string,
	providers ...updater.Provider,
) (*wailsUpdaterAdapter, error) {
	if backend == nil {
		return nil, errWailsUpdaterBackendRequired
	}
	if err := backend.Init(updater.Config{
		CurrentVersion: currentVersion,
		Providers:      slices.Clone(providers),
		Window:         updater.WindowNone,
	}); err != nil {
		return nil, sanitizedApplicationUpdateBackendError("configure Wails updater", err)
	}
	return &wailsUpdaterAdapter{backend: backend}, nil
}

func (adapter *wailsUpdaterAdapter) Check(ctx context.Context) (*updater.Release, error) {
	release, err := adapter.backend.Check(ctx)
	if err != nil {
		return nil, sanitizedApplicationUpdateBackendError("check application updates", err)
	}
	return release, nil
}

func (adapter *wailsUpdaterAdapter) DownloadAndInstall(ctx context.Context) error {
	return sanitizedApplicationUpdateBackendError(
		"download and verify application update", adapter.backend.DownloadAndInstall(ctx),
	)
}

func (adapter *wailsUpdaterAdapter) DownloadedPath() string {
	return adapter.backend.DownloadedPath()
}

func (adapter *wailsUpdaterAdapter) StopPeriodicCheck() {
	adapter.backend.StopPeriodicCheck()
}

// PrepareApplicationUpdate runs Wails' authenticated download, verification,
// and extraction pipeline, then hands the extracted tree to the
// application-owned manifest validator and same-volume stager.
func (adapter *wailsUpdaterAdapter) PrepareApplicationUpdate(
	ctx context.Context,
	candidate updateservice.UpdateCandidate,
	attemptID string,
	installedUnit string,
	installedLaunchRelativePath string,
	report func(updateservice.UpdateState, updateservice.UpdateProgress),
) (updateservice.PreparedApplicationUnit, error) {
	if adapter == nil || adapter.backend == nil || adapter.events == nil {
		return updateservice.PreparedApplicationUnit{}, errors.New("application update preparation is unavailable")
	}
	if ctx == nil {
		return updateservice.PreparedApplicationUnit{}, errors.New("application update preparation context is required")
	}

	var progressMu sync.Mutex
	progress := updateservice.UpdateProgress{}
	reportProgress := func(state updateservice.UpdateState, next *updateservice.UpdateProgress) {
		progressMu.Lock()
		if next != nil {
			progress = *next
		}
		current := progress
		progressMu.Unlock()
		if report != nil {
			report(state, current)
		}
	}
	removeProgressListener := adapter.events.OnEvent(updater.EventDownloadProgress, func(payload any) {
		if translated, ok := applicationUpdateProgressFromWails(payload); ok {
			reportProgress(updateservice.UpdateStateDownloading, &translated)
		}
	})
	defer removeProgressListener()
	removeVerifyingListener := adapter.events.OnEvent(updater.EventVerifying, func(any) {
		reportProgress(updateservice.UpdateStateVerifying, nil)
	})
	defer removeVerifyingListener()

	if err := adapter.DownloadAndInstall(ctx); err != nil {
		return updateservice.PreparedApplicationUnit{}, err
	}
	extractedRoot := adapter.DownloadedPath()
	if extractedRoot == "" {
		return updateservice.PreparedApplicationUnit{}, errors.New("application update extraction produced no package")
	}
	reportProgress(updateservice.UpdateStateStaging, nil)
	prepared, err := updateservice.PrepareApplicationUnit(ctx, updateservice.PrepareApplicationUnitRequest{
		Candidate:                   candidate,
		AttemptID:                   attemptID,
		ExtractedRoot:               extractedRoot,
		InstalledUnit:               installedUnit,
		InstalledLaunchRelativePath: installedLaunchRelativePath,
	})
	if safeApplicationUpdateExtractionRoot(extractedRoot) {
		_ = os.RemoveAll(extractedRoot)
	}
	if err != nil {
		return updateservice.PreparedApplicationUnit{}, sanitizedApplicationUpdateBackendError(
			"stage application update", err,
		)
	}
	return prepared, nil
}

func applicationUpdateProgressFromWails(payload any) (updateservice.UpdateProgress, bool) {
	var progress updater.Progress
	switch value := payload.(type) {
	case updater.Progress:
		progress = value
	case *updater.Progress:
		if value == nil {
			return updateservice.UpdateProgress{}, false
		}
		progress = *value
	default:
		return updateservice.UpdateProgress{}, false
	}
	if progress.Written < 0 {
		progress.Written = 0
	}
	result := updateservice.UpdateProgress{BytesDownloaded: uint64(progress.Written)}
	if progress.Total > 0 {
		if progress.Written > progress.Total {
			result.BytesDownloaded = uint64(progress.Total)
		}
		result.DownloadSize = uint64(progress.Total)
		result.DownloadSizeKnown = true
	}
	return result, true
}

func safeApplicationUpdateExtractionRoot(path string) bool {
	cleaned := filepath.Clean(path)
	if path == "" || cleaned != path || !filepath.IsAbs(cleaned) ||
		filepath.Dir(cleaned) == cleaned || filepath.Base(cleaned) != wailsApplicationName {
		return false
	}
	info, err := os.Lstat(cleaned)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0
}
