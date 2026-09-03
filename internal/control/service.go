// Package control owns the process-local coordination aggregate shared by the
// trusted desktop boundary and untrusted player connections.
package control

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/nav"
)

const defaultRequestResultLimit = 256

// ErrCommandStateStorageUnavailable identifies coordinators constructed
// without the durable command-state capability.
var ErrCommandStateStorageUnavailable = errors.New("command state storage is unavailable")

// IDSource produces opaque process-local identifiers. Implementations must be
// safe for concurrent use because IDs may be prepared outside a transaction.
type IDSource interface {
	Next() string
}

// RuntimeActions applies one already-authorized command to a coordinator-owned
// terminal checkpoint. It performs gameplay rules only; authorization,
// revisions, request replay, and publication remain Service responsibilities.
type RuntimeActions interface {
	Apply(*domain.TerminalRuntime, domain.RuntimeCommand) (*domain.PublicLiveState, bool)
}

// TerminalRuntimeLifecycle creates and refreshes coordinator-owned terminal
// checkpoints without installing them into a second canonical live slot.
type TerminalRuntimeLifecycle interface {
	CreateRuntime(domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState)
	UpdateRuntime(*domain.TerminalRuntime, domain.TerminalTarget) *domain.PublicLiveState
	ProjectRuntime(*domain.TerminalRuntime) *domain.PublicLiveState
}

type terminalDecisionLifecycle interface {
	TerminalRuntimeLifecycle
	SuspendRuntime(*domain.TerminalRuntime)
	ReactivateRuntime(*domain.TerminalRuntime, domain.TerminalTarget) *domain.PublicLiveState
	DiscardRuntime(domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState)
}

type failedHackLifecycle interface {
	ResetFailedHack(*domain.TerminalRuntime, domain.TerminalTarget) (*domain.TerminalRuntime, *domain.PublicLiveState)
}

// TrustedHackRuntime is the private Overseer-only hacking operation used
// during the transition from the legacy live aggregate to coordinator-owned
// terminal slots. It is never exposed through the player protocol.
type TrustedHackRuntime interface {
	ForceRuntimeHackSuccess(*domain.TerminalRuntime) (*domain.PublicLiveState, bool)
}

// RosterStore persists one complete candidate player config. The coordinator
// calls it while holding the transition lock and commits only after success.
type RosterStore interface {
	Save(domain.PlayerConfigHandle, []domain.CharacterRosterEntry) (domain.PlayerConfigHandle, error)
}

// CommandStateMutation is the detached durable document result returned by a
// trusted command-state store. The coordinator owns publication and never
// supplies a callback to the store.
type CommandStateMutation struct {
	Changed  bool
	Revision uint64
	Session  domain.Session
}

// CommandStateStore is the narrow synchronous durability boundary for
// server-owned command execution snapshots. Coordinator transitions call it
// in the one-way control-to-session lock order and commit only after success.
type CommandStateStore interface {
	ExecuteCommandState(context.Context, string, string) (CommandStateMutation, error)
	ResetCommandState(context.Context, string, string) (CommandStateMutation, error)
	ResetTerminalCommandStates(context.Context, string) (CommandStateMutation, error)
}

// TerminalGroupMutation is the detached durable document returned after one
// complete group-set compare-and-replace.
type TerminalGroupMutation struct {
	Changed  bool
	Revision uint64
	Session  domain.Session
}

// TerminalGroupStoreRejection marks feedback already sanitized by the session
// owner. The coordinator preserves this message and canonical document while
// collapsing every untyped storage error at the trust boundary.
type TerminalGroupStoreRejection struct {
	Message string
}

func (rejection *TerminalGroupStoreRejection) Error() string {
	return rejection.Message
}

// TerminalGroupStore is the one-way control-to-session durability seam.
type TerminalGroupStore interface {
	ReplaceTerminalGroups(context.Context, []domain.TerminalGroup, uint64) (TerminalGroupMutation, error)
}

// TerminalCatalog resolves only detached values from the latest validated session.
type TerminalCatalog interface {
	LookupTerminal(terminalID string) (domain.TerminalTarget, bool)
	LookupTerminalTransition(sourceTerminalID, commandID string) (domain.TerminalTransitionTarget, bool)
}

type terminalGroupCatalog interface {
	LookupTerminalGroup(terminalID string) (domain.TerminalGroupSnapshot, bool)
}

// Config supplies deterministic seams for identifiers and ordered effects.
// Enqueue must only place the detached effect onto another owner; it must not
// call back into Service while the coordinator transaction is still locked.
type Config struct {
	IDs                IDSource
	Enqueue            func(Effect)
	Runtime            RuntimeActions
	Terminals          TerminalRuntimeLifecycle
	TrustedHack        TrustedHackRuntime
	RosterStore        RosterStore
	CommandStateStore  CommandStateStore
	TerminalGroupStore TerminalGroupStore
	TerminalCatalog    TerminalCatalog
	RequestResultLimit int
}

// SessionIdentity is the fresh process-local identity returned after an
// unrecognized player connection establishes a logical session.
type SessionIdentity struct {
	SessionID    domain.LogicalSessionID
	BrowserToken domain.BrowserToken
	State        *domain.PlayerState
}

// CharacterSelection is one transport-independent player claim request.
type CharacterSelection struct {
	ConnectionID domain.ConnectionID
	SessionID    domain.LogicalSessionID
	RequestID    domain.RequestID
	BroadcastID  domain.BroadcastID
	CharacterID  domain.CharacterID
	Fingerprint  string
}

// Effect is one transport-neutral publication produced by a transaction.
// Target IDs identify the intended logical session or concrete connection;
// nil payloads simply mean that projection family is absent from this effect.
type Effect struct {
	Revision          uint64
	SessionID         domain.LogicalSessionID
	ConnectionID      domain.ConnectionID
	Master            *domain.MasterCoordinationState
	Player            *domain.PlayerState
	Live              *domain.PublicLiveState
	Hack              *domain.PublicHackState
	TerminalID        string
	Result            *domain.ActionResult
	ClearLiveTerminal bool
	Update            *domain.CompoundUpdate
	Audit             []AuditEvent
}

// AuditEvent is a closed, display-content-free diagnostic fact emitted by an
// authoritative coordinator transition.
type AuditEvent struct {
	Name             string
	Decision         string
	Outcome          string
	Reason           string
	SessionID        domain.LogicalSessionID
	Role             domain.PlayerRole
	PreviousRole     domain.PlayerRole
	RequestID        string
	BroadcastID      domain.BroadcastID
	TerminalID       string
	CommandID        string
	Mode             string
	PuzzleID         string
	PreviousPuzzleID string
	HackLevel        int
	AttemptsMax      int
	AttemptsLeft     int
}

// Service serializes every process-runtime transition under one mutex.
type Service struct {
	mu sync.RWMutex

	runtime             domain.ProcessRuntime
	ids                 IDSource
	enqueue             func(Effect)
	actions             RuntimeActions
	terminals           TerminalRuntimeLifecycle
	trustedHack         TrustedHackRuntime
	rosterStore         RosterStore
	commandStateStore   CommandStateStore
	terminalGroupStore  TerminalGroupStore
	terminalCatalog     TerminalCatalog
	requirePlayerConfig bool
	requestResultLimit  int
}

type transition struct {
	accepted bool
	persist  bool
	effects  []Effect
	boundary func(uint64)
}

type transitionResult struct {
	accepted bool
	revision uint64
}

type cryptoIDSource struct {
	fallback atomic.Uint64
}

func (source *cryptoIDSource) Next() string {
	var value [16]byte
	if _, err := cryptorand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return fmt.Sprintf("%x-%x", time.Now().UnixNano(), source.fallback.Add(1))
}

// New returns an empty coordinator with process-local runtime state.
func New(config Config) *Service {
	ids := config.IDs
	if ids == nil {
		ids = &cryptoIDSource{}
	}
	enqueue := config.Enqueue
	if enqueue == nil {
		enqueue = func(Effect) {}
	}
	requestResultLimit := config.RequestResultLimit
	if requestResultLimit <= 0 {
		requestResultLimit = defaultRequestResultLimit
	}
	return &Service{
		runtime:             newProcessRuntime(),
		ids:                 ids,
		enqueue:             enqueue,
		actions:             config.Runtime,
		terminals:           config.Terminals,
		trustedHack:         config.TrustedHack,
		rosterStore:         config.RosterStore,
		commandStateStore:   config.CommandStateStore,
		terminalGroupStore:  config.TerminalGroupStore,
		terminalCatalog:     config.TerminalCatalog,
		requirePlayerConfig: config.RosterStore != nil,
		requestResultLimit:  requestResultLimit,
	}
}

// InstallPlayerConfig replaces the authored roster only while no broadcast is
// active. Logical sessions remain recognized; claims cannot exist here.
func (service *Service) InstallPlayerConfig(handle domain.PlayerConfigHandle, roster []domain.CharacterRosterEntry) (*domain.MasterCoordinationState, error) {
	var clonedRoster []domain.CharacterRosterEntry
	if roster != nil {
		clonedRoster = make([]domain.CharacterRosterEntry, len(roster))
		copy(clonedRoster, roster)
	}
	config := domain.PlayerConfig{Version: handle.Version, Name: handle.Name, Roster: clonedRoster}
	if strings.TrimSpace(handle.Path) == "" {
		return service.Snapshot(), fmt.Errorf("player config path must not be blank")
	}
	if err := domain.ValidatePlayerConfig(config); err != nil {
		return service.Snapshot(), err
	}

	var state *domain.MasterCoordinationState
	var installErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		if runtime.Broadcast != nil {
			state = masterSnapshot(runtime)
			installErr = fmt.Errorf("player config cannot change during a broadcast")
			return transition{}
		}
		runtime.RosterByID = make(map[domain.CharacterID]*domain.CharacterRosterEntry, len(roster))
		runtime.RosterOrder = make([]domain.CharacterID, 0, len(roster))
		for _, entry := range roster {
			value := entry
			runtime.RosterByID[entry.ID] = &value
			runtime.RosterOrder = append(runtime.RosterOrder, entry.ID)
		}
		value := handle
		runtime.ActivePlayerConfig = &value
		state = masterSnapshot(runtime)
		return transition{accepted: true, effects: stateEffects(runtime)}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), installErr
}

// ClearPlayerConfig removes only the active authored roster/config binding.
// It is used when another durable session is opened without an association.
func (service *Service) ClearPlayerConfig() (*domain.MasterCoordinationState, error) {
	var state *domain.MasterCoordinationState
	var clearErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		if runtime.Broadcast != nil {
			state = masterSnapshot(runtime)
			clearErr = fmt.Errorf("player config cannot change during a broadcast")
			return transition{}
		}
		runtime.ActivePlayerConfig = nil
		runtime.RosterByID = make(map[domain.CharacterID]*domain.CharacterRosterEntry)
		runtime.RosterOrder = nil
		state = masterSnapshot(runtime)
		return transition{accepted: true, effects: stateEffects(runtime)}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), clearErr
}

// Revision returns the latest accepted canonical revision.
func (service *Service) Revision() uint64 {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return service.runtime.Revision
}

// ResolveRecognition returns the current process-local logical session for an
// opaque recognition handle. It performs identity lookup only; callers must
// still enforce presence, assignment, controller, terminal, and action rules.
func (service *Service) ResolveRecognition(handle domain.RecognitionHandle) (domain.LogicalSessionID, bool) {
	if service == nil || handle == "" {
		return "", false
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	sessionID, ok := service.runtime.SessionIDByBrowserToken[handle]
	if !ok || service.runtime.SessionsByID[sessionID] == nil {
		return "", false
	}
	return sessionID, true
}

// ResolveConnection returns the logical owner of one active physical stream.
func (service *Service) ResolveConnection(connectionID domain.ConnectionID) (domain.LogicalSessionID, bool) {
	if service == nil || connectionID == "" {
		return "", false
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	sessionID, session := sessionForConnection(&service.runtime, connectionID)
	return sessionID, session != nil
}

// ActiveStreamCount reports raw physical stream membership, not logical
// presence. It is safe for the private desktop client-count projection.
func (service *Service) ActiveStreamCount() int {
	if service == nil {
		return 0
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	count := 0
	for _, session := range service.runtime.SessionsByID {
		if session != nil {
			count += len(session.ConnectionIDs)
		}
	}
	return count
}

func (service *Service) compoundUpdateLocked(runtime *domain.ProcessRuntime, sessionID domain.LogicalSessionID, revision uint64) *domain.CompoundUpdate {
	state, ok := playerSnapshot(runtime, sessionID)
	if !ok {
		return nil
	}
	state.Revision = revision
	update := &domain.CompoundUpdate{Revision: revision, Player: state}
	if service.terminals == nil || runtime.Broadcast == nil || !sessionAssigned(runtime.Broadcast, sessionID) {
		return update
	}
	terminal := activeTerminalRuntime(runtime.Broadcast)
	if terminal == nil || terminal.Lifecycle != domain.TerminalLifecycleActive {
		presentation := domain.TerminalPresentation{NoLiveTerminal: true}
		update.Terminal = &presentation
		return update
	}
	if live := decorateTerminalNavigation(runtime, service.terminals.ProjectRuntime(terminal)); live != nil {
		presentation := domain.TerminalPresentation{Live: clonePublicLiveState(live)}
		update.Terminal = &presentation
		update.Nav = &live.Nav
		update.Hack = live.Hack
	}
	return domain.CloneCompoundUpdate(update)
}

// Snapshot returns a deeply detached Overseer coordination projection.
func (service *Service) Snapshot() *domain.MasterCoordinationState {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return domain.CloneMasterCoordinationState(masterSnapshot(&service.runtime))
}

// ReplaceTerminalGroups serializes one trusted complete-set replacement with
// player navigation, validates runtime routes against the proposal, and only
// advances coordination after durable session replacement succeeds.
func (service *Service) ReplaceTerminalGroups(
	ctx context.Context,
	candidate domain.TerminalGroupCandidate,
) (*domain.MasterCoordinationState, *TerminalGroupMutation, error) {
	if ctx == nil {
		return service.Snapshot(), nil, fmt.Errorf("terminal group replacement context is required")
	}
	var state *domain.MasterCoordinationState
	var mutation *TerminalGroupMutation
	var replacementErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		state = masterSnapshot(runtime)
		if err := ctx.Err(); err != nil {
			replacementErr = fmt.Errorf("terminal group replacement was canceled")
			return transition{}
		}
		if candidate.ExpectedCoordinationRevision != runtime.Revision {
			replacementErr = fmt.Errorf(
				"coordination revision is stale: expected %d, current %d",
				candidate.ExpectedCoordinationRevision,
				runtime.Revision,
			)
			return transition{}
		}
		if service.terminalGroupStore == nil {
			replacementErr = fmt.Errorf("terminal group storage is unavailable")
			return transition{}
		}
		if err := validateRuntimeTerminalGroups(runtime, candidate.TerminalGroups); err != nil {
			replacementErr = err
			return transition{}
		}
		durable, err := service.terminalGroupStore.ReplaceTerminalGroups(
			ctx,
			domain.CloneTerminalGroups(candidate.TerminalGroups),
			candidate.ExpectedSessionRevision,
		)
		if err != nil {
			if rejection, ok := errors.AsType[*TerminalGroupStoreRejection](err); ok {
				mutation = &TerminalGroupMutation{
					Changed:  durable.Changed,
					Revision: durable.Revision,
					Session:  domain.CloneSession(durable.Session),
				}
				replacementErr = rejection
			} else {
				replacementErr = fmt.Errorf("could not save terminal groups")
			}
			return transition{}
		}
		mutation = &TerminalGroupMutation{
			Changed: durable.Changed, Revision: durable.Revision, Session: domain.CloneSession(durable.Session),
		}
		if !durable.Changed {
			return transition{}
		}
		state = masterSnapshot(runtime)
		return transition{accepted: true, effects: stateEffects(runtime)}
	})
	if state == nil {
		state = service.Snapshot()
	}
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), mutation, replacementErr
}

func validateRuntimeTerminalGroups(runtime *domain.ProcessRuntime, groups []domain.TerminalGroup) error {
	type membership struct {
		groupID  string
		position int
	}
	byTerminal := make(map[string]membership)
	byGroup := make(map[string][]string, len(groups))
	for _, group := range groups {
		byGroup[group.ID] = append([]string(nil), group.TerminalIDs...)
		for position, terminalID := range group.TerminalIDs {
			byTerminal[terminalID] = membership{groupID: group.ID, position: position}
		}
	}
	broadcast := runtime.Broadcast
	if broadcast != nil && broadcast.InitialTerminalEstablished && broadcast.InitialTerminalGroupID != "" {
		initial, exists := byTerminal[broadcast.InitialTerminalID]
		if !exists || initial.groupID != broadcast.InitialTerminalGroupID ||
			initial.position != broadcast.InitialTerminalGroupPosition {
			return fmt.Errorf(
				"terminal group candidate invalidates initialized terminal %q position in group %q",
				broadcast.InitialTerminalID,
				broadcast.InitialTerminalGroupID,
			)
		}
	}
	pending := runtime.PendingTerminalNavigation
	if pending != nil {
		source, sourceOK := byTerminal[pending.SourceTerminalID]
		target, targetOK := byTerminal[pending.TargetTerminalID]
		if !sourceOK || !targetOK || source.groupID != target.groupID {
			return fmt.Errorf(
				"terminal group candidate invalidates pending %s navigation %q -> %q",
				pending.Direction,
				pending.SourceTerminalID,
				pending.TargetTerminalID,
			)
		}
		if pending.Direction == domain.TerminalNavigationReturn {
			if broadcast == nil || len(broadcast.Route) == 0 ||
				!sameTerminalReturnPoint(broadcast.Route[len(broadcast.Route)-1], pending.ReturnPoint) {
				return fmt.Errorf(
					"terminal group candidate invalidates pending return %q -> %q route point",
					pending.SourceTerminalID,
					pending.TargetTerminalID,
				)
			}
			if pending.ReturnPoint.Origin == domain.TerminalReturnInitialPrefix &&
				(source.position != target.position+1 ||
					pending.ReturnPoint.GroupID != target.groupID ||
					pending.ReturnPoint.GroupPosition != target.position) {
				return fmt.Errorf(
					"terminal group candidate invalidates pending return adjacency %q -> %q",
					pending.SourceTerminalID,
					pending.TargetTerminalID,
				)
			}
		}
	}
	if broadcast == nil || broadcast.ActiveTerminalID == nil {
		return nil
	}
	active, activeOK := byTerminal[*broadcast.ActiveTerminalID]
	if !activeOK {
		return fmt.Errorf("terminal group candidate omits the active terminal %q", *broadcast.ActiveTerminalID)
	}
	seededPosition := 0
	seededChainEnded := false
	for routeIndex, point := range broadcast.Route {
		member, exists := byTerminal[point.TerminalID]
		if !exists || member.groupID != active.groupID {
			return fmt.Errorf("terminal group candidate invalidates the active return route at terminal %q", point.TerminalID)
		}
		if point.Origin != domain.TerminalReturnInitialPrefix {
			seededChainEnded = true
			continue
		}
		ordered := byGroup[point.GroupID]
		if seededChainEnded || point.GroupPosition != seededPosition {
			return fmt.Errorf("terminal group candidate invalidates seeded return successor chain at terminal %q", point.TerminalID)
		}
		if point.GroupID != member.groupID || point.GroupPosition < 0 || point.GroupPosition >= len(ordered) ||
			ordered[point.GroupPosition] != point.TerminalID || member.position != point.GroupPosition {
			return fmt.Errorf("terminal group candidate invalidates seeded return order at terminal %q", point.TerminalID)
		}
		if broadcast.InitialTerminalGroupID != "" &&
			(point.GroupID != broadcast.InitialTerminalGroupID || point.GroupPosition >= broadcast.InitialTerminalGroupPosition) {
			return fmt.Errorf("terminal group candidate invalidates seeded return successor chain at terminal %q", point.TerminalID)
		}
		successorID := *broadcast.ActiveTerminalID
		if routeIndex+1 < len(broadcast.Route) {
			successorID = broadcast.Route[routeIndex+1].TerminalID
		}
		successor := byTerminal[successorID]
		if successor.groupID != member.groupID || successor.position != point.GroupPosition+1 {
			if routeIndex == len(broadcast.Route)-1 {
				return fmt.Errorf(
					"terminal group candidate invalidates active seeded return adjacency %q -> %q",
					successorID,
					point.TerminalID,
				)
			}
			return fmt.Errorf(
				"terminal group candidate invalidates seeded return successor adjacency %q -> %q",
				successorID,
				point.TerminalID,
			)
		}
		seededPosition++
	}
	return nil
}

// PlayerSnapshot returns a deeply detached personalized projection for a
// recognized logical session.
func (service *Service) PlayerSnapshot(sessionID domain.LogicalSessionID) (*domain.PlayerState, bool) {
	service.mu.RLock()
	defer service.mu.RUnlock()
	state, ok := playerSnapshot(&service.runtime, sessionID)
	if !ok {
		return nil, false
	}
	return domain.ClonePlayerState(state), true
}

// CurrentLiveForSession returns the coordinator-owned active terminal only to
// a recognized session with a current-broadcast assignment. The projection
// and revision are captured under the same coordinator read lock so reconnects
// and newly opened tabs resume the canonical runtime without regenerating it.
func (service *Service) CurrentLiveForSession(sessionID domain.LogicalSessionID) (*domain.PublicLiveState, uint64, bool) {
	service.mu.RLock()
	defer service.mu.RUnlock()

	broadcast := service.runtime.Broadcast
	if service.terminals == nil || broadcast == nil || !sessionAssigned(broadcast, sessionID) {
		return nil, service.runtime.Revision, false
	}
	terminal := activeTerminalRuntime(broadcast)
	if terminal == nil || terminal.Lifecycle != domain.TerminalLifecycleActive {
		return nil, service.runtime.Revision, false
	}
	projection := decorateTerminalNavigation(&service.runtime, service.terminals.ProjectRuntime(terminal))
	if projection == nil {
		return nil, service.runtime.Revision, false
	}
	return clonePublicLiveState(projection), service.runtime.Revision, true
}

// AddCharacter appends one complete stable roster profile while no broadcast
// is active. All transaction guards run before stable-ID allocation or disk
// persistence so stale and retried requests cannot create duplicate players.
func (service *Service) AddCharacter(payload domain.CharacterCreatePayload) (*domain.MasterCoordinationState, error) {
	name, err := domain.ValidateCharacterName(payload.Name)
	if err != nil {
		return service.Snapshot(), err
	}
	if err := domain.ValidateCharacterIntelligence(payload.Intelligence); err != nil {
		return service.Snapshot(), err
	}

	var state *domain.MasterCoordinationState
	var addErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		if payload.ExpectedRevision != runtime.Revision {
			state = masterSnapshot(runtime)
			addErr = fmt.Errorf(
				"coordination revision is stale: expected %d, current %d",
				payload.ExpectedRevision,
				runtime.Revision,
			)
			return transition{}
		}
		if service.requirePlayerConfig && runtime.ActivePlayerConfig == nil {
			state = masterSnapshot(runtime)
			addErr = fmt.Errorf("select or create a player config first")
			return transition{}
		}
		if runtime.Broadcast != nil {
			state = masterSnapshot(runtime)
			addErr = fmt.Errorf("player roster cannot change during a broadcast")
			return transition{}
		}
		characterID := domain.CharacterID(service.nextID())
		candidateByID, candidateOrder := cloneRosterState(runtime)
		candidateByID[characterID] = &domain.CharacterRosterEntry{
			ID:                  characterID,
			Name:                name,
			Intelligence:        payload.Intelligence,
			HackerPerkAvailable: payload.HackerPerkAvailable,
		}
		candidateOrder = append(candidateOrder, characterID)
		refreshedHandle, err := service.persistRoster(runtime, candidateByID, candidateOrder)
		if err != nil {
			state = masterSnapshot(runtime)
			addErr = fmt.Errorf("could not save player config: %w", err)
			return transition{}
		}
		if runtime.ActivePlayerConfig != nil {
			value := refreshedHandle
			runtime.ActivePlayerConfig = &value
		}
		runtime.RosterByID[characterID] = &domain.CharacterRosterEntry{
			ID:                  characterID,
			Name:                name,
			Intelligence:        payload.Intelligence,
			HackerPerkAvailable: payload.HackerPerkAvailable,
		}
		runtime.RosterOrder = append(runtime.RosterOrder, characterID)
		state = masterSnapshot(runtime)
		return transition{accepted: true, effects: stateEffects(runtime)}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), addErr
}

// UpdateCharacter replaces one complete roster profile while retaining its
// stable identity and position. No-op replacements do not write, publish, or
// advance the coordination revision.
func (service *Service) UpdateCharacter(payload domain.CharacterUpdatePayload) (*domain.MasterCoordinationState, error) {
	var state *domain.MasterCoordinationState
	var updateErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		if payload.ExpectedRevision != runtime.Revision {
			state = masterSnapshot(runtime)
			updateErr = fmt.Errorf(
				"coordination revision is stale: expected %d, current %d",
				payload.ExpectedRevision,
				runtime.Revision,
			)
			return transition{}
		}
		if service.requirePlayerConfig && runtime.ActivePlayerConfig == nil {
			state = masterSnapshot(runtime)
			updateErr = fmt.Errorf("select or create a player config first")
			return transition{}
		}
		if runtime.Broadcast != nil {
			state = masterSnapshot(runtime)
			updateErr = fmt.Errorf("player roster cannot change during a broadcast")
			return transition{}
		}
		character := runtime.RosterByID[payload.CharacterID]
		if character == nil {
			state = masterSnapshot(runtime)
			updateErr = fmt.Errorf("character %q does not exist", payload.CharacterID)
			return transition{}
		}
		name, err := domain.ValidateCharacterName(payload.Name)
		if err != nil {
			state = masterSnapshot(runtime)
			updateErr = err
			return transition{}
		}
		if err := domain.ValidateCharacterIntelligence(payload.Intelligence); err != nil {
			state = masterSnapshot(runtime)
			updateErr = err
			return transition{}
		}
		if character.Name == name &&
			character.Intelligence == payload.Intelligence &&
			character.HackerPerkAvailable == payload.HackerPerkAvailable {
			state = masterSnapshot(runtime)
			return transition{}
		}
		candidateByID, candidateOrder := cloneRosterState(runtime)
		candidateByID[payload.CharacterID] = &domain.CharacterRosterEntry{
			ID:                  payload.CharacterID,
			Name:                name,
			Intelligence:        payload.Intelligence,
			HackerPerkAvailable: payload.HackerPerkAvailable,
		}
		refreshedHandle, err := service.persistRoster(runtime, candidateByID, candidateOrder)
		if err != nil {
			state = masterSnapshot(runtime)
			updateErr = fmt.Errorf("could not save player config: %w", err)
			return transition{}
		}
		if runtime.ActivePlayerConfig != nil {
			value := refreshedHandle
			runtime.ActivePlayerConfig = &value
		}
		character.Name = name
		character.Intelligence = payload.Intelligence
		character.HackerPerkAvailable = payload.HackerPerkAvailable
		state = masterSnapshot(runtime)
		return transition{accepted: true, effects: stateEffects(runtime)}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), updateErr
}

// DeleteCharacter removes an unclaimed roster entry while preserving the
// stable order of every survivor.
func (service *Service) DeleteCharacter(payload domain.CharacterDeletePayload) (*domain.MasterCoordinationState, error) {
	var state *domain.MasterCoordinationState
	var deleteErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		if payload.ExpectedRevision != runtime.Revision {
			state = masterSnapshot(runtime)
			deleteErr = fmt.Errorf(
				"coordination revision is stale: expected %d, current %d",
				payload.ExpectedRevision,
				runtime.Revision,
			)
			return transition{}
		}
		if service.requirePlayerConfig && runtime.ActivePlayerConfig == nil {
			state = masterSnapshot(runtime)
			deleteErr = fmt.Errorf("select or create a player config first")
			return transition{}
		}
		if runtime.Broadcast != nil {
			state = masterSnapshot(runtime)
			deleteErr = fmt.Errorf("player roster cannot change during a broadcast")
			return transition{}
		}
		if runtime.RosterByID[payload.CharacterID] == nil {
			state = masterSnapshot(runtime)
			deleteErr = fmt.Errorf("character %q does not exist", payload.CharacterID)
			return transition{}
		}
		candidateByID, candidateOrder := cloneRosterState(runtime)
		delete(candidateByID, payload.CharacterID)
		filtered := candidateOrder[:0]
		for _, candidate := range candidateOrder {
			if candidate != payload.CharacterID {
				filtered = append(filtered, candidate)
			}
		}
		candidateOrder = filtered
		refreshedHandle, err := service.persistRoster(runtime, candidateByID, candidateOrder)
		if err != nil {
			state = masterSnapshot(runtime)
			deleteErr = fmt.Errorf("could not save player config: %w", err)
			return transition{}
		}
		if runtime.ActivePlayerConfig != nil {
			value := refreshedHandle
			runtime.ActivePlayerConfig = &value
		}
		delete(runtime.RosterByID, payload.CharacterID)
		order := runtime.RosterOrder[:0]
		for _, candidate := range runtime.RosterOrder {
			if candidate != payload.CharacterID {
				order = append(order, candidate)
			}
		}
		runtime.RosterOrder = order
		state = masterSnapshot(runtime)
		return transition{accepted: true, effects: stateEffects(runtime)}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), deleteErr
}

// RenameLogicalSession changes only the process-local technical label. The
// trimmed label remains unique across all recognized sessions.
func (service *Service) RenameLogicalSession(sessionID domain.LogicalSessionID, fallbackName string) (*domain.MasterCoordinationState, error) {
	fallbackName, err := validatedCoordinationName(fallbackName, "session fallback name")
	if err != nil {
		return service.Snapshot(), err
	}

	var state *domain.MasterCoordinationState
	var renameErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		session := runtime.SessionsByID[sessionID]
		if session == nil {
			state = masterSnapshot(runtime)
			renameErr = fmt.Errorf("logical session %q does not exist", sessionID)
			return transition{}
		}
		if session.FallbackName == fallbackName {
			state = masterSnapshot(runtime)
			return transition{}
		}
		for candidateID, candidate := range runtime.SessionsByID {
			if candidateID != sessionID && candidate != nil && candidate.FallbackName == fallbackName {
				state = masterSnapshot(runtime)
				renameErr = fmt.Errorf("session fallback name %q is already in use", fallbackName)
				return transition{}
			}
		}
		session.FallbackName = fallbackName
		state = masterSnapshot(runtime)
		return transition{accepted: true, effects: stateEffects(runtime)}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), renameErr
}

// AssignCharacter installs one available claim for an unassigned session. A
// connected first assignee establishes initial control when none exists.
func (service *Service) AssignCharacter(sessionID domain.LogicalSessionID, characterID domain.CharacterID) (*domain.MasterCoordinationState, error) {
	var state *domain.MasterCoordinationState
	var assignErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		broadcast := runtime.Broadcast
		session := runtime.SessionsByID[sessionID]
		switch {
		case broadcast == nil:
			assignErr = fmt.Errorf("no broadcast is active")
		case session == nil:
			assignErr = fmt.Errorf("logical session %q does not exist", sessionID)
		case runtime.RosterByID[characterID] == nil:
			assignErr = fmt.Errorf("character %q does not exist", characterID)
		case sessionAssigned(broadcast, sessionID):
			assignErr = fmt.Errorf("logical session %q already has a character", sessionID)
		case characterClaimed(broadcast, characterID):
			assignErr = fmt.Errorf("character %q is currently claimed", characterID)
		}
		if assignErr != nil {
			state = masterSnapshot(runtime)
			return transition{}
		}
		ensureAssignmentIndexes(broadcast)
		broadcast.AssignmentsBySession[sessionID] = characterID
		broadcast.SessionByCharacter[characterID] = sessionID
		if broadcast.ControllerSessionID == nil && len(session.ConnectionIDs) > 0 {
			controller := sessionID
			broadcast.ControllerSessionID = &controller
		}
		state = masterSnapshot(runtime)
		effects := stateEffects(runtime)
		effects = service.appendCurrentTerminalEffect(runtime, effects, sessionID)
		return transition{accepted: true, effects: effects}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), assignErr
}

// ReleaseCharacter removes one current claim. Releasing the controller clears
// control without electing any observer.
func (service *Service) ReleaseCharacter(sessionID domain.LogicalSessionID) (*domain.MasterCoordinationState, error) {
	var state *domain.MasterCoordinationState
	var releaseErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		broadcast := runtime.Broadcast
		if broadcast == nil {
			releaseErr = fmt.Errorf("no broadcast is active")
			state = masterSnapshot(runtime)
			return transition{}
		}
		if runtime.SessionsByID[sessionID] == nil {
			releaseErr = fmt.Errorf("logical session %q does not exist", sessionID)
			state = masterSnapshot(runtime)
			return transition{}
		}
		characterID, assigned := broadcast.AssignmentsBySession[sessionID]
		if !assigned {
			releaseErr = fmt.Errorf("logical session %q has no character", sessionID)
			state = masterSnapshot(runtime)
			return transition{}
		}
		delete(broadcast.AssignmentsBySession, sessionID)
		delete(broadcast.SessionByCharacter, characterID)
		clearControllerIfSession(broadcast, sessionID)
		state = masterSnapshot(runtime)
		return transition{accepted: true, effects: stateEffects(runtime)}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), releaseErr
}

// MoveCharacter transfers one roster identity from its current owner, when
// any, to an unassigned destination. Control is cleared when the old owner was
// active and is never transferred implicitly.
func (service *Service) MoveCharacter(characterID domain.CharacterID, toSessionID domain.LogicalSessionID) (*domain.MasterCoordinationState, error) {
	var state *domain.MasterCoordinationState
	var moveErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		broadcast := runtime.Broadcast
		switch {
		case broadcast == nil:
			moveErr = fmt.Errorf("no broadcast is active")
		case runtime.RosterByID[characterID] == nil:
			moveErr = fmt.Errorf("character %q does not exist", characterID)
		case runtime.SessionsByID[toSessionID] == nil:
			moveErr = fmt.Errorf("logical session %q does not exist", toSessionID)
		case sessionAssigned(broadcast, toSessionID):
			moveErr = fmt.Errorf("logical session %q already has a character", toSessionID)
		}
		if moveErr != nil {
			state = masterSnapshot(runtime)
			return transition{}
		}

		ensureAssignmentIndexes(broadcast)
		if fromSessionID, claimed := characterOwner(broadcast, characterID); claimed {
			delete(broadcast.AssignmentsBySession, fromSessionID)
			clearControllerIfSession(broadcast, fromSessionID)
		}
		broadcast.AssignmentsBySession[toSessionID] = characterID
		broadcast.SessionByCharacter[characterID] = toSessionID
		state = masterSnapshot(runtime)
		return transition{accepted: true, effects: stateEffects(runtime)}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), moveErr
}

// SetActiveController atomically replaces controller identity with one
// connected, character-assigned logical session. It shares commit ordering
// with player actions and changes no claim or terminal runtime field.
func (service *Service) SetActiveController(sessionID domain.LogicalSessionID) (*domain.MasterCoordinationState, error) {
	var state *domain.MasterCoordinationState
	var controllerErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		broadcast := runtime.Broadcast
		session := runtime.SessionsByID[sessionID]
		switch {
		case broadcast == nil:
			controllerErr = fmt.Errorf("no broadcast is active")
		case session == nil:
			controllerErr = fmt.Errorf("logical session %q does not exist", sessionID)
		case !sessionAssigned(broadcast, sessionID):
			controllerErr = fmt.Errorf("logical session %q has no character", sessionID)
		case len(session.ConnectionIDs) == 0:
			controllerErr = fmt.Errorf("logical session %q is disconnected", sessionID)
		}
		if controllerErr != nil {
			state = masterSnapshot(runtime)
			return transition{}
		}
		if broadcast.ControllerSessionID != nil && *broadcast.ControllerSessionID == sessionID {
			state = masterSnapshot(runtime)
			return transition{}
		}
		controller := sessionID
		broadcast.ControllerSessionID = &controller
		state = masterSnapshot(runtime)
		return transition{accepted: true, effects: stateEffects(runtime)}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), controllerErr
}

// RequestTerminalActivation directly activates a new or retained checkpoint
// when the current source has no unfinished puzzle. Unfinished-puzzle switch
// decisions are introduced by the later terminal-decision slice.
func (service *Service) RequestTerminalActivation(target domain.TerminalTarget) (*domain.MasterCoordinationState, error) {
	if strings.TrimSpace(target.TerminalID) == "" {
		return service.Snapshot(), fmt.Errorf("terminal ID must not be blank")
	}

	var state *domain.MasterCoordinationState
	var activationErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		broadcast := runtime.Broadcast
		if broadcast == nil {
			state = masterSnapshot(runtime)
			activationErr = fmt.Errorf("no broadcast is active")
			return transition{}
		}
		if service.terminals == nil {
			state = masterSnapshot(runtime)
			activationErr = fmt.Errorf("terminal runtime lifecycle is unavailable")
			return transition{}
		}
		clearCommandExecutionRuntime(runtime)
		clearTerminalNavigationRuntime(runtime)
		if source := activeTerminalRuntime(broadcast); unfinishedTerminalRuntime(source) && source.TerminalID != target.TerminalID {
			runtime.PendingSwitch = &domain.TerminalSwitchDecision{
				ID: domain.SwitchID(service.nextID()), BroadcastID: broadcast.ID,
				SourceTerminalID: source.TerminalID, Target: cloneTerminalTarget(&target),
			}
			state = masterSnapshot(runtime)
			return transition{accepted: true, effects: stateEffects(runtime)}
		}

		ensureTerminalRuntimeSlots(broadcast)
		targetRuntime := broadcast.TerminalRuntimes[target.TerminalID]
		var projection *domain.PublicLiveState
		if targetRuntime == nil {
			targetRuntime, projection = service.terminals.CreateRuntime(target)
			if targetRuntime == nil || projection == nil || targetRuntime.TerminalID != target.TerminalID {
				state = masterSnapshot(runtime)
				activationErr = fmt.Errorf("terminal runtime could not be created")
				return transition{}
			}
			broadcast.TerminalRuntimes[target.TerminalID] = targetRuntime
		} else if targetRuntime.Lifecycle == domain.TerminalLifecycleSuspended {
			if lifecycle, ok := service.terminals.(terminalDecisionLifecycle); ok {
				projection = lifecycle.ReactivateRuntime(targetRuntime, target)
			} else {
				projection = service.terminals.UpdateRuntime(targetRuntime, target)
				targetRuntime.Lifecycle = domain.TerminalLifecycleActive
			}
		} else {
			projection = service.terminals.UpdateRuntime(targetRuntime, target)
			if projection == nil {
				state = masterSnapshot(runtime)
				activationErr = fmt.Errorf("terminal runtime could not be updated")
				return transition{}
			}
		}

		if source := activeTerminalRuntime(broadcast); source != nil && source != targetRuntime {
			source.Lifecycle = domain.TerminalLifecycleSuspended
		}
		targetRuntime.Lifecycle = domain.TerminalLifecycleActive
		activeTerminalID := target.TerminalID
		broadcast.ActiveTerminalID = &activeTerminalID
		service.establishInitialTerminalRoute(broadcast, target.TerminalID)
		runtime.PendingSwitch = nil
		state = masterSnapshot(runtime)
		effects := stateEffects(runtime)
		effects = append(effects, Effect{Live: projection})
		return transition{accepted: true, effects: effects}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), activationErr
}

// RequestTerminalClear keeps the broadcast, identities, claims, and controller
// while suspending a directly clearable active checkpoint and publishing a
// canonical terminal clear at the same revision.
func (service *Service) RequestTerminalClear() (*domain.MasterCoordinationState, error) {
	var state *domain.MasterCoordinationState
	var clearErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		broadcast := runtime.Broadcast
		if broadcast == nil {
			state = masterSnapshot(runtime)
			clearErr = fmt.Errorf("no broadcast is active")
			return transition{}
		}
		source := activeTerminalRuntime(broadcast)
		clearCommandExecutionRuntime(runtime)
		navigationCleared := clearTerminalNavigationRuntime(runtime)
		if unfinishedTerminalRuntime(source) {
			runtime.PendingSwitch = &domain.TerminalSwitchDecision{
				ID: domain.SwitchID(service.nextID()), BroadcastID: broadcast.ID,
				SourceTerminalID: source.TerminalID,
			}
			state = masterSnapshot(runtime)
			return transition{accepted: true, effects: stateEffects(runtime)}
		}
		if source == nil && broadcast.ActiveTerminalID == nil {
			state = masterSnapshot(runtime)
			if navigationCleared {
				return transition{accepted: true, effects: stateEffects(runtime)}
			}
			return transition{}
		}
		if source != nil {
			source.Lifecycle = domain.TerminalLifecycleSuspended
		}
		broadcast.ActiveTerminalID = nil
		runtime.PendingSwitch = nil
		state = masterSnapshot(runtime)
		effects := stateEffects(runtime)
		effects = append(effects, Effect{ClearLiveTerminal: true})
		return transition{accepted: true, effects: effects}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), clearErr
}

// ResolveTerminalSwitch applies one explicit unfinished-puzzle decision
// against the still-current source and broadcast. The opaque switch ID is the
// only authority to resolve a pending request.
func (service *Service) ResolveTerminalSwitch(switchID domain.SwitchID, choice domain.TerminalSwitchChoice) (*domain.MasterCoordinationState, error) {
	if switchID == "" {
		return service.Snapshot(), fmt.Errorf("switch ID must not be blank")
	}
	if choice != domain.TerminalSwitchPreserve && choice != domain.TerminalSwitchDiscard && choice != domain.TerminalSwitchCancel {
		return service.Snapshot(), fmt.Errorf("terminal switch decision must be preserve, discard, or cancel")
	}

	var state *domain.MasterCoordinationState
	var resolveErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		pending := runtime.PendingSwitch
		broadcast := runtime.Broadcast
		if pending == nil || pending.ID != switchID || broadcast == nil || pending.BroadcastID != broadcast.ID || broadcast.ActiveTerminalID == nil || *broadcast.ActiveTerminalID != pending.SourceTerminalID {
			state = masterSnapshot(runtime)
			resolveErr = fmt.Errorf("terminal switch decision is stale")
			return transition{}
		}
		if choice == domain.TerminalSwitchCancel {
			runtime.PendingSwitch = nil
			state = masterSnapshot(runtime)
			return transition{accepted: true, effects: stateEffects(runtime)}
		}
		lifecycle, ok := service.terminals.(terminalDecisionLifecycle)
		if !ok {
			state = masterSnapshot(runtime)
			resolveErr = fmt.Errorf("terminal decision lifecycle is unavailable")
			return transition{}
		}
		source := activeTerminalRuntime(broadcast)
		if source == nil || source.TerminalID != pending.SourceTerminalID {
			state = masterSnapshot(runtime)
			resolveErr = fmt.Errorf("terminal switch decision is stale")
			return transition{}
		}

		if choice == domain.TerminalSwitchPreserve {
			lifecycle.SuspendRuntime(source)
		} else {
			delete(broadcast.TerminalRuntimes, source.TerminalID)
		}

		var projection *domain.PublicLiveState
		clear := pending.Target == nil
		if clear {
			broadcast.ActiveTerminalID = nil
		} else {
			target := *cloneTerminalTarget(pending.Target)
			targetRuntime := broadcast.TerminalRuntimes[target.TerminalID]
			if targetRuntime == nil {
				targetRuntime, projection = service.terminals.CreateRuntime(target)
			} else {
				projection = lifecycle.ReactivateRuntime(targetRuntime, target)
			}
			if targetRuntime == nil || projection == nil {
				state = masterSnapshot(runtime)
				resolveErr = fmt.Errorf("terminal runtime could not be activated")
				return transition{}
			}
			targetRuntime.Lifecycle = domain.TerminalLifecycleActive
			broadcast.TerminalRuntimes[target.TerminalID] = targetRuntime
			targetID := target.TerminalID
			broadcast.ActiveTerminalID = &targetID
		}
		runtime.PendingSwitch = nil
		state = masterSnapshot(runtime)
		effects := stateEffects(runtime)
		if clear {
			effects = append(effects, Effect{ClearLiveTerminal: true})
		} else {
			effects = append(effects, Effect{Live: projection})
		}
		return transition{accepted: true, effects: effects}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), resolveErr
}

// ResolveCommandExecution applies one exact private decision to the single
// pending command. Approve holds the coordinator transaction
// across the one-way durable store call and publishes completed state only
// after that call succeeds.
func (service *Service) ResolveCommandExecution(ctx context.Context, requestID string, decision domain.CommandExecutionDecision) (*domain.MasterCoordinationState, *CommandStateMutation, error) {
	if ctx == nil {
		return service.Snapshot(), nil, fmt.Errorf("command execution context is required")
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return service.Snapshot(), nil, fmt.Errorf("command execution request ID must not be blank")
	}
	if decision != domain.CommandExecutionApprove && decision != domain.CommandExecutionReject {
		return service.Snapshot(), nil, fmt.Errorf("command execution decision must be approve or reject")
	}

	var state *domain.MasterCoordinationState
	var mutation *CommandStateMutation
	var resolveErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		pending := runtime.PendingCommandExecution
		broadcast := runtime.Broadcast
		terminal := activeTerminalRuntime(broadcast)
		if !currentPendingCommandExecution(pending, broadcast, terminal, requestID) {
			state = masterSnapshot(runtime)
			resolveErr = ErrCommandExecutionStale
			return transition{effects: []Effect{{Audit: []AuditEvent{{
				Name: "command.decision", Decision: string(decision), Outcome: "stale", RequestID: requestID,
			}}}}}
		}
		authored, current := selectedAuthoredCommand(terminal, domain.RuntimeCommand{
			Kind: domain.RuntimeCommandNavigate, Action: "command", NodeID: pending.CommandID,
		})
		if !current || authored.Behavior() == domain.CommandBehaviorInvalid ||
			authored.Behavior() == domain.CommandBehaviorTerminalTransition ||
			displayedCommandName(terminal, authored) != pending.CommandName ||
			commandApprovalMode(terminal, authored) != pending.Mode ||
			commandConfirmationText(authored) != pending.ConfirmationText {
			state = masterSnapshot(runtime)
			resolveErr = ErrCommandExecutionStale
			return transition{effects: []Effect{{Audit: []AuditEvent{
				commandDecisionAudit(pending, decision, "stale"),
			}}}}
		}

		if decision == domain.CommandExecutionReject {
			runtime.PendingCommandExecution = nil
			terminal.CommandExecution = &domain.CommandExecutionPresentation{
				Phase: domain.CommandExecutionPhaseRejected, CommandID: pending.CommandID,
			}
			state = masterSnapshot(runtime)
			effects := stateEffects(runtime)
			if projection := service.projectActiveTerminal(runtime); projection != nil {
				effects = append(effects, Effect{Live: projection})
			}
			change := transition{accepted: true, effects: effects}
			change.audit(commandDecisionAudit(pending, decision, "declined"))
			return change
		}

		if pending.Mode == domain.CommandApprovalModeOrdinary || pending.Mode == domain.CommandApprovalModeCompletedStateChange {
			terminal.CommandExecution = nil
			terminal.Nav = nav.ApplyAction(terminal.Nav, terminal.Tree, "command", pending.CommandID)
			runtime.PendingCommandExecution = nil
			state = masterSnapshot(runtime)
			effects := stateEffects(runtime)
			if projection := service.projectActiveTerminal(runtime); projection != nil {
				effects = append(effects, Effect{Live: projection})
			}
			change := transition{accepted: true, effects: effects}
			change.audit(commandDecisionAudit(pending, decision, "succeeded"))
			return change
		}

		if service.commandStateStore == nil {
			resolveErr = ErrCommandExecutionPersistence
			return service.failPendingCommandExecution(runtime, terminal, pending, &state, decision)
		}
		durable, err := service.commandStateStore.ExecuteCommandState(ctx, pending.TerminalID, pending.CommandID)
		if err != nil {
			resolveErr = ErrCommandExecutionPersistence
			return service.failPendingCommandExecution(runtime, terminal, pending, &state, decision)
		}
		if err := domain.ValidateSession(durable.Session); err != nil {
			resolveErr = commandExecutionPersistenceFailure("command execution returned an invalid durable state")
			return service.failPendingCommandExecution(runtime, terminal, pending, &state, decision)
		}
		durableTerminal := terminalByStableID(&durable.Session, pending.TerminalID)
		if durableTerminal == nil {
			resolveErr = commandExecutionPersistenceFailure("command execution returned an invalid durable state")
			return service.failPendingCommandExecution(runtime, terminal, pending, &state, decision)
		}
		completed, exists := durableTerminal.CommandStates[pending.CommandID]
		if !exists || !durableCommandStateMatchesAuthored(completed, authored) {
			resolveErr = commandExecutionPersistenceFailure("command execution returned an invalid durable state")
			return service.failPendingCommandExecution(runtime, terminal, pending, &state, decision)
		}

		terminal.CommandStates = cloneCommandStates(durableTerminal.CommandStates)
		terminal.CommandExecution = nil
		commandID := pending.CommandID
		terminal.Nav.CommandNodeID = &commandID
		runtime.PendingCommandExecution = nil
		value := durable
		mutation = &value
		state = masterSnapshot(runtime)
		effects := stateEffects(runtime)
		if projection := service.projectActiveTerminal(runtime); projection != nil {
			effects = append(effects, Effect{Live: projection})
		}
		change := transition{accepted: true, effects: effects}
		change.audit(commandDecisionAudit(pending, decision, "succeeded"))
		return change
	})
	if state == nil {
		state = service.Snapshot()
	}
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), mutation, resolveErr
}

// ResetCommandState removes one complete durable command snapshot and installs
// the canonical terminal state before publishing the accepted coordination revision.
func (service *Service) ResetCommandState(ctx context.Context, terminalID, commandID string) (*domain.MasterCoordinationState, *CommandStateMutation, error) {
	terminalID = strings.TrimSpace(terminalID)
	commandID = strings.TrimSpace(commandID)
	if terminalID == "" {
		return service.Snapshot(), nil, fmt.Errorf("terminal ID must not be blank")
	}
	if commandID == "" {
		return service.Snapshot(), nil, fmt.Errorf("command ID must not be blank")
	}
	return service.resetCommandStates(ctx, terminalID, commandID, false)
}

// ResetTerminalCommandStates removes every complete durable command snapshot
// owned by one terminal and publishes the canonical result as one revision.
func (service *Service) ResetTerminalCommandStates(ctx context.Context, terminalID string) (*domain.MasterCoordinationState, *CommandStateMutation, error) {
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return service.Snapshot(), nil, fmt.Errorf("terminal ID must not be blank")
	}
	return service.resetCommandStates(ctx, terminalID, "", true)
}

func (service *Service) resetCommandStates(
	ctx context.Context,
	terminalID, commandID string,
	terminalWide bool,
) (*domain.MasterCoordinationState, *CommandStateMutation, error) {
	if ctx == nil {
		return service.Snapshot(), nil, fmt.Errorf("command state reset context is required")
	}

	var state *domain.MasterCoordinationState
	var mutation *CommandStateMutation
	var resetErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		state = masterSnapshot(runtime)
		if ctx.Err() != nil {
			resetErr = fmt.Errorf("command state reset was canceled")
			return transition{}
		}
		if service.commandStateStore == nil {
			resetErr = ErrCommandStateStorageUnavailable
			return transition{}
		}

		var durable CommandStateMutation
		var err error
		if terminalWide {
			durable, err = service.commandStateStore.ResetTerminalCommandStates(ctx, terminalID)
		} else {
			durable, err = service.commandStateStore.ResetCommandState(ctx, terminalID, commandID)
		}
		if err != nil {
			resetErr = fmt.Errorf("command state could not be reset")
			return transition{}
		}
		durableTerminal, err := validatedCommandStateReset(durable.Session, terminalID, commandID, terminalWide)
		if err != nil {
			resetErr = err
			return transition{}
		}
		canonical := CommandStateMutation{
			Changed: durable.Changed, Revision: durable.Revision, Session: domain.CloneSession(durable.Session),
		}
		mutation = &canonical
		if !durable.Changed {
			return transition{}
		}

		var projection *domain.PublicLiveState
		if broadcast := runtime.Broadcast; broadcast != nil {
			terminal := broadcast.TerminalRuntimes[terminalID]
			if terminal != nil {
				clearResetCommandPresentation(runtime, terminal, commandID, terminalWide)
				terminal.CommandStates = cloneCommandStates(durableTerminal.CommandStates)
				if broadcast.ActiveTerminalID != nil && *broadcast.ActiveTerminalID == terminalID && service.terminals != nil {
					projection = service.terminals.ProjectRuntime(terminal)
				}
			}
		}
		state = masterSnapshot(runtime)
		effects := stateEffects(runtime)
		if projection != nil {
			effects = append(effects, Effect{Live: projection})
		}
		return transition{accepted: true, effects: effects}
	})
	if state == nil {
		state = service.Snapshot()
	}
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), mutation, resetErr
}

func validatedCommandStateReset(
	session domain.Session,
	terminalID, commandID string,
	terminalWide bool,
) (*domain.Terminal, error) {
	if err := domain.ValidateSession(session); err != nil {
		return nil, fmt.Errorf("command state reset returned an invalid durable state")
	}
	terminal := terminalByStableID(&session, terminalID)
	if terminal == nil {
		return nil, fmt.Errorf("command state reset returned an invalid durable state")
	}
	if terminalWide {
		if len(terminal.CommandStates) != 0 {
			return nil, fmt.Errorf("command state reset returned an invalid durable state")
		}
	} else if _, exists := terminal.CommandStates[commandID]; exists {
		return nil, fmt.Errorf("command state reset returned an invalid durable state")
	}
	return terminal, nil
}

func clearResetCommandPresentation(
	runtime *domain.ProcessRuntime,
	terminal *domain.TerminalRuntime,
	commandID string,
	terminalWide bool,
) {
	affected := func(candidate string) bool {
		if terminalWide {
			_, completed := terminal.CommandStates[candidate]
			return completed
		}
		return candidate == commandID
	}
	if terminal.Nav.CommandNodeID != nil && affected(*terminal.Nav.CommandNodeID) {
		terminal.Nav.CommandNodeID = nil
	}
	if terminal.CommandExecution != nil && affected(terminal.CommandExecution.CommandID) {
		terminal.CommandExecution = nil
	}
	if pending := runtime.PendingCommandExecution; pending != nil &&
		pending.TerminalID == terminal.TerminalID && affected(pending.CommandID) {
		runtime.PendingCommandExecution = nil
	}
}

// ResolveTerminalNavigation applies one exact Overseer decision without
// entering the manual terminal-switch lifecycle.
func (service *Service) ResolveTerminalNavigation(requestID string, decision domain.TerminalNavigationDecision) (*domain.MasterCoordinationState, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return service.Snapshot(), fmt.Errorf("terminal navigation request ID must not be blank")
	}
	if decision != domain.TerminalNavigationApprove && decision != domain.TerminalNavigationReject {
		return service.Snapshot(), fmt.Errorf("terminal navigation decision must be approve or reject")
	}

	var state *domain.MasterCoordinationState
	var resolveErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		pending := runtime.PendingTerminalNavigation
		broadcast := runtime.Broadcast
		source := activeTerminalRuntime(broadcast)
		if pending == nil || pending.RequestID != requestID || broadcast == nil || pending.BroadcastID != broadcast.ID ||
			source == nil || source.TerminalID != pending.SourceTerminalID {
			state = masterSnapshot(runtime)
			resolveErr = fmt.Errorf("terminal navigation decision is stale")
			return transition{}
		}
		if decision == domain.TerminalNavigationReject {
			runtime.PendingTerminalNavigation = nil
			runtime.TerminalNavigationNotice = nil
			if pending.Direction == domain.TerminalNavigationForward {
				source.CommandExecution = &domain.CommandExecutionPresentation{
					Phase: domain.CommandExecutionPhaseRejected, CommandID: pending.CommandID,
				}
			}
			state = masterSnapshot(runtime)
			effects := stateEffects(runtime)
			if projection := service.projectActiveTerminal(runtime); projection != nil {
				effects = append(effects, Effect{Live: projection})
			}
			return transition{accepted: true, effects: effects}
		}
		if pending.Direction == domain.TerminalNavigationReturn {
			if service.terminalCatalog == nil || len(broadcast.Route) == 0 ||
				!sameTerminalReturnPoint(broadcast.Route[len(broadcast.Route)-1], pending.ReturnPoint) ||
				!service.validTerminalReturnPoint(pending.SourceTerminalID, pending.ReturnPoint) {
				state = masterSnapshot(runtime)
				resolveErr = fmt.Errorf("terminal navigation route changed")
				return transition{}
			}
			latest, ok := service.terminalCatalog.LookupTerminal(pending.TargetTerminalID)
			if !ok || latest.TerminalID != pending.ReturnPoint.TerminalID ||
				!service.sameTerminalGroup(pending.SourceTerminalID, pending.TargetTerminalID) {
				targetID := pending.TargetTerminalID
				runtime.PendingTerminalNavigation = nil
				runtime.TerminalNavigationNotice = &domain.TerminalNavigationNotice{
					Reason: domain.TerminalNavigationNoticeTargetMissing, SourceTerminalID: pending.SourceTerminalID,
					CommandID: pending.CommandID, TargetTerminalID: &targetID,
				}
				state = masterSnapshot(runtime)
				resolveErr = fmt.Errorf("terminal navigation return target is unavailable")
				effects := stateEffects(runtime)
				if projection := service.projectActiveTerminal(runtime); projection != nil {
					effects = append(effects, Effect{Live: projection})
				}
				return transition{accepted: true, effects: effects}
			}
			lifecycle, ok := service.terminals.(terminalDecisionLifecycle)
			if !ok {
				state = masterSnapshot(runtime)
				resolveErr = fmt.Errorf("terminal navigation lifecycle is unavailable")
				return transition{}
			}
			lifecycle.SuspendRuntime(source)
			targetRuntime := broadcast.TerminalRuntimes[latest.TerminalID]
			var projection *domain.PublicLiveState
			if targetRuntime == nil {
				targetRuntime, projection = lifecycle.CreateRuntime(latest)
				broadcast.TerminalRuntimes[latest.TerminalID] = targetRuntime
			} else {
				projection = lifecycle.ReactivateRuntime(targetRuntime, latest)
			}
			if targetRuntime == nil || projection == nil {
				state = masterSnapshot(runtime)
				resolveErr = fmt.Errorf("terminal navigation return target could not be activated")
				return transition{}
			}
			targetRuntime.Nav = nav.RestoreFolder(latest.Tree, pending.ReturnPoint.FolderID, pending.ReturnPoint.AncestorFolderIDs)
			projection.Nav = targetRuntime.Nav
			broadcast.Route = broadcast.Route[:len(broadcast.Route)-1]
			targetID := latest.TerminalID
			broadcast.ActiveTerminalID = &targetID
			runtime.PendingTerminalNavigation = nil
			runtime.TerminalNavigationNotice = nil
			projection = decorateTerminalNavigation(runtime, projection)
			state = masterSnapshot(runtime)
			return transition{accepted: true, effects: append(stateEffects(runtime), Effect{Live: projection})}
		}
		if pending.Direction != domain.TerminalNavigationForward || service.terminalCatalog == nil {
			state = masterSnapshot(runtime)
			resolveErr = fmt.Errorf("terminal navigation target is unavailable")
			return transition{}
		}
		latest, ok := service.terminalCatalog.LookupTerminalTransition(pending.SourceTerminalID, pending.CommandID)
		if !ok || latest.Target.TerminalID != pending.TargetTerminalID || latest.Target.TerminalID == pending.SourceTerminalID ||
			!service.sameTerminalGroup(pending.SourceTerminalID, pending.TargetTerminalID) {
			targetID := pending.TargetTerminalID
			runtime.PendingTerminalNavigation = nil
			runtime.TerminalNavigationNotice = &domain.TerminalNavigationNotice{
				Reason: domain.TerminalNavigationNoticeTargetChanged, SourceTerminalID: pending.SourceTerminalID,
				CommandID: pending.CommandID, TargetTerminalID: &targetID,
			}
			state = masterSnapshot(runtime)
			resolveErr = fmt.Errorf("terminal navigation target changed")
			effects := stateEffects(runtime)
			if projection := service.projectActiveTerminal(runtime); projection != nil {
				effects = append(effects, Effect{Live: projection})
			}
			return transition{accepted: true, effects: effects}
		}
		lifecycle, ok := service.terminals.(terminalDecisionLifecycle)
		if !ok {
			state = masterSnapshot(runtime)
			resolveErr = fmt.Errorf("terminal navigation lifecycle is unavailable")
			return transition{}
		}
		lifecycle.SuspendRuntime(source)
		targetRuntime := broadcast.TerminalRuntimes[latest.Target.TerminalID]
		var projection *domain.PublicLiveState
		if targetRuntime == nil {
			targetRuntime, projection = lifecycle.CreateRuntime(latest.Target)
			broadcast.TerminalRuntimes[latest.Target.TerminalID] = targetRuntime
		} else {
			projection = lifecycle.ReactivateRuntime(targetRuntime, latest.Target)
		}
		if targetRuntime == nil || projection == nil {
			state = masterSnapshot(runtime)
			resolveErr = fmt.Errorf("terminal navigation target could not be activated")
			return transition{}
		}
		targetRuntime.Nav = domain.NavState{Path: []string{"root"}, Mode: "list"}
		projection.Nav = targetRuntime.Nav
		broadcast.Route = append(broadcast.Route, pending.ReturnPoint)
		targetID := latest.Target.TerminalID
		broadcast.ActiveTerminalID = &targetID
		runtime.PendingTerminalNavigation = nil
		runtime.TerminalNavigationNotice = nil
		projection = decorateTerminalNavigation(runtime, projection)
		state = masterSnapshot(runtime)
		return transition{accepted: true, effects: append(stateEffects(runtime), Effect{Live: projection})}
	})
	if state == nil {
		state = service.Snapshot()
	}
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), resolveErr
}

func (service *Service) failPendingCommandExecution(
	runtime *domain.ProcessRuntime,
	terminal *domain.TerminalRuntime,
	pending *domain.PendingCommandExecution,
	state **domain.MasterCoordinationState,
	decision domain.CommandExecutionDecision,
) transition {
	runtime.PendingCommandExecution = nil
	terminal.CommandExecution = nil
	if session := runtime.SessionsByID[pending.ControllerSessionID]; session != nil {
		session.Notice = &domain.PlayerNotice{Kind: domain.PlayerNoticeCommandPersistenceFailed}
	}
	*state = masterSnapshot(runtime)
	effects := stateEffects(runtime)
	if projection := service.projectActiveTerminal(runtime); projection != nil {
		effects = append(effects, Effect{Live: projection})
	}
	change := transition{accepted: true, effects: effects}
	change.audit(commandDecisionAudit(pending, decision, "failed"))
	return change
}

// CanDeleteTerminal prevents durable deletion while a process-local runtime
// still owns active or preserved state for that authored terminal.
func (service *Service) CanDeleteTerminal(terminalID string) error {
	terminalID = strings.TrimSpace(terminalID)
	if terminalID == "" {
		return fmt.Errorf("terminal ID must not be blank")
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	if service.runtime.Broadcast != nil && service.runtime.Broadcast.TerminalRuntimes[terminalID] != nil {
		return fmt.Errorf("terminal %q has active or preserved runtime state", terminalID)
	}
	return nil
}

// UpdateLiveTerminal refreshes the active checkpoint's authored tree and
// optional introduction while preserving navigation validity and puzzle state.
func (service *Service) UpdateLiveTerminal(tree domain.ContentNode, introText *string) (*domain.MasterCoordinationState, error) {
	var state *domain.MasterCoordinationState
	var updateErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		broadcast := runtime.Broadcast
		active := activeTerminalRuntime(broadcast)
		if broadcast == nil || active == nil || broadcast.ActiveTerminalID == nil {
			state = masterSnapshot(runtime)
			updateErr = fmt.Errorf("no terminal is active")
			return transition{}
		}
		if service.terminals == nil {
			state = masterSnapshot(runtime)
			updateErr = fmt.Errorf("terminal runtime lifecycle is unavailable")
			return transition{}
		}
		intro := active.IntroText
		if introText != nil {
			intro = *introText
		}
		projection := service.terminals.UpdateRuntime(active, domain.TerminalTarget{
			TerminalID: active.TerminalID, TerminalName: active.TerminalName,
			Tree: tree, CommandStates: cloneCommandStates(active.CommandStates),
			HackLevel: active.HackLevel, IntroText: intro,
		})
		if projection == nil {
			state = masterSnapshot(runtime)
			updateErr = fmt.Errorf("active terminal could not be updated")
			return transition{}
		}
		state = masterSnapshot(runtime)
		effects := stateEffects(runtime)
		effects = append(effects, Effect{Live: projection})
		return transition{accepted: true, effects: effects}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), updateErr
}

// RefreshActiveTerminal installs one complete trusted backend target. Unlike
// authored live updates, this path intentionally replaces CommandStates and is
// used after a durable reset so the active projection cannot retain stale
// frontend or runtime snapshots.
func (service *Service) RefreshActiveTerminal(target domain.TerminalTarget) (*domain.MasterCoordinationState, error) {
	target.TerminalID = strings.TrimSpace(target.TerminalID)
	if target.TerminalID == "" {
		return service.Snapshot(), fmt.Errorf("terminal ID must not be blank")
	}
	var state *domain.MasterCoordinationState
	var refreshErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		broadcast := runtime.Broadcast
		active := activeTerminalRuntime(broadcast)
		if broadcast == nil || active == nil || broadcast.ActiveTerminalID == nil ||
			active.TerminalID != target.TerminalID {
			state = masterSnapshot(runtime)
			refreshErr = fmt.Errorf("target terminal is not active")
			return transition{}
		}
		if service.terminals == nil {
			state = masterSnapshot(runtime)
			refreshErr = fmt.Errorf("terminal runtime lifecycle is unavailable")
			return transition{}
		}
		// A durable reset removes the frozen snapshot that made the current
		// command result view valid. Clear only that stale completed view before
		// applying the canonical target; ordinary command results and completed
		// commands whose snapshots remain are preserved.
		if active.Nav.CommandNodeID != nil {
			commandID := *active.Nav.CommandNodeID
			_, wasCompleted := active.CommandStates[commandID]
			_, remainsCompleted := target.CommandStates[commandID]
			if wasCompleted && !remainsCompleted {
				active.Nav.CommandNodeID = nil
			}
		}
		projection := service.terminals.UpdateRuntime(active, target)
		if projection == nil {
			state = masterSnapshot(runtime)
			refreshErr = fmt.Errorf("active terminal could not be refreshed")
			return transition{}
		}
		state = masterSnapshot(runtime)
		effects := stateEffects(runtime)
		effects = append(effects, Effect{Live: projection})
		return transition{accepted: true, effects: effects}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), refreshErr
}

// ResetFailedHack atomically replaces only the failed puzzle checkpoint of
// the current active terminal. The latest validated authored target comes
// from the trusted desktop boundary; every unrelated coordinator field and
// runtime slot remains on the transaction's private clone unchanged.
func (service *Service) ResetFailedHack(target domain.TerminalTarget) (*domain.MasterCoordinationState, error) {
	target.TerminalID = strings.TrimSpace(target.TerminalID)
	if target.TerminalID == "" {
		return service.Snapshot(), fmt.Errorf("terminal ID must not be blank")
	}

	var state *domain.MasterCoordinationState
	var resetErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		broadcast := runtime.Broadcast
		active := activeTerminalRuntime(broadcast)
		if broadcast == nil || broadcast.ActiveTerminalID == nil || active == nil {
			state = masterSnapshot(runtime)
			resetErr = fmt.Errorf("no terminal is active")
			return transition{}
		}
		if *broadcast.ActiveTerminalID != target.TerminalID || active.TerminalID != target.TerminalID {
			state = masterSnapshot(runtime)
			resetErr = fmt.Errorf("failed hacking puzzle reset is stale")
			return transition{}
		}
		if active.Lifecycle != domain.TerminalLifecycleActive || active.Hack == nil || !active.Hack.Failed || active.Hack.Solved {
			state = masterSnapshot(runtime)
			resetErr = fmt.Errorf("active hacking puzzle is not failed")
			return transition{}
		}
		lifecycle, ok := service.terminals.(failedHackLifecycle)
		if !ok {
			state = masterSnapshot(runtime)
			resetErr = fmt.Errorf("failed hacking puzzle lifecycle is unavailable")
			return transition{}
		}

		replacement, projection := lifecycle.ResetFailedHack(active, target)
		if replacement == nil || projection == nil || replacement.TerminalID != target.TerminalID || replacement.Hack == nil || replacement.Hack.Failed || replacement.Hack.Solved {
			state = masterSnapshot(runtime)
			resetErr = fmt.Errorf("failed hacking puzzle could not be reset")
			return transition{}
		}
		replacement.Lifecycle = domain.TerminalLifecycleActive
		broadcast.TerminalRuntimes[target.TerminalID] = replacement
		state = masterSnapshot(runtime)
		effects := stateEffects(runtime)
		effects = append(effects, Effect{Live: projection})
		return transition{accepted: true, effects: effects}
	})
	if state == nil {
		state = service.Snapshot()
	}
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), resetErr
}

// StartBroadcast creates a fresh assignment epoch while retaining recognized
// sessions, fallback names, and the process-local roster.
func (service *Service) StartBroadcast() (*domain.MasterCoordinationState, error) {
	var state *domain.MasterCoordinationState
	var startErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		if service.requirePlayerConfig && runtime.ActivePlayerConfig == nil {
			state = masterSnapshot(runtime)
			startErr = fmt.Errorf("select or create a player config first")
			return transition{}
		}
		if runtime.Broadcast != nil {
			state = masterSnapshot(runtime)
			startErr = fmt.Errorf("a broadcast is already active")
			return transition{}
		}
		runtime.Broadcast = &domain.LiveBroadcast{
			ID:                   domain.BroadcastID(service.nextID()),
			AssignmentsBySession: make(map[domain.LogicalSessionID]domain.CharacterID),
			SessionByCharacter:   make(map[domain.CharacterID]domain.LogicalSessionID),
			TerminalRuntimes:     make(map[string]*domain.TerminalRuntime),
		}
		runtime.PendingSwitch = nil
		runtime.PendingTerminalNavigation = nil
		runtime.TerminalNavigationNotice = nil
		clearRequestResults(runtime)
		state = masterSnapshot(runtime)
		return transition{accepted: true, effects: stateEffects(runtime)}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), startErr
}

// EndBroadcast atomically drops the entire broadcast-scoped epoch while
// retaining process-local browser recognition, fallback names, presence, and
// roster identities. Every request cache is cleared for the next epoch.
func (service *Service) EndBroadcast() (*domain.MasterCoordinationState, error) {
	var state *domain.MasterCoordinationState
	var endErr error
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		if runtime.Broadcast == nil {
			state = masterSnapshot(runtime)
			endErr = fmt.Errorf("no broadcast is active")
			return transition{}
		}
		runtime.Broadcast = nil
		runtime.PendingSwitch = nil
		runtime.PendingCommandExecution = nil
		runtime.PendingTerminalNavigation = nil
		runtime.TerminalNavigationNotice = nil
		clearRequestResults(runtime)
		state = masterSnapshot(runtime)
		effects := stateEffects(runtime)
		effects = append(effects, Effect{ClearLiveTerminal: true})
		return transition{accepted: true, effects: effects}
	})
	state.Revision = result.revision
	return domain.CloneMasterCoordinationState(state), endErr
}

// Shutdown discards the complete process-local aggregate. It intentionally
// persists nothing and publishes nothing because transports are already being
// torn down by the application lifecycle.
func (service *Service) Shutdown() {
	if service == nil {
		return
	}
	service.mu.Lock()
	service.runtime = newProcessRuntime()
	service.mu.Unlock()
}

// CreateSession establishes one new logical session and attaches the supplied
// concrete connection. It is the compatibility form of AttachConnection for
// callers that do not yet hold a browser token.
func (service *Service) CreateSession(connectionID domain.ConnectionID) SessionIdentity {
	browserToken, state := service.AttachConnection(connectionID, "")
	return SessionIdentity{SessionID: state.SessionID, BrowserToken: browserToken, State: state}
}

// AttachConnection resolves a known browser token or creates a fresh logical
// session and replacement token. Additional tabs mutate only private
// membership; aggregate presence effects are emitted on the first connection.
func (service *Service) AttachConnection(connectionID domain.ConnectionID, browserToken domain.BrowserToken) (domain.BrowserToken, *domain.PlayerState) {
	var handle *domain.RecognitionHandle
	if browserToken != "" {
		value := domain.RecognitionHandle(browserToken)
		handle = &value
	}
	snapshot, err := service.AttachSubscription(connectionID, handle)
	if err != nil || snapshot == nil {
		return "", nil
	}
	return snapshot.RecognitionHandle, domain.ClonePlayerState(snapshot.PlayerState)
}

// AttachSubscription atomically resolves recognition, registers the physical
// stream, and captures the complete revision-R personalized snapshot. The
// active terminal projection is read under the same coordinator order and
// never creates or regenerates a puzzle.
func (service *Service) AttachSubscription(connectionID domain.ConnectionID, recognitionHandle *domain.RecognitionHandle) (*domain.PersonalizedSnapshot, error) {
	return service.AttachSubscriptionAndRegister(connectionID, recognitionHandle, nil)
}

// AttachSubscriptionAndRegister makes canonical attachment, complete snapshot
// capture, and transport registration one ordered boundary. register runs with
// the committed revision while the coordinator lock is still held and before
// any revision-R effects are published, so a later mutation can neither pass
// the snapshot nor publish before the physical stream is ready to receive it.
// The callback must not call back into Service.
func (service *Service) AttachSubscriptionAndRegister(connectionID domain.ConnectionID, recognitionHandle *domain.RecognitionHandle, register func(*domain.PersonalizedSnapshot)) (*domain.PersonalizedSnapshot, error) {
	if service == nil {
		return nil, fmt.Errorf("coordinator is not configured")
	}
	if connectionID == "" {
		return nil, fmt.Errorf("physical stream ID must not be blank")
	}
	browserToken := domain.BrowserToken("")
	if recognitionHandle != nil {
		browserToken = domain.BrowserToken(*recognitionHandle)
	}
	var returnedToken domain.BrowserToken
	var snapshot *domain.PersonalizedSnapshot
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		sessionID, known := runtime.SessionIDByBrowserToken[browserToken]
		session := runtime.SessionsByID[sessionID]
		if browserToken == "" || !known || session == nil {
			sessionID = service.nextUniqueSessionID(runtime)
			returnedToken = service.nextUniqueBrowserToken(runtime)
			connections := make(map[domain.ConnectionID]struct{})
			if connectionID != "" {
				removeConnectionFromOtherSessions(runtime, connectionID, sessionID)
				connections[connectionID] = struct{}{}
			}
			session = &domain.LogicalSession{
				ID:             sessionID,
				FallbackName:   nextFallbackName(runtime),
				ConnectionIDs:  connections,
				RequestResults: make(map[domain.RequestID]domain.RequestResultRecord),
			}
			runtime.SessionsByID[sessionID] = session
			runtime.SessionIDByBrowserToken[returnedToken] = sessionID
			snapshot = service.subscriptionSnapshot(runtime, sessionID, returnedToken)
			return transition{accepted: true, effects: presenceEffects(runtime), boundary: subscriptionBoundary(&snapshot, register)}
		}

		returnedToken = browserToken
		if session.ConnectionIDs == nil {
			session.ConnectionIDs = make(map[domain.ConnectionID]struct{})
		}
		if _, alreadyAttached := session.ConnectionIDs[connectionID]; alreadyAttached {
			snapshot = service.subscriptionSnapshot(runtime, sessionID, returnedToken)
			return transition{boundary: subscriptionBoundary(&snapshot, register)}
		}
		wasConnected := len(session.ConnectionIDs) > 0
		otherPresenceChanged := removeConnectionFromOtherSessions(runtime, connectionID, sessionID)
		session.ConnectionIDs[connectionID] = struct{}{}
		snapshot = service.subscriptionSnapshot(runtime, sessionID, returnedToken)
		effects := []Effect(nil)
		if !wasConnected || otherPresenceChanged {
			effects = presenceEffects(runtime)
		}
		return transition{accepted: true, effects: effects, boundary: subscriptionBoundary(&snapshot, register)}
	})
	if snapshot == nil {
		return nil, fmt.Errorf("could not capture subscription snapshot")
	}
	snapshot.Revision = result.revision
	snapshot.PlayerState.Revision = result.revision
	return domain.ClonePersonalizedSnapshot(snapshot), nil
}

func subscriptionBoundary(snapshot **domain.PersonalizedSnapshot, register func(*domain.PersonalizedSnapshot)) func(uint64) {
	return func(revision uint64) {
		if snapshot == nil || *snapshot == nil {
			return
		}
		(*snapshot).Revision = revision
		if (*snapshot).PlayerState != nil {
			(*snapshot).PlayerState.Revision = revision
		}
		if register != nil {
			register(domain.ClonePersonalizedSnapshot(*snapshot))
		}
	}
}

func (service *Service) subscriptionSnapshot(runtime *domain.ProcessRuntime, sessionID domain.LogicalSessionID, handle domain.BrowserToken) *domain.PersonalizedSnapshot {
	state, ok := playerSnapshot(runtime, sessionID)
	if !ok {
		return nil
	}
	presentation := domain.TerminalPresentation{NoLiveTerminal: true}
	if service.terminals != nil && runtime.Broadcast != nil && sessionAssigned(runtime.Broadcast, sessionID) {
		terminal := activeTerminalRuntime(runtime.Broadcast)
		if terminal != nil && terminal.Lifecycle == domain.TerminalLifecycleActive {
			if live := decorateTerminalNavigation(runtime, service.terminals.ProjectRuntime(terminal)); live != nil {
				presentation = domain.TerminalPresentation{Live: clonePublicLiveState(live)}
			}
		}
	}
	return &domain.PersonalizedSnapshot{
		RecognitionHandle: domain.RecognitionHandle(handle),
		Revision:          runtime.Revision,
		PlayerState:       state,
		Terminal:          presentation,
	}
}

// SelectCharacterForRecognition resolves a unary recognition handle without
// creating replacement state and requires an active subscription before the
// existing atomic selection transaction is evaluated.
func (service *Service) SelectCharacterForRecognition(handle domain.RecognitionHandle, requestID domain.RequestID, broadcastID domain.BroadcastID, characterID domain.CharacterID) domain.ActionResult {
	sessionID, ok := service.ResolveRecognition(handle)
	if !ok {
		return domain.ActionResult{RequestID: requestID, Reason: domain.ActionReasonInvalidSession, Revision: service.Revision()}
	}
	return service.SelectCharacter(CharacterSelection{
		SessionID:   sessionID,
		RequestID:   requestID,
		BroadcastID: broadcastID,
		CharacterID: characterID,
	})
}

// DetachConnection removes one concrete membership idempotently. Claims,
// controller identity, fallback names, recognition, and request caches remain;
// an effect is emitted only when the final connection closes.
func (service *Service) DetachConnection(connectionID domain.ConnectionID) {
	if connectionID == "" {
		return
	}
	service.commit(func(runtime *domain.ProcessRuntime) transition {
		_, session := sessionForConnection(runtime, connectionID)
		if session == nil {
			return transition{}
		}
		finalConnection := len(session.ConnectionIDs) == 1
		delete(session.ConnectionIDs, connectionID)
		effects := []Effect(nil)
		if finalConnection {
			effects = presenceEffects(runtime)
		}
		return transition{accepted: true, effects: effects}
	})
}

// SelectCharacter atomically installs both sides of one exclusive claim. The
// first accepted assignment becomes controller in the same transaction.
func (service *Service) SelectCharacter(selection CharacterSelection) domain.ActionResult {
	var outcome domain.ActionResult
	commitResult := service.commit(func(runtime *domain.ProcessRuntime) transition {
		session := runtime.SessionsByID[selection.SessionID]
		if session == nil {
			outcome = rejectedSelection(selection.RequestID, domain.ActionReasonInvalidSession, runtime.Revision)
			return transition{effects: []Effect{resultEffect(selection, outcome)}}
		}
		if selection.RequestID == "" {
			outcome = rejectedSelection(selection.RequestID, domain.ActionReasonInvalidAction, runtime.Revision)
			return transition{effects: selectionResultEffects(runtime, selection, outcome)}
		}
		if len(session.ConnectionIDs) == 0 {
			outcome = rejectedSelection(selection.RequestID, domain.ActionReasonInvalidSession, runtime.Revision)
			return transition{effects: selectionResultEffects(runtime, selection, outcome)}
		}

		fingerprint := selectionFingerprint(selection)
		if cached, exists := service.requestResult(runtime, selection.SessionID, selection.RequestID); exists {
			if cached.Fingerprint == fingerprint {
				outcome = cached.Result
				return transition{effects: selectionResultEffects(runtime, selection, outcome)}
			}
			outcome = rejectedSelection(selection.RequestID, domain.ActionReasonDuplicate, runtime.Revision)
			return transition{effects: selectionResultEffects(runtime, selection, outcome)}
		}
		if selection.CharacterID == "" {
			outcome = rejectedSelection(selection.RequestID, domain.ActionReasonInvalidAction, runtime.Revision)
			return cacheSelectionRejection(service, runtime, selection, outcome)
		}
		if runtime.Broadcast == nil || runtime.Broadcast.ID != selection.BroadcastID {
			outcome = rejectedSelection(selection.RequestID, domain.ActionReasonStaleBroadcast, runtime.Revision)
			return transition{effects: selectionResultEffects(runtime, selection, outcome)}
		}
		if _, assigned := runtime.Broadcast.AssignmentsBySession[selection.SessionID]; assigned {
			outcome = rejectedSelection(selection.RequestID, domain.ActionReasonConflict, runtime.Revision)
			return cacheSelectionRejection(service, runtime, selection, outcome)
		}
		if runtime.RosterByID[selection.CharacterID] == nil {
			outcome = rejectedSelection(selection.RequestID, domain.ActionReasonConflict, runtime.Revision)
			return cacheSelectionRejection(service, runtime, selection, outcome)
		}
		if _, claimed := runtime.Broadcast.SessionByCharacter[selection.CharacterID]; claimed {
			outcome = rejectedSelection(selection.RequestID, domain.ActionReasonConflict, runtime.Revision)
			return cacheSelectionRejection(service, runtime, selection, outcome)
		}

		runtime.Broadcast.AssignmentsBySession[selection.SessionID] = selection.CharacterID
		runtime.Broadcast.SessionByCharacter[selection.CharacterID] = selection.SessionID
		if runtime.Broadcast.ControllerSessionID == nil {
			controller := selection.SessionID
			runtime.Broadcast.ControllerSessionID = &controller
		}
		outcome = domain.ActionResult{
			RequestID: selection.RequestID,
			Accepted:  true,
			Reason:    domain.ActionReasonAccepted,
			Revision:  runtime.Revision + 1,
		}
		service.storeRequestResult(runtime, selection.SessionID, selection.RequestID, domain.RequestResultRecord{
			Fingerprint: fingerprint,
			Result:      outcome,
		})
		effects := stateEffects(runtime)
		effects = service.appendCurrentTerminalEffect(runtime, effects, selection.SessionID)
		effects = append(effects, resultEffect(selection, outcome))
		return transition{accepted: true, effects: effects}
	})
	if outcome.Accepted {
		outcome.Revision = commitResult.revision
	}
	return outcome
}

type preparedPlayerAction struct {
	sessionID   domain.LogicalSessionID
	session     *domain.LogicalSession
	fingerprint string
	terminal    *domain.TerminalRuntime
}

func (service *Service) preparePlayerAction(
	runtime *domain.ProcessRuntime,
	connectionID domain.ConnectionID,
	command domain.RuntimeCommand,
) (preparedPlayerAction, domain.ActionResult, transition, bool) {
	sessionID, session := sessionForConnection(runtime, connectionID)
	if session == nil {
		result := rejectedAction(command.RequestID, domain.ActionReasonInvalidSession, runtime.Revision)
		change := transition{effects: []Effect{playerActionResultEffect(connectionID, "", result)}}
		if event, ok := commandRequestOutcomeAudit(runtime, "", command, "invalid"); ok {
			change.audit(event)
		}
		return preparedPlayerAction{}, result, change, false
	}
	if command.RequestID == "" {
		result := rejectedAction(command.RequestID, domain.ActionReasonInvalidAction, runtime.Revision)
		return preparedPlayerAction{}, result, transition{effects: []Effect{playerActionResultEffect(connectionID, sessionID, result)}}, false
	}

	fingerprint := playerActionFingerprint(command)
	if cached, exists := service.requestResult(runtime, sessionID, command.RequestID); exists {
		if cached.Fingerprint == fingerprint {
			change := transition{effects: []Effect{playerActionResultEffect(connectionID, sessionID, cached.Result)}}
			if event, ok := commandRequestOutcomeAudit(runtime, sessionID, command, "replayed"); ok {
				change.audit(event)
			}
			return preparedPlayerAction{}, cached.Result, change, false
		}
		result := rejectedAction(command.RequestID, domain.ActionReasonDuplicate, runtime.Revision)
		change := transition{effects: []Effect{playerActionResultEffect(connectionID, sessionID, result)}}
		if event, ok := commandRequestOutcomeAudit(runtime, sessionID, command, "duplicate"); ok {
			change.audit(event)
		}
		return preparedPlayerAction{}, result, change, false
	}
	reject := func(reason domain.ActionReason) (preparedPlayerAction, domain.ActionResult, transition, bool) {
		result := rejectedAction(command.RequestID, reason, runtime.Revision)
		return preparedPlayerAction{}, result, service.cachePlayerActionRejection(runtime, connectionID, sessionID, command, result), false
	}
	if !validRuntimeCommand(command) {
		return reject(domain.ActionReasonInvalidAction)
	}
	if runtime.Broadcast == nil || runtime.Broadcast.ID != command.BroadcastID {
		return reject(domain.ActionReasonStaleBroadcast)
	}
	if _, assigned := runtime.Broadcast.AssignmentsBySession[sessionID]; !assigned {
		return reject(domain.ActionReasonUnassigned)
	}
	if runtime.Broadcast.ControllerSessionID == nil || *runtime.Broadcast.ControllerSessionID != sessionID {
		return reject(domain.ActionReasonNotController)
	}
	if len(session.ConnectionIDs) == 0 {
		return reject(domain.ActionReasonControllerDisconnected)
	}
	if runtime.Broadcast.ActiveTerminalID == nil || *runtime.Broadcast.ActiveTerminalID != command.TerminalID {
		return reject(domain.ActionReasonStaleTerminal)
	}
	terminal := runtime.Broadcast.TerminalRuntimes[command.TerminalID]
	if terminal == nil || terminal.Lifecycle != domain.TerminalLifecycleActive {
		return reject(domain.ActionReasonStaleTerminal)
	}
	if runtime.PendingCommandExecution != nil || runtime.PendingTerminalNavigation != nil {
		return reject(domain.ActionReasonConflict)
	}
	return preparedPlayerAction{
		sessionID: sessionID, session: session, fingerprint: fingerprint, terminal: terminal,
	}, domain.ActionResult{}, transition{}, true
}

func (service *Service) beginTerminalReturn(
	runtime *domain.ProcessRuntime,
	sessionID domain.LogicalSessionID,
	terminal *domain.TerminalRuntime,
	returnPoint domain.TerminalReturnPoint,
) {
	runtime.PendingTerminalNavigation = &domain.PendingTerminalNavigation{
		RequestID: service.nextID(), BroadcastID: runtime.Broadcast.ID, ControllerSessionID: sessionID,
		Direction:        domain.TerminalNavigationReturn,
		SourceTerminalID: terminal.TerminalID, SourceTerminalName: terminal.TerminalName,
		CommandID: returnPoint.CommandID, CommandName: returnPoint.CommandName,
		TargetTerminalID: returnPoint.TerminalID, TargetTerminalName: returnPoint.TerminalName,
		ReturnPoint: returnPoint,
	}
	runtime.TerminalNavigationNotice = nil
}

func (service *Service) beginTerminalTransition(
	runtime *domain.ProcessRuntime,
	sessionID domain.LogicalSessionID,
	terminal *domain.TerminalRuntime,
	authored domain.ContentNode,
	target domain.TerminalTransitionTarget,
) {
	folderID := "root"
	ancestors := []string(nil)
	if len(terminal.Nav.Path) != 0 {
		folderID = terminal.Nav.Path[len(terminal.Nav.Path)-1]
		ancestors = append([]string(nil), terminal.Nav.Path[:len(terminal.Nav.Path)-1]...)
	}
	runtime.PendingTerminalNavigation = &domain.PendingTerminalNavigation{
		RequestID: service.nextID(), BroadcastID: runtime.Broadcast.ID, ControllerSessionID: sessionID,
		Direction:        domain.TerminalNavigationForward,
		SourceTerminalID: terminal.TerminalID, SourceTerminalName: terminal.TerminalName,
		CommandID: authored.ID, CommandName: authored.Name,
		TargetTerminalID: target.Target.TerminalID, TargetTerminalName: target.Target.TerminalName,
		ReturnPoint: domain.TerminalReturnPoint{
			TerminalID: terminal.TerminalID, TerminalName: terminal.TerminalName,
			FolderID: folderID, AncestorFolderIDs: ancestors,
			CommandID: authored.ID, CommandName: authored.Name,
			Origin: domain.TerminalReturnAuthored,
		},
	}
	runtime.TerminalNavigationNotice = nil
}

func (service *Service) acceptedPlayerAction(
	runtime *domain.ProcessRuntime,
	connectionID domain.ConnectionID,
	sessionID domain.LogicalSessionID,
	command domain.RuntimeCommand,
	fingerprint string,
	effects []Effect,
) (domain.ActionResult, transition) {
	result := domain.ActionResult{
		RequestID: command.RequestID, Accepted: true, Reason: domain.ActionReasonAccepted,
		Revision: runtime.Revision + 1,
	}
	return result, service.commitPlayerActionResult(runtime, connectionID, sessionID, command.RequestID, fingerprint, result, effects)
}

func (service *Service) commitPlayerActionResult(
	runtime *domain.ProcessRuntime,
	connectionID domain.ConnectionID,
	sessionID domain.LogicalSessionID,
	requestID domain.RequestID,
	fingerprint string,
	result domain.ActionResult,
	effects []Effect,
) transition {
	service.storeRequestResult(runtime, sessionID, requestID, domain.RequestResultRecord{Fingerprint: fingerprint, Result: result})
	effects = append(effects, playerActionResultEffect(connectionID, sessionID, result))
	return transition{accepted: true, effects: effects}
}

func (service *Service) playerActionStateEffects(runtime *domain.ProcessRuntime) []Effect {
	effects := stateEffects(runtime)
	if projection := service.projectActiveTerminal(runtime); projection != nil {
		effects = append(effects, Effect{Live: projection})
	}
	return effects
}

// DispatchPlayerAction resolves the sending connection and processes one
// shared navigation or hacking command. Every failed precondition returns
// before the gameplay boundary is invoked. Accepted canonical effects are
// enqueued before the initiating connection's correlated result.
func (service *Service) DispatchPlayerAction(connectionID domain.ConnectionID, command domain.RuntimeCommand) domain.ActionResult {
	var outcome domain.ActionResult
	service.commit(func(runtime *domain.ProcessRuntime) transition {
		prepared, rejected, stop, ok := service.preparePlayerAction(runtime, connectionID, command)
		if !ok {
			outcome = rejected
			return stop
		}
		sessionID := prepared.sessionID
		session := prepared.session
		fingerprint := prepared.fingerprint
		terminal := prepared.terminal
		if rootReturnRequested(terminal, command, runtime.Broadcast.Route) {
			top := cloneTerminalReturnPoint(runtime.Broadcast.Route[len(runtime.Broadcast.Route)-1])
			if !service.validTerminalReturnPoint(terminal.TerminalID, top) {
				outcome = rejectedAction(command.RequestID, domain.ActionReasonInvalidAction, runtime.Revision)
				return service.cachePlayerActionRejection(runtime, connectionID, sessionID, command, outcome)
			}
			service.beginTerminalReturn(runtime, sessionID, terminal, top)
			outcome, stop = service.acceptedPlayerAction(
				runtime, connectionID, sessionID, command, fingerprint, service.playerActionStateEffects(runtime),
			)
			return stop
		}
		authored, commandSelected := selectedAuthoredCommand(terminal, command)
		if command.Kind == domain.RuntimeCommandNavigate && command.Action == "command" && !commandSelected {
			outcome = rejectedAction(command.RequestID, domain.ActionReasonInvalidAction, runtime.Revision)
			return service.cachePlayerActionRejection(runtime, connectionID, sessionID, command, outcome)
		}
		if commandSelected && authored.Behavior() == domain.CommandBehaviorInvalid {
			outcome = rejectedAction(command.RequestID, domain.ActionReasonInvalidAction, runtime.Revision)
			return service.cachePlayerActionRejection(runtime, connectionID, sessionID, command, outcome)
		}
		if commandSelected && authored.Behavior() == domain.CommandBehaviorTerminalTransition {
			lookup, ok := domain.TerminalTransitionTarget{}, false
			if service.terminalCatalog != nil {
				lookup, ok = service.terminalCatalog.LookupTerminalTransition(terminal.TerminalID, authored.ID)
			}
			if !ok || lookup.Target.TerminalID == terminal.TerminalID || lookup.Target.TerminalID != authored.TerminalTransition.TargetTerminalID ||
				!service.sameTerminalGroup(terminal.TerminalID, lookup.Target.TerminalID) {
				targetID := authored.TerminalTransition.TargetTerminalID
				runtime.TerminalNavigationNotice = &domain.TerminalNavigationNotice{
					Reason: domain.TerminalNavigationNoticeTargetMissing, SourceTerminalID: terminal.TerminalID,
					CommandID: authored.ID, TargetTerminalID: &targetID,
				}
				outcome = rejectedAction(command.RequestID, domain.ActionReasonInvalidAction, runtime.Revision+1)
				return service.commitPlayerActionResult(
					runtime, connectionID, sessionID, command.RequestID, fingerprint, outcome, stateEffects(runtime),
				)
			}
			service.beginTerminalTransition(runtime, sessionID, terminal, *authored, lookup)
			outcome, stop = service.acceptedPlayerAction(
				runtime, connectionID, sessionID, command, fingerprint, service.playerActionStateEffects(runtime),
			)
			return stop
		}
		if commandSelected {
			mode := commandApprovalMode(terminal, authored)
			runtime.PendingCommandExecution = &domain.PendingCommandExecution{
				RequestID: service.nextID(), BroadcastID: runtime.Broadcast.ID,
				TerminalID: terminal.TerminalID, CommandID: authored.ID,
				CommandName: displayedCommandName(terminal, authored), Mode: mode,
				ConfirmationText:    commandConfirmationText(authored),
				ControllerSessionID: sessionID,
			}
			terminal.CommandExecution = &domain.CommandExecutionPresentation{
				Phase: domain.CommandExecutionPhasePending, CommandID: authored.ID,
			}
			session.Notice = nil
			outcome, stop = service.acceptedPlayerAction(
				runtime, connectionID, sessionID, command, fingerprint, service.playerActionStateEffects(runtime),
			)
			stop.audit(AuditEvent{
				Name: "command.request_received", Outcome: "accepted", SessionID: sessionID,
				Role: roleForSession(runtime.Broadcast, sessionID), RequestID: runtime.PendingCommandExecution.RequestID,
				BroadcastID: runtime.Broadcast.ID, TerminalID: terminal.TerminalID, CommandID: authored.ID,
				Mode: string(mode),
			})
			return stop
		}
		if service.actions == nil {
			outcome = rejectedAction(command.RequestID, domain.ActionReasonInvalidAction, runtime.Revision)
			return service.cachePlayerActionRejection(runtime, connectionID, sessionID, command, outcome)
		}

		before := cloneTerminalRuntime(terminal)
		projection, ok := service.actions.Apply(terminal, command)
		if !ok || projection == nil {
			runtime.Broadcast.TerminalRuntimes[command.TerminalID] = before
			outcome = rejectedAction(command.RequestID, domain.ActionReasonInvalidAction, runtime.Revision)
			return service.cachePlayerActionRejection(runtime, connectionID, sessionID, command, outcome)
		}
		if command.Kind != domain.RuntimeCommandPresentation {
			session.Notice = nil
		}
		outcome, stop = service.acceptedPlayerAction(
			runtime, connectionID, sessionID, command, fingerprint, []Effect{{Live: projection}},
		)
		if command.Kind == domain.RuntimeCommandGuess || command.Kind == domain.RuntimeCommandActivatePattern {
			stop.audit(hackActionAudit(runtime, sessionID, command, before.Hack, terminal.Hack))
		}
		return stop
	})
	return outcome
}

// DispatchPlayerActionForRecognition resolves the opaque handle to one active
// physical stream without creating state, then executes the same canonical
// transaction used by transport compatibility callers.
func (service *Service) DispatchPlayerActionForRecognition(handle domain.RecognitionHandle, command domain.RuntimeCommand) domain.ActionResult {
	if service == nil {
		return domain.ActionResult{RequestID: command.RequestID, Reason: domain.ActionReasonInvalidSession}
	}
	service.mu.RLock()
	sessionID, ok := service.runtime.SessionIDByBrowserToken[handle]
	session := service.runtime.SessionsByID[sessionID]
	var connectionID domain.ConnectionID
	if ok && session != nil {
		connections := make([]domain.ConnectionID, 0, len(session.ConnectionIDs))
		for candidate := range session.ConnectionIDs {
			connections = append(connections, candidate)
		}
		slices.Sort(connections)
		if len(connections) > 0 {
			connectionID = connections[0]
		}
	}
	revision := service.runtime.Revision
	service.mu.RUnlock()
	if !ok || session == nil {
		return domain.ActionResult{RequestID: command.RequestID, Reason: domain.ActionReasonInvalidSession, Revision: revision}
	}
	if connectionID == "" {
		return domain.ActionResult{RequestID: command.RequestID, Reason: domain.ActionReasonControllerDisconnected, Revision: revision}
	}
	return service.DispatchPlayerAction(connectionID, command)
}

// ForceHackSuccess executes the exact private Overseer operation inside the
// coordinator order. It spends no attempt and grants no player authority.
func (service *Service) ForceHackSuccess() (*domain.PublicHackState, bool) {
	var state *domain.PublicHackState
	result := service.commit(func(runtime *domain.ProcessRuntime) transition {
		if service.trustedHack == nil || runtime.Broadcast == nil {
			return transition{}
		}
		terminal := activeTerminalRuntime(runtime.Broadcast)
		if terminal == nil || terminal.Lifecycle != domain.TerminalLifecycleActive {
			return transition{}
		}
		projection, ok := service.trustedHack.ForceRuntimeHackSuccess(terminal)
		if !ok || projection == nil || projection.Hack == nil || projection.TerminalID != terminal.TerminalID {
			return transition{}
		}
		state = clonePublicHackState(projection.Hack)
		return transition{
			accepted: true,
			effects: []Effect{{
				Live:   projection,
				Master: masterSnapshot(runtime),
			}},
		}
	})
	if !result.accepted {
		return nil, false
	}
	return clonePublicHackState(state), true
}

func (service *Service) nextID() string {
	return service.ids.Next()
}

// commit is the sole canonical transaction boundary. The mutation runs on a
// private deep copy, so rejection cannot leak partial state. Accepted changes,
// boundary registration, complete compound-update assembly, effect revision
// stamping, detachment, and synchronous enqueueing all occur before the mutex
// is released, preventing later transitions from publishing ahead of an
// earlier one.
func (service *Service) commit(apply func(*domain.ProcessRuntime) transition) transitionResult {
	service.mu.Lock()
	defer service.mu.Unlock()

	before := cloneProcessRuntime(&service.runtime)
	working := cloneProcessRuntime(&service.runtime)
	result := apply(working)
	if result.accepted {
		working.Revision = service.runtime.Revision + 1
		for _, event := range deriveAuditEvents(before, working) {
			result.audit(event)
		}
		if event, ok := supersededCommandAudit(before, working, result.effects); ok {
			result.audit(event)
		}
		service.runtime = *working
	} else if result.persist {
		working.Revision = service.runtime.Revision
		service.runtime = *working
	}
	revision := service.runtime.Revision
	if result.boundary != nil {
		result.boundary(revision)
	}
	for _, effect := range result.effects {
		service.enqueue(detachEffect(effect, revision))
	}
	if result.accepted {
		for _, sessionID := range sortedSessionIDs(&service.runtime) {
			update := service.compoundUpdateLocked(&service.runtime, sessionID, revision)
			if update != nil {
				service.enqueue(detachEffect(Effect{SessionID: sessionID, Update: update}, revision))
			}
		}
	}
	return transitionResult{accepted: result.accepted, revision: revision}
}

func (service *Service) requestResult(runtime *domain.ProcessRuntime, sessionID domain.LogicalSessionID, requestID domain.RequestID) (domain.RequestResultRecord, bool) {
	session := runtime.SessionsByID[sessionID]
	if session == nil {
		return domain.RequestResultRecord{}, false
	}
	record, ok := session.RequestResults[requestID]
	return record, ok
}

// storeRequestResult adds or replaces one replay record while keeping each
// logical session's cache bounded. When full, the lexically smallest request
// ID is evicted; the deterministic policy keeps race tests reproducible.
func (service *Service) storeRequestResult(runtime *domain.ProcessRuntime, sessionID domain.LogicalSessionID, requestID domain.RequestID, record domain.RequestResultRecord) bool {
	session := runtime.SessionsByID[sessionID]
	if session == nil {
		return false
	}
	if session.RequestResults == nil {
		session.RequestResults = make(map[domain.RequestID]domain.RequestResultRecord)
	}
	if _, exists := session.RequestResults[requestID]; !exists && len(session.RequestResults) >= service.requestResultLimit {
		var evict domain.RequestID
		for candidate := range session.RequestResults {
			if evict == "" || candidate < evict {
				evict = candidate
			}
		}
		delete(session.RequestResults, evict)
	}
	session.RequestResults[requestID] = record
	return true
}

func clearRequestResults(runtime *domain.ProcessRuntime) {
	for _, session := range runtime.SessionsByID {
		if session != nil {
			session.RequestResults = make(map[domain.RequestID]domain.RequestResultRecord)
		}
	}
}

func nextFallbackName(runtime *domain.ProcessRuntime) string {
	used := make(map[string]struct{}, len(runtime.SessionsByID))
	for _, session := range runtime.SessionsByID {
		if session != nil {
			used[session.FallbackName] = struct{}{}
		}
	}
	for number := 1; ; number++ {
		candidate := fmt.Sprintf("PLAYER %d", number)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

func validatedCoordinationName(value string, label string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s must not be blank", label)
	}
	if utf8.RuneCountInString(value) > 80 {
		return "", fmt.Errorf("%s must not exceed 80 characters", label)
	}
	return value, nil
}

func ensureAssignmentIndexes(broadcast *domain.LiveBroadcast) {
	if broadcast.AssignmentsBySession == nil {
		broadcast.AssignmentsBySession = make(map[domain.LogicalSessionID]domain.CharacterID)
	}
	if broadcast.SessionByCharacter == nil {
		broadcast.SessionByCharacter = make(map[domain.CharacterID]domain.LogicalSessionID)
	}
}

func ensureTerminalRuntimeSlots(broadcast *domain.LiveBroadcast) {
	if broadcast.TerminalRuntimes == nil {
		broadcast.TerminalRuntimes = make(map[string]*domain.TerminalRuntime)
	}
}

func activeTerminalRuntime(broadcast *domain.LiveBroadcast) *domain.TerminalRuntime {
	if broadcast == nil || broadcast.ActiveTerminalID == nil {
		return nil
	}
	return broadcast.TerminalRuntimes[*broadcast.ActiveTerminalID]
}

func clearCommandExecutionRuntime(runtime *domain.ProcessRuntime) {
	if runtime == nil {
		return
	}
	runtime.PendingCommandExecution = nil
	if runtime.Broadcast == nil {
		return
	}
	for _, terminal := range runtime.Broadcast.TerminalRuntimes {
		if terminal != nil {
			terminal.CommandExecution = nil
		}
	}
}

func selectedAuthoredCommand(runtime *domain.TerminalRuntime, command domain.RuntimeCommand) (*domain.ContentNode, bool) {
	if runtime == nil || command.Kind != domain.RuntimeCommandNavigate || command.Action != "command" || command.NodeID == "" {
		return nil, false
	}
	folder := &runtime.Tree
	if len(runtime.Nav.Path) == 0 || runtime.Nav.Path[0] != runtime.Tree.ID {
		return nil, false
	}
	for _, folderID := range runtime.Nav.Path[1:] {
		var next *domain.ContentNode
		for index := range folder.Children {
			child := &folder.Children[index]
			if child.ID == folderID && child.Type == domain.NodeFolder {
				next = child
				break
			}
		}
		if next == nil {
			return nil, false
		}
		folder = next
	}
	for index := range folder.Children {
		candidate := &folder.Children[index]
		if candidate.ID == command.NodeID && candidate.Type == domain.NodeCommand {
			return candidate, true
		}
	}
	return nil, false
}

func commandApprovalMode(runtime *domain.TerminalRuntime, command *domain.ContentNode) domain.CommandApprovalMode {
	if command == nil || command.StateChange == nil {
		return domain.CommandApprovalModeOrdinary
	}
	if runtime != nil {
		if _, completed := runtime.CommandStates[command.ID]; completed {
			return domain.CommandApprovalModeCompletedStateChange
		}
	}
	return domain.CommandApprovalModeStateChange
}

func displayedCommandName(runtime *domain.TerminalRuntime, command *domain.ContentNode) string {
	if command == nil {
		return ""
	}
	if runtime != nil {
		if completed, ok := runtime.CommandStates[command.ID]; ok {
			return completed.CompletedName
		}
	}
	return command.Name
}

func commandConfirmationText(command *domain.ContentNode) string {
	if command != nil && command.StateChange != nil {
		return command.StateChange.ConfirmationText
	}
	return "Выполнить команду?"
}

func currentPendingCommandExecution(pending *domain.PendingCommandExecution, broadcast *domain.LiveBroadcast, terminal *domain.TerminalRuntime, requestID string) bool {
	return pending != nil && pending.RequestID == requestID &&
		broadcast != nil && pending.BroadcastID == broadcast.ID &&
		broadcast.ActiveTerminalID != nil && *broadcast.ActiveTerminalID == pending.TerminalID &&
		terminal != nil && terminal.TerminalID == pending.TerminalID && terminal.Lifecycle == domain.TerminalLifecycleActive &&
		terminal.CommandExecution != nil && terminal.CommandExecution.Phase == domain.CommandExecutionPhasePending &&
		terminal.CommandExecution.CommandID == pending.CommandID
}

func terminalByStableID(session *domain.Session, terminalID string) *domain.Terminal {
	if session == nil {
		return nil
	}
	for index := range session.Terminals {
		if session.Terminals[index].ID == terminalID {
			return &session.Terminals[index]
		}
	}
	return nil
}

func durableCommandStateMatchesAuthored(state domain.CommandExecutionState, command *domain.ContentNode) bool {
	if command == nil || command.StateChange == nil {
		return false
	}
	authored := command.StateChange.EntryContentChange
	frozen := state.EntryContentChange
	if authored == nil || frozen == nil {
		return authored == nil && frozen == nil
	}
	return frozen.BlockID == authored.BlockID
}

func cloneCommandStates(states map[string]domain.CommandExecutionState) map[string]domain.CommandExecutionState {
	return domain.CloneCommandExecutionStates(states)
}

func (service *Service) projectActiveTerminal(runtime *domain.ProcessRuntime) *domain.PublicLiveState {
	if service == nil || service.terminals == nil || runtime == nil || runtime.Broadcast == nil {
		return nil
	}
	terminal := activeTerminalRuntime(runtime.Broadcast)
	if terminal == nil || terminal.Lifecycle != domain.TerminalLifecycleActive {
		return nil
	}
	return decorateTerminalNavigation(runtime, service.terminals.ProjectRuntime(terminal))
}

func decorateTerminalNavigation(runtime *domain.ProcessRuntime, live *domain.PublicLiveState) *domain.PublicLiveState {
	if live == nil {
		return nil
	}
	result := clonePublicLiveState(live)
	if runtime == nil || runtime.Broadcast == nil {
		result.TerminalNavigation = nil
		return result
	}
	broadcast := runtime.Broadcast
	if len(broadcast.Route) == 0 && runtime.PendingTerminalNavigation == nil {
		result.TerminalNavigation = nil
		return result
	}
	presentation := &domain.TerminalNavigationPresentation{RouteDepth: uint32(len(broadcast.Route))}
	if len(broadcast.Route) != 0 {
		top := broadcast.Route[len(broadcast.Route)-1]
		presentation.ReturnTarget = &domain.TerminalReturnTarget{TerminalID: top.TerminalID, TerminalName: top.TerminalName}
	}
	if pending := runtime.PendingTerminalNavigation; pending != nil {
		presentation.Pending = &domain.PendingTerminalNavigationPresentation{
			Direction: pending.Direction, TargetTerminalID: pending.TargetTerminalID, TargetTerminalName: pending.TargetTerminalName,
		}
	}
	result.TerminalNavigation = presentation
	return result
}

func unfinishedTerminalRuntime(runtime *domain.TerminalRuntime) bool {
	return runtime != nil && runtime.Hack != nil && !runtime.Hack.Solved && !runtime.Hack.Failed
}

func clearTerminalNavigationRuntime(runtime *domain.ProcessRuntime) bool {
	if runtime == nil {
		return false
	}
	changed := runtime.PendingTerminalNavigation != nil || runtime.TerminalNavigationNotice != nil
	runtime.PendingTerminalNavigation = nil
	runtime.TerminalNavigationNotice = nil
	if runtime.Broadcast != nil && len(runtime.Broadcast.Route) != 0 {
		runtime.Broadcast.Route = nil
		changed = true
	}
	return changed
}

func rootReturnRequested(terminal *domain.TerminalRuntime, command domain.RuntimeCommand, route []domain.TerminalReturnPoint) bool {
	return terminal != nil && command.Kind == domain.RuntimeCommandNavigate && command.Action == "back" &&
		len(route) != 0 && terminal.Nav.Mode == "list" && len(terminal.Nav.Path) == 1 && terminal.Nav.Path[0] == "root" &&
		terminal.Nav.ViewEntryID == nil && terminal.Nav.CommandNodeID == nil
}

func cloneTerminalReturnPoint(point domain.TerminalReturnPoint) domain.TerminalReturnPoint {
	point.AncestorFolderIDs = append([]string(nil), point.AncestorFolderIDs...)
	return point
}

func sameTerminalReturnPoint(left, right domain.TerminalReturnPoint) bool {
	if left.TerminalID != right.TerminalID || left.TerminalName != right.TerminalName || left.FolderID != right.FolderID ||
		left.CommandID != right.CommandID || left.CommandName != right.CommandName || left.Origin != right.Origin ||
		left.GroupID != right.GroupID || left.GroupPosition != right.GroupPosition || len(left.AncestorFolderIDs) != len(right.AncestorFolderIDs) {
		return false
	}
	for index := range left.AncestorFolderIDs {
		if left.AncestorFolderIDs[index] != right.AncestorFolderIDs[index] {
			return false
		}
	}
	return true
}

func (service *Service) sameTerminalGroup(sourceTerminalID, targetTerminalID string) bool {
	catalog, ok := service.terminalCatalog.(terminalGroupCatalog)
	if !ok {
		// Compatibility for focused pre-grouping fakes. Production session.Service
		// implements terminalGroupCatalog and therefore always takes the strict path.
		return true
	}
	source, sourceOK := catalog.LookupTerminalGroup(sourceTerminalID)
	target, targetOK := catalog.LookupTerminalGroup(targetTerminalID)
	return sourceOK && targetOK && source.ID != "" && source.ID == target.ID
}

func (service *Service) establishInitialTerminalRoute(broadcast *domain.LiveBroadcast, terminalID string) {
	if broadcast == nil || broadcast.InitialTerminalEstablished {
		return
	}
	broadcast.InitialTerminalEstablished = true
	catalog, ok := service.terminalCatalog.(terminalGroupCatalog)
	if !ok || service.terminalCatalog == nil {
		return
	}
	group, ok := catalog.LookupTerminalGroup(terminalID)
	if !ok || group.ID == "" {
		return
	}
	position := -1
	for index, memberID := range group.TerminalIDs {
		if memberID == terminalID {
			position = index
			break
		}
	}
	if position < 0 {
		return
	}
	if position == 0 {
		broadcast.InitialTerminalID = terminalID
		broadcast.InitialTerminalGroupID = group.ID
		broadcast.InitialTerminalGroupPosition = position
		return
	}
	route := make([]domain.TerminalReturnPoint, 0, position)
	for index, memberID := range group.TerminalIDs[:position] {
		target, found := service.terminalCatalog.LookupTerminal(memberID)
		if !found || target.TerminalID != memberID {
			return
		}
		route = append(route, domain.TerminalReturnPoint{
			TerminalID: target.TerminalID, TerminalName: target.TerminalName,
			FolderID: "root", Origin: domain.TerminalReturnInitialPrefix,
			GroupID: group.ID, GroupPosition: index,
		})
	}
	broadcast.InitialTerminalID = terminalID
	broadcast.InitialTerminalGroupID = group.ID
	broadcast.InitialTerminalGroupPosition = position
	broadcast.Route = route
}

func (service *Service) validTerminalReturnPoint(sourceTerminalID string, point domain.TerminalReturnPoint) bool {
	if !service.sameTerminalGroup(sourceTerminalID, point.TerminalID) {
		return false
	}
	if point.Origin != domain.TerminalReturnInitialPrefix {
		return true
	}
	catalog, ok := service.terminalCatalog.(terminalGroupCatalog)
	if !ok {
		return false
	}
	sourceGroup, sourceOK := catalog.LookupTerminalGroup(sourceTerminalID)
	targetGroup, targetOK := catalog.LookupTerminalGroup(point.TerminalID)
	if !sourceOK || !targetOK || sourceGroup.ID == "" || sourceGroup.ID != targetGroup.ID ||
		point.GroupID != targetGroup.ID {
		return false
	}
	sourcePosition, targetPosition := -1, -1
	for index, terminalID := range targetGroup.TerminalIDs {
		if terminalID == sourceTerminalID {
			sourcePosition = index
		}
		if terminalID == point.TerminalID {
			targetPosition = index
		}
	}
	return targetPosition == point.GroupPosition && sourcePosition == targetPosition+1
}

func sessionAssigned(broadcast *domain.LiveBroadcast, sessionID domain.LogicalSessionID) bool {
	if broadcast == nil {
		return false
	}
	if _, assigned := broadcast.AssignmentsBySession[sessionID]; assigned {
		return true
	}
	for _, ownerID := range broadcast.SessionByCharacter {
		if ownerID == sessionID {
			return true
		}
	}
	return false
}

func characterOwner(broadcast *domain.LiveBroadcast, characterID domain.CharacterID) (domain.LogicalSessionID, bool) {
	if broadcast == nil {
		return "", false
	}
	if sessionID, claimed := broadcast.SessionByCharacter[characterID]; claimed {
		return sessionID, true
	}
	for sessionID, assignedCharacterID := range broadcast.AssignmentsBySession {
		if assignedCharacterID == characterID {
			return sessionID, true
		}
	}
	return "", false
}

func characterClaimed(broadcast *domain.LiveBroadcast, characterID domain.CharacterID) bool {
	_, claimed := characterOwner(broadcast, characterID)
	return claimed
}

func clearControllerIfSession(broadcast *domain.LiveBroadcast, sessionID domain.LogicalSessionID) {
	if broadcast != nil && broadcast.ControllerSessionID != nil && *broadcast.ControllerSessionID == sessionID {
		broadcast.ControllerSessionID = nil
	}
}

func (service *Service) nextUniqueSessionID(runtime *domain.ProcessRuntime) domain.LogicalSessionID {
	for {
		sessionID := domain.LogicalSessionID(service.nextID())
		if sessionID == "" {
			continue
		}
		if _, exists := runtime.SessionsByID[sessionID]; !exists {
			return sessionID
		}
	}
}

func (service *Service) nextUniqueBrowserToken(runtime *domain.ProcessRuntime) domain.BrowserToken {
	for {
		browserToken := domain.BrowserToken(service.nextID())
		if browserToken == "" {
			continue
		}
		if _, exists := runtime.SessionIDByBrowserToken[browserToken]; !exists {
			return browserToken
		}
	}
}

func stateEffects(runtime *domain.ProcessRuntime) []Effect {
	return presenceEffects(runtime)
}

// presenceEffects deliberately contains only complete master/player
// projections. Connection membership never publishes terminal, hacking, or
// request-result payloads and therefore cannot regenerate gameplay state.
func presenceEffects(runtime *domain.ProcessRuntime) []Effect {
	effects := []Effect{{Master: masterSnapshot(runtime)}}
	sessionIDs := sortedSessionIDs(runtime)
	for _, sessionID := range sessionIDs {
		state, ok := playerSnapshot(runtime, sessionID)
		if ok {
			effects = append(effects, Effect{SessionID: sessionID, Player: state})
		}
	}
	return effects
}

func (service *Service) appendCurrentTerminalEffect(runtime *domain.ProcessRuntime, effects []Effect, sessionID domain.LogicalSessionID) []Effect {
	if service.terminals == nil || runtime == nil || runtime.Broadcast == nil {
		return effects
	}
	terminal := activeTerminalRuntime(runtime.Broadcast)
	if terminal == nil || terminal.Lifecycle != domain.TerminalLifecycleActive {
		return effects
	}
	projection := decorateTerminalNavigation(runtime, service.terminals.ProjectRuntime(terminal))
	if projection == nil {
		return effects
	}
	return append(effects, Effect{SessionID: sessionID, Live: projection})
}

func selectionResultEffects(runtime *domain.ProcessRuntime, selection CharacterSelection, result domain.ActionResult) []Effect {
	var effects []Effect
	if state, ok := playerSnapshot(runtime, selection.SessionID); ok {
		effects = append(effects, Effect{SessionID: selection.SessionID, Player: state})
	}
	return append(effects, resultEffect(selection, result))
}

func resultEffect(selection CharacterSelection, result domain.ActionResult) Effect {
	return Effect{
		ConnectionID: selection.ConnectionID,
		SessionID:    selection.SessionID,
		Result:       &result,
	}
}

func cacheSelectionRejection(service *Service, runtime *domain.ProcessRuntime, selection CharacterSelection, result domain.ActionResult) transition {
	service.storeRequestResult(runtime, selection.SessionID, selection.RequestID, domain.RequestResultRecord{
		Fingerprint: selectionFingerprint(selection),
		Result:      result,
	})
	return transition{
		persist: true,
		effects: selectionResultEffects(runtime, selection, result),
	}
}

func rejectedSelection(requestID domain.RequestID, reason domain.ActionReason, revision uint64) domain.ActionResult {
	return domain.ActionResult{RequestID: requestID, Reason: reason, Revision: revision}
}

func selectionFingerprint(selection CharacterSelection) string {
	if selection.Fingerprint != "" {
		return selection.Fingerprint
	}
	fields := []string{string(selection.BroadcastID), string(selection.CharacterID)}
	return fingerprintFields("SelectCharacter", fields[0], fields[1])
}

func sessionForConnection(runtime *domain.ProcessRuntime, connectionID domain.ConnectionID) (domain.LogicalSessionID, *domain.LogicalSession) {
	for _, sessionID := range sortedSessionIDs(runtime) {
		session := runtime.SessionsByID[sessionID]
		if session == nil {
			continue
		}
		if _, connected := session.ConnectionIDs[connectionID]; connected {
			return sessionID, session
		}
	}
	return "", nil
}

// removeConnectionFromOtherSessions preserves the invariant that one concrete
// connection belongs to at most one logical session. The return value reports
// whether removing it changed any logical session from present to absent.
func removeConnectionFromOtherSessions(runtime *domain.ProcessRuntime, connectionID domain.ConnectionID, targetSessionID domain.LogicalSessionID) bool {
	presenceChanged := false
	for _, sessionID := range sortedSessionIDs(runtime) {
		if sessionID == targetSessionID {
			continue
		}
		session := runtime.SessionsByID[sessionID]
		if session == nil {
			continue
		}
		if _, exists := session.ConnectionIDs[connectionID]; !exists {
			continue
		}
		delete(session.ConnectionIDs, connectionID)
		if len(session.ConnectionIDs) == 0 {
			presenceChanged = true
		}
	}
	return presenceChanged
}

func validRuntimeCommand(command domain.RuntimeCommand) bool {
	if command.RequestID == "" || command.BroadcastID == "" || strings.TrimSpace(command.TerminalID) == "" {
		return false
	}
	switch command.Kind {
	case domain.RuntimeCommandNavigate:
		if command.TargetID != "" || command.PatternID != "" {
			return false
		}
		switch command.Action {
		case "enter", "command", "entry":
			return strings.TrimSpace(command.NodeID) != ""
		case "back":
			return command.NodeID == "" || strings.TrimSpace(command.NodeID) != ""
		default:
			return false
		}
	case domain.RuntimeCommandGuess:
		return strings.TrimSpace(command.TargetID) != "" && command.Action == "" && command.NodeID == "" && command.PatternID == ""
	case domain.RuntimeCommandActivatePattern:
		return strings.TrimSpace(command.PatternID) != "" && command.Action == "" && command.NodeID == "" && command.TargetID == ""
	case domain.RuntimeCommandPresentation:
		if command.Action != "" || command.NodeID != "" || command.TargetID != "" || command.PatternID != "" ||
			domain.ValidatePublicField(domain.PublicFieldActionTarget, command.Presentation.ContextKey) != nil {
			return false
		}
		switch command.Presentation.Kind {
		case domain.ControllerTerminalPresentationNone:
			return command.Presentation.TargetID == "" && command.Presentation.PatternID == "" && command.Presentation.PageIndex == 0
		case domain.ControllerTerminalPresentationMenu:
			return domain.ValidatePublicField(domain.PublicFieldActionTarget, command.Presentation.TargetID) == nil &&
				command.Presentation.PatternID == "" && command.Presentation.PageIndex == 0
		case domain.ControllerTerminalPresentationPage:
			return command.Presentation.TargetID == "" && command.Presentation.PatternID == "" &&
				command.Presentation.PageIndex <= domain.MaxPresentationPageIndex
		case domain.ControllerTerminalPresentationHacking:
			targetValid := command.Presentation.TargetID != "" && command.Presentation.PatternID == "" &&
				domain.ValidatePublicField(domain.PublicFieldActionTarget, command.Presentation.TargetID) == nil
			patternValid := command.Presentation.PatternID != "" && command.Presentation.TargetID == "" &&
				domain.ValidatePublicField(domain.PublicFieldActionTarget, command.Presentation.PatternID) == nil
			return command.Presentation.PageIndex == 0 && (targetValid || patternValid)
		default:
			return false
		}
	default:
		return false
	}
}

func playerActionFingerprint(command domain.RuntimeCommand) string {
	if command.PayloadFingerprint != "" {
		return command.PayloadFingerprint
	}
	return fingerprintFields(
		string(command.Kind),
		string(command.BroadcastID),
		command.TerminalID,
		command.Action,
		command.NodeID,
		command.TargetID,
		command.PatternID,
		string(command.Presentation.Kind),
		command.Presentation.ContextKey,
		command.Presentation.TargetID,
		command.Presentation.PatternID,
		strconv.FormatUint(uint64(command.Presentation.PageIndex), 10),
	)
}

func fingerprintFields(fields ...string) string {
	var fingerprint strings.Builder
	for _, field := range fields {
		fmt.Fprintf(&fingerprint, "%d:%s;", len(field), field)
	}
	return fingerprint.String()
}

func rejectedAction(requestID domain.RequestID, reason domain.ActionReason, revision uint64) domain.ActionResult {
	return domain.ActionResult{RequestID: requestID, Reason: reason, Revision: revision}
}

func playerActionResultEffect(connectionID domain.ConnectionID, sessionID domain.LogicalSessionID, result domain.ActionResult) Effect {
	return Effect{ConnectionID: connectionID, SessionID: sessionID, Result: &result}
}

func (service *Service) cachePlayerActionRejection(runtime *domain.ProcessRuntime, connectionID domain.ConnectionID, sessionID domain.LogicalSessionID, command domain.RuntimeCommand, result domain.ActionResult) transition {
	service.storeRequestResult(runtime, sessionID, command.RequestID, domain.RequestResultRecord{
		Fingerprint: playerActionFingerprint(command),
		Result:      result,
	})
	change := transition{
		persist: true,
		effects: []Effect{
			playerActionResultEffect(connectionID, sessionID, result),
		},
	}
	if event, ok := commandRequestOutcomeAudit(runtime, sessionID, command, commandOutcomeForReason(result.Reason)); ok {
		change.audit(event)
	}
	if event, ok := rejectedHackActionAudit(runtime, sessionID, command); ok {
		change.audit(event)
	}
	return change
}

func rejectedHackActionAudit(runtime *domain.ProcessRuntime, sessionID domain.LogicalSessionID, command domain.RuntimeCommand) (AuditEvent, bool) {
	if command.Kind != domain.RuntimeCommandGuess && command.Kind != domain.RuntimeCommandActivatePattern {
		return AuditEvent{}, false
	}
	event := AuditEvent{
		Name: "hack.guess", Outcome: "rejected", SessionID: sessionID,
		TerminalID: command.TerminalID,
	}
	if command.Kind == domain.RuntimeCommandActivatePattern {
		event.Name = "hack.pattern"
	}
	if runtime == nil || runtime.Broadcast == nil {
		return event, true
	}
	event.BroadcastID = runtime.Broadcast.ID
	event.Role = roleForSession(runtime.Broadcast, sessionID)
	terminal := activeTerminalRuntime(runtime.Broadcast)
	if terminal != nil && terminal.TerminalID == command.TerminalID && terminal.Hack != nil {
		event.PuzzleID = terminal.Hack.GenerationID
		event.AttemptsLeft = terminal.Hack.AttemptsLeft
	}
	return event, true
}

func commandRequestOutcomeAudit(runtime *domain.ProcessRuntime, sessionID domain.LogicalSessionID, command domain.RuntimeCommand, outcome string) (AuditEvent, bool) {
	if command.Kind != domain.RuntimeCommandNavigate || command.Action != "command" || command.RequestID == "" {
		return AuditEvent{}, false
	}
	event := AuditEvent{
		Name: "command.request_outcome", Outcome: outcome,
		RequestID: command.RequestID, SessionID: sessionID,
	}
	if runtime != nil && runtime.Broadcast != nil {
		event.BroadcastID = runtime.Broadcast.ID
		event.Role = roleForSession(runtime.Broadcast, sessionID)
	}
	if validRuntimeCommand(command) {
		event.TerminalID = command.TerminalID
		event.CommandID = command.NodeID
	}
	return event, true
}

func commandOutcomeForReason(reason domain.ActionReason) string {
	switch reason {
	case domain.ActionReasonDuplicate:
		return "duplicate"
	case domain.ActionReasonStaleBroadcast:
		return "stale-broadcast"
	case domain.ActionReasonStaleTerminal:
		return "stale-terminal"
	case domain.ActionReasonUnassigned:
		return "unassigned"
	case domain.ActionReasonNotController:
		return "not-controller"
	case domain.ActionReasonControllerDisconnected:
		return "controller-disconnected"
	case domain.ActionReasonConflict:
		return "conflict"
	default:
		return "invalid"
	}
}

func newProcessRuntime() domain.ProcessRuntime {
	return domain.ProcessRuntime{
		SessionsByID:            make(map[domain.LogicalSessionID]*domain.LogicalSession),
		SessionIDByBrowserToken: make(map[domain.BrowserToken]domain.LogicalSessionID),
		RosterByID:              make(map[domain.CharacterID]*domain.CharacterRosterEntry),
	}
}

func sortedSessionIDs(runtime *domain.ProcessRuntime) []domain.LogicalSessionID {
	sessionIDs := make([]domain.LogicalSessionID, 0, len(runtime.SessionsByID))
	for sessionID := range runtime.SessionsByID {
		sessionIDs = append(sessionIDs, sessionID)
	}
	slices.Sort(sessionIDs)
	return sessionIDs
}

func masterSnapshot(runtime *domain.ProcessRuntime) *domain.MasterCoordinationState {
	state := &domain.MasterCoordinationState{Revision: runtime.Revision}
	if runtime.ActivePlayerConfig != nil {
		state.PlayerConfig = &domain.PlayerConfigMetadata{
			Status: "loaded", FilePath: runtime.ActivePlayerConfig.Path, Version: runtime.ActivePlayerConfig.Version, Name: runtime.ActivePlayerConfig.Name,
		}
	}

	for _, characterID := range orderedRosterIDs(runtime) {
		character := runtime.RosterByID[characterID]
		if character == nil {
			continue
		}
		entry := domain.MasterRosterEntry{
			ID:                  character.ID,
			Name:                character.Name,
			Intelligence:        character.Intelligence,
			HackerPerkAvailable: character.HackerPerkAvailable,
		}
		if runtime.Broadcast != nil {
			if sessionID, claimed := runtime.Broadcast.SessionByCharacter[character.ID]; claimed {
				claimedBy := sessionID
				entry.ClaimedBySessionID = &claimedBy
			}
		}
		state.Roster = append(state.Roster, entry)
	}

	for _, sessionID := range sortedSessionIDs(runtime) {
		session := runtime.SessionsByID[sessionID]
		if session == nil {
			continue
		}
		entry := domain.MasterSessionEntry{
			ID:           session.ID,
			FallbackName: session.FallbackName,
			Connected:    len(session.ConnectionIDs) > 0,
			Role:         roleForSession(runtime.Broadcast, session.ID),
		}
		if character := assignedCharacter(runtime, session.ID); character != nil {
			entry.Character = &domain.PlayerCharacter{ID: character.ID, Name: character.Name}
		}
		state.Sessions = append(state.Sessions, entry)
	}

	if broadcast := runtime.Broadcast; broadcast != nil {
		state.Broadcast = &domain.MasterBroadcastState{
			ID:                  broadcast.ID,
			ControllerSessionID: cloneLogicalSessionID(broadcast.ControllerSessionID),
			ActiveTerminalID:    cloneString(broadcast.ActiveTerminalID),
		}
	}
	if pending := runtime.PendingSwitch; pending != nil {
		state.PendingSwitch = &domain.MasterPendingSwitch{
			SwitchID:         pending.ID,
			BroadcastID:      pending.BroadcastID,
			SourceTerminalID: pending.SourceTerminalID,
		}
		if pending.Target != nil {
			targetID := pending.Target.TerminalID
			state.PendingSwitch.TargetTerminalID = &targetID
		}
	}
	if pending := runtime.PendingCommandExecution; pending != nil {
		state.PendingCommandExecution = &domain.MasterPendingCommandExecution{
			RequestID: pending.RequestID, BroadcastID: pending.BroadcastID,
			TerminalID: pending.TerminalID, CommandID: pending.CommandID,
			CommandName: pending.CommandName, Mode: pending.Mode,
			ConfirmationText: pending.ConfirmationText,
		}
	}
	if pending := runtime.PendingTerminalNavigation; pending != nil {
		routeDepth := uint32(0)
		if runtime.Broadcast != nil {
			routeDepth = uint32(len(runtime.Broadcast.Route))
		}
		state.PendingTerminalNavigation = &domain.MasterPendingTerminalNavigation{
			RequestID: pending.RequestID, BroadcastID: pending.BroadcastID, Direction: pending.Direction,
			SourceTerminalID: pending.SourceTerminalID, SourceTerminalName: pending.SourceTerminalName,
			CommandID: pending.CommandID, CommandName: pending.CommandName,
			TargetTerminalID: pending.TargetTerminalID, TargetTerminalName: pending.TargetTerminalName,
			RouteDepth: routeDepth,
		}
	}
	if notice := runtime.TerminalNavigationNotice; notice != nil {
		state.TerminalNavigationNotice = &domain.MasterTerminalNavigationNotice{
			Reason: notice.Reason, SourceTerminalID: notice.SourceTerminalID,
			CommandID: notice.CommandID, TargetTerminalID: cloneString(notice.TargetTerminalID),
		}
	}
	return state
}

func playerSnapshot(runtime *domain.ProcessRuntime, sessionID domain.LogicalSessionID) (*domain.PlayerState, bool) {
	session := runtime.SessionsByID[sessionID]
	if session == nil {
		return nil, false
	}
	state := &domain.PlayerState{
		Revision:     runtime.Revision,
		SessionID:    session.ID,
		FallbackName: session.FallbackName,
		Role:         roleForSession(runtime.Broadcast, session.ID),
	}
	if session.Notice != nil {
		notice := *session.Notice
		state.Notice = &notice
	}
	if runtime.Broadcast == nil {
		state.Phase = domain.PlayerPhaseNoBroadcast
	} else {
		state.BroadcastID = runtime.Broadcast.ID
		if runtime.Broadcast.ActiveTerminalID != nil {
			state.ActiveTerminalID = *runtime.Broadcast.ActiveTerminalID
		}
		character := assignedCharacter(runtime, session.ID)
		if character == nil {
			state.Phase = domain.PlayerPhaseSelecting
		} else {
			state.Character = &domain.PlayerCharacter{ID: character.ID, Name: character.Name}
			switch {
			case runtime.Broadcast.ActiveTerminalID == nil:
				state.Phase = domain.PlayerPhaseWaiting
			case state.Role == domain.PlayerRoleActive:
				state.Phase = domain.PlayerPhaseControlling
			default:
				state.Phase = domain.PlayerPhaseObserving
			}
		}
	}
	for _, characterID := range orderedRosterIDs(runtime) {
		character := runtime.RosterByID[characterID]
		if character == nil {
			continue
		}
		status := domain.RosterStatusAvailable
		if runtime.Broadcast != nil {
			if _, claimed := runtime.Broadcast.SessionByCharacter[character.ID]; claimed {
				status = domain.RosterStatusClaimed
			}
		}
		state.Roster = append(state.Roster, domain.PlayerRosterEntry{
			ID: character.ID, Name: character.Name, Status: status,
		})
	}
	return state, true
}

func orderedRosterIDs(runtime *domain.ProcessRuntime) []domain.CharacterID {
	ids := make([]domain.CharacterID, 0, len(runtime.RosterByID))
	seen := make(map[domain.CharacterID]struct{}, len(runtime.RosterByID))
	for _, characterID := range runtime.RosterOrder {
		if _, exists := runtime.RosterByID[characterID]; !exists {
			continue
		}
		if _, duplicate := seen[characterID]; duplicate {
			continue
		}
		seen[characterID] = struct{}{}
		ids = append(ids, characterID)
	}
	var remainder []domain.CharacterID
	for characterID := range runtime.RosterByID {
		if _, exists := seen[characterID]; !exists {
			remainder = append(remainder, characterID)
		}
	}
	slices.Sort(remainder)
	return append(ids, remainder...)
}

func cloneRosterState(runtime *domain.ProcessRuntime) (map[domain.CharacterID]*domain.CharacterRosterEntry, []domain.CharacterID) {
	byID := make(map[domain.CharacterID]*domain.CharacterRosterEntry, len(runtime.RosterByID))
	for characterID, character := range runtime.RosterByID {
		if character == nil {
			continue
		}
		value := *character
		byID[characterID] = &value
	}
	return byID, append([]domain.CharacterID(nil), runtime.RosterOrder...)
}

func rosterEntries(byID map[domain.CharacterID]*domain.CharacterRosterEntry, order []domain.CharacterID) []domain.CharacterRosterEntry {
	runtime := &domain.ProcessRuntime{RosterByID: byID, RosterOrder: order}
	entries := make([]domain.CharacterRosterEntry, 0, len(byID))
	for _, characterID := range orderedRosterIDs(runtime) {
		if character := byID[characterID]; character != nil {
			entries = append(entries, *character)
		}
	}
	return entries
}

func (service *Service) persistRoster(runtime *domain.ProcessRuntime, byID map[domain.CharacterID]*domain.CharacterRosterEntry, order []domain.CharacterID) (domain.PlayerConfigHandle, error) {
	if !service.requirePlayerConfig {
		if runtime.ActivePlayerConfig == nil {
			return domain.PlayerConfigHandle{}, nil
		}
		return *runtime.ActivePlayerConfig, nil
	}
	if runtime.ActivePlayerConfig == nil || service.rosterStore == nil {
		return domain.PlayerConfigHandle{}, fmt.Errorf("no active player config")
	}
	return service.rosterStore.Save(*runtime.ActivePlayerConfig, rosterEntries(byID, order))
}

func roleForSession(broadcast *domain.LiveBroadcast, sessionID domain.LogicalSessionID) domain.PlayerRole {
	if broadcast == nil {
		return domain.PlayerRoleUnassigned
	}
	if _, assigned := broadcast.AssignmentsBySession[sessionID]; !assigned {
		return domain.PlayerRoleUnassigned
	}
	if broadcast.ControllerSessionID != nil && *broadcast.ControllerSessionID == sessionID {
		return domain.PlayerRoleActive
	}
	return domain.PlayerRoleObserver
}

func assignedCharacter(runtime *domain.ProcessRuntime, sessionID domain.LogicalSessionID) *domain.CharacterRosterEntry {
	if runtime.Broadcast == nil {
		return nil
	}
	characterID, assigned := runtime.Broadcast.AssignmentsBySession[sessionID]
	if !assigned {
		return nil
	}
	return runtime.RosterByID[characterID]
}

func detachEffect(effect Effect, revision uint64) Effect {
	detached := effect
	detached.Revision = revision
	detached.Master = domain.CloneMasterCoordinationState(effect.Master)
	if detached.Master != nil {
		detached.Master.Revision = revision
	}
	detached.Player = domain.ClonePlayerState(effect.Player)
	if detached.Player != nil {
		detached.Player.Revision = revision
	}
	detached.Live = clonePublicLiveState(effect.Live)
	detached.Hack = clonePublicHackState(effect.Hack)
	detached.Update = domain.CloneCompoundUpdate(effect.Update)
	detached.Audit = append([]AuditEvent(nil), effect.Audit...)
	if effect.Result != nil {
		result := *effect.Result
		if result.Revision == 0 {
			result.Revision = revision
		}
		detached.Result = &result
	}
	return detached
}

func (change *transition) audit(event AuditEvent) {
	if event.Name == "" {
		return
	}
	change.effects = append(change.effects, Effect{Audit: []AuditEvent{event}})
}

func deriveAuditEvents(before, after *domain.ProcessRuntime) []AuditEvent {
	var events []AuditEvent
	for _, sessionID := range sortedSessionIDs(after) {
		current := after.SessionsByID[sessionID]
		previous := before.SessionsByID[sessionID]
		wasConnected := previous != nil && len(previous.ConnectionIDs) > 0
		connected := current != nil && len(current.ConnectionIDs) > 0
		previousRole := roleForSession(before.Broadcast, sessionID)
		role := roleForSession(after.Broadcast, sessionID)
		if !wasConnected && connected {
			events = append(events, AuditEvent{Name: "player.connected", Outcome: "connected", SessionID: sessionID, Role: role})
		}
		if wasConnected && !connected {
			events = append(events, AuditEvent{Name: "player.disconnected", Outcome: "disconnected", SessionID: sessionID, Role: previousRole})
		}
		if connected && previous != nil && previousRole != role {
			events = append(events, AuditEvent{Name: "player.role_changed", Outcome: "selected", SessionID: sessionID, Role: role, PreviousRole: previousRole})
		}
	}
	events = append(events, deriveHackLifecycleEvents(before, after)...)
	return events
}

func commandDecisionAudit(pending *domain.PendingCommandExecution, decision domain.CommandExecutionDecision, outcome string) AuditEvent {
	if pending == nil {
		return AuditEvent{Name: "command.decision", Decision: string(decision), Outcome: outcome}
	}
	return AuditEvent{
		Name: "command.decision", Decision: string(decision), Outcome: outcome,
		SessionID: pending.ControllerSessionID, RequestID: pending.RequestID,
		BroadcastID: pending.BroadcastID, TerminalID: pending.TerminalID,
		CommandID: pending.CommandID, Mode: string(pending.Mode),
	}
}

func supersededCommandAudit(before, after *domain.ProcessRuntime, effects []Effect) (AuditEvent, bool) {
	pending := before.PendingCommandExecution
	if pending == nil || after.PendingCommandExecution != nil {
		return AuditEvent{}, false
	}
	for _, effect := range effects {
		for _, event := range effect.Audit {
			if event.Name == "command.decision" && event.RequestID == pending.RequestID {
				return AuditEvent{}, false
			}
		}
	}
	event := commandDecisionAudit(pending, "", "superseded")
	event.Name = "command.request_outcome"
	return event, true
}

func deriveHackLifecycleEvents(before, after *domain.ProcessRuntime) []AuditEvent {
	oldTerminal, newTerminal := activeTerminalRuntime(before.Broadcast), activeTerminalRuntime(after.Broadcast)
	var oldHack, newHack *domain.HackState
	if oldTerminal != nil {
		oldHack = oldTerminal.Hack
	}
	if newTerminal != nil {
		newHack = newTerminal.Hack
	}
	oldTerminalID, newTerminalID := "", ""
	if oldTerminal != nil {
		oldTerminalID = oldTerminal.TerminalID
	}
	if newTerminal != nil {
		newTerminalID = newTerminal.TerminalID
	}
	if oldHack != nil && newHack != nil && oldTerminalID != newTerminalID {
		var events []AuditEvent
		if activeHack(oldHack) {
			event := hackLifecycleAudit(before, oldTerminal, oldHack, "hack.interrupted", "interrupted")
			event.Reason = hackInterruptionReason(before, after, oldTerminalID)
			events = append(events, event)
		}
		return append(events, hackLifecycleAudit(after, newTerminal, newHack, "hack.started", "started"))
	}
	if oldHack == nil && newHack != nil {
		return []AuditEvent{hackLifecycleAudit(after, newTerminal, newHack, "hack.started", "started")}
	}
	if oldHack != nil && newHack == nil {
		if !activeHack(oldHack) {
			return nil
		}
		event := hackLifecycleAudit(before, oldTerminal, oldHack, "hack.interrupted", "interrupted")
		event.Reason = hackInterruptionReason(before, after, oldTerminalID)
		return []AuditEvent{event}
	}
	if oldHack == nil || newHack == nil {
		return nil
	}
	if oldHack.GenerationID != newHack.GenerationID {
		event := hackLifecycleAudit(after, newTerminal, newHack, "hack.reset", "reset")
		event.PreviousPuzzleID = oldHack.GenerationID
		return []AuditEvent{event}
	}
	if activeHack(oldHack) && activeHack(newHack) && controllingSessionChanged(before, after) {
		event := hackLifecycleAudit(before, oldTerminal, oldHack, "hack.interrupted", "interrupted")
		event.Reason = hackInterruptionReason(before, after, oldTerminalID)
		return []AuditEvent{event}
	}
	var events []AuditEvent
	if !oldHack.Solved && newHack.Solved {
		events = append(events, hackLifecycleAudit(after, newTerminal, newHack, "hack.succeeded", "succeeded"))
	}
	if !oldHack.Failed && newHack.Failed {
		events = append(events, hackLifecycleAudit(after, newTerminal, newHack, "hack.failed", "failed"))
	}
	return events
}

func hackActionAudit(runtime *domain.ProcessRuntime, sessionID domain.LogicalSessionID, command domain.RuntimeCommand, before, after *domain.HackState) AuditEvent {
	name := "hack.guess"
	if command.Kind == domain.RuntimeCommandActivatePattern {
		name = "hack.pattern"
	}
	event := AuditEvent{Name: name, SessionID: sessionID, Role: roleForSession(runtime.Broadcast, sessionID), TerminalID: command.TerminalID}
	if runtime.Broadcast != nil {
		event.BroadcastID = runtime.Broadcast.ID
	}
	if before != nil {
		event.PuzzleID = before.GenerationID
		event.AttemptsLeft = before.AttemptsLeft
	}
	if after != nil {
		event.PuzzleID = after.GenerationID
		event.AttemptsLeft = after.AttemptsLeft
	}
	if name == "hack.pattern" {
		event.Outcome = "attempts-replenished"
		if before != nil && after != nil && len(after.WordsByID) < len(before.WordsByID) {
			event.Outcome = "dud-removed"
		}
		return event
	}
	switch {
	case after == nil:
		event.Outcome = "rejected"
	case after.Solved:
		event.Outcome = "succeeded"
	case after.Failed:
		event.Outcome = "failed"
	default:
		event.Outcome = "mismatch"
	}
	return event
}

func activeHack(state *domain.HackState) bool {
	return state != nil && !state.Solved && !state.Failed
}

func controllingSessionChanged(before, after *domain.ProcessRuntime) bool {
	if before == nil || before.Broadcast == nil || before.Broadcast.ControllerSessionID == nil ||
		after == nil || after.Broadcast == nil {
		return false
	}
	return after.Broadcast.ControllerSessionID == nil ||
		*before.Broadcast.ControllerSessionID != *after.Broadcast.ControllerSessionID
}

func hackLifecycleAudit(runtime *domain.ProcessRuntime, terminal *domain.TerminalRuntime, state *domain.HackState, name, outcome string) AuditEvent {
	event := AuditEvent{Name: name, Outcome: outcome}
	if terminal != nil {
		event.TerminalID = terminal.TerminalID
	}
	if state != nil {
		event.PuzzleID = state.GenerationID
		event.HackLevel = state.Level
		event.AttemptsMax = state.AttemptsMax
		event.AttemptsLeft = state.AttemptsLeft
	}
	if runtime != nil && runtime.Broadcast != nil {
		event.BroadcastID = runtime.Broadcast.ID
		if runtime.Broadcast.ControllerSessionID != nil {
			event.SessionID = *runtime.Broadcast.ControllerSessionID
			event.Role = roleForSession(runtime.Broadcast, event.SessionID)
		}
	}
	return event
}

func hackInterruptionReason(before, after *domain.ProcessRuntime, terminalID string) string {
	if after.Broadcast == nil {
		return "broadcast-ended"
	}
	if after.Broadcast.ActiveTerminalID == nil {
		return "terminal-cleared"
	}
	terminal := after.Broadcast.TerminalRuntimes[terminalID]
	if terminal == nil {
		return "terminal-discarded"
	}
	if terminal.Lifecycle == domain.TerminalLifecycleSuspended {
		return "terminal-suspended"
	}
	if controllingSessionChanged(before, after) {
		return "controller-unavailable"
	}
	return "terminal-suspended"
}

func cloneProcessRuntime(runtime *domain.ProcessRuntime) *domain.ProcessRuntime {
	if runtime == nil {
		value := newProcessRuntime()
		return &value
	}
	clone := *runtime
	clone.SessionsByID = make(map[domain.LogicalSessionID]*domain.LogicalSession, len(runtime.SessionsByID))
	for sessionID, session := range runtime.SessionsByID {
		clone.SessionsByID[sessionID] = cloneLogicalSession(session)
	}
	clone.SessionIDByBrowserToken = make(map[domain.BrowserToken]domain.LogicalSessionID, len(runtime.SessionIDByBrowserToken))
	maps.Copy(clone.SessionIDByBrowserToken, runtime.SessionIDByBrowserToken)
	clone.RosterByID = make(map[domain.CharacterID]*domain.CharacterRosterEntry, len(runtime.RosterByID))
	for characterID, character := range runtime.RosterByID {
		if character == nil {
			clone.RosterByID[characterID] = nil
			continue
		}
		value := *character
		clone.RosterByID[characterID] = &value
	}
	clone.RosterOrder = append([]domain.CharacterID(nil), runtime.RosterOrder...)
	if runtime.ActivePlayerConfig != nil {
		value := *runtime.ActivePlayerConfig
		clone.ActivePlayerConfig = &value
	}
	clone.Broadcast = cloneBroadcast(runtime.Broadcast)
	clone.PendingSwitch = clonePendingSwitch(runtime.PendingSwitch)
	if runtime.PendingCommandExecution != nil {
		pending := *runtime.PendingCommandExecution
		clone.PendingCommandExecution = &pending
	}
	if runtime.PendingTerminalNavigation != nil {
		pending := *runtime.PendingTerminalNavigation
		pending.ReturnPoint.AncestorFolderIDs = append([]string(nil), runtime.PendingTerminalNavigation.ReturnPoint.AncestorFolderIDs...)
		clone.PendingTerminalNavigation = &pending
	}
	if runtime.TerminalNavigationNotice != nil {
		notice := *runtime.TerminalNavigationNotice
		notice.TargetTerminalID = cloneString(runtime.TerminalNavigationNotice.TargetTerminalID)
		clone.TerminalNavigationNotice = &notice
	}
	return &clone
}

func cloneLogicalSession(session *domain.LogicalSession) *domain.LogicalSession {
	if session == nil {
		return nil
	}
	clone := *session
	clone.ConnectionIDs = make(map[domain.ConnectionID]struct{}, len(session.ConnectionIDs))
	for connectionID := range session.ConnectionIDs {
		clone.ConnectionIDs[connectionID] = struct{}{}
	}
	clone.RequestResults = make(map[domain.RequestID]domain.RequestResultRecord, len(session.RequestResults))
	maps.Copy(clone.RequestResults, session.RequestResults)
	if session.Notice != nil {
		notice := *session.Notice
		clone.Notice = &notice
	}
	return &clone
}

func cloneBroadcast(broadcast *domain.LiveBroadcast) *domain.LiveBroadcast {
	if broadcast == nil {
		return nil
	}
	clone := *broadcast
	clone.AssignmentsBySession = make(map[domain.LogicalSessionID]domain.CharacterID, len(broadcast.AssignmentsBySession))
	maps.Copy(clone.AssignmentsBySession, broadcast.AssignmentsBySession)
	clone.SessionByCharacter = make(map[domain.CharacterID]domain.LogicalSessionID, len(broadcast.SessionByCharacter))
	maps.Copy(clone.SessionByCharacter, broadcast.SessionByCharacter)
	clone.ControllerSessionID = cloneLogicalSessionID(broadcast.ControllerSessionID)
	clone.ActiveTerminalID = cloneString(broadcast.ActiveTerminalID)
	clone.TerminalRuntimes = make(map[string]*domain.TerminalRuntime, len(broadcast.TerminalRuntimes))
	for terminalID, runtime := range broadcast.TerminalRuntimes {
		clone.TerminalRuntimes[terminalID] = cloneTerminalRuntime(runtime)
	}
	clone.Route = make([]domain.TerminalReturnPoint, len(broadcast.Route))
	for index, point := range broadcast.Route {
		clone.Route[index] = point
		clone.Route[index].AncestorFolderIDs = append([]string(nil), point.AncestorFolderIDs...)
	}
	return &clone
}

func clonePendingSwitch(pending *domain.TerminalSwitchDecision) *domain.TerminalSwitchDecision {
	if pending == nil {
		return nil
	}
	clone := *pending
	if pending.Target != nil {
		clone.Target = cloneTerminalTarget(pending.Target)
	}
	return &clone
}

func cloneTerminalTarget(target *domain.TerminalTarget) *domain.TerminalTarget {
	if target == nil {
		return nil
	}
	clone := *target
	clone.Tree = domain.CloneContentNode(target.Tree)
	clone.CommandStates = cloneCommandStates(target.CommandStates)
	return &clone
}

func cloneTerminalRuntime(runtime *domain.TerminalRuntime) *domain.TerminalRuntime {
	if runtime == nil {
		return nil
	}
	clone := *runtime
	clone.Tree = domain.CloneContentNode(runtime.Tree)
	clone.CommandStates = cloneCommandStates(runtime.CommandStates)
	if runtime.CommandExecution != nil {
		execution := *runtime.CommandExecution
		clone.CommandExecution = &execution
	}
	clone.Nav = cloneNavState(runtime.Nav)
	clone.Hack = cloneHackState(runtime.Hack)
	return &clone
}

func clonePublicLiveState(state *domain.PublicLiveState) *domain.PublicLiveState {
	if state == nil {
		return nil
	}
	clone := *state
	clone.Tree = domain.CloneContentNode(state.Tree)
	clone.Nav = cloneNavState(state.Nav)
	clone.Hack = clonePublicHackState(state.Hack)
	if state.CommandExecution != nil {
		execution := *state.CommandExecution
		clone.CommandExecution = &execution
	}
	if state.TerminalNavigation != nil {
		navigation := *state.TerminalNavigation
		if state.TerminalNavigation.ReturnTarget != nil {
			value := *state.TerminalNavigation.ReturnTarget
			navigation.ReturnTarget = &value
		}
		if state.TerminalNavigation.Pending != nil {
			value := *state.TerminalNavigation.Pending
			navigation.Pending = &value
		}
		clone.TerminalNavigation = &navigation
	}
	return &clone
}

func cloneNavState(state domain.NavState) domain.NavState {
	clone := state
	clone.Path = append([]string(nil), state.Path...)
	clone.ViewEntryID = cloneString(state.ViewEntryID)
	clone.CommandNodeID = cloneString(state.CommandNodeID)
	return clone
}

func cloneHackState(state *domain.HackState) *domain.HackState {
	if state == nil {
		return nil
	}
	clone := *state
	if state.WordsByID != nil {
		clone.WordsByID = make(map[string]domain.HackCandidate, len(state.WordsByID))
		maps.Copy(clone.WordsByID, state.WordsByID)
	}
	if state.UsedPatterns != nil {
		clone.UsedPatterns = make(map[domain.HackPatternIdentity]struct{}, len(state.UsedPatterns))
		for identity := range state.UsedPatterns {
			clone.UsedPatterns[identity] = struct{}{}
		}
	}
	if state.Log != nil {
		clone.Log = append([]string{}, state.Log...)
	}
	clone.Columns = cloneHackColumns(state.Columns)
	return &clone
}

func clonePublicHackState(state *domain.PublicHackState) *domain.PublicHackState {
	if state == nil {
		return nil
	}
	clone := *state
	if state.Log != nil {
		clone.Log = append([]string{}, state.Log...)
	}
	clone.Columns = cloneHackColumns(state.Columns)
	clone.Patterns = append([]domain.PublicHackPattern(nil), state.Patterns...)
	return &clone
}

func cloneHackColumns(columns []domain.HackColumn) []domain.HackColumn {
	if columns == nil {
		return nil
	}
	clone := make([]domain.HackColumn, len(columns))
	for index, column := range columns {
		clone[index] = column
		if column.Addresses != nil {
			clone[index].Addresses = append([]string(nil), column.Addresses...)
		}
		if column.Words != nil {
			clone[index].Words = append([]domain.HackWord(nil), column.Words...)
		}
	}
	return clone
}

func cloneLogicalSessionID(value *domain.LogicalSessionID) *domain.LogicalSessionID {
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
