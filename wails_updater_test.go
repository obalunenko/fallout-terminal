package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

const applicationUpdateRepositoryPath = "/repos/obalunenko/Fallout-Terminal/releases"

type githubReleaseFixture struct {
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	Body        string               `json:"body"`
	Draft       bool                 `json:"draft"`
	Prerelease  bool                 `json:"prerelease"`
	PublishedAt string               `json:"published_at"`
	HTMLURL     string               `json:"html_url"`
	Assets      []githubAssetFixture `json:"assets"`
}

type githubAssetFixture struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	State              string `json:"state"`
	ContentType        string `json:"content_type"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type writerFunc func([]byte) (int, error)

func (function writerFunc) Write(payload []byte) (int, error) {
	return function(payload)
}

func TestApplicationGitHubProviderSelectsGreatestEligibleSemanticVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		installedVersion string
		releases         []githubReleaseFixture
		wantVersion      string
		wantChannel      string
	}{
		{
			name:             "stable installation ignores newer prereleases and compares numeric components",
			installedVersion: "2.8.0",
			releases: []githubReleaseFixture{
				newGitHubReleaseFixture("v2.11.0-beta.1", true),
				newGitHubReleaseFixture("v2.9.0", false),
				newGitHubReleaseFixture("v2.10.0", false),
			},
			wantVersion: "2.10.0",
			wantChannel: "stable",
		},
		{
			name:             "prerelease installation accepts the greatest newer prerelease",
			installedVersion: "2.10.0-beta.2",
			releases: []githubReleaseFixture{
				newGitHubReleaseFixture("v2.10.0-beta.3", true),
				newGitHubReleaseFixture("v2.10.0-beta.11", true),
				newGitHubReleaseFixture("v2.10.0-rc.1", true),
				newGitHubReleaseFixture("v2.9.99", false),
			},
			wantVersion: "2.10.0-rc.1",
			wantChannel: "prerelease",
		},
		{
			name:             "stable release outranks prereleases with the same numeric core",
			installedVersion: "2.12.0-beta.1",
			releases: []githubReleaseFixture{
				newGitHubReleaseFixture("v2.12.0-rc.9", true),
				newGitHubReleaseFixture("v2.12.0", false),
				newGitHubReleaseFixture("v2.12.0-beta.10", true),
			},
			wantVersion: "2.12.0",
			wantChannel: "stable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			provider := newGitHubProviderFixture(t, test.releases)
			release, err := provider.Check(t.Context(), updater.CheckRequest{
				CurrentVersion: test.installedVersion,
				Platform:       "linux",
				Arch:           "amd64",
			})
			require.NoError(t, err)
			require.NotNil(t, release)
			assert.Equal(t, test.wantVersion, release.Version)
			assert.Equal(t, test.wantChannel, release.Channel)
			assert.Equal(t, "Fallout-Terminal-linux-amd64.tar.gz", release.Artifact.Filename)
		})
	}
}

func TestApplicationGitHubProviderReturnsCurrentWithoutNewerEligibleRelease(t *testing.T) {
	t.Parallel()

	provider := newGitHubProviderFixture(t, []githubReleaseFixture{
		newGitHubReleaseFixture("v2.9.0-rc.1", true),
		newGitHubReleaseFixture("v2.8.0", false),
	})
	release, err := provider.Check(t.Context(), updater.CheckRequest{
		CurrentVersion: "2.9.0",
		Platform:       "linux",
		Arch:           "amd64",
	})
	require.NoError(t, err)
	assert.Nil(t, release)
}

func TestApplicationGitHubProviderRequiresExactFiveAssetInventory(t *testing.T) {
	t.Parallel()

	base := newGitHubReleaseFixture("v2.3.0", false)
	tests := []struct {
		name   string
		mutate func(*githubReleaseFixture)
	}{
		{
			name: "missing target archive",
			mutate: func(release *githubReleaseFixture) {
				release.Assets = release.Assets[:len(release.Assets)-1]
			},
		},
		{
			name: "duplicate target archive",
			mutate: func(release *githubReleaseFixture) {
				release.Assets[len(release.Assets)-1] = release.Assets[0]
			},
		},
		{
			name: "extra sidecar",
			mutate: func(release *githubReleaseFixture) {
				release.Assets = append(release.Assets, githubAssetFixture{
					ID: 99, Name: "SHA256SUMS", State: "uploaded", Size: 64,
					Digest: "sha256:" + strings.Repeat("99", sha256.Size),
				})
			},
		},
		{
			name: "empty inventory",
			mutate: func(release *githubReleaseFixture) {
				release.Assets = nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			releaseFixture := cloneGitHubReleaseFixture(base)
			test.mutate(&releaseFixture)
			provider := newGitHubProviderFixture(t, []githubReleaseFixture{releaseFixture})
			release, err := provider.Check(t.Context(), updater.CheckRequest{
				CurrentVersion: "2.2.0",
				Platform:       "windows",
				Arch:           "amd64",
			})
			require.Error(t, err)
			assert.Nil(t, release)
		})
	}
}

func TestApplicationGitHubProviderParsesGitHubSHA256AndInjectsWailsVerification(t *testing.T) {
	t.Parallel()

	releaseFixture := newGitHubReleaseFixture("v2.4.0", false)
	selected := findGitHubAssetFixture(t, releaseFixture.Assets, "Fallout-Terminal-darwin-arm64.zip")
	selected.Digest = "sha256:" + strings.ToUpper(strings.Repeat("a7", sha256.Size))
	replaceGitHubAssetFixture(t, releaseFixture.Assets, selected)

	provider := newGitHubProviderFixture(t, []githubReleaseFixture{releaseFixture})
	release, err := provider.Check(t.Context(), updater.CheckRequest{
		CurrentVersion: "2.3.0",
		Platform:       "darwin",
		Arch:           "arm64",
	})
	require.NoError(t, err)
	require.NotNil(t, release)
	require.NotNil(t, release.Verification)

	wantDigest, err := hex.DecodeString(strings.Repeat("a7", sha256.Size))
	require.NoError(t, err)
	assert.Equal(t, "sha256", release.Verification.DigestAlgo)
	assert.Equal(t, wantDigest, release.Verification.Digest)
	assert.Equal(t, selected.Name, release.Artifact.Filename)
	assert.Equal(t, selected.Size, release.Artifact.Size)
	assert.Equal(t, "darwin", release.Artifact.Platform)
	assert.Equal(t, "arm64", release.Artifact.Arch)
}

func TestApplicationGitHubProviderRequiresCanonicalSHA256ForEveryGovernedAsset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		digest string
	}{
		{name: "missing", digest: ""},
		{name: "wrong algorithm", digest: "sha512:" + strings.Repeat("ab", sha256.Size)},
		{name: "uppercase algorithm", digest: "SHA256:" + strings.Repeat("ab", sha256.Size)},
		{name: "short", digest: "sha256:" + strings.Repeat("ab", sha256.Size-1)},
		{name: "long", digest: "sha256:" + strings.Repeat("ab", sha256.Size+1)},
		{name: "non hex", digest: "sha256:" + strings.Repeat("gg", sha256.Size)},
		{name: "surrounding whitespace", digest: " sha256:" + strings.Repeat("ab", sha256.Size) + " "},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			releaseFixture := newGitHubReleaseFixture("v2.4.0", false)
			unselected := findGitHubAssetFixture(t, releaseFixture.Assets, "Fallout-Terminal-windows-arm64.zip")
			unselected.Digest = test.digest
			replaceGitHubAssetFixture(t, releaseFixture.Assets, unselected)

			provider := newGitHubProviderFixture(t, []githubReleaseFixture{releaseFixture})
			release, err := provider.Check(t.Context(), updater.CheckRequest{
				CurrentVersion: "2.3.0", Platform: "linux", Arch: "amd64",
			})
			require.EqualError(t, err, "check application updates: release asset digest is invalid")
			assert.Nil(t, release)
		})
	}
}

func TestApplicationGitHubProviderRejectsAmbiguousOrIncompleteGovernedInventory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*githubReleaseFixture)
	}{
		{
			name: "selected archive appears twice while another target is absent",
			mutate: func(release *githubReleaseFixture) {
				selected := release.Assets[2]
				release.Assets[len(release.Assets)-1] = selected
			},
		},
		{
			name: "case variant cannot substitute for exact target archive",
			mutate: func(release *githubReleaseFixture) {
				release.Assets[0].Name = "fallout-terminal-windows-amd64.zip"
			},
		},
		{
			name: "all five entries cannot come from only four targets",
			mutate: func(release *githubReleaseFixture) {
				release.Assets[4] = release.Assets[3]
				release.Assets[4].ID = 99
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			releaseFixture := newGitHubReleaseFixture("v2.5.0", false)
			test.mutate(&releaseFixture)
			provider := newGitHubProviderFixture(t, []githubReleaseFixture{releaseFixture})
			release, err := provider.Check(t.Context(), updater.CheckRequest{
				CurrentVersion: "2.4.0", Platform: "linux", Arch: "amd64",
			})
			require.EqualError(t, err, "check application updates: release asset inventory is invalid")
			assert.Nil(t, release)
		})
	}
}

func TestApplicationGitHubProviderErrorsRedactURLsTokensAndLocalPaths(t *testing.T) {
	t.Parallel()

	const (
		secretURL   = "https://updates.example.invalid/archive.zip?token=github_pat_update_canary"
		secretToken = "github_pat_update_canary"
		secretPath  = "/Users/update-canary-account/Downloads/Fallout-Terminal.zip"
	)
	assertRedacted := func(t *testing.T, err error) {
		t.Helper()
		require.Error(t, err)
		for _, secret := range []string{secretURL, secretToken, secretPath, "update-canary-account"} {
			assert.NotContains(t, err.Error(), secret)
		}
	}

	t.Run("release discovery transport", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New(secretURL + " " + secretToken + " " + secretPath)
		})}
		provider, err := newApplicationGitHubProvider(applicationGitHubProviderConfig{
			BaseURL: "https://api.example.invalid", HTTPClient: client, Token: secretToken,
		})
		require.NoError(t, err)
		_, checkErr := provider.Check(t.Context(), updater.CheckRequest{
			CurrentVersion: "2.3.0", Platform: "linux", Arch: "amd64",
		})
		assertRedacted(t, checkErr)
	})

	t.Run("asset download transport", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New(secretURL + " " + secretToken + " " + secretPath)
		})}
		provider, err := newApplicationGitHubProvider(applicationGitHubProviderConfig{
			BaseURL: "https://api.example.invalid", HTTPClient: client, Token: secretToken,
		})
		require.NoError(t, err)
		release := &updater.Release{
			Artifact: updater.Artifact{Size: 1024},
			Metadata: map[string]any{"github.asset.url": secretURL},
		}
		downloadErr := provider.Download(t.Context(), release, writerFunc(func(payload []byte) (int, error) {
			return len(payload), nil
		}), nil)
		assertRedacted(t, downloadErr)
	})

	t.Run("artifact destination", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			_, _ = response.Write([]byte("archive"))
		}))
		t.Cleanup(server.Close)
		provider, err := newApplicationGitHubProvider(applicationGitHubProviderConfig{
			BaseURL: server.URL, HTTPClient: server.Client(), Token: secretToken,
		})
		require.NoError(t, err)
		release := &updater.Release{
			Metadata: map[string]any{"github.asset.url": server.URL + "/archive.zip?token=" + secretToken},
		}
		downloadErr := provider.Download(t.Context(), release, writerFunc(func([]byte) (int, error) {
			return 0, errors.New(secretPath)
		}), nil)
		assertRedacted(t, downloadErr)
		assert.NotContains(t, downloadErr.Error(), server.URL)
	})
}

func TestApplicationGitHubProviderMatchesExactlyOneGovernedRuntimeAsset(t *testing.T) {
	t.Parallel()

	releaseFixture := newGitHubReleaseFixture("v2.5.0", false)
	for _, test := range []struct {
		platform string
		arch     string
		filename string
	}{
		{platform: "windows", arch: "amd64", filename: "Fallout-Terminal-windows-amd64.zip"},
		{platform: "windows", arch: "arm64", filename: "Fallout-Terminal-windows-arm64.zip"},
		{platform: "linux", arch: "amd64", filename: "Fallout-Terminal-linux-amd64.tar.gz"},
		{platform: "linux", arch: "arm64", filename: "Fallout-Terminal-linux-arm64.tar.gz"},
		{platform: "darwin", arch: "arm64", filename: "Fallout-Terminal-darwin-arm64.zip"},
	} {
		t.Run(test.platform+"_"+test.arch, func(t *testing.T) {
			t.Parallel()

			provider := newGitHubProviderFixture(t, []githubReleaseFixture{releaseFixture})
			release, err := provider.Check(t.Context(), updater.CheckRequest{
				CurrentVersion: "2.4.0",
				Platform:       test.platform,
				Arch:           test.arch,
			})
			require.NoError(t, err)
			require.NotNil(t, release)
			assert.Equal(t, test.filename, release.Artifact.Filename)
		})
	}

	t.Run("unsupported runtime has no match", func(t *testing.T) {
		t.Parallel()

		provider := newGitHubProviderFixture(t, []githubReleaseFixture{releaseFixture})
		release, err := provider.Check(t.Context(), updater.CheckRequest{
			CurrentVersion: "2.4.0",
			Platform:       "darwin",
			Arch:           "amd64",
		})
		require.Error(t, err)
		assert.Nil(t, release)
	})
}

func TestApplicationGitHubProviderSkipsDraftsAndRejectsMalformedReleaseMetadata(t *testing.T) {
	t.Parallel()

	t.Run("draft does not hide the greatest published release", func(t *testing.T) {
		t.Parallel()

		draft := newGitHubReleaseFixture("v2.9.0", false)
		draft.Draft = true
		provider := newGitHubProviderFixture(t, []githubReleaseFixture{
			draft,
			newGitHubReleaseFixture("v2.7.0", false),
			newGitHubReleaseFixture("v2.8.0", false),
		})
		release, err := provider.Check(t.Context(), updater.CheckRequest{
			CurrentVersion: "2.6.0", Platform: "linux", Arch: "amd64",
		})
		require.NoError(t, err)
		require.NotNil(t, release)
		assert.Equal(t, "2.8.0", release.Version)
	})

	for _, test := range []struct {
		name   string
		mutate func(*githubReleaseFixture)
	}{
		{
			name: "noncanonical tag",
			mutate: func(release *githubReleaseFixture) {
				release.TagName = "2.8.0"
			},
		},
		{
			name: "build metadata",
			mutate: func(release *githubReleaseFixture) {
				release.TagName = "v2.8.0+rebuilt"
			},
		},
		{
			name: "leading zero prerelease identifier",
			mutate: func(release *githubReleaseFixture) {
				release.TagName = "v2.8.0-rc.01"
				release.Prerelease = true
			},
		},
		{
			name: "tag and prerelease flag disagree",
			mutate: func(release *githubReleaseFixture) {
				release.TagName = "v2.8.0-rc.1"
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			releaseFixture := newGitHubReleaseFixture("v2.8.0", false)
			test.mutate(&releaseFixture)
			provider := newGitHubProviderFixture(t, []githubReleaseFixture{releaseFixture})
			release, err := provider.Check(t.Context(), updater.CheckRequest{
				CurrentVersion: "2.7.0", Platform: "linux", Arch: "amd64",
			})
			require.Error(t, err)
			assert.Nil(t, release)
		})
	}
}

func TestApplicationGitHubProviderPropagatesCancellation(t *testing.T) {
	started := make(chan struct{})
	var startOnce sync.Once
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		startOnce.Do(func() { close(started) })
		<-request.Context().Done()
	}))
	t.Cleanup(server.Close)

	provider, err := newApplicationGitHubProvider(applicationGitHubProviderConfig{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	result := make(chan error, 1)
	go func() {
		_, checkErr := provider.Check(ctx, updater.CheckRequest{
			CurrentVersion: "2.1.0", Platform: "linux", Arch: "amd64",
		})
		result <- checkErr
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("provider check did not reach the release service")
	}
	cancel()
	select {
	case checkErr := <-result:
		require.ErrorIs(t, checkErr, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("provider check did not return after cancellation")
	}
}

func TestApplicationUpdateProgressFromWailsSanitizesBackendPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		payload   any
		wantBytes uint64
		wantSize  uint64
		wantKnown bool
		wantOK    bool
	}{
		{name: "value", payload: updater.Progress{Written: 512, Total: 4096}, wantBytes: 512, wantSize: 4096, wantKnown: true, wantOK: true},
		{name: "pointer", payload: &updater.Progress{Written: 256}, wantBytes: 256, wantOK: true},
		{name: "negative values", payload: updater.Progress{Written: -1, Total: -1}, wantOK: true},
		{name: "written beyond total", payload: updater.Progress{Written: 8192, Total: 4096}, wantBytes: 4096, wantSize: 4096, wantKnown: true, wantOK: true},
		{name: "unknown payload", payload: "provider detail", wantOK: false},
		{name: "nil pointer", payload: (*updater.Progress)(nil), wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			progress, ok := applicationUpdateProgressFromWails(test.payload)
			assert.Equal(t, test.wantOK, ok)
			assert.Equal(t, test.wantBytes, progress.BytesDownloaded)
			assert.Equal(t, test.wantSize, progress.DownloadSize)
			assert.Equal(t, test.wantKnown, progress.DownloadSizeKnown)
		})
	}
}

func TestSafeApplicationUpdateExtractionRootRejectsUnsafeCleanupTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	extracted := filepath.Join(root, wailsApplicationName)
	require.NoError(t, os.MkdirAll(extracted, 0o755))
	assert.True(t, safeApplicationUpdateExtractionRoot(extracted))
	assert.False(t, safeApplicationUpdateExtractionRoot(root))
	assert.False(t, safeApplicationUpdateExtractionRoot(string(filepath.Separator)))
	assert.False(t, safeApplicationUpdateExtractionRoot(filepath.Join(root, "missing", wailsApplicationName)))
}

func newGitHubProviderFixture(t *testing.T, releases []githubReleaseFixture) updater.Provider {
	t.Helper()

	var serverURL string
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != applicationUpdateRepositoryPath {
			http.Error(response, "unexpected release request", http.StatusNotFound)
			return
		}
		fixtures := cloneGitHubReleaseFixtures(releases)
		for releaseIndex := range fixtures {
			for assetIndex := range fixtures[releaseIndex].Assets {
				asset := &fixtures[releaseIndex].Assets[assetIndex]
				if asset.BrowserDownloadURL == "" {
					asset.BrowserDownloadURL = serverURL + "/downloads/" + asset.Name
				}
			}
		}
		response.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(response).Encode(fixtures); err != nil {
			t.Errorf("encode GitHub release fixture: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	serverURL = server.URL

	provider, err := newApplicationGitHubProvider(applicationGitHubProviderConfig{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	require.NoError(t, err)
	return provider
}

func newGitHubReleaseFixture(tag string, prerelease bool) githubReleaseFixture {
	assets := []githubAssetFixture{
		{ID: 1, Name: "Fallout-Terminal-windows-amd64.zip"},
		{ID: 2, Name: "Fallout-Terminal-windows-arm64.zip"},
		{ID: 3, Name: "Fallout-Terminal-linux-amd64.tar.gz"},
		{ID: 4, Name: "Fallout-Terminal-linux-arm64.tar.gz"},
		{ID: 5, Name: "Fallout-Terminal-darwin-arm64.zip"},
	}
	for index := range assets {
		assets[index].State = "uploaded"
		assets[index].ContentType = "application/octet-stream"
		assets[index].Size = int64(1024 + index)
		assets[index].Digest = "sha256:" + strings.Repeat(string("0123456789abcdef"[index]), sha256.Size*2)
	}
	return githubReleaseFixture{
		TagName: tag, Name: "Fallout Terminal " + tag, Body: "Release notes for " + tag,
		Prerelease: prerelease, PublishedAt: "2026-08-27T12:00:00Z",
		HTMLURL: "https://github.com/obalunenko/Fallout-Terminal/releases/tag/" + tag,
		Assets:  assets,
	}
}

func cloneGitHubReleaseFixtures(releases []githubReleaseFixture) []githubReleaseFixture {
	clones := make([]githubReleaseFixture, len(releases))
	for index := range releases {
		clones[index] = cloneGitHubReleaseFixture(releases[index])
	}
	return clones
}

func cloneGitHubReleaseFixture(release githubReleaseFixture) githubReleaseFixture {
	release.Assets = append([]githubAssetFixture(nil), release.Assets...)
	return release
}

func findGitHubAssetFixture(t *testing.T, assets []githubAssetFixture, name string) githubAssetFixture {
	t.Helper()
	index := slices.IndexFunc(assets, func(asset githubAssetFixture) bool { return asset.Name == name })
	require.NotEqual(t, -1, index, "GitHub asset fixture %q not found", name)
	return assets[index]
}

func replaceGitHubAssetFixture(t *testing.T, assets []githubAssetFixture, replacement githubAssetFixture) {
	t.Helper()
	index := slices.IndexFunc(assets, func(asset githubAssetFixture) bool {
		return asset.Name == replacement.Name
	})
	require.NotEqual(t, -1, index, "GitHub asset fixture %q not found", replacement.Name)
	assets[index] = replacement
}
