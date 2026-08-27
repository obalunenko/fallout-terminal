package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/obalunenko/Fallout-Terminal/internal/tunnel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeychainServiceNamesAreStableAndSigningIndependent(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "com.vaulttec.fallout-terminal.public-access", KeychainServiceName(true))
	assert.Equal(t, "com.vaulttec.fallout-terminal.dev.public-access", KeychainServiceName(false))
	assert.Equal(t, KeychainServiceName(true), KeychainServiceNameForSigning(true, "Developer ID Application: Example"))
	assert.Equal(t, KeychainServiceName(false), KeychainServiceNameForSigning(false, "unsigned"))
}

func TestWindowsAndLinuxCredentialProvidersShareSecretStoreSemantics(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{"Windows Credential Manager", "Linux Secret Service"} {
		t.Run(provider, func(t *testing.T) {
			t.Parallel()
			testCredentialProviderPresenceAndIdentity(t)
			testCredentialProviderReplaceAndDelete(t)
			testCredentialProviderScopedUseAndClearing(t)
		})
	}
}

func testCredentialProviderPresenceAndIdentity(t *testing.T) {
	t.Helper()

	backend := newFakeCredentialBackend()
	backend.present[tunnel.ProviderAccountTokenAccount] = true
	store := NewKeychainSecretStore(true, backend)

	providerPresence, err := store.Presence(t.Context(), tunnel.ProviderAccountToken)
	require.NoError(t, err)
	passwordPresence, err := store.Presence(t.Context(), tunnel.PlayerBasicAuthPassword)
	require.NoError(t, err)

	assert.Equal(t, tunnel.SecretPresent, providerPresence)
	assert.Equal(t, tunnel.SecretAbsent, passwordPresence)
	assert.Equal(t, []string{KeychainServiceName(true), KeychainServiceName(true)}, backend.presenceServices)
	assert.Equal(t, []string{tunnel.ProviderAccountTokenAccount, tunnel.PlayerPasswordAccount}, backend.presenceAccounts)
	assert.Empty(t, backend.readAccounts, "presence must use provider metadata and never retrieve secret bytes")
}

func testCredentialProviderReplaceAndDelete(t *testing.T) {
	t.Helper()

	backend := newFakeCredentialBackend()
	backend.updateErrors[tunnel.ProviderAccountTokenAccount] = errCredentialNotFound
	store := NewKeychainSecretStore(false, backend)
	input := []byte("provider-input-remains-caller-owned")

	require.NoError(t, store.Replace(t.Context(), tunnel.ProviderAccountToken, input))
	assert.Equal(t, []byte("provider-input-remains-caller-owned"), input)
	assert.Equal(t, []string{tunnel.ProviderAccountTokenAccount}, backend.updateAccounts)
	assert.Equal(t, []string{tunnel.ProviderAccountTokenAccount}, backend.addAccounts)
	assert.Equal(t, []string{KeychainServiceName(false)}, backend.updateServices)
	assert.Equal(t, []string{KeychainServiceName(false)}, backend.addServices)
	assertClearedByteSlices(t, backend.updateInputs)
	assertClearedByteSlices(t, backend.addInputs)

	delete(backend.updateErrors, tunnel.ProviderAccountTokenAccount)
	require.NoError(t, store.Replace(t.Context(), tunnel.ProviderAccountToken, []byte("updated-provider-value")))
	assert.Len(t, backend.addAccounts, 1, "a successful native update must not create a duplicate item")
	assert.Equal(t, []byte("updated-provider-value"), backend.values[tunnel.ProviderAccountTokenAccount])

	require.NoError(t, store.Delete(t.Context(), tunnel.ProviderAccountToken))
	backend.deleteErrors[tunnel.ProviderAccountTokenAccount] = errCredentialNotFound
	require.NoError(t, store.Delete(t.Context(), tunnel.ProviderAccountToken))
	assert.Equal(t, []string{tunnel.ProviderAccountTokenAccount, tunnel.ProviderAccountTokenAccount}, backend.deleteAccounts)
	assert.Equal(t, []string{KeychainServiceName(false), KeychainServiceName(false)}, backend.deleteServices)

	err := store.WithSecrets(t.Context(), []tunnel.SecretRef{tunnel.ProviderAccountToken}, func(*tunnel.SecretUse) error {
		require.Fail(t, "callback must not run for a missing native credential")
		return nil
	})
	require.ErrorIs(t, err, tunnel.ErrSecretStoreUnavailable)
}

func testCredentialProviderScopedUseAndClearing(t *testing.T) {
	t.Helper()

	backend := newFakeCredentialBackend()
	backend.values[tunnel.ProviderAccountTokenAccount] = []byte("provider-returned-buffer")
	backend.values[tunnel.PlayerPasswordAccount] = []byte("password-returned-buffer")
	store := NewKeychainSecretStore(true, backend)

	var capturedUse *tunnel.SecretUse
	var callbackProvider []byte
	var callbackPassword []byte
	require.NoError(t, store.WithSecrets(t.Context(), []tunnel.SecretRef{
		tunnel.ProviderAccountToken,
		tunnel.PlayerBasicAuthPassword,
	}, func(use *tunnel.SecretUse) error {
		capturedUse = use
		callbackProvider = use.ProviderToken
		callbackPassword = use.PlayerPassword
		assert.Equal(t, []byte("provider-returned-buffer"), use.ProviderToken)
		assert.Equal(t, []byte("password-returned-buffer"), use.PlayerPassword)
		return nil
	}))

	require.NotNil(t, capturedUse)
	assert.Empty(t, capturedUse.ProviderToken)
	assert.Empty(t, capturedUse.PlayerPassword)
	assertClearedByteSlices(t, [][]byte{callbackProvider, callbackPassword})
	assertClearedByteSlices(t, backend.returnedValues)
	assert.Equal(t, []string{KeychainServiceName(true), KeychainServiceName(true)}, backend.readServices)
	assert.Equal(t, []string{tunnel.ProviderAccountTokenAccount, tunnel.PlayerPasswordAccount}, backend.readAccounts)

	callbackErr := errors.New("synthetic callback failure")
	backend.returnedValues = nil
	err := store.WithSecrets(t.Context(), []tunnel.SecretRef{tunnel.ProviderAccountToken}, func(use *tunnel.SecretUse) error {
		callbackProvider = use.ProviderToken
		return callbackErr
	})
	require.ErrorIs(t, err, callbackErr)
	assertClearedByteSlices(t, [][]byte{callbackProvider})
	assertClearedByteSlices(t, backend.returnedValues)
}

func assertClearedByteSlices(t *testing.T, values [][]byte) {
	t.Helper()

	require.NotEmpty(t, values)
	for _, value := range values {
		assert.NotEmpty(t, value, "the observed backing buffer must remain inspectable")
		for index, item := range value {
			assert.Zerof(t, item, "secret byte %d was not cleared", index)
		}
	}
}

func TestKeychainPresenceUsesAttributesOnlyAndFixedAccounts(t *testing.T) {
	t.Parallel()

	backend := newFakeCredentialBackend()
	backend.present[tunnel.ProviderAccountTokenAccount] = true
	store := NewKeychainSecretStore(false, backend)

	presence, err := store.Presence(t.Context(), tunnel.ProviderAccountToken)
	require.NoError(t, err)
	assert.Equal(t, tunnel.SecretPresent, presence)
	presence, err = store.Presence(t.Context(), tunnel.PlayerBasicAuthPassword)
	require.NoError(t, err)
	assert.Equal(t, tunnel.SecretAbsent, presence)
	assert.Equal(t, []string{tunnel.ProviderAccountTokenAccount, tunnel.PlayerPasswordAccount}, backend.presenceAccounts)
	assert.Empty(t, backend.readAccounts, "presence must never request secret data")
}

func TestKeychainReplaceUpdateAddDeleteAndNotFoundSemantics(t *testing.T) {
	t.Parallel()

	backend := newFakeCredentialBackend()
	backend.updateErrors[tunnel.ProviderAccountTokenAccount] = errCredentialNotFound
	store := NewKeychainSecretStore(true, backend)

	input := []byte("synthetic-provider-value")
	require.NoError(t, store.Replace(t.Context(), tunnel.ProviderAccountToken, input))
	assert.Equal(t, []string{tunnel.ProviderAccountTokenAccount}, backend.updateAccounts)
	assert.Equal(t, []string{tunnel.ProviderAccountTokenAccount}, backend.addAccounts)
	assert.Equal(t, []byte("synthetic-provider-value"), input)

	delete(backend.updateErrors, tunnel.ProviderAccountTokenAccount)
	require.NoError(t, store.Replace(t.Context(), tunnel.ProviderAccountToken, []byte("replacement-value")))
	assert.Len(t, backend.addAccounts, 1, "existing items update without a duplicate add")

	backend.deleteErrors[tunnel.ProviderAccountTokenAccount] = errCredentialNotFound
	require.NoError(t, store.Delete(t.Context(), tunnel.ProviderAccountToken))
	require.Error(t, store.Replace(t.Context(), tunnel.SecretRef(99), input))
}

func TestKeychainScopedReadClearsReturnedBuffers(t *testing.T) {
	t.Parallel()

	backend := newFakeCredentialBackend()
	backend.values[tunnel.ProviderAccountTokenAccount] = []byte("synthetic-provider-value")
	backend.values[tunnel.PlayerPasswordAccount] = []byte("synthetic-player-value")
	store := NewKeychainSecretStore(false, backend)

	var captured *tunnel.SecretUse
	require.NoError(t, store.WithSecrets(t.Context(), []tunnel.SecretRef{
		tunnel.ProviderAccountToken, tunnel.PlayerBasicAuthPassword,
	}, func(use *tunnel.SecretUse) error {
		captured = use
		assert.NotEmpty(t, use.ProviderToken)
		assert.NotEmpty(t, use.PlayerPassword)
		return nil
	}))
	require.NotNil(t, captured)
	assert.Empty(t, captured.ProviderToken)
	assert.Empty(t, captured.PlayerPassword)
	assert.Equal(t, []string{tunnel.ProviderAccountTokenAccount, tunnel.PlayerPasswordAccount}, backend.readAccounts)
}

func TestKeychainFailuresMapToStableSecretFreeCategories(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		backend error
		want    error
	}{
		{name: "not found", backend: errCredentialNotFound, want: tunnel.ErrSecretStoreUnavailable},
		{name: "locked", backend: errCredentialLocked, want: tunnel.ErrSecretStoreLocked},
		{name: "denied", backend: errCredentialDenied, want: tunnel.ErrSecretStoreDenied},
		{name: "unavailable", backend: errCredentialUnavailable, want: tunnel.ErrSecretStoreUnavailable},
		{name: "cancelled", backend: errCredentialUserCancelled, want: tunnel.ErrSecretStoreUserCancelled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			backend := newFakeCredentialBackend()
			backend.presenceErrors[tunnel.ProviderAccountTokenAccount] = test.backend
			store := NewKeychainSecretStore(false, backend)
			presence, err := store.Presence(t.Context(), tunnel.ProviderAccountToken)
			assert.Equal(t, tunnel.SecretUnknown, presence)
			require.ErrorIs(t, err, test.want)
			assert.NotContains(t, err.Error(), "synthetic-provider-value")
			assert.NotContains(t, err.Error(), "OSStatus")
		})
	}
}

func TestKeychainMutationAndReadFailuresUseTheSameStableCategories(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		backend error
		want    error
	}{
		{name: "locked", backend: errCredentialLocked, want: tunnel.ErrSecretStoreLocked},
		{name: "denied", backend: errCredentialDenied, want: tunnel.ErrSecretStoreDenied},
		{name: "unavailable", backend: errCredentialUnavailable, want: tunnel.ErrSecretStoreUnavailable},
		{name: "cancelled", backend: errCredentialUserCancelled, want: tunnel.ErrSecretStoreUserCancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			updateBackend := newFakeCredentialBackend()
			updateBackend.updateErrors[tunnel.ProviderAccountTokenAccount] = test.backend
			updateStore := NewKeychainSecretStore(true, updateBackend)
			require.ErrorIs(t, updateStore.Replace(t.Context(), tunnel.ProviderAccountToken, []byte("replace-canary")), test.want)
			assertClearedByteSlices(t, updateBackend.updateInputs)

			addBackend := newFakeCredentialBackend()
			addBackend.updateErrors[tunnel.ProviderAccountTokenAccount] = errCredentialNotFound
			addBackend.addErrors[tunnel.ProviderAccountTokenAccount] = test.backend
			addStore := NewKeychainSecretStore(true, addBackend)
			require.ErrorIs(t, addStore.Replace(t.Context(), tunnel.ProviderAccountToken, []byte("add-canary")), test.want)
			assertClearedByteSlices(t, addBackend.updateInputs)
			assertClearedByteSlices(t, addBackend.addInputs)

			deleteBackend := newFakeCredentialBackend()
			deleteBackend.deleteErrors[tunnel.ProviderAccountTokenAccount] = test.backend
			deleteStore := NewKeychainSecretStore(true, deleteBackend)
			require.ErrorIs(t, deleteStore.Delete(t.Context(), tunnel.ProviderAccountToken), test.want)

			readBackend := newFakeCredentialBackend()
			readBackend.readErrors[tunnel.ProviderAccountTokenAccount] = test.backend
			readStore := NewKeychainSecretStore(true, readBackend)
			err := readStore.WithSecrets(t.Context(), []tunnel.SecretRef{tunnel.ProviderAccountToken}, func(*tunnel.SecretUse) error {
				require.Fail(t, "callback must not run after a native provider read failure")
				return nil
			})
			require.ErrorIs(t, err, test.want)
		})
	}
}

func TestKeychainCancelledContextStopsBeforeProviderAccess(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	backend := newFakeCredentialBackend()
	store := NewKeychainSecretStore(true, backend)

	presence, err := store.Presence(ctx, tunnel.ProviderAccountToken)
	assert.Equal(t, tunnel.SecretUnknown, presence)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, store.Replace(ctx, tunnel.ProviderAccountToken, []byte("replace-canary")), context.Canceled)
	require.ErrorIs(t, store.Delete(ctx, tunnel.ProviderAccountToken), context.Canceled)
	require.ErrorIs(t, store.WithSecrets(ctx, []tunnel.SecretRef{tunnel.ProviderAccountToken}, func(*tunnel.SecretUse) error {
		require.Fail(t, "callback must not run for a cancelled context")
		return nil
	}), context.Canceled)
	assert.Empty(t, backend.presenceAccounts)
	assert.Empty(t, backend.updateAccounts)
	assert.Empty(t, backend.deleteAccounts)
	assert.Empty(t, backend.readAccounts)
}

type fakeCredentialBackend struct {
	present map[string]bool
	values  map[string][]byte

	presenceErrors map[string]error
	updateErrors   map[string]error
	addErrors      map[string]error
	deleteErrors   map[string]error
	readErrors     map[string]error

	presenceAccounts []string
	presenceServices []string
	updateAccounts   []string
	updateServices   []string
	updateInputs     [][]byte
	addAccounts      []string
	addServices      []string
	addInputs        [][]byte
	deleteAccounts   []string
	deleteServices   []string
	readAccounts     []string
	readServices     []string
	returnedValues   [][]byte
}

func newFakeCredentialBackend() *fakeCredentialBackend {
	return &fakeCredentialBackend{
		present: make(map[string]bool), values: make(map[string][]byte),
		presenceErrors: make(map[string]error), updateErrors: make(map[string]error),
		addErrors: make(map[string]error), deleteErrors: make(map[string]error), readErrors: make(map[string]error),
	}
}

func (backend *fakeCredentialBackend) Presence(_ context.Context, service, account string) (bool, error) {
	backend.presenceServices = append(backend.presenceServices, service)
	backend.presenceAccounts = append(backend.presenceAccounts, account)
	return backend.present[account], backend.presenceErrors[account]
}

func (backend *fakeCredentialBackend) Update(_ context.Context, service, account string, value []byte) error {
	backend.updateServices = append(backend.updateServices, service)
	backend.updateAccounts = append(backend.updateAccounts, account)
	backend.updateInputs = append(backend.updateInputs, value)
	if err := backend.updateErrors[account]; err != nil {
		return err
	}
	backend.values[account] = append([]byte(nil), value...)
	backend.present[account] = true
	return nil
}

func (backend *fakeCredentialBackend) Add(_ context.Context, service, account string, value []byte) error {
	backend.addServices = append(backend.addServices, service)
	backend.addAccounts = append(backend.addAccounts, account)
	backend.addInputs = append(backend.addInputs, value)
	if err := backend.addErrors[account]; err != nil {
		return err
	}
	backend.values[account] = append([]byte(nil), value...)
	backend.present[account] = true
	return nil
}

func (backend *fakeCredentialBackend) Delete(_ context.Context, service, account string) error {
	backend.deleteServices = append(backend.deleteServices, service)
	backend.deleteAccounts = append(backend.deleteAccounts, account)
	if err := backend.deleteErrors[account]; err != nil {
		return err
	}
	delete(backend.values, account)
	delete(backend.present, account)
	return nil
}

func (backend *fakeCredentialBackend) Read(_ context.Context, service, account string) ([]byte, error) {
	backend.readServices = append(backend.readServices, service)
	backend.readAccounts = append(backend.readAccounts, account)
	if err := backend.readErrors[account]; err != nil {
		return nil, err
	}
	value, ok := backend.values[account]
	if !ok {
		return nil, errCredentialNotFound
	}
	result := append([]byte(nil), value...)
	backend.returnedValues = append(backend.returnedValues, result)
	return result, nil
}

var _ credentialBackend = (*fakeCredentialBackend)(nil)
