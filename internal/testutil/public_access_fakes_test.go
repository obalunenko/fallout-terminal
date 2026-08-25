package testutil

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/obalunenko/Fallout-Terminal/internal/tunnel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakePublicIngressTracksStartsAndCleanup(t *testing.T) {
	t.Parallel()

	factory := NewFakePublicIngressFactory()
	ingress, err := factory.Start(t.Context(), "http://127.0.0.1:3690")
	require.NoError(t, err)
	t.Cleanup(func() { assert.Zero(t, factory.ActiveIngresses()) })
	t.Cleanup(func() { require.NoError(t, ingress.Close(context.WithoutCancel(t.Context()))) })

	assert.Equal(t, 1, factory.StartCalls())
	assert.Equal(t, 1, factory.ActiveIngresses())
	assert.Equal(t, "http://127.0.0.1:43690", ingress.URL().String())
	require.NoError(t, ingress.Activate("public.example", "players", []byte("synthetic-player-input")))
	ingress.Deny()
	require.NoError(t, ingress.Close(t.Context()))
	require.NoError(t, ingress.Close(t.Context()))
	assert.Zero(t, factory.ActiveIngresses())
}

func TestFakeSecretStoreScopesCopiesAndClearsSecretBuffers(t *testing.T) {
	t.Parallel()

	store := NewFakeSecretStore()
	provider := []byte("provider-test-value")
	password := []byte("password-test-value")
	require.NoError(t, store.Replace(t.Context(), tunnel.ProviderAccountToken, provider))
	require.NoError(t, store.Replace(t.Context(), tunnel.PlayerBasicAuthPassword, password))
	clear(provider)
	clear(password)

	var callbackUse *tunnel.SecretUse
	require.NoError(t, store.WithSecrets(t.Context(), []tunnel.SecretRef{
		tunnel.ProviderAccountToken, tunnel.PlayerBasicAuthPassword,
	}, func(use *tunnel.SecretUse) error {
		callbackUse = use
		assert.Equal(t, []byte("provider-test-value"), use.ProviderToken)
		assert.Equal(t, []byte("password-test-value"), use.PlayerPassword)
		return nil
	}))
	require.NotNil(t, callbackUse)
	assert.Empty(t, callbackUse.ProviderToken)
	assert.Empty(t, callbackUse.PlayerPassword)
	assert.True(t, store.LastUseCleared())

	presence, err := store.Presence(t.Context(), tunnel.ProviderAccountToken)
	require.NoError(t, err)
	assert.Equal(t, tunnel.SecretPresent, presence)
	require.NoError(t, store.Delete(t.Context(), tunnel.ProviderAccountToken))
	require.NoError(t, store.Delete(t.Context(), tunnel.ProviderAccountToken))
}

func TestFakeTunnelControlsDelayedStartDoneCloseAndActiveCount(t *testing.T) {
	t.Parallel()

	endpoint := NewFakeTunnelEndpoint("https://public.example")
	service := NewFakeTunnelService(endpoint)
	service.DelayStart()

	result := make(chan tunnel.TunnelEndpoint, 1)
	go func() {
		started, err := service.Start(t.Context(), tunnel.TunnelStartRequest{
			UpstreamURL: "http://127.0.0.1:41000", AccountToken: []byte("ephemeral-test-value"),
		})
		if err == nil {
			result <- started
		}
	}()
	require.Eventually(t, func() bool { return service.StartCalls() == 1 }, time.Second, time.Millisecond)
	assert.Equal(t, 0, service.ActiveEndpoints())
	service.ReleaseStart()
	started := <-result
	require.Same(t, endpoint, started)
	assert.Equal(t, 1, service.ActiveEndpoints())

	endpoint.Complete()
	select {
	case <-started.Done():
	case <-time.After(time.Second):
		assert.Fail(t, "fake endpoint Done did not close")
	}

	closeFailure := errors.New("injected close failure")
	endpoint.CloseErr = closeFailure
	require.ErrorIs(t, started.Close(t.Context()), closeFailure)
	assert.Equal(t, 1, service.ActiveEndpoints(), "failed Close retains ownership")
	endpoint.CloseErr = nil
	require.NoError(t, started.Close(t.Context()))
	assert.Equal(t, 0, service.ActiveEndpoints())
	assert.True(t, service.LastStartSecretsCleared())
	assert.Equal(t, "http://127.0.0.1:41000", service.LastUpstreamURL())
}

func TestFakeClockFilesystemAndEventsAreControllable(t *testing.T) {
	t.Parallel()

	now := time.Unix(100, 0)
	clock := NewFakeClock(now)
	timer := clock.After(5 * time.Second)
	clock.Advance(4 * time.Second)
	select {
	case <-timer:
		assert.Fail(t, "fake clock fired early")
	default:
	}
	clock.Advance(time.Second)
	select {
	case fired := <-timer:
		assert.Equal(t, now.Add(5*time.Second), fired)
	default:
		assert.Fail(t, "fake clock did not fire")
	}

	filesystem := NewFakeFileSystem()
	var _ tunnel.FileSystem = filesystem
	require.NoError(t, filesystem.MkdirAll("/settings", 0o700))
	file, err := filesystem.CreateTemp("/settings", ".public-access-*")
	require.NoError(t, err)
	_, err = file.Write([]byte("non-secret"))
	require.NoError(t, err)
	require.NoError(t, file.Chmod(0o600))
	require.NoError(t, file.Sync())
	require.NoError(t, file.Close())

	events := NewFakeSnapshotPublisher()
	snapshot := tunnel.PublicAccessSnapshot{Status: tunnel.PublicAccessStatus{State: tunnel.LifecycleDisabled}}
	events.Publish(snapshot)
	snapshot.Status.Generation = 99
	require.Equal(t, uint64(0), events.Snapshots()[0].Status.Generation)
}
