package tunnel

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	PublicAccessStartupTarget  = 15 * time.Second
	PublicAccessStartupTimeout = 30 * time.Second
	publicAccessCleanupTimeout = 5 * time.Second
)

var (
	errPublicAccessContextRequired = errors.New("public-access context is required")
	errPublicAccessStartCompleted  = errors.New("public-access startup completed")
	errPublicAccessStartTimedOut   = errors.New("public-access startup timed out")
	errPublicAccessStopped         = errors.New("public access stopped")
	errPublicAccessReconfigured    = errors.New("public access reconfigured")
	errPublicAccessCleanupComplete = errors.New("public-access cleanup complete")
	errPublicAccessCleanupTimedOut = errors.New("public-access cleanup timed out")
)

type PublicAccessSettings interface {
	Load() (PublicAccessPreferences, error)
	Save(PublicAccessPreferences) error
}

type ManagerConfig struct {
	Settings    PublicAccessSettings
	Secrets     SecretStore
	Tunnel      TunnelService
	Ingress     PublicIngressFactory
	Publish     SnapshotPublisher
	Clock       Clock
	UpstreamURL string
}

type PublicAccessResult struct {
	OK             bool
	Error          string
	DiagnosticCode PublicAccessDiagnosticCode
	Snapshot       PublicAccessSnapshot
}

type realClock struct{}

func (realClock) Now() time.Time                                { return time.Now() }
func (realClock) After(duration time.Duration) <-chan time.Time { return time.After(duration) }

type startOperation struct {
	done       chan struct{}
	settled    chan struct{}
	settleOnce sync.Once
	generation uint64
	revision   uint64
	context    context.Context
	cancel     context.CancelCauseFunc
	result     PublicAccessResult
}

type stopOperation struct {
	done   chan struct{}
	result PublicAccessResult
}

type cleanupOperation struct {
	done chan struct{}
}

type reconfigureOperation struct {
	done             chan struct{}
	expectedRevision uint64
	generation       uint64
	result           PublicAccessResult
}

// SecretMutation is an ephemeral trusted change. A zero value preserves the
// existing secure-store item; replacement and deletion are mutually exclusive.
type SecretMutation struct {
	Replacement []byte
	Delete      bool
}

// PublicAccessMutation combines one expected settings revision with its
// non-secret replacement and two independent ephemeral secure-store changes.
type PublicAccessMutation struct {
	ExpectedRevision        uint64
	Preferences             PublicAccessPreferences
	ProviderToken           SecretMutation
	PlayerPassword          SecretMutation
	PersistVisibleOverrides bool
}

func (mutation *PublicAccessMutation) clear() {
	if mutation == nil {
		return
	}
	clear(mutation.ProviderToken.Replacement)
	clear(mutation.PlayerPassword.Replacement)
	mutation.ProviderToken.Replacement = nil
	mutation.PlayerPassword.Replacement = nil
}

type startResponse struct {
	endpoint     TunnelEndpoint
	ingress      PublicIngress
	canonicalURL string
	err          error
}

type PublicAccessManager struct {
	mu sync.Mutex

	settings  PublicAccessSettings
	secrets   SecretStore
	tunnel    TunnelService
	ingresses PublicIngressFactory
	publish   SnapshotPublisher
	clock     Clock
	upstream  string

	preferences  PublicAccessPreferences
	provider     SecretPresence
	password     SecretPresence
	status       PublicAccessStatus
	endpoint     TunnelEndpoint
	ingress      PublicIngress
	start        *startOperation
	pendingStart *startOperation
	cleanup      *cleanupOperation
	stop         *stopOperation
	reconfigure  *reconfigureOperation
	initialized  bool
}

func NewPublicAccessManager(config ManagerConfig) (*PublicAccessManager, error) {
	if config.Settings == nil || config.Secrets == nil || config.Tunnel == nil || config.Ingress == nil {
		return nil, errors.New("public-access manager dependencies are incomplete")
	}
	if config.UpstreamURL != "http://"+PlayerUpstreamAddress {
		return nil, errors.New(ErrorValidation.SafeMessage())
	}
	if config.Clock == nil {
		config.Clock = realClock{}
	}
	return &PublicAccessManager{
		settings: config.Settings, secrets: config.Secrets, tunnel: config.Tunnel, ingresses: config.Ingress,
		publish: config.Publish, clock: config.Clock, upstream: config.UpstreamURL,
		preferences: DefaultPublicAccessPreferences(), provider: SecretUnknown, password: SecretUnknown,
		status: PublicAccessStatus{State: LifecycleDisabled},
	}, nil
}

func (manager *PublicAccessManager) Initialize(ctx context.Context) PublicAccessSnapshot {
	if ctx == nil {
		snapshot := manager.Snapshot()
		snapshot.Status = failedStatus(snapshot.Status, ErrorValidation)
		return snapshot
	}
	preferences, settingsErr := manager.settings.Load()
	if settingsErr == nil {
		preferences, settingsErr = preferences.Normalized()
	}
	if settingsErr != nil {
		preferences = DefaultPublicAccessPreferences()
	}
	provider, providerErr := manager.secrets.Presence(ctx, ProviderAccountToken)
	password, passwordErr := manager.secrets.Presence(ctx, PlayerBasicAuthPassword)
	if providerErr != nil {
		provider = SecretUnknown
	}
	if passwordErr != nil {
		password = SecretUnknown
	}

	manager.mu.Lock()
	manager.preferences = preferences
	manager.provider = provider
	manager.password = password
	manager.status = PublicAccessStatus{State: LifecycleDisabled, SettingsRevision: preferences.Revision}
	if settingsErr != nil {
		manager.status = failedStatus(manager.status, ErrorSettingsCorrupt)
	} else if providerErr != nil || passwordErr != nil {
		manager.status = failedStatus(manager.status, secretErrorCategory(errors.Join(providerErr, passwordErr)))
	}
	manager.initialized = true
	snapshot := manager.snapshotLocked()
	manager.mu.Unlock()
	return snapshot
}

func (manager *PublicAccessManager) Snapshot() PublicAccessSnapshot {
	if manager == nil {
		return PublicAccessSnapshot{}
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.snapshotLocked()
}

func (manager *PublicAccessManager) Start(ctx context.Context, expectedRevision uint64) PublicAccessResult {
	return manager.startPublicAccess(ctx, expectedRevision, nil)
}

func (manager *PublicAccessManager) startPublicAccess(ctx context.Context, expectedRevision uint64, owner *reconfigureOperation) PublicAccessResult {
	if manager == nil {
		return PublicAccessResult{Error: ErrorProviderFailure.SafeMessage()}
	}
	if ctx == nil {
		return PublicAccessResult{Error: errPublicAccessContextRequired.Error(), Snapshot: manager.Snapshot()}
	}

	manager.mu.Lock()
	if manager.reconfigure != nil && manager.reconfigure != owner {
		operation := manager.reconfigure
		manager.mu.Unlock()
		select {
		case <-operation.done:
			return manager.startPublicAccess(ctx, expectedRevision, owner)
		case <-ctx.Done():
			return PublicAccessResult{Error: ErrorTimeout.SafeMessage(), Snapshot: manager.Snapshot()}
		}
	}
	if manager.pendingStart != nil && manager.status.State != LifecycleStarting {
		operation := manager.pendingStart
		manager.mu.Unlock()
		select {
		case <-operation.settled:
			return manager.startPublicAccess(ctx, expectedRevision, owner)
		case <-ctx.Done():
			return PublicAccessResult{Error: ErrorTimeout.SafeMessage(), Snapshot: manager.Snapshot()}
		}
	}
	if manager.cleanup != nil {
		operation := manager.cleanup
		manager.mu.Unlock()
		select {
		case <-operation.done:
			return manager.startPublicAccess(ctx, expectedRevision, owner)
		case <-ctx.Done():
			return PublicAccessResult{Error: ErrorTimeout.SafeMessage(), Snapshot: manager.Snapshot()}
		}
	}
	if expectedRevision != manager.preferences.Revision {
		result := manager.conflictResultLocked()
		manager.mu.Unlock()
		return result
	}
	if manager.status.State == LifecycleReady {
		result := manager.resultLocked(true)
		manager.mu.Unlock()
		return result
	}
	if manager.status.State == LifecycleStarting && manager.start != nil {
		operation := manager.start
		manager.mu.Unlock()
		select {
		case <-operation.done:
			return operation.result
		case <-ctx.Done():
			return PublicAccessResult{Error: ErrorTimeout.SafeMessage(), Snapshot: manager.Snapshot()}
		}
	}
	if manager.provider != SecretPresent || manager.password != SecretPresent {
		category := ErrorCredentialMissing
		if manager.provider == SecretUnknown || manager.password == SecretUnknown {
			if secretStoreFailureCategory(manager.status.ErrorCategory) {
				category = manager.status.ErrorCategory
			}
		}
		manager.status.Generation++
		manager.status = failedStatus(manager.status, category)
		result := manager.resultLocked(false)
		manager.mu.Unlock()
		manager.emit(result.Snapshot)
		return result
	}
	if manager.endpoint != nil || manager.ingress != nil {
		endpoint := manager.endpoint
		ingress := manager.ingress
		if ingress != nil {
			ingress.Deny()
		}
		manager.mu.Unlock()
		closeErr := closePublicAccessRuntime(ctx, endpoint, ingress)
		manager.mu.Lock()
		if closeErr == nil {
			if manager.endpoint == endpoint {
				manager.endpoint = nil
			}
			if manager.ingress == ingress {
				manager.ingress = nil
			}
		}
		if closeErr != nil {
			category, message := redactedPublicAccessFailure(closeErr)
			if errors.Is(closeErr, context.DeadlineExceeded) || errors.Is(closeErr, context.Canceled) {
				category = ErrorShutdownTimeout
			}
			manager.status = failedStatus(manager.status, category)
			manager.status.ErrorMessage = message
			result := manager.resultLocked(false)
			manager.mu.Unlock()
			manager.emit(result.Snapshot)
			return result
		}
		manager.mu.Unlock()
		return manager.startPublicAccess(ctx, expectedRevision, owner)
	}

	manager.status.Generation++
	generation := manager.status.Generation
	revision := manager.preferences.Revision
	preferences := manager.preferences
	startContext, cancel := context.WithCancelCause(ctx)
	operation := &startOperation{
		done: make(chan struct{}), settled: make(chan struct{}), generation: generation, revision: revision,
		context: startContext, cancel: cancel,
	}
	manager.start = operation
	manager.pendingStart = operation
	manager.status = PublicAccessStatus{State: LifecycleStarting, Generation: generation, SettingsRevision: revision}
	starting := manager.snapshotLocked()
	manager.mu.Unlock()
	manager.emit(starting)

	response := make(chan startResponse, 1)
	go func() {
		ingress, ingressErr := manager.ingresses.Start(startContext, manager.upstream)
		if ingressErr != nil {
			response <- startResponse{err: ingressErr}
			return
		}
		var endpoint TunnelEndpoint
		var canonicalURL string
		err := WithPublicAccessSecrets(startContext, manager.secrets, func(use *SecretUse) error {
			privateURL := ingress.URL()
			if privateURL == nil {
				return errors.New(ErrorProviderFailure.SafeMessage())
			}
			var startErr error
			endpoint, startErr = manager.tunnel.Start(startContext, TunnelStartRequest{
				UpstreamURL: privateURL.String(), ReservedDomain: preferences.ReservedDomain,
				AccountToken: use.ProviderToken, Timeout: PublicAccessStartupTimeout,
			})
			if startErr != nil {
				return startErr
			}
			if endpoint == nil || endpoint.URL() == nil {
				return errors.New(ErrorProviderFailure.SafeMessage())
			}
			var host string
			canonicalURL, host, startErr = NormalizeEndpointURL(endpoint.URL().String(), preferences.ReservedDomain)
			if startErr != nil {
				return errors.New(ErrorProviderFailure.SafeMessage())
			}
			if startContext.Err() != nil {
				return publicAccessCategorizedError{category: ErrorTimeout}
			}
			return ingress.Activate(host, preferences.Username, use.PlayerPassword)
		})
		response <- startResponse{endpoint: endpoint, ingress: ingress, canonicalURL: canonicalURL, err: err}
	}()

	select {
	case started := <-response:
		cancel(errPublicAccessStartCompleted)
		return manager.finishStart(operation, started.endpoint, started.ingress, started.canonicalURL, started.err)
	case <-manager.clock.After(PublicAccessStartupTimeout):
		cancel(errPublicAccessStartTimedOut)
		manager.failCurrentStart(operation, ErrorTimeout)
		go manager.disposeLateStart(operation, response)
		return manager.completedStartResult(operation)
	case <-ctx.Done():
		cancel(context.Cause(ctx))
		manager.failCurrentStart(operation, ErrorTimeout)
		go manager.disposeLateStart(operation, response)
		return manager.completedStartResult(operation)
	}
}

func (manager *PublicAccessManager) finishStart(
	operation *startOperation,
	endpoint TunnelEndpoint,
	ingress PublicIngress,
	canonicalURL string,
	startErr error,
) PublicAccessResult {
	if startErr == nil && (endpoint == nil || ingress == nil || canonicalURL == "") {
		startErr = errors.New(ErrorProviderFailure.SafeMessage())
	}

	manager.mu.Lock()
	current := manager.start == operation && manager.status.State == LifecycleStarting &&
		manager.status.Generation == operation.generation && manager.preferences.Revision == operation.revision
	if !current {
		result := manager.resultLocked(false)
		manager.mu.Unlock()
		if ingress != nil {
			ingress.Deny()
		}
		if closeErr := closePublicAccessRuntime(operation.context, endpoint, ingress); closeErr != nil {
			manager.retainRuntimeAfterFailedCleanup(endpoint, ingress)
		}
		manager.mu.Lock()
		manager.completeStartLocked(operation, result)
		manager.settleStartLocked(operation)
		manager.mu.Unlock()
		return result
	}
	if startErr != nil {
		category, message := redactedPublicAccessFailure(startErr)
		manager.status = failedStatus(manager.status, category)
		manager.status.ErrorMessage = message
		result := manager.resultLocked(false)
		result.DiagnosticCode = safePublicAccessDiagnosticCode(startErr)
		manager.mu.Unlock()
		if ingress != nil {
			ingress.Deny()
		}
		if closeErr := closePublicAccessRuntime(operation.context, endpoint, ingress); closeErr != nil {
			manager.retainRuntimeAfterFailedCleanup(endpoint, ingress)
		}
		manager.mu.Lock()
		manager.completeStartLocked(operation, result)
		manager.settleStartLocked(operation)
		manager.mu.Unlock()
		manager.emit(result.Snapshot)
		return result
	}

	manager.endpoint = endpoint
	manager.ingress = ingress
	manager.status = PublicAccessStatus{State: LifecycleReady, Generation: operation.generation, SettingsRevision: operation.revision, PublicURL: canonicalURL}
	result := manager.resultLocked(true)
	manager.completeStartLocked(operation, result)
	manager.settleStartLocked(operation)
	manager.mu.Unlock()
	manager.emit(result.Snapshot)
	go manager.monitor(operation.context, operation.generation, endpoint, ingress)
	return result
}

func (manager *PublicAccessManager) Stop(ctx context.Context, expectedRevision uint64) PublicAccessResult {
	if manager == nil {
		return PublicAccessResult{Error: ErrorProviderFailure.SafeMessage()}
	}
	if ctx == nil {
		return PublicAccessResult{Error: errPublicAccessContextRequired.Error(), Snapshot: manager.Snapshot()}
	}
	manager.mu.Lock()
	if manager.reconfigure != nil {
		operation := manager.reconfigure
		manager.mu.Unlock()
		select {
		case <-operation.done:
			return manager.Stop(ctx, expectedRevision)
		case <-ctx.Done():
			return PublicAccessResult{Error: ErrorShutdownTimeout.SafeMessage(), Snapshot: manager.Snapshot()}
		}
	}
	if manager.cleanup != nil {
		cleanup := manager.cleanup
		manager.mu.Unlock()
		select {
		case <-cleanup.done:
			return manager.Stop(ctx, expectedRevision)
		case <-ctx.Done():
			return PublicAccessResult{Error: ErrorShutdownTimeout.SafeMessage(), Snapshot: manager.Snapshot()}
		}
	}
	if expectedRevision != manager.preferences.Revision {
		result := manager.conflictResultLocked()
		manager.mu.Unlock()
		return result
	}
	if manager.stop != nil && manager.status.State == LifecycleStopping {
		operation := manager.stop
		manager.mu.Unlock()
		select {
		case <-operation.done:
			return operation.result
		case <-ctx.Done():
			return PublicAccessResult{Error: ErrorShutdownTimeout.SafeMessage(), Snapshot: manager.Snapshot()}
		}
	}
	if manager.status.State == LifecycleDisabled && manager.endpoint == nil && manager.ingress == nil {
		result := manager.resultLocked(true)
		manager.mu.Unlock()
		return result
	}

	manager.status.Generation++
	generation := manager.status.Generation
	if manager.start != nil && manager.start.cancel != nil {
		manager.start.cancel(errPublicAccessStopped)
	}
	pendingStart := manager.pendingStart
	endpoint := manager.endpoint
	ingress := manager.ingress
	if ingress != nil {
		ingress.Deny()
	}
	manager.status = PublicAccessStatus{State: LifecycleStopping, Generation: generation, SettingsRevision: manager.preferences.Revision}
	operation := &stopOperation{done: make(chan struct{})}
	manager.stop = operation
	stopping := manager.snapshotLocked()
	manager.mu.Unlock()
	manager.emit(stopping)

	closeErr := error(nil)
	if pendingStart != nil {
		select {
		case <-pendingStart.settled:
		case <-ctx.Done():
			closeErr = context.DeadlineExceeded
		}
	}
	if closeErr == nil && endpoint == nil && ingress == nil {
		manager.mu.Lock()
		endpoint = manager.endpoint
		ingress = manager.ingress
		if ingress != nil {
			ingress.Deny()
		}
		manager.mu.Unlock()
	}
	if closeErr == nil && (endpoint != nil || ingress != nil) {
		closeErr = closePublicAccessRuntime(ctx, endpoint, ingress)
	}
	manager.mu.Lock()
	if closeErr != nil {
		category, message := redactedPublicAccessFailure(closeErr)
		if errors.Is(closeErr, context.DeadlineExceeded) || errors.Is(closeErr, context.Canceled) {
			category = ErrorShutdownTimeout
		}
		manager.status = failedStatus(manager.status, category)
		manager.status.ErrorMessage = message
		operation.result = manager.resultLocked(false)
	} else {
		if manager.endpoint == endpoint {
			manager.endpoint = nil
		}
		if manager.ingress == ingress {
			manager.ingress = nil
		}
		manager.status = PublicAccessStatus{State: LifecycleDisabled, Generation: generation, SettingsRevision: manager.preferences.Revision}
		operation.result = manager.resultLocked(true)
	}
	result := operation.result
	manager.stop = nil
	close(operation.done)
	manager.mu.Unlock()
	manager.emit(result.Snapshot)
	return result
}

// Reconfigure performs one protected stop/change/restart transaction. It
// never persists or publishes the ephemeral replacement buffers.
func (manager *PublicAccessManager) Reconfigure(ctx context.Context, mutation PublicAccessMutation) PublicAccessResult {
	if manager == nil {
		mutation.clear()
		return PublicAccessResult{Error: ErrorProviderFailure.SafeMessage()}
	}
	if ctx == nil {
		mutation.clear()
		return PublicAccessResult{Error: errPublicAccessContextRequired.Error(), Snapshot: manager.Snapshot()}
	}
	owned := PublicAccessMutation{
		ExpectedRevision:        mutation.ExpectedRevision,
		Preferences:             mutation.Preferences,
		PersistVisibleOverrides: mutation.PersistVisibleOverrides,
		ProviderToken: SecretMutation{
			Replacement: append([]byte(nil), mutation.ProviderToken.Replacement...), Delete: mutation.ProviderToken.Delete,
		},
		PlayerPassword: SecretMutation{
			Replacement: append([]byte(nil), mutation.PlayerPassword.Replacement...), Delete: mutation.PlayerPassword.Delete,
		},
	}
	mutation.clear()
	defer owned.clear()
	if err := validatePublicAccessMutation(owned); err != nil {
		return manager.mutationFailure(ErrorValidation)
	}

	manager.mu.Lock()
	if manager.reconfigure != nil {
		operation := manager.reconfigure
		manager.mu.Unlock()
		select {
		case <-operation.done:
			if operation.expectedRevision == owned.ExpectedRevision {
				return operation.result
			}
			return manager.Reconfigure(ctx, owned)
		case <-ctx.Done():
			return PublicAccessResult{Error: ErrorTimeout.SafeMessage(), Snapshot: manager.Snapshot()}
		}
	}
	if manager.cleanup != nil {
		cleanup := manager.cleanup
		manager.mu.Unlock()
		select {
		case <-cleanup.done:
			return manager.Reconfigure(ctx, owned)
		case <-ctx.Done():
			return PublicAccessResult{Error: ErrorTimeout.SafeMessage(), Snapshot: manager.Snapshot()}
		}
	}
	if owned.ExpectedRevision != manager.preferences.Revision {
		result := manager.conflictResultLocked()
		manager.mu.Unlock()
		return result
	}
	if manager.stop != nil {
		operation := manager.stop
		manager.mu.Unlock()
		select {
		case <-operation.done:
			return manager.Reconfigure(ctx, owned)
		case <-ctx.Done():
			return PublicAccessResult{Error: ErrorTimeout.SafeMessage(), Snapshot: manager.Snapshot()}
		}
	}

	manager.status.Generation++
	generation := manager.status.Generation
	wasActive := manager.status.State == LifecycleStarting || manager.status.State == LifecycleReady ||
		manager.endpoint != nil || manager.ingress != nil || manager.pendingStart != nil
	if manager.start != nil && manager.start.cancel != nil {
		manager.start.cancel(errPublicAccessReconfigured)
	}
	pendingStart := manager.pendingStart
	endpoint := manager.endpoint
	ingress := manager.ingress
	if ingress != nil {
		ingress.Deny()
	}
	operation := &reconfigureOperation{done: make(chan struct{}), expectedRevision: owned.ExpectedRevision, generation: generation}
	manager.reconfigure = operation
	manager.status = PublicAccessStatus{State: LifecycleStopping, Generation: generation, SettingsRevision: manager.preferences.Revision}
	stopping := manager.snapshotLocked()
	manager.mu.Unlock()
	manager.emit(stopping)

	if pendingStart != nil {
		select {
		case <-pendingStart.settled:
		case <-ctx.Done():
			return manager.finishReconfigureFailure(operation, ErrorTimeout)
		}
	}
	if endpoint == nil && ingress == nil {
		manager.mu.Lock()
		endpoint = manager.endpoint
		ingress = manager.ingress
		if ingress != nil {
			ingress.Deny()
		}
		manager.mu.Unlock()
	}
	if endpoint != nil || ingress != nil {
		if err := closePublicAccessRuntime(ctx, endpoint, ingress); err != nil {
			category, _ := redactedPublicAccessFailure(err)
			return manager.finishReconfigureFailure(operation, category)
		}
		manager.mu.Lock()
		if manager.endpoint == endpoint {
			manager.endpoint = nil
		}
		if manager.ingress == ingress {
			manager.ingress = nil
		}
		manager.mu.Unlock()
	}

	proposed := owned.Preferences
	proposed.Version = PublicAccessSettingsVersion
	proposed.Revision = owned.ExpectedRevision + 1
	proposed.ProviderTokenPresentHint = false
	proposed.PlayerPasswordPresentHint = false
	normalized, err := proposed.Normalized()
	if err != nil {
		return manager.finishReconfigureFailure(operation, ErrorValidation)
	}
	if err := applySecretMutation(ctx, manager.secrets, ProviderAccountToken, owned.ProviderToken); err != nil {
		manager.reconcilePresence(ctx)
		return manager.finishReconfigureFailure(operation, secretErrorCategory(err))
	}
	if err := applySecretMutation(ctx, manager.secrets, PlayerBasicAuthPassword, owned.PlayerPassword); err != nil {
		manager.reconcilePresence(ctx)
		return manager.finishReconfigureFailure(operation, secretErrorCategory(err))
	}
	provider, providerErr := manager.secrets.Presence(ctx, ProviderAccountToken)
	password, passwordErr := manager.secrets.Presence(ctx, PlayerBasicAuthPassword)
	if providerErr != nil || passwordErr != nil {
		manager.setPresence(provider, password)
		return manager.finishReconfigureFailure(operation, secretErrorCategory(errors.Join(providerErr, passwordErr)))
	}
	normalized.ProviderTokenPresentHint = provider == SecretPresent
	normalized.PlayerPasswordPresentHint = password == SecretPresent
	if err := savePublicAccessMutation(manager.settings, normalized, owned.PersistVisibleOverrides); err != nil {
		manager.setPresence(provider, password)
		return manager.finishReconfigureFailure(operation, ErrorSettingsCorrupt)
	}

	manager.mu.Lock()
	if manager.reconfigure != operation || manager.status.Generation != operation.generation {
		result := manager.resultLocked(false)
		manager.completeReconfigureLocked(operation, result)
		manager.mu.Unlock()
		return result
	}
	manager.preferences = normalized
	manager.provider = provider
	manager.password = password
	manager.status = PublicAccessStatus{State: LifecycleDisabled, Generation: generation, SettingsRevision: normalized.Revision}
	disabled := manager.snapshotLocked()
	manager.mu.Unlock()
	manager.emit(disabled)

	if wasActive && provider == SecretPresent && password == SecretPresent {
		result := manager.startPublicAccess(ctx, normalized.Revision, operation)
		manager.mu.Lock()
		manager.completeReconfigureLocked(operation, result)
		manager.mu.Unlock()
		return result
	}
	manager.mu.Lock()
	result := manager.resultLocked(true)
	manager.completeReconfigureLocked(operation, result)
	manager.mu.Unlock()
	return result
}

type mutationAwarePublicAccessSettings interface {
	SaveForMutation(PublicAccessPreferences, bool) error
}

func savePublicAccessMutation(settings PublicAccessSettings, preferences PublicAccessPreferences, persistVisibleOverrides bool) error {
	if aware, ok := settings.(mutationAwarePublicAccessSettings); ok {
		return aware.SaveForMutation(preferences, persistVisibleOverrides)
	}
	return settings.Save(preferences)
}

func (manager *PublicAccessManager) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errPublicAccessContextRequired
	}
	for {
		result := manager.Stop(ctx, manager.Snapshot().Preferences.Revision)
		if result.OK {
			return nil
		}
		if result.Snapshot.Status.ErrorCategory != ErrorConflict || ctx.Err() != nil {
			return errors.New(result.Error)
		}
	}
}

func (manager *PublicAccessManager) monitor(parent context.Context, generation uint64, endpoint TunnelEndpoint, ingress PublicIngress) {
	<-endpoint.Done()
	manager.mu.Lock()
	if manager.endpoint != endpoint || manager.ingress != ingress || manager.status.State != LifecycleReady || manager.status.Generation != generation {
		manager.mu.Unlock()
		return
	}
	ingress.Deny()
	cleanup := &cleanupOperation{done: make(chan struct{})}
	manager.cleanup = cleanup
	manager.status.Generation++
	category, message := publicAccessEndpointFailure(endpoint)
	manager.status = failedStatus(PublicAccessStatus{State: LifecycleFailed, Generation: manager.status.Generation, SettingsRevision: manager.preferences.Revision}, category)
	manager.status.ErrorMessage = message
	snapshot := manager.snapshotLocked()
	manager.mu.Unlock()
	manager.emit(snapshot)

	closeErr := closePublicAccessRuntime(parent, endpoint, ingress)
	manager.mu.Lock()
	if closeErr == nil {
		if manager.endpoint == endpoint {
			manager.endpoint = nil
		}
		if manager.ingress == ingress {
			manager.ingress = nil
		}
	}
	if manager.cleanup == cleanup {
		manager.cleanup = nil
	}
	close(cleanup.done)
	manager.mu.Unlock()
}

func publicAccessEndpointFailure(endpoint TunnelEndpoint) (ErrorCategory, string) {
	source, ok := endpoint.(interface{ Failure() error })
	if !ok {
		return ErrorProviderFailure, ErrorProviderFailure.SafeMessage()
	}
	failure := source.Failure()
	if failure == nil {
		return ErrorProviderFailure, ErrorProviderFailure.SafeMessage()
	}
	return redactedPublicAccessFailure(failure)
}

func boundedPublicAccessCleanupContext(parent context.Context) (context.Context, context.CancelCauseFunc) {
	detached := context.WithoutCancel(parent)
	deadline := time.Now().Add(publicAccessCleanupTimeout)
	if parent.Err() == nil {
		if parentDeadline, ok := parent.Deadline(); ok && parentDeadline.Before(deadline) {
			deadline = parentDeadline
		}
	}
	deadlineContext, stopDeadline := context.WithDeadlineCause(detached, deadline, errPublicAccessCleanupTimedOut)
	ctx, cancel := context.WithCancelCause(deadlineContext)
	return ctx, func(cause error) {
		cancel(cause)
		stopDeadline()
	}
}

func closePublicAccessRuntime(parent context.Context, endpoint TunnelEndpoint, ingress PublicIngress) error {
	ctx, cancel := boundedPublicAccessCleanupContext(parent)
	defer cancel(errPublicAccessCleanupComplete)
	if ingress != nil {
		ingress.Deny()
	}
	endpointErr := closeOwnedPublicAccessResource(ctx, func(closeContext context.Context) error {
		if endpoint == nil {
			return nil
		}
		return endpoint.Close(closeContext)
	})
	ingressErr := closeOwnedPublicAccessResource(ctx, func(closeContext context.Context) error {
		if ingress == nil {
			return nil
		}
		return ingress.Close(closeContext)
	})
	return errors.Join(endpointErr, ingressErr)
}

func closeOwnedPublicAccessResource(ctx context.Context, close func(context.Context) error) error {
	result := make(chan error, 1)
	go func() { result <- close(ctx) }()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (manager *PublicAccessManager) retainRuntimeAfterFailedCleanup(endpoint TunnelEndpoint, ingress PublicIngress) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.endpoint == nil {
		manager.endpoint = endpoint
	}
	if manager.ingress == nil {
		manager.ingress = ingress
	}
}

func (manager *PublicAccessManager) failCurrentStart(operation *startOperation, category ErrorCategory) {
	manager.mu.Lock()
	if manager.start == operation && manager.status.State == LifecycleStarting && manager.status.Generation == operation.generation {
		manager.status = failedStatus(manager.status, category)
		result := manager.resultLocked(false)
		manager.completeStartLocked(operation, result)
		manager.mu.Unlock()
		manager.emit(result.Snapshot)
		return
	}
	result := manager.resultLocked(false)
	manager.completeStartLocked(operation, result)
	manager.mu.Unlock()
}

func (manager *PublicAccessManager) disposeLateStart(operation *startOperation, response <-chan startResponse) {
	started := <-response
	if started.ingress != nil {
		started.ingress.Deny()
	}
	if closeErr := closePublicAccessRuntime(operation.context, started.endpoint, started.ingress); closeErr != nil {
		manager.retainRuntimeAfterFailedCleanup(started.endpoint, started.ingress)
	}
	manager.mu.Lock()
	manager.settleStartLocked(operation)
	manager.mu.Unlock()
}

func (manager *PublicAccessManager) completedStartResult(operation *startOperation) PublicAccessResult {
	<-operation.done
	return operation.result
}

func (manager *PublicAccessManager) completeStartLocked(operation *startOperation, result PublicAccessResult) {
	select {
	case <-operation.done:
		return
	default:
	}
	operation.result = result
	if manager.start == operation {
		manager.start = nil
	}
	close(operation.done)
}

func (manager *PublicAccessManager) settleStartLocked(operation *startOperation) {
	if operation == nil {
		return
	}
	operation.settleOnce.Do(func() { close(operation.settled) })
	if manager.pendingStart == operation {
		manager.pendingStart = nil
	}
}

func (manager *PublicAccessManager) completeReconfigureLocked(operation *reconfigureOperation, result PublicAccessResult) {
	select {
	case <-operation.done:
		return
	default:
	}
	operation.result = result
	if manager.reconfigure == operation {
		manager.reconfigure = nil
	}
	close(operation.done)
}

func (manager *PublicAccessManager) finishReconfigureFailure(operation *reconfigureOperation, category ErrorCategory) PublicAccessResult {
	manager.mu.Lock()
	if manager.reconfigure != operation {
		result := manager.resultLocked(false)
		manager.completeReconfigureLocked(operation, result)
		manager.mu.Unlock()
		return result
	}
	manager.status = failedStatus(PublicAccessStatus{
		State: LifecycleFailed, Generation: operation.generation, SettingsRevision: manager.preferences.Revision,
	}, category)
	result := manager.resultLocked(false)
	manager.completeReconfigureLocked(operation, result)
	manager.mu.Unlock()
	manager.emit(result.Snapshot)
	return result
}

func (manager *PublicAccessManager) mutationFailure(category ErrorCategory) PublicAccessResult {
	manager.mu.Lock()
	snapshot := manager.snapshotLocked()
	manager.mu.Unlock()
	return PublicAccessResult{Error: category.SafeMessage(), Snapshot: snapshot}
}

func (manager *PublicAccessManager) conflictResultLocked() PublicAccessResult {
	snapshot := manager.snapshotLocked()
	snapshot.Status = failedStatus(snapshot.Status, ErrorConflict)
	return PublicAccessResult{Error: ErrorConflict.SafeMessage(), Snapshot: snapshot}
}

func (manager *PublicAccessManager) reconcilePresence(ctx context.Context) {
	provider, providerErr := manager.secrets.Presence(ctx, ProviderAccountToken)
	password, passwordErr := manager.secrets.Presence(ctx, PlayerBasicAuthPassword)
	if providerErr != nil {
		provider = SecretUnknown
	}
	if passwordErr != nil {
		password = SecretUnknown
	}
	manager.setPresence(provider, password)
}

func (manager *PublicAccessManager) setPresence(provider, password SecretPresence) {
	manager.mu.Lock()
	manager.provider = provider
	manager.password = password
	manager.mu.Unlock()
}

func validatePublicAccessMutation(mutation PublicAccessMutation) error {
	if mutation.ProviderToken.Delete && len(mutation.ProviderToken.Replacement) > 0 ||
		mutation.PlayerPassword.Delete && len(mutation.PlayerPassword.Replacement) > 0 {
		return errors.New(ErrorValidation.SafeMessage())
	}
	if len(mutation.ProviderToken.Replacement) > 0 {
		if err := ValidateProviderToken(mutation.ProviderToken.Replacement); err != nil {
			return err
		}
	}
	if len(mutation.PlayerPassword.Replacement) > 0 {
		if err := ValidatePlayerPassword(mutation.PlayerPassword.Replacement); err != nil {
			return err
		}
	}
	preferences := mutation.Preferences
	preferences.Version = PublicAccessSettingsVersion
	_, err := preferences.Normalized()
	return err
}

func applySecretMutation(ctx context.Context, store SecretStore, ref SecretRef, mutation SecretMutation) error {
	if len(mutation.Replacement) > 0 {
		return ReplaceSecret(ctx, store, ref, mutation.Replacement)
	}
	if mutation.Delete {
		return DeleteSecret(ctx, store, ref)
	}
	return nil
}

func (manager *PublicAccessManager) snapshotLocked() PublicAccessSnapshot {
	return PublicAccessSnapshot{
		Preferences: manager.preferences, ProviderTokenPresence: manager.provider,
		PlayerPasswordPresence: manager.password, Status: manager.status,
	}
}

func (manager *PublicAccessManager) resultLocked(ok bool) PublicAccessResult {
	result := PublicAccessResult{OK: ok, Snapshot: manager.snapshotLocked()}
	if !ok {
		result.Error = manager.status.ErrorMessage
		if result.Error == "" {
			result.Error = ErrorProviderFailure.SafeMessage()
		}
	}
	return result
}

func (manager *PublicAccessManager) emit(snapshot PublicAccessSnapshot) {
	if manager.publish != nil {
		manager.publish(snapshot)
	}
}

func failedStatus(status PublicAccessStatus, category ErrorCategory) PublicAccessStatus {
	status.State = LifecycleFailed
	status.PublicURL = ""
	status.ErrorCategory = category
	status.ErrorMessage = category.SafeMessage()
	return status
}

func secretErrorCategory(err error) ErrorCategory {
	switch {
	case errors.Is(err, ErrSecretStoreLocked):
		return ErrorSecretStoreLocked
	case errors.Is(err, ErrSecretStoreDenied), errors.Is(err, ErrSecretStoreUserCancelled):
		return ErrorSecretStoreDenied
	default:
		return ErrorSecretStoreUnavailable
	}
}

func secretStoreFailureCategory(category ErrorCategory) bool {
	return category == ErrorSecretStoreLocked ||
		category == ErrorSecretStoreDenied ||
		category == ErrorSecretStoreUnavailable
}
