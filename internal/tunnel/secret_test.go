package tunnel

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicAccessSecretReferencesAreClosedAndDistinct(t *testing.T) {
	t.Parallel()

	require.NotEqual(t, ProviderAccountToken, PlayerBasicAuthPassword)
	assert.Equal(t, "ngrok-authtoken", ProviderAccountToken.Account())
	assert.Equal(t, "player-basic-auth-password", PlayerBasicAuthPassword.Account())
	assert.Empty(t, SecretRef(0).Account())
	assert.False(t, SecretRef(0).Valid())
}

func TestManualPlayerPasswordValidationUsesOnlyEightCharacterMinimum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "seven ASCII", value: "1234567", wantErr: true},
		{name: "eight ASCII", value: "12345678"},
		{name: "eight repeated", value: "aaaaaaaa"},
		{name: "eight symbols", value: "!!!!!!!!"},
		{name: "eight Unicode characters", value: "паролики"},
		{name: "newline rejected", value: "1234\n678", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePlayerPassword([]byte(test.value))
			if test.wantErr {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), test.value)
				return
			}
			require.NoError(t, err)
			assert.GreaterOrEqual(t, utf8.RuneCountInString(test.value), 8)
		})
	}
}

func TestGeneratedPlayerPasswordConsumesAtLeast128BitsAndRedactsFailures(t *testing.T) {
	t.Parallel()

	input := bytes.Repeat([]byte{0x5a}, GeneratedPasswordEntropyBytes)
	generated, err := GeneratePlayerPassword(bytes.NewReader(input))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, GeneratedPasswordEntropyBytes*8, 128)
	assert.GreaterOrEqual(t, len(generated), 8)
	assert.NotEqual(t, input, generated)

	generated, err = GeneratePlayerPassword(failingSecretReader{err: errors.New("sensitive random backend detail")})
	require.Error(t, err)
	assert.Nil(t, generated)
	assert.NotContains(t, err.Error(), "sensitive random backend detail")
}

func TestReplaceAndDeleteSecretUseFixedRefsAndClearTemporaryCopies(t *testing.T) {
	t.Parallel()

	store := newObservingSecretStore()
	providerInput := []byte("synthetic-provider-value")
	passwordInput := []byte("synthetic-player-value")
	require.NoError(t, ReplaceSecret(t.Context(), store, ProviderAccountToken, providerInput))
	require.NoError(t, ReplaceSecret(t.Context(), store, PlayerBasicAuthPassword, passwordInput))
	assert.Equal(t, []SecretRef{ProviderAccountToken, PlayerBasicAuthPassword}, store.replacedRefs)
	for _, observed := range store.observedReplacementBuffers {
		assert.Equal(t, make([]byte, len(observed)), observed, "temporary replacement copy was not cleared")
	}
	assert.Equal(t, []byte("synthetic-provider-value"), providerInput, "caller owns and clears its UI buffer")

	require.NoError(t, DeleteSecret(t.Context(), store, ProviderAccountToken))
	require.NoError(t, DeleteSecret(t.Context(), store, ProviderAccountToken))
	assert.Equal(t, []SecretRef{ProviderAccountToken, ProviderAccountToken}, store.deletedRefs)
	require.Error(t, ReplaceSecret(t.Context(), store, SecretRef(99), providerInput))
}

func TestScopedPublicAccessSecretUseClearsCallbackBuffersAndRedactsStoreErrors(t *testing.T) {
	t.Parallel()

	store := newObservingSecretStore()
	store.values[ProviderAccountToken] = []byte("synthetic-provider-value")
	store.values[PlayerBasicAuthPassword] = []byte("synthetic-player-value")
	var captured *SecretUse
	require.NoError(t, WithPublicAccessSecrets(t.Context(), store, func(use *SecretUse) error {
		captured = use
		assert.Equal(t, store.values[ProviderAccountToken], use.ProviderToken)
		assert.Equal(t, store.values[PlayerBasicAuthPassword], use.PlayerPassword)
		return nil
	}))
	require.NotNil(t, captured)
	assert.Empty(t, captured.ProviderToken)
	assert.Empty(t, captured.PlayerPassword)

	store.useErr = errors.New("synthetic-provider-value was rejected by a backend")
	err := WithPublicAccessSecrets(t.Context(), store, func(*SecretUse) error { return nil })
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "synthetic-provider-value")
}

func TestScopedPlayerPasswordUseReadsOnlyPasswordAndClearsCallbackBuffer(t *testing.T) {
	t.Parallel()

	store := newObservingSecretStore()
	store.values[ProviderAccountToken] = []byte("synthetic-provider-value")
	store.values[PlayerBasicAuthPassword] = []byte("synthetic-player-value")
	var captured []byte
	require.NoError(t, WithPlayerPassword(t.Context(), store, func(password []byte) error {
		captured = password
		assert.Equal(t, []byte("synthetic-player-value"), password)
		return nil
	}))
	assert.Equal(t, []SecretRef{PlayerBasicAuthPassword}, store.requestedRefs)
	assert.Equal(t, make([]byte, len(captured)), captured, "callback password buffer was not cleared")

	store.values[PlayerBasicAuthPassword] = []byte("short")
	err := WithPlayerPassword(t.Context(), store, func([]byte) error { return nil })
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "short")

	store.useErr = errors.New("synthetic-player-value was rejected by a backend")
	err = WithPlayerPassword(t.Context(), store, func([]byte) error { return nil })
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "synthetic-player-value")
}

func TestDevelopmentOverrideUsesNonEmptyEnvironmentSecretsPerFieldWithoutMutationOrReadback(t *testing.T) {
	providerOverride := "generated-" + strings.Repeat("p", 32)
	passwordOverride := "generated-" + strings.Repeat("w", 16)
	storedProvider := "generated-" + strings.Repeat("s", 32)
	storedPassword := "generated-" + strings.Repeat("f", 16)
	for _, test := range []struct {
		name             string
		values           map[string]string
		wantProviderSize int
		wantPasswordSize int
	}{
		{name: "token only", values: map[string]string{DevelopmentNgrokAuthtokenEnvironment: providerOverride}, wantProviderSize: len(providerOverride), wantPasswordSize: len(storedPassword)},
		{name: "password only", values: map[string]string{DevelopmentPlayerPasswordEnvironment: passwordOverride}, wantProviderSize: len(storedProvider), wantPasswordSize: len(passwordOverride)},
		{name: "both", values: map[string]string{DevelopmentNgrokAuthtokenEnvironment: providerOverride, DevelopmentPlayerPasswordEnvironment: passwordOverride}, wantProviderSize: len(providerOverride), wantPasswordSize: len(passwordOverride)},
		{name: "empty and unset", values: map[string]string{DevelopmentNgrokAuthtokenEnvironment: ""}, wantProviderSize: len(storedProvider), wantPasswordSize: len(storedPassword)},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newObservingSecretStore()
			store.values[ProviderAccountToken] = []byte(storedProvider)
			store.values[PlayerBasicAuthPassword] = []byte(storedPassword)
			override := NewDevelopmentTestPublicAccessOverride(nil, store, func(name string) (string, bool) {
				value, ok := test.values[name]
				return value, ok
			})

			providerPresence, err := override.Presence(t.Context(), ProviderAccountToken)
			require.NoError(t, err)
			passwordPresence, err := override.Presence(t.Context(), PlayerBasicAuthPassword)
			require.NoError(t, err)
			assert.Equal(t, SecretPresent, providerPresence)
			assert.Equal(t, SecretPresent, passwordPresence)

			var captured *SecretUse
			require.NoError(t, WithPublicAccessSecrets(t.Context(), override, func(use *SecretUse) error {
				captured = use
				assert.Equal(t, test.wantProviderSize, len(use.ProviderToken))
				assert.Equal(t, test.wantPasswordSize, len(use.PlayerPassword))
				return nil
			}))
			require.NotNil(t, captured)
			assert.Empty(t, captured.ProviderToken)
			assert.Empty(t, captured.PlayerPassword)
			assert.Empty(t, store.replacedRefs)
			assert.Empty(t, store.deletedRefs)
		})
	}
}

type failingSecretReader struct{ err error }

func (reader failingSecretReader) Read([]byte) (int, error) { return 0, reader.err }

type observingSecretStore struct {
	values                     map[SecretRef][]byte
	replacedRefs               []SecretRef
	deletedRefs                []SecretRef
	observedReplacementBuffers [][]byte
	requestedRefs              []SecretRef
	useErr                     error
}

func newObservingSecretStore() *observingSecretStore {
	return &observingSecretStore{values: make(map[SecretRef][]byte)}
}

func (store *observingSecretStore) Presence(_ context.Context, ref SecretRef) (SecretPresence, error) {
	if len(store.values[ref]) == 0 {
		return SecretAbsent, nil
	}
	return SecretPresent, nil
}

func (store *observingSecretStore) Replace(_ context.Context, ref SecretRef, value []byte) error {
	store.replacedRefs = append(store.replacedRefs, ref)
	store.observedReplacementBuffers = append(store.observedReplacementBuffers, value)
	store.values[ref] = append([]byte(nil), value...)
	return nil
}

func (store *observingSecretStore) Delete(_ context.Context, ref SecretRef) error {
	store.deletedRefs = append(store.deletedRefs, ref)
	delete(store.values, ref)
	return nil
}

func (store *observingSecretStore) WithSecrets(_ context.Context, refs []SecretRef, callback func(*SecretUse) error) error {
	if store.useErr != nil {
		return store.useErr
	}
	store.requestedRefs = append(store.requestedRefs, refs...)
	use := &SecretUse{}
	for _, ref := range refs {
		switch ref {
		case ProviderAccountToken:
			use.ProviderToken = append([]byte(nil), store.values[ref]...)
		case PlayerBasicAuthPassword:
			use.PlayerPassword = append([]byte(nil), store.values[ref]...)
		}
	}
	err := callback(use)
	use.Clear()
	return err
}
