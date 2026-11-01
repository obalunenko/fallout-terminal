package session

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
)

// AuthoringSaveRequest is one complete authored session saved against the
// authoring revision the Overseer last saw. The live values in Candidate are
// ignored and replaced by the canonical ones.
type AuthoringSaveRequest struct {
	Candidate                 domain.Session
	ExpectedAuthoringRevision uint64
	CorrelationID             string
}

// AuthoringSaveResult reports one authoring save. On success Session is the
// saved canonical session; on rejection it is the canonical session to
// rebase onto, when one is active.
type AuthoringSaveResult = domain.FacilityOperationResult

// SaveAuthoring validates and durably saves one complete authored session.
//
// The save is checked against the authoring revision only, so command
// executions, world actions, recoveries, and resets never reject it. The
// candidate's command states, device current states, fault activity, terminal
// groups, and facility revision are replaced with the canonical values, and
// entities new to the candidate start at their initial values. The merged
// candidate is written once; the facility revision steps only when the
// facility definition changed, and the authoring revision steps only when the
// document changed.
func (service *Service) SaveAuthoring(ctx context.Context, request AuthoringSaveRequest) AuthoringSaveResult {
	if service == nil || contextError(ctx) != nil {
		return authoringSaveFailure(request, domain.FacilityFailureRuntimeContextEnded, 0, 0, 0, nil, nil)
	}
	service.commandMu.Lock()
	defer service.commandMu.Unlock()
	service.documentMu.Lock()
	defer service.documentMu.Unlock()

	service.mu.Lock()
	if service.closed || service.active.Path == "" || service.active.Session == nil {
		sessionRevision := service.active.SavedRevision
		service.mu.Unlock()
		return authoringSaveFailure(request, domain.FacilityFailureRuntimeContextEnded, 0, sessionRevision, 0, nil, nil)
	}

	canonical := domain.CloneSession(*service.active.Session)
	path := service.active.Path
	sessionRevision := service.active.SavedRevision
	authoringRevision := service.active.AuthoringRevision
	previousFacilityRevision := facilityRevision(&canonical)
	// reject releases service.mu and reports a failure against the canonical
	// state read above.
	reject := func(failure domain.FacilityFailureCode, issues ...domain.FacilityIssue) AuthoringSaveResult {
		service.mu.Unlock()
		return authoringSaveFailure(
			request, failure, authoringRevision, sessionRevision, previousFacilityRevision, &canonical, issues,
		)
	}

	if samePath(path, service.locations.BundledDemo) {
		return reject(domain.FacilityFailureInvalidConfiguration, authoringIssue("session", ""))
	}
	if request.ExpectedAuthoringRevision != authoringRevision {
		return reject(domain.FacilityFailureStaleRevision)
	}
	if strings.TrimSpace(request.CorrelationID) == "" {
		return reject(domain.FacilityFailureInvalidConfiguration, authoringIssue("request", ""))
	}

	candidate, issues := mergeAuthoringLiveValues(request.Candidate, canonical)
	if len(issues) == 0 {
		issues = validateAuthoringCandidate(candidate, canonical)
	}
	if len(issues) != 0 {
		return reject(facilityAuthoringFailureCode(issues), issues...)
	}
	if sessionsEqual(candidate, canonical) {
		service.mu.Unlock()
		result := facilityAuthoringSuccess(
			request.CorrelationID, false, sessionRevision, previousFacilityRevision, nil, nil, canonical,
		)
		result.AuthoringRevision = authoringRevision
		return result
	}

	var affectedDeviceIDs, affectedConditionIDs []string
	if facilityDefinitionChanged(canonical.Facility, candidate.Facility) {
		affectedDeviceIDs, affectedConditionIDs = facilityAuthoringAffectedIDs(canonical.Facility, candidate.Facility)
		candidate.Facility.Revision++
	}
	saved, err := service.commitCandidateLocked(path, canonical, candidate, true)
	if err != nil {
		return authoringSaveFailure(
			request, domain.FacilityFailureInvalidConfiguration, authoringRevision, sessionRevision,
			previousFacilityRevision, &canonical, []domain.FacilityIssue{authoringIssue("session", "")},
		)
	}
	if !saved.OK {
		return authoringSaveFailure(
			request, domain.FacilityFailurePersistenceFailed, authoringRevision, saved.SavedRevision,
			previousFacilityRevision, &canonical, nil,
		)
	}

	// commandMu excludes every other authored mutation and session switch, so
	// the authoring revision is still the one this save's write advanced.
	authoringRevision = service.Snapshot().AuthoringRevision
	result := facilityAuthoringSuccess(
		request.CorrelationID, true, saved.SavedRevision, previousFacilityRevision,
		affectedDeviceIDs, affectedConditionIDs, candidate,
	)
	result.AuthoringRevision = authoringRevision
	return result
}

// mergeAuthoringLiveValues returns a detached copy of candidate whose live
// values are the canonical ones: terminal command states, terminal groups,
// device current states, fault activity, and the facility revision.
func mergeAuthoringLiveValues(candidate, canonical domain.Session) (domain.Session, []domain.FacilityIssue) {
	merged := mergeCanonicalTerminalGroups(candidate, canonical)
	if issues := mergeAuthoringCommandStates(&merged, canonical); len(issues) != 0 {
		return domain.Session{}, issues
	}
	if canonical.Facility != nil && merged.Facility == nil {
		return domain.Session{}, []domain.FacilityIssue{authoringIssue("facility", "")}
	}
	protectFacilityAuthoringCurrentValues(canonical.Facility, merged.Facility)
	if merged.Facility != nil {
		merged.Facility.Revision = facilityRevision(&canonical)
	}
	return merged, nil
}

// mergeAuthoringCommandStates replaces the candidate's command states with
// the canonical ones. The state of a removed command is dropped; a command
// that keeps its state must keep its state-change configuration.
func mergeAuthoringCommandStates(candidate *domain.Session, canonical domain.Session) []domain.FacilityIssue {
	canonicalStates := make(map[string]map[string]domain.CommandExecutionState, len(canonical.Terminals))
	for _, terminal := range canonical.Terminals {
		canonicalStates[terminal.ID] = terminal.CommandStates
	}
	for index := range candidate.Terminals {
		terminal := &candidate.Terminals[index]
		states := canonicalStates[terminal.ID]
		terminal.CommandStates = nil
		for _, commandID := range slices.Sorted(maps.Keys(states)) {
			node := contentNodeByID(&terminal.Root, commandID)
			if node == nil {
				continue
			}
			if node.Type != domain.NodeCommand || node.StateChange == nil {
				return []domain.FacilityIssue{authoringIssue("command", commandID)}
			}
			if terminal.CommandStates == nil {
				terminal.CommandStates = make(map[string]domain.CommandExecutionState, len(states))
			}
			terminal.CommandStates[commandID] = states[commandID]
		}
	}
	return nil
}

// validateAuthoringCandidate checks a merged candidate against the canonical
// session it replaces: no new cross-group terminal transition, every removed
// facility entity unreferenced, and a valid private session contract.
func validateAuthoringCandidate(candidate, canonical domain.Session) []domain.FacilityIssue {
	if err := validateAuthoredTransitionChanges(candidate, canonical); err != nil {
		return []domain.FacilityIssue{authoringIssue("terminal-transition", "")}
	}
	if issues := domain.ValidateFacilityAuthoringCandidate(canonical, candidate); len(issues) != 0 {
		return issues
	}
	if err := verifySessionContract(candidate); err != nil {
		return []domain.FacilityIssue{authoringIssue("session", "")}
	}
	return nil
}

// facilityDefinitionChanged reports whether candidate would be stored
// differently from canonical. Both carry the same live values, so only an
// authored change makes them differ.
func facilityDefinitionChanged(canonical, candidate *domain.Facility) bool {
	if canonical == nil || candidate == nil {
		return canonical != candidate
	}
	canonicalData, canonicalErr := json.Marshal(canonical)
	candidateData, candidateErr := json.Marshal(candidate)
	return canonicalErr != nil || candidateErr != nil || !bytes.Equal(canonicalData, candidateData)
}

// authoringIssue names the entity an authoring save was refused for as an
// invalid configuration.
func authoringIssue(entityKind, entityID string) domain.FacilityIssue {
	issue := domain.FacilityIssue{Code: domain.FacilityFailureInvalidConfiguration, EntityKind: entityKind}
	if entityID != "" {
		issue.EntityID = new(entityID)
	}
	return issue
}

func authoringSaveFailure(
	request AuthoringSaveRequest,
	failure domain.FacilityFailureCode,
	authoringRevision uint64,
	sessionRevision uint64,
	facilityRevision uint64,
	session *domain.Session,
	issues []domain.FacilityIssue,
) AuthoringSaveResult {
	result := AuthoringSaveResult{
		CorrelationID:             request.CorrelationID,
		Failure:                   failure,
		Issues:                    cloneFacilityIssues(issues),
		SessionRevision:           sessionRevision,
		PreviousFacilityRevision:  facilityRevision,
		ResultingFacilityRevision: facilityRevision,
		AuthoringRevision:         authoringRevision,
	}
	if session != nil {
		result.Session = worldActionSessionPointer(*session)
	}
	return result
}
