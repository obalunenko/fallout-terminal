package session

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/obalunenko/Fallout-Terminal/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageWriteAtomicUsesPrivateSameDirectoryTemporaryFile(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	storage := NewStorage(fileSystem)
	target := filepath.Join(testLocations.DocumentsDefault, "campaign.json")
	want := []byte("{\n  \"version\": 1\n}\n")

	require.NoError(t, storage.WriteAtomic(target, want))

	mkdirs := fileSystem.MkdirCalls()
	require.Len(t, mkdirs, 1)
	assert.Equal(t, filepath.Dir(target), mkdirs[0].Path)
	writes := fileSystem.WriteCalls()
	require.Len(t, writes, 1)
	temporary := writes[0].Path
	assert.NotEqual(t, target, temporary)
	assert.Equal(t, filepath.Dir(target), filepath.Dir(temporary))
	assert.Equal(t, fs.FileMode(0o600), writes[0].Perm)
	assert.Equal(t, want, writes[0].Data)
	assert.Equal(t, []testutil.FileRename{{OldPath: temporary, NewPath: target}}, fileSystem.RenameCalls())
	got, ok := fileSystem.File(target)
	require.True(t, ok)
	assert.Equal(t, want, got)
	_, temporaryExists := fileSystem.File(temporary)
	assert.False(t, temporaryExists)
}

func TestStorageWriteAtomicKeepsOldTargetAndCleansTemporaryOnRenameFailure(t *testing.T) {
	t.Parallel()

	base := testutil.NewFakeFileSystem()
	fileSystem := &renameFailingFileSystem{FakeFileSystem: base, err: errors.New("volume unavailable")}
	storage := NewStorage(fileSystem)
	target := filepath.Join(testLocations.DocumentsDefault, "campaign.json")
	oldData := []byte("old complete document\n")
	base.SeedFile(target, oldData)

	err := storage.WriteAtomic(target, []byte("new document\n"))
	require.Error(t, err)
	got, ok := base.File(target)
	require.True(t, ok)
	assert.Equal(t, oldData, got)
	writes := base.WriteCalls()
	require.Len(t, writes, 1)
	_, temporaryExists := base.File(writes[0].Path)
	assert.False(t, temporaryExists)
	assert.Equal(t, []string{writes[0].Path}, base.RemoveCalls())
}

func TestStorageWriteAtomicKeepsOldTargetAndSkipsRenameOnTemporaryWriteFailure(t *testing.T) {
	t.Parallel()

	base := testutil.NewFakeFileSystem()
	fileSystem := &writeFailingFileSystem{FakeFileSystem: base, err: errors.New("disk full")}
	storage := NewStorage(fileSystem)
	target := filepath.Join(testLocations.DocumentsDefault, "campaign.json")
	oldData := []byte("old complete document\n")
	base.SeedFile(target, oldData)

	err := storage.WriteAtomic(target, []byte("new document\n"))
	require.Error(t, err)
	got, ok := base.File(target)
	require.True(t, ok)
	assert.Equal(t, oldData, got)
	writes := base.WriteCalls()
	require.Len(t, writes, 1)
	assert.Empty(t, base.RenameCalls())
	_, temporaryExists := base.File(writes[0].Path)
	assert.False(t, temporaryExists)
	assert.Equal(t, []string{writes[0].Path}, base.RemoveCalls())
}

func TestStorageCopyAtomicLeavesBundledDemoUnchanged(t *testing.T) {
	t.Parallel()

	fileSystem := testutil.NewFakeFileSystem()
	storage := NewStorage(fileSystem)
	source := testLocations.BundledDemo
	destination := filepath.Join(testLocations.DocumentsDefault, "demo-copy.json")
	demo := []byte("{\n  \"version\": 1,\n  \"name\": \"demo\",\n  \"terminals\": []\n}\n")
	fileSystem.SeedFile(source, demo)

	require.NoError(t, storage.CopyAtomic(source, destination))
	got, ok := fileSystem.File(source)
	require.True(t, ok)
	assert.Equal(t, demo, got)
	got, ok = fileSystem.File(destination)
	require.True(t, ok)
	assert.Equal(t, demo, got)
	assert.Equal(t, []string{source}, fileSystem.ReadCalls())
	renames := fileSystem.RenameCalls()
	require.Len(t, renames, 1)
	assert.Equal(t, destination, renames[0].NewPath)
}

type renameFailingFileSystem struct {
	*testutil.FakeFileSystem
	err error
}

func (fileSystem *renameFailingFileSystem) Rename(oldPath, newPath string) error {
	fileSystem.RenameErrors[oldPath] = fileSystem.err
	return fileSystem.FakeFileSystem.Rename(oldPath, newPath)
}

type writeFailingFileSystem struct {
	*testutil.FakeFileSystem
	err error
}

func (fileSystem *writeFailingFileSystem) WriteFile(path string, data []byte, perm fs.FileMode) error {
	fileSystem.WriteErrors[path] = fileSystem.err
	return fileSystem.FakeFileSystem.WriteFile(path, data, perm)
}
