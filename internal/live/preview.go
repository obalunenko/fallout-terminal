package live

import (
	"strings"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
)

// PreviewDifferenceKind names one way a preview changes what players see.
type PreviewDifferenceKind string

const (
	PreviewDifferenceNameChanged        PreviewDifferenceKind = "name-changed"
	PreviewDifferenceTextChanged        PreviewDifferenceKind = "text-changed"
	PreviewDifferenceAppears            PreviewDifferenceKind = "appears"
	PreviewDifferenceDisappears         PreviewDifferenceKind = "disappears"
	PreviewDifferenceBecomesAvailable   PreviewDifferenceKind = "becomes-available"
	PreviewDifferenceBecomesUnavailable PreviewDifferenceKind = "becomes-unavailable"
)

// PreviewDifference is one change players of a terminal would see under a
// preview override. BlockID is set only for a text change. Before holds the
// current value and After the previewed one: the node name for a name change,
// the block text for a text change, the previewed name for an appearance, and
// the current name for a disappearance. Availability changes carry neither.
type PreviewDifference struct {
	Kind    PreviewDifferenceKind `json:"kind"`
	NodeID  string                `json:"nodeId"`
	BlockID *string               `json:"blockId,omitempty"`
	Before  *string               `json:"before,omitempty"`
	After   *string               `json:"after,omitempty"`
}

// previewSide holds the inputs that decide what players see on one side of a
// preview comparison.
type previewSide struct {
	completed       map[string]domain.CommandExecutionState
	completedBlocks map[string]string
	deviceStates    map[string]projectedDeviceState
	diagnostics     diagnosticProjection
}

// PreviewDifferences lists what players of the checkpoint's terminal would
// see change if the preview override replaced the facility's current values.
// It rejects an override exactly as PreviewFacility does and never changes
// its inputs.
//
// Both sides are resolved node by node and block by block with the resolvers
// the projection uses. Differences follow tree order; a node's own
// differences precede those of its blocks, which follow block order. A node
// that appears or disappears is reported once, with nothing beneath it, and a
// node hidden on both sides adds nothing.
func PreviewDifferences(
	state *domain.TerminalRuntime,
	facility *domain.Facility,
	preview domain.FacilityPreview,
) ([]PreviewDifference, []domain.FacilityIssue) {
	candidate, issues := previewFacilityCandidate(state, facility, preview)
	if len(issues) != 0 {
		return nil, issues
	}
	authored := state.AuthoredTree
	if authored.ID == "" {
		authored = state.Tree
	}
	before := newPreviewSide(authored, state.CommandStates, facility, state.TerminalID)
	after := newPreviewSide(authored, state.CommandStates, candidate, state.TerminalID)
	return appendPreviewDifferences(nil, authored, before, after, true), nil
}

// previewFacilityCandidate validates an override against the checkpoint and
// facility like PreviewFacility and returns a detached facility carrying it.
func previewFacilityCandidate(
	state *domain.TerminalRuntime,
	facility *domain.Facility,
	preview domain.FacilityPreview,
) (*domain.Facility, []domain.FacilityIssue) {
	if state == nil {
		return nil, facilityPreviewIssue(domain.FacilityFailureRuntimeContextEnded, "terminal", "")
	}
	if facility == nil {
		return nil, facilityPreviewIssue(domain.FacilityFailureMissingReference, "facility", "")
	}
	if preview.ExpectedFacilityRevision != facility.Revision {
		return nil, facilityPreviewIssue(domain.FacilityFailureStaleRevision, "facility", "")
	}
	if preview.TerminalID == "" || strings.TrimSpace(preview.TerminalID) != preview.TerminalID ||
		preview.TerminalID != state.TerminalID {
		return nil, facilityPreviewIssue(domain.FacilityFailureMissingReference, "terminal", preview.TerminalID)
	}
	if (preview.DeviceState == nil) == (preview.Condition == nil) {
		return nil, facilityPreviewIssue(domain.FacilityFailureInvalidConfiguration, "preview", "")
	}

	candidate := domain.CloneFacility(facility)
	var issues []domain.FacilityIssue
	if preview.DeviceState != nil {
		issues = applyDeviceStatePreview(candidate, *preview.DeviceState)
	} else {
		issues = applyConditionPreview(candidate, *preview.Condition)
	}
	if len(issues) != 0 {
		return nil, issues
	}
	return candidate, nil
}

// newPreviewSide prepares the same resolver inputs projectFacility derives
// from detached values.
func newPreviewSide(
	authored domain.ContentNode,
	completed map[string]domain.CommandExecutionState,
	facility *domain.Facility,
	terminalID string,
) previewSide {
	diagnostics := evaluateDiagnostics(authored, facility, terminalID)
	markDiagnosticPathAncestors(authored, diagnostics.forcedVisible)
	return previewSide{
		completed:       completed,
		completedBlocks: completedBlockTexts(completed),
		deviceStates:    indexProjectedDeviceStates(facility),
		diagnostics:     diagnostics,
	}
}

func appendPreviewDifferences(
	differences []PreviewDifference,
	node domain.ContentNode,
	before previewSide,
	after previewSide,
	root bool,
) []PreviewDifference {
	visibleBefore := resolveNodeVisible(node, before.deviceStates, before.diagnostics, root)
	visibleAfter := resolveNodeVisible(node, after.deviceStates, after.diagnostics, root)
	switch {
	case !visibleBefore && !visibleAfter:
		return differences
	case !visibleBefore:
		return append(differences, PreviewDifference{
			Kind: PreviewDifferenceAppears, NodeID: node.ID,
			After: new(resolveNodeName(node, after.completed, after.deviceStates)),
		})
	case !visibleAfter:
		return append(differences, PreviewDifference{
			Kind: PreviewDifferenceDisappears, NodeID: node.ID,
			Before: new(resolveNodeName(node, before.completed, before.deviceStates)),
		})
	}

	nameBefore := resolveNodeName(node, before.completed, before.deviceStates)
	nameAfter := resolveNodeName(node, after.completed, after.deviceStates)
	if nameBefore != nameAfter {
		differences = append(differences, PreviewDifference{
			Kind: PreviewDifferenceNameChanged, NodeID: node.ID, Before: new(nameBefore), After: new(nameAfter),
		})
	}
	availableBefore := previewNodeAvailable(node, before.deviceStates)
	availableAfter := previewNodeAvailable(node, after.deviceStates)
	if availableBefore != availableAfter {
		kind := PreviewDifferenceBecomesUnavailable
		if availableAfter {
			kind = PreviewDifferenceBecomesAvailable
		}
		differences = append(differences, PreviewDifference{Kind: kind, NodeID: node.ID})
	}
	if node.Type == domain.NodeEntry {
		for _, block := range node.Blocks {
			textBefore := resolveBlockText(block, before.completedBlocks, before.deviceStates, before.diagnostics)
			textAfter := resolveBlockText(block, after.completedBlocks, after.deviceStates, after.diagnostics)
			if textBefore != textAfter {
				differences = append(differences, PreviewDifference{
					Kind: PreviewDifferenceTextChanged, NodeID: node.ID, BlockID: new(block.ID),
					Before: new(textBefore), After: new(textAfter),
				})
			}
		}
	}

	for _, child := range node.Children {
		differences = appendPreviewDifferences(differences, child, before, after, false)
	}
	return differences
}

// previewNodeAvailable reports the projected availability, where a node with
// no availability rule is available.
func previewNodeAvailable(node domain.ContentNode, deviceStates map[string]projectedDeviceState) bool {
	available := resolveNodeAvailable(node, deviceStates)
	return available == nil || *available
}
