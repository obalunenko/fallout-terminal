//go:build darwin

package platform

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/tunnel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDarwinKeychainAdapterOptInRoundTripWithoutReadbackSurface(t *testing.T) {
	if os.Getenv("FALLOUT_KEYCHAIN_INTEGRATION") != "1" {
		t.Skip("NOT RUN: set FALLOUT_KEYCHAIN_INTEGRATION=1 to exercise an isolated temporary Keychain service")
	}

	suffix := make([]byte, 16)
	_, err := rand.Read(suffix)
	require.NoError(t, err)
	service := "com.vaulttec.fallout-terminal.dev.public-access.integration." + hex.EncodeToString(suffix)
	store := newDarwinKeychainSecretStoreWithService(service)
	t.Cleanup(func() { _ = store.Delete(context.WithoutCancel(t.Context()), tunnel.ProviderAccountToken) })

	value := make([]byte, 24)
	_, err = rand.Read(value)
	require.NoError(t, err)
	require.NoError(t, store.Replace(t.Context(), tunnel.ProviderAccountToken, value))
	presence, err := store.Presence(t.Context(), tunnel.ProviderAccountToken)
	require.NoError(t, err)
	assert.Equal(t, tunnel.SecretPresent, presence)
	require.NoError(t, store.WithSecrets(t.Context(), []tunnel.SecretRef{tunnel.ProviderAccountToken}, func(use *tunnel.SecretUse) error {
		assert.Equal(t, value, use.ProviderToken)
		return nil
	}))
	require.NoError(t, store.Delete(t.Context(), tunnel.ProviderAccountToken))
}
