//go:build windows

package platform

import (
	"context"
	"testing"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/tunnel"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestTranslateWindowsCredentialErrorCategories(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		got  error
		want error
	}{
		{name: "locked", got: translateWindowsCredentialError(windows.ERROR_NOT_LOGGED_ON), want: errCredentialLocked},
		{name: "denied", got: translateWindowsCredentialError(windows.ERROR_ACCESS_DENIED), want: errCredentialDenied},
		{name: "unavailable", got: translateWindowsCredentialError(windows.ERROR_SERVICE_DISABLED), want: errCredentialUnavailable},
		{name: "cancelled context", got: translateWindowsCredentialError(context.Canceled), want: context.Canceled},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, test.got, test.want)
		})
	}

	backend := newFakeCredentialBackend()
	backend.presenceErrors[tunnel.ProviderAccountTokenAccount] = errCredentialDenied
	store := NewKeychainSecretStore(true, backend)
	_, err := store.Presence(t.Context(), tunnel.ProviderAccountToken)
	require.ErrorIs(t, err, tunnel.ErrSecretStoreDenied)
}
