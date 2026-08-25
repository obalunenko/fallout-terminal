package tunnel

import (
	"context"
	"errors"
	"net"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ngrok "golang.ngrok.com/ngrok/v2"
)

type fakeNgrokCodedError struct{ code string }

func (failure fakeNgrokCodedError) Error() string { return "synthetic provider diagnostic" }
func (failure fakeNgrokCodedError) Code() string  { return failure.code }

type recordingNgrokAgent struct {
	upstream        *ngrok.Upstream
	endpointOptions int
}

func (agent *recordingNgrokAgent) Connect(context.Context) error { return nil }
func (agent *recordingNgrokAgent) Disconnect() error             { return nil }
func (agent *recordingNgrokAgent) Forward(
	_ context.Context,
	upstream *ngrok.Upstream,
	options ...ngrok.EndpointOption,
) (ngrok.EndpointForwarder, error) {
	agent.upstream = upstream
	agent.endpointOptions = len(options)
	return nil, errors.New("synthetic capture complete")
}

func TestSDKAgentForwardsWithExactHTTP2UpstreamOption(t *testing.T) {
	recorder := &recordingNgrokAgent{}
	agent := &sdkAgent{agent: recorder}
	_, err := agent.Forward(t.Context(), ngrokForwardRequest{
		UpstreamURL: "http://127.0.0.1:41000", ReservedDomain: "vault.example",
	})
	require.Error(t, err)
	require.NotNil(t, recorder.upstream)
	upstream := reflect.ValueOf(recorder.upstream).Elem()
	assert.Equal(t, "http://127.0.0.1:41000", upstream.FieldByName("addr").String())
	assert.Equal(t, "http2", upstream.FieldByName("protocol").String())
	assert.Equal(t, 1, recorder.endpointOptions, "reserved domain remains the sole endpoint option")
}

type fakeSDKFactory struct {
	agent *fakeSDKAgent
	err   error
	seen  []byte
}

func (factory *fakeSDKFactory) New(accountToken []byte) (ngrokAgent, error) {
	factory.seen = append([]byte(nil), accountToken...)
	if factory.err != nil {
		return nil, factory.err
	}
	return factory.agent, nil
}

type fakeSDKAgent struct {
	mu             sync.Mutex
	forwarder      *fakeSDKForwarder
	request        ngrokForwardRequest
	forwardErr     error
	forwardGate    chan struct{}
	disconnectErrs []error
	disconnectGate chan struct{}
	disconnects    int
	trackContext   bool
	forwardContext context.Context
}

func (agent *fakeSDKAgent) Forward(ctx context.Context, request ngrokForwardRequest) (ngrokForwarder, error) {
	agent.mu.Lock()
	agent.request = request
	agent.forwardContext = ctx
	gate := agent.forwardGate
	agent.mu.Unlock()
	if gate != nil {
		<-gate
	}
	agent.mu.Lock()
	defer agent.mu.Unlock()
	if agent.forwardErr != nil {
		return nil, agent.forwardErr
	}
	if agent.trackContext && agent.forwarder != nil {
		go func(forwarder *fakeSDKForwarder) {
			<-ctx.Done()
			forwarder.doneOnce.Do(func() { close(forwarder.done) })
		}(agent.forwarder)
	}
	return agent.forwarder, nil
}

func (agent *fakeSDKAgent) Disconnect() error {
	agent.mu.Lock()
	gate := agent.disconnectGate
	agent.mu.Unlock()
	if gate != nil {
		<-gate
	}
	agent.mu.Lock()
	defer agent.mu.Unlock()
	agent.disconnects++
	if agent.disconnects <= len(agent.disconnectErrs) {
		return agent.disconnectErrs[agent.disconnects-1]
	}
	return nil
}

func (agent *fakeSDKAgent) disconnectCalls() int {
	agent.mu.Lock()
	defer agent.mu.Unlock()
	return agent.disconnects
}

type fakeSDKForwarder struct {
	mu                         sync.Mutex
	endpoint                   *url.URL
	done                       chan struct{}
	doneOnce                   sync.Once
	closes                     int
	closeErrs                  []error
	closeGate                  chan struct{}
	forwardContext             context.Context
	failCloseOnCanceledContext bool
}

func newFakeSDKForwarder(raw string) *fakeSDKForwarder {
	parsed, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return &fakeSDKForwarder{endpoint: parsed, done: make(chan struct{})}
}

func (forwarder *fakeSDKForwarder) URL() *url.URL         { return forwarder.endpoint }
func (forwarder *fakeSDKForwarder) Done() <-chan struct{} { return forwarder.done }
func (forwarder *fakeSDKForwarder) Close(context.Context) error {
	forwarder.mu.Lock()
	gate := forwarder.closeGate
	forwardContext := forwarder.forwardContext
	failCloseOnCanceledContext := forwarder.failCloseOnCanceledContext
	forwarder.mu.Unlock()
	if gate != nil {
		<-gate
	}
	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()
	forwarder.closes++
	if failCloseOnCanceledContext && forwardContext != nil && forwardContext.Err() != nil {
		return errors.New("synthetic forward context canceled before endpoint close")
	}
	if forwarder.closes <= len(forwarder.closeErrs) {
		return forwarder.closeErrs[forwarder.closes-1]
	}
	forwarder.doneOnce.Do(func() { close(forwarder.done) })
	return nil
}

func (forwarder *fakeSDKForwarder) closeCalls() int {
	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()
	return forwarder.closes
}

func protectedStartRequest(reserved string) TunnelStartRequest {
	return TunnelStartRequest{
		UpstreamURL: "http://127.0.0.1:41000", ReservedDomain: reserved,
		AccountToken: []byte("synthetic-scoped-input"),
	}
}

func TestEmbeddedNgrokAdapterForwardsOnlyToPrivateLoopbackWithoutPlayerCredentialsOrPolicy(t *testing.T) {
	for _, test := range []struct{ name, reserved string }{
		{name: "random address omits URL"},
		{name: "reserved address is exact", reserved: "vault.example"},
	} {
		t.Run(test.name, func(t *testing.T) {
			forwarder := newFakeSDKForwarder("https://" + map[bool]string{true: "vault.example", false: "random.example"}[test.reserved != ""])
			agent := &fakeSDKAgent{forwarder: forwarder}
			factory := &fakeSDKFactory{agent: agent}
			endpoint, err := newNgrokServiceWithFactory(factory).Start(t.Context(), protectedStartRequest(test.reserved))
			require.NoError(t, err)
			require.NotNil(t, endpoint)
			assert.Equal(t, []byte("synthetic-scoped-input"), factory.seen)
			assert.Equal(t, "http://127.0.0.1:41000", agent.request.UpstreamURL)
			assert.Equal(t, test.reserved, agent.request.ReservedDomain)
		})
	}
}

func TestEmbeddedNgrokAdapterRejectsMissingAccountTokenOrUnsafePrivateIngressBeforeForward(t *testing.T) {
	for _, mutate := range []func(*TunnelStartRequest){
		func(request *TunnelStartRequest) { request.AccountToken = nil },
		func(request *TunnelStartRequest) { request.UpstreamURL = "https://127.0.0.1:41000" },
		func(request *TunnelStartRequest) { request.UpstreamURL = "http://example.com:41000" },
		func(request *TunnelStartRequest) { request.UpstreamURL = "http://127.0.0.1:41000/path" },
		func(request *TunnelStartRequest) { request.UpstreamURL = "http://127.0.0.1" },
	} {
		agent := &fakeSDKAgent{forwarder: newFakeSDKForwarder("https://random.example")}
		request := protectedStartRequest("")
		mutate(&request)
		endpoint, err := newNgrokServiceWithFactory(&fakeSDKFactory{agent: agent}).Start(t.Context(), request)
		require.Error(t, err)
		assert.Nil(t, endpoint)
		assert.Empty(t, agent.request.UpstreamURL)
	}
}

func TestEmbeddedNgrokAdapterRejectsUnsafeOrMismatchedHTTPSPrivately(t *testing.T) {
	for _, test := range []struct{ endpoint, reserved string }{
		{endpoint: "http://random.example"},
		{endpoint: "https://user@random.example"},
		{endpoint: "https://other.example", reserved: "vault.example"},
	} {
		forwarder := newFakeSDKForwarder(test.endpoint)
		agent := &fakeSDKAgent{forwarder: forwarder}
		endpoint, err := newNgrokServiceWithFactory(&fakeSDKFactory{agent: agent}).Start(t.Context(), protectedStartRequest(test.reserved))
		require.Error(t, err)
		assert.Nil(t, endpoint)
		assert.Equal(t, 1, forwarder.closes)
		assert.Equal(t, 1, agent.disconnects)
	}
}

func TestEmbeddedNgrokEndpointDoneAndIdempotentBoundedCloseDisconnect(t *testing.T) {
	forwarder := newFakeSDKForwarder("https://random.example")
	agent := &fakeSDKAgent{forwarder: forwarder}
	endpoint, err := newNgrokServiceWithFactory(&fakeSDKFactory{agent: agent}).Start(t.Context(), protectedStartRequest(""))
	require.NoError(t, err)
	assert.True(t, endpoint.Done() == forwarder.done)
	require.NoError(t, endpoint.Close(t.Context()))
	require.NoError(t, endpoint.Close(t.Context()))
	assert.Equal(t, 1, forwarder.closes)
	assert.Equal(t, 1, agent.disconnects)
}

func TestEmbeddedNgrokEndpointLifetimeSurvivesCompletedStartupContext(t *testing.T) {
	forwarder := newFakeSDKForwarder("https://random.example")
	agent := &fakeSDKAgent{forwarder: forwarder, trackContext: true}
	ctx, cancel := context.WithCancelCause(t.Context())
	endpoint, err := newNgrokServiceWithFactory(&fakeSDKFactory{agent: agent}).Start(ctx, protectedStartRequest(""))
	require.NoError(t, err)
	cancel(errors.New("test startup request complete"))

	require.Never(t, func() bool {
		select {
		case <-endpoint.Done():
			return true
		default:
			return false
		}
	}, 25*time.Millisecond, time.Millisecond, "completed startup context stopped the owned endpoint")
	require.NoError(t, endpoint.Close(t.Context()))
	require.ErrorIs(t, context.Cause(agent.forwardContext), errEmbeddedEndpointClosed)
}

func TestEmbeddedNgrokEndpointCloseUsesSDKCloseBeforeCancelingOwnedLifetime(t *testing.T) {
	forwarder := newFakeSDKForwarder("https://random.example")
	forwarder.failCloseOnCanceledContext = true
	agent := &fakeSDKAgent{forwarder: forwarder}
	service := newNgrokServiceWithFactory(&fakeSDKFactory{agent: agent})
	endpoint, err := service.Start(t.Context(), protectedStartRequest(""))
	require.NoError(t, err)

	agent.mu.Lock()
	forwarder.mu.Lock()
	forwarder.forwardContext = agent.forwardContext
	forwarder.mu.Unlock()
	agent.mu.Unlock()

	require.NoError(t, endpoint.Close(t.Context()))
}

func TestEmbeddedNgrokEndpointCloseFailureIsRedactedAndRetryable(t *testing.T) {
	forwarder := newFakeSDKForwarder("https://random.example")
	forwarder.closeErrs = []error{errors.New("synthetic forwarder close diagnostic")}
	agent := &fakeSDKAgent{
		forwarder:      forwarder,
		disconnectErrs: []error{errors.New("synthetic agent disconnect diagnostic")},
	}
	endpoint, err := newNgrokServiceWithFactory(&fakeSDKFactory{agent: agent}).Start(t.Context(), protectedStartRequest(""))
	require.NoError(t, err)

	closeErr := endpoint.Close(t.Context())
	require.Error(t, closeErr)
	assert.Equal(t, ErrorProviderFailure.SafeMessage(), closeErr.Error())
	assert.NotContains(t, closeErr.Error(), "synthetic")
	assert.Nil(t, endpoint.URL())
	require.NoError(t, endpoint.Close(t.Context()))
	require.NoError(t, endpoint.Close(t.Context()))
	assert.Equal(t, 2, forwarder.closes)
	assert.Equal(t, 2, agent.disconnects)
}

func TestEmbeddedNgrokAdapterMapsAgentAndForwardFailuresWithoutSDKDiagnostics(t *testing.T) {
	for _, factory := range []*fakeSDKFactory{
		{err: errors.New("sensitive agent diagnostic")},
		{agent: &fakeSDKAgent{forwardErr: errors.New("sensitive forward diagnostic")}},
	} {
		_, err := newNgrokServiceWithFactory(factory).Start(t.Context(), protectedStartRequest(""))
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "sensitive")
	}
}

func TestEmbeddedNgrokAdapterFailureMatrixIsStableAndRedacted(t *testing.T) {
	tests := []struct {
		name     string
		failure  error
		category ErrorCategory
	}{
		{name: "invalid token", failure: fakeNgrokCodedError{code: "ERR_NGROK_105"}, category: ErrorProviderAuthentication},
		{name: "revoked token", failure: fakeNgrokCodedError{code: "ERR_NGROK_107"}, category: ErrorProviderAuthentication},
		{name: "domain conflict", failure: fakeNgrokCodedError{code: "ERR_NGROK_320"}, category: ErrorDomainUnavailable},
		{name: "dns unavailable", failure: &net.DNSError{Err: "synthetic lookup diagnostic", Name: "private.invalid"}, category: ErrorNetworkUnavailable},
		{name: "deadline", failure: context.DeadlineExceeded, category: ErrorTimeout},
		{name: "provider setup", failure: errors.New("synthetic provider setup diagnostic"), category: ErrorProviderFailure},
		{name: "provider failure", failure: fakeNgrokCodedError{code: "ERR_NGROK_999"}, category: ErrorProviderFailure},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factory := &fakeSDKFactory{agent: &fakeSDKAgent{forwardErr: test.failure}}
			_, err := newNgrokServiceWithFactory(factory).Start(t.Context(), protectedStartRequest(""))
			require.Error(t, err)
			category, message := redactedPublicAccessFailure(err)
			assert.Equal(t, test.category, category)
			if coded, ok := test.failure.(fakeNgrokCodedError); ok {
				assert.Contains(t, message, coded.code)
			} else {
				assert.Equal(t, test.category.SafeMessage(), message)
			}
			assert.NotContains(t, message, "synthetic")
			assert.NotContains(t, message, "private.invalid")
		})
	}
}

func TestSDKAgentDisconnectFailureRetainsOnlySafeProviderCode(t *testing.T) {
	canary := "synthetic-token-canary synthetic-password-canary synthetic-domain-canary.example"
	agent := &sdkAgent{}
	agent.handleEvent(&ngrok.EventAgentDisconnected{Error: codedProviderError{
		code: "ERR_NGROK_123", diagnostic: canary,
	}})

	failure := agent.Failure()
	require.Error(t, failure)
	category, message := redactedPublicAccessFailure(failure)
	assert.Equal(t, ErrorProviderAuthentication, category)
	assert.Contains(t, message, "ERR_NGROK_123")
	assert.NotContains(t, message, "synthetic-")
}

func TestEmbeddedNgrokStartCancellationAfterLateForwardClosesAcquiredResources(t *testing.T) {
	forwardGate := make(chan struct{})
	forwarder := newFakeSDKForwarder("https://late.example")
	agent := &fakeSDKAgent{forwarder: forwarder, forwardGate: forwardGate}
	ctx, cancel := context.WithCancelCause(t.Context())
	result := make(chan struct {
		endpoint TunnelEndpoint
		err      error
	}, 1)
	go func() {
		endpoint, err := newNgrokServiceWithFactory(&fakeSDKFactory{agent: agent}).Start(ctx, protectedStartRequest(""))
		result <- struct {
			endpoint TunnelEndpoint
			err      error
		}{endpoint: endpoint, err: err}
	}()
	require.Eventually(t, func() bool {
		agent.mu.Lock()
		defer agent.mu.Unlock()
		return agent.request.UpstreamURL != ""
	}, time.Second, time.Millisecond)
	cancel(errors.New("test late forward canceled"))
	close(forwardGate)
	started := <-result
	require.Error(t, started.err)
	assert.Nil(t, started.endpoint)
	assert.Equal(t, 1, forwarder.closeCalls())
	assert.Equal(t, 1, agent.disconnects)
}

func TestEmbeddedNgrokEndpointConcurrentCloseIsBoundedAndReleasesReferences(t *testing.T) {
	forwarder := newFakeSDKForwarder("https://random.example")
	agent := &fakeSDKAgent{forwarder: forwarder}
	owned, err := newNgrokServiceWithFactory(&fakeSDKFactory{agent: agent}).Start(t.Context(), protectedStartRequest(""))
	require.NoError(t, err)
	endpoint := owned.(*embeddedNgrokEndpoint)

	results := make(chan error, 32)
	for range 32 {
		go func() { results <- endpoint.Close(t.Context()) }()
	}
	for range 32 {
		assert.NoError(t, <-results)
	}
	assert.Equal(t, 1, forwarder.closeCalls())
	assert.Equal(t, 1, agent.disconnects)
	endpoint.stateMu.Lock()
	assert.Nil(t, endpoint.forwarder)
	assert.Nil(t, endpoint.agent)
	endpoint.stateMu.Unlock()
	require.Eventually(t, func() bool {
		select {
		case <-endpoint.Done():
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond, "Done must be closed after successful cleanup")
}

func TestEmbeddedNgrokEndpointCloseDoesNotTrustBlockingSDKComponentsToHonorContext(t *testing.T) {
	closeGate := make(chan struct{})
	var closeGateOnce sync.Once
	t.Cleanup(func() { closeGateOnce.Do(func() { close(closeGate) }) })
	forwarder := newFakeSDKForwarder("https://random.example")
	forwarder.closeGate = closeGate
	agent := &fakeSDKAgent{forwarder: forwarder}
	endpoint, err := newNgrokServiceWithFactory(&fakeSDKFactory{agent: agent}).Start(t.Context(), protectedStartRequest(""))
	require.NoError(t, err)

	deadline, stopDeadline := context.WithTimeoutCause(t.Context(), 25*time.Millisecond, errors.New("test endpoint close timed out"))
	ctx, cancel := context.WithCancelCause(deadline)
	t.Cleanup(func() {
		cancel(errors.New("test endpoint close completed"))
		stopDeadline()
	})
	finished := make(chan error, 1)
	go func() { finished <- endpoint.Close(ctx) }()
	var closeErr error
	require.Eventually(t, func() bool {
		select {
		case closeErr = <-finished:
			return true
		default:
			return false
		}
	}, 250*time.Millisecond, time.Millisecond, "endpoint Close exceeded its bound when the SDK forwarder ignored context")
	require.Error(t, closeErr)
	assert.Equal(t, ErrorShutdownTimeout.SafeMessage(), closeErr.Error())
	require.Eventually(t, func() bool {
		return agent.disconnectCalls() == 1
	}, time.Second, time.Millisecond, "agent disconnect was not attempted while forwarder Close was blocked")

	closeGateOnce.Do(func() { close(closeGate) })
	require.NoError(t, endpoint.Close(t.Context()))
	assert.Equal(t, 1, agent.disconnects)
}
