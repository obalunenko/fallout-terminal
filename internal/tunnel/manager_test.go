package tunnel_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/obalunenko/Fallout-Terminal/internal/testutil"
	"github.com/obalunenko/Fallout-Terminal/internal/tunnel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memorySettings struct {
	mu          sync.Mutex
	preferences tunnel.PublicAccessPreferences
	saves       int
}

func (settings *memorySettings) Load() (tunnel.PublicAccessPreferences, error) {
	settings.mu.Lock()
	defer settings.mu.Unlock()
	return settings.preferences, nil
}

func (settings *memorySettings) Save(preferences tunnel.PublicAccessPreferences) error {
	settings.mu.Lock()
	defer settings.mu.Unlock()
	settings.preferences = preferences
	settings.saves++
	return nil
}

func (settings *memorySettings) Snapshot() (tunnel.PublicAccessPreferences, int) {
	settings.mu.Lock()
	defer settings.mu.Unlock()
	return settings.preferences, settings.saves
}

type orderedEvents struct {
	mu     sync.Mutex
	events []string
}

func (events *orderedEvents) add(event string) {
	events.mu.Lock()
	defer events.mu.Unlock()
	events.events = append(events.events, event)
}

func (events *orderedEvents) snapshot() []string {
	events.mu.Lock()
	defer events.mu.Unlock()
	return append([]string(nil), events.events...)
}

type scheduledStart struct {
	gate               chan struct{}
	ignoreCancellation bool
	err                error
}

type scheduledTunnelService struct {
	mu          sync.Mutex
	plans       []scheduledStart
	starts      int
	active      int
	maxActive   int
	endpoints   []*scheduledTunnelEndpoint
	events      *orderedEvents
	closeErrors map[int]error
	closeGates  map[int]chan struct{}
	ignoreClose map[int]bool
}

type scheduledTunnelEndpoint struct {
	mu        sync.Mutex
	owner     *scheduledTunnelService
	index     int
	url       *url.URL
	done      chan struct{}
	doneOnce  sync.Once
	closed    bool
	closeCall int
}

type scheduledIngressFactory struct {
	mu        sync.Mutex
	events    *orderedEvents
	starts    int
	active    int
	maxActive int
}

type scheduledIngress struct {
	mu       sync.Mutex
	owner    *scheduledIngressFactory
	index    int
	url      *url.URL
	denied   bool
	closed   bool
	activate int
}

func (factory *scheduledIngressFactory) Start(context.Context, string) (tunnel.PublicIngress, error) {
	factory.mu.Lock()
	factory.starts++
	index := factory.starts
	factory.active++
	if factory.active > factory.maxActive {
		factory.maxActive = factory.active
	}
	factory.mu.Unlock()
	parsed, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", 41000+index))
	if err != nil {
		return nil, err
	}
	factory.events.add(fmt.Sprintf("ingress-start:%d", index))
	return &scheduledIngress{owner: factory, index: index, url: parsed, denied: true}, nil
}

func (ingress *scheduledIngress) URL() *url.URL {
	copyURL := *ingress.url
	return &copyURL
}

func (ingress *scheduledIngress) Activate(host, _ string, _ []byte) error {
	ingress.mu.Lock()
	defer ingress.mu.Unlock()
	ingress.denied = false
	ingress.activate++
	ingress.owner.events.add(fmt.Sprintf("activate:%d:%s", ingress.index, host))
	return nil
}

func (ingress *scheduledIngress) Deny() {
	ingress.mu.Lock()
	defer ingress.mu.Unlock()
	if ingress.denied {
		return
	}
	ingress.denied = true
	ingress.owner.events.add(fmt.Sprintf("deny:%d", ingress.index))
}

func (ingress *scheduledIngress) Close(context.Context) error {
	ingress.mu.Lock()
	defer ingress.mu.Unlock()
	if ingress.closed {
		return nil
	}
	ingress.closed = true
	ingress.owner.mu.Lock()
	ingress.owner.active--
	ingress.owner.mu.Unlock()
	ingress.owner.events.add(fmt.Sprintf("ingress-close:%d", ingress.index))
	return nil
}

func (factory *scheduledIngressFactory) counts() (starts, active, maximum int) {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	return factory.starts, factory.active, factory.maxActive
}

func newScheduledTunnelService(events *orderedEvents, plans ...scheduledStart) *scheduledTunnelService {
	return &scheduledTunnelService{
		plans: plans, events: events, closeErrors: make(map[int]error),
		closeGates: make(map[int]chan struct{}), ignoreClose: make(map[int]bool),
	}
}

func (service *scheduledTunnelService) Start(ctx context.Context, request tunnel.TunnelStartRequest) (tunnel.TunnelEndpoint, error) {
	service.mu.Lock()
	service.starts++
	index := service.starts
	var plan scheduledStart
	if index <= len(service.plans) {
		plan = service.plans[index-1]
	}
	service.mu.Unlock()

	if plan.gate != nil {
		if plan.ignoreCancellation {
			<-plan.gate
		} else {
			select {
			case <-plan.gate:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	if plan.err != nil {
		return nil, plan.err
	}
	host := request.ReservedDomain
	if host == "" {
		host = fmt.Sprintf("public-%d.example", index)
	}
	parsed, err := url.Parse("https://" + host)
	if err != nil {
		return nil, err
	}
	endpoint := &scheduledTunnelEndpoint{owner: service, index: index, url: parsed, done: make(chan struct{})}
	service.mu.Lock()
	service.active++
	if service.active > service.maxActive {
		service.maxActive = service.active
	}
	service.endpoints = append(service.endpoints, endpoint)
	service.mu.Unlock()
	service.events.add(fmt.Sprintf("start:%d", index))
	return endpoint, nil
}

func (service *scheduledTunnelService) counts() (starts, active, maximum int) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.starts, service.active, service.maxActive
}

func (service *scheduledTunnelService) setCloseError(index int, err error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.closeErrors[index] = err
}

func (service *scheduledTunnelService) setCloseGate(index int, gate chan struct{}, ignoreContext bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.closeGates[index] = gate
	service.ignoreClose[index] = ignoreContext
}

func (endpoint *scheduledTunnelEndpoint) URL() *url.URL {
	endpoint.mu.Lock()
	defer endpoint.mu.Unlock()
	copyURL := *endpoint.url
	return &copyURL
}

func (endpoint *scheduledTunnelEndpoint) Done() <-chan struct{} { return endpoint.done }

func (endpoint *scheduledTunnelEndpoint) Close(ctx context.Context) error {
	endpoint.owner.mu.Lock()
	gate := endpoint.owner.closeGates[endpoint.index]
	ignoreContext := endpoint.owner.ignoreClose[endpoint.index]
	closeErr := endpoint.owner.closeErrors[endpoint.index]
	endpoint.owner.mu.Unlock()
	if gate != nil {
		if ignoreContext {
			<-gate
		} else {
			select {
			case <-gate:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	endpoint.mu.Lock()
	endpoint.closeCall++
	if endpoint.closed {
		endpoint.mu.Unlock()
		return nil
	}
	if closeErr != nil {
		endpoint.mu.Unlock()
		return closeErr
	}
	endpoint.owner.events.add(fmt.Sprintf("close:%d", endpoint.index))
	endpoint.owner.mu.Lock()
	endpoint.owner.active--
	endpoint.owner.mu.Unlock()
	endpoint.doneOnce.Do(func() { close(endpoint.done) })
	// Publish the idempotent closed state only after every cleanup side effect
	// is complete. A concurrent retry that observes closed must be able to
	// trust that active resource accounting and Done have already settled.
	endpoint.closed = true
	endpoint.mu.Unlock()
	return nil
}

func newScheduledManager(t *testing.T, service tunnel.TunnelService, events *orderedEvents) (*tunnel.PublicAccessManager, *memorySettings) {
	t.Helper()
	preferences := tunnel.DefaultPublicAccessPreferences()
	preferences.Revision = 7
	settings := &memorySettings{preferences: preferences}
	secrets := testutil.NewFakeSecretStore()
	require.NoError(t, secrets.Replace(t.Context(), tunnel.ProviderAccountToken, []byte("synthetic-account-input")))
	require.NoError(t, secrets.Replace(t.Context(), tunnel.PlayerBasicAuthPassword, []byte("synthetic-player-input")))
	manager, err := tunnel.NewPublicAccessManager(tunnel.ManagerConfig{
		Settings: settings, Secrets: secrets, Tunnel: service,
		Ingress: &scheduledIngressFactory{events: events}, UpstreamURL: "http://127.0.0.1:3690",
		Publish: func(snapshot tunnel.PublicAccessSnapshot) {
			events.add(fmt.Sprintf("publish:%d:%t:%d", snapshot.Status.State, snapshot.Status.PublicURL != "", snapshot.Preferences.Revision))
		},
	})
	require.NoError(t, err)
	require.Equal(t, tunnel.LifecycleDisabled, manager.Initialize(t.Context()).Status.State)
	return manager, settings
}

type managerFixture struct {
	manager   *tunnel.PublicAccessManager
	service   *testutil.FakeTunnelService
	endpoint  *testutil.FakeTunnelEndpoint
	publisher *testutil.FakeSnapshotPublisher
	clock     *testutil.FakeClock
}

func newManagerFixture(t *testing.T) managerFixture {
	t.Helper()
	preferences := tunnel.DefaultPublicAccessPreferences()
	preferences.Revision = 7
	settings := &memorySettings{preferences: preferences}
	secrets := testutil.NewFakeSecretStore()
	require.NoError(t, secrets.Replace(t.Context(), tunnel.ProviderAccountToken, []byte("synthetic-account-input")))
	require.NoError(t, secrets.Replace(t.Context(), tunnel.PlayerBasicAuthPassword, []byte("synthetic-player-input")))
	endpoint := testutil.NewFakeTunnelEndpoint("https://public.example")
	service := testutil.NewFakeTunnelService(endpoint)
	publisher := testutil.NewFakeSnapshotPublisher()
	clock := testutil.NewFakeClock(time.Unix(100, 0))
	events := &orderedEvents{}
	manager, err := tunnel.NewPublicAccessManager(tunnel.ManagerConfig{
		Settings: settings, Secrets: secrets, Tunnel: service,
		Ingress: &scheduledIngressFactory{events: events}, Publish: publisher.Publish, Clock: clock,
		UpstreamURL: "http://127.0.0.1:3690",
	})
	require.NoError(t, err)
	require.Equal(t, tunnel.LifecycleDisabled, manager.Initialize(t.Context()).Status.State)
	return managerFixture{manager: manager, service: service, endpoint: endpoint, publisher: publisher, clock: clock}
}

func TestPublicAccessManagerPublishesPrivateStartingThenReadyOnlyAfterProtectedEndpointAcquisition(t *testing.T) {
	fixture := newManagerFixture(t)
	result := fixture.manager.Start(t.Context(), 7)
	require.True(t, result.OK, result.Error)
	require.Equal(t, tunnel.LifecycleReady, result.Snapshot.Status.State)
	require.Equal(t, "https://public.example", result.Snapshot.Status.PublicURL)
	require.Equal(t, uint64(1), result.Snapshot.Status.Generation)
	assert.Equal(t, "http://127.0.0.1:41001", fixture.service.LastUpstreamURL())
	assert.True(t, fixture.service.LastStartSecretsCleared())

	snapshots := fixture.publisher.Snapshots()
	require.GreaterOrEqual(t, len(snapshots), 2)
	assert.Equal(t, tunnel.LifecycleStarting, snapshots[0].Status.State)
	assert.Empty(t, snapshots[0].Status.PublicURL)
	assert.Equal(t, tunnel.LifecycleReady, snapshots[len(snapshots)-1].Status.State)
	assert.Equal(t, "https://public.example", snapshots[len(snapshots)-1].Status.PublicURL)
}

func TestPublicAccessManagerActivatesBeforePublishAndDeniesBeforeEndpointClose(t *testing.T) {
	events := &orderedEvents{}
	service := newScheduledTunnelService(events)
	ingresses := &scheduledIngressFactory{events: events}
	preferences := tunnel.DefaultPublicAccessPreferences()
	preferences.Revision = 7
	settings := &memorySettings{preferences: preferences}
	secrets := testutil.NewFakeSecretStore()
	require.NoError(t, secrets.Replace(t.Context(), tunnel.ProviderAccountToken, []byte("synthetic-account-input")))
	require.NoError(t, secrets.Replace(t.Context(), tunnel.PlayerBasicAuthPassword, []byte("synthetic-player-input")))
	manager, err := tunnel.NewPublicAccessManager(tunnel.ManagerConfig{
		Settings: settings, Secrets: secrets, Tunnel: service, Ingress: ingresses,
		UpstreamURL: "http://127.0.0.1:3690",
		Publish: func(snapshot tunnel.PublicAccessSnapshot) {
			events.add(fmt.Sprintf("publish:%d:%t:%d", snapshot.Status.State, snapshot.Status.PublicURL != "", snapshot.Preferences.Revision))
		},
	})
	require.NoError(t, err)
	manager.Initialize(t.Context())
	require.True(t, manager.Start(t.Context(), 7).OK)
	require.True(t, manager.Stop(t.Context(), 7).OK)

	ordered := events.snapshot()
	activate := indexOfEvent(ordered, "activate:1:public-1.example")
	ready := indexOfEvent(ordered, fmt.Sprintf("publish:%d:true:7", tunnel.LifecycleReady))
	deny := indexOfEvent(ordered, "deny:1")
	withdraw := indexOfEvent(ordered, fmt.Sprintf("publish:%d:false:7", tunnel.LifecycleStopping))
	endpointClose := indexOfEvent(ordered, "close:1")
	ingressClose := indexOfEvent(ordered, "ingress-close:1")
	assert.GreaterOrEqual(t, activate, 0, ordered)
	assert.Greater(t, ready, activate, ordered)
	assert.Greater(t, withdraw, deny, ordered)
	assert.Greater(t, endpointClose, withdraw, ordered)
	assert.Greater(t, ingressClose, endpointClose, ordered)
	starts, active, maximum := ingresses.counts()
	assert.Equal(t, 1, starts)
	assert.Equal(t, 0, active)
	assert.Equal(t, 1, maximum)
}

func TestPublicAccessManagerRejectsStaleRevisionAndJoinsRepeatedStartStop(t *testing.T) {
	fixture := newManagerFixture(t)
	conflict := fixture.manager.Start(t.Context(), 6)
	require.False(t, conflict.OK)
	assert.Equal(t, tunnel.ErrorConflict, conflict.Snapshot.Status.ErrorCategory)
	assert.Equal(t, 0, fixture.service.StartCalls())

	require.True(t, fixture.manager.Start(t.Context(), 7).OK)
	require.True(t, fixture.manager.Start(t.Context(), 7).OK)
	assert.Equal(t, 1, fixture.service.StartCalls())
	assert.LessOrEqual(t, fixture.service.ActiveEndpoints(), 1)

	require.True(t, fixture.manager.Stop(t.Context(), 7).OK)
	require.True(t, fixture.manager.Stop(t.Context(), 7).OK)
	assert.Equal(t, tunnel.LifecycleDisabled, fixture.manager.Snapshot().Status.State)
	assert.Empty(t, fixture.manager.Snapshot().Status.PublicURL)
	assert.Equal(t, 0, fixture.service.ActiveEndpoints())
}

func TestPublicAccessManagerStopSupersedesBlockedStartAndStaleCompletionCannotPublish(t *testing.T) {
	fixture := newManagerFixture(t)
	fixture.service.DelayStart()
	startDone := make(chan tunnel.PublicAccessResult, 1)
	startContext := t.Context()
	go func() { startDone <- fixture.manager.Start(startContext, 7) }()
	require.Eventually(t, func() bool {
		return fixture.manager.Snapshot().Status.State == tunnel.LifecycleStarting
	}, time.Second, time.Millisecond)

	stop := fixture.manager.Stop(t.Context(), 7)
	require.True(t, stop.OK, stop.Error)
	fixture.service.ReleaseStart()
	result := <-startDone
	assert.False(t, result.OK)
	assert.Equal(t, tunnel.LifecycleDisabled, fixture.manager.Snapshot().Status.State)
	assert.Empty(t, fixture.manager.Snapshot().Status.PublicURL)
	assert.Equal(t, 0, fixture.service.ActiveEndpoints())
}

func TestPublicAccessManagerEnforcesThirtySecondBoundAndRedactsProviderFailure(t *testing.T) {
	require.Equal(t, 15*time.Second, tunnel.PublicAccessStartupTarget)
	require.Equal(t, 30*time.Second, tunnel.PublicAccessStartupTimeout)

	fixture := newManagerFixture(t)
	fixture.service.DelayStart()
	done := make(chan tunnel.PublicAccessResult, 1)
	startContext := t.Context()
	go func() { done <- fixture.manager.Start(startContext, 7) }()
	require.Eventually(t, func() bool { return fixture.service.StartCalls() == 1 }, time.Second, time.Millisecond)
	fixture.clock.Advance(tunnel.PublicAccessStartupTimeout)
	result := <-done
	require.False(t, result.OK)
	assert.Equal(t, tunnel.ErrorTimeout, result.Snapshot.Status.ErrorCategory)
	assert.NotContains(t, result.Error, "synthetic-account-input")
	assert.Empty(t, result.Snapshot.Status.PublicURL)

	failure := newManagerFixture(t)
	failure.service.StartErr = errors.New("provider diagnostic with sensitive-marker")
	failed := failure.manager.Start(t.Context(), 7)
	require.False(t, failed.OK)
	assert.Equal(t, tunnel.ErrorProviderFailure, failed.Snapshot.Status.ErrorCategory)
	assert.NotContains(t, failed.Error, "sensitive-marker")
}

func TestPublicAccessManagerUnexpectedDoneWithdrawsBeforeFailedPublication(t *testing.T) {
	fixture := newManagerFixture(t)
	require.True(t, fixture.manager.Start(t.Context(), 7).OK)
	fixture.endpoint.Complete()
	require.Eventually(t, func() bool {
		return fixture.manager.Snapshot().Status.State == tunnel.LifecycleFailed
	}, time.Second, time.Millisecond)
	status := fixture.manager.Snapshot().Status
	assert.Empty(t, status.PublicURL)
	assert.Equal(t, tunnel.ErrorProviderFailure, status.ErrorCategory)
}

type diagnosticEndpointFailure struct {
	code string
}

func (failure diagnosticEndpointFailure) Error() string {
	return "synthetic sensitive provider diagnostic"
}
func (failure diagnosticEndpointFailure) Code() string { return failure.code }

type diagnosticEndpoint struct {
	publicURL *url.URL
	done      chan struct{}
	failure   error
}

func (endpoint *diagnosticEndpoint) URL() *url.URL               { return endpoint.publicURL }
func (endpoint *diagnosticEndpoint) Done() <-chan struct{}       { return endpoint.done }
func (endpoint *diagnosticEndpoint) Failure() error              { return endpoint.failure }
func (endpoint *diagnosticEndpoint) Close(context.Context) error { return nil }

type diagnosticTunnelService struct{ endpoint *diagnosticEndpoint }

func (service diagnosticTunnelService) Start(context.Context, tunnel.TunnelStartRequest) (tunnel.TunnelEndpoint, error) {
	return service.endpoint, nil
}

func TestPublicAccessManagerUnexpectedDonePublishesOnlySafeNgrokCode(t *testing.T) {
	publicURL, err := url.Parse("https://public.example")
	require.NoError(t, err)
	endpoint := &diagnosticEndpoint{
		publicURL: publicURL,
		done:      make(chan struct{}),
		failure:   diagnosticEndpointFailure{code: "ERR_NGROK_123"},
	}
	preferences := tunnel.DefaultPublicAccessPreferences()
	preferences.Revision = 7
	secrets := testutil.NewFakeSecretStore()
	require.NoError(t, secrets.Replace(t.Context(), tunnel.ProviderAccountToken, []byte("synthetic-account-input")))
	require.NoError(t, secrets.Replace(t.Context(), tunnel.PlayerBasicAuthPassword, []byte("synthetic-player-input")))
	manager, err := tunnel.NewPublicAccessManager(tunnel.ManagerConfig{
		Settings: &memorySettings{preferences: preferences}, Secrets: secrets,
		Tunnel:  diagnosticTunnelService{endpoint: endpoint},
		Ingress: &scheduledIngressFactory{events: &orderedEvents{}}, UpstreamURL: "http://127.0.0.1:3690",
	})
	require.NoError(t, err)
	manager.Initialize(t.Context())
	require.True(t, manager.Start(t.Context(), 7).OK)
	close(endpoint.done)
	require.Eventually(t, func() bool { return manager.Snapshot().Status.State == tunnel.LifecycleFailed }, time.Second, time.Millisecond)
	status := manager.Snapshot().Status
	assert.Equal(t, tunnel.ErrorProviderAuthentication, status.ErrorCategory)
	assert.Contains(t, status.ErrorMessage, "ERR_NGROK_123")
	assert.NotContains(t, status.ErrorMessage, "synthetic")
}

func TestPublicAccessManagerConcurrentReconfigureConvergesWithoutEndpointOverlapAcross100Schedules(t *testing.T) {
	for schedule := range 100 {
		events := &orderedEvents{}
		service := newScheduledTunnelService(events)
		manager, settings := newScheduledManager(t, service, events)
		require.True(t, manager.Start(t.Context(), 7).OK)

		mutation := tunnel.PublicAccessMutation{
			ExpectedRevision: 7,
			Preferences: tunnel.PublicAccessPreferences{
				Version: tunnel.PublicAccessSettingsVersion, EnabledPreference: true,
				ReservedDomain: fmt.Sprintf("round-%d.example", schedule), Username: fmt.Sprintf("players-%d", schedule),
			},
		}
		start := make(chan struct{})
		results := make(chan tunnel.PublicAccessResult, 4)
		for range 4 {
			go func() {
				<-start
				results <- manager.Reconfigure(t.Context(), mutation)
			}()
		}
		close(start)
		for range 4 {
			<-results
		}

		snapshot := manager.Snapshot()
		require.Equalf(t, tunnel.LifecycleReady, snapshot.Status.State, "schedule %d snapshot = %#v", schedule, snapshot)
		assert.Equalf(t, uint64(8), snapshot.Preferences.Revision, "schedule %d", schedule)
		assert.Equalf(t, fmt.Sprintf("https://round-%d.example", schedule), snapshot.Status.PublicURL, "schedule %d", schedule)
		starts, active, maximum := service.counts()
		assert.Equalf(t, 2, starts, "schedule %d", schedule)
		assert.Equalf(t, 1, active, "schedule %d", schedule)
		assert.LessOrEqualf(t, maximum, 1, "schedule %d", schedule)
		persisted, saves := settings.Snapshot()
		assert.Equalf(t, uint64(8), persisted.Revision, "schedule %d", schedule)
		assert.Equalf(t, 1, saves, "schedule %d", schedule)
		require.True(t, manager.Stop(t.Context(), 8).OK)

		ordered := events.snapshot()
		stoppingIndex := indexOfEvent(ordered, fmt.Sprintf("publish:%d:false:7", tunnel.LifecycleStopping))
		closeIndex := indexOfEvent(ordered, "close:1")
		replacementIndex := indexOfEvent(ordered, "start:2")
		assert.GreaterOrEqualf(t, stoppingIndex, 0, "schedule %d events = %#v", schedule, ordered)
		assert.Greaterf(t, closeIndex, stoppingIndex, "schedule %d events = %#v", schedule, ordered)
		assert.Greaterf(t, replacementIndex, closeIndex, "schedule %d events = %#v", schedule, ordered)
	}
}

func TestPublicAccessManagerGeneratedPasswordDoesNotPersistDevelopmentVisibleOverrides(t *testing.T) {
	stored := tunnel.DefaultPublicAccessPreferences()
	stored.ReservedDomain = "stored.example"
	stored.Username = "stored-players"
	stored.Revision = 7
	base := &memorySettings{preferences: stored}
	secrets := testutil.NewFakeSecretStore()
	override := tunnel.NewDevelopmentTestPublicAccessOverride(base, secrets, func(name string) (string, bool) {
		values := map[string]string{
			tunnel.DevelopmentNgrokAuthtokenEnvironment: "synthetic-environment-provider-token",
			tunnel.DevelopmentNgrokDomainEnvironment:    "override.example",
			tunnel.DevelopmentPlayerUsernameEnvironment: "override-players",
			tunnel.DevelopmentPlayerPasswordEnvironment: "synthetic-environment-player-password",
		}
		value, ok := values[name]
		return value, ok
	})
	ingresses := testutil.NewFakePublicIngressFactory()
	manager, err := tunnel.NewPublicAccessManager(tunnel.ManagerConfig{
		Settings: override, Secrets: override,
		Tunnel:  testutil.NewFakeTunnelService(testutil.NewFakeTunnelEndpoint("https://override.example")),
		Ingress: ingresses, UpstreamURL: "http://127.0.0.1:3690",
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.Zero(t, ingresses.ActiveIngresses()) })
	t.Cleanup(func() { require.NoError(t, manager.Shutdown(context.WithoutCancel(t.Context()))) })
	effective := manager.Initialize(t.Context())
	require.Equal(t, "override.example", effective.Preferences.ReservedDomain)
	require.Equal(t, "override-players", effective.Preferences.Username)
	require.True(t, manager.Start(t.Context(), effective.Preferences.Revision).OK)
	require.True(t, manager.Stop(t.Context(), effective.Preferences.Revision).OK)
	persisted, saves := base.Snapshot()
	assert.Equal(t, stored, persisted)
	assert.Zero(t, saves, "Load, Start, and Stop must not persist visible overrides")

	generated := manager.Reconfigure(t.Context(), tunnel.PublicAccessMutation{
		ExpectedRevision: effective.Preferences.Revision,
		Preferences:      effective.Preferences,
		PlayerPassword:   tunnel.SecretMutation{Replacement: []byte("synthetic-generated-player-password")},
	})
	require.True(t, generated.OK, generated.Error)
	persisted, saves = base.Snapshot()
	assert.Equal(t, "stored.example", persisted.ReservedDomain)
	assert.Equal(t, "stored-players", persisted.Username)
	assert.Equal(t, uint64(8), persisted.Revision)
	assert.True(t, persisted.PlayerPasswordPresentHint)
	assert.Equal(t, 1, saves)
	assert.Equal(t, "override.example", generated.Snapshot.Preferences.ReservedDomain)
	assert.Equal(t, "override-players", generated.Snapshot.Preferences.Username)

	explicitSave := manager.Reconfigure(t.Context(), tunnel.PublicAccessMutation{
		ExpectedRevision:        generated.Snapshot.Preferences.Revision,
		Preferences:             generated.Snapshot.Preferences,
		PersistVisibleOverrides: true,
	})
	require.True(t, explicitSave.OK, explicitSave.Error)
	persisted, saves = base.Snapshot()
	assert.Equal(t, "override.example", persisted.ReservedDomain)
	assert.Equal(t, "override-players", persisted.Username)
	assert.Equal(t, uint64(9), persisted.Revision)
	assert.Equal(t, 2, saves)
}

func TestPublicAccessManagerReconfigureDisposesLateStartBeforeReplacement(t *testing.T) {
	for _, lateErr := range []error{nil, errors.New("late synthetic provider failure")} {
		t.Run(fmt.Sprintf("late-error-%t", lateErr != nil), func(t *testing.T) {
			events := &orderedEvents{}
			gate := make(chan struct{})
			service := newScheduledTunnelService(events,
				scheduledStart{gate: gate, ignoreCancellation: true, err: lateErr},
				scheduledStart{},
			)
			manager, _ := newScheduledManager(t, service, events)
			started := make(chan tunnel.PublicAccessResult, 1)
			go func() { started <- manager.Start(t.Context(), 7) }()
			require.Eventually(t, func() bool { return manager.Snapshot().Status.State == tunnel.LifecycleStarting }, time.Second, time.Millisecond)

			reconfigured := make(chan tunnel.PublicAccessResult, 1)
			go func() {
				reconfigured <- manager.Reconfigure(t.Context(), tunnel.PublicAccessMutation{
					ExpectedRevision: 7,
					Preferences:      tunnel.PublicAccessPreferences{Version: 1, Username: "replacement", ReservedDomain: "replacement.example"},
				})
			}()
			require.Eventually(t, func() bool { return manager.Snapshot().Status.State == tunnel.LifecycleStopping }, time.Second, time.Millisecond)
			close(gate)
			assert.False(t, (<-started).OK)
			require.True(t, (<-reconfigured).OK)
			assert.Equal(t, "https://replacement.example", manager.Snapshot().Status.PublicURL)
			_, active, maximum := service.counts()
			assert.Equal(t, 1, active)
			assert.LessOrEqual(t, maximum, 1)
		})
	}
}

func TestPublicAccessManagerCloseFailureRetainsOwnershipForRetryBeforeReplacement(t *testing.T) {
	events := &orderedEvents{}
	service := newScheduledTunnelService(events)
	manager, settings := newScheduledManager(t, service, events)
	require.True(t, manager.Start(t.Context(), 7).OK)
	service.setCloseError(1, errors.New("synthetic close failure"))
	mutation := tunnel.PublicAccessMutation{
		ExpectedRevision: 7,
		Preferences:      tunnel.PublicAccessPreferences{Version: 1, Username: "replacement", ReservedDomain: "replacement.example"},
	}

	failed := manager.Reconfigure(t.Context(), mutation)
	require.False(t, failed.OK)
	assert.Empty(t, failed.Snapshot.Status.PublicURL)
	assert.Equal(t, uint64(7), failed.Snapshot.Preferences.Revision)
	_, saves := settings.Snapshot()
	assert.Equal(t, 0, saves)
	starts, active, maximum := service.counts()
	assert.Equal(t, 1, starts)
	assert.Equal(t, 1, active)
	assert.Equal(t, 1, maximum)

	service.setCloseError(1, nil)
	retried := manager.Reconfigure(t.Context(), mutation)
	require.True(t, retried.OK, retried.Error)
	assert.Equal(t, uint64(8), retried.Snapshot.Preferences.Revision)
	starts, active, maximum = service.counts()
	assert.Equal(t, 2, starts)
	assert.Equal(t, 1, active)
	assert.Equal(t, 1, maximum)
}

func TestPublicAccessManagerShutdownDuringReconfigureUsesTheCompletedRevisionAndJoinsCleanup(t *testing.T) {
	events := &orderedEvents{}
	service := newScheduledTunnelService(events)
	manager, _ := newScheduledManager(t, service, events)
	require.True(t, manager.Start(t.Context(), 7).OK)
	closeGate := make(chan struct{})
	var closeGateOnce sync.Once
	t.Cleanup(func() { closeGateOnce.Do(func() { close(closeGate) }) })
	service.setCloseGate(1, closeGate, false)

	reconfigured := make(chan tunnel.PublicAccessResult, 1)
	go func() {
		reconfigured <- manager.Reconfigure(t.Context(), tunnel.PublicAccessMutation{
			ExpectedRevision: 7,
			Preferences: tunnel.PublicAccessPreferences{
				Version: 1, Username: "replacement", ReservedDomain: "replacement.example",
			},
		})
	}()
	require.Eventually(t, func() bool {
		return manager.Snapshot().Status.State == tunnel.LifecycleStopping
	}, time.Second, time.Millisecond)

	shutdowns := make(chan error, 4)
	for range 4 {
		go func() { shutdowns <- manager.Shutdown(t.Context()) }()
	}
	closeGateOnce.Do(func() { close(closeGate) })
	require.True(t, (<-reconfigured).OK)
	for range 4 {
		assert.NoError(t, <-shutdowns)
	}
	status := manager.Snapshot().Status
	assert.Equal(t, tunnel.LifecycleDisabled, status.State)
	assert.Equal(t, uint64(8), status.SettingsRevision)
	assert.Empty(t, status.PublicURL)
	starts, active, maximum := service.counts()
	assert.Equal(t, 2, starts)
	assert.Equal(t, 0, active)
	assert.LessOrEqual(t, maximum, 1)
}

func TestPublicAccessManagerShutdownBoundsContextIgnoringCloseAndRetainsRetryOwnership(t *testing.T) {
	events := &orderedEvents{}
	service := newScheduledTunnelService(events)
	manager, _ := newScheduledManager(t, service, events)
	require.True(t, manager.Start(t.Context(), 7).OK)
	closeGate := make(chan struct{})
	var closeGateOnce sync.Once
	t.Cleanup(func() { closeGateOnce.Do(func() { close(closeGate) }) })
	service.setCloseGate(1, closeGate, true)

	shutdownDeadline, stopShutdownDeadline := context.WithTimeoutCause(t.Context(), 25*time.Millisecond, errors.New("test manager shutdown timed out"))
	shutdownContext, cancelShutdown := context.WithCancelCause(shutdownDeadline)
	t.Cleanup(func() {
		cancelShutdown(errors.New("test manager shutdown completed"))
		stopShutdownDeadline()
	})
	finished := make(chan error, 1)
	go func() { finished <- manager.Shutdown(shutdownContext) }()

	var shutdownErr error
	require.Eventually(t, func() bool {
		select {
		case shutdownErr = <-finished:
			return true
		default:
			return false
		}
	}, 250*time.Millisecond, time.Millisecond, "Shutdown did not enforce its own bound around a context-ignoring Close")
	require.Error(t, shutdownErr)
	assert.Equal(t, tunnel.ErrorShutdownTimeout, manager.Snapshot().Status.ErrorCategory)
	_, active, maximum := service.counts()
	assert.Equal(t, 1, active)
	assert.Equal(t, 1, maximum)

	closeGateOnce.Do(func() { close(closeGate) })
	service.setCloseGate(1, nil, false)
	require.NoError(t, manager.Shutdown(t.Context()))
	_, active, maximum = service.counts()
	assert.Equal(t, 0, active)
	assert.Equal(t, 1, maximum)
}

func TestPublicAccessManagerShutdownDoneAndLateStartRacesLeaveZeroEndpointsAcross100Schedules(t *testing.T) {
	for schedule := range 100 {
		events := &orderedEvents{}
		startGate := make(chan struct{})
		service := newScheduledTunnelService(events, scheduledStart{gate: startGate, ignoreCancellation: true})
		manager, _ := newScheduledManager(t, service, events)
		started := make(chan tunnel.PublicAccessResult, 1)
		go func() { started <- manager.Start(t.Context(), 7) }()
		require.Eventually(t, func() bool {
			starts, _, _ := service.counts()
			return starts == 1
		}, time.Second, time.Millisecond)
		shutdown := make(chan error, 1)
		go func() { shutdown <- manager.Shutdown(t.Context()) }()
		require.Eventually(t, func() bool {
			return manager.Snapshot().Status.State == tunnel.LifecycleStopping
		}, time.Second, time.Millisecond)
		close(startGate)
		assert.Falsef(t, (<-started).OK, "schedule %d", schedule)
		assert.NoErrorf(t, <-shutdown, "schedule %d", schedule)
		require.Eventually(t, func() bool {
			_, active, _ := service.counts()
			return active == 0
		}, time.Second, time.Millisecond)
		assert.Equalf(t, tunnel.LifecycleDisabled, manager.Snapshot().Status.State, "schedule %d", schedule)
		assert.Emptyf(t, manager.Snapshot().Status.PublicURL, "schedule %d", schedule)
	}
}

func indexOfEvent(events []string, target string) int {
	for index, event := range events {
		if event == target {
			return index
		}
	}
	return -1
}
