package update

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const managerTestTimeout = time.Second

type managerCheckProbe struct {
	calls     atomic.Int64
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
	candidate *UpdateCandidate
}

func newManagerCheckProbe(candidate *UpdateCandidate, blocked bool) *managerCheckProbe {
	probe := &managerCheckProbe{
		started:   make(chan struct{}),
		candidate: candidate,
	}
	if blocked {
		probe.release = make(chan struct{})
	}
	return probe
}

func (probe *managerCheckProbe) Check(ctx context.Context) (*UpdateCandidate, error) {
	probe.calls.Add(1)
	probe.startOnce.Do(func() { close(probe.started) })
	if probe.release != nil {
		select {
		case <-probe.release:
		case <-ctx.Done():
			return nil, context.Cause(ctx)
		}
	}
	if probe.candidate == nil {
		return nil, nil
	}
	candidate := *probe.candidate
	return &candidate, nil
}

type managerPrepareProbe struct {
	calls     atomic.Int64
	active    atomic.Int64
	maximum   atomic.Int64
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
	relOnce   sync.Once
	prepared  PreparedApplicationUnit
	err       error
}

func newManagerPrepareProbe(t *testing.T) *managerPrepareProbe {
	t.Helper()
	probe := &managerPrepareProbe{
		started:  make(chan struct{}),
		release:  make(chan struct{}),
		prepared: preparedApplicationUnitFixture(),
	}
	t.Cleanup(probe.Release)
	return probe
}

func (probe *managerPrepareProbe) Release() {
	probe.relOnce.Do(func() { close(probe.release) })
}

func (probe *managerPrepareProbe) Prepare(
	ctx context.Context,
	_ UpdateCandidate,
	report func(UpdateState, UpdateProgress),
) (PreparedApplicationUnit, error) {
	probe.calls.Add(1)
	active := probe.active.Add(1)
	defer probe.active.Add(-1)
	for {
		maximum := probe.maximum.Load()
		if active <= maximum || probe.maximum.CompareAndSwap(maximum, active) {
			break
		}
	}
	probe.startOnce.Do(func() { close(probe.started) })
	select {
	case <-probe.release:
		return probe.prepared, probe.err
	case <-ctx.Done():
		return PreparedApplicationUnit{}, context.Cause(ctx)
	}
}

type immediateManagerPreparer struct {
	calls   atomic.Int64
	prepare func(func(UpdateState, UpdateProgress)) (PreparedApplicationUnit, error)
}

func (preparer *immediateManagerPreparer) Prepare(
	_ context.Context,
	_ UpdateCandidate,
	report func(UpdateState, UpdateProgress),
) (PreparedApplicationUnit, error) {
	preparer.calls.Add(1)
	return preparer.prepare(report)
}

type managerRestartProbe struct {
	calls     atomic.Int64
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
	relOnce   sync.Once
	unit      PreparedApplicationUnit
}

func newManagerRestartProbe(t *testing.T) *managerRestartProbe {
	t.Helper()
	probe := &managerRestartProbe{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	t.Cleanup(probe.Release)
	return probe
}

func (probe *managerRestartProbe) Release() {
	probe.relOnce.Do(func() { close(probe.release) })
}

func (probe *managerRestartProbe) Restart(ctx context.Context, unit PreparedApplicationUnit) error {
	probe.calls.Add(1)
	probe.unit = unit
	probe.startOnce.Do(func() { close(probe.started) })
	select {
	case <-probe.release:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

type managerPublications struct {
	mu        sync.Mutex
	snapshots []UpdateSnapshot
}

func (publications *managerPublications) Publish(snapshot UpdateSnapshot) {
	publications.mu.Lock()
	defer publications.mu.Unlock()
	publications.snapshots = append(publications.snapshots, snapshot)
}

func (publications *managerPublications) Snapshot() []UpdateSnapshot {
	publications.mu.Lock()
	defer publications.mu.Unlock()
	return append([]UpdateSnapshot(nil), publications.snapshots...)
}

func TestManagerDisablesProductionChecksOutsidePackagedReleases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		installedVersion string
		packaged         bool
	}{
		{name: "development version", installedVersion: "development", packaged: true},
		{name: "missing version", installedVersion: "", packaged: true},
		{name: "unpackaged release version", installedVersion: "2.4.0", packaged: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			checker := newManagerCheckProbe(nil, false)
			manager, err := NewManager(ManagerConfig{
				InstalledVersion: test.installedVersion,
				Packaged:         test.packaged,
				Check:            checker.Check,
				IDs:              func() string { return "disabled-check-must-not-have-an-attempt" },
			})
			require.NoError(t, err)

			initial := manager.Snapshot()
			assert.Equal(t, UpdateStateDisabled, initial.State)
			assert.Equal(t, test.installedVersion, initial.InstalledVersion)
			assert.Zero(t, initial.Revision)
			assert.Empty(t, initial.AttemptID)

			for range 8 {
				assert.Equal(t, initial, manager.Status(t.Context()))
			}
			assert.Never(t, func() bool { return checker.calls.Load() != 0 }, 25*time.Millisecond, time.Millisecond)
			assert.Equal(t, initial, manager.Snapshot())
		})
	}
}

func TestManagerStatusArmsOneAsynchronousCheckAcrossConcurrentCallers(t *testing.T) {
	t.Parallel()

	checker := newManagerCheckProbe(&UpdateCandidate{Version: "2.5.0", Channel: ChannelStable}, true)
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(checker.release) }) })
	publications := &managerPublications{}
	manager, err := NewManager(ManagerConfig{
		InstalledVersion: "2.4.0",
		Packaged:         true,
		Check:            checker.Check,
		Publish:          publications.Publish,
		IDs:              func() string { return "attempt-one" },
	})
	require.NoError(t, err)
	require.Equal(t, UpdateSnapshot{
		State: UpdateStateIdle, InstalledVersion: "2.4.0",
	}, manager.Snapshot())

	const callers = 32
	start := make(chan struct{})
	results := make(chan UpdateSnapshot, callers)
	for range callers {
		go func() {
			<-start
			results <- manager.Status(t.Context())
		}()
	}
	close(start)
	for range callers {
		snapshot := requireManagerReceive(t, results, "Status blocked on the asynchronous release check")
		assert.Contains(t, []UpdateState{UpdateStateIdle, UpdateStateChecking}, snapshot.State)
	}

	requireManagerReceive(t, checker.started, "release check was not armed")
	assert.Equal(t, int64(1), checker.calls.Load())
	releaseOnce.Do(func() { close(checker.release) })
	require.Eventually(t, func() bool {
		return manager.Snapshot().State == UpdateStateAvailable
	}, managerTestTimeout, time.Millisecond)
	assert.Equal(t, int64(1), checker.calls.Load())
	for range 8 {
		manager.Status(t.Context())
	}
	assert.Equal(t, int64(1), checker.calls.Load())

	assert.Equal(t, []UpdateSnapshot{
		{
			Revision: 1, AttemptID: "attempt-one", State: UpdateStateChecking,
			InstalledVersion: "2.4.0",
		},
		{
			Revision: 2, AttemptID: "attempt-one", State: UpdateStateAvailable,
			InstalledVersion: "2.4.0", AvailableVersion: "2.5.0",
		},
	}, publications.Snapshot())
}

func TestManagerSnapshotsAreDetachedAndRevisionsAdvanceOnlyForVisibleChanges(t *testing.T) {
	t.Parallel()

	publications := &managerPublications{}
	manager := newAvailableManager(t, publications, nil)
	available := manager.Snapshot()
	require.Equal(t, UpdateStateAvailable, available.State)
	require.Equal(t, uint64(2), available.Revision)

	mutated := available
	mutated.AttemptID = "mutated"
	mutated.AvailableVersion = "999.0.0"
	mutated.ReleaseNotes = "injected"
	assert.Equal(t, available, manager.Snapshot(), "caller mutation changed the manager-owned snapshot")

	stale := manager.ResolveOffer(t.Context(), "stale-attempt", OfferDecisionDefer)
	assert.False(t, stale.OK)
	assert.NotEmpty(t, stale.Error)
	assert.Equal(t, available, stale.Snapshot)
	assert.Equal(t, available, manager.Snapshot())
	assert.Len(t, publications.Snapshot(), 2, "a rejected command published a new revision")

	deferred := manager.ResolveOffer(t.Context(), available.AttemptID, OfferDecisionDefer)
	require.True(t, deferred.OK, "ResolveOffer() = %#v", deferred)
	assert.Empty(t, deferred.Error)
	assert.Equal(t, UpdateStateDeferred, deferred.Snapshot.State)
	assert.Equal(t, available.Revision+1, deferred.Snapshot.Revision)
	assert.Equal(t, deferred.Snapshot, manager.Snapshot())

	rejected := manager.ResolveOffer(t.Context(), available.AttemptID, OfferDecisionDefer)
	assert.False(t, rejected.OK)
	assert.Equal(t, deferred.Snapshot, rejected.Snapshot)
	assert.Equal(t, deferred.Snapshot, manager.Snapshot())
	assert.Equal(t, []uint64{1, 2, 3}, publicationRevisions(publications.Snapshot()))
}

func TestManagerRejectsStaleAttemptBeforeStartingPreparation(t *testing.T) {
	t.Parallel()

	preparer := newManagerPrepareProbe(t)
	manager := newAvailableManager(t, nil, preparer.Prepare)
	before := manager.Snapshot()

	result := manager.ResolveOffer(t.Context(), "previous-attempt", OfferDecisionAccept)
	assert.False(t, result.OK)
	assert.NotEmpty(t, result.Error)
	assert.Equal(t, before, result.Snapshot)
	assert.Equal(t, int64(0), preparer.calls.Load())
	assert.Equal(t, before, manager.Snapshot())
}

func TestManagerSerializesConcurrentAcceptOperations(t *testing.T) {
	t.Parallel()

	preparer := newManagerPrepareProbe(t)
	manager := newAvailableManager(t, nil, preparer.Prepare)
	available := manager.Snapshot()
	firstDone := make(chan CommandResult, 1)
	go func() {
		firstDone <- manager.ResolveOffer(t.Context(), available.AttemptID, OfferDecisionAccept)
	}()

	requireManagerReceive(t, preparer.started, "first preparation did not start")
	second := manager.ResolveOffer(t.Context(), available.AttemptID, OfferDecisionAccept)
	assert.False(t, second.OK)
	assert.NotEmpty(t, second.Error)
	assert.Equal(t, UpdateStateDownloading, second.Snapshot.State)
	assert.Equal(t, available.Revision+1, second.Snapshot.Revision)
	assert.Equal(t, int64(1), preparer.calls.Load())

	preparer.Release()
	first := requireManagerReceive(t, firstDone, "first preparation did not finish")
	assert.True(t, first.OK, "first ResolveOffer() = %#v", first)
	assert.Equal(t, UpdateStateReadyToRestart, first.Snapshot.State)
	assert.Equal(t, int64(1), preparer.calls.Load())
	assert.Equal(t, int64(1), preparer.maximum.Load())
	assert.Equal(t, available.Revision+2, manager.Snapshot().Revision)
}

func TestManagerAcceptIsTheOnlyPreparationGateAndPublishesMonotonicProgress(t *testing.T) {
	t.Parallel()

	publications := &managerPublications{}
	preparer := &immediateManagerPreparer{
		prepare: func(report func(UpdateState, UpdateProgress)) (PreparedApplicationUnit, error) {
			report(UpdateStateDownloading, UpdateProgress{
				BytesDownloaded: 512, DownloadSize: 4096, DownloadSizeKnown: true,
			})
			// A provider callback must not be able to regress visible progress.
			report(UpdateStateDownloading, UpdateProgress{
				BytesDownloaded: 256, DownloadSize: 4096, DownloadSizeKnown: true,
			})
			report(UpdateStateDownloading, UpdateProgress{
				BytesDownloaded: 4096, DownloadSize: 4096, DownloadSizeKnown: true,
			})
			report(UpdateStateVerifying, UpdateProgress{
				BytesDownloaded: 4096, DownloadSize: 4096, DownloadSizeKnown: true,
			})
			report(UpdateStateStaging, UpdateProgress{
				BytesDownloaded: 4096, DownloadSize: 4096, DownloadSizeKnown: true,
			})
			return preparedApplicationUnitFixture(), nil
		},
	}
	manager := newAvailableManager(t, publications, preparer.Prepare)
	available := manager.Snapshot()

	assert.Equal(t, int64(0), preparer.calls.Load(), "discovery downloaded update bytes before consent")
	result := manager.ResolveOffer(t.Context(), available.AttemptID, OfferDecisionAccept)
	require.True(t, result.OK, "ResolveOffer() = %#v", result)
	assert.Equal(t, int64(1), preparer.calls.Load())
	assert.Equal(t, UpdateStateReadyToRestart, result.Snapshot.State)
	assert.Equal(t, UpdateProgress{
		BytesDownloaded: 4096, DownloadSize: 4096, DownloadSizeKnown: true,
	}, result.Snapshot.Progress)

	preparation := publications.Snapshot()[2:]
	assert.Equal(t, []UpdateState{
		UpdateStateDownloading,
		UpdateStateDownloading,
		UpdateStateDownloading,
		UpdateStateVerifying,
		UpdateStateStaging,
		UpdateStateReadyToRestart,
	}, publicationStates(preparation))
	assert.Equal(t, []uint64{0, 512, 4096, 4096, 4096, 4096}, publicationDownloadedBytes(preparation))
	assert.Equal(t, []uint64{3, 4, 5, 6, 7, 8}, publicationRevisions(preparation))
	assert.Equal(t, result.Snapshot, manager.Snapshot())
}

func TestManagerNeverPublishesReadyWhenVerificationOrManifestValidationFails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		lastState UpdateState
		wantStage FailureStage
	}{
		{name: "publisher digest mismatch", lastState: UpdateStateVerifying, wantStage: FailureStageVerify},
		{name: "artifact manifest mismatch", lastState: UpdateStateStaging, wantStage: FailureStageStage},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			publications := &managerPublications{}
			preparer := &immediateManagerPreparer{
				prepare: func(report func(UpdateState, UpdateProgress)) (PreparedApplicationUnit, error) {
					report(UpdateStateDownloading, UpdateProgress{
						BytesDownloaded: 4096, DownloadSize: 4096, DownloadSizeKnown: true,
					})
					report(test.lastState, UpdateProgress{
						BytesDownloaded: 4096, DownloadSize: 4096, DownloadSizeKnown: true,
					})
					return PreparedApplicationUnit{}, errors.New("unsafe provider detail: /Users/private/update.zip")
				},
			}
			manager := newAvailableManager(t, publications, preparer.Prepare)
			available := manager.Snapshot()

			result := manager.ResolveOffer(t.Context(), available.AttemptID, OfferDecisionAccept)
			assert.False(t, result.OK)
			assert.Equal(t, UpdateStateFailed, result.Snapshot.State)
			assert.Equal(t, test.wantStage, result.Snapshot.Failure.Stage)
			assert.NotEmpty(t, result.Snapshot.Failure.Message)
			assert.NotEmpty(t, result.Snapshot.Failure.RecoveryAction)
			assert.NotContains(t, result.Error, "/Users/private")
			assert.NotContains(t, result.Snapshot.Failure.Message, "/Users/private")
			assert.NotContains(t, publicationStates(publications.Snapshot()), UpdateStateReadyToRestart)

			restart := manager.ResolveRestart(t.Context(), available.AttemptID, RestartDecisionRestart)
			assert.False(t, restart.OK)
			assert.Equal(t, UpdateStateFailed, restart.Snapshot.State)
		})
	}
}

func TestManagerBoundsAStalledCheckAndKeepsTheInstalledApplicationUsable(t *testing.T) {
	t.Parallel()

	const installedVersion = "2.4.0"
	checker := func(ctx context.Context) (*UpdateCandidate, error) {
		<-ctx.Done()
		return nil, context.Cause(ctx)
	}
	manager, err := NewManager(ManagerConfig{
		InstalledVersion: installedVersion,
		Packaged:         true,
		CheckTimeout:     5 * time.Millisecond,
		Check:            checker,
		IDs:              func() string { return "attempt-timeout" },
	})
	require.NoError(t, err)

	manager.Status(t.Context())
	require.Eventually(t, func() bool {
		return manager.Snapshot().State == UpdateStateFailed
	}, managerTestTimeout, time.Millisecond)

	snapshot := manager.Snapshot()
	assert.Equal(t, installedVersion, snapshot.InstalledVersion)
	assert.Equal(t, FailureStageCheck, snapshot.Failure.Stage)
	assert.NotEmpty(t, snapshot.Failure.Message)
	assert.NotEmpty(t, snapshot.Failure.RecoveryAction)
	assert.NotContains(t, snapshot.Failure.Message, context.DeadlineExceeded.Error())
}

func TestManagerPreparationFailuresExposeStableActionsWithoutBackendDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		state           UpdateState
		failure         error
		wantStage       FailureStage
		wantActionToken string
	}{
		{
			name:            "caller cancellation",
			state:           UpdateStateDownloading,
			failure:         context.Canceled,
			wantStage:       FailureStageDownload,
			wantActionToken: "try",
		},
		{
			name:            "insufficient staging space",
			state:           UpdateStateStaging,
			failure:         errors.New("write /Users/private/stage: no space left on device; token=secret"),
			wantStage:       FailureStageStage,
			wantActionToken: "space",
		},
		{
			name:            "read only installation",
			state:           UpdateStateStaging,
			failure:         errors.New("rename /Applications/Fallout Terminal: read-only file system"),
			wantStage:       FailureStageStage,
			wantActionToken: "writable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			preparer := &immediateManagerPreparer{
				prepare: func(report func(UpdateState, UpdateProgress)) (PreparedApplicationUnit, error) {
					report(test.state, UpdateProgress{})
					return PreparedApplicationUnit{}, test.failure
				},
			}
			manager := newAvailableManager(t, nil, preparer.Prepare)
			available := manager.Snapshot()

			result := manager.ResolveOffer(t.Context(), available.AttemptID, OfferDecisionAccept)
			assert.False(t, result.OK)
			assert.Equal(t, UpdateStateFailed, result.Snapshot.State)
			assert.Equal(t, test.wantStage, result.Snapshot.Failure.Stage)
			assert.Contains(t, strings.ToLower(result.Snapshot.Failure.RecoveryAction), test.wantActionToken)
			assert.Equal(t, "2.4.0", result.Snapshot.InstalledVersion)
			for _, privateDetail := range []string{"/Users/private", "/Applications", "token=secret"} {
				assert.NotContains(t, result.Error, privateDetail)
				assert.NotContains(t, result.Snapshot.Failure.Message, privateDetail)
				assert.NotContains(t, result.Snapshot.Failure.RecoveryAction, privateDetail)
			}
		})
	}
}

func TestManagerPreparationHonorsCallerCancellation(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	prepare := func(
		ctx context.Context,
		_ UpdateCandidate,
		_ func(UpdateState, UpdateProgress),
	) (PreparedApplicationUnit, error) {
		close(started)
		<-ctx.Done()
		return PreparedApplicationUnit{}, context.Cause(ctx)
	}
	manager := newAvailableManager(t, nil, prepare)
	available := manager.Snapshot()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	cancel()

	result := manager.ResolveOffer(ctx, available.AttemptID, OfferDecisionAccept)
	requireManagerReceive(t, started, "preparation was not called with the canceled command")
	assert.False(t, result.OK)
	assert.Equal(t, UpdateStateFailed, result.Snapshot.State)
	assert.Equal(t, FailureStageDownload, result.Snapshot.Failure.Stage)
	assert.Equal(t, "2.4.0", result.Snapshot.InstalledVersion)
	assert.NotEmpty(t, result.Snapshot.Failure.RecoveryAction)
}

func TestManagerPostponeRetainsPreparedUnitAndRestartHandoffRunsExactlyOnce(t *testing.T) {
	t.Parallel()

	prepared := preparedApplicationUnitFixture()
	preparer := &immediateManagerPreparer{
		prepare: func(report func(UpdateState, UpdateProgress)) (PreparedApplicationUnit, error) {
			report(UpdateStateVerifying, UpdateProgress{})
			report(UpdateStateStaging, UpdateProgress{})
			return prepared, nil
		},
	}
	restarter := newManagerRestartProbe(t)
	manager := newAvailableManagerWithRestart(t, nil, preparer.Prepare, restarter.Restart)
	available := manager.Snapshot()
	accepted := manager.ResolveOffer(t.Context(), available.AttemptID, OfferDecisionAccept)
	require.True(t, accepted.OK, "ResolveOffer() = %#v", accepted)
	require.Equal(t, UpdateStateReadyToRestart, accepted.Snapshot.State)
	ready := accepted.Snapshot

	postponed := manager.ResolveRestart(t.Context(), ready.AttemptID, RestartDecisionPostpone)
	require.True(t, postponed.OK, "ResolveRestart(postpone) = %#v", postponed)
	assert.Equal(t, ready, postponed.Snapshot)
	assert.Equal(t, ready, manager.Snapshot())
	assert.Equal(t, int64(0), restarter.calls.Load())

	restartDone := make(chan CommandResult, 1)
	go func() {
		restartDone <- manager.ResolveRestart(t.Context(), ready.AttemptID, RestartDecisionRestart)
	}()
	requireManagerReceive(t, restarter.started, "restart handoff did not start")

	duplicate := manager.ResolveRestart(t.Context(), ready.AttemptID, RestartDecisionRestart)
	assert.False(t, duplicate.OK)
	assert.Equal(t, UpdateStateApplying, duplicate.Snapshot.State)
	assert.Equal(t, int64(1), restarter.calls.Load())

	restarter.Release()
	result := requireManagerReceive(t, restartDone, "restart handoff did not finish")
	require.True(t, result.OK, "ResolveRestart(restart) = %#v", result)
	assert.Equal(t, UpdateStateApplying, result.Snapshot.State)
	assert.Equal(t, int64(1), restarter.calls.Load())
	assert.Equal(t, prepared, restarter.unit)
}

func requireManagerReceive[T any](t *testing.T, values <-chan T, message string) T {
	t.Helper()

	var value T
	require.Eventually(t, func() bool {
		select {
		case value = <-values:
			return true
		default:
			return false
		}
	}, managerTestTimeout, time.Millisecond, message)
	return value
}

func newAvailableManager(
	t *testing.T,
	publications *managerPublications,
	prepare func(
		context.Context,
		UpdateCandidate,
		func(UpdateState, UpdateProgress),
	) (PreparedApplicationUnit, error),
) *Manager {
	t.Helper()
	return newAvailableManagerWithRestart(t, publications, prepare, nil)
}

func newAvailableManagerWithRestart(
	t *testing.T,
	publications *managerPublications,
	prepare func(
		context.Context,
		UpdateCandidate,
		func(UpdateState, UpdateProgress),
	) (PreparedApplicationUnit, error),
	restart func(context.Context, PreparedApplicationUnit) error,
) *Manager {
	t.Helper()
	candidate := &UpdateCandidate{
		Version: "2.5.0", Channel: ChannelStable, ReleaseNotes: "Maintenance release.",
	}
	checker := newManagerCheckProbe(candidate, false)
	config := ManagerConfig{
		InstalledVersion: "2.4.0",
		Packaged:         true,
		Check:            checker.Check,
		Prepare:          prepare,
		Restart:          restart,
		IDs:              func() string { return "attempt-current" },
	}
	if publications != nil {
		config.Publish = publications.Publish
	}
	manager, err := NewManager(config)
	require.NoError(t, err)
	manager.Status(t.Context())
	require.Eventually(t, func() bool {
		return manager.Snapshot().State == UpdateStateAvailable
	}, managerTestTimeout, time.Millisecond)
	require.Equal(t, int64(1), checker.calls.Load())
	return manager
}

func preparedApplicationUnitFixture() PreparedApplicationUnit {
	return PreparedApplicationUnit{
		AttemptID:          "attempt-current",
		Version:            "2.5.0",
		Target:             Target{OS: "linux", Arch: "amd64"},
		InstalledUnit:      "/opt/fallout-terminal/Fallout Terminal",
		StagedUnit:         "/opt/fallout-terminal/.fallout-terminal-update-attempt-current",
		LaunchRelativePath: "Fallout Terminal",
	}
}

func publicationRevisions(snapshots []UpdateSnapshot) []uint64 {
	revisions := make([]uint64, 0, len(snapshots))
	for _, snapshot := range snapshots {
		revisions = append(revisions, snapshot.Revision)
	}
	return revisions
}

func publicationStates(snapshots []UpdateSnapshot) []UpdateState {
	states := make([]UpdateState, 0, len(snapshots))
	for _, snapshot := range snapshots {
		states = append(states, snapshot.State)
	}
	return states
}

func publicationDownloadedBytes(snapshots []UpdateSnapshot) []uint64 {
	bytes := make([]uint64, 0, len(snapshots))
	for _, snapshot := range snapshots {
		bytes = append(bytes, snapshot.Progress.BytesDownloaded)
	}
	return bytes
}
