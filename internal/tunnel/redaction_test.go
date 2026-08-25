package tunnel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type codedProviderError struct {
	code       string
	diagnostic string
}

func (failure codedProviderError) Error() string { return failure.diagnostic }
func (failure codedProviderError) Code() string  { return failure.code }

func TestRedactedPublicAccessFailureMapsStableCategoriesWithoutSDKDiagnostics(t *testing.T) {
	canary := strings.Join([]string{
		"synthetic-token-canary", "synthetic-password-canary", "synthetic-username-canary",
		"synthetic-account-canary", "synthetic-domain-canary.example",
	}, " ")
	for _, test := range []struct {
		name     string
		err      error
		category ErrorCategory
		code     string
	}{
		{name: "provider authentication code", err: codedProviderError{code: "ERR_NGROK_105", diagnostic: canary}, category: ErrorProviderAuthentication, code: "ERR_NGROK_105"},
		{name: "account session limit", err: codedProviderError{code: "ERR_NGROK_108", diagnostic: canary}, category: ErrorProviderAuthentication, code: "ERR_NGROK_108"},
		{name: "unverified account", err: codedProviderError{code: "ERR_NGROK_123", diagnostic: canary}, category: ErrorProviderAuthentication, code: "ERR_NGROK_123"},
		{name: "reserved domain code", err: codedProviderError{code: "ERR_NGROK_320", diagnostic: canary}, category: ErrorDomainUnavailable, code: "ERR_NGROK_320"},
		{name: "invalid policy", err: codedProviderError{code: "ERR_NGROK_9026", diagnostic: canary}, category: ErrorValidation, code: "ERR_NGROK_9026"},
		{name: "network connectivity", err: codedProviderError{code: "ERR_NGROK_8001", diagnostic: canary}, category: ErrorNetworkUnavailable, code: "ERR_NGROK_8001"},
		{name: "DNS unavailable", err: &net.DNSError{Err: canary, Name: "synthetic-domain-canary.example"}, category: ErrorNetworkUnavailable},
		{name: "listener unavailable", err: &net.OpError{Op: "listen", Net: "tcp4", Err: errors.New(canary)}, category: ErrorNetworkUnavailable},
		{name: "deadline", err: fmt.Errorf("wrapped: %w", context.DeadlineExceeded), category: ErrorTimeout},
		{name: "Keychain denied", err: fmt.Errorf("wrapped: %w", ErrSecretStoreDenied), category: ErrorSecretStoreDenied},
		{name: "unknown provider code", err: codedProviderError{code: "ERR_NGROK_9999", diagnostic: canary}, category: ErrorProviderFailure, code: "ERR_NGROK_9999"},
		{name: "unsafe provider code", err: codedProviderError{code: "ERR_NGROK_1 secret", diagnostic: canary}, category: ErrorProviderFailure},
		{name: "raw provider failure", err: errors.New(canary), category: ErrorProviderFailure},
	} {
		t.Run(test.name, func(t *testing.T) {
			category, message := redactedPublicAccessFailure(test.err)
			assert.Equal(t, test.category, category)
			if test.code == "" {
				assert.Equal(t, test.category.SafeMessage(), message)
			} else {
				assert.Contains(t, message, test.code)
			}
			for marker := range strings.FieldsSeq(canary) {
				assert.NotContains(t, message, marker)
			}
		})
	}
}

func TestPublicIngressListenFailureReturnsOnlySafeNetworkCategory(t *testing.T) {
	canary := "synthetic-listener-diagnostic-canary"
	failure := publicIngressListenFailure(&net.OpError{
		Op: "listen", Net: "tcp4", Err: errors.New(canary),
	})
	category, message := redactedPublicAccessFailure(failure)

	assert.Equal(t, ErrorNetworkUnavailable, category)
	assert.Equal(t, ErrorNetworkUnavailable.SafeMessage(), message)
	assert.NotContains(t, failure.Error(), canary)
	code := safePublicAccessDiagnosticCode(failure)
	assert.Equal(t, DiagnosticPublicIngressListenFailed, code)
	assert.Equal(t, "public_ingress_listen_failed", code.String())
}

func TestRedactedPublicAccessFailureDropsLongCanariesAndIsConcurrent(t *testing.T) {
	diagnostic := strings.Repeat("diagnostic-prefix-", 1_000) + strings.Join([]string{
		"synthetic-token-canary", "synthetic-password-canary", "synthetic-username-canary",
		"synthetic-account-canary", "synthetic-domain-canary.example",
	}, ":")
	failure := codedProviderError{code: "ERR_NGROK_320", diagnostic: diagnostic}
	results := make(chan string, 100)
	var workers sync.WaitGroup
	for range 100 {
		workers.Go(func() {
			category, message := redactedPublicAccessFailure(failure)
			results <- fmt.Sprintf("%d:%s", category, message)
		})
	}
	workers.Wait()
	close(results)
	want := fmt.Sprintf("%d:%s", ErrorDomainUnavailable, "The reserved domain is unavailable for this account (ERR_NGROK_320).")
	for result := range results {
		assert.Equal(t, want, result)
		assert.LessOrEqual(t, len(result), maximumPublicAccessDiagnosticBytes)
		assert.NotContains(t, result, "synthetic-")
		assert.NotContains(t, result, "diagnostic-prefix")
	}
}

type redactionSettings struct{ preferences PublicAccessPreferences }

func (settings *redactionSettings) Load() (PublicAccessPreferences, error) {
	return settings.preferences, nil
}

func (settings *redactionSettings) Save(preferences PublicAccessPreferences) error {
	settings.preferences = preferences
	return nil
}

type redactionSecrets struct {
	provider []byte
	password []byte
}

func (secrets *redactionSecrets) Presence(_ context.Context, ref SecretRef) (SecretPresence, error) {
	if ref == ProviderAccountToken && len(secrets.provider) > 0 || ref == PlayerBasicAuthPassword && len(secrets.password) > 0 {
		return SecretPresent, nil
	}
	return SecretAbsent, nil
}

func (secrets *redactionSecrets) Replace(_ context.Context, ref SecretRef, value []byte) error {
	if ref == ProviderAccountToken {
		secrets.provider = append(secrets.provider[:0], value...)
	} else {
		secrets.password = append(secrets.password[:0], value...)
	}
	return nil
}

func (secrets *redactionSecrets) Delete(_ context.Context, ref SecretRef) error {
	if ref == ProviderAccountToken {
		clear(secrets.provider)
		secrets.provider = nil
	} else {
		clear(secrets.password)
		secrets.password = nil
	}
	return nil
}

func (secrets *redactionSecrets) WithSecrets(_ context.Context, _ []SecretRef, callback func(*SecretUse) error) error {
	use := &SecretUse{ProviderToken: append([]byte(nil), secrets.provider...), PlayerPassword: append([]byte(nil), secrets.password...)}
	defer use.Clear()
	return callback(use)
}

type retryingSDKFactory struct {
	mu       sync.Mutex
	failure  error
	agent    ngrokAgent
	attempts int
}

type redactionIngressFactory struct{ startErr error }

func (factory redactionIngressFactory) Start(context.Context, string) (PublicIngress, error) {
	if factory.startErr != nil {
		return nil, factory.startErr
	}
	return &redactionIngress{url: url.URL{Scheme: "http", Host: "127.0.0.1:43690"}}, nil
}

type redactionIngress struct{ url url.URL }

func (ingress *redactionIngress) URL() *url.URL {
	copyURL := ingress.url
	return &copyURL
}

func (*redactionIngress) Activate(string, string, []byte) error { return nil }
func (*redactionIngress) Deny()                                 {}
func (*redactionIngress) Close(context.Context) error           { return nil }

func TestPublicAccessManagerPropagatesSafeIngressDiagnosticCode(t *testing.T) {
	preferences := DefaultPublicAccessPreferences()
	preferences.Revision = 7
	canary := "synthetic-listener-diagnostic-canary"
	manager, err := NewPublicAccessManager(ManagerConfig{
		Settings: &redactionSettings{preferences: preferences},
		Secrets: &redactionSecrets{
			provider: []byte("synthetic-provider-input"), password: []byte("synthetic-player-input"),
		},
		Tunnel: newNgrokServiceWithFactory(&fakeSDKFactory{}),
		Ingress: redactionIngressFactory{startErr: publicIngressListenFailure(&net.OpError{
			Op: "listen", Net: "tcp4", Err: errors.New(canary),
		})},
		UpstreamURL: "http://127.0.0.1:3690",
	})
	require.NoError(t, err)
	manager.Initialize(t.Context())
	t.Cleanup(func() { require.NoError(t, manager.Shutdown(context.WithoutCancel(t.Context()))) })

	result := manager.Start(t.Context(), preferences.Revision)
	require.False(t, result.OK)
	assert.Equal(t, DiagnosticPublicIngressListenFailed, result.DiagnosticCode)
	assert.Equal(t, ErrorNetworkUnavailable, result.Snapshot.Status.ErrorCategory)
	assert.Equal(t, ErrorNetworkUnavailable.SafeMessage(), result.Error)
	assert.NotContains(t, fmt.Sprintf("%#v", result), canary)
}

func (factory *retryingSDKFactory) New(_ []byte) (ngrokAgent, error) {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	factory.attempts++
	if factory.attempts == 1 {
		return nil, factory.failure
	}
	return factory.agent, nil
}

func TestRedactionSurvivesDirectResultStatusEventAndRetryPaths(t *testing.T) {
	diagnostic := "synthetic-token-canary synthetic-password-canary synthetic-username-canary synthetic-account-canary synthetic-domain-canary.example " + strings.Repeat("long-provider-detail", 1_000)
	forwarder := newFakeSDKForwarder("https://retry.example")
	factory := &retryingSDKFactory{
		failure: codedProviderError{code: "ERR_NGROK_105", diagnostic: diagnostic},
		agent:   &fakeSDKAgent{forwarder: forwarder},
	}
	preferences := DefaultPublicAccessPreferences()
	preferences.Revision = 7
	serializedEvents := make([]string, 0, 4)
	var eventMu sync.Mutex
	manager, err := NewPublicAccessManager(ManagerConfig{
		Settings: &redactionSettings{preferences: preferences},
		Secrets: &redactionSecrets{
			provider: []byte("synthetic-provider-input"), password: []byte("synthetic-player-input"),
		},
		Tunnel: newNgrokServiceWithFactory(factory), Ingress: redactionIngressFactory{},
		UpstreamURL: "http://127.0.0.1:3690",
		Publish: func(snapshot PublicAccessSnapshot) {
			eventMu.Lock()
			serializedEvents = append(serializedEvents, fmt.Sprintf("%#v", snapshot))
			eventMu.Unlock()
		},
	})
	require.NoError(t, err)
	manager.Initialize(t.Context())
	t.Cleanup(func() { require.NoError(t, manager.Shutdown(context.WithoutCancel(t.Context()))) })

	failed := manager.Start(t.Context(), 7)
	require.False(t, failed.OK)
	assert.Equal(t, ErrorProviderAuthentication, failed.Snapshot.Status.ErrorCategory)
	assert.Equal(t, "The provider rejected the account credential (ERR_NGROK_105).", failed.Error)
	assert.Empty(t, failed.Snapshot.Status.PublicURL)
	for _, surface := range []string{failed.Error, failed.Snapshot.Status.ErrorMessage, fmt.Sprintf("%#v", failed)} {
		assert.NotContains(t, surface, "synthetic-token-canary")
		assert.NotContains(t, surface, "synthetic-domain-canary")
		assert.NotContains(t, surface, "long-provider-detail")
	}
	assert.LessOrEqual(t, len(failed.Error), maximumPublicAccessDiagnosticBytes)
	assert.LessOrEqual(t, len(failed.Snapshot.Status.ErrorMessage), maximumPublicAccessDiagnosticBytes)

	retried := manager.Start(t.Context(), 7)
	require.True(t, retried.OK, retried.Error)
	assert.Equal(t, LifecycleReady, retried.Snapshot.Status.State)
	assert.Equal(t, "https://retry.example", retried.Snapshot.Status.PublicURL)
	eventMu.Lock()
	defer eventMu.Unlock()
	for _, event := range serializedEvents {
		assert.NotContains(t, event, "synthetic-token-canary")
		assert.NotContains(t, event, "synthetic-password-canary")
		assert.NotContains(t, event, "long-provider-detail")
	}
}
