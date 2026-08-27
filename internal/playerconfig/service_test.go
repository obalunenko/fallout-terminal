package playerconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestCreateLoadSaveAndReusePlayerConfig(t *testing.T) {
	t.Parallel()

	fs := testutil.NewFakeFileSystem()
	root := t.TempDir()
	target := filepath.Join(root, "shared", "vault-13-players.json")
	dialog := &testutil.FakeDialog{SaveResult: target}
	service := NewService(NewStorage(fs), dialog, root)

	created := service.Create(t.Context())
	require.Falsef(t, !created.OK || created.Canceled || created.FilePath != target || created.Config == nil,
		"Create() = %#v", created)
	require.Falsef(t, created.Config.Version != 1 || created.Config.Name != "vault-13-players" || len(created.Config.Roster) != 0,
		"created config = %#v", created.Config)
	require.False(t, created.Config.Roster == nil,
		"created config roster is nil, want a non-nil empty array")

	stored, ok := fs.File(target)
	require.Falsef(t, !ok,
		"created config was not written to %q", target)
	require.Falsef(t, !strings.Contains(string(stored), `"roster": []`),
		"created config does not encode an empty roster array: %s", stored)
	require.Equal(t, expectedContentDigest(stored), created.ContentDigest)

	handle := domain.PlayerConfigHandle{
		Path: target, Version: 1, Name: created.Config.Name, ContentDigest: created.ContentDigest,
	}
	wantRoster := []domain.CharacterRosterEntry{
		{ID: "mara", Name: "Mara", Intelligence: 8, HackerPerkAvailable: true},
		{ID: "boone", Name: "Boone", Intelligence: 4, HackerPerkAvailable: false},
		{ID: "arcade", Name: "Arcade", Intelligence: 10, HackerPerkAvailable: true},
	}
	refreshedHandle, err := service.Save(handle, wantRoster)
	require.NoError(t, err)
	require.Equal(t, handle.Path, refreshedHandle.Path)
	require.Equal(t, handle.Version, refreshedHandle.Version)
	require.Equal(t, handle.Name, refreshedHandle.Name)
	require.NotEmpty(t, refreshedHandle.ContentDigest)
	require.NotEqual(t, handle.ContentDigest, refreshedHandle.ContentDigest)
	saved, ok := fs.File(target)
	require.True(t, ok)
	require.Equal(t, expectedContentDigest(saved), refreshedHandle.ContentDigest)
	require.Contains(t, string(saved), `"intelligence": 8`)
	require.Contains(t, string(saved), `"hackerPerkAvailable": true`)
	require.Contains(t, string(saved), `"hackerPerkAvailable": false`)

	sessionPath := filepath.Join(root, "sessions", "game.json")
	reference, err := RelativeReference(sessionPath, target)
	if err != nil {
		require.NoError(t, err)
	}
	{
		want := filepath.Join("..", "shared", "vault-13-players.json")
		require.Falsef(t, reference != want,
			"reference = %q, want %q", reference, want)
	}

	loaded := service.LoadReferenced(sessionPath, reference)
	require.Falsef(t, !loaded.OK || loaded.Config == nil || !cmp.Equal(loaded.Config.Roster, wantRoster),
		"LoadReferenced() = %#v", loaded)
	require.Equal(t, 1, loaded.Config.Version)
	require.Equal(t, refreshedHandle.ContentDigest, loaded.ContentDigest)

	dialog.OpenResult = target
	opened := service.Open(t.Context())
	require.Falsef(t, !opened.OK || opened.Config == nil || !cmp.Equal(opened.Config.Roster, wantRoster),
		"Open() shared config = %#v", opened)
	require.Equal(t, 1, opened.Config.Version)
	require.Equal(t, refreshedHandle.ContentDigest, opened.ContentDigest)
}

func TestPlayerConfigSaveRejectsMissingReplacedOrUnreadableActiveFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "players.json")
	original := []byte("{\n  \"version\": 1,\n  \"name\": \"Players\",\n  \"roster\": [\n    {\n      \"id\": \"mara\",\n      \"name\": \"Mara\",\n      \"intelligence\": 8,\n      \"hackerPerkAvailable\": true\n    }\n  ]\n}\n")
	replacement := []byte("{\n  \"version\": 1,\n  \"name\": \"Externally replaced\",\n  \"roster\": []\n}\n")
	candidate := []domain.CharacterRosterEntry{
		{ID: "mara", Name: "Mara", Intelligence: 9, HackerPerkAvailable: false},
	}

	tests := []struct {
		name       string
		mutate     func(*testing.T, *testutil.FakeFileSystem)
		wantExists bool
		wantData   []byte
	}{
		{
			name: "missing",
			mutate: func(t *testing.T, fileSystem *testutil.FakeFileSystem) {
				require.NoError(t, fileSystem.Remove(target))
			},
		},
		{
			name: "replaced",
			mutate: func(_ *testing.T, fileSystem *testutil.FakeFileSystem) {
				fileSystem.SeedFile(target, replacement)
			},
			wantExists: true,
			wantData:   replacement,
		},
		{
			name: "unreadable",
			mutate: func(_ *testing.T, fileSystem *testutil.FakeFileSystem) {
				fileSystem.ReadErrors[target] = errors.New("permission denied")
			},
			wantExists: true,
			wantData:   original,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fileSystem := testutil.NewFakeFileSystem()
			fileSystem.SeedFile(target, original)
			service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{}, root)
			loaded := service.LoadReferenced(filepath.Join(root, "game.json"), "players.json")
			require.Truef(t, loaded.OK, "LoadReferenced() = %#v", loaded)
			require.NotEmpty(t, loaded.ContentDigest)
			handle := domain.PlayerConfigHandle{
				Path: target, Version: loaded.Config.Version, Name: loaded.Config.Name, ContentDigest: loaded.ContentDigest,
			}

			test.mutate(t, fileSystem)
			_, err := service.Save(handle, candidate)
			require.Error(t, err)
			require.ErrorIs(t, err, errActivePlayerConfigNeedsReselection)
			require.EqualError(t, err, "active player configuration is missing, unreadable, or changed; reopen or reselect it")
			require.NotContains(t, err.Error(), "permission denied", "the recovery message must not leak storage details")
			require.Empty(t, fileSystem.WriteCalls(), "a conflicted save must not start atomic replacement")
			require.Empty(t, fileSystem.RenameCalls(), "a conflicted save must not replace the target")
			got, exists := fileSystem.File(target)
			require.Equal(t, test.wantExists, exists)
			require.Equal(t, test.wantData, got)
		})
	}
}

func TestPlayerConfigSaveKeepsOldContentWhenAtomicReplacementFails(t *testing.T) {
	t.Parallel()

	base := testutil.NewFakeFileSystem()
	fileSystem := &renameFailingPlayerConfigFileSystem{
		FakeFileSystem: base,
		err:            errors.New("volume unavailable"),
	}
	root := t.TempDir()
	target := filepath.Join(root, "players.json")
	original := []byte("{\n  \"version\": 1,\n  \"name\": \"Players\",\n  \"roster\": [\n    {\n      \"id\": \"mara\",\n      \"name\": \"Mara\",\n      \"intelligence\": 8,\n      \"hackerPerkAvailable\": true\n    }\n  ]\n}\n")
	base.SeedFile(target, original)
	service := NewService(NewStorage(fileSystem), &testutil.FakeDialog{}, root)
	loaded := service.LoadReferenced(filepath.Join(root, "game.json"), "players.json")
	require.Truef(t, loaded.OK, "LoadReferenced() = %#v", loaded)
	handle := domain.PlayerConfigHandle{
		Path: target, Version: loaded.Config.Version, Name: loaded.Config.Name, ContentDigest: loaded.ContentDigest,
	}

	_, err := service.Save(handle, []domain.CharacterRosterEntry{
		{ID: "mara", Name: "Mara", Intelligence: 10, HackerPerkAvailable: false},
	})
	require.Error(t, err)
	stored, ok := base.File(target)
	require.True(t, ok)
	require.Equal(t, original, stored)
	require.Len(t, base.RenameCalls(), 1)
	require.Len(t, base.RemoveCalls(), 1)
}

func TestPlayerConfigCancellationAndFailuresAreNonMutating(t *testing.T) {
	t.Parallel()

	fs := testutil.NewFakeFileSystem()
	root := t.TempDir()
	service := NewService(NewStorage(fs), &testutil.FakeDialog{}, root)
	{
		result := service.Create(t.Context())
		require.Falsef(t, !result.Canceled || result.OK,
			"canceled Create() = %#v", result)
	}
	{

		result := service.Open(t.Context())
		require.Falsef(t, !result.Canceled || result.OK,
			"canceled Open() = %#v", result)
	}
	{

		writes := fs.WriteCalls()
		require.Falsef(t, len(writes) != 0,
			"cancellation wrote files: %#v", writes)
	}

	fs.SeedFile(filepath.Join(root, "invalid.json"), []byte(`{"version":1,"name":"Players","roster":[{"id":"","name":"Mara"}]}`))
	result := service.LoadReferenced(filepath.Join(root, "game.json"), "invalid.json")
	require.Falsef(t, result.OK || result.Error == "" || result.Config != nil,
		"invalid LoadReferenced() = %#v", result)

}

func TestPlayerConfigRejectsUnknownFieldsWithoutReplacingKnownFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source []byte
	}{
		{
			name:   "top-level field",
			source: []byte(`{"version":1,"name":"Players","roster":[],"futureCapability":true}`),
		},
		{
			name: "nested roster field",
			source: []byte(`{"version":1,"name":"Players","roster":[{` +
				`"id":"mara","name":"Mara","intelligence":8,"hackerPerkAvailable":true,"futureCapability":{"keep":true}}]}`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fs := testutil.NewFakeFileSystem()
			root := t.TempDir()
			target := filepath.Join(root, "players.json")
			fs.SeedFile(target, test.source)
			service := NewService(NewStorage(fs), &testutil.FakeDialog{OpenResult: target}, root)

			result := service.Open(t.Context())
			require.False(t, result.OK)
			require.False(t, result.Canceled)
			require.Nil(t, result.Config)
			require.Contains(t, result.Error, "not a valid version-1 player config")
			require.Empty(t, fs.WriteCalls(), "rejected source must not be rewritten")
			require.Empty(t, fs.RenameCalls(), "rejected source must not be replaced")
			require.Empty(t, fs.RemoveCalls(), "rejected source must not be removed")
			stored, ok := fs.File(target)
			require.True(t, ok)
			require.Equal(t, test.source, stored)
		})
	}
}

func TestCompleteCandidateSaveFailurePublishesNoSuccessfulConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := &failingPlayerConfigStore{err: errors.New("disk full")}
	service := NewService(store, &testutil.FakeDialog{SaveResult: filepath.Join(root, "players.json")}, root)
	created := service.Create(t.Context())
	require.Falsef(t, created.OK || created.Config != nil || created.FilePath != "" || store.writes != 1,
		"failed atomic create published state: result=%#v writes=%d", created, store.writes)
}

type failingPlayerConfigStore struct {
	err    error
	writes int
}

func (*failingPlayerConfigStore) Read(string) ([]byte, error) { return nil, errors.New("not found") }
func (store *failingPlayerConfigStore) WriteAtomic(string, []byte) error {
	store.writes++
	return store.err
}

type renameFailingPlayerConfigFileSystem struct {
	*testutil.FakeFileSystem
	err error
}

func (fileSystem *renameFailingPlayerConfigFileSystem) Rename(oldPath, newPath string) error {
	fileSystem.RenameErrors[oldPath] = fileSystem.err
	return fileSystem.FakeFileSystem.Rename(oldPath, newPath)
}

func expectedContentDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
