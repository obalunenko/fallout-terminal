package tunnel_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/testutil"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/tunnel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const settingsPath = "/private/application-support/public-access.json"

func TestPublicAccessSettingsMissingFileReturnsSafeDisabledDefaults(t *testing.T) {
	t.Parallel()

	store := tunnel.NewPublicAccessSettingsStore(settingsPath, testutil.NewFakeFileSystem(), testutil.NewFakeClock(time.Unix(100, 0)))
	preferences, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, tunnel.DefaultPublicAccessPreferences(), preferences)
	assert.False(t, preferences.EnabledPreference, "saved preference never auto-starts an endpoint")
}

func TestPublicAccessSettingsRoundTripExactVersionOneSecretFreeJSON(t *testing.T) {
	t.Parallel()

	filesystem := testutil.NewFakeFileSystem()
	store := tunnel.NewPublicAccessSettingsStore(settingsPath, filesystem, testutil.NewFakeClock(time.Unix(100, 0)))
	preferences := tunnel.PublicAccessPreferences{
		Version: 1, EnabledPreference: true, ReservedDomain: "Vault.Example.", Username: " players ",
		ProviderTokenPresentHint: true, PlayerPasswordPresentHint: true, Revision: 7,
	}
	require.NoError(t, store.Save(preferences))

	raw, ok := filesystem.File(settingsPath)
	require.True(t, ok)
	var document map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &document))
	assert.ElementsMatch(t, []string{
		"version", "enabledPreference", "reservedDomain", "username",
		"providerTokenPresentHint", "playerPasswordPresentHint", "revision",
	}, mapKeys(document))
	for _, forbidden := range []string{"token", "password", "authtoken", "credential", "secret"} {
		assert.NotContains(t, strings.ToLower(string(raw)), `"`+forbidden+`"`)
	}

	loaded, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, "vault.example", loaded.ReservedDomain)
	assert.Equal(t, "players", loaded.Username)
	assert.True(t, loaded.EnabledPreference, "enabled preference is presentation only")
	assert.Equal(t, uint64(7), loaded.Revision)

	directoryMode, ok := filesystem.Mode(filepath.Dir(settingsPath))
	require.True(t, ok)
	assert.Equal(t, 0o700, int(directoryMode.Perm()))
	fileMode, ok := filesystem.Mode(settingsPath)
	require.True(t, ok)
	assert.Equal(t, 0o600, int(fileMode.Perm()))
	assert.Len(t, filesystem.RenameCalls(), 1, "save uses one same-directory atomic rename")
	for _, path := range filesystem.Paths() {
		assert.True(t, strings.HasPrefix(path, filepath.Dir(settingsPath)+string(filepath.Separator)))
		assert.NotContains(t, path, "session")
		assert.NotContains(t, path, "player-config")
	}
}

func TestPublicAccessSettingsQuarantinesMalformedUnknownAndFutureDocuments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "malformed", raw: `{"version":`},
		{name: "unknown field", raw: `{"version":1,"username":"players","surprise":true}`},
		{name: "future version", raw: `{"version":2,"username":"players"}`},
		{name: "invalid domain", raw: `{"version":1,"username":"players","reservedDomain":"https://vault.example"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			filesystem := testutil.NewFakeFileSystem()
			filesystem.SeedFile(settingsPath, []byte(test.raw))
			store := tunnel.NewPublicAccessSettingsStore(settingsPath, filesystem, testutil.NewFakeClock(time.Unix(100, 0)))
			preferences, err := store.Load()
			require.ErrorIs(t, err, tunnel.ErrSettingsRecovered)
			assert.NotContains(t, err.Error(), test.raw)
			assert.Equal(t, tunnel.DefaultPublicAccessPreferences(), preferences)
			_, exists := filesystem.File(settingsPath)
			assert.False(t, exists)
			paths := filesystem.Paths()
			require.Len(t, paths, 1)
			assert.Contains(t, filepath.Base(paths[0]), "public-access.corrupt-")
			mode, ok := filesystem.Mode(paths[0])
			require.True(t, ok)
			assert.Equal(t, 0o600, int(mode.Perm()))
		})
	}
}

func TestPublicAccessSettingsAtomicFailureRemovesTemporaryFile(t *testing.T) {
	t.Parallel()

	filesystem := testutil.NewFakeFileSystem()
	temporary := filepath.Join(filepath.Dir(settingsPath), ".public-access-000001")
	filesystem.RenameErrors[temporary] = errors.New("injected rename failure")
	store := tunnel.NewPublicAccessSettingsStore(settingsPath, filesystem, testutil.NewFakeClock(time.Unix(100, 0)))
	err := store.Save(tunnel.DefaultPublicAccessPreferences())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), temporary)
	_, exists := filesystem.File(temporary)
	assert.False(t, exists)
	_, exists = filesystem.File(settingsPath)
	assert.False(t, exists)
}

func TestDevelopmentOverridePrefillsNonEmptyDomainAndUsernameWithPerFieldFallback(t *testing.T) {
	for _, test := range []struct {
		name       string
		values     map[string]string
		wantDomain string
		wantUser   string
	}{
		{name: "domain only", values: map[string]string{tunnel.DevelopmentNgrokDomainEnvironment: "override.example"}, wantDomain: "override.example", wantUser: "stored-players"},
		{name: "username only", values: map[string]string{tunnel.DevelopmentPlayerUsernameEnvironment: "override-players"}, wantDomain: "stored.example", wantUser: "override-players"},
		{name: "both", values: map[string]string{tunnel.DevelopmentNgrokDomainEnvironment: "override.example", tunnel.DevelopmentPlayerUsernameEnvironment: "override-players"}, wantDomain: "override.example", wantUser: "override-players"},
		{name: "empty and unset", values: map[string]string{tunnel.DevelopmentNgrokDomainEnvironment: ""}, wantDomain: "stored.example", wantUser: "stored-players"},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := &recordingPublicAccessSettings{preferences: tunnel.PublicAccessPreferences{
				Version: 1, ReservedDomain: "stored.example", Username: "stored-players", Revision: 7,
			}}
			override := tunnel.NewDevelopmentTestPublicAccessOverride(base, nil, func(name string) (string, bool) {
				value, ok := test.values[name]
				return value, ok
			})

			loaded, err := override.Load()
			require.NoError(t, err)
			assert.Equal(t, test.wantDomain, loaded.ReservedDomain)
			assert.Equal(t, test.wantUser, loaded.Username)
			assert.Equal(t, uint64(7), loaded.Revision)
			assert.Zero(t, base.saves, "loading an override must not write JSON")

			loaded.ReservedDomain = "saved.example"
			loaded.Username = "saved-players"
			require.NoError(t, override.Save(loaded))
			assert.Equal(t, 1, base.saves)
			assert.Equal(t, "saved.example", base.preferences.ReservedDomain)
			assert.Equal(t, "saved-players", base.preferences.Username)
		})
	}
}

func TestDevelopmentOverridePersistsVisibleValuesOnlyForExplicitSaveMutation(t *testing.T) {
	base := &recordingPublicAccessSettings{preferences: tunnel.PublicAccessPreferences{
		Version: 1, ReservedDomain: "stored.example", Username: "stored-players", Revision: 7,
	}}
	override := tunnel.NewDevelopmentTestPublicAccessOverride(base, nil, func(name string) (string, bool) {
		values := map[string]string{
			tunnel.DevelopmentNgrokDomainEnvironment:    "override.example",
			tunnel.DevelopmentPlayerUsernameEnvironment: "override-players",
		}
		value, ok := values[name]
		return value, ok
	})

	effective, err := override.Load()
	require.NoError(t, err)
	effective.Revision = 8
	effective.PlayerPasswordPresentHint = true
	require.NoError(t, override.SaveForMutation(effective, false))
	assert.Equal(t, "stored.example", base.preferences.ReservedDomain)
	assert.Equal(t, "stored-players", base.preferences.Username)
	assert.Equal(t, uint64(8), base.preferences.Revision)
	assert.True(t, base.preferences.PlayerPasswordPresentHint)

	effective.Revision = 9
	require.NoError(t, override.SaveForMutation(effective, true))
	assert.Equal(t, "override.example", base.preferences.ReservedDomain)
	assert.Equal(t, "override-players", base.preferences.Username)
	assert.Equal(t, uint64(9), base.preferences.Revision)
}

func TestDevelopmentOverrideRejectsInvalidVisibleValuesWithoutSaving(t *testing.T) {
	for _, test := range []struct {
		name  string
		env   string
		value string
	}{
		{name: "invalid domain", env: tunnel.DevelopmentNgrokDomainEnvironment, value: "https://override.example"},
		{name: "invalid username", env: tunnel.DevelopmentPlayerUsernameEnvironment, value: "bad\nuser"},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := &recordingPublicAccessSettings{preferences: tunnel.DefaultPublicAccessPreferences()}
			override := tunnel.NewDevelopmentTestPublicAccessOverride(base, nil, func(name string) (string, bool) {
				return test.value, name == test.env
			})
			_, err := override.Load()
			require.Error(t, err)
			assert.NotContains(t, err.Error(), test.value)
			assert.Zero(t, base.saves)
		})
	}
}

type recordingPublicAccessSettings struct {
	preferences tunnel.PublicAccessPreferences
	saves       int
}

func (settings *recordingPublicAccessSettings) Load() (tunnel.PublicAccessPreferences, error) {
	return settings.preferences, nil
}

func (settings *recordingPublicAccessSettings) Save(preferences tunnel.PublicAccessPreferences) error {
	settings.saves++
	settings.preferences = preferences
	return nil
}

func mapKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
