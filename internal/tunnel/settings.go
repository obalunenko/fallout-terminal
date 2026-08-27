package tunnel

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	configv1 "github.com/obalunenko/Fallout-Terminal/v2/internal/gen/fallout/terminal/config/v1"
)

var ErrSettingsRecovered = errors.New("saved public-access settings were reset safely")

type PublicAccessSettingsStore struct {
	path       string
	filesystem FileSystem
	clock      Clock
}

func NewPublicAccessSettingsStore(path string, filesystem FileSystem, clock Clock) *PublicAccessSettingsStore {
	if filesystem == nil {
		filesystem = osFileSystem{}
	}
	if clock == nil {
		clock = wallClock{}
	}
	return &PublicAccessSettingsStore{path: filepath.Clean(path), filesystem: filesystem, clock: clock}
}

type publicAccessSettingsDocument struct {
	Version                   uint32  `json:"version"`
	EnabledPreference         bool    `json:"enabledPreference"`
	ReservedDomain            *string `json:"reservedDomain,omitempty"`
	Username                  string  `json:"username"`
	ProviderTokenPresentHint  bool    `json:"providerTokenPresentHint"`
	PlayerPasswordPresentHint bool    `json:"playerPasswordPresentHint"`
	Revision                  uint64  `json:"revision"`
}

func (store *PublicAccessSettingsStore) Load() (PublicAccessPreferences, error) {
	defaults := DefaultPublicAccessPreferences()
	if store == nil || store.filesystem == nil || !filepath.IsAbs(store.path) {
		return defaults, errors.New("public-access settings path is unavailable")
	}
	raw, err := store.filesystem.ReadFile(store.path)
	if errors.Is(err, fs.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return defaults, errors.New("read public-access settings failed")
	}
	document, decodeErr := decodePublicAccessSettings(raw)
	clear(raw)
	if decodeErr != nil {
		store.quarantine()
		return defaults, ErrSettingsRecovered
	}
	preferences, normalizeErr := document.native().Normalized()
	if normalizeErr != nil {
		store.quarantine()
		return defaults, ErrSettingsRecovered
	}
	return preferences, nil
}

func (store *PublicAccessSettingsStore) Save(preferences PublicAccessPreferences) error {
	if store == nil || store.filesystem == nil || !filepath.IsAbs(store.path) {
		return errors.New("public-access settings path is unavailable")
	}
	normalized, err := preferences.Normalized()
	if err != nil {
		return err
	}
	document := settingsDocument(normalized)
	raw, err := json.Marshal(document)
	if err != nil {
		return errors.New("encode public-access settings failed")
	}
	raw = append(raw, '\n')
	defer clear(raw)

	directory := filepath.Dir(store.path)
	if err := store.filesystem.MkdirAll(directory, 0o700); err != nil {
		return errors.New("create public-access settings directory failed")
	}
	if err := store.filesystem.Chmod(directory, 0o700); err != nil {
		return errors.New("secure public-access settings directory failed")
	}
	temporary, err := store.filesystem.CreateTemp(directory, ".public-access-*")
	if err != nil {
		return errors.New("create temporary public-access settings failed")
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = store.filesystem.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return errors.New("secure temporary public-access settings failed")
	}
	if _, err := temporary.Write(raw); err != nil {
		return errors.New("write temporary public-access settings failed")
	}
	if err := temporary.Sync(); err != nil {
		return errors.New("sync temporary public-access settings failed")
	}
	if err := temporary.Close(); err != nil {
		return errors.New("close temporary public-access settings failed")
	}
	if err := store.filesystem.Rename(temporaryPath, store.path); err != nil {
		return errors.New("commit public-access settings failed")
	}
	committed = true
	if err := store.filesystem.Chmod(store.path, 0o600); err != nil {
		return errors.New("secure public-access settings failed")
	}
	directoryHandle, err := store.filesystem.Open(directory)
	if err != nil {
		return errors.New("open public-access settings directory failed")
	}
	if err := directoryHandle.Sync(); err != nil {
		_ = directoryHandle.Close()
		return errors.New("sync public-access settings directory failed")
	}
	if err := directoryHandle.Close(); err != nil {
		return errors.New("close public-access settings directory failed")
	}
	return nil
}

func (store *PublicAccessSettingsStore) quarantine() {
	directory := filepath.Dir(store.path)
	_ = store.filesystem.MkdirAll(directory, 0o700)
	_ = store.filesystem.Chmod(directory, 0o700)
	name := "public-access.corrupt-" + store.clock.Now().UTC().Format("20060102T150405.000000000Z") + ".json"
	quarantinePath := filepath.Join(directory, name)
	if err := store.filesystem.Rename(store.path, quarantinePath); err == nil {
		_ = store.filesystem.Chmod(quarantinePath, 0o600)
	}
}

func decodePublicAccessSettings(raw []byte) (publicAccessSettingsDocument, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var document publicAccessSettingsDocument
	if err := decoder.Decode(&document); err != nil {
		return publicAccessSettingsDocument{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return publicAccessSettingsDocument{}, errors.New("public-access settings contain trailing data")
	}
	return document, nil
}

func settingsDocument(preferences PublicAccessPreferences) publicAccessSettingsDocument {
	document := publicAccessSettingsDocument{
		Version: preferences.Version, EnabledPreference: preferences.EnabledPreference,
		Username: preferences.Username, ProviderTokenPresentHint: preferences.ProviderTokenPresentHint,
		PlayerPasswordPresentHint: preferences.PlayerPasswordPresentHint, Revision: preferences.Revision,
	}
	if preferences.ReservedDomain != "" {
		domain := preferences.ReservedDomain
		document.ReservedDomain = &domain
	}
	return document
}

func (document publicAccessSettingsDocument) native() PublicAccessPreferences {
	preferences := PublicAccessPreferences{
		Version: document.Version, EnabledPreference: document.EnabledPreference, Username: document.Username,
		ProviderTokenPresentHint:  document.ProviderTokenPresentHint,
		PlayerPasswordPresentHint: document.PlayerPasswordPresentHint, Revision: document.Revision,
	}
	if document.ReservedDomain != nil {
		preferences.ReservedDomain = *document.ReservedDomain
	}
	return preferences
}

func PreferencesToProto(preferences PublicAccessPreferences) *configv1.PublicAccessPreferences {
	message := &configv1.PublicAccessPreferences{
		Version: preferences.Version, EnabledPreference: preferences.EnabledPreference,
		Username: preferences.Username, ProviderTokenPresentHint: preferences.ProviderTokenPresentHint,
		PlayerPasswordPresentHint: preferences.PlayerPasswordPresentHint, Revision: preferences.Revision,
	}
	if preferences.ReservedDomain != "" {
		domain := preferences.ReservedDomain
		message.ReservedDomain = &domain
	}
	return message
}

func PreferencesFromProto(message *configv1.PublicAccessPreferences) (PublicAccessPreferences, error) {
	if message == nil {
		return PublicAccessPreferences{}, errors.New("public-access preferences are missing")
	}
	preferences := PublicAccessPreferences{
		Version: message.GetVersion(), EnabledPreference: message.GetEnabledPreference(),
		Username: message.GetUsername(), ProviderTokenPresentHint: message.GetProviderTokenPresentHint(),
		PlayerPasswordPresentHint: message.GetPlayerPasswordPresentHint(), Revision: message.GetRevision(),
	}
	if message.ReservedDomain != nil {
		preferences.ReservedDomain = message.GetReservedDomain()
	}
	return preferences.Normalized()
}

type wallClock struct{}

func (wallClock) Now() time.Time                                { return time.Now() }
func (wallClock) After(duration time.Duration) <-chan time.Time { return time.After(duration) }

type osFileSystem struct{}

func (osFileSystem) ReadFile(path string) ([]byte, error)         { return os.ReadFile(path) }
func (osFileSystem) MkdirAll(path string, mode fs.FileMode) error { return os.MkdirAll(path, mode) }
func (osFileSystem) CreateTemp(path, pattern string) (SyncWriteCloser, error) {
	return os.CreateTemp(path, pattern)
}
func (osFileSystem) Rename(oldPath, newPath string) error      { return os.Rename(oldPath, newPath) }
func (osFileSystem) Remove(path string) error                  { return os.Remove(path) }
func (osFileSystem) Chmod(path string, mode fs.FileMode) error { return os.Chmod(path, mode) }
func (osFileSystem) Open(path string) (SyncCloser, error)      { return os.Open(path) }

var _ FileSystem = osFileSystem{}
