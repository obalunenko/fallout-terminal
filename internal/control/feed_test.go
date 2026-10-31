package control

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/stretchr/testify/require"
)

// feedTestTime is the commit time the feed tests stamp, in Unix milliseconds.
const feedTestTime = int64(1793390820000)

func TestAppendFacilityChangesKeepsTheFiveNewestEntriesOfEachDevice(t *testing.T) {
	t.Parallel()

	runtime := &domain.ProcessRuntime{Facility: feedFacility()}
	at := time.UnixMilli(feedTestTime)
	for range 3 {
		appendFacilityChanges(runtime, at, feedForcedEntry("door"))
	}
	appendFacilityChanges(runtime, at, feedForcedEntry("power"))
	for range 4 {
		appendFacilityChanges(runtime, at, feedForcedEntry("door"))
	}

	require.Equal(t, uint64(8), runtime.FacilityChangeSequence)
	require.Equal(t, []uint64{8, 7, 6, 5, 3}, feedSequences(runtime.FacilityChanges["door"]),
		"a device keeps its five newest entries, newest first")
	require.Equal(t, []uint64{4}, feedSequences(runtime.FacilityChanges["power"]),
		"one device's bound must not drop another device's entries")
}

func TestAppendFacilityChangesNumbersOneCommitInOrderWithOneTime(t *testing.T) {
	t.Parallel()

	runtime := &domain.ProcessRuntime{Facility: feedFacility(), FacilityChangeSequence: 41}
	forced := &domain.FacilityChangeForced{FromStateID: "online", ToStateID: "offline"}
	transition := &domain.FacilityChangeTransition{TransitionID: "open", FromStateID: "sealed", ToStateID: "open"}
	withdrawn := &domain.FacilityChangePlayerRefusal{
		TerminalID: "terminal-1", CommandID: "command-open", Reason: domain.FacilityChangeRefusalWithdrawn,
	}

	recorded := appendFacilityChanges(runtime, time.UnixMilli(feedTestTime),
		domain.FacilityChange{Sequence: 999, OccurredAtUnixMs: 1, DeviceID: "power", Forced: forced},
		domain.FacilityChange{DeviceID: "door", Transition: transition},
		domain.FacilityChange{DeviceID: "door", PlayerRefusal: withdrawn},
	)

	require.True(t, recorded)
	require.Equal(t, uint64(44), runtime.FacilityChangeSequence)
	want := map[string][]domain.FacilityChange{
		"door": {
			{Sequence: 44, OccurredAtUnixMs: feedTestTime, DeviceID: "door", PlayerRefusal: withdrawn},
			{Sequence: 43, OccurredAtUnixMs: feedTestTime, DeviceID: "door", Transition: transition},
		},
		"power": {
			{Sequence: 42, OccurredAtUnixMs: feedTestTime, DeviceID: "power", Forced: forced},
		},
	}
	require.Empty(t, cmp.Diff(want, runtime.FacilityChanges),
		"entries are numbered in argument order and share the commit's time")
}

func TestAppendFacilityChangesNeverReusesASequenceInARun(t *testing.T) {
	t.Parallel()

	runtime := &domain.ProcessRuntime{Facility: feedFacility()}
	var sequences []uint64
	record := func(deviceID string) {
		appendFacilityChanges(runtime, time.UnixMilli(feedTestTime), feedForcedEntry(deviceID))
		sequences = append(sequences, runtime.FacilityChanges[deviceID][0].Sequence)
	}
	for range 6 {
		record("door")
	}
	// Opening another session clears the feed and keeps the sequence.
	runtime.FacilityChanges = nil
	record("power")
	record("door")

	require.Equal(t, []uint64{1, 2, 3, 4, 5, 6, 7, 8}, sequences)
	require.Equal(t, []uint64{8}, feedSequences(runtime.FacilityChanges["door"]))
	require.Equal(t, []uint64{7}, feedSequences(runtime.FacilityChanges["power"]))
}

func TestAppendFacilityChangesRecordsOnlyDevicesOfTheFacility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		facility     *domain.Facility
		changes      []domain.FacilityChange
		wantRecorded bool
		wantSequence uint64
		want         map[string][]domain.FacilityChange
	}{
		{
			name:         "an unknown device is skipped without a sequence",
			facility:     feedFacility(),
			changes:      []domain.FacilityChange{feedForcedEntry("lamp"), feedForcedEntry("door")},
			wantRecorded: true,
			wantSequence: 1,
			want: map[string][]domain.FacilityChange{
				"door": {feedStamped(feedForcedEntry("door"), 1)},
			},
		},
		{
			name:     "only unknown devices",
			facility: feedFacility(),
			changes:  []domain.FacilityChange{feedForcedEntry("lamp")},
		},
		{
			name:    "no facility",
			changes: []domain.FacilityChange{feedForcedEntry("door")},
		},
		{
			name:     "no entries",
			facility: feedFacility(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			runtime := &domain.ProcessRuntime{Facility: test.facility}

			recorded := appendFacilityChanges(runtime, time.UnixMilli(feedTestTime), test.changes...)

			require.Equal(t, test.wantRecorded, recorded)
			require.Equal(t, test.wantSequence, runtime.FacilityChangeSequence)
			require.Empty(t, cmp.Diff(test.want, runtime.FacilityChanges, cmpopts.EquateEmpty()))
		})
	}
}

func TestFacilityChangeSnapshotListsEveryDeviceNewestFirst(t *testing.T) {
	t.Parallel()

	feed := map[string][]domain.FacilityChange{
		"door": {
			feedStamped(feedRefusalEntry("door", domain.FacilityChangeRefusalDeclined), 9),
			feedStamped(feedForcedEntry("door"), 4),
			feedStamped(feedForcedEntry("door"), 2),
		},
		"power": {
			feedStamped(feedForcedEntry("power"), 7),
			feedStamped(feedForcedEntry("power"), 3),
		},
		"vent": {
			feedStamped(domain.FacilityChange{DeviceID: "vent", SceneReset: &domain.FacilityChangeReset{
				FromStateID: "clogged", ToStateID: "clean", ConditionIDs: []string{"vent-dust"},
			}}, 8),
		},
	}
	original := cloneFeedForTest(feed)

	snapshot := facilityChangeSnapshot(feed)

	require.Equal(t, []uint64{9, 8, 7, 4, 3, 2}, feedSequences(snapshot))
	require.Equal(t, []string{"door", "vent", "power", "door", "power", "door"}, feedDeviceIDs(snapshot))
	snapshot[0].PlayerRefusal.Reason = domain.FacilityChangeRefusalStale
	snapshot[1].SceneReset.ConditionIDs[0] = "tampered"
	require.Empty(t, cmp.Diff(original, feed), "the snapshot must be detached from the runtime feed")

	require.Nil(t, facilityChangeSnapshot(nil))
	require.Nil(t, facilityChangeSnapshot(map[string][]domain.FacilityChange{}))
	require.Nil(t, facilityChangeSnapshot(map[string][]domain.FacilityChange{"door": nil}))
}

func TestPruneFacilityChangesDropsDevicesMissingFromTheFacility(t *testing.T) {
	t.Parallel()

	feed := map[string][]domain.FacilityChange{
		"door":  {feedStamped(feedForcedEntry("door"), 3)},
		"power": {feedStamped(feedForcedEntry("power"), 2)},
		"lamp":  {feedStamped(feedForcedEntry("lamp"), 1)},
	}
	withoutPower := feedFacility()
	withoutPower.Devices = withoutPower.Devices[1:]
	tests := []struct {
		name     string
		facility *domain.Facility
		want     map[string][]domain.FacilityChange
	}{
		{
			name:     "devices of the facility keep their feed",
			facility: feedFacility(),
			want:     map[string][]domain.FacilityChange{"door": feed["door"], "power": feed["power"]},
		},
		{
			name:     "a deleted device loses its feed",
			facility: withoutPower,
			want:     map[string][]domain.FacilityChange{"door": feed["door"]},
		},
		{
			name: "no facility drops every feed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			runtime := &domain.ProcessRuntime{
				Facility: test.facility, FacilityChanges: cloneFeedForTest(feed), FacilityChangeSequence: 9,
			}

			pruneFacilityChanges(runtime)

			require.Empty(t, cmp.Diff(test.want, runtime.FacilityChanges, cmpopts.EquateEmpty()))
			require.Equal(t, uint64(9), runtime.FacilityChangeSequence, "pruning must keep the run's sequence")
		})
	}
}

func TestCloneFacilityChangeFeedDetachesEveryDevice(t *testing.T) {
	t.Parallel()

	original := map[string][]domain.FacilityChange{
		"door": {
			feedStamped(domain.FacilityChange{DeviceID: "door", Reset: &domain.FacilityChangeReset{
				FromStateID: "open", ToStateID: "sealed", ConditionIDs: []string{"door-alarm"},
			}}, 2),
			feedStamped(feedRefusalEntry("door", domain.FacilityChangeRefusalDeclined), 1),
		},
		"power": {feedStamped(feedForcedEntry("power"), 3)},
	}
	want := cloneFeedForTest(original)

	clone := cloneFacilityChangeFeed(original)

	require.Empty(t, cmp.Diff(want, clone))
	clone["door"][0].Reset.ConditionIDs[0] = "tampered"
	clone["door"][1].PlayerRefusal.Reason = domain.FacilityChangeRefusalStale
	clone["door"][1].Sequence = 99
	delete(clone, "power")
	require.Empty(t, cmp.Diff(want, original), "a commit's working copy must not reach the installed feed")
	require.Nil(t, cloneFacilityChangeFeed(nil))
}

func TestFacilityChangeRefusalReasonMapsEveryFailureCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		failure    domain.FacilityFailureCode
		wantReason domain.FacilityChangeRefusalReason
		wantEntry  bool
	}{
		{domain.FacilityFailureRejected, domain.FacilityChangeRefusalDeclined, true},
		{domain.FacilityFailurePreconditionFailed, domain.FacilityChangeRefusalPreconditionFailed, true},
		{domain.FacilityFailureInvalidTransition, domain.FacilityChangeRefusalTransitionUnavailable, true},
		{domain.FacilityFailureStaleRevision, domain.FacilityChangeRefusalStale, true},
		{domain.FacilityFailureMissingReference, domain.FacilityChangeRefusalFailed, true},
		{domain.FacilityFailureConflict, domain.FacilityChangeRefusalFailed, true},
		{domain.FacilityFailureInvalidConfiguration, domain.FacilityChangeRefusalFailed, true},
		{domain.FacilityFailurePersistenceFailed, domain.FacilityChangeRefusalFailed, true},
		{domain.FacilityFailureRuntimeContextEnded, domain.FacilityChangeRefusalFailed, true},
		{"unknown-failure", domain.FacilityChangeRefusalFailed, true},
		// A duplicate decision answers a request that is already resolved,
		// and an unspecified code refuses nothing.
		{domain.FacilityFailureDuplicate, "", false},
		{domain.FacilityFailureUnspecified, "", false},
		{"", "", false},
	}
	for _, test := range tests {
		name := string(test.failure)
		if name == "" {
			name = "empty code"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			reason, ok := facilityChangeRefusalReason(test.failure)

			require.Equal(t, test.wantEntry, ok)
			if test.wantEntry {
				require.Equal(t, test.wantReason, reason)
			}
		})
	}
}

func TestFacilityChangeBuildersRecordWhatChanged(t *testing.T) {
	t.Parallel()

	initial := feedFacility()
	tests := []struct {
		name  string
		build func(*testing.T) []domain.FacilityChange
		want  []domain.FacilityChange
	}{
		{
			name: "card transition",
			build: func(t *testing.T) []domain.FacilityChange {
				resulting := feedFacilityWith(func(facility *domain.Facility) {
					feedDevice(t, facility, "door").CurrentStateID = "open"
					feedCondition(t, facility, "door-alarm").CurrentActive = true
				})
				return feedOne(facilityTransitionChange(initial, resulting, "door", "open"))
			},
			want: []domain.FacilityChange{{DeviceID: "door", Transition: &domain.FacilityChangeTransition{
				TransitionID: "open", FromStateID: "sealed", ToStateID: "open",
			}}},
		},
		{
			name: "card transition that left the device in place",
			build: func(t *testing.T) []domain.FacilityChange {
				return feedOne(facilityTransitionChange(initial, feedFacility(), "door", "open"))
			},
		},
		{
			name: "forced change",
			build: func(t *testing.T) []domain.FacilityChange {
				resulting := feedFacilityWith(func(facility *domain.Facility) {
					feedDevice(t, facility, "power").CurrentStateID = "offline"
				})
				return feedOne(facilityForcedChange(initial, resulting, "power"))
			},
			want: []domain.FacilityChange{{DeviceID: "power", Forced: &domain.FacilityChangeForced{
				FromStateID: "online", ToStateID: "offline",
			}}},
		},
		{
			name: "forced change of a missing device",
			build: func(t *testing.T) []domain.FacilityChange {
				return feedOne(facilityForcedChange(initial, feedFacility(), "lamp"))
			},
		},
		{
			name: "fault switched on goes to its device",
			build: func(t *testing.T) []domain.FacilityChange {
				resulting := feedFacilityWith(func(facility *domain.Facility) {
					feedCondition(t, facility, "door-jam").CurrentActive = true
				})
				return feedOne(facilityFaultChange(resulting, "door-jam"))
			},
			want: []domain.FacilityChange{{DeviceID: "door", Fault: &domain.FacilityChangeFault{
				ConditionID: "door-jam", Active: true,
			}}},
		},
		{
			name: "fault switched off records the resulting position",
			build: func(t *testing.T) []domain.FacilityChange {
				return feedOne(facilityFaultChange(feedFacility(), "door-jam"))
			},
			want: []domain.FacilityChange{{DeviceID: "door", Fault: &domain.FacilityChangeFault{
				ConditionID: "door-jam", Active: false,
			}}},
		},
		{
			name: "terminal fault has no device feed",
			build: func(t *testing.T) []domain.FacilityChange {
				return feedOne(facilityFaultChange(feedFacility(), "screen-noise"))
			},
		},
		{
			name: "missing fault",
			build: func(t *testing.T) []domain.FacilityChange {
				return feedOne(facilityFaultChange(feedFacility(), "lamp-flicker"))
			},
		},
		{
			name: "device reset of the state and its faults",
			build: func(t *testing.T) []domain.FacilityChange {
				previous := feedFacilityWith(func(facility *domain.Facility) {
					feedDevice(t, facility, "door").CurrentStateID = "open"
					feedCondition(t, facility, "door-alarm").CurrentActive = true
					feedCondition(t, facility, "door-jam").CurrentActive = true
					feedCondition(t, facility, "screen-noise").CurrentActive = true
				})
				resulting := feedFacilityWith(func(facility *domain.Facility) {
					feedCondition(t, facility, "screen-noise").CurrentActive = true
				})
				return feedOne(facilityResetChange(previous, resulting, "door"))
			},
			want: []domain.FacilityChange{{DeviceID: "door", Reset: &domain.FacilityChangeReset{
				FromStateID: "open", ToStateID: "sealed", ConditionIDs: []string{"door-alarm", "door-jam"},
			}}},
		},
		{
			name: "device reset of faults only",
			build: func(t *testing.T) []domain.FacilityChange {
				previous := feedFacilityWith(func(facility *domain.Facility) {
					feedCondition(t, facility, "door-jam").CurrentActive = true
				})
				return feedOne(facilityResetChange(previous, feedFacility(), "door"))
			},
			want: []domain.FacilityChange{{DeviceID: "door", Reset: &domain.FacilityChangeReset{
				FromStateID: "sealed", ToStateID: "sealed", ConditionIDs: []string{"door-jam"},
			}}},
		},
		{
			name: "device reset that changed nothing",
			build: func(t *testing.T) []domain.FacilityChange {
				return feedOne(facilityResetChange(initial, feedFacility(), "door"))
			},
		},
		{
			name: "scene reset records each changed device in ID order",
			build: func(t *testing.T) []domain.FacilityChange {
				previous := feedFacilityWith(func(facility *domain.Facility) {
					feedDevice(t, facility, "power").CurrentStateID = "offline"
					feedDevice(t, facility, "door").CurrentStateID = "open"
					feedCondition(t, facility, "door-alarm").CurrentActive = true
					feedCondition(t, facility, "screen-noise").CurrentActive = true
				})
				return facilitySceneResetChanges(previous, feedFacility())
			},
			want: []domain.FacilityChange{
				{DeviceID: "door", SceneReset: &domain.FacilityChangeReset{
					FromStateID: "open", ToStateID: "sealed", ConditionIDs: []string{"door-alarm"},
				}},
				{DeviceID: "power", SceneReset: &domain.FacilityChangeReset{
					FromStateID: "offline", ToStateID: "online",
				}},
			},
		},
		{
			name: "scene reset of a terminal fault only",
			build: func(t *testing.T) []domain.FacilityChange {
				previous := feedFacilityWith(func(facility *domain.Facility) {
					feedCondition(t, facility, "screen-noise").CurrentActive = true
				})
				return facilitySceneResetChanges(previous, feedFacility())
			},
		},
		{
			name: "player command records each moved device in ID order",
			build: func(t *testing.T) []domain.FacilityChange {
				resulting := feedFacilityWith(func(facility *domain.Facility) {
					feedDevice(t, facility, "power").CurrentStateID = "offline"
					feedDevice(t, facility, "door").CurrentStateID = "open"
				})
				return facilityPlayerCommandChanges(initial, resulting, "terminal-1", "command-open",
					[]domain.FacilityTransitionRequest{
						{DeviceID: "power", TransitionID: "shut-down"},
						{DeviceID: "door", TransitionID: "open"},
					})
			},
			want: []domain.FacilityChange{
				{DeviceID: "door", PlayerCommand: &domain.FacilityChangePlayerCommand{
					TerminalID: "terminal-1", CommandID: "command-open", TransitionID: "open",
					FromStateID: "sealed", ToStateID: "open",
				}},
				{DeviceID: "power", PlayerCommand: &domain.FacilityChangePlayerCommand{
					TerminalID: "terminal-1", CommandID: "command-open", TransitionID: "shut-down",
					FromStateID: "online", ToStateID: "offline",
				}},
			},
		},
		{
			name: "player command skips a device that did not move",
			build: func(t *testing.T) []domain.FacilityChange {
				resulting := feedFacilityWith(func(facility *domain.Facility) {
					feedDevice(t, facility, "door").CurrentStateID = "open"
				})
				return facilityPlayerCommandChanges(initial, resulting, "terminal-1", "command-open",
					[]domain.FacilityTransitionRequest{
						{DeviceID: "power", TransitionID: "shut-down"},
						{DeviceID: "door", TransitionID: "open"},
					})
			},
			want: []domain.FacilityChange{{DeviceID: "door", PlayerCommand: &domain.FacilityChangePlayerCommand{
				TerminalID: "terminal-1", CommandID: "command-open", TransitionID: "open",
				FromStateID: "sealed", ToStateID: "open",
			}}},
		},
		{
			name: "player refusal records each named device once in ID order",
			build: func(t *testing.T) []domain.FacilityChange {
				return facilityPlayerRefusalChanges("terminal-1", "command-open", []domain.FacilityTransitionRequest{
					{DeviceID: "power", TransitionID: "shut-down"},
					{DeviceID: "door", TransitionID: "open"},
					{DeviceID: "power", TransitionID: "restart"},
				}, domain.FacilityChangeRefusalFaultBlocked)
			},
			want: []domain.FacilityChange{
				feedRefusalEntry("door", domain.FacilityChangeRefusalFaultBlocked),
				feedRefusalEntry("power", domain.FacilityChangeRefusalFaultBlocked),
			},
		},
		{
			name: "player refusal of an action without transitions",
			build: func(t *testing.T) []domain.FacilityChange {
				return facilityPlayerRefusalChanges("terminal-1", "command-open", nil, domain.FacilityChangeRefusalDeclined)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := test.build(t)

			require.Empty(t, cmp.Diff(test.want, got, cmpopts.EquateEmpty()))
			require.Empty(t, cmp.Diff(feedFacility(), initial), "a builder must not change the facility it reads")
		})
	}
}

func TestFacilityChangesCarryIDsOnly(t *testing.T) {
	t.Parallel()

	initial := feedFacility()
	played := feedFacilityWith(func(facility *domain.Facility) {
		feedDevice(t, facility, "power").CurrentStateID = "offline"
		feedDevice(t, facility, "door").CurrentStateID = "open"
		feedCondition(t, facility, "door-alarm").CurrentActive = true
	})
	requests := []domain.FacilityTransitionRequest{{DeviceID: "door", TransitionID: "open"}}
	var changes []domain.FacilityChange
	changes = append(changes, feedOne(facilityTransitionChange(initial, played, "door", "open"))...)
	changes = append(changes, feedOne(facilityForcedChange(initial, played, "power"))...)
	changes = append(changes, feedOne(facilityFaultChange(played, "door-alarm"))...)
	changes = append(changes, feedOne(facilityResetChange(played, initial, "door"))...)
	changes = append(changes, facilitySceneResetChanges(played, initial)...)
	changes = append(changes, facilityPlayerCommandChanges(initial, played, "terminal-1", "command-open", requests)...)
	changes = append(changes, facilityPlayerRefusalChanges("terminal-1", "command-open", requests,
		domain.FacilityChangeRefusalWithdrawn)...)
	require.Len(t, changes, 8, "every builder must produce its entry")
	runtime := &domain.ProcessRuntime{Facility: initial}
	require.True(t, appendFacilityChanges(runtime, time.UnixMilli(feedTestTime), changes...))

	encoded, err := json.Marshal(facilityChangeSnapshot(runtime.FacilityChanges))

	require.NoError(t, err)
	for _, name := range feedFacilityNames() {
		require.NotContains(t, string(encoded), name, "feed entries must carry IDs, never names")
	}
}

// feedFacility lists its devices out of ID order, power before door, and
// holds two faults of the door, listed out of ID order, and one fault of a
// terminal. Every name differs from every ID.
func feedFacility() *domain.Facility {
	return &domain.Facility{
		Revision: 3,
		Devices: []domain.FacilityDevice{
			{
				ID: "power", Name: "Power grid", Kind: domain.FacilityDeviceKindPowerGrid,
				InitialStateID: "online", CurrentStateID: "online",
				States: []domain.FacilityDeviceState{{ID: "online", Name: "Grid online"}, {ID: "offline", Name: "Grid offline"}},
				Transitions: []domain.FacilityDeviceTransition{{
					ID: "shut-down", Name: "Shut the grid down", SourceStateID: "online", DestinationStateID: "offline",
				}},
			},
			{
				ID: "door", Name: "Vault door", Kind: domain.FacilityDeviceKindDoor,
				InitialStateID: "sealed", CurrentStateID: "sealed",
				States: []domain.FacilityDeviceState{{ID: "sealed", Name: "Door sealed"}, {ID: "open", Name: "Door open"}},
				Transitions: []domain.FacilityDeviceTransition{{
					ID: "open", Name: "Open the vault door", SourceStateID: "sealed", DestinationStateID: "open",
					Preconditions:    []domain.FacilityStateEquality{{DeviceID: "power", StateID: "online"}},
					ConditionEffects: []domain.FacilityConditionEffect{{ConditionID: "door-alarm", Active: true}},
				}},
			},
		},
		Conditions: []domain.DiagnosticCondition{
			{ID: "door-jam", Name: "Door jammed", Device: &domain.DiagnosticDeviceScope{DeviceID: "door"}},
			{ID: "door-alarm", Name: "Door alarm", Device: &domain.DiagnosticDeviceScope{DeviceID: "door"}},
			{
				ID: "screen-noise", Name: "Screen noise",
				Terminal: &domain.DiagnosticTerminalScope{TerminalID: "terminal-1"},
			},
		},
	}
}

func feedFacilityNames() []string {
	return []string{
		"Power grid", "Grid online", "Grid offline", "Shut the grid down",
		"Vault door", "Door sealed", "Door open", "Open the vault door",
		"Door jammed", "Door alarm", "Screen noise",
	}
}

func feedFacilityWith(edit func(*domain.Facility)) *domain.Facility {
	facility := feedFacility()
	edit(facility)
	return facility
}

func feedDevice(t *testing.T, facility *domain.Facility, deviceID string) *domain.FacilityDevice {
	t.Helper()
	for index := range facility.Devices {
		if facility.Devices[index].ID == deviceID {
			return &facility.Devices[index]
		}
	}
	require.FailNowf(t, "missing device", "no device %q", deviceID)
	return nil
}

func feedCondition(t *testing.T, facility *domain.Facility, conditionID string) *domain.DiagnosticCondition {
	t.Helper()
	condition := facilityCondition(facility, conditionID)
	require.NotNilf(t, condition, "no condition %q", conditionID)
	return condition
}

func feedOne(change domain.FacilityChange, ok bool) []domain.FacilityChange {
	if !ok {
		return nil
	}
	return []domain.FacilityChange{change}
}

func feedForcedEntry(deviceID string) domain.FacilityChange {
	return domain.FacilityChange{
		DeviceID: deviceID, Forced: &domain.FacilityChangeForced{FromStateID: "from", ToStateID: "to"},
	}
}

func feedRefusalEntry(deviceID string, reason domain.FacilityChangeRefusalReason) domain.FacilityChange {
	return domain.FacilityChange{DeviceID: deviceID, PlayerRefusal: &domain.FacilityChangePlayerRefusal{
		TerminalID: "terminal-1", CommandID: "command-open", Reason: reason,
	}}
}

func feedStamped(change domain.FacilityChange, sequence uint64) domain.FacilityChange {
	change.Sequence = sequence
	change.OccurredAtUnixMs = feedTestTime
	return change
}

func cloneFeedForTest(feed map[string][]domain.FacilityChange) map[string][]domain.FacilityChange {
	clone := make(map[string][]domain.FacilityChange, len(feed))
	for deviceID, changes := range feed {
		clone[deviceID] = domain.CloneFacilityChanges(changes)
	}
	return clone
}

func feedSequences(changes []domain.FacilityChange) []uint64 {
	sequences := make([]uint64, len(changes))
	for index, change := range changes {
		sequences[index] = change.Sequence
	}
	return sequences
}

func feedDeviceIDs(changes []domain.FacilityChange) []string {
	deviceIDs := make([]string, len(changes))
	for index, change := range changes {
		deviceIDs[index] = change.DeviceID
	}
	return deviceIDs
}
