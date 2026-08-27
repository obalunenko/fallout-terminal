package tunnel

import (
	"context"
	"errors"
	"net/url"
	"sync"

	ngrok "golang.ngrok.com/ngrok/v2"
)

var (
	errEmbeddedContextRequired       = errors.New("embedded tunnel context is required")
	errEmbeddedEndpointStartComplete = errors.New("embedded tunnel endpoint startup complete")
	errEmbeddedEndpointStartAborted  = errors.New("embedded tunnel endpoint startup aborted")
	errEmbeddedEndpointClosed        = errors.New("embedded tunnel endpoint closed")
	errEmbeddedEndpointCloseTimedOut = errors.New("embedded tunnel endpoint close timed out")
)

type ngrokForwardRequest struct {
	UpstreamURL    string
	ReservedDomain string
}

type ngrokForwarder interface {
	URL() *url.URL
	Done() <-chan struct{}
	Close(context.Context) error
}

type ngrokAgent interface {
	Forward(context.Context, ngrokForwardRequest) (ngrokForwarder, error)
	Disconnect() error
}

type ngrokAgentFactory interface {
	New([]byte) (ngrokAgent, error)
}

type sdkAgentFactory struct{}

func (sdkAgentFactory) New(accountToken []byte) (ngrokAgent, error) {
	if len(accountToken) == 0 {
		return nil, errors.New(ErrorCredentialMissing.SafeMessage())
	}
	token := string(accountToken)
	wrapped := &sdkAgent{}
	agent, err := ngrok.NewAgent(
		ngrok.WithAuthtoken(token), ngrok.WithAutoConnect(false), ngrok.WithEventHandler(wrapped.handleEvent),
	)
	if err != nil {
		return nil, newRedactedPublicAccessError(err)
	}
	wrapped.agent = agent
	return wrapped, nil
}

type sdkAgent struct {
	agent     ngrokForwardAgent
	failureMu sync.Mutex
	failure   error
}

type ngrokForwardAgent interface {
	Connect(context.Context) error
	Forward(context.Context, *ngrok.Upstream, ...ngrok.EndpointOption) (ngrok.EndpointForwarder, error)
	Disconnect() error
}

func (agent *sdkAgent) handleEvent(event ngrok.Event) {
	disconnected, ok := event.(*ngrok.EventAgentDisconnected)
	if !ok || disconnected.Error == nil {
		return
	}
	failure := newRedactedPublicAccessError(disconnected.Error)
	agent.failureMu.Lock()
	agent.failure = failure
	agent.failureMu.Unlock()
}

func (agent *sdkAgent) Failure() error {
	if agent == nil {
		return nil
	}
	agent.failureMu.Lock()
	defer agent.failureMu.Unlock()
	return agent.failure
}

func (agent *sdkAgent) Forward(ctx context.Context, request ngrokForwardRequest) (ngrokForwarder, error) {
	if err := agent.agent.Connect(ctx); err != nil {
		return nil, newRedactedPublicAccessError(err)
	}
	options := make([]ngrok.EndpointOption, 0, 1)
	if request.ReservedDomain != "" {
		options = append(options, ngrok.WithURL("https://"+request.ReservedDomain))
	}
	forwarder, err := agent.agent.Forward(ctx, ngrok.WithUpstream(
		request.UpstreamURL,
		ngrok.WithUpstreamProtocol("http2"),
	), options...)
	if err != nil {
		return nil, newRedactedPublicAccessError(err)
	}
	return sdkForwarder{EndpointForwarder: forwarder}, nil
}

func (agent *sdkAgent) Disconnect() error {
	return agent.agent.Disconnect()
}

type sdkForwarder struct {
	ngrok.EndpointForwarder
}

func (forwarder sdkForwarder) Close(ctx context.Context) error {
	return forwarder.CloseWithContext(ctx)
}

type embeddedNgrokService struct {
	factory ngrokAgentFactory
}

type embeddedEndpointLifetime struct {
	startup   context.Context
	context   context.Context
	cancel    context.CancelCauseFunc
	settled   chan struct{}
	settle    sync.Once
	mu        sync.Mutex
	committed bool
}

func newEmbeddedEndpointLifetime(startup context.Context) *embeddedEndpointLifetime {
	ctx, cancel := context.WithCancelCause(context.WithoutCancel(startup))
	lifetime := &embeddedEndpointLifetime{
		startup: startup, context: ctx, cancel: cancel, settled: make(chan struct{}),
	}
	go func() {
		select {
		case <-startup.Done():
			lifetime.mu.Lock()
			if !lifetime.committed {
				lifetime.cancel(context.Cause(startup))
			}
			lifetime.mu.Unlock()
		case <-lifetime.settled:
		}
	}()
	return lifetime
}

func (lifetime *embeddedEndpointLifetime) Commit() bool {
	lifetime.mu.Lock()
	if lifetime.startup.Err() != nil {
		lifetime.cancel(context.Cause(lifetime.startup))
		lifetime.mu.Unlock()
		lifetime.settle.Do(func() { close(lifetime.settled) })
		return false
	}
	lifetime.committed = true
	lifetime.mu.Unlock()
	lifetime.settle.Do(func() { close(lifetime.settled) })
	return true
}

func (lifetime *embeddedEndpointLifetime) Abort(cause error) {
	lifetime.cancel(cause)
	lifetime.settle.Do(func() { close(lifetime.settled) })
}

func NewEmbeddedNgrokService() TunnelService {
	return newNgrokServiceWithFactory(sdkAgentFactory{})
}

func newNgrokServiceWithFactory(factory ngrokAgentFactory) *embeddedNgrokService {
	return &embeddedNgrokService{factory: factory}
}

func (service *embeddedNgrokService) Start(ctx context.Context, request TunnelStartRequest) (TunnelEndpoint, error) {
	if ctx == nil {
		request.Clear()
		return nil, errEmbeddedContextRequired
	}
	ownedToken := append([]byte(nil), request.AccountToken...)
	request.Clear()
	defer clear(ownedToken)

	if service == nil || service.factory == nil {
		return nil, errors.New(ErrorProviderFailure.SafeMessage())
	}
	privateUpstream, err := normalizeLoopbackHTTPURL(request.UpstreamURL)
	if err != nil {
		return nil, errors.New(ErrorValidation.SafeMessage())
	}
	reservedDomain, err := NormalizeReservedDomain(request.ReservedDomain)
	if err != nil {
		return nil, errors.New(ErrorValidation.SafeMessage())
	}
	if len(ownedToken) == 0 {
		return nil, errors.New(ErrorCredentialMissing.SafeMessage())
	}
	if request.Timeout > 0 {
		deadlineContext, stopDeadline := context.WithTimeoutCause(ctx, request.Timeout, errPublicAccessStartTimedOut)
		var cancel context.CancelCauseFunc
		ctx, cancel = context.WithCancelCause(deadlineContext)
		defer func() {
			cancel(errEmbeddedEndpointStartComplete)
			stopDeadline()
		}()
	}

	agent, err := service.factory.New(ownedToken)
	if err != nil {
		return nil, newRedactedPublicAccessError(err)
	}
	lifetime := newEmbeddedEndpointLifetime(ctx)
	forwarder, err := agent.Forward(lifetime.context, ngrokForwardRequest{
		UpstreamURL:    privateUpstream.String(),
		ReservedDomain: reservedDomain,
	})
	if err != nil {
		lifetime.Abort(errEmbeddedEndpointStartAborted)
		_ = agent.Disconnect()
		return nil, newRedactedPublicAccessError(err)
	}
	ownedEndpoint := newEmbeddedNgrokEndpoint(nil, forwarder, agent, lifetime.cancel)
	if ctx.Err() != nil {
		_ = ownedEndpoint.Close(ctx)
		return nil, publicAccessCategorizedError{category: ErrorTimeout}
	}

	publicURL := forwarder.URL()
	if publicURL == nil {
		_ = ownedEndpoint.Close(ctx)
		return nil, errors.New(ErrorProviderFailure.SafeMessage())
	}
	canonicalURL, _, err := NormalizeEndpointURL(publicURL.String(), reservedDomain)
	if err != nil {
		_ = ownedEndpoint.Close(ctx)
		return nil, errors.New(ErrorProviderFailure.SafeMessage())
	}
	parsed, err := url.Parse(canonicalURL)
	if err != nil {
		_ = ownedEndpoint.Close(ctx)
		return nil, errors.New(ErrorProviderFailure.SafeMessage())
	}
	if !lifetime.Commit() {
		_ = ownedEndpoint.Close(ctx)
		return nil, publicAccessCategorizedError{category: ErrorTimeout}
	}
	ownedEndpoint.stateMu.Lock()
	ownedEndpoint.url = parsed
	ownedEndpoint.stateMu.Unlock()
	return ownedEndpoint, nil
}

type embeddedNgrokEndpoint struct {
	url               *url.URL
	forwarder         ngrokForwarder
	agent             ngrokAgent
	stateMu           sync.Mutex
	closeMu           sync.Mutex
	forwarderClosed   bool
	agentDisconnected bool
	closeAttempt      *embeddedNgrokCloseAttempt
	done              <-chan struct{}
	lifetimeCancel    context.CancelCauseFunc
	cancelOnce        sync.Once
}

type embeddedNgrokCloseAttempt struct {
	done                 chan struct{}
	forwarderClosed      bool
	agentDisconnected    bool
	forwarderError       error
	agentDisconnectError error
}

func newEmbeddedNgrokEndpoint(publicURL *url.URL, forwarder ngrokForwarder, agent ngrokAgent, lifetimeCancel ...context.CancelCauseFunc) *embeddedNgrokEndpoint {
	endpoint := &embeddedNgrokEndpoint{
		url: publicURL, forwarder: forwarder, agent: agent,
		done: forwarder.Done(),
	}
	if len(lifetimeCancel) > 0 {
		endpoint.lifetimeCancel = lifetimeCancel[0]
	}
	return endpoint
}

func (endpoint *embeddedNgrokEndpoint) URL() *url.URL {
	if endpoint == nil {
		return nil
	}
	endpoint.stateMu.Lock()
	defer endpoint.stateMu.Unlock()
	if endpoint.url == nil {
		return nil
	}
	copy := *endpoint.url
	return &copy
}

func (endpoint *embeddedNgrokEndpoint) Done() <-chan struct{} {
	if endpoint == nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return endpoint.done
}

func (endpoint *embeddedNgrokEndpoint) Failure() error {
	if endpoint == nil {
		return nil
	}
	endpoint.stateMu.Lock()
	agent := endpoint.agent
	endpoint.stateMu.Unlock()
	source, ok := agent.(interface{ Failure() error })
	if !ok {
		return nil
	}
	return source.Failure()
}

func (endpoint *embeddedNgrokEndpoint) Close(ctx context.Context) error {
	if endpoint == nil {
		return nil
	}
	if ctx == nil {
		return errEmbeddedContextRequired
	}
	ctx, cancel := boundedPublicAccessCleanupContext(ctx)
	defer cancel(errPublicAccessCleanupComplete)

	endpoint.closeMu.Lock()
	endpoint.stateMu.Lock()
	endpoint.url = nil
	complete := endpoint.forwarderClosed && endpoint.agentDisconnected
	endpoint.stateMu.Unlock()
	if complete {
		endpoint.closeMu.Unlock()
		endpoint.cancelLifetime(errEmbeddedEndpointClosed)
		return nil
	}
	attempt := endpoint.closeAttempt
	if attempt == nil {
		endpoint.stateMu.Lock()
		forwarder := endpoint.forwarder
		agent := endpoint.agent
		forwarderClosed := endpoint.forwarderClosed || forwarder == nil
		agentDisconnected := endpoint.agentDisconnected || agent == nil
		endpoint.stateMu.Unlock()
		attempt = &embeddedNgrokCloseAttempt{done: make(chan struct{})}
		endpoint.closeAttempt = attempt
		go runEmbeddedNgrokCloseAttempt(ctx, attempt, forwarder, agent, forwarderClosed, agentDisconnected)
	}
	endpoint.closeMu.Unlock()

	select {
	case <-attempt.done:
		endpoint.cancelLifetime(errEmbeddedEndpointClosed)
	case <-ctx.Done():
		endpoint.cancelLifetime(errEmbeddedEndpointCloseTimedOut)
		return publicAccessCategorizedError{category: ErrorShutdownTimeout}
	}

	endpoint.closeMu.Lock()
	if endpoint.closeAttempt == attempt {
		endpoint.closeAttempt = nil
	}
	endpoint.stateMu.Lock()
	endpoint.forwarderClosed = endpoint.forwarderClosed || attempt.forwarderClosed
	endpoint.agentDisconnected = endpoint.agentDisconnected || attempt.agentDisconnected
	if endpoint.forwarderClosed {
		endpoint.forwarder = nil
	}
	if endpoint.agentDisconnected {
		endpoint.agent = nil
	}
	complete = endpoint.forwarderClosed && endpoint.agentDisconnected
	endpoint.stateMu.Unlock()
	endpoint.closeMu.Unlock()
	if !complete {
		failure := errors.Join(attempt.forwarderError, attempt.agentDisconnectError)
		category, _ := redactedPublicAccessFailure(failure)
		if errors.Is(attempt.forwarderError, context.DeadlineExceeded) || errors.Is(attempt.forwarderError, context.Canceled) {
			category = ErrorShutdownTimeout
		}
		if category == ErrorShutdownTimeout {
			return publicAccessCategorizedError{category: category}
		}
		return newRedactedPublicAccessError(failure)
	}
	return nil
}

func (endpoint *embeddedNgrokEndpoint) cancelLifetime(cause error) {
	endpoint.cancelOnce.Do(func() {
		if endpoint.lifetimeCancel != nil {
			endpoint.lifetimeCancel(cause)
		}
	})
}

func runEmbeddedNgrokCloseAttempt(
	ctx context.Context,
	attempt *embeddedNgrokCloseAttempt,
	forwarder ngrokForwarder,
	agent ngrokAgent,
	forwarderClosed bool,
	agentDisconnected bool,
) {
	if !forwarderClosed {
		forwarderResult := make(chan error, 1)
		go func() { forwarderResult <- forwarder.Close(ctx) }()
		select {
		case attempt.forwarderError = <-forwarderResult:
		case <-ctx.Done():
			if !agentDisconnected {
				attempt.agentDisconnectError = agent.Disconnect()
				agentDisconnected = attempt.agentDisconnectError == nil
			}
			attempt.forwarderError = <-forwarderResult
		}
	}
	if !agentDisconnected {
		attempt.agentDisconnectError = agent.Disconnect()
	}
	attempt.forwarderClosed = forwarderClosed || attempt.forwarderError == nil
	attempt.agentDisconnected = agentDisconnected || attempt.agentDisconnectError == nil
	close(attempt.done)
}
