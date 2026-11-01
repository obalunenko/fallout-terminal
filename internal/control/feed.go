package control

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
)

// facilityChangeLimit is how many recent changes each device's feed keeps.
const facilityChangeLimit = 5

// appendFacilityChanges records changes, in order, in their devices' feeds.
// Every entry gets the next run-wide sequence and the commit time at. Entries
// for devices outside the runtime facility are skipped. It reports whether
// any entry was recorded.
func appendFacilityChanges(runtime *domain.ProcessRuntime, at time.Time, changes ...domain.FacilityChange) bool {
	if runtime == nil || runtime.Facility == nil {
		return false
	}
	recorded := false
	for _, change := range changes {
		if facilityDevice(runtime.Facility, change.DeviceID) == nil {
			continue
		}
		runtime.FacilityChangeSequence++
		change.Sequence = runtime.FacilityChangeSequence
		change.OccurredAtUnixMs = at.UnixMilli()
		if runtime.FacilityChanges == nil {
			runtime.FacilityChanges = make(map[string][]domain.FacilityChange)
		}
		feed := append([]domain.FacilityChange{change}, runtime.FacilityChanges[change.DeviceID]...)
		if len(feed) > facilityChangeLimit {
			feed = feed[:facilityChangeLimit]
		}
		runtime.FacilityChanges[change.DeviceID] = domain.CloneFacilityChanges(feed)
		recorded = true
	}
	return recorded
}

// facilityChangeSnapshot flattens every device's feed, newest first by
// sequence. An empty feed yields nil.
func facilityChangeSnapshot(feed map[string][]domain.FacilityChange) []domain.FacilityChange {
	var flat []domain.FacilityChange
	for _, changes := range feed {
		flat = append(flat, domain.CloneFacilityChanges(changes)...)
	}
	if len(flat) == 0 {
		return nil
	}
	slices.SortFunc(flat, func(left, right domain.FacilityChange) int {
		return cmp.Compare(right.Sequence, left.Sequence)
	})
	return flat
}

// pruneFacilityChanges drops the feeds of devices missing from the runtime
// facility.
func pruneFacilityChanges(runtime *domain.ProcessRuntime) {
	if runtime == nil {
		return
	}
	maps.DeleteFunc(runtime.FacilityChanges, func(deviceID string, _ []domain.FacilityChange) bool {
		return facilityDevice(runtime.Facility, deviceID) == nil
	})
}

// cloneFacilityChangeFeed returns a deeply detached per-device feed.
func cloneFacilityChangeFeed(feed map[string][]domain.FacilityChange) map[string][]domain.FacilityChange {
	if feed == nil {
		return nil
	}
	clone := make(map[string][]domain.FacilityChange, len(feed))
	for deviceID, changes := range feed {
		clone[deviceID] = domain.CloneFacilityChanges(changes)
	}
	return clone
}

// facilityChangeRefusalReason maps a facility failure of a player request to
// its feed reason. A duplicate decision, or no failure, records nothing.
func facilityChangeRefusalReason(failure domain.FacilityFailureCode) (domain.FacilityChangeRefusalReason, bool) {
	switch failure {
	case "", domain.FacilityFailureUnspecified, domain.FacilityFailureDuplicate:
		return "", false
	case domain.FacilityFailureRejected:
		return domain.FacilityChangeRefusalDeclined, true
	case domain.FacilityFailurePreconditionFailed:
		return domain.FacilityChangeRefusalPreconditionFailed, true
	case domain.FacilityFailureInvalidTransition:
		return domain.FacilityChangeRefusalTransitionUnavailable, true
	case domain.FacilityFailureStaleRevision:
		return domain.FacilityChangeRefusalStale, true
	default:
		return domain.FacilityChangeRefusalFailed, true
	}
}

// facilityTransitionChange records an Overseer card transition of deviceID.
func facilityTransitionChange(
	previous, resulting *domain.Facility,
	deviceID, transitionID string,
) (domain.FacilityChange, bool) {
	from, to, ok := movedFacilityDevice(previous, resulting, deviceID)
	if !ok {
		return domain.FacilityChange{}, false
	}
	return domain.FacilityChange{DeviceID: deviceID, Transition: &domain.FacilityChangeTransition{
		TransitionID: transitionID, FromStateID: from, ToStateID: to,
	}}, true
}

// facilityForcedChange records a state the Overseer forced on deviceID.
func facilityForcedChange(previous, resulting *domain.Facility, deviceID string) (domain.FacilityChange, bool) {
	from, to, ok := movedFacilityDevice(previous, resulting, deviceID)
	if !ok {
		return domain.FacilityChange{}, false
	}
	return domain.FacilityChange{
		DeviceID: deviceID,
		Forced:   &domain.FacilityChangeForced{FromStateID: from, ToStateID: to},
	}, true
}

// facilityFaultChange records a fault switch in the feed of the fault's
// device. A terminal-scoped fault has no device feed.
func facilityFaultChange(resulting *domain.Facility, conditionID string) (domain.FacilityChange, bool) {
	condition := facilityCondition(resulting, conditionID)
	if condition == nil || condition.Device == nil {
		return domain.FacilityChange{}, false
	}
	return domain.FacilityChange{DeviceID: condition.Device.DeviceID, Fault: &domain.FacilityChangeFault{
		ConditionID: conditionID, Active: condition.CurrentActive,
	}}, true
}

// facilityResetChange records a device reset that changed deviceID's state or
// its device-scoped faults.
func facilityResetChange(previous, resulting *domain.Facility, deviceID string) (domain.FacilityChange, bool) {
	reset, ok := facilityDeviceReset(previous, resulting, deviceID)
	if !ok {
		return domain.FacilityChange{}, false
	}
	return domain.FacilityChange{DeviceID: deviceID, Reset: reset}, true
}

// facilitySceneResetChanges records one scene-reset entry per changed device,
// in device ID order.
func facilitySceneResetChanges(previous, resulting *domain.Facility) []domain.FacilityChange {
	if resulting == nil {
		return nil
	}
	deviceIDs := make([]string, 0, len(resulting.Devices))
	for _, device := range resulting.Devices {
		deviceIDs = append(deviceIDs, device.ID)
	}
	slices.Sort(deviceIDs)
	var changes []domain.FacilityChange
	for _, deviceID := range deviceIDs {
		if reset, ok := facilityDeviceReset(previous, resulting, deviceID); ok {
			changes = append(changes, domain.FacilityChange{DeviceID: deviceID, SceneReset: reset})
		}
	}
	return changes
}

// facilityPlayerCommandChanges records each device an approved player command
// moved, in device ID order.
func facilityPlayerCommandChanges(
	previous, resulting *domain.Facility,
	terminalID, commandID string,
	requests []domain.FacilityTransitionRequest,
) []domain.FacilityChange {
	ordered := slices.Clone(requests)
	slices.SortStableFunc(ordered, func(left, right domain.FacilityTransitionRequest) int {
		return strings.Compare(left.DeviceID, right.DeviceID)
	})
	var changes []domain.FacilityChange
	for _, request := range ordered {
		from, to, ok := movedFacilityDevice(previous, resulting, request.DeviceID)
		if !ok {
			continue
		}
		changes = append(changes, domain.FacilityChange{
			DeviceID: request.DeviceID,
			PlayerCommand: &domain.FacilityChangePlayerCommand{
				TerminalID: terminalID, CommandID: commandID, TransitionID: request.TransitionID,
				FromStateID: from, ToStateID: to,
			},
		})
	}
	return changes
}

// facilityPlayerRefusalChanges records one refusal per device a player request
// targeted, in device ID order.
func facilityPlayerRefusalChanges(
	terminalID, commandID string,
	requests []domain.FacilityTransitionRequest,
	reason domain.FacilityChangeRefusalReason,
) []domain.FacilityChange {
	deviceIDs := make([]string, 0, len(requests))
	for _, request := range requests {
		deviceIDs = append(deviceIDs, request.DeviceID)
	}
	var changes []domain.FacilityChange
	for _, deviceID := range sortedUniqueStrings(deviceIDs) {
		changes = append(changes, domain.FacilityChange{
			DeviceID: deviceID,
			PlayerRefusal: &domain.FacilityChangePlayerRefusal{
				TerminalID: terminalID, CommandID: commandID, Reason: reason,
			},
		})
	}
	return changes
}

func movedFacilityDevice(previous, resulting *domain.Facility, deviceID string) (string, string, bool) {
	before := facilityDevice(previous, deviceID)
	after := facilityDevice(resulting, deviceID)
	if before == nil || after == nil || before.CurrentStateID == after.CurrentStateID {
		return "", "", false
	}
	return before.CurrentStateID, after.CurrentStateID, true
}

func facilityDeviceReset(previous, resulting *domain.Facility, deviceID string) (*domain.FacilityChangeReset, bool) {
	before := facilityDevice(previous, deviceID)
	after := facilityDevice(resulting, deviceID)
	if before == nil || after == nil {
		return nil, false
	}
	var conditionIDs []string
	for _, condition := range resulting.Conditions {
		if condition.Device == nil || condition.Device.DeviceID != deviceID {
			continue
		}
		if prior := facilityCondition(previous, condition.ID); prior != nil && prior.CurrentActive != condition.CurrentActive {
			conditionIDs = append(conditionIDs, condition.ID)
		}
	}
	if before.CurrentStateID == after.CurrentStateID && len(conditionIDs) == 0 {
		return nil, false
	}
	slices.Sort(conditionIDs)
	return &domain.FacilityChangeReset{
		FromStateID: before.CurrentStateID, ToStateID: after.CurrentStateID, ConditionIDs: conditionIDs,
	}, true
}

// facilityDevice returns the device with deviceID, or nil.
func facilityDevice(facility *domain.Facility, deviceID string) *domain.FacilityDevice {
	if facility == nil {
		return nil
	}
	for index := range facility.Devices {
		if facility.Devices[index].ID == deviceID {
			return &facility.Devices[index]
		}
	}
	return nil
}
