// Package domain contains transport-independent Fallout Terminal models.
package domain

import (
	"encoding/json"
	"maps"
)

const (
	// NodeFolder identifies a recursive folder node.
	NodeFolder = "folder"
	// NodeCommand identifies a command/output leaf node.
	NodeCommand = "command"
	// NodeEntry identifies a descriptive entry leaf node.
	NodeEntry = "entry"
)

// Session is the durable version-1 campaign document.
type Session struct {
	Version        int                        `json:"version"`
	Name           string                     `json:"name"`
	PlayerConfig   string                     `json:"playerConfig,omitempty"`
	Terminals      []Terminal                 `json:"terminals"`
	TerminalGroups []TerminalGroup            `json:"terminalGroups,omitempty"`
	Extra          map[string]json.RawMessage `json:"-"`
}

// TerminalGroup is the durable high-level representation of one ordered set
// of terminals. A standalone terminal is represented by a one-member group.
type TerminalGroup struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	TerminalIDs []string `json:"terminalIds"`
}

// TerminalGroupSnapshot is a detached lookup of one canonical ordered group.
type TerminalGroupSnapshot struct {
	ID          string
	Name        string
	TerminalIDs []string
}

// TerminalGroupCandidate is one complete trusted replacement proposal guarded
// by both durable-session and process-runtime revisions.
type TerminalGroupCandidate struct {
	TerminalGroups               []TerminalGroup
	ExpectedSessionRevision      uint64
	ExpectedCoordinationRevision uint64
}

// TerminalReturnOrigin distinguishes an authored route point from a prefix
// seeded when a fresh broadcast starts in the middle of an ordered group.
type TerminalReturnOrigin string

const (
	TerminalReturnAuthored      TerminalReturnOrigin = "authored-transition"
	TerminalReturnInitialPrefix TerminalReturnOrigin = "initial-group-prefix"
)

// PlayerConfig is the durable version-1 authored player roster. It is stored
// separately from Session so runtime recognition, claims, and terminal state
// cannot cross the persistence boundary.
type PlayerConfig struct {
	Version int                    `json:"version"`
	Name    string                 `json:"name"`
	Roster  []CharacterRosterEntry `json:"roster"`
}

// PlayerConfigHandle is the private active-file identity used for atomic
// roster saves. Path is exposed only to the trusted desktop projection.
type PlayerConfigHandle struct {
	Path          string
	Version       int
	Name          string
	ContentDigest string
}

// PlayerConfigMetadata is the detached Overseer view of the active config.
type PlayerConfigMetadata struct {
	Status   string `json:"status"`
	Name     string `json:"name"`
	FilePath string `json:"filePath"`
	Version  int    `json:"version"`
}

// StateChangeConfig is the optional authored configuration for a command
// whose first successful execution durably changes its menu presentation.
type StateChangeConfig struct {
	CompletedName    string `json:"completedName"`
	ConfirmationText string `json:"confirmationText"`
}

// TerminalTransitionConfig links an authored command to another terminal in
// the same durable version-1 session.
type TerminalTransitionConfig struct {
	TargetTerminalID string `json:"targetTerminalId"`
}

// CommandBehavior is the single semantic variant selected for a command.
// The durable JSON-v1 fields remain pointers so malformed documents containing
// both known fields can be decoded and rejected instead of silently losing data.
type CommandBehavior string

const (
	CommandBehaviorOrdinary           CommandBehavior = "ordinary"
	CommandBehaviorStateChange        CommandBehavior = "state-change"
	CommandBehaviorTerminalTransition CommandBehavior = "terminal-transition"
	CommandBehaviorInvalid            CommandBehavior = "invalid"
)

// CommandExecutionState is the immutable durable snapshot captured from a
// state-changing command's first successfully persisted execution.
type CommandExecutionState struct {
	CompletedName string `json:"completedName"`
	ResultText    string `json:"resultText"`
}

// Terminal is one durable authoring and broadcast target.
type Terminal struct {
	ID            string                           `json:"id"`
	Name          string                           `json:"name"`
	HackLevel     int                              `json:"hackLevel"`
	IntroText     string                           `json:"introText"`
	Root          ContentNode                      `json:"root"`
	CommandStates map[string]CommandExecutionState `json:"commandStates,omitempty"`
	Extra         map[string]json.RawMessage       `json:"-"`
}

// ContentNode is a tagged folder, command, or entry node.
type ContentNode struct {
	ID                 string                     `json:"id"`
	Type               string                     `json:"type"`
	Name               string                     `json:"name"`
	Children           []ContentNode              `json:"children,omitempty"`
	Text               string                     `json:"text,omitempty"`
	Description        string                     `json:"description,omitempty"`
	StateChange        *StateChangeConfig         `json:"stateChange,omitempty"`
	TerminalTransition *TerminalTransitionConfig  `json:"terminalTransition,omitempty"`
	Extra              map[string]json.RawMessage `json:"-"`
}

// Behavior returns the command's discriminated behavior without mutating its
// JSON-v1 representation. Invalid identifies a defensive dual-config state.
func (n ContentNode) Behavior() CommandBehavior {
	switch {
	case n.StateChange != nil && n.TerminalTransition != nil:
		return CommandBehaviorInvalid
	case n.StateChange != nil:
		return CommandBehaviorStateChange
	case n.TerminalTransition != nil:
		return CommandBehaviorTerminalTransition
	default:
		return CommandBehaviorOrdinary
	}
}

// NavState is the shared server-authoritative player position.
type NavState struct {
	Path          []string `json:"path"`
	Mode          string   `json:"mode"`
	ViewEntryID   *string  `json:"viewEntryId"`
	CommandNodeID *string  `json:"commandNodeId"`
}

// HackWord describes a visible word placement in a hacking column.
type HackWord struct {
	ID     string `json:"id"`
	Start  int    `json:"start"`
	Length int    `json:"length"`
}

// HackColumn is one 192-character public hacking column.
type HackColumn struct {
	Addresses []string   `json:"addresses"`
	Text      string     `json:"text"`
	Words     []HackWord `json:"words"`
}

// HackCandidate is private lookup data for a placed hacking word.
type HackCandidate struct {
	Text string
}

// HackPatternIdentity is the complete one-use identity of a bracket span.
// Row, Start, and End are rendered-row coordinates; GenerationID prevents a
// delayed action from targeting coincident coordinates in a later puzzle.
type HackPatternIdentity struct {
	GenerationID string
	Row          int
	Start        int
	End          int
}

// HackPattern is one valid bracket span derived from the current board text.
// Storage coordinates and Pair remain private discovery metadata.
type HackPattern struct {
	Identity      HackPatternIdentity
	ColumnIndex   int
	AbsoluteStart int
	AbsoluteEnd   int
	Pair          string
}

// PublicHackPattern is the client-safe projection of one current bracket span.
type PublicHackPattern struct {
	ID    string `json:"id"`
	Row   int    `json:"row"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Used  bool   `json:"used"`
}

// HackState is the canonical private hacking aggregate.
type HackState struct {
	GenerationID string
	Level        int
	WordLength   int
	AttemptsMax  int
	AttemptsLeft int
	SecretWord   string
	WordsByID    map[string]HackCandidate
	UsedPatterns map[HackPatternIdentity]struct{}
	Solved       bool
	Failed       bool
	Log          []string
	Columns      []HackColumn
}

// PublicHackState is the only hacking representation permitted at a client boundary.
type PublicHackState struct {
	Level        int                 `json:"level"`
	WordLength   int                 `json:"wordLength"`
	AttemptsMax  int                 `json:"attemptsMax"`
	AttemptsLeft int                 `json:"attemptsLeft"`
	Solved       bool                `json:"solved"`
	Failed       bool                `json:"failed"`
	Log          []string            `json:"log"`
	Columns      []HackColumn        `json:"columns"`
	Patterns     []PublicHackPattern `json:"patterns"`
}

// LiveState is the private process-local canonical broadcast state.
type LiveState struct {
	TerminalID   string
	TerminalName string
	Tree         ContentNode
	HackLevel    int
	IntroText    string
	Nav          NavState
	Hack         *HackState
}

// PublicLiveState is the immutable client-facing live snapshot.
type PublicLiveState struct {
	TerminalID         string                          `json:"terminalId"`
	TerminalName       string                          `json:"terminalName"`
	Tree               ContentNode                     `json:"tree"`
	HackLevel          int                             `json:"hackLevel"`
	IntroText          string                          `json:"introText"`
	Nav                NavState                        `json:"nav"`
	Hack               *PublicHackState                `json:"hack"`
	CommandExecution   *CommandExecutionPresentation   `json:"commandExecution,omitempty"`
	TerminalNavigation *TerminalNavigationPresentation `json:"terminalNavigation,omitempty"`
	Presentation       ControllerTerminalPresentation  `json:"controllerPresentation"`
}

// LogicalSessionID identifies one browser profile for the lifetime of a server process.
type LogicalSessionID string

// BrowserToken is an opaque, process-local recognition handle. It is private
// coordinator state and must never appear in a public projection.
type BrowserToken string

// RecognitionHandle is the public name for the opaque process-local browser
// recognition value. BrowserToken remains as a compatibility alias while the
// legacy transport is removed.
type RecognitionHandle = BrowserToken

// ConnectionID identifies one concrete public stream.
type ConnectionID string

// PhysicalStream is detached identity metadata for one active subscription.
// Queue, cancellation, and synchronization objects remain transport-owned and
// are deliberately not part of this serializable boundary value.
type PhysicalStream struct {
	ID        ConnectionID
	SessionID LogicalSessionID
}

// CharacterID identifies one process-local roster entry.
type CharacterID string

// BroadcastID identifies one live-broadcast lifetime.
type BroadcastID string

// SwitchID identifies one pending terminal-switch decision.
type SwitchID string

// RequestID correlates one player command with its authoritative result.
// It remains wire-compatible with the browser-generated string identifier.
type RequestID = string

// RuntimeCommandKind identifies one supported shared player command after decoding.
type RuntimeCommandKind string

const (
	RuntimeCommandSelectCharacter RuntimeCommandKind = "select-character"
	RuntimeCommandNavigate        RuntimeCommandKind = "navigate"
	RuntimeCommandGuess           RuntimeCommandKind = "guess"
	RuntimeCommandActivatePattern RuntimeCommandKind = "activate-pattern"
	RuntimeCommandPresentation    RuntimeCommandKind = "presentation"
)

// ControllerTerminalPresentationKind identifies the stable semantic portion
// of the terminal view shared by the controller and every observer. Pointer
// coordinates, DOM focus, viewport geometry, and audio eligibility remain
// deliberately outside this process-local value.
type ControllerTerminalPresentationKind string

const (
	ControllerTerminalPresentationNone    ControllerTerminalPresentationKind = "none"
	ControllerTerminalPresentationMenu    ControllerTerminalPresentationKind = "menu"
	ControllerTerminalPresentationPage    ControllerTerminalPresentationKind = "page"
	ControllerTerminalPresentationHacking ControllerTerminalPresentationKind = "hacking"
)

// ControllerTerminalPresentation is the complete semantic selection for one
// active terminal context. Exactly the fields admitted by Kind may be set.
type ControllerTerminalPresentation struct {
	Kind       ControllerTerminalPresentationKind `json:"kind"`
	ContextKey string                             `json:"contextKey"`
	TargetID   string                             `json:"targetId,omitempty"`
	PatternID  string                             `json:"patternId,omitempty"`
	PageIndex  uint32                             `json:"pageIndex,omitempty"`
}

// PlayerRole is a logical session's current broadcast-wide authority.
type PlayerRole string

const (
	PlayerRoleUnassigned PlayerRole = "unassigned"
	PlayerRoleActive     PlayerRole = "active"
	PlayerRoleObserver   PlayerRole = "observer"
)

// PlayerPhase determines which authoritative player surface is currently visible.
type PlayerPhase string

const (
	PlayerPhaseNoBroadcast PlayerPhase = "no-broadcast"
	PlayerPhaseSelecting   PlayerPhase = "selecting"
	PlayerPhaseWaiting     PlayerPhase = "waiting"
	PlayerPhaseControlling PlayerPhase = "controlling"
	PlayerPhaseObserving   PlayerPhase = "observing"
)

// RosterStatus is the player-safe availability of a roster entry.
type RosterStatus string

const (
	RosterStatusAvailable RosterStatus = "available"
	RosterStatusClaimed   RosterStatus = "claimed"
)

// TerminalLifecycle distinguishes the active runtime from an exact suspended checkpoint.
type TerminalLifecycle string

const (
	TerminalLifecycleActive    TerminalLifecycle = "active"
	TerminalLifecycleSuspended TerminalLifecycle = "suspended"
)

// TerminalSwitchChoice is the Overseer's explicit unfinished-puzzle decision.
type TerminalSwitchChoice string

const (
	TerminalSwitchPreserve TerminalSwitchChoice = "preserve"
	TerminalSwitchDiscard  TerminalSwitchChoice = "discard"
	TerminalSwitchCancel   TerminalSwitchChoice = "cancel"
)

// CommandExecutionDecision is the trusted Overseer resolution for the
// exact currently pending state-changing command request.
type CommandExecutionDecision string

const (
	CommandExecutionApprove CommandExecutionDecision = "approve"
	CommandExecutionReject  CommandExecutionDecision = "reject"
)

// CommandApprovalMode records the exact behavior that will run after the
// Overseer approves a pending command. Completed state-changing commands
// remain distinct because approval must show their frozen result without a
// second durable write.
type CommandApprovalMode string

const (
	CommandApprovalModeOrdinary             CommandApprovalMode = "ordinary"
	CommandApprovalModeStateChange          CommandApprovalMode = "state-change"
	CommandApprovalModeCompletedStateChange CommandApprovalMode = "completed-state-change"
)

// CommandExecutionPhase is the public broadcast-scoped presentation state.
type CommandExecutionPhase string

const (
	CommandExecutionPhasePending  CommandExecutionPhase = "pending"
	CommandExecutionPhaseRejected CommandExecutionPhase = "rejected"
)

// CommandExecutionPresentation exposes only the shared phase and stable
// command identity. Master prompt and request identity remain private.
type CommandExecutionPresentation struct {
	Phase     CommandExecutionPhase `json:"phase"`
	CommandID string                `json:"commandId"`
}

// TerminalNavigationDirection distinguishes forward links from LIFO returns.
type TerminalNavigationDirection string

const (
	TerminalNavigationForward TerminalNavigationDirection = "forward"
	TerminalNavigationReturn  TerminalNavigationDirection = "return"
)

type TerminalNavigationDecision string

const (
	TerminalNavigationApprove TerminalNavigationDecision = "approve"
	TerminalNavigationReject  TerminalNavigationDecision = "reject"
)

// TerminalReturnPoint is one immutable broadcast-scoped route stack entry.
type TerminalReturnPoint struct {
	TerminalID        string
	TerminalName      string
	FolderID          string
	AncestorFolderIDs []string
	CommandID         string
	CommandName       string
	Origin            TerminalReturnOrigin
	GroupID           string
	GroupPosition     int
}

// PendingTerminalNavigation is the exact private decision awaiting the Overseer.
type PendingTerminalNavigation struct {
	RequestID           string
	BroadcastID         BroadcastID
	ControllerSessionID LogicalSessionID
	Direction           TerminalNavigationDirection
	SourceTerminalID    string
	SourceTerminalName  string
	CommandID           string
	CommandName         string
	TargetTerminalID    string
	TargetTerminalName  string
	ReturnPoint         TerminalReturnPoint
}

// TerminalNavigationNoticeReason is a safe private failure category.
type TerminalNavigationNoticeReason string

const (
	TerminalNavigationNoticeTargetMissing TerminalNavigationNoticeReason = "target-missing"
	TerminalNavigationNoticeSelfTarget    TerminalNavigationNoticeReason = "self-target"
	TerminalNavigationNoticeCommandStale  TerminalNavigationNoticeReason = "command-stale"
	TerminalNavigationNoticeTargetChanged TerminalNavigationNoticeReason = "target-changed"
)

type TerminalNavigationNotice struct {
	Reason           TerminalNavigationNoticeReason
	SourceTerminalID string
	CommandID        string
	TargetTerminalID *string
}

type TerminalReturnTarget struct {
	TerminalID   string `json:"terminalId"`
	TerminalName string `json:"terminalName"`
}

type PendingTerminalNavigationPresentation struct {
	Direction          TerminalNavigationDirection `json:"direction"`
	TargetTerminalID   string                      `json:"targetTerminalId"`
	TargetTerminalName string                      `json:"targetTerminalName"`
}

type TerminalNavigationPresentation struct {
	RouteDepth   uint32                                 `json:"routeDepth"`
	ReturnTarget *TerminalReturnTarget                  `json:"returnTarget,omitempty"`
	Pending      *PendingTerminalNavigationPresentation `json:"pending,omitempty"`
}

// PlayerNoticeKind is an enum-only, detail-free personalized notice.
type PlayerNoticeKind string

const PlayerNoticeCommandPersistenceFailed PlayerNoticeKind = "command-persistence-failed"

type PlayerNotice struct {
	Kind PlayerNoticeKind `json:"kind"`
}

// ActionReason is a stable public explanation for a player-command outcome.
type ActionReason string

const (
	ActionReasonAccepted               ActionReason = "accepted"
	ActionReasonInvalidSession         ActionReason = "invalid-session"
	ActionReasonStaleBroadcast         ActionReason = "stale-broadcast"
	ActionReasonUnassigned             ActionReason = "unassigned"
	ActionReasonNotController          ActionReason = "not-controller"
	ActionReasonControllerDisconnected ActionReason = "controller-disconnected"
	ActionReasonStaleTerminal          ActionReason = "stale-terminal"
	ActionReasonInvalidAction          ActionReason = "invalid-action"
	ActionReasonConflict               ActionReason = "conflict"
	ActionReasonDuplicate              ActionReason = "duplicate"
)

// ActionResult is the authoritative outcome of one correlated player command.
type ActionResult struct {
	RequestID RequestID    `json:"requestId"`
	Accepted  bool         `json:"accepted"`
	Reason    ActionReason `json:"reason"`
	Revision  uint64       `json:"revision"`
}

// RuntimeCommand is the transport-independent form of a shared player request.
// Only fields relevant to its command kind are populated.
type RuntimeCommand struct {
	RequestID          RequestID
	BroadcastID        BroadcastID
	TerminalID         string
	Kind               RuntimeCommandKind
	Action             string
	NodeID             string
	TargetID           string
	PatternID          string
	Presentation       ControllerTerminalPresentation
	PayloadFingerprint string
}

// RequestResultRecord retains enough information to make request replay idempotent.
type RequestResultRecord struct {
	Fingerprint string
	Result      ActionResult
}

// RequestReplayRecord is the complete detached value retained by the bounded
// Connect mutation replay cache. It carries no transport request object.
type RequestReplayRecord struct {
	RequestID          RequestID
	Procedure          string
	PayloadFingerprint string
	Result             ActionResult
	Revision           uint64
}

// BrowserRecognition is the private mapping from an opaque browser token to a
// process-local logical session.
type BrowserRecognition struct {
	BrowserToken BrowserToken
	SessionID    LogicalSessionID
}

// LogicalSession is canonical process-local browser-profile state.
type LogicalSession struct {
	ID             LogicalSessionID
	FallbackName   string
	ConnectionIDs  map[ConnectionID]struct{}
	RequestResults map[RequestID]RequestResultRecord
	Notice         *PlayerNotice
}

// CharacterCreatePayload is the complete trusted roster profile requested by
// the Overseer. ExpectedRevision makes retries and concurrent edits
// explicit at the coordinator transaction boundary.
type CharacterCreatePayload struct {
	Name                string
	Intelligence        int
	HackerPerkAvailable bool
	ExpectedRevision    uint64
}

// CharacterUpdatePayload is the complete replacement profile for one stable
// roster identity. ExpectedRevision serializes trusted desktop edits against
// the coordinator's authoritative state.
type CharacterUpdatePayload struct {
	CharacterID         CharacterID
	Name                string
	Intelligence        int
	HackerPerkAvailable bool
	ExpectedRevision    uint64
}

// CharacterDeletePayload identifies one roster profile to remove at an exact
// coordinator revision.
type CharacterDeletePayload struct {
	CharacterID      CharacterID
	ExpectedRevision uint64
}

// CharacterRosterEntry is one stable process-local player identity option.
type CharacterRosterEntry struct {
	ID                  CharacterID `json:"id"`
	Name                string      `json:"name"`
	Intelligence        int         `json:"intelligence"`
	HackerPerkAvailable bool        `json:"hackerPerkAvailable"`
}

// CharacterAssignment is one broadcast-scoped exclusive claim.
type CharacterAssignment struct {
	BroadcastID BroadcastID
	SessionID   LogicalSessionID
	CharacterID CharacterID
}

// ControllerAssignment designates the one assigned session allowed to mutate shared state.
type ControllerAssignment struct {
	SessionID LogicalSessionID
}

// TerminalRuntime is an exact canonical active or suspended terminal checkpoint.
type TerminalRuntime struct {
	TerminalID       string
	TerminalName     string
	Tree             ContentNode
	CommandStates    map[string]CommandExecutionState
	CommandExecution *CommandExecutionPresentation
	HackLevel        int
	IntroText        string
	Nav              NavState
	Hack             *HackState
	Presentation     ControllerTerminalPresentation
	Lifecycle        TerminalLifecycle
}

// LiveBroadcast owns all state whose lifetime ends with the current broadcast.
type LiveBroadcast struct {
	ID                           BroadcastID
	AssignmentsBySession         map[LogicalSessionID]CharacterID
	SessionByCharacter           map[CharacterID]LogicalSessionID
	ControllerSessionID          *LogicalSessionID
	ActiveTerminalID             *string
	TerminalRuntimes             map[string]*TerminalRuntime
	Route                        []TerminalReturnPoint
	InitialTerminalEstablished   bool
	InitialTerminalID            string
	InitialTerminalGroupID       string
	InitialTerminalGroupPosition int
}

// TerminalTarget is the validated authored payload retained by a pending switch.
type TerminalTarget struct {
	TerminalID    string
	TerminalName  string
	Tree          ContentNode
	CommandStates map[string]CommandExecutionState
	HackLevel     int
	IntroText     string
}

// TerminalTransitionTarget is a detached trusted lookup of an authored link.
type TerminalTransitionTarget struct {
	SourceTerminalID   string
	SourceTerminalName string
	CommandID          string
	CommandName        string
	Target             TerminalTarget
}

// PendingCommandExecution is the single broadcast-scoped request awaiting a
// private Overseer decision. ControllerSessionID is coordinator-private.
type PendingCommandExecution struct {
	RequestID           string
	BroadcastID         BroadcastID
	TerminalID          string
	CommandID           string
	CommandName         string
	Mode                CommandApprovalMode
	ConfirmationText    string
	ControllerSessionID LogicalSessionID
}

// TerminalSwitchDecision keeps a switch request ordered against the source runtime.
// A nil Target requests clearing the active terminal while retaining the broadcast.
type TerminalSwitchDecision struct {
	ID               SwitchID
	BroadcastID      BroadcastID
	SourceTerminalID string
	Target           *TerminalTarget
}

// ProcessRuntime is the private canonical root owned by the coordination service.
// It is intentionally unrelated to the durable version-1 Session document.
type ProcessRuntime struct {
	Revision                  uint64
	SessionsByID              map[LogicalSessionID]*LogicalSession
	SessionIDByBrowserToken   map[BrowserToken]LogicalSessionID
	RosterByID                map[CharacterID]*CharacterRosterEntry
	RosterOrder               []CharacterID
	ActivePlayerConfig        *PlayerConfigHandle
	Broadcast                 *LiveBroadcast
	PendingSwitch             *TerminalSwitchDecision
	PendingCommandExecution   *PendingCommandExecution
	PendingTerminalNavigation *PendingTerminalNavigation
	TerminalNavigationNotice  *TerminalNavigationNotice
}

// PlayerCharacter is the assigned identity visible at a projection boundary.
type PlayerCharacter struct {
	ID   CharacterID `json:"id"`
	Name string      `json:"name"`
}

// PlayerRosterEntry contains availability without claimant or presence details.
type PlayerRosterEntry struct {
	ID     CharacterID  `json:"id"`
	Name   string       `json:"name"`
	Status RosterStatus `json:"status"`
}

// PlayerState is one complete personalized, secret-free browser projection.
// Empty broadcast and terminal IDs marshal as null to match the frozen protocol.
type PlayerState struct {
	Revision         uint64              `json:"revision"`
	SessionID        LogicalSessionID    `json:"sessionId"`
	FallbackName     string              `json:"fallbackName"`
	Character        *PlayerCharacter    `json:"character"`
	Role             PlayerRole          `json:"role"`
	Phase            PlayerPhase         `json:"phase"`
	BroadcastID      BroadcastID         `json:"-"`
	ActiveTerminalID string              `json:"-"`
	Roster           []PlayerRosterEntry `json:"roster"`
	Notice           *PlayerNotice       `json:"notice,omitempty"`
}

// TerminalPresentation is an exclusive detached public terminal projection.
// Live is non-nil for a complete active terminal; NoLiveTerminal is true for
// the explicit empty variant. Adapters reject every other combination.
type TerminalPresentation struct {
	Live           *PublicLiveState
	NoLiveTerminal bool
}

// PersonalizedSnapshot is the mandatory first value for every subscription.
type PersonalizedSnapshot struct {
	RecognitionHandle RecognitionHandle
	Revision          uint64
	PlayerState       *PlayerState
	Terminal          TerminalPresentation
}

// CompoundUpdate is one complete personalized publication for a committed
// revision. Nil components mean unchanged, never clear or partial patch.
type CompoundUpdate struct {
	Revision uint64
	Player   *PlayerState
	Terminal *TerminalPresentation
	Nav      *NavState
	Hack     *PublicHackState
}

// BrowserPendingAction tracks the two independent acknowledgements required
// before an accepted browser action is no longer pending.
type BrowserPendingAction struct {
	RequestID      RequestID
	Result         *ActionResult
	StreamRevision uint64
}

// SoundCategory is one stable allowlisted same-origin asset group.
type SoundCategory string

const (
	SoundCategoryAmbient    SoundCategory = "ambient"
	SoundCategoryHackGood   SoundCategory = "hack-good"
	SoundCategoryHackBad    SoundCategory = "hack-bad"
	SoundCategoryMenuFocus  SoundCategory = "menu-focus"
	SoundCategorySingle     SoundCategory = "single"
	SoundCategoryMultiple   SoundCategory = "multiple"
	SoundCategoryEnter      SoundCategory = "enter"
	SoundCategoryCharscroll SoundCategory = "charscroll"
)

// SoundManifest contains only sorted safe relative same-origin asset paths.
type SoundManifest struct {
	Category SoundCategory
	Assets   []string
}

// MarshalJSON preserves nullable identifiers while keeping convenient typed
// scalar fields for coordinator and protocol construction.
func (state PlayerState) MarshalJSON() ([]byte, error) {
	var broadcastID *BroadcastID
	if state.BroadcastID != "" {
		value := state.BroadcastID
		broadcastID = &value
	}
	var activeTerminalID *string
	if state.ActiveTerminalID != "" {
		value := state.ActiveTerminalID
		activeTerminalID = &value
	}
	return json.Marshal(struct {
		Revision         uint64              `json:"revision"`
		SessionID        LogicalSessionID    `json:"sessionId"`
		FallbackName     string              `json:"fallbackName"`
		Character        *PlayerCharacter    `json:"character"`
		Role             PlayerRole          `json:"role"`
		Phase            PlayerPhase         `json:"phase"`
		BroadcastID      *BroadcastID        `json:"broadcastId"`
		ActiveTerminalID *string             `json:"activeTerminalId"`
		Roster           []PlayerRosterEntry `json:"roster"`
		Notice           *PlayerNotice       `json:"notice,omitempty"`
	}{
		Revision:         state.Revision,
		SessionID:        state.SessionID,
		FallbackName:     state.FallbackName,
		Character:        state.Character,
		Role:             state.Role,
		Phase:            state.Phase,
		BroadcastID:      broadcastID,
		ActiveTerminalID: activeTerminalID,
		Roster:           state.Roster,
		Notice:           state.Notice,
	})
}

// MasterRosterEntry is the Overseer view of one roster claim.
type MasterRosterEntry struct {
	ID                  CharacterID       `json:"id"`
	Name                string            `json:"name"`
	Intelligence        int               `json:"intelligence"`
	HackerPerkAvailable bool              `json:"hackerPerkAvailable"`
	ClaimedBySessionID  *LogicalSessionID `json:"claimedBySessionId"`
}

// MasterSessionEntry is the Overseer view of one recognized logical session.
type MasterSessionEntry struct {
	ID           LogicalSessionID `json:"id"`
	FallbackName string           `json:"fallbackName"`
	Connected    bool             `json:"connected"`
	Character    *PlayerCharacter `json:"character"`
	Role         PlayerRole       `json:"role"`
}

// MasterBroadcastState is the Overseer view of the current broadcast epoch.
type MasterBroadcastState struct {
	ID                  BroadcastID       `json:"id"`
	ControllerSessionID *LogicalSessionID `json:"controllerSessionId"`
	ActiveTerminalID    *string           `json:"activeTerminalId"`
}

// MasterPendingSwitch is the non-secret metadata for one pending switch decision.
type MasterPendingSwitch struct {
	SwitchID         SwitchID    `json:"switchId"`
	BroadcastID      BroadcastID `json:"broadcastId"`
	SourceTerminalID string      `json:"sourceTerminalId"`
	TargetTerminalID *string     `json:"targetTerminalId"`
}

// MasterPendingCommandExecution is the complete private prompt projection.
type MasterPendingCommandExecution struct {
	RequestID        string              `json:"requestId"`
	BroadcastID      BroadcastID         `json:"broadcastId"`
	TerminalID       string              `json:"terminalId"`
	CommandID        string              `json:"commandId"`
	CommandName      string              `json:"commandName"`
	Mode             CommandApprovalMode `json:"mode"`
	ConfirmationText string              `json:"confirmationText"`
}

type MasterPendingTerminalNavigation struct {
	RequestID          string                      `json:"requestId"`
	BroadcastID        BroadcastID                 `json:"broadcastId"`
	Direction          TerminalNavigationDirection `json:"direction"`
	SourceTerminalID   string                      `json:"sourceTerminalId"`
	SourceTerminalName string                      `json:"sourceTerminalName"`
	CommandID          string                      `json:"commandId"`
	CommandName        string                      `json:"commandName"`
	TargetTerminalID   string                      `json:"targetTerminalId"`
	TargetTerminalName string                      `json:"targetTerminalName"`
	RouteDepth         uint32                      `json:"routeDepth"`
}

type MasterTerminalNavigationNotice struct {
	Reason           TerminalNavigationNoticeReason `json:"reason"`
	SourceTerminalID string                         `json:"sourceTerminalId"`
	CommandID        string                         `json:"commandId"`
	TargetTerminalID *string                        `json:"targetTerminalId,omitempty"`
}

// MasterCoordinationState is one detached private-desktop projection.
type MasterCoordinationState struct {
	Revision                  uint64                           `json:"revision"`
	PlayerConfig              *PlayerConfigMetadata            `json:"playerConfig"`
	Roster                    []MasterRosterEntry              `json:"roster"`
	Sessions                  []MasterSessionEntry             `json:"sessions"`
	Broadcast                 *MasterBroadcastState            `json:"broadcast"`
	PendingSwitch             *MasterPendingSwitch             `json:"pendingSwitch"`
	PendingCommandExecution   *MasterPendingCommandExecution   `json:"pendingCommandExecution"`
	PendingTerminalNavigation *MasterPendingTerminalNavigation `json:"pendingTerminalNavigation"`
	TerminalNavigationNotice  *MasterTerminalNavigationNotice  `json:"terminalNavigationNotice"`
}

// CloneMasterCoordinationState returns a deeply detached desktop projection.
func CloneMasterCoordinationState(state *MasterCoordinationState) *MasterCoordinationState {
	if state == nil {
		return nil
	}
	clone := *state
	if state.PlayerConfig != nil {
		value := *state.PlayerConfig
		clone.PlayerConfig = &value
	}
	clone.Roster = append([]MasterRosterEntry(nil), state.Roster...)
	for index := range clone.Roster {
		clone.Roster[index].ClaimedBySessionID = cloneLogicalSessionID(state.Roster[index].ClaimedBySessionID)
	}
	clone.Sessions = append([]MasterSessionEntry(nil), state.Sessions...)
	for index := range clone.Sessions {
		clone.Sessions[index].Character = clonePlayerCharacter(state.Sessions[index].Character)
	}
	if state.Broadcast != nil {
		broadcast := *state.Broadcast
		broadcast.ControllerSessionID = cloneLogicalSessionID(state.Broadcast.ControllerSessionID)
		broadcast.ActiveTerminalID = cloneString(state.Broadcast.ActiveTerminalID)
		clone.Broadcast = &broadcast
	}
	if state.PendingSwitch != nil {
		pending := *state.PendingSwitch
		pending.TargetTerminalID = cloneString(state.PendingSwitch.TargetTerminalID)
		clone.PendingSwitch = &pending
	}
	if state.PendingCommandExecution != nil {
		pending := *state.PendingCommandExecution
		clone.PendingCommandExecution = &pending
	}
	if state.PendingTerminalNavigation != nil {
		pending := *state.PendingTerminalNavigation
		clone.PendingTerminalNavigation = &pending
	}
	if state.TerminalNavigationNotice != nil {
		notice := *state.TerminalNavigationNotice
		notice.TargetTerminalID = cloneString(state.TerminalNavigationNotice.TargetTerminalID)
		clone.TerminalNavigationNotice = &notice
	}
	return &clone
}

// ClonePlayerState returns a deeply detached personalized browser projection.
func ClonePlayerState(state *PlayerState) *PlayerState {
	if state == nil {
		return nil
	}
	clone := *state
	clone.Character = clonePlayerCharacter(state.Character)
	clone.Roster = append([]PlayerRosterEntry(nil), state.Roster...)
	if state.Notice != nil {
		notice := *state.Notice
		clone.Notice = &notice
	}
	return &clone
}

// CloneTerminalPresentation returns a deeply detached terminal variant.
func CloneTerminalPresentation(presentation TerminalPresentation) TerminalPresentation {
	return TerminalPresentation{
		Live:           clonePublicLiveState(presentation.Live),
		NoLiveTerminal: presentation.NoLiveTerminal,
	}
}

// ClonePersonalizedSnapshot returns a deeply detached first-stream value.
func ClonePersonalizedSnapshot(snapshot *PersonalizedSnapshot) *PersonalizedSnapshot {
	if snapshot == nil {
		return nil
	}
	clone := *snapshot
	clone.PlayerState = ClonePlayerState(snapshot.PlayerState)
	clone.Terminal = CloneTerminalPresentation(snapshot.Terminal)
	return &clone
}

// CloneCompoundUpdate returns a deeply detached authoritative publication.
func CloneCompoundUpdate(update *CompoundUpdate) *CompoundUpdate {
	if update == nil {
		return nil
	}
	clone := *update
	clone.Player = ClonePlayerState(update.Player)
	if update.Terminal != nil {
		terminal := CloneTerminalPresentation(*update.Terminal)
		clone.Terminal = &terminal
	}
	if update.Nav != nil {
		nav := *update.Nav
		nav.Path = append([]string(nil), update.Nav.Path...)
		nav.ViewEntryID = cloneString(update.Nav.ViewEntryID)
		nav.CommandNodeID = cloneString(update.Nav.CommandNodeID)
		clone.Nav = &nav
	}
	clone.Hack = clonePublicHackState(update.Hack)
	return &clone
}

func clonePublicLiveState(state *PublicLiveState) *PublicLiveState {
	if state == nil {
		return nil
	}
	clone := *state
	clone.Tree = CloneContentNode(state.Tree)
	clone.Nav.Path = append([]string(nil), state.Nav.Path...)
	clone.Nav.ViewEntryID = cloneString(state.Nav.ViewEntryID)
	clone.Nav.CommandNodeID = cloneString(state.Nav.CommandNodeID)
	clone.Hack = clonePublicHackState(state.Hack)
	if state.CommandExecution != nil {
		execution := *state.CommandExecution
		clone.CommandExecution = &execution
	}
	if state.TerminalNavigation != nil {
		navigation := *state.TerminalNavigation
		if state.TerminalNavigation.ReturnTarget != nil {
			returnTarget := *state.TerminalNavigation.ReturnTarget
			navigation.ReturnTarget = &returnTarget
		}
		if state.TerminalNavigation.Pending != nil {
			pending := *state.TerminalNavigation.Pending
			navigation.Pending = &pending
		}
		clone.TerminalNavigation = &navigation
	}
	return &clone
}

// CloneContentNode returns a deeply detached copy of authored terminal content.
func CloneContentNode(node ContentNode) ContentNode {
	clone := node
	if node.StateChange != nil {
		stateChange := *node.StateChange
		clone.StateChange = &stateChange
	}
	if node.TerminalTransition != nil {
		transition := *node.TerminalTransition
		clone.TerminalTransition = &transition
	}
	clone.Extra = cloneRawMessages(node.Extra)
	if node.Children != nil {
		clone.Children = make([]ContentNode, len(node.Children))
		for index := range node.Children {
			clone.Children[index] = CloneContentNode(node.Children[index])
		}
	}
	return clone
}

// CloneLiveBroadcast returns a deeply detached process-local broadcast state.
// Runtime-only navigation provenance is intentionally preserved so snapshots
// cannot lose the distinction between authored and initially seeded routes.
func CloneLiveBroadcast(broadcast *LiveBroadcast) *LiveBroadcast {
	if broadcast == nil {
		return nil
	}
	clone := *broadcast
	if broadcast.AssignmentsBySession != nil {
		clone.AssignmentsBySession = make(map[LogicalSessionID]CharacterID, len(broadcast.AssignmentsBySession))
		maps.Copy(clone.AssignmentsBySession, broadcast.AssignmentsBySession)
	}
	if broadcast.SessionByCharacter != nil {
		clone.SessionByCharacter = make(map[CharacterID]LogicalSessionID, len(broadcast.SessionByCharacter))
		maps.Copy(clone.SessionByCharacter, broadcast.SessionByCharacter)
	}
	clone.ControllerSessionID = cloneLogicalSessionID(broadcast.ControllerSessionID)
	clone.ActiveTerminalID = cloneString(broadcast.ActiveTerminalID)
	if broadcast.TerminalRuntimes != nil {
		clone.TerminalRuntimes = make(map[string]*TerminalRuntime, len(broadcast.TerminalRuntimes))
		for terminalID, runtime := range broadcast.TerminalRuntimes {
			clone.TerminalRuntimes[terminalID] = cloneLiveTerminalRuntime(runtime)
		}
	}
	if broadcast.Route != nil {
		clone.Route = make([]TerminalReturnPoint, len(broadcast.Route))
		for index, point := range broadcast.Route {
			clone.Route[index] = point
			clone.Route[index].AncestorFolderIDs = append([]string(nil), point.AncestorFolderIDs...)
		}
	}
	return &clone
}

func cloneLiveTerminalRuntime(runtime *TerminalRuntime) *TerminalRuntime {
	if runtime == nil {
		return nil
	}
	clone := *runtime
	clone.Tree = CloneContentNode(runtime.Tree)
	if runtime.CommandStates != nil {
		clone.CommandStates = make(map[string]CommandExecutionState, len(runtime.CommandStates))
		maps.Copy(clone.CommandStates, runtime.CommandStates)
	}
	if runtime.CommandExecution != nil {
		execution := *runtime.CommandExecution
		clone.CommandExecution = &execution
	}
	clone.Nav.Path = append([]string(nil), runtime.Nav.Path...)
	clone.Nav.ViewEntryID = cloneString(runtime.Nav.ViewEntryID)
	clone.Nav.CommandNodeID = cloneString(runtime.Nav.CommandNodeID)
	clone.Hack = cloneLiveHackState(runtime.Hack)
	return &clone
}

func cloneLiveHackState(state *HackState) *HackState {
	if state == nil {
		return nil
	}
	clone := *state
	if state.WordsByID != nil {
		clone.WordsByID = make(map[string]HackCandidate, len(state.WordsByID))
		maps.Copy(clone.WordsByID, state.WordsByID)
	}
	if state.UsedPatterns != nil {
		clone.UsedPatterns = make(map[HackPatternIdentity]struct{}, len(state.UsedPatterns))
		maps.Copy(clone.UsedPatterns, state.UsedPatterns)
	}
	clone.Log = append([]string(nil), state.Log...)
	if state.Columns != nil {
		clone.Columns = make([]HackColumn, len(state.Columns))
		for index, column := range state.Columns {
			clone.Columns[index] = column
			clone.Columns[index].Addresses = append([]string(nil), column.Addresses...)
			clone.Columns[index].Words = append([]HackWord(nil), column.Words...)
		}
	}
	return &clone
}

// CloneSession returns a deeply detached durable document.
func CloneSession(session Session) Session {
	clone := session
	clone.Extra = cloneRawMessages(session.Extra)
	if session.TerminalGroups != nil {
		clone.TerminalGroups = CloneTerminalGroups(session.TerminalGroups)
	}
	if session.Terminals != nil {
		clone.Terminals = make([]Terminal, len(session.Terminals))
		for index, terminal := range session.Terminals {
			clone.Terminals[index] = terminal
			clone.Terminals[index].Extra = cloneRawMessages(terminal.Extra)
			clone.Terminals[index].Root = CloneContentNode(terminal.Root)
			if terminal.CommandStates != nil {
				clone.Terminals[index].CommandStates = make(map[string]CommandExecutionState, len(terminal.CommandStates))
				maps.Copy(clone.Terminals[index].CommandStates, terminal.CommandStates)
			}
		}
	}
	return clone
}

// CloneTerminalGroups returns a deeply detached ordered group set.
func CloneTerminalGroups(groups []TerminalGroup) []TerminalGroup {
	clone := make([]TerminalGroup, len(groups))
	for index, group := range groups {
		clone[index] = group
		clone[index].TerminalIDs = append([]string(nil), group.TerminalIDs...)
	}
	return clone
}

func cloneRawMessages(values map[string]json.RawMessage) map[string]json.RawMessage {
	if values == nil {
		return nil
	}
	clone := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		clone[key] = append(json.RawMessage(nil), value...)
	}
	return clone
}

func clonePublicHackState(state *PublicHackState) *PublicHackState {
	if state == nil {
		return nil
	}
	clone := *state
	clone.Log = append([]string(nil), state.Log...)
	clone.Patterns = append([]PublicHackPattern(nil), state.Patterns...)
	clone.Columns = make([]HackColumn, len(state.Columns))
	for index, column := range state.Columns {
		clone.Columns[index] = HackColumn{
			Addresses: append([]string(nil), column.Addresses...),
			Text:      column.Text,
			Words:     append([]HackWord(nil), column.Words...),
		}
	}
	return &clone
}

func cloneLogicalSessionID(value *LogicalSessionID) *LogicalSessionID {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func clonePlayerCharacter(value *PlayerCharacter) *PlayerCharacter {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

// ServerInfo is safe status displayed to the Overseer.
type ServerInfo struct {
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	URL         string `json:"url"`
	LocalURL    string `json:"localUrl,omitempty"`
	Tunnel      bool   `json:"tunnel"`
	TunnelError string `json:"tunnelError,omitempty"`
}
