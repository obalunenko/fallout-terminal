package playerconfig

import sessionservice "github.com/obalunenko/Fallout-Terminal/v2/internal/session"

// FileSystem is the atomic filesystem seam shared with durable session files.
type FileSystem = sessionservice.FileSystem

// Storage keeps player-config replacement behavior identical to session saves.
type Storage struct {
	inner *sessionservice.Storage
}

// NewStorage constructs atomic player-config storage over fileSystem.
func NewStorage(fileSystem FileSystem) *Storage {
	return &Storage{inner: sessionservice.NewStorage(fileSystem)}
}

func (storage *Storage) Read(path string) ([]byte, error) {
	if storage == nil || storage.inner == nil {
		return nil, errStorageUnavailable
	}
	return storage.inner.Read(path)
}

func (storage *Storage) WriteAtomic(path string, data []byte) error {
	if storage == nil || storage.inner == nil {
		return errStorageUnavailable
	}
	return storage.inner.WriteAtomic(path, data)
}
