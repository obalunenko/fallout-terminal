// Package session owns validated, user-selected session documents and their
// ordered durable saves.
package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
)

// SaveState is the user-visible persistence state of the active session.
type SaveState string

const (
	SaveStateIdle   SaveState = "idle"
	SaveStateSaving SaveState = "saving"
	SaveStateSaved  SaveState = "saved"
	SaveStateFailed SaveState = "failed"
)

var errContextRequired = errors.New("session context is required")

// Locations separates user session suggestions, the immutable bundled demo,
// and application-owned metadata. Session content is never written to
// ApplicationSupport.
type Locations struct {
	DocumentsDefault   string
	BundledDemo        string
	ApplicationSupport string
}

// Dialog is the native file-dialog boundary used by the session service. An
// empty path with no error means the user canceled the dialog.
type Dialog interface {
	OpenFile(defaultPath string) (string, error)
	SaveFile(defaultPath string) (string, error)
}

// Store is the durable storage boundary used by the session service.
type Store interface {
	Read(path string) ([]byte, error)
	WriteAtomic(path string, data []byte) error
	CopyAtomic(source, destination string) error
}

// TerminalCatalog exposes detached, validated snapshots from the currently
// active session without allowing callers to mutate persistence-owned state.
type TerminalCatalog interface {
	LookupTerminal(terminalID string) (domain.TerminalTarget, bool)
	LookupTerminalTransition(sourceTerminalID, commandID string) (domain.TerminalTransitionTarget, bool)
	LookupTerminalGroup(terminalID string) (domain.TerminalGroupSnapshot, bool)
}

// SessionResult is returned by create, open, and demo-copy commands.
type SessionResult struct {
	OK       bool            `json:"ok"`
	Canceled bool            `json:"canceled"`
	Error    string          `json:"error,omitempty"`
	FilePath string          `json:"filePath,omitempty"`
	Session  *domain.Session `json:"session,omitempty"`
}

// SaveResult reports both the caller's revision and the newest revision known
// to be durable when that caller completes.
type SaveResult struct {
	OK                bool   `json:"ok"`
	Error             string `json:"error,omitempty"`
	RequestedRevision uint64 `json:"requestedRevision"`
	SavedRevision     uint64 `json:"savedRevision,omitempty"`
}

// CommandStateResult reports one trusted ID-addressed durable mutation.
// Changed distinguishes a committed document replacement from an idempotent
// no-op; Revision is the newest durable document revision on success.
type CommandStateResult struct {
	OK       bool            `json:"ok"`
	Changed  bool            `json:"changed"`
	Error    string          `json:"error,omitempty"`
	Revision uint64          `json:"revision"`
	Session  *domain.Session `json:"session,omitempty"`
}

// TerminalGroupMutationResult reports one synchronous complete-set group
// replacement. Session is authoritative and detached on both success and
// rejection so callers can discard stale drafts safely.
type TerminalGroupMutationResult struct {
	OK       bool            `json:"ok"`
	Changed  bool            `json:"changed"`
	Error    string          `json:"error,omitempty"`
	Revision uint64          `json:"revision"`
	Session  *domain.Session `json:"session,omitempty"`
}

// ActiveSession is an immutable snapshot of the current user-owned document.
type ActiveSession struct {
	Path              string          `json:"path,omitempty"`
	Session           *domain.Session `json:"session,omitempty"`
	RequestedRevision uint64          `json:"requestedRevision"`
	SavedRevision     uint64          `json:"savedRevision"`
	SaveState         SaveState       `json:"saveState"`
}

type savePayload struct {
	epoch             uint64
	path              string
	revision          uint64
	session           domain.Session
	data              []byte
	rollbackOnFailure bool
	priorSession      *domain.Session
	priorRevision     uint64
}

type saveWaiter struct {
	epoch             uint64
	revision          uint64
	requestedRevision uint64
	reply             chan SaveResult
}

// Service coordinates dialogs, validation, active-path ownership, and one
// serialized writer. The writer may coalesce queued revisions for the same
// document, but every accepted caller waits until its revision (or a newer
// one) is durable.
type Service struct {
	store     Store
	dialog    Dialog
	locations Locations

	commandMu  sync.Mutex
	documentMu sync.Mutex
	mu         sync.Mutex
	active     ActiveSession
	epoch      uint64
	closed     bool

	pending    []savePayload
	waiters    []saveWaiter
	durable    map[uint64]uint64
	wake       chan struct{}
	workerDone chan struct{}
}

// NewService starts an idle session service and its serialized save worker.
func NewService(store Store, dialog Dialog, locations Locations) *Service {
	service := &Service{
		store:      store,
		dialog:     dialog,
		locations:  locations,
		active:     ActiveSession{SaveState: SaveStateIdle},
		durable:    make(map[uint64]uint64),
		wake:       make(chan struct{}, 1),
		workerDone: make(chan struct{}),
	}
	go service.runSaveWorker()
	return service
}

// LookupTerminal returns the latest detached target authored in the active session.
func (service *Service) LookupTerminal(terminalID string) (domain.TerminalTarget, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	terminal := terminalByID(service.active.Session, terminalID)
	if terminal == nil {
		return domain.TerminalTarget{}, false
	}
	return terminalTarget(*terminal), true
}

// LookupTerminalGroup returns the latest detached ordered group containing terminalID.
func (service *Service) LookupTerminalGroup(terminalID string) (domain.TerminalGroupSnapshot, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.active.Session == nil {
		return domain.TerminalGroupSnapshot{}, false
	}
	return domain.TerminalGroupFor(*service.active.Session, terminalID)
}

// LookupTerminalTransition resolves a command link and target from one locked
// current-session snapshot, so source and destination cannot come from different revisions.
func (service *Service) LookupTerminalTransition(sourceTerminalID, commandID string) (domain.TerminalTransitionTarget, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	source := terminalByID(service.active.Session, sourceTerminalID)
	if source == nil {
		return domain.TerminalTransitionTarget{}, false
	}
	command := contentNodeByID(&source.Root, commandID)
	if command == nil || command.Type != domain.NodeCommand || command.TerminalTransition == nil {
		return domain.TerminalTransitionTarget{}, false
	}
	target := terminalByID(service.active.Session, command.TerminalTransition.TargetTerminalID)
	if target == nil || target.ID == source.ID {
		return domain.TerminalTransitionTarget{}, false
	}
	sourceGroup, sourceGrouped := domain.TerminalGroupFor(*service.active.Session, source.ID)
	targetGroup, targetGrouped := domain.TerminalGroupFor(*service.active.Session, target.ID)
	if !sourceGrouped || !targetGrouped || sourceGroup.ID != targetGroup.ID {
		return domain.TerminalTransitionTarget{}, false
	}
	return domain.TerminalTransitionTarget{
		SourceTerminalID:   source.ID,
		SourceTerminalName: source.Name,
		CommandID:          command.ID,
		CommandName:        command.Name,
		Target:             terminalTarget(*target),
	}, true
}

func terminalTarget(terminal domain.Terminal) domain.TerminalTarget {
	cloned := domain.CloneSession(domain.Session{Version: 1, Name: "catalog", Terminals: []domain.Terminal{terminal}}).Terminals[0]
	return domain.TerminalTarget{
		TerminalID: cloned.ID, TerminalName: cloned.Name, Tree: cloned.Root,
		CommandStates: cloned.CommandStates, HackLevel: cloned.HackLevel, IntroText: cloned.IntroText,
	}
}

// Create asks for an explicit destination, writes a valid starter document,
// and activates it only after the write succeeds.
func (service *Service) Create(ctx context.Context) SessionResult {
	service.commandMu.Lock()
	defer service.commandMu.Unlock()

	if err := service.available(ctx); err != nil {
		return sessionFailure("new session is unavailable")
	}
	if service.dialog == nil {
		return sessionFailure("new session dialog is unavailable")
	}

	defaultPath := filepath.Join(service.locations.DocumentsDefault, "session.json")
	target, err := service.dialog.SaveFile(defaultPath)
	if err != nil {
		return sessionFailure("could not choose a new session destination")
	}
	if target == "" {
		return SessionResult{Canceled: true}
	}
	if err := contextError(ctx); err != nil {
		return sessionFailure("new session was canceled")
	}
	target, err = service.writableTarget(target)
	if err != nil {
		return sessionFailure(err.Error())
	}

	created := starterSession(sessionNameFromPath(target))
	if err := verifySessionContract(created); err != nil {
		return sessionFailure("could not verify the new session contract")
	}
	data, err := domain.EncodeSession(created)
	if err != nil {
		return sessionFailure("could not prepare the new session")
	}
	if service.store == nil {
		return sessionFailure("session storage is unavailable")
	}
	if err := service.store.WriteAtomic(target, data); err != nil {
		return sessionFailure("could not write the new session")
	}
	return service.activate(target, created)
}

// Open asks for an explicit document and activates it only after the complete
// version-1 document has decoded and validated successfully.
func (service *Service) Open(ctx context.Context) SessionResult {
	service.commandMu.Lock()
	defer service.commandMu.Unlock()

	if err := service.available(ctx); err != nil {
		return sessionFailure("open session is unavailable")
	}
	if service.dialog == nil {
		return sessionFailure("open session dialog is unavailable")
	}

	target, err := service.dialog.OpenFile(service.locations.DocumentsDefault)
	if err != nil {
		return sessionFailure("could not choose a session to open")
	}
	if target == "" {
		return SessionResult{Canceled: true}
	}
	if err := contextError(ctx); err != nil {
		return sessionFailure("open session was canceled")
	}
	target, err = service.userDocumentPath(target)
	if err != nil {
		return sessionFailure(err.Error())
	}
	if samePath(target, service.locations.BundledDemo) {
		return sessionFailure("the bundled demo is read-only; copy it before editing")
	}
	if service.store == nil {
		return sessionFailure("session storage is unavailable")
	}

	data, err := service.store.Read(target)
	if err != nil {
		return sessionFailure("could not read the selected session")
	}
	opened, err := domain.DecodeSession(data)
	if err != nil {
		return sessionFailure("the selected file is not a valid version-1 session")
	}
	if err := verifySessionContract(opened); err != nil {
		return sessionFailure("the selected file does not satisfy the session contract")
	}
	return service.activate(target, opened)
}

// CopyDemo validates the immutable bundled sample, asks for a distinct
// destination, and activates only the writable copy.
func (service *Service) CopyDemo(ctx context.Context) SessionResult {
	service.commandMu.Lock()
	defer service.commandMu.Unlock()

	if err := service.available(ctx); err != nil {
		return sessionFailure("demo copy is unavailable")
	}
	if service.dialog == nil {
		return sessionFailure("demo copy dialog is unavailable")
	}

	defaultPath := filepath.Join(service.locations.DocumentsDefault, "demo.json")
	destination, err := service.dialog.SaveFile(defaultPath)
	if err != nil {
		return sessionFailure("could not choose a demo destination")
	}
	if destination == "" {
		return SessionResult{Canceled: true}
	}
	if err := contextError(ctx); err != nil {
		return sessionFailure("demo copy was canceled")
	}
	destination, err = service.writableTarget(destination)
	if err != nil {
		return sessionFailure(err.Error())
	}
	if service.store == nil {
		return sessionFailure("session storage is unavailable")
	}

	data, err := service.store.Read(service.locations.BundledDemo)
	if err != nil {
		return sessionFailure("could not read the bundled demo")
	}
	demo, err := domain.DecodeSession(data)
	if err != nil {
		return sessionFailure("the bundled demo is not a valid version-1 session")
	}
	if err := verifySessionContract(demo); err != nil {
		return sessionFailure("the bundled demo does not satisfy the session contract")
	}
	if demo.PlayerConfig != "" {
		source, destinationConfig, err := demoCompanionPaths(service.locations.BundledDemo, destination, demo.PlayerConfig)
		if err != nil {
			return sessionFailure("the bundled demo player config reference is invalid")
		}
		configData, err := service.store.Read(source)
		if err != nil {
			return sessionFailure("could not read the bundled demo player config")
		}
		if _, err := domain.DecodePlayerConfig(configData); err != nil {
			return sessionFailure("the bundled demo player config is not valid version 1")
		}
		if err := service.store.CopyAtomic(source, destinationConfig); err != nil {
			return sessionFailure("could not create a writable demo player config copy")
		}
	}
	if err := service.store.CopyAtomic(service.locations.BundledDemo, destination); err != nil {
		return sessionFailure("could not create a writable demo copy")
	}
	return service.activate(destination, demo)
}

func demoCompanionPaths(bundledSession, destination, reference string) (string, string, error) {
	reference = filepath.Clean(reference)
	if reference == "." || filepath.IsAbs(reference) || reference == ".." || strings.HasPrefix(reference, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("demo companion reference must stay beside the session")
	}
	sourceDirectory := filepath.Dir(bundledSession)
	destinationDirectory := filepath.Dir(destination)
	source := filepath.Join(sourceDirectory, reference)
	destinationConfig := filepath.Join(destinationDirectory, reference)
	if samePath(source, bundledSession) || samePath(destinationConfig, destination) {
		return "", "", fmt.Errorf("demo companion must differ from the session")
	}
	return source, destinationConfig, nil
}

// Save validates and accepts a monotonically increasing revision, then waits
// for the serialized writer to make that revision or a newer one durable.
func (service *Service) Save(ctx context.Context, session domain.Session, revision uint64) SaveResult {
	if err := contextError(ctx); err != nil {
		return saveFailure(revision, 0, "save was canceled")
	}
	if revision == 0 {
		return saveFailure(revision, 0, "save revision must be greater than zero")
	}
	authored := cloneSession(session)
	clearCommandStates(&authored)
	if err := verifyAuthoredSession(authored); err != nil {
		return saveFailure(revision, 0, "session is invalid and was not saved")
	}

	reply := make(chan SaveResult, 1)
	service.documentMu.Lock()
	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		service.documentMu.Unlock()
		return saveFailure(revision, 0, "session service is shut down")
	}
	if service.active.Path == "" || service.active.Session == nil {
		saved := service.active.SavedRevision
		service.mu.Unlock()
		service.documentMu.Unlock()
		return saveFailure(revision, saved, "there is no active session path")
	}
	if samePath(service.active.Path, service.locations.BundledDemo) {
		saved := service.active.SavedRevision
		service.mu.Unlock()
		service.documentMu.Unlock()
		return saveFailure(revision, saved, "the bundled demo cannot be saved in place")
	}

	accepted, err := mergeCanonicalSessionState(authored, *service.active.Session)
	if err != nil {
		saved := service.active.SavedRevision
		service.mu.Unlock()
		service.documentMu.Unlock()
		return saveFailure(revision, saved, "session is invalid and was not saved")
	}
	data, err := encodeAcceptedSession(accepted)
	if err != nil {
		saved := service.active.SavedRevision
		service.mu.Unlock()
		service.documentMu.Unlock()
		return saveFailure(revision, saved, "session is invalid and was not saved")
	}
	epoch := service.epoch
	if saved := service.durable[epoch]; saved >= revision && sessionsEqual(accepted, *service.active.Session) {
		service.mu.Unlock()
		service.documentMu.Unlock()
		return SaveResult{OK: true, RequestedRevision: revision, SavedRevision: saved}
	}

	effectiveRevision := revision
	if effectiveRevision <= service.active.RequestedRevision {
		effectiveRevision = service.active.RequestedRevision + 1
	}
	service.active.RequestedRevision = effectiveRevision
	service.active.Session = sessionPointer(accepted)
	service.active.SaveState = SaveStateSaving
	service.pending = append(service.pending, savePayload{
		epoch: epoch, path: service.active.Path, revision: effectiveRevision,
		session: accepted, data: append([]byte(nil), data...),
	})
	service.waiters = append(service.waiters, saveWaiter{
		epoch: epoch, revision: effectiveRevision, requestedRevision: revision, reply: reply,
	})
	service.signalWorkerLocked()
	service.mu.Unlock()
	service.documentMu.Unlock()

	return <-reply
}

// ExecuteCommandState captures one state-changing command's current authored
// completed name and result text, then waits until the new document revision
// is durably replaced. An existing snapshot is an idempotent no-op.
func (service *Service) ExecuteCommandState(ctx context.Context, terminalID, commandID string) CommandStateResult {
	return service.mutateCommandStates(ctx, func(candidate *domain.Session) (bool, error) {
		terminal := terminalByID(candidate, terminalID)
		if terminal == nil {
			return false, fmt.Errorf("terminal does not exist")
		}
		command := contentNodeByID(&terminal.Root, commandID)
		if command == nil || command.Type != domain.NodeCommand || command.StateChange == nil {
			return false, fmt.Errorf("state-changing command does not exist")
		}
		if _, exists := terminal.CommandStates[commandID]; exists {
			return false, nil
		}
		if terminal.CommandStates == nil {
			terminal.CommandStates = make(map[string]domain.CommandExecutionState)
		}
		state := domain.CommandExecutionState{
			CompletedName: command.StateChange.CompletedName,
			ResultText:    command.Text,
		}
		if change := command.StateChange.EntryContentChange; change != nil {
			state.EntryContentChange = &domain.EntryContentChange{
				BlockID:       change.BlockID,
				CompletedText: change.CompletedText,
			}
		}
		terminal.CommandStates[commandID] = state
		return true, nil
	})
}

// ResetCommandState removes only one durable snapshot. A valid command that
// is already in its initial state is an idempotent no-op without a file write.
func (service *Service) ResetCommandState(ctx context.Context, terminalID, commandID string) CommandStateResult {
	return service.mutateCommandStates(ctx, func(candidate *domain.Session) (bool, error) {
		terminal := terminalByID(candidate, terminalID)
		if terminal == nil {
			return false, fmt.Errorf("terminal does not exist")
		}
		command := contentNodeByID(&terminal.Root, commandID)
		if command == nil || command.Type != domain.NodeCommand || command.StateChange == nil {
			return false, fmt.Errorf("state-changing command does not exist")
		}
		if _, exists := terminal.CommandStates[commandID]; !exists {
			return false, nil
		}
		delete(terminal.CommandStates, commandID)
		if len(terminal.CommandStates) == 0 {
			terminal.CommandStates = nil
		}
		return true, nil
	})
}

// ResetTerminalCommandStates removes all durable command snapshots belonging
// to one terminal in a single document revision.
func (service *Service) ResetTerminalCommandStates(ctx context.Context, terminalID string) CommandStateResult {
	return service.mutateCommandStates(ctx, func(candidate *domain.Session) (bool, error) {
		terminal := terminalByID(candidate, terminalID)
		if terminal == nil {
			return false, fmt.Errorf("terminal does not exist")
		}
		if len(terminal.CommandStates) == 0 {
			return false, nil
		}
		terminal.CommandStates = nil
		return true, nil
	})
}

func (service *Service) mutateCommandStates(ctx context.Context, mutate func(*domain.Session) (bool, error)) CommandStateResult {
	if err := contextError(ctx); err != nil {
		return commandStateFailure(0, "command state mutation was canceled")
	}
	service.commandMu.Lock()
	defer service.commandMu.Unlock()
	service.documentMu.Lock()
	defer service.documentMu.Unlock()

	reply := make(chan SaveResult, 1)
	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		return commandStateFailure(0, "session service is shut down")
	}
	if service.active.Path == "" || service.active.Session == nil {
		revision := service.active.SavedRevision
		service.mu.Unlock()
		return commandStateFailure(revision, "there is no active session path")
	}
	if samePath(service.active.Path, service.locations.BundledDemo) {
		revision := service.active.SavedRevision
		service.mu.Unlock()
		return commandStateFailure(revision, "the bundled demo cannot be changed in place")
	}

	prior := cloneSession(*service.active.Session)
	candidate := cloneSession(prior)
	changed, err := mutate(&candidate)
	if err != nil {
		revision := service.active.SavedRevision
		service.mu.Unlock()
		return commandStateFailure(revision, err.Error())
	}
	if !changed {
		result := CommandStateResult{
			OK: true, Revision: service.active.SavedRevision, Session: sessionPointer(candidate),
		}
		service.mu.Unlock()
		return result
	}
	data, err := encodeAcceptedSession(candidate)
	if err != nil {
		revision := service.active.SavedRevision
		service.mu.Unlock()
		return commandStateFailure(revision, "command state mutation is invalid")
	}

	epoch := service.epoch
	priorRevision := service.active.RequestedRevision
	revision := priorRevision + 1
	if revision <= service.active.SavedRevision {
		revision = service.active.SavedRevision + 1
	}
	service.active.RequestedRevision = revision
	service.active.Session = sessionPointer(candidate)
	service.active.SaveState = SaveStateSaving
	service.pending = append(service.pending, savePayload{
		epoch: epoch, path: service.active.Path, revision: revision,
		session: candidate, data: append([]byte(nil), data...),
		rollbackOnFailure: true, priorSession: sessionPointer(prior), priorRevision: priorRevision,
	})
	service.waiters = append(service.waiters, saveWaiter{
		epoch: epoch, revision: revision, requestedRevision: revision, reply: reply,
	})
	service.signalWorkerLocked()
	service.mu.Unlock()

	saved := <-reply
	if !saved.OK {
		return commandStateFailure(saved.SavedRevision, "could not save the command state")
	}
	active := service.Snapshot()
	return CommandStateResult{
		OK: true, Changed: true, Revision: saved.SavedRevision, Session: active.Session,
	}
}

// ReplaceTerminalGroups synchronously compares and replaces the complete
// canonical group set. It is the narrow trusted persistence capability used
// by the coordinator while holding its own transition lock.
func (service *Service) ReplaceTerminalGroups(
	ctx context.Context,
	groups []domain.TerminalGroup,
	expectedRevision uint64,
) TerminalGroupMutationResult {
	if err := contextError(ctx); err != nil {
		return terminalGroupMutationFailure(0, nil, "terminal group replacement was canceled")
	}
	service.commandMu.Lock()
	defer service.commandMu.Unlock()
	service.documentMu.Lock()
	defer service.documentMu.Unlock()

	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		return terminalGroupMutationFailure(0, nil, "session service is shut down")
	}
	if service.active.Path == "" || service.active.Session == nil {
		revision := service.active.SavedRevision
		service.mu.Unlock()
		return terminalGroupMutationFailure(revision, nil, "there is no active session path")
	}
	canonical := cloneSession(*service.active.Session)
	revision := service.active.SavedRevision
	path := service.active.Path
	if expectedRevision != revision {
		service.mu.Unlock()
		return terminalGroupMutationFailure(revision, &canonical, "session revision changed; review the latest terminal groups")
	}
	if samePath(path, service.locations.BundledDemo) {
		service.mu.Unlock()
		return terminalGroupMutationFailure(revision, &canonical, "the bundled demo cannot be changed in place")
	}
	service.mu.Unlock()

	diff, err := domain.ValidateTerminalGroupReplacement(canonical, groups)
	if err != nil {
		return terminalGroupMutationFailure(revision, &canonical, err.Error())
	}
	if !diff.Changed {
		return TerminalGroupMutationResult{OK: true, Revision: revision, Session: sessionPointer(canonical)}
	}
	candidate := cloneSession(canonical)
	candidate.TerminalGroups = domain.CloneTerminalGroups(groups)
	data, err := encodeAcceptedSession(candidate)
	if err != nil {
		return terminalGroupMutationFailure(revision, &canonical, "terminal group candidate is invalid")
	}
	if service.store == nil || service.store.WriteAtomic(path, data) != nil {
		return terminalGroupMutationFailure(revision, &canonical, "could not save terminal groups")
	}

	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.active.Path != path || service.active.SavedRevision != revision {
		return terminalGroupMutationFailure(service.active.SavedRevision, service.active.Session, "active session changed while saving terminal groups")
	}
	nextRevision := revision + 1
	service.active.Session = sessionPointer(candidate)
	service.active.RequestedRevision = nextRevision
	service.active.SavedRevision = nextRevision
	service.active.SaveState = SaveStateSaved
	service.durable[service.epoch] = nextRevision
	return TerminalGroupMutationResult{
		OK: true, Changed: true, Revision: nextRevision, Session: sessionPointer(candidate),
	}
}

// AssociatePlayerConfig atomically saves a normalized relative reference and
// updates the active session only after the durable replacement succeeds.
func (service *Service) AssociatePlayerConfig(ctx context.Context, playerConfigPath string) SessionResult {
	service.commandMu.Lock()
	defer service.commandMu.Unlock()

	if err := contextError(ctx); err != nil {
		return sessionFailure("player config association was canceled")
	}
	service.mu.Lock()
	if service.closed || service.active.Path == "" || service.active.Session == nil {
		service.mu.Unlock()
		return sessionFailure("there is no active session")
	}
	active := cloneActive(service.active)
	epoch := service.epoch
	service.mu.Unlock()

	playerConfigPath = filepath.Clean(strings.TrimSpace(playerConfigPath))
	if !filepath.IsAbs(playerConfigPath) || playerConfigPath == string(filepath.Separator) {
		return sessionFailure("player config path must be absolute")
	}
	reference, err := filepath.Rel(filepath.Dir(active.Path), playerConfigPath)
	if err != nil || filepath.IsAbs(reference) || filepath.Clean(reference) == "." {
		return sessionFailure("could not create a relative player config reference")
	}
	candidate := cloneSession(*active.Session)
	candidate.PlayerConfig = filepath.Clean(reference)
	if err := verifySessionContract(candidate); err != nil {
		return sessionFailure("could not verify the player config association")
	}
	data, err := domain.EncodeSession(candidate)
	if err != nil {
		return sessionFailure("could not prepare the player config association")
	}
	if service.store == nil || service.store.WriteAtomic(active.Path, data) != nil {
		return sessionFailure("could not save the player config association")
	}

	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.epoch != epoch || service.active.Path != active.Path {
		return sessionFailure("active session changed while saving the player config association")
	}
	revision := service.active.RequestedRevision + 1
	if revision <= service.active.SavedRevision {
		revision = service.active.SavedRevision + 1
	}
	service.active.Session = sessionPointer(candidate)
	service.active.RequestedRevision = revision
	service.active.SavedRevision = revision
	service.active.SaveState = SaveStateSaved
	service.durable[epoch] = revision
	return SessionResult{OK: true, FilePath: active.Path, Session: sessionPointer(candidate)}
}

// Snapshot returns detached state safe for concurrent callers to inspect.
func (service *Service) Snapshot() ActiveSession {
	if service == nil {
		return ActiveSession{SaveState: SaveStateIdle}
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	return cloneActive(service.active)
}

// Shutdown rejects new work, drains every already accepted save, and is safe
// to call repeatedly or concurrently.
func (service *Service) Shutdown(ctx context.Context) error {
	if service == nil {
		return nil
	}
	if ctx == nil {
		return errContextRequired
	}
	service.mu.Lock()
	service.closed = true
	service.signalWorkerLocked()
	done := service.workerDone
	service.mu.Unlock()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (service *Service) runSaveWorker() {
	defer close(service.workerDone)
	for {
		<-service.wake
		for {
			payload, ok, stop := service.nextPayload()
			if stop {
				return
			}
			if !ok {
				break
			}

			var writeErr error
			if service.store == nil {
				writeErr = errors.New("session storage is unavailable")
			} else {
				writeErr = service.store.WriteAtomic(payload.path, payload.data)
			}
			service.finishPayload(payload, writeErr)
		}
	}
}

// nextPayload takes the newest queued revision for the oldest document epoch.
// This preserves path ordering while collapsing bursts of edits for one file.
func (service *Service) nextPayload() (savePayload, bool, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if len(service.pending) == 0 {
		return savePayload{}, false, service.closed
	}

	selected := service.pending[0]
	consumed := 1
	for consumed < len(service.pending) && service.pending[consumed].epoch == selected.epoch {
		selected = service.pending[consumed]
		consumed++
	}
	service.pending = append([]savePayload(nil), service.pending[consumed:]...)
	if service.epoch == selected.epoch {
		service.active.SaveState = SaveStateSaving
	}
	return selected, true, false
}

func (service *Service) finishPayload(payload savePayload, writeErr error) {
	service.mu.Lock()
	defer service.mu.Unlock()

	if writeErr == nil && payload.revision > service.durable[payload.epoch] {
		service.durable[payload.epoch] = payload.revision
	}
	durableRevision := service.durable[payload.epoch]
	if service.epoch == payload.epoch {
		service.active.SavedRevision = durableRevision
		if writeErr == nil {
			service.active.Session = sessionPointer(payload.session)
			if service.active.RequestedRevision <= durableRevision {
				service.active.SaveState = SaveStateSaved
			} else {
				service.active.SaveState = SaveStateSaving
			}
		} else if payload.rollbackOnFailure && service.active.RequestedRevision == payload.revision {
			service.active.Session = sessionPointer(*payload.priorSession)
			service.active.RequestedRevision = payload.priorRevision
			service.active.SaveState = SaveStateFailed
		} else {
			service.active.SaveState = SaveStateFailed
		}
	}

	remaining := service.waiters[:0]
	for _, waiter := range service.waiters {
		if waiter.epoch != payload.epoch || waiter.revision > payload.revision {
			remaining = append(remaining, waiter)
			continue
		}
		if durableRevision >= waiter.revision {
			waiter.reply <- SaveResult{
				OK:                true,
				RequestedRevision: waiter.requestedRevision,
				SavedRevision:     durableRevision,
			}
		} else {
			waiter.reply <- saveFailure(waiter.requestedRevision, durableRevision, "could not save the session")
		}
	}
	service.waiters = remaining
}

func (service *Service) activate(path string, session domain.Session) SessionResult {
	copy := domain.NormalizeTerminalGroups(session)
	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		return sessionFailure("session service is shut down")
	}
	service.epoch++
	service.active = ActiveSession{
		Path:      path,
		Session:   sessionPointer(copy),
		SaveState: SaveStateSaved,
	}
	service.durable[service.epoch] = 0
	service.mu.Unlock()
	return SessionResult{OK: true, FilePath: path, Session: sessionPointer(copy)}
}

func (service *Service) available(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed {
		return errors.New("session service is shut down")
	}
	return nil
}

func (service *Service) writableTarget(path string) (string, error) {
	path, err := service.userDocumentPath(path)
	if err != nil {
		return "", err
	}
	if samePath(path, service.locations.BundledDemo) {
		return "", errors.New("the bundled demo is read-only; choose another destination")
	}
	return path, nil
}

func (service *Service) userDocumentPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("session path is empty")
	}
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return "", errors.New("session path must be absolute")
	}
	if cleaned == string(filepath.Separator) {
		return "", errors.New("session path does not name a file")
	}
	return cleaned, nil
}

func (service *Service) signalWorkerLocked() {
	select {
	case service.wake <- struct{}{}:
	default:
	}
}

func starterSession(name string) domain.Session {
	return domain.NormalizeTerminalGroups(domain.Session{
		Version: 1,
		Name:    name,
		Terminals: []domain.Terminal{{
			ID:        "terminal-1",
			Name:      "Terminal 1",
			HackLevel: 0,
			IntroText: "",
			Root: domain.ContentNode{
				ID:       "root",
				Type:     domain.NodeFolder,
				Name:     "ROOT",
				Children: []domain.ContentNode{},
			},
		}},
	})
}

func sessionNameFromPath(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSpace(strings.TrimSuffix(base, filepath.Ext(base)))
	if name == "" {
		return "New Session"
	}
	return name
}

func samePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errContextRequired
	}
	return ctx.Err()
}

func sessionFailure(message string) SessionResult {
	return SessionResult{Error: message}
}

func saveFailure(requested, saved uint64, message string) SaveResult {
	return SaveResult{Error: message, RequestedRevision: requested, SavedRevision: saved}
}

func commandStateFailure(revision uint64, message string) CommandStateResult {
	return CommandStateResult{Error: message, Revision: revision}
}

func terminalGroupMutationFailure(revision uint64, session *domain.Session, message string) TerminalGroupMutationResult {
	result := TerminalGroupMutationResult{Error: message, Revision: revision}
	if session != nil {
		result.Session = sessionPointer(*session)
	}
	return result
}

func sessionPointer(session domain.Session) *domain.Session {
	copy := cloneSession(session)
	return &copy
}

func cloneActive(active ActiveSession) ActiveSession {
	copy := active
	if active.Session != nil {
		copy.Session = sessionPointer(*active.Session)
	}
	return copy
}

func cloneSession(session domain.Session) domain.Session {
	copy := session
	copy.Extra = cloneExtra(session.Extra)
	copy.TerminalGroups = domain.CloneTerminalGroups(session.TerminalGroups)
	copy.Terminals = make([]domain.Terminal, len(session.Terminals))
	for index, terminal := range session.Terminals {
		copy.Terminals[index] = terminal
		copy.Terminals[index].Extra = cloneExtra(terminal.Extra)
		copy.Terminals[index].Root = domain.CloneContentNode(terminal.Root)
		copy.Terminals[index].CommandStates = domain.CloneCommandExecutionStates(terminal.CommandStates)
	}
	return copy
}

func cloneExtra(extra map[string]json.RawMessage) map[string]json.RawMessage {
	if extra == nil {
		return nil
	}
	copy := make(map[string]json.RawMessage, len(extra))
	for key, value := range extra {
		copy[key] = append([]byte(nil), value...)
	}
	return copy
}

func clearCommandStates(session *domain.Session) {
	if session == nil {
		return
	}
	for index := range session.Terminals {
		session.Terminals[index].CommandStates = nil
	}
}

func mergeCanonicalSessionState(authored, canonical domain.Session) (domain.Session, error) {
	merged := mergeCanonicalTerminalGroups(authored, canonical)
	if err := validateAuthoredTransitionChanges(merged, canonical); err != nil {
		return domain.Session{}, err
	}
	canonicalByTerminal := make(map[string]domain.Terminal, len(canonical.Terminals))
	for _, terminal := range canonical.Terminals {
		canonicalByTerminal[terminal.ID] = terminal
	}
	for index := range merged.Terminals {
		terminal := &merged.Terminals[index]
		canonicalTerminal, exists := canonicalByTerminal[terminal.ID]
		if !exists || len(canonicalTerminal.CommandStates) == 0 {
			terminal.CommandStates = nil
			continue
		}
		terminal.CommandStates = nil
		for commandID, state := range canonicalTerminal.CommandStates {
			node := contentNodeByID(&terminal.Root, commandID)
			if node == nil {
				continue
			}
			if node.Type != domain.NodeCommand || node.StateChange == nil {
				return domain.Session{}, fmt.Errorf("completed command %q must retain its state-change configuration", commandID)
			}
			if terminal.CommandStates == nil {
				terminal.CommandStates = make(map[string]domain.CommandExecutionState)
			}
			terminal.CommandStates[commandID] = state
		}
	}
	if err := verifySessionContract(merged); err != nil {
		return domain.Session{}, err
	}
	return merged, nil
}

type terminalTransitionKey struct {
	terminalID string
	commandID  string
}

func validateAuthoredTransitionChanges(candidate, canonical domain.Session) error {
	canonicalTargets := terminalTransitionTargets(canonical)
	groupByTerminal := make(map[string]string, len(candidate.Terminals))
	for _, group := range candidate.TerminalGroups {
		for _, terminalID := range group.TerminalIDs {
			groupByTerminal[terminalID] = group.ID
		}
	}

	for key, targetID := range terminalTransitionTargets(candidate) {
		if groupByTerminal[key.terminalID] == groupByTerminal[targetID] {
			continue
		}
		if canonicalTargets[key] == targetID {
			continue
		}
		return fmt.Errorf(
			"terminal %q transition %q to %q crosses terminal groups",
			key.terminalID, key.commandID, targetID,
		)
	}
	return nil
}

func terminalTransitionTargets(session domain.Session) map[terminalTransitionKey]string {
	targets := make(map[terminalTransitionKey]string)
	for _, terminal := range session.Terminals {
		walkTerminalTransitions(terminal.ID, terminal.Root, targets)
	}
	return targets
}

func walkTerminalTransitions(
	terminalID string,
	node domain.ContentNode,
	targets map[terminalTransitionKey]string,
) {
	if node.Type == domain.NodeCommand && node.TerminalTransition != nil {
		targets[terminalTransitionKey{terminalID: terminalID, commandID: node.ID}] =
			node.TerminalTransition.TargetTerminalID
	}
	for _, child := range node.Children {
		walkTerminalTransitions(terminalID, child, targets)
	}
}

func verifyAuthoredSession(authored domain.Session) error {
	candidate := cloneSession(authored)
	candidate.TerminalGroups = nil
	candidate = domain.NormalizeTerminalGroups(candidate)
	return verifySessionContract(candidate)
}

func mergeCanonicalTerminalGroups(authored, canonical domain.Session) domain.Session {
	merged := cloneSession(authored)
	authoredTerminalIDs := make(map[string]struct{}, len(merged.Terminals))
	for _, terminal := range merged.Terminals {
		authoredTerminalIDs[terminal.ID] = struct{}{}
	}

	merged.TerminalGroups = make([]domain.TerminalGroup, 0, len(canonical.TerminalGroups))
	for _, canonicalGroup := range canonical.TerminalGroups {
		group := domain.TerminalGroup{
			ID:          canonicalGroup.ID,
			Name:        canonicalGroup.Name,
			TerminalIDs: make([]string, 0, len(canonicalGroup.TerminalIDs)),
		}
		for _, terminalID := range canonicalGroup.TerminalIDs {
			if _, exists := authoredTerminalIDs[terminalID]; exists {
				group.TerminalIDs = append(group.TerminalIDs, terminalID)
			}
		}
		if len(group.TerminalIDs) != 0 {
			merged.TerminalGroups = append(merged.TerminalGroups, group)
		}
	}
	return domain.EnsureTerminalGroups(merged)
}

func encodeAcceptedSession(candidate domain.Session) ([]byte, error) {
	if err := verifySessionContract(candidate); err != nil {
		return nil, err
	}
	data, err := domain.EncodeSession(candidate)
	if err != nil {
		return nil, err
	}
	if _, err := domain.DecodeSession(data); err != nil {
		return nil, err
	}
	return data, nil
}

func sessionsEqual(left, right domain.Session) bool {
	leftData, leftErr := domain.EncodeSession(left)
	rightData, rightErr := domain.EncodeSession(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftData, rightData)
}

func terminalByID(session *domain.Session, terminalID string) *domain.Terminal {
	if session == nil || strings.TrimSpace(terminalID) == "" {
		return nil
	}
	for index := range session.Terminals {
		if session.Terminals[index].ID == terminalID {
			return &session.Terminals[index]
		}
	}
	return nil
}

func contentNodeByID(node *domain.ContentNode, nodeID string) *domain.ContentNode {
	if node == nil || strings.TrimSpace(nodeID) == "" {
		return nil
	}
	if node.ID == nodeID {
		return node
	}
	for index := range node.Children {
		if found := contentNodeByID(&node.Children[index], nodeID); found != nil {
			return found
		}
	}
	return nil
}
