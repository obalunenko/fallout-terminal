package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	controlservice "github.com/obalunenko/Fallout-Terminal/v2/internal/control"
	"github.com/obalunenko/Fallout-Terminal/v2/internal/domain"
	playerconfigservice "github.com/obalunenko/Fallout-Terminal/v2/internal/playerconfig"
	sessionservice "github.com/obalunenko/Fallout-Terminal/v2/internal/session"
	tunnelservice "github.com/obalunenko/Fallout-Terminal/v2/internal/tunnel"
	updateservice "github.com/obalunenko/Fallout-Terminal/v2/internal/update"
	"github.com/obalunenko/logger"
)

const (
	serverInfoEvent         = "server-info"
	clientCountEvent        = "client-count"
	hackStateEvent          = "hack-state"
	coordinationStateEvent  = "coordination-state"
	sessionStateEvent       = "session-state"
	publicAccessStatusEvent = "public-access-status"
)

var (
	errApplicationContextRequired       = errors.New("application context is required")
	errApplicationShutdown              = errors.New("application shutdown")
	errApplicationStartupComplete       = errors.New("application startup complete")
	errApplicationStartupTimeout        = errors.New("application startup timed out")
	errApplicationCleanupComplete       = errors.New("application cleanup complete")
	errApplicationCleanupTimeout        = errors.New("application cleanup timed out")
	errPublicAccessClipboardUnavailable = errors.New("system clipboard is unavailable")
)

// SessionService is the lifecycle boundary for the ordered persistence worker.
// Session commands extend this interface when they are introduced.
type SessionService interface {
	Shutdown(context.Context) error
}

type sessionCommands interface {
	Create(context.Context) sessionservice.SessionResult
	Open(context.Context) sessionservice.SessionResult
	CopyDemo(context.Context) sessionservice.SessionResult
	Save(context.Context, domain.Session, uint64) sessionservice.SaveResult
	Snapshot() sessionservice.ActiveSession
}

type sessionPlayerConfigCommands interface {
	Snapshot() sessionservice.ActiveSession
	AssociatePlayerConfig(context.Context, string) sessionservice.SessionResult
}

type sessionSnapshotService interface {
	Snapshot() sessionservice.ActiveSession
}

type sessionCommandStateCommands interface {
	ResetCommandState(context.Context, string, string) sessionservice.CommandStateResult
	ResetTerminalCommandStates(context.Context, string) sessionservice.CommandStateResult
}

// PlayerConfigService owns trusted native selection and strict durable files.
type PlayerConfigService interface {
	Create(context.Context) playerconfigservice.Result
	Open(context.Context) playerconfigservice.Result
	LoadReferenced(string, string) playerconfigservice.Result
}

// LiveService owns canonical shared player state.
type LiveService interface {
	Update(domain.ContentNode, *string) (*domain.PublicLiveState, bool)
	Clear()
	Snapshot() *domain.PublicLiveState
	ForceHackSuccess() (*domain.PublicHackState, bool)
}

// CoordinationService owns process-local logical sessions, roster, claims,
// broadcast lifetime, and controller state behind one transaction boundary.
type CoordinationService interface {
	Snapshot() *domain.MasterCoordinationState
	AddCharacter(domain.CharacterCreatePayload) (*domain.MasterCoordinationState, error)
	StartBroadcast() (*domain.MasterCoordinationState, error)
}

// coordinationCorrectionService is the additive trusted roster/session seam.
// Keeping it additive lets focused lifecycle fakes provide only the commands
// they exercise while production control.Service implements the complete set.
type coordinationCorrectionService interface {
	UpdateCharacter(domain.CharacterUpdatePayload) (*domain.MasterCoordinationState, error)
	DeleteCharacter(domain.CharacterDeletePayload) (*domain.MasterCoordinationState, error)
	RenameLogicalSession(domain.LogicalSessionID, string) (*domain.MasterCoordinationState, error)
	AssignCharacter(domain.LogicalSessionID, domain.CharacterID) (*domain.MasterCoordinationState, error)
	ReleaseCharacter(domain.LogicalSessionID) (*domain.MasterCoordinationState, error)
	MoveCharacter(domain.CharacterID, domain.LogicalSessionID) (*domain.MasterCoordinationState, error)
}

type coordinationControllerService interface {
	SetActiveController(domain.LogicalSessionID) (*domain.MasterCoordinationState, error)
}

type coordinationTerminalGroupService interface {
	ReplaceTerminalGroups(context.Context, domain.TerminalGroupCandidate) (*domain.MasterCoordinationState, *controlservice.TerminalGroupMutation, error)
}

type coordinationFacilityAuthoringService interface {
	SaveFacilityAuthoring(context.Context, controlservice.FacilityAuthoringRequest) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error)
}

type coordinationFacilityRecoveryService interface {
	RecoverFacilityCondition(context.Context, controlservice.FacilityRecoveryRequest) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error)
}

type coordinationFacilityPreviewService interface {
	PreviewFacility(context.Context, domain.FacilityPreview) controlservice.FacilityPreviewResult
}

type coordinationFacilityInspectionService interface {
	InspectFacilityDependencies(context.Context, controlservice.FacilityDependencyInspectionRequest) controlservice.FacilityDependencyInspectionResult
}

type coordinationFacilityResetService interface {
	ResetFacilityDevice(context.Context, controlservice.FacilityDeviceResetRequest) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error)
	ResetFacility(context.Context, controlservice.FacilityResetRequest) (*domain.MasterCoordinationState, *domain.FacilityOperationResult, error)
}

type coordinationFacilityLifecycleService interface {
	ReplaceFacility(*domain.Facility) (*domain.MasterCoordinationState, error)
}

type coordinationMasterEventDeferral interface {
	Commit()
	Discard(*domain.MasterCoordinationState)
}

type coordinationMasterEventDeferrer interface {
	DeferMasterEvents() coordinationMasterEventDeferral
}

type noopCoordinationMasterEventDeferral struct{}

func (noopCoordinationMasterEventDeferral) Commit() {}

func (noopCoordinationMasterEventDeferral) Discard(*domain.MasterCoordinationState) {}

// coordinationTerminalService is the trusted terminal-selection boundary. It
// keeps terminal choice, runtime checkpoints, and publication ordered by the
// same coordinator that owns assignments and controller authority.
type coordinationTerminalService interface {
	RequestTerminalActivation(domain.TerminalTarget) (*domain.MasterCoordinationState, error)
	RequestTerminalClear() (*domain.MasterCoordinationState, error)
	UpdateLiveTerminal(domain.ContentNode, *string) (*domain.MasterCoordinationState, error)
	ResetFailedHack(domain.TerminalTarget) (*domain.MasterCoordinationState, error)
}

type coordinationTerminalCanonicalRefreshService interface {
	RefreshActiveTerminal(domain.TerminalTarget) (*domain.MasterCoordinationState, error)
}

type coordinationTerminalDecisionService interface {
	ResolveTerminalSwitch(domain.SwitchID, domain.TerminalSwitchChoice) (*domain.MasterCoordinationState, error)
}

type coordinationCommandExecutionService interface {
	ResolveCommandExecution(context.Context, string, domain.CommandExecutionDecision) (*domain.MasterCoordinationState, *controlservice.CommandStateMutation, *domain.FacilityOperationResult, error)
}

type coordinationCommandStateResetService interface {
	ResetCommandState(context.Context, string, string) (*domain.MasterCoordinationState, *controlservice.CommandStateMutation, error)
	ResetTerminalCommandStates(context.Context, string) (*domain.MasterCoordinationState, *controlservice.CommandStateMutation, error)
}

type coordinationTerminalNavigationService interface {
	ResolveTerminalNavigation(string, domain.TerminalNavigationDecision) (*domain.MasterCoordinationState, error)
}

type coordinationBroadcastLifecycleService interface {
	EndBroadcast() (*domain.MasterCoordinationState, error)
}

type coordinationPlayerConfigService interface {
	InstallPlayerConfig(domain.PlayerConfigHandle, []domain.CharacterRosterEntry) (*domain.MasterCoordinationState, error)
	ClearPlayerConfig() (*domain.MasterCoordinationState, error)
}

type coordinationShutdownService interface {
	Shutdown()
}

type coordinatedHackSuccess interface {
	ForceHackSuccess() (*domain.PublicHackState, bool)
}

// Browser is the allowlisted external-browser boundary.
type Browser interface {
	OpenURL(string) error
}

// ClipboardWriter is the native-only boundary for one-shot trusted text.
type ClipboardWriter interface {
	SetText(string) bool
}

// PlayerServer owns the in-process HTTP/Connect listener.
type PlayerServer interface {
	Start(context.Context) (domain.ServerInfo, error)
	Stop(context.Context) error
}

// playerPublisher is implemented by the complete player server. Keeping
// publication separate from the lifecycle interface preserves construction
// compatibility with partial-start and test servers while making every live
// bridge mutation observable by connected browsers.
type playerPublisher interface {
	PublishUpdate()
	PublishHack()
}

// DesktopRuntime represents the readiness and release boundary of the desktop
// host independently of Wails globals.
type DesktopRuntime interface {
	Ready(context.Context) error
	Close(context.Context) error
}

type logDirectoryOpener interface {
	OpenDirectory(string) error
}

// EventSink publishes narrow, public values to the Overseer frontend.
type EventSink interface {
	Emit(name string, payload any) error
}

// PublicAccessSettingsStore persists only the versioned, non-secret public-
// access preferences. Production credentials remain behind SecretStore.
type PublicAccessSettingsStore interface {
	Load() (tunnelservice.PublicAccessPreferences, error)
	Save(tunnelservice.PublicAccessPreferences) error
}

// PublicAccessCore owns the embedded endpoint lifecycle and its secret-free
// state. App exposes only trusted desktop methods around this core.
type PublicAccessCore interface {
	Initialize(context.Context) tunnelservice.PublicAccessSnapshot
	Snapshot() tunnelservice.PublicAccessSnapshot
	Start(context.Context, uint64) tunnelservice.PublicAccessResult
	Stop(context.Context, uint64) tunnelservice.PublicAccessResult
	Reconfigure(context.Context, tunnelservice.PublicAccessMutation) tunnelservice.PublicAccessResult
	Shutdown(context.Context) error
}

// ApplicationUpdateService is the transport-neutral update boundary exposed
// through the narrow desktop facade. Provider and replacement details remain
// owned by internal/update and its composition adapters.
type ApplicationUpdateService interface {
	Snapshot() updateservice.UpdateSnapshot
	Status(context.Context) updateservice.UpdateSnapshot
	ResolveOffer(context.Context, string, updateservice.OfferDecision) updateservice.CommandResult
	ResolveRestart(context.Context, string, updateservice.RestartDecision) updateservice.CommandResult
}

// coordinationStateObserver receives accepted private coordination snapshots
// for root-owned side effects that are not part of the frontend event bridge.
type coordinationStateObserver interface {
	observeCoordinationState(*domain.MasterCoordinationState)
}

// AppDependencies contains constructed services. Construction acquires no
// external resources; Start owns acquisition in contract order.
type AppDependencies struct {
	Sessions             SessionService
	PlayerConfigs        PlayerConfigService
	Live                 LiveService
	Coordination         CoordinationService
	CoordinationEvents   coordinationMasterEventDeferrer
	Player               PlayerServer
	Desktop              DesktopRuntime
	Browser              Browser
	LogDirectoryOpener   logDirectoryOpener
	LogDirectory         string
	ActiveLogPath        func() string
	Clipboard            ClipboardWriter
	Events               EventSink
	PublicSettings       PublicAccessSettingsStore
	PublicSecrets        tunnelservice.SecretStore
	PublicAccess         PublicAccessCore
	Updates              ApplicationUpdateService
	CoordinationObserver coordinationStateObserver
	PasswordEntropy      io.Reader
	Logger               logger.Logger
	StartupTimeout       time.Duration
	ShutdownTimeout      time.Duration
}

// RuntimeStatus is the synchronous startup/status snapshot used to avoid
// losing events emitted before the frontend subscribes.
type RuntimeStatus struct {
	ServerInfo        *domain.ServerInfo              `json:"serverInfo"`
	ClientCount       int                             `json:"clientCount"`
	HackState         *domain.PublicHackState         `json:"hackState"`
	StartupError      string                          `json:"startupError,omitempty"`
	SaveState         string                          `json:"saveState"`
	RequestedRevision uint64                          `json:"requestedRevision"`
	SavedRevision     uint64                          `json:"savedRevision"`
	CoordinationState *domain.MasterCoordinationState `json:"coordinationState"`
}

// CommandResult is used for privileged commands that do not return a model.
type CommandResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// LogAccessResult identifies the fixed retained-log location even when the
// native shell cannot open it, so the Overseer can navigate there manually.
type LogAccessResult struct {
	OK            bool   `json:"ok"`
	Error         string `json:"error,omitempty"`
	DirectoryPath string `json:"directoryPath"`
	ActiveLogPath string `json:"activeLogPath,omitempty"`
}

// ApplicationUpdateSnapshot is the narrow, non-sensitive update projection
// exposed to the Overseer. Provider metadata, verification evidence, local
// paths, and helper state remain behind the native boundary.
type ApplicationUpdateSnapshot struct {
	Revision         uint64  `json:"revision"`
	AttemptID        string  `json:"attemptId,omitempty"`
	State            string  `json:"state"`
	InstalledVersion string  `json:"installedVersion"`
	AvailableVersion string  `json:"availableVersion,omitempty"`
	ReleaseNotes     string  `json:"releaseNotes,omitempty"`
	BytesDownloaded  uint64  `json:"bytesDownloaded"`
	DownloadSize     *uint64 `json:"downloadSize,omitempty"`
	FailedStage      string  `json:"failedStage,omitempty"`
	ErrorMessage     string  `json:"errorMessage,omitempty"`
	RecoveryAction   string  `json:"recoveryAction,omitempty"`
}

type ApplicationUpdateOfferDecisionPayload struct {
	AttemptID string `json:"attemptId"`
	Decision  string `json:"decision"`
}

type ApplicationUpdateRestartDecisionPayload struct {
	AttemptID string `json:"attemptId"`
	Decision  string `json:"decision"`
}

// ApplicationUpdateCommandResult includes the authoritative snapshot for
// both accepted and rejected decisions.
type ApplicationUpdateCommandResult struct {
	OK       bool                      `json:"ok"`
	Error    string                    `json:"error,omitempty"`
	Snapshot ApplicationUpdateSnapshot `json:"snapshot"`
}

// SessionStateResult returns the canonical durable document and its
// session-owned revision after a trusted command-state mutation.
type SessionStateResult struct {
	OK       bool            `json:"ok"`
	Error    string          `json:"error,omitempty"`
	Revision uint64          `json:"revision"`
	Session  *domain.Session `json:"session,omitempty"`
}

// SessionStateEvent is emitted only after a command-state mutation reaches
// durability. It intentionally excludes the user-selected file path.
type SessionStateEvent struct {
	Revision uint64          `json:"revision"`
	Session  *domain.Session `json:"session"`
}

// TerminalGroupReplacementPayload is the private complete-set authoring
// request. Both revisions are captured from the canonical state reviewed by
// the Overseer.
type TerminalGroupReplacementPayload struct {
	TerminalGroups               []domain.TerminalGroup `json:"terminalGroups"`
	ExpectedSessionRevision      uint64                 `json:"expectedSessionRevision"`
	ExpectedCoordinationRevision uint64                 `json:"expectedCoordinationRevision"`
}

// TerminalGroupReplacementResult returns both authoritative owners on success
// and on stale rejection so the frontend never keeps an optimistic draft.
type TerminalGroupReplacementResult struct {
	OK                bool                            `json:"ok"`
	Error             string                          `json:"error,omitempty"`
	SessionRevision   uint64                          `json:"sessionRevision"`
	Session           *domain.Session                 `json:"session,omitempty"`
	CoordinationState *domain.MasterCoordinationState `json:"coordinationState"`
}

// FacilityAuthoringPayload is the complete private facility draft and the
// canonical revisions against which the Overseer reviewed it.
type FacilityAuthoringPayload struct {
	Session                  *domain.Session `json:"session"`
	ExpectedSessionRevision  uint64          `json:"expectedSessionRevision"`
	ExpectedFacilityRevision uint64          `json:"expectedFacilityRevision"`
	CorrelationID            string          `json:"correlationId"`
}

// FacilityRecoveryPayload selects one authored private recovery against the
// exact facility revision reviewed by the Overseer.
type FacilityRecoveryPayload struct {
	ConditionID              string                              `json:"conditionId"`
	ExpectedFacilityRevision uint64                              `json:"expectedFacilityRevision"`
	CorrelationID            string                              `json:"correlationId"`
	Recovery                 *domain.DiagnosticRecoveryReference `json:"recovery"`
}

// FacilityPreviewPayload selects one detached state or condition override for
// a single terminal at the exact facility revision inspected by the Overseer.
type FacilityPreviewPayload struct {
	ExpectedFacilityRevision uint64                             `json:"expectedFacilityRevision"`
	TerminalID               string                             `json:"terminalId"`
	DeviceState              *domain.FacilityDeviceStatePreview `json:"deviceState,omitempty"`
	Condition                *domain.FacilityConditionPreview   `json:"condition,omitempty"`
}

// FacilityPreviewResult is a detached, private effective terminal projection.
type FacilityPreviewResult struct {
	OK               bool                       `json:"ok"`
	Failure          domain.FacilityFailureCode `json:"failure,omitempty"`
	Issues           []domain.FacilityIssue     `json:"issues,omitempty"`
	FacilityRevision uint64                     `json:"facilityRevision"`
	Terminal         *domain.PublicLiveState    `json:"terminal,omitempty"`
}

// FacilityDeviceResetPayload restores one device and its directly scoped
// conditions after the Overseer confirms the operation.
type FacilityDeviceResetPayload struct {
	DeviceID                 string `json:"deviceId"`
	ExpectedFacilityRevision uint64 `json:"expectedFacilityRevision"`
	CorrelationID            string `json:"correlationId"`
}

// FacilityResetPayload restores every authored current value after the
// Overseer confirms the operation.
type FacilityResetPayload struct {
	ExpectedFacilityRevision uint64 `json:"expectedFacilityRevision"`
	CorrelationID            string `json:"correlationId"`
}

// FacilityDependencyInspectionPayload identifies one stable authored entity
// without exposing the facility graph through the player service.
type FacilityDependencyInspectionPayload struct {
	Target                   *domain.FacilityEntityReference `json:"target"`
	ExpectedSessionRevision  uint64                          `json:"expectedSessionRevision"`
	ExpectedFacilityRevision uint64                          `json:"expectedFacilityRevision"`
}

// FacilityDependencyInspectionResult is the detached private dependency
// projection for one canonical session revision.
type FacilityDependencyInspectionResult struct {
	OK               bool                             `json:"ok"`
	Failure          domain.FacilityFailureCode       `json:"failure"`
	Issues           []domain.FacilityIssue           `json:"issues,omitempty"`
	SessionRevision  uint64                           `json:"sessionRevision"`
	FacilityRevision uint64                           `json:"facilityRevision"`
	Report           *domain.FacilityDependencyReport `json:"report,omitempty"`
}

// PublicAccessPreferences is the secret-free native desktop projection.
type PublicAccessPreferences struct {
	Version                   uint32 `json:"version"`
	EnabledPreference         bool   `json:"enabledPreference"`
	ReservedDomain            string `json:"reservedDomain,omitempty"`
	Username                  string `json:"username"`
	ProviderTokenPresentHint  bool   `json:"providerTokenPresentHint"`
	PlayerPasswordPresentHint bool   `json:"playerPasswordPresentHint"`
	Revision                  uint64 `json:"revision"`
}

type PublicAccessStatus struct {
	State            string `json:"state"`
	Generation       uint64 `json:"generation"`
	SettingsRevision uint64 `json:"settingsRevision"`
	PublicURL        string `json:"publicUrl,omitempty"`
	ErrorCategory    string `json:"errorCategory,omitempty"`
	ErrorMessage     string `json:"errorMessage,omitempty"`
}

type PublicAccessSnapshot struct {
	Preferences            PublicAccessPreferences `json:"preferences"`
	ProviderTokenPresence  string                  `json:"providerTokenPresence"`
	PlayerPasswordPresence string                  `json:"playerPasswordPresence"`
	Status                 PublicAccessStatus      `json:"status"`
}

// SavePublicAccessSettingsPayload is ephemeral trusted input. Its secret
// fields are consumed into byte buffers and are never copied into a result,
// event, status, or persistence model.
type SavePublicAccessSettingsPayload struct {
	ExpectedRevision          uint64 `json:"expectedRevision"`
	EnabledPreference         bool   `json:"enabledPreference"`
	ReservedDomain            string `json:"reservedDomain,omitempty"`
	Username                  string `json:"username"`
	ReplacementProviderToken  string `json:"replacementProviderToken,omitempty"`
	DeleteProviderToken       bool   `json:"deleteProviderToken,omitempty"`
	ReplacementPlayerPassword string `json:"replacementPlayerPassword,omitempty"`
	DeletePlayerPassword      bool   `json:"deletePlayerPassword,omitempty"`
}

type PublicAccessCommandPayload struct {
	ExpectedRevision uint64 `json:"expectedRevision"`
}

type PublicAccessCommandResult struct {
	OK       bool                 `json:"ok"`
	Error    string               `json:"error,omitempty"`
	Snapshot PublicAccessSnapshot `json:"snapshot"`
}

// GeneratedPlayerPasswordResult is deliberately not reusable and contains no
// snapshot. GeneratedPassword is populated only after secure-store replacement
// and settings persistence both succeed.
type GeneratedPlayerPasswordResult struct {
	OK                bool   `json:"ok"`
	Error             string `json:"error,omitempty"`
	GeneratedPassword string `json:"generatedPassword,omitempty"`
	SettingsRevision  uint64 `json:"settingsRevision"`
}

// CoordinationCommandResult returns the authoritative detached state for
// both accepted and rejected roster/broadcast commands.
type CoordinationCommandResult struct {
	OK    bool                            `json:"ok"`
	Error string                          `json:"error,omitempty"`
	State *domain.MasterCoordinationState `json:"state"`
}

// ResolveCommandExecutionResult is the private Overseer-only response to one
// exact pending command decision.
type ResolveCommandExecutionResult struct {
	OK             bool                            `json:"ok"`
	Error          string                          `json:"error,omitempty"`
	State          *domain.MasterCoordinationState `json:"state"`
	FacilityResult *domain.FacilityOperationResult `json:"facilityResult,omitempty"`
}

// ResolveTerminalNavigationResult is the private response for one exact
// forward/return decision and its resulting authoritative coordination state.
type ResolveTerminalNavigationResult struct {
	OK    bool                            `json:"ok"`
	Error string                          `json:"error,omitempty"`
	State *domain.MasterCoordinationState `json:"state"`
}

// PlayerConfigCommandResult combines durable metadata with authoritative roster state.
type PlayerConfigCommandResult struct {
	OK       bool                            `json:"ok"`
	Canceled bool                            `json:"canceled"`
	Error    string                          `json:"error,omitempty"`
	Config   *domain.PlayerConfigMetadata    `json:"playerConfig,omitempty"`
	Session  *domain.Session                 `json:"session,omitempty"`
	State    *domain.MasterCoordinationState `json:"state"`
}

// TerminalSwitchCommandResult describes either a completed terminal selection
// or a later decision gate. SwitchID remains empty for direct activation and
// clear operations; unfinished-puzzle choices populate it in the US8 flow.
type TerminalSwitchCommandResult struct {
	OK       bool                            `json:"ok"`
	Error    string                          `json:"error,omitempty"`
	Status   string                          `json:"status,omitempty"`
	SwitchID domain.SwitchID                 `json:"switchId,omitempty"`
	State    *domain.MasterCoordinationState `json:"state"`
}

// CharacterCreatePayload carries one complete trusted player profile and the
// coordination revision it was based on.
type CharacterCreatePayload struct {
	Name                string `json:"name"`
	Intelligence        int    `json:"intelligence"`
	HackerPerkAvailable *bool  `json:"hackerPerkAvailable"`
	ExpectedRevision    uint64 `json:"expectedRevision"`
}

// CharacterUpdatePayload carries a complete replacement profile for one
// stable roster identity and the coordination revision it was based on.
type CharacterUpdatePayload struct {
	CharacterID         domain.CharacterID `json:"characterId"`
	Name                string             `json:"name"`
	Intelligence        int                `json:"intelligence"`
	HackerPerkAvailable *bool              `json:"hackerPerkAvailable"`
	ExpectedRevision    uint64             `json:"expectedRevision"`
}

// CharacterDeletePayload identifies one stable roster identity and the
// coordination revision the deletion was based on.
type CharacterDeletePayload struct {
	CharacterID      domain.CharacterID `json:"characterId"`
	ExpectedRevision uint64             `json:"expectedRevision"`
}

// LogicalSessionRenamePayload changes only a process-local fallback label.
type LogicalSessionRenamePayload struct {
	SessionID    domain.LogicalSessionID `json:"sessionId"`
	FallbackName string                  `json:"fallbackName"`
}

// AssignmentPayload assigns one available character to one unassigned
// logical session.
type AssignmentPayload struct {
	SessionID   domain.LogicalSessionID `json:"sessionId"`
	CharacterID domain.CharacterID      `json:"characterId"`
}

// MoveCharacterPayload atomically transfers one stable character identity.
type MoveCharacterPayload struct {
	CharacterID domain.CharacterID      `json:"characterId"`
	ToSessionID domain.LogicalSessionID `json:"toSessionId"`
}

// LiveTerminalPayload is the validated set-live bridge input.
type LiveTerminalPayload struct {
	TerminalID   string             `json:"terminalId"`
	TerminalName string             `json:"terminalName"`
	Tree         domain.ContentNode `json:"tree"`
	HackLevel    int                `json:"hackLevel"`
	IntroText    string             `json:"introText"`
}

// LiveUpdatePayload replaces published content without resetting a puzzle.
type LiveUpdatePayload struct {
	Tree      domain.ContentNode `json:"tree"`
	IntroText *string            `json:"introText,omitempty"`
}

// TerminalSwitchDecisionPayload resolves one opaque unfinished-puzzle switch
// request with an exact allowlisted decision.
type TerminalSwitchDecisionPayload struct {
	SwitchID domain.SwitchID             `json:"switchId"`
	Decision domain.TerminalSwitchChoice `json:"decision"`
}

// CommandExecutionDecisionPayload resolves one exact server-owned pending
// request. The authored prompt itself is carried only by coordination state.
type CommandExecutionDecisionPayload struct {
	RequestID string                          `json:"requestId"`
	Decision  domain.CommandExecutionDecision `json:"decision"`
}

type TerminalNavigationDecisionPayload struct {
	RequestID string                            `json:"requestId"`
	Decision  domain.TerminalNavigationDecision `json:"decision"`
}

// ResetCommandStatePayload addresses one authored state-changing command by
// stable terminal and command IDs.
type ResetCommandStatePayload struct {
	TerminalID string `json:"terminalId"`
	CommandID  string `json:"commandId"`
}

// ResetTerminalCommandStatesPayload addresses one authored terminal whose
// durable command snapshots should be cleared atomically.
type ResetTerminalCommandStatesPayload struct {
	TerminalID string `json:"terminalId"`
}

// App is the Wails composition root. Domain behavior remains in internal
// packages; App owns lifecycle and the narrow desktop facade.
type App struct {
	lifecycleMu           sync.Mutex
	coordinationCommandMu sync.Mutex
	publicAccessCommandMu sync.Mutex
	mu                    sync.RWMutex

	deps   AppDependencies
	root   context.Context
	ctx    context.Context
	cancel context.CancelCauseFunc
	log    logger.Logger

	phase                    string
	serverInfo               *domain.ServerInfo
	clientCount              int
	hackState                *domain.PublicHackState
	coordinationState        *domain.MasterCoordinationState
	startupError             string
	saveState                string
	requestedRevision        uint64
	savedRevision            uint64
	publishedSessionRevision uint64
	playerStarted            bool
	desktopReady             bool
	sessionsClosed           bool
	processStateCleared      bool
	publicAccessClosed       bool
	stopped                  bool

	publicAccessLoaded      bool
	publicAccessPreferences tunnelservice.PublicAccessPreferences
	providerTokenPresence   tunnelservice.SecretPresence
	playerPasswordPresence  tunnelservice.SecretPresence
	publicAccessStatus      tunnelservice.PublicAccessStatus
}

// lifecyclePhase returns the host-owned lifecycle observation used by Go
// tests and adapters. It is deliberately absent from RuntimeStatus and every
// protobuf/native DTO; the Overseer derives presentation from serverInfo and
// startupError instead.
func (app *App) lifecyclePhase() string {
	app.mu.RLock()
	defer app.mu.RUnlock()
	return app.phase
}

// NewApp constructs the application without acquiring external resources.
func NewApp(ctx context.Context) *App {
	return NewAppWithDependencies(ctx, AppDependencies{})
}

// NewAppWithDependencies constructs a testable composition root.
func NewAppWithDependencies(ctx context.Context, deps AppDependencies) *App {
	if ctx == nil {
		panic(errApplicationContextRequired)
	}
	preferences := tunnelservice.DefaultPublicAccessPreferences()
	applicationLogger := deps.Logger
	if applicationLogger == nil {
		applicationLogger = logger.FromContext(ctx)
	}
	app := &App{
		deps: deps, root: ctx, ctx: ctx, log: applicationLogger, phase: "constructed", saveState: string(sessionservice.SaveStateIdle),
		publicAccessPreferences: preferences,
		providerTokenPresence:   tunnelservice.SecretUnknown,
		playerPasswordPresence:  tunnelservice.SecretUnknown,
		publicAccessStatus: tunnelservice.PublicAccessStatus{
			State: tunnelservice.LifecycleDisabled, SettingsRevision: preferences.Revision,
		},
	}
	if deps.Coordination != nil {
		app.coordinationState = domain.CloneMasterCoordinationState(deps.Coordination.Snapshot())
	}
	return app
}

func (app *App) recordOperation(operation, outcome string, fields logger.Fields) {
	if app == nil || app.log == nil {
		return
	}
	entry := app.log.WithFields(fields).WithFields(logger.Fields{
		"operation": operation,
		"outcome":   outcome,
	})
	switch outcome {
	case "failed", "rejected":
		entry.Warn("application operation completed")
	default:
		entry.Info("application operation completed")
	}
}

func (app *App) recordAuditEvents(events []controlservice.AuditEvent, revision uint64) {
	if app == nil || app.log == nil {
		return
	}
	for _, event := range events {
		fields := logger.Fields{"event": event.Name, "outcome": event.Outcome, "revision": revision}
		if facilityAuditEvent(event.Name) {
			outcome := facilityAuditOutcome(event.Outcome)
			fields["outcome"] = outcome
			addFacilityAuditLogFields(fields, event)
			app.recordRuntimeAuditEvent(outcome, fields)
			continue
		}
		if event.Decision != "" {
			fields["decision"] = event.Decision
		}
		if event.Reason != "" {
			fields["reason"] = event.Reason
		}
		if event.SessionID != "" {
			fields["session_id"] = string(event.SessionID)
		}
		if event.Role != "" {
			fields["role"] = string(event.Role)
		}
		if event.PreviousRole != "" {
			fields["previous_role"] = string(event.PreviousRole)
		}
		if event.RequestID != "" {
			fields["request_id"] = event.RequestID
		}
		if event.BroadcastID != "" {
			fields["broadcast_id"] = string(event.BroadcastID)
		}
		if event.TerminalID != "" {
			fields["terminal_id"] = event.TerminalID
		}
		if event.CommandID != "" {
			fields["command_id"] = event.CommandID
		}
		if event.Mode != "" {
			fields["mode"] = event.Mode
		}
		if event.PuzzleID != "" {
			fields["puzzle_id"] = event.PuzzleID
		}
		if event.PreviousPuzzleID != "" {
			fields["previous_puzzle_id"] = event.PreviousPuzzleID
		}
		if strings.HasPrefix(event.Name, "hack.") {
			fields["attempts_left"] = event.AttemptsLeft
			if event.Name == "hack.started" {
				fields["hack_level"] = event.HackLevel
				fields["attempts_max"] = event.AttemptsMax
			}
		}
		app.recordRuntimeAuditEvent(event.Outcome, fields)
	}
}

func (app *App) recordRuntimeAuditEvent(outcome string, fields logger.Fields) {
	entry := app.log.WithFields(fields)
	switch outcome {
	case "failed", "declined", "rejected", "stale", "superseded", "duplicate", "invalid", "conflict", "interrupted":
		entry.Warn("runtime audit event")
	default:
		entry.Info("runtime audit event")
	}
}

func facilityAuditEvent(name string) bool {
	switch name {
	case "facility.request_received", "facility.decision", "facility.transition", "facility.failure",
		"facility.recovery", "facility.reset":
		return true
	default:
		return false
	}
}

func facilityAuditOutcome(outcome string) string {
	switch outcome {
	case "pending", "succeeded", "failed", "declined", "rejected", "committed", "unchanged",
		"stale", "duplicate", "conflict", "invalid", "missing-reference", "invalid-transition",
		"precondition-failed", "stale-revision", "invalid-configuration", "persistence-failed",
		"runtime-context-ended":
		return outcome
	default:
		return "invalid"
	}
}

func addFacilityAuditLogFields(fields logger.Fields, event controlservice.AuditEvent) {
	facts := event.Facility
	if facts == nil {
		return
	}
	if facilityAuditDecision(event.Decision) {
		fields["decision"] = event.Decision
	}
	correlationID := facts.CorrelationID
	if correlationID == "" {
		correlationID = event.RequestID
	}
	if correlationID != "" {
		fields["correlation_id"] = correlationID
	}
	if event.RequestID != "" {
		fields["request_id"] = event.RequestID
	}
	broadcastID := event.BroadcastID
	if broadcastID == "" {
		broadcastID = facts.BroadcastID
	}
	if broadcastID != "" {
		fields["broadcast_id"] = string(broadcastID)
	}
	terminalID := event.TerminalID
	if terminalID == "" {
		terminalID = facts.TerminalID
	}
	if terminalID != "" {
		fields["terminal_id"] = terminalID
	}
	commandID := event.CommandID
	if commandID == "" {
		commandID = facts.CommandID
	}
	if commandID != "" {
		fields["command_id"] = commandID
	}
	if knownFacilityAuditAction(facts.Action) {
		fields["facility_action"] = string(facts.Action)
	}
	fields["previous_facility_revision"] = facts.PreviousFacilityRevision
	fields["resulting_facility_revision"] = facts.ResultingFacilityRevision
	fields["device_ids"] = sortedUniqueAuditIDs(facts.DeviceIDs)
	fields["condition_ids"] = sortedUniqueAuditIDs(facts.ConditionIDs)
	if transitions := sortedDeviceAuditTransitions(facts.DeviceTransitions); len(transitions) != 0 {
		fields["device_transitions"] = transitions
	}
	if transitions := sortedConditionAuditTransitions(facts.ConditionTransitions); len(transitions) != 0 {
		fields["condition_transitions"] = transitions
	}
	if facts.Action == controlservice.FacilityAuditActionReset &&
		(facts.ResetScope == "device" || facts.ResetScope == "facility") {
		fields["reset_scope"] = facts.ResetScope
	}
	if knownFacilityFailure(facts.Failure) && facts.Failure != domain.FacilityFailureUnspecified {
		fields["facility_failure"] = string(facts.Failure)
	}
}

func facilityAuditDecision(decision string) bool {
	switch decision {
	case "approve", "decline", "reject":
		return true
	default:
		return false
	}
}

func knownFacilityAuditAction(action controlservice.FacilityAuditAction) bool {
	switch action {
	case controlservice.FacilityAuditActionCommand,
		controlservice.FacilityAuditActionReset,
		controlservice.FacilityAuditActionRecover:
		return true
	default:
		return false
	}
}

func knownFacilityFailure(failure domain.FacilityFailureCode) bool {
	switch failure {
	case domain.FacilityFailureUnspecified,
		domain.FacilityFailureRejected,
		domain.FacilityFailureMissingReference,
		domain.FacilityFailureInvalidTransition,
		domain.FacilityFailurePreconditionFailed,
		domain.FacilityFailureStaleRevision,
		domain.FacilityFailureConflict,
		domain.FacilityFailureDuplicate,
		domain.FacilityFailureInvalidConfiguration,
		domain.FacilityFailurePersistenceFailed,
		domain.FacilityFailureRuntimeContextEnded:
		return true
	default:
		return false
	}
}

func sortedUniqueAuditIDs(ids []string) []string {
	clone := slices.Clone(ids)
	slices.Sort(clone)
	return slices.Compact(clone)
}

func sortedDeviceAuditTransitions(
	transitions []controlservice.FacilityDeviceAuditTransition,
) []controlservice.FacilityDeviceAuditTransition {
	clone := slices.Clone(transitions)
	slices.SortFunc(clone, func(left, right controlservice.FacilityDeviceAuditTransition) int {
		if order := strings.Compare(left.DeviceID, right.DeviceID); order != 0 {
			return order
		}
		if order := strings.Compare(left.PreviousStateID, right.PreviousStateID); order != 0 {
			return order
		}
		return strings.Compare(left.ResultingStateID, right.ResultingStateID)
	})
	return slices.Compact(clone)
}

func sortedConditionAuditTransitions(
	transitions []controlservice.FacilityConditionAuditTransition,
) []controlservice.FacilityConditionAuditTransition {
	clone := slices.Clone(transitions)
	slices.SortFunc(clone, func(left, right controlservice.FacilityConditionAuditTransition) int {
		if order := strings.Compare(left.ConditionID, right.ConditionID); order != 0 {
			return order
		}
		if left.PreviousActive != right.PreviousActive {
			if !left.PreviousActive {
				return -1
			}
			return 1
		}
		if left.ResultingActive == right.ResultingActive {
			return 0
		}
		if !left.ResultingActive {
			return -1
		}
		return 1
	})
	return slices.Compact(clone)
}

func operationOutcome(ok, canceled bool) string {
	if ok {
		return "succeeded"
	}
	if canceled {
		return "cancelled"
	}
	return "failed"
}

func publicAccessLogFields(snapshot PublicAccessSnapshot) logger.Fields {
	fields := logger.Fields{
		"state":    snapshot.Status.State,
		"revision": snapshot.Status.SettingsRevision,
	}
	if snapshot.Status.ErrorCategory != "" {
		fields["error_category"] = snapshot.Status.ErrorCategory
	}
	return fields
}

func (app *App) emitEvent(name string, payload any) {
	if app == nil || app.deps.Events == nil {
		return
	}
	if err := app.deps.Events.Emit(name, payload); err != nil && app.log != nil {
		app.log.WithFields(logger.Fields{
			"error_category": "delivery_failed",
			"event":          name,
		}).Error("desktop event delivery failed")
	}
}

// GetPublicAccess returns only reconciled presence and non-secret settings.
func (app *App) GetPublicAccess() PublicAccessSnapshot {
	app.publicAccessCommandMu.Lock()
	defer app.publicAccessCommandMu.Unlock()
	if app.deps.PublicAccess != nil {
		snapshot := app.deps.PublicAccess.Snapshot()
		app.acceptPublicAccessSnapshot(snapshot, false)
		return routePublicAccessSnapshot(snapshot)
	}
	app.loadPublicAccessLocked(app.contextSnapshot())
	return app.publicAccessSnapshotLocked()
}

// CopyPublicAccessCredentials writes the saved player login and password to
// the native clipboard without returning the password through the desktop API.
func (app *App) CopyPublicAccessCredentials() (result CommandResult) {
	defer func() {
		app.recordOperation("public-access.credentials-share", operationOutcome(result.OK, false), nil)
	}()
	app.publicAccessCommandMu.Lock()
	defer app.publicAccessCommandMu.Unlock()

	ctx := app.contextSnapshot()
	var preferences tunnelservice.PublicAccessPreferences
	if app.deps.PublicAccess != nil {
		snapshot := app.deps.PublicAccess.Snapshot()
		app.acceptPublicAccessSnapshot(snapshot, false)
		preferences = snapshot.Preferences
	} else {
		app.loadPublicAccessLocked(ctx)
		app.mu.RLock()
		preferences = app.publicAccessPreferences
		app.mu.RUnlock()
	}
	normalized, err := preferences.Normalized()
	if err != nil {
		return routeCommandResult(CommandResult{Error: tunnelservice.ErrorValidation.SafeMessage()})
	}
	if app.deps.Clipboard == nil {
		return routeCommandResult(CommandResult{Error: errPublicAccessClipboardUnavailable.Error()})
	}
	err = tunnelservice.WithPlayerPassword(ctx, app.deps.PublicSecrets, func(password []byte) error {
		if !app.deps.Clipboard.SetText("Логин: " + normalized.Username + "\nПароль: " + string(password)) {
			return errPublicAccessClipboardUnavailable
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errPublicAccessClipboardUnavailable) {
			return routeCommandResult(CommandResult{Error: errPublicAccessClipboardUnavailable.Error()})
		}
		return routeCommandResult(CommandResult{Error: publicAccessSecretCategory(err).SafeMessage()})
	}
	return routeCommandResult(CommandResult{OK: true})
}

// SavePublicAccessSettings validates the complete proposed revision before
// applying scoped secure-store changes and then the atomic non-secret file write.
func (app *App) SavePublicAccessSettings(payload SavePublicAccessSettingsPayload) (result PublicAccessCommandResult) {
	defer func() {
		app.recordOperation("public-access.settings", operationOutcome(result.OK, false), publicAccessLogFields(result.Snapshot))
	}()
	app.publicAccessCommandMu.Lock()
	defer app.publicAccessCommandMu.Unlock()
	ctx := app.contextSnapshot()
	if app.deps.PublicAccess != nil {
		routed, err := routeSavePublicAccessSettingsRequest(payload)
		if err != nil {
			return app.publicAccessCoreFailure(app.deps.PublicAccess.Snapshot(), tunnelservice.ErrorValidation)
		}
		providerValue := []byte(routed.ReplacementProviderToken)
		passwordValue := []byte(routed.ReplacementPlayerPassword)
		routed.ReplacementProviderToken = ""
		routed.ReplacementPlayerPassword = ""
		defer clear(providerValue)
		defer clear(passwordValue)
		current := app.deps.PublicAccess.Snapshot()
		mutation := tunnelservice.PublicAccessMutation{
			ExpectedRevision:        routed.ExpectedRevision,
			PersistVisibleOverrides: true,
			Preferences: tunnelservice.PublicAccessPreferences{
				Version: tunnelservice.PublicAccessSettingsVersion, EnabledPreference: routed.EnabledPreference,
				ReservedDomain: routed.ReservedDomain, Username: routed.Username,
			},
			ProviderToken:  tunnelservice.SecretMutation{Replacement: providerValue, Delete: routed.DeleteProviderToken},
			PlayerPassword: tunnelservice.SecretMutation{Replacement: passwordValue, Delete: routed.DeletePlayerPassword},
		}
		if routed.ExpectedRevision != current.Preferences.Revision {
			return app.publicAccessCoreFailure(current, tunnelservice.ErrorConflict)
		}
		result := app.deps.PublicAccess.Reconfigure(ctx, mutation)
		app.acceptPublicAccessSnapshot(result.Snapshot, true)
		return routePublicAccessCommandResult(PublicAccessCommandResult{
			OK: result.OK, Error: result.Error, Snapshot: routePublicAccessSnapshot(result.Snapshot),
		})
	}
	app.loadPublicAccessLocked(ctx)

	routed, err := routeSavePublicAccessSettingsRequest(payload)
	if err != nil {
		return app.publicAccessFailureLocked(tunnelservice.ErrorValidation, err.Error())
	}
	app.mu.RLock()
	current := app.publicAccessPreferences
	app.mu.RUnlock()
	if routed.ExpectedRevision != current.Revision {
		return app.publicAccessFailureLocked(tunnelservice.ErrorConflict, tunnelservice.ErrorConflict.SafeMessage())
	}
	proposed := tunnelservice.PublicAccessPreferences{
		Version: tunnelservice.PublicAccessSettingsVersion, EnabledPreference: routed.EnabledPreference,
		ReservedDomain: routed.ReservedDomain, Username: routed.Username, Revision: current.Revision + 1,
		ProviderTokenPresentHint:  current.ProviderTokenPresentHint,
		PlayerPasswordPresentHint: current.PlayerPasswordPresentHint,
	}
	proposed, err = proposed.Normalized()
	if err != nil {
		return app.publicAccessFailureLocked(tunnelservice.ErrorValidation, tunnelservice.ErrorValidation.SafeMessage())
	}
	providerValue := []byte(routed.ReplacementProviderToken)
	passwordValue := []byte(routed.ReplacementPlayerPassword)
	defer clear(providerValue)
	defer clear(passwordValue)
	if len(providerValue) > 0 {
		if err := tunnelservice.ValidateProviderToken(providerValue); err != nil {
			return app.publicAccessFailureLocked(tunnelservice.ErrorValidation, tunnelservice.ErrorValidation.SafeMessage())
		}
	}
	if len(passwordValue) > 0 {
		if err := tunnelservice.ValidatePlayerPassword(passwordValue); err != nil {
			return app.publicAccessFailureLocked(tunnelservice.ErrorValidation, tunnelservice.ErrorValidation.SafeMessage())
		}
	}
	if err := app.applySecretMutationLocked(ctx, tunnelservice.ProviderAccountToken, providerValue, routed.DeleteProviderToken); err != nil {
		return app.publicAccessSecretFailureLocked(err)
	}
	if err := app.applySecretMutationLocked(ctx, tunnelservice.PlayerBasicAuthPassword, passwordValue, routed.DeletePlayerPassword); err != nil {
		app.reconcilePublicAccessPresenceLocked(ctx)
		return app.publicAccessSecretFailureLocked(err)
	}
	app.reconcilePublicAccessPresenceLocked(ctx)
	app.mu.RLock()
	proposed.ProviderTokenPresentHint = app.providerTokenPresence == tunnelservice.SecretPresent
	proposed.PlayerPasswordPresentHint = app.playerPasswordPresence == tunnelservice.SecretPresent
	app.mu.RUnlock()
	if app.deps.PublicSettings == nil {
		return app.publicAccessFailureLocked(tunnelservice.ErrorSettingsCorrupt, tunnelservice.ErrorSettingsCorrupt.SafeMessage())
	}
	if err := app.deps.PublicSettings.Save(proposed); err != nil {
		return app.publicAccessFailureLocked(tunnelservice.ErrorSettingsCorrupt, tunnelservice.ErrorSettingsCorrupt.SafeMessage())
	}
	app.mu.Lock()
	app.publicAccessPreferences = proposed
	app.publicAccessStatus = tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleDisabled, Generation: app.publicAccessStatus.Generation + 1, SettingsRevision: proposed.Revision}
	app.mu.Unlock()
	return app.publicAccessSuccessLocked()
}

func (app *App) GeneratePlayerPassword(payload PublicAccessCommandPayload) (result GeneratedPlayerPasswordResult) {
	defer func() {
		app.recordOperation("public-access.password", operationOutcome(result.OK, false), logger.Fields{"revision": result.SettingsRevision})
	}()
	app.publicAccessCommandMu.Lock()
	defer app.publicAccessCommandMu.Unlock()
	ctx := app.contextSnapshot()
	if app.deps.PublicAccess != nil {
		current := app.deps.PublicAccess.Snapshot()
		routed := routePublicAccessCommandRequest(payload)
		if routed.ExpectedRevision != current.Preferences.Revision {
			return routeGeneratedPlayerPasswordResult(GeneratedPlayerPasswordResult{Error: tunnelservice.ErrorConflict.SafeMessage(), SettingsRevision: current.Preferences.Revision})
		}
		source := app.deps.PasswordEntropy
		if source == nil {
			source = rand.Reader
		}
		generated, err := tunnelservice.GeneratePlayerPassword(source)
		if err != nil {
			return routeGeneratedPlayerPasswordResult(GeneratedPlayerPasswordResult{Error: tunnelservice.ErrorProviderFailure.SafeMessage(), SettingsRevision: current.Preferences.Revision})
		}
		defer clear(generated)
		oneTimeValue := string(generated)
		result := app.deps.PublicAccess.Reconfigure(ctx, tunnelservice.PublicAccessMutation{
			ExpectedRevision: routed.ExpectedRevision,
			Preferences:      current.Preferences,
			PlayerPassword:   tunnelservice.SecretMutation{Replacement: generated},
		})
		app.acceptPublicAccessSnapshot(result.Snapshot, true)
		if !result.OK {
			return routeGeneratedPlayerPasswordResult(GeneratedPlayerPasswordResult{Error: result.Error, SettingsRevision: result.Snapshot.Preferences.Revision})
		}
		return routeGeneratedPlayerPasswordResult(GeneratedPlayerPasswordResult{OK: true, GeneratedPassword: oneTimeValue, SettingsRevision: result.Snapshot.Preferences.Revision})
	}
	app.loadPublicAccessLocked(ctx)
	routed := routePublicAccessCommandRequest(payload)
	app.mu.RLock()
	current := app.publicAccessPreferences
	app.mu.RUnlock()
	if routed.ExpectedRevision != current.Revision {
		return routeGeneratedPlayerPasswordResult(GeneratedPlayerPasswordResult{Error: tunnelservice.ErrorConflict.SafeMessage(), SettingsRevision: current.Revision})
	}
	source := app.deps.PasswordEntropy
	if source == nil {
		source = rand.Reader
	}
	generated, err := tunnelservice.GeneratePlayerPassword(source)
	if err != nil {
		return routeGeneratedPlayerPasswordResult(GeneratedPlayerPasswordResult{Error: tunnelservice.ErrorProviderFailure.SafeMessage(), SettingsRevision: current.Revision})
	}
	defer clear(generated)
	if err := tunnelservice.ReplaceSecret(ctx, app.deps.PublicSecrets, tunnelservice.PlayerBasicAuthPassword, generated); err != nil {
		app.publicAccessSecretFailureLocked(err)
		return routeGeneratedPlayerPasswordResult(GeneratedPlayerPasswordResult{Error: publicAccessSecretCategory(err).SafeMessage(), SettingsRevision: current.Revision})
	}
	current.Revision++
	current.PlayerPasswordPresentHint = true
	if app.deps.PublicSettings == nil || savePublicAccessMutationSettings(app.deps.PublicSettings, current, false) != nil {
		app.reconcilePublicAccessPresenceLocked(ctx)
		app.publicAccessFailureLocked(tunnelservice.ErrorSettingsCorrupt, tunnelservice.ErrorSettingsCorrupt.SafeMessage())
		return routeGeneratedPlayerPasswordResult(GeneratedPlayerPasswordResult{Error: tunnelservice.ErrorSettingsCorrupt.SafeMessage(), SettingsRevision: current.Revision - 1})
	}
	app.mu.Lock()
	app.publicAccessPreferences = current
	app.playerPasswordPresence = tunnelservice.SecretPresent
	app.publicAccessStatus = tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleDisabled, Generation: app.publicAccessStatus.Generation + 1, SettingsRevision: current.Revision}
	app.mu.Unlock()
	app.emitPublicAccessStatusLocked()
	return routeGeneratedPlayerPasswordResult(GeneratedPlayerPasswordResult{OK: true, GeneratedPassword: string(generated), SettingsRevision: current.Revision})
}

type mutationAwarePublicAccessSettingsStore interface {
	SaveForMutation(tunnelservice.PublicAccessPreferences, bool) error
}

func savePublicAccessMutationSettings(settings PublicAccessSettingsStore, preferences tunnelservice.PublicAccessPreferences, persistVisibleOverrides bool) error {
	if aware, ok := settings.(mutationAwarePublicAccessSettingsStore); ok {
		return aware.SaveForMutation(preferences, persistVisibleOverrides)
	}
	return settings.Save(preferences)
}

func (app *App) StartPublicAccess(payload PublicAccessCommandPayload) (result PublicAccessCommandResult) {
	var diagnosticCode tunnelservice.PublicAccessDiagnosticCode
	defer func() {
		fields := publicAccessLogFields(result.Snapshot)
		if diagnosticCode.Valid() {
			fields["diagnostic_code"] = diagnosticCode.String()
		}
		app.recordOperation("public-access.start", operationOutcome(result.OK, false), fields)
	}()
	app.publicAccessCommandMu.Lock()
	defer app.publicAccessCommandMu.Unlock()
	if app.deps.PublicAccess != nil {
		result := app.deps.PublicAccess.Start(app.contextSnapshot(), routePublicAccessCommandRequest(payload).ExpectedRevision)
		diagnosticCode = result.DiagnosticCode
		app.acceptPublicAccessSnapshot(result.Snapshot, true)
		return routePublicAccessCommandResult(PublicAccessCommandResult{
			OK: result.OK, Error: result.Error, Snapshot: routePublicAccessSnapshot(result.Snapshot),
		})
	}
	app.loadPublicAccessLocked(app.contextSnapshot())
	if routePublicAccessCommandRequest(payload).ExpectedRevision != app.publicAccessPreferences.Revision {
		return app.publicAccessFailureLocked(tunnelservice.ErrorConflict, tunnelservice.ErrorConflict.SafeMessage())
	}
	return app.publicAccessFailureLocked(tunnelservice.ErrorProviderFailure, "Public access is not available yet; local access remains available.")
}

func (app *App) StopPublicAccess(payload PublicAccessCommandPayload) (result PublicAccessCommandResult) {
	defer func() {
		app.recordOperation("public-access.stop", operationOutcome(result.OK, false), publicAccessLogFields(result.Snapshot))
	}()
	app.publicAccessCommandMu.Lock()
	defer app.publicAccessCommandMu.Unlock()
	if app.deps.PublicAccess != nil {
		result := app.deps.PublicAccess.Stop(app.contextSnapshot(), routePublicAccessCommandRequest(payload).ExpectedRevision)
		app.acceptPublicAccessSnapshot(result.Snapshot, true)
		return routePublicAccessCommandResult(PublicAccessCommandResult{
			OK: result.OK, Error: result.Error, Snapshot: routePublicAccessSnapshot(result.Snapshot),
		})
	}
	app.loadPublicAccessLocked(app.contextSnapshot())
	if routePublicAccessCommandRequest(payload).ExpectedRevision != app.publicAccessPreferences.Revision {
		return app.publicAccessFailureLocked(tunnelservice.ErrorConflict, tunnelservice.ErrorConflict.SafeMessage())
	}
	app.mu.Lock()
	app.publicAccessStatus = tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleDisabled, Generation: app.publicAccessStatus.Generation + 1, SettingsRevision: app.publicAccessPreferences.Revision}
	app.mu.Unlock()
	return app.publicAccessSuccessLocked()
}

func (app *App) loadPublicAccessLocked(ctx context.Context) {
	app.mu.RLock()
	loaded := app.publicAccessLoaded
	app.mu.RUnlock()
	if loaded {
		return
	}
	preferences := tunnelservice.DefaultPublicAccessPreferences()
	category := tunnelservice.ErrorCategory(0)
	message := ""
	if app.deps.PublicSettings != nil {
		loadedPreferences, err := app.deps.PublicSettings.Load()
		if err == nil {
			preferences = loadedPreferences
		} else if errors.Is(err, tunnelservice.ErrSettingsRecovered) {
			category, message = tunnelservice.ErrorSettingsCorrupt, tunnelservice.ErrorSettingsCorrupt.SafeMessage()
		} else {
			category, message = tunnelservice.ErrorSettingsCorrupt, tunnelservice.ErrorSettingsCorrupt.SafeMessage()
		}
	}
	app.mu.Lock()
	app.publicAccessLoaded = true
	app.publicAccessPreferences = preferences
	app.publicAccessStatus = tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleDisabled, SettingsRevision: preferences.Revision}
	if category != 0 {
		app.publicAccessStatus.State = tunnelservice.LifecycleFailed
		app.publicAccessStatus.ErrorCategory = category
		app.publicAccessStatus.ErrorMessage = message
	}
	app.mu.Unlock()
	app.reconcilePublicAccessPresenceLocked(ctx)
}

func (app *App) reconcilePublicAccessPresenceLocked(ctx context.Context) {
	provider, providerErr := tunnelservice.SecretUnknown, error(nil)
	password, passwordErr := tunnelservice.SecretUnknown, error(nil)
	if app.deps.PublicSecrets != nil {
		provider, providerErr = app.deps.PublicSecrets.Presence(ctx, tunnelservice.ProviderAccountToken)
		password, passwordErr = app.deps.PublicSecrets.Presence(ctx, tunnelservice.PlayerBasicAuthPassword)
	}
	app.mu.Lock()
	app.providerTokenPresence = provider
	app.playerPasswordPresence = password
	if providerErr != nil || passwordErr != nil {
		failure := providerErr
		if failure == nil {
			failure = passwordErr
		}
		category := publicAccessSecretCategory(failure)
		app.publicAccessStatus = tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleFailed, Generation: app.publicAccessStatus.Generation, SettingsRevision: app.publicAccessPreferences.Revision, ErrorCategory: category, ErrorMessage: category.SafeMessage()}
	}
	app.mu.Unlock()
}

func (app *App) applySecretMutationLocked(ctx context.Context, ref tunnelservice.SecretRef, replacement []byte, deleteValue bool) error {
	if len(replacement) > 0 {
		return tunnelservice.ReplaceSecret(ctx, app.deps.PublicSecrets, ref, replacement)
	}
	if deleteValue {
		return tunnelservice.DeleteSecret(ctx, app.deps.PublicSecrets, ref)
	}
	return nil
}

func (app *App) publicAccessSnapshotLocked() PublicAccessSnapshot {
	app.mu.RLock()
	snapshot := tunnelservice.PublicAccessSnapshot{Preferences: app.publicAccessPreferences, ProviderTokenPresence: app.providerTokenPresence, PlayerPasswordPresence: app.playerPasswordPresence, Status: app.publicAccessStatus}
	app.mu.RUnlock()
	return routePublicAccessSnapshot(snapshot)
}

func (app *App) publicAccessSuccessLocked() PublicAccessCommandResult {
	snapshot := app.publicAccessSnapshotLocked()
	app.emitPublicAccessStatusLocked()
	return routePublicAccessCommandResult(PublicAccessCommandResult{OK: true, Snapshot: snapshot})
}

func (app *App) publicAccessFailureLocked(category tunnelservice.ErrorCategory, message string) PublicAccessCommandResult {
	app.mu.Lock()
	app.publicAccessStatus = tunnelservice.PublicAccessStatus{State: tunnelservice.LifecycleFailed, Generation: app.publicAccessStatus.Generation, SettingsRevision: app.publicAccessPreferences.Revision, ErrorCategory: category, ErrorMessage: message}
	app.mu.Unlock()
	snapshot := app.publicAccessSnapshotLocked()
	app.emitPublicAccessStatusLocked()
	return routePublicAccessCommandResult(PublicAccessCommandResult{Error: message, Snapshot: snapshot})
}

func (app *App) publicAccessSecretFailureLocked(err error) PublicAccessCommandResult {
	category := publicAccessSecretCategory(err)
	return app.publicAccessFailureLocked(category, category.SafeMessage())
}

func publicAccessSecretCategory(err error) tunnelservice.ErrorCategory {
	switch {
	case errors.Is(err, tunnelservice.ErrSecretStoreLocked):
		return tunnelservice.ErrorSecretStoreLocked
	case errors.Is(err, tunnelservice.ErrSecretStoreDenied), errors.Is(err, tunnelservice.ErrSecretStoreUserCancelled):
		return tunnelservice.ErrorSecretStoreDenied
	default:
		return tunnelservice.ErrorSecretStoreUnavailable
	}
}

func (app *App) emitPublicAccessStatusLocked() {
	if app.deps.Events != nil {
		app.emitEvent(publicAccessStatusEvent, app.publicAccessSnapshotLocked())
	}
}

func (app *App) acceptPublicAccessSnapshot(snapshot tunnelservice.PublicAccessSnapshot, emit bool) {
	app.mu.Lock()
	if app.publicAccessLoaded && publicAccessSnapshotOlder(snapshot, app.publicAccessPreferences, app.publicAccessStatus) {
		app.mu.Unlock()
		return
	}
	app.publicAccessLoaded = true
	app.publicAccessPreferences = snapshot.Preferences
	app.providerTokenPresence = snapshot.ProviderTokenPresence
	app.playerPasswordPresence = snapshot.PlayerPasswordPresence
	app.publicAccessStatus = snapshot.Status
	serverInfoChanged := false
	if app.serverInfo != nil {
		if snapshot.Status.State == tunnelservice.LifecycleReady && snapshot.Status.PublicURL != "" {
			localURL := app.serverInfo.LocalURL
			if localURL == "" || !app.serverInfo.Tunnel {
				localURL = app.serverInfo.URL
			}
			if app.serverInfo.URL != snapshot.Status.PublicURL || !app.serverInfo.Tunnel {
				app.serverInfo.URL = snapshot.Status.PublicURL
				app.serverInfo.LocalURL = localURL
				app.serverInfo.Tunnel = true
				app.serverInfo.TunnelError = ""
				serverInfoChanged = true
			}
		} else if app.serverInfo.Tunnel {
			app.serverInfo.URL = app.serverInfo.LocalURL
			app.serverInfo.Tunnel = false
			app.serverInfo.TunnelError = snapshot.Status.ErrorMessage
			serverInfoChanged = true
		}
	}
	serverInfo := cloneServerInfoPointer(app.serverInfo)
	app.mu.Unlock()
	if emit && app.deps.Events != nil {
		if serverInfoChanged && serverInfo != nil {
			app.emitEvent(serverInfoEvent, routeServerInfoEvent(*serverInfo))
		}
		app.emitEvent(publicAccessStatusEvent, routePublicAccessSnapshot(snapshot))
	}
}

func (app *App) publicAccessCoreFailure(snapshot tunnelservice.PublicAccessSnapshot, category tunnelservice.ErrorCategory) PublicAccessCommandResult {
	app.acceptPublicAccessSnapshot(snapshot, false)
	return routePublicAccessCommandResult(PublicAccessCommandResult{
		Error: category.SafeMessage(), Snapshot: routePublicAccessSnapshot(snapshot),
	})
}

func publicAccessSnapshotOlder(candidate tunnelservice.PublicAccessSnapshot, preferences tunnelservice.PublicAccessPreferences, status tunnelservice.PublicAccessStatus) bool {
	return candidate.Status.Generation < status.Generation ||
		candidate.Status.Generation == status.Generation && candidate.Preferences.Revision < preferences.Revision
}

// Start acquires the player listener, publishes local status, allows the
// desktop to become ready, then starts optional public access. Fatal partial
// startup is unwound in reverse acquisition order.
func (app *App) Start(ctx context.Context) error {
	app.lifecycleMu.Lock()
	defer app.lifecycleMu.Unlock()
	if ctx == nil {
		return errApplicationContextRequired
	}
	app.mu.RLock()
	alreadyStarted := app.playerStarted
	app.mu.RUnlock()
	if alreadyStarted {
		return nil
	}
	if app.log != nil {
		app.log.WithField("phase", "starting").Info("application startup started")
	}
	if app.deps.Player == nil {
		return app.failLocked(errors.New("player server is not configured"))
	}
	runtimeContext, cancel := context.WithCancelCause(ctx)
	if setter, ok := app.deps.Events.(interface{ SetContext(context.Context) }); ok {
		setter.SetContext(runtimeContext)
	}
	app.mu.Lock()
	app.ctx = runtimeContext
	app.cancel = cancel
	app.stopped = false
	app.mu.Unlock()
	if app.deps.PublicAccess == nil {
		app.publicAccessCommandMu.Lock()
		app.loadPublicAccessLocked(runtimeContext)
		app.publicAccessCommandMu.Unlock()
	}

	acquisitionContext := runtimeContext
	if app.deps.StartupTimeout > 0 {
		deadlineContext, stopDeadline := context.WithTimeoutCause(runtimeContext, app.deps.StartupTimeout, errApplicationStartupTimeout)
		bounded, cancel := context.WithCancelCause(deadlineContext)
		defer func() {
			cancel(errApplicationStartupComplete)
			stopDeadline()
		}()
		acquisitionContext = bounded
	}
	app.coordinationCommandMu.Lock()
	restoreErr := app.restoreActiveSessionFacility()
	app.coordinationCommandMu.Unlock()
	if restoreErr != nil {
		return app.failLocked(errors.New("restore active session facility"))
	}

	app.setPhase("starting-player-server")
	info, err := app.deps.Player.Start(acquisitionContext)
	if err != nil {
		return app.failLocked(fmt.Errorf("start player server: %w", err))
	}
	app.mu.Lock()
	app.playerStarted = true
	app.serverInfo = cloneServerInfo(info)
	app.mu.Unlock()
	if app.log != nil {
		app.log.WithField("port", info.Port).Info("player server ready")
	}
	if app.deps.PublicAccess != nil {
		app.acceptPublicAccessSnapshot(app.deps.PublicAccess.Initialize(runtimeContext), false)
	}

	if app.deps.Events != nil {
		if err := app.deps.Events.Emit(serverInfoEvent, routeServerInfoEvent(info)); err != nil {
			return app.failLocked(fmt.Errorf("publish player server status to desktop bridge: %w", err))
		}
	}

	app.setPhase("desktop-loading")
	if app.deps.Desktop != nil {
		if err := app.deps.Desktop.Ready(runtimeContext); err != nil {
			return app.failLocked(fmt.Errorf("make desktop ready: %w", err))
		}
		app.mu.Lock()
		app.desktopReady = true
		app.mu.Unlock()
		if app.log != nil {
			app.log.Info("desktop runtime ready")
		}
	}
	app.setPhase("ready-local")
	if app.log != nil {
		app.log.WithField("phase", "ready-local").Info("application ready")
	}
	if app.deps.PublicAccess != nil || app.deps.PublicSettings != nil || app.deps.PublicSecrets != nil {
		app.publicAccessCommandMu.Lock()
		app.emitPublicAccessStatusLocked()
		app.publicAccessCommandMu.Unlock()
	}

	return nil
}

// Shutdown releases public access, the player listener, persistence worker, then desktop.
// It is safe in every lifecycle phase and on repeated calls.
func (app *App) Shutdown(ctx context.Context) error {
	app.lifecycleMu.Lock()
	defer app.lifecycleMu.Unlock()
	if ctx == nil {
		return errApplicationContextRequired
	}
	app.mu.RLock()
	alreadyStopped := app.stopped
	app.mu.RUnlock()
	if alreadyStopped {
		return nil
	}
	app.cancelRuntime(errApplicationShutdown)
	if app.log != nil {
		app.log.WithField("phase", "stopping").Info("application shutdown started")
	}
	cleanupContext, cancel := app.shutdownContext(ctx)
	defer cancel(errApplicationCleanupComplete)
	err := app.shutdownLocked(cleanupContext, false)
	if err == nil && app.log != nil {
		app.log.WithField("phase", "stopped").Info("application shutdown completed")
	}
	return err
}

// GetRuntimeStatus returns a detached status snapshot.
func (app *App) GetRuntimeStatus() RuntimeStatus {
	// The coordinator is the canonical owner of transient request state. Read it
	// before the app lock so a frontend reload can recover a request even when
	// its original bridge event was published before the new listener existed.
	var canonicalCoordination *domain.MasterCoordinationState
	if app.deps.Coordination != nil {
		canonicalCoordination = domain.CloneMasterCoordinationState(app.deps.Coordination.Snapshot())
	}

	app.mu.Lock()
	defer app.mu.Unlock()
	if canonicalCoordination != nil &&
		(app.coordinationState == nil || canonicalCoordination.Revision >= app.coordinationState.Revision) {
		app.coordinationState = canonicalCoordination
	}
	status := RuntimeStatus{
		ServerInfo:        cloneServerInfoPointer(app.serverInfo),
		ClientCount:       app.clientCount,
		HackState:         clonePublicHackState(app.hackState),
		StartupError:      app.startupError,
		SaveState:         app.saveState,
		RequestedRevision: app.requestedRevision,
		SavedRevision:     app.savedRevision,
		CoordinationState: domain.CloneMasterCoordinationState(app.coordinationState),
	}
	return routeRuntimeStatus(status)
}

// GetApplicationUpdateStatus returns immediately and lets the manager arm its
// launch-scoped check only after the frontend has installed its event listener.
func (app *App) GetApplicationUpdateStatus() ApplicationUpdateSnapshot {
	if app.deps.Updates == nil {
		return ApplicationUpdateSnapshot{State: string(updateservice.UpdateStateDisabled)}
	}
	app.mu.RLock()
	ready := app.desktopReady && app.phase == "ready-local"
	app.mu.RUnlock()
	if !ready {
		return nativeApplicationUpdateSnapshot(app.deps.Updates.Snapshot())
	}
	return nativeApplicationUpdateSnapshot(app.deps.Updates.Status(app.contextSnapshot()))
}

// publishApplicationUpdateSnapshot is the only update-state event boundary.
// The manager retains the authoritative state; the core only converts its
// detached, safe projection after local startup has completed.
func (app *App) publishApplicationUpdateSnapshot(snapshot updateservice.UpdateSnapshot) {
	app.emitEvent(applicationUpdateStatusEvent, nativeApplicationUpdateSnapshot(snapshot))
}

// ResolveApplicationUpdateOffer validates the authored decision vocabulary
// before forwarding it to the application-owned manager.
func (app *App) ResolveApplicationUpdateOffer(payload ApplicationUpdateOfferDecisionPayload) ApplicationUpdateCommandResult {
	routed, err := routeApplicationUpdateOfferDecisionRequest(payload)
	if err != nil {
		return app.applicationUpdateFailure(err.Error())
	}
	if app.deps.Updates == nil {
		return app.applicationUpdateFailure("Application update service is unavailable.")
	}
	return nativeApplicationUpdateCommandResult(app.deps.Updates.ResolveOffer(
		app.contextSnapshot(), routed.AttemptID, updateservice.OfferDecision(routed.Decision),
	))
}

// ResolveApplicationUpdateRestart forwards only validated restart decisions.
func (app *App) ResolveApplicationUpdateRestart(payload ApplicationUpdateRestartDecisionPayload) ApplicationUpdateCommandResult {
	routed, err := routeApplicationUpdateRestartDecisionRequest(payload)
	if err != nil {
		return app.applicationUpdateFailure(err.Error())
	}
	if app.deps.Updates == nil {
		return app.applicationUpdateFailure("Application update service is unavailable.")
	}
	return nativeApplicationUpdateCommandResult(app.deps.Updates.ResolveRestart(
		app.contextSnapshot(), routed.AttemptID, updateservice.RestartDecision(routed.Decision),
	))
}

func (app *App) applicationUpdateFailure(message string) ApplicationUpdateCommandResult {
	snapshot := ApplicationUpdateSnapshot{State: string(updateservice.UpdateStateDisabled)}
	if app.deps.Updates != nil {
		snapshot = nativeApplicationUpdateSnapshot(app.deps.Updates.Snapshot())
	}
	return ApplicationUpdateCommandResult{Error: message, Snapshot: snapshot}
}

func nativeApplicationUpdateSnapshot(snapshot updateservice.UpdateSnapshot) ApplicationUpdateSnapshot {
	return applicationUpdateSnapshotFromPrivate(applicationUpdateSnapshotToPrivate(snapshot))
}

func nativeApplicationUpdateCommandResult(result updateservice.CommandResult) ApplicationUpdateCommandResult {
	return ApplicationUpdateCommandResult{
		OK:       result.OK,
		Error:    result.Error,
		Snapshot: nativeApplicationUpdateSnapshot(result.Snapshot),
	}
}

// NewSession opens the native destination dialog and creates a validated
// starter session.
func (app *App) NewSession() sessionservice.SessionResult {
	return app.runSessionCommand("session.create", sessionCommands.Create)
}

// OpenSession opens and validates an existing version-1 session.
func (app *App) OpenSession() sessionservice.SessionResult {
	return app.runSessionCommand("session.open", sessionCommands.Open)
}

// CopyDemo creates an explicit writable copy of the bundled demo.
func (app *App) CopyDemo() sessionservice.SessionResult {
	return app.runSessionCommand("session.copy-demo", sessionCommands.CopyDemo)
}

func (app *App) runSessionCommand(
	operation string,
	command func(sessionCommands, context.Context) sessionservice.SessionResult,
) (result sessionservice.SessionResult) {
	defer func() {
		app.recordOperation(operation, operationOutcome(result.OK, result.Canceled), nil)
	}()
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()

	commands, ok := app.deps.Sessions.(sessionCommands)
	if !ok {
		return sessionservice.SessionResult{Error: "session service is unavailable"}
	}
	commandResult := command(commands, app.contextSnapshot())
	app.captureSessionStatus(commands)
	if commandResult.OK {
		app.resetSessionStateOrdering()
		if err := app.replaceSessionFacility(commandResult.Session); err != nil {
			commandResult.OK = false
			commandResult.Error = "session facility could not be restored"
		}
	}
	app.resetPlayerConfigForSession(commandResult)
	return routeSessionOperationResult(commandResult)
}

func (app *App) restoreActiveSessionFacility() error {
	sessions, ok := app.deps.Sessions.(sessionSnapshotService)
	if !ok {
		return nil
	}
	active := sessions.Snapshot()
	app.captureActiveSessionStatus(active)
	if active.Session == nil {
		return nil
	}
	app.resetSessionStateOrdering()
	return app.replaceSessionFacility(active.Session)
}

func (app *App) replaceSessionFacility(session *domain.Session) error {
	coordination, ok := app.deps.Coordination.(coordinationFacilityLifecycleService)
	if !ok {
		return nil
	}
	if session == nil {
		return errors.New("loaded session is unavailable")
	}
	state, err := coordination.ReplaceFacility(domain.CloneFacility(session.Facility))
	if err != nil {
		return err
	}
	if state == nil {
		return errors.New("facility replacement returned no coordination state")
	}
	app.publishCoordinationStateIfNewer(state)
	return nil
}

// SaveSession assigns a monotonic revision and waits until it or a newer
// accepted revision is durably replaced.
func (app *App) SaveSession(session domain.Session) (result sessionservice.SaveResult) {
	defer func() {
		app.recordOperation("session.save", operationOutcome(result.OK, false), logger.Fields{"revision": result.RequestedRevision})
	}()
	commands, ok := app.deps.Sessions.(sessionCommands)
	if !ok {
		return sessionservice.SaveResult{Error: "session service is unavailable"}
	}
	app.mu.Lock()
	app.requestedRevision++
	revision := app.requestedRevision
	app.saveState = string(sessionservice.SaveStateSaving)
	app.mu.Unlock()

	commandResult := commands.Save(app.contextSnapshot(), session, revision)
	app.mu.Lock()
	if commandResult.SavedRevision > app.savedRevision {
		app.savedRevision = commandResult.SavedRevision
	}
	if revision == app.requestedRevision {
		if commandResult.OK {
			app.saveState = string(sessionservice.SaveStateSaved)
		} else {
			app.saveState = string(sessionservice.SaveStateFailed)
		}
	}
	app.mu.Unlock()
	return routeSaveSessionResult(commandResult)
}

// ReplaceTerminalGroups routes the one private complete-set mutation through
// the coordinator, then publishes the durable session before the matching
// coordination revision.
func (app *App) ReplaceTerminalGroups(payload TerminalGroupReplacementPayload) TerminalGroupReplacementResult {
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()
	payload = routeTerminalGroupReplacementRequest(payload)

	coordination, ok := app.deps.Coordination.(coordinationTerminalGroupService)
	if !ok {
		return routeTerminalGroupReplacementResult(app.terminalGroupReplacementFailure("coordination service is unavailable", nil))
	}
	candidate := domain.TerminalGroupCandidate{
		TerminalGroups:               domain.CloneTerminalGroups(payload.TerminalGroups),
		ExpectedSessionRevision:      payload.ExpectedSessionRevision,
		ExpectedCoordinationRevision: payload.ExpectedCoordinationRevision,
	}
	state, mutation, err := coordination.ReplaceTerminalGroups(app.contextSnapshot(), candidate)
	if err != nil {
		result := app.terminalGroupReplacementFailure(err.Error(), state)
		if mutation != nil {
			result.SessionRevision = mutation.Revision
			if mutation.Session.Version != 0 {
				result.Session = sessionPointerForApp(mutation.Session)
			}
		}
		return routeTerminalGroupReplacementResult(result)
	}
	if mutation == nil {
		return routeTerminalGroupReplacementResult(app.terminalGroupReplacementFailure("terminal groups were not replaced", state))
	}
	canonicalSession := domain.CloneSession(mutation.Session)
	app.acceptSessionStateRevision(mutation.Revision)
	if mutation.Changed {
		app.publishSessionState(SessionStateEvent{Revision: mutation.Revision, Session: &canonicalSession})
		app.publishCoordinationState(state)
	}
	return routeTerminalGroupReplacementResult(TerminalGroupReplacementResult{
		OK:                true,
		SessionRevision:   mutation.Revision,
		Session:           sessionPointerForApp(canonicalSession),
		CoordinationState: domain.CloneMasterCoordinationState(state),
	})
}

func (app *App) terminalGroupReplacementFailure(message string, state *domain.MasterCoordinationState) TerminalGroupReplacementResult {
	result := TerminalGroupReplacementResult{
		Error:             message,
		CoordinationState: domain.CloneMasterCoordinationState(state),
	}
	if result.CoordinationState == nil {
		app.mu.RLock()
		result.CoordinationState = domain.CloneMasterCoordinationState(app.coordinationState)
		app.mu.RUnlock()
	}
	if sessions, ok := app.deps.Sessions.(sessionCommands); ok {
		active := sessions.Snapshot()
		app.captureSessionStatus(sessions)
		result.SessionRevision = active.SavedRevision
		if active.Session != nil {
			result.Session = sessionPointerForApp(*active.Session)
		}
	}
	return result
}

// SaveFacilityAuthoring validates one complete private draft through the
// protobuf contract and publishes only a changed canonical durable result.
func (app *App) SaveFacilityAuthoring(payload FacilityAuthoringPayload) domain.FacilityOperationResult {
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()

	if payload.Session == nil {
		return facilityOperationFailureForApp(
			payload.CorrelationID,
			domain.FacilityFailureInvalidConfiguration,
			"session",
			payload.ExpectedSessionRevision,
			payload.ExpectedFacilityRevision,
		)
	}
	request := controlservice.FacilityAuthoringRequest{
		Candidate:                domain.CloneSession(*payload.Session),
		ExpectedSessionRevision:  payload.ExpectedSessionRevision,
		ExpectedFacilityRevision: payload.ExpectedFacilityRevision,
		CorrelationID:            payload.CorrelationID,
	}
	semantic, err := facilityAuthoringRequestToPrivate(request)
	if err != nil {
		return facilityOperationFailureForApp(
			payload.CorrelationID,
			domain.FacilityFailureInvalidConfiguration,
			"session",
			payload.ExpectedSessionRevision,
			payload.ExpectedFacilityRevision,
		)
	}
	routed, err := facilityAuthoringRequestFromPrivate(semantic, *payload.Session)
	if err != nil {
		return facilityOperationFailureForApp(
			payload.CorrelationID,
			domain.FacilityFailureInvalidConfiguration,
			"session",
			payload.ExpectedSessionRevision,
			payload.ExpectedFacilityRevision,
		)
	}
	coordination, ok := app.deps.Coordination.(coordinationFacilityAuthoringService)
	if !ok {
		return facilityOperationFailureForApp(
			routed.CorrelationID,
			domain.FacilityFailureRuntimeContextEnded,
			"request",
			routed.ExpectedSessionRevision,
			routed.ExpectedFacilityRevision,
		)
	}
	masterEvents := app.deferMasterEvents()
	defer masterEvents.Commit()

	state, result, _ := coordination.SaveFacilityAuthoring(app.contextSnapshot(), routed)
	if result == nil {
		masterEvents.Discard(state)
		return facilityOperationFailureForApp(
			routed.CorrelationID,
			domain.FacilityFailurePersistenceFailed,
			"facility",
			routed.ExpectedSessionRevision,
			routed.ExpectedFacilityRevision,
		)
	}
	detached, conversionErr := detachedFacilityOperationResultForApp(*result)
	if conversionErr != nil || !validFacilityAuthoringResultForApp(detached, routed) ||
		(detached.OK && detached.Changed && !app.canPublishFacilityMutationState(
			state,
			detached.SessionRevision,
			detached.ResultingFacilityRevision,
		)) {
		masterEvents.Discard(state)
		return facilityOperationFailureForApp(
			routed.CorrelationID,
			domain.FacilityFailurePersistenceFailed,
			"facility",
			routed.ExpectedSessionRevision,
			routed.ExpectedFacilityRevision,
		)
	}
	if detached.OK && detached.Changed && detached.Session != nil {
		canonical := domain.CloneSession(*detached.Session)
		app.acceptSessionStateRevision(detached.SessionRevision)
		app.publishSessionState(SessionStateEvent{Revision: detached.SessionRevision, Session: &canonical})
		masterEvents.Commit()
		app.publishCoordinationStateIfNewer(state)
	} else {
		masterEvents.Discard(state)
	}
	return detached
}

func validFacilityAuthoringResultForApp(
	result domain.FacilityOperationResult,
	request controlservice.FacilityAuthoringRequest,
) bool {
	if result.CorrelationID != request.CorrelationID {
		return false
	}
	if !result.OK {
		return !result.Changed && knownFacilityRecoveryFailureForApp(result.Failure) &&
			result.PreviousFacilityRevision == result.ResultingFacilityRevision &&
			len(result.AffectedDeviceIDs) == 0 && len(result.AffectedConditionIDs) == 0 &&
			(result.Session == nil || facilityRevisionForApp(result.Session.Facility) == result.ResultingFacilityRevision)
	}
	if result.Failure != "" && result.Failure != domain.FacilityFailureUnspecified || result.Session == nil ||
		result.PreviousFacilityRevision != request.ExpectedFacilityRevision ||
		hasDuplicateStrings(result.AffectedDeviceIDs) || hasDuplicateStrings(result.AffectedConditionIDs) {
		return false
	}
	wantSessionRevision := request.ExpectedSessionRevision
	wantFacilityRevision := request.ExpectedFacilityRevision
	if result.Changed {
		wantSessionRevision++
		if result.Session.Facility != nil {
			wantFacilityRevision++
		} else {
			wantFacilityRevision = 0
		}
	}
	if !result.Changed && (len(result.AffectedDeviceIDs) != 0 || len(result.AffectedConditionIDs) != 0) {
		return false
	}
	return result.SessionRevision == wantSessionRevision &&
		result.ResultingFacilityRevision == wantFacilityRevision &&
		facilityRevisionForApp(result.Session.Facility) == result.ResultingFacilityRevision
}

func facilityRevisionForApp(facility *domain.Facility) uint64 {
	if facility == nil {
		return 0
	}
	return facility.Revision
}

// RecoverFacilityCondition routes one authored private recovery through the
// coordinator and publishes the canonical durable session before its matching
// coordination state becomes visible.
func (app *App) RecoverFacilityCondition(payload FacilityRecoveryPayload) domain.FacilityOperationResult {
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()

	if payload.ConditionID == "" || strings.TrimSpace(payload.ConditionID) != payload.ConditionID ||
		payload.CorrelationID == "" || strings.TrimSpace(payload.CorrelationID) != payload.CorrelationID {
		return facilityOperationFailureForApp(
			payload.CorrelationID,
			domain.FacilityFailureInvalidConfiguration,
			"request",
			0,
			payload.ExpectedFacilityRevision,
		)
	}
	request := controlservice.FacilityRecoveryRequest{
		ConditionID:              payload.ConditionID,
		ExpectedFacilityRevision: payload.ExpectedFacilityRevision,
		CorrelationID:            payload.CorrelationID,
		Recovery:                 cloneDiagnosticRecoveryReference(payload.Recovery),
	}
	semantic, err := facilityRecoveryRequestToPrivate(request)
	if err != nil {
		return facilityOperationFailureForApp(
			payload.CorrelationID,
			domain.FacilityFailureInvalidConfiguration,
			"request",
			0,
			payload.ExpectedFacilityRevision,
		)
	}
	routed, err := facilityRecoveryRequestFromPrivate(semantic)
	if err != nil {
		return facilityOperationFailureForApp(
			payload.CorrelationID,
			domain.FacilityFailureInvalidConfiguration,
			"request",
			0,
			payload.ExpectedFacilityRevision,
		)
	}
	coordination, ok := app.deps.Coordination.(coordinationFacilityRecoveryService)
	if !ok {
		return facilityOperationFailureForApp(
			routed.CorrelationID,
			domain.FacilityFailureRuntimeContextEnded,
			"request",
			0,
			routed.ExpectedFacilityRevision,
		)
	}

	masterEvents := app.deferMasterEvents()
	defer masterEvents.Commit()

	state, result, _ := coordination.RecoverFacilityCondition(app.contextSnapshot(), routed)
	if result == nil {
		masterEvents.Discard(state)
		return facilityOperationFailureForApp(
			routed.CorrelationID,
			domain.FacilityFailurePersistenceFailed,
			"facility",
			0,
			routed.ExpectedFacilityRevision,
		)
	}
	detached, conversionErr := detachedFacilityOperationResultForApp(*result)
	if conversionErr != nil || detached.CorrelationID != routed.CorrelationID ||
		!validFacilityRecoveryResultForApp(detached, routed) ||
		(detached.OK && detached.Changed && !app.canPublishFacilityMutationState(
			state,
			detached.SessionRevision,
			detached.ResultingFacilityRevision,
		)) {
		masterEvents.Discard(state)
		return facilityOperationFailureForApp(
			routed.CorrelationID,
			domain.FacilityFailurePersistenceFailed,
			"facility",
			0,
			routed.ExpectedFacilityRevision,
		)
	}
	if detached.OK && detached.Changed && detached.Session != nil {
		canonical := domain.CloneSession(*detached.Session)
		app.acceptSessionStateRevision(detached.SessionRevision)
		app.publishSessionState(SessionStateEvent{Revision: detached.SessionRevision, Session: &canonical})
		masterEvents.Commit()
		app.publishCoordinationStateIfNewer(state)
	} else {
		masterEvents.Discard(state)
	}
	return detached
}

func validFacilityRecoveryResultForApp(
	result domain.FacilityOperationResult,
	request controlservice.FacilityRecoveryRequest,
) bool {
	if !result.OK {
		return !result.Changed && knownFacilityRecoveryFailureForApp(result.Failure) &&
			result.CorrelationID == request.CorrelationID && result.Session == nil &&
			result.PreviousFacilityRevision == result.ResultingFacilityRevision &&
			len(result.AffectedDeviceIDs) == 0 && len(result.AffectedConditionIDs) == 0
	}
	if result.Failure != "" && result.Failure != domain.FacilityFailureUnspecified {
		return false
	}
	if result.PreviousFacilityRevision != request.ExpectedFacilityRevision ||
		len(result.AffectedDeviceIDs) != 0 {
		return false
	}
	if !result.Changed {
		return result.ResultingFacilityRevision == result.PreviousFacilityRevision &&
			len(result.AffectedConditionIDs) == 0
	}
	return result.SessionRevision != 0 && result.Session != nil && result.Session.Facility != nil &&
		result.ResultingFacilityRevision == result.PreviousFacilityRevision+1 &&
		result.Session.Facility.Revision == result.ResultingFacilityRevision &&
		len(result.AffectedConditionIDs) == 1 && result.AffectedConditionIDs[0] == request.ConditionID
}

func knownFacilityRecoveryFailureForApp(failure domain.FacilityFailureCode) bool {
	switch failure {
	case domain.FacilityFailureMissingReference,
		domain.FacilityFailureInvalidTransition,
		domain.FacilityFailurePreconditionFailed,
		domain.FacilityFailureStaleRevision,
		domain.FacilityFailureConflict,
		domain.FacilityFailureDuplicate,
		domain.FacilityFailureInvalidConfiguration,
		domain.FacilityFailurePersistenceFailed,
		domain.FacilityFailureRuntimeContextEnded:
		return true
	case "", domain.FacilityFailureUnspecified, domain.FacilityFailureRejected:
		return false
	default:
		return false
	}
}

func (app *App) deferMasterEvents() coordinationMasterEventDeferral {
	if app.deps.CoordinationEvents != nil {
		return app.deps.CoordinationEvents.DeferMasterEvents()
	}
	return noopCoordinationMasterEventDeferral{}
}

// PreviewFacility validates one private preview request through the protobuf
// boundary and returns a detached projection without publishing runtime state.
func (app *App) PreviewFacility(payload FacilityPreviewPayload) FacilityPreviewResult {
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()

	request := domain.FacilityPreview{
		ExpectedFacilityRevision: payload.ExpectedFacilityRevision,
		TerminalID:               payload.TerminalID,
		DeviceState:              cloneFacilityDeviceStatePreview(payload.DeviceState),
		Condition:                cloneFacilityConditionPreview(payload.Condition),
	}
	semantic, err := facilityPreviewRequestToPrivate(request)
	if err != nil || !validFacilityPreviewRequestForApp(request) {
		return facilityPreviewFailureForApp(
			domain.FacilityFailureInvalidConfiguration,
			payload.ExpectedFacilityRevision,
			"request",
		)
	}
	routed, err := facilityPreviewRequestFromPrivate(semantic)
	if err != nil {
		return facilityPreviewFailureForApp(
			domain.FacilityFailureInvalidConfiguration,
			payload.ExpectedFacilityRevision,
			"request",
		)
	}
	coordination, ok := app.deps.Coordination.(coordinationFacilityPreviewService)
	if !ok {
		return facilityPreviewFailureForApp(
			domain.FacilityFailureRuntimeContextEnded,
			payload.ExpectedFacilityRevision,
			"preview",
		)
	}

	result := coordination.PreviewFacility(app.contextSnapshot(), routed)
	detached, conversionErr := facilityPreviewResultFromPrivate(facilityPreviewResultToPrivate(result))
	if conversionErr != nil || !validFacilityPreviewResultForApp(detached, routed) {
		return facilityPreviewFailureForApp(
			domain.FacilityFailurePersistenceFailed,
			payload.ExpectedFacilityRevision,
			"preview",
		)
	}
	return FacilityPreviewResult{
		OK:               detached.OK,
		Failure:          detached.Failure,
		Issues:           detached.Issues,
		FacilityRevision: detached.FacilityRevision,
		Terminal:         detached.Terminal,
	}
}

func validFacilityPreviewRequestForApp(preview domain.FacilityPreview) bool {
	if preview.TerminalID == "" || strings.TrimSpace(preview.TerminalID) != preview.TerminalID {
		return false
	}
	if preview.DeviceState != nil {
		return preview.Condition == nil && preview.DeviceState.DeviceID != "" && preview.DeviceState.StateID != "" &&
			strings.TrimSpace(preview.DeviceState.DeviceID) == preview.DeviceState.DeviceID &&
			strings.TrimSpace(preview.DeviceState.StateID) == preview.DeviceState.StateID
	}
	return preview.Condition != nil && preview.Condition.ConditionID != "" &&
		strings.TrimSpace(preview.Condition.ConditionID) == preview.Condition.ConditionID
}

func validFacilityPreviewResultForApp(
	result controlservice.FacilityPreviewResult,
	request domain.FacilityPreview,
) bool {
	if !result.OK {
		return result.Terminal == nil && knownFacilityRecoveryFailureForApp(result.Failure)
	}
	return result.Failure == "" && result.FacilityRevision == request.ExpectedFacilityRevision &&
		result.Terminal != nil && result.Terminal.TerminalID == request.TerminalID
}

func facilityPreviewFailureForApp(
	failure domain.FacilityFailureCode,
	facilityRevision uint64,
	entityKind string,
) FacilityPreviewResult {
	result, err := facilityPreviewResultFromPrivate(facilityPreviewResultToPrivate(controlservice.FacilityPreviewResult{
		Failure: failure,
		Issues: []domain.FacilityIssue{{
			Code: failure, EntityKind: entityKind,
		}},
		FacilityRevision: facilityRevision,
	}))
	if err != nil {
		return FacilityPreviewResult{
			Failure: failure,
			Issues:  []domain.FacilityIssue{{Code: failure, EntityKind: entityKind}},
		}
	}
	return FacilityPreviewResult{
		Failure:          result.Failure,
		Issues:           result.Issues,
		FacilityRevision: result.FacilityRevision,
	}
}

func cloneFacilityDeviceStatePreview(preview *domain.FacilityDeviceStatePreview) *domain.FacilityDeviceStatePreview {
	if preview == nil {
		return nil
	}
	clone := *preview
	return &clone
}

func cloneFacilityConditionPreview(preview *domain.FacilityConditionPreview) *domain.FacilityConditionPreview {
	if preview == nil {
		return nil
	}
	clone := *preview
	return &clone
}

// ResetFacilityDevice restores one device and its directly scoped conditions
// through the serialized coordinator operation.
func (app *App) ResetFacilityDevice(payload FacilityDeviceResetPayload) domain.FacilityOperationResult {
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()

	request := controlservice.FacilityDeviceResetRequest{
		DeviceID:                 payload.DeviceID,
		ExpectedFacilityRevision: payload.ExpectedFacilityRevision,
		CorrelationID:            payload.CorrelationID,
	}
	routed := facilityDeviceResetRequestFromPrivate(facilityDeviceResetRequestToPrivate(request))
	if routed.DeviceID == "" || strings.TrimSpace(routed.DeviceID) != routed.DeviceID ||
		routed.CorrelationID == "" || strings.TrimSpace(routed.CorrelationID) != routed.CorrelationID {
		return facilityOperationFailureForApp(
			routed.CorrelationID,
			domain.FacilityFailureInvalidConfiguration,
			"request",
			0,
			routed.ExpectedFacilityRevision,
		)
	}
	return app.resetFacility(routed, false)
}

// ResetFacility restores every authored current value through one serialized
// coordinator operation.
func (app *App) ResetFacility(payload FacilityResetPayload) domain.FacilityOperationResult {
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()

	routed := facilityResetRequestFromPrivate(facilityResetRequestToPrivate(controlservice.FacilityResetRequest{
		ExpectedFacilityRevision: payload.ExpectedFacilityRevision,
		CorrelationID:            payload.CorrelationID,
	}))
	if routed.CorrelationID == "" || strings.TrimSpace(routed.CorrelationID) != routed.CorrelationID {
		return facilityOperationFailureForApp(
			routed.CorrelationID,
			domain.FacilityFailureInvalidConfiguration,
			"request",
			0,
			routed.ExpectedFacilityRevision,
		)
	}
	return app.resetFacility(controlservice.FacilityDeviceResetRequest{
		ExpectedFacilityRevision: routed.ExpectedFacilityRevision,
		CorrelationID:            routed.CorrelationID,
	}, true)
}

func (app *App) resetFacility(
	request controlservice.FacilityDeviceResetRequest,
	wholeFacility bool,
) domain.FacilityOperationResult {
	coordination, ok := app.deps.Coordination.(coordinationFacilityResetService)
	if !ok {
		return facilityOperationFailureForApp(
			request.CorrelationID,
			domain.FacilityFailureRuntimeContextEnded,
			"request",
			0,
			request.ExpectedFacilityRevision,
		)
	}

	masterEvents := app.deferMasterEvents()
	defer masterEvents.Commit()

	var state *domain.MasterCoordinationState
	var result *domain.FacilityOperationResult
	if wholeFacility {
		state, result, _ = coordination.ResetFacility(app.contextSnapshot(), controlservice.FacilityResetRequest{
			ExpectedFacilityRevision: request.ExpectedFacilityRevision,
			CorrelationID:            request.CorrelationID,
		})
	} else {
		state, result, _ = coordination.ResetFacilityDevice(app.contextSnapshot(), request)
	}
	if result == nil {
		masterEvents.Discard(state)
		return facilityOperationFailureForApp(
			request.CorrelationID,
			domain.FacilityFailurePersistenceFailed,
			"facility",
			0,
			request.ExpectedFacilityRevision,
		)
	}
	detached, conversionErr := detachedFacilityOperationResultForApp(*result)
	if conversionErr != nil || !validFacilityResetResultForApp(detached, request, wholeFacility) ||
		(detached.OK && detached.Changed && !app.canPublishFacilityMutationState(
			state,
			detached.SessionRevision,
			detached.ResultingFacilityRevision,
		)) {
		masterEvents.Discard(state)
		return facilityOperationFailureForApp(
			request.CorrelationID,
			domain.FacilityFailurePersistenceFailed,
			"facility",
			0,
			request.ExpectedFacilityRevision,
		)
	}
	if detached.OK && detached.Changed {
		canonical := domain.CloneSession(*detached.Session)
		app.acceptSessionStateRevision(detached.SessionRevision)
		app.publishSessionState(SessionStateEvent{Revision: detached.SessionRevision, Session: &canonical})
		masterEvents.Commit()
		app.publishCoordinationStateIfNewer(state)
	} else {
		masterEvents.Discard(state)
	}
	return detached
}

func validFacilityResetResultForApp(
	result domain.FacilityOperationResult,
	request controlservice.FacilityDeviceResetRequest,
	wholeFacility bool,
) bool {
	if result.CorrelationID != request.CorrelationID {
		return false
	}
	if !result.OK {
		return !result.Changed && knownFacilityRecoveryFailureForApp(result.Failure) && result.Session == nil &&
			result.PreviousFacilityRevision == result.ResultingFacilityRevision &&
			len(result.AffectedDeviceIDs) == 0 && len(result.AffectedConditionIDs) == 0
	}
	if result.Failure != "" && result.Failure != domain.FacilityFailureUnspecified {
		return false
	}
	if result.PreviousFacilityRevision != request.ExpectedFacilityRevision {
		return false
	}
	if !result.Changed {
		return result.ResultingFacilityRevision == result.PreviousFacilityRevision &&
			len(result.AffectedDeviceIDs) == 0 && len(result.AffectedConditionIDs) == 0
	}
	if result.SessionRevision == 0 || result.Session == nil || result.Session.Facility == nil ||
		result.ResultingFacilityRevision != result.PreviousFacilityRevision+1 ||
		result.Session.Facility.Revision != result.ResultingFacilityRevision ||
		(len(result.AffectedDeviceIDs) == 0 && len(result.AffectedConditionIDs) == 0) ||
		hasDuplicateStrings(result.AffectedDeviceIDs) || hasDuplicateStrings(result.AffectedConditionIDs) {
		return false
	}
	return wholeFacility || slices.Equal(result.AffectedDeviceIDs, []string{request.DeviceID})
}

func hasDuplicateStrings(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			return true
		}
		if _, duplicate := seen[value]; duplicate {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func (app *App) canPublishFacilityMutationState(
	state *domain.MasterCoordinationState,
	sessionRevision uint64,
	facilityRevision uint64,
) bool {
	if state == nil || state.FacilityRevision == nil || *state.FacilityRevision != facilityRevision {
		return false
	}
	app.mu.RLock()
	defer app.mu.RUnlock()
	if app.coordinationState != nil && state.Revision <= app.coordinationState.Revision {
		return false
	}
	return sessionRevision > app.publishedSessionRevision && sessionRevision > app.savedRevision
}

// InspectFacilityDependencies reads one exact canonical session snapshot and
// never publishes or mutates facility, coordination, or player state.
func (app *App) InspectFacilityDependencies(payload FacilityDependencyInspectionPayload) FacilityDependencyInspectionResult {
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()

	request := facilityDependencyInspectionRequest(payload)
	routed := facilityDependencyInspectionRequestFromPrivate(facilityDependencyInspectionRequestToPrivate(request))
	if routed.Target == nil {
		return routeFacilityDependencyInspectionResultForApp(facilityDependencyInspectionResult{
			Failure: domain.FacilityFailureInvalidConfiguration,
			Issues:  []domain.FacilityIssue{{Code: domain.FacilityFailureInvalidConfiguration, EntityKind: "request"}},
		})
	}
	sessions, ok := app.deps.Sessions.(sessionSnapshotService)
	if !ok {
		return routeFacilityDependencyInspectionResultForApp(facilityDependencyInspectionResult{
			Failure: domain.FacilityFailureRuntimeContextEnded,
			Issues:  []domain.FacilityIssue{{Code: domain.FacilityFailureRuntimeContextEnded, EntityKind: "session"}},
		})
	}
	active := sessions.Snapshot()
	if active.Session == nil {
		return routeFacilityDependencyInspectionResultForApp(facilityDependencyInspectionResult{
			Failure:         domain.FacilityFailureRuntimeContextEnded,
			Issues:          []domain.FacilityIssue{{Code: domain.FacilityFailureRuntimeContextEnded, EntityKind: "session"}},
			SessionRevision: active.SavedRevision,
		})
	}
	facilityRevision := uint64(0)
	if active.Session.Facility != nil {
		facilityRevision = active.Session.Facility.Revision
	}
	if active.RequestedRevision != active.SavedRevision {
		return routeFacilityDependencyInspectionResultForApp(facilityDependencyInspectionResult{
			Failure:          domain.FacilityFailureStaleRevision,
			Issues:           []domain.FacilityIssue{{Code: domain.FacilityFailureStaleRevision, EntityKind: "session"}},
			SessionRevision:  active.SavedRevision,
			FacilityRevision: facilityRevision,
		})
	}
	if routed.ExpectedSessionRevision != active.SavedRevision || routed.ExpectedFacilityRevision != facilityRevision {
		return routeFacilityDependencyInspectionResultForApp(facilityDependencyInspectionResult{
			Failure:          domain.FacilityFailureStaleRevision,
			Issues:           []domain.FacilityIssue{{Code: domain.FacilityFailureStaleRevision, EntityKind: "facility"}},
			SessionRevision:  active.SavedRevision,
			FacilityRevision: facilityRevision,
		})
	}
	coordination, available := app.deps.Coordination.(coordinationFacilityInspectionService)
	if !available {
		return routeFacilityDependencyInspectionResultForApp(facilityDependencyInspectionResult{
			Failure: domain.FacilityFailureRuntimeContextEnded,
			Issues: []domain.FacilityIssue{{
				Code: domain.FacilityFailureRuntimeContextEnded, EntityKind: "inspection",
			}},
			SessionRevision:  active.SavedRevision,
			FacilityRevision: facilityRevision,
		})
	}
	inspected := coordination.InspectFacilityDependencies(app.contextSnapshot(), controlservice.FacilityDependencyInspectionRequest{
		Session:                  domain.CloneSession(*active.Session),
		Target:                   *routed.Target,
		ExpectedFacilityRevision: routed.ExpectedFacilityRevision,
	})
	if !validFacilityDependencyInspectionResultForApp(inspected, routed.ExpectedFacilityRevision) {
		return routeFacilityDependencyInspectionResultForApp(facilityDependencyInspectionResult{
			Failure: domain.FacilityFailurePersistenceFailed,
			Issues: []domain.FacilityIssue{{
				Code: domain.FacilityFailurePersistenceFailed, EntityKind: "inspection",
			}},
			SessionRevision:  active.SavedRevision,
			FacilityRevision: facilityRevision,
		})
	}
	return routeFacilityDependencyInspectionResultForApp(facilityDependencyInspectionResult{
		OK:               inspected.OK,
		Failure:          inspected.Failure,
		Issues:           inspected.Issues,
		SessionRevision:  active.SavedRevision,
		FacilityRevision: inspected.FacilityRevision,
		Report:           inspected.Report,
	})
}

func validFacilityDependencyInspectionResultForApp(
	result controlservice.FacilityDependencyInspectionResult,
	expectedFacilityRevision uint64,
) bool {
	if result.OK {
		return result.Failure == "" && result.FacilityRevision == expectedFacilityRevision && result.Report != nil
	}
	return knownFacilityRecoveryFailureForApp(result.Failure) && result.Report == nil
}

func detachedFacilityOperationResultForApp(result domain.FacilityOperationResult) (domain.FacilityOperationResult, error) {
	semantic, err := facilityOperationResultToPrivate(result)
	if err != nil {
		return domain.FacilityOperationResult{}, err
	}
	detached, err := facilityOperationResultFromPrivate(semantic, result)
	if err != nil {
		return domain.FacilityOperationResult{}, err
	}
	if detached == nil {
		return domain.FacilityOperationResult{}, fmt.Errorf("facility operation result is required")
	}
	return *detached, nil
}

func facilityOperationFailureForApp(
	correlationID string,
	failure domain.FacilityFailureCode,
	entityKind string,
	sessionRevision uint64,
	facilityRevision uint64,
) domain.FacilityOperationResult {
	result, err := detachedFacilityOperationResultForApp(domain.FacilityOperationResult{
		CorrelationID: correlationID,
		Failure:       failure,
		Issues: []domain.FacilityIssue{{
			Code: failure, EntityKind: entityKind,
		}},
		SessionRevision:           sessionRevision,
		PreviousFacilityRevision:  facilityRevision,
		ResultingFacilityRevision: facilityRevision,
	})
	if err != nil {
		return domain.FacilityOperationResult{
			CorrelationID: correlationID,
			Failure:       failure,
			Issues:        []domain.FacilityIssue{{Code: failure, EntityKind: entityKind}},
		}
	}
	return result
}

func routeFacilityDependencyInspectionResultForApp(result facilityDependencyInspectionResult) FacilityDependencyInspectionResult {
	routed := facilityDependencyInspectionResultFromPrivate(facilityDependencyInspectionResultToPrivate(result))
	return FacilityDependencyInspectionResult(routed)
}

func sessionPointerForApp(session domain.Session) *domain.Session {
	clone := domain.CloneSession(session)
	return &clone
}

// LoadReferencedPlayerConfig reloads the active session's durable roster.
func (app *App) LoadReferencedPlayerConfig() (result PlayerConfigCommandResult) {
	defer func() {
		app.recordOperation("player-config.load-referenced", operationOutcome(result.OK, result.Canceled), nil)
	}()
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()
	sessions, ok := app.deps.Sessions.(sessionPlayerConfigCommands)
	if !ok || app.deps.PlayerConfigs == nil {
		return app.playerConfigFailure("player config service is unavailable", false)
	}
	if failure := app.playerConfigChangePrecondition(); failure != nil {
		return *failure
	}
	active := sessions.Snapshot()
	if strings.TrimSpace(active.Session.PlayerConfig) == "" {
		return app.playerConfigFailure("no player config is associated with this session", false)
	}
	loaded := app.deps.PlayerConfigs.LoadReferenced(active.Path, active.Session.PlayerConfig)
	return app.installPlayerConfig(loaded, false)
}

// NewPlayerConfig creates, associates, and installs one empty durable roster.
func (app *App) NewPlayerConfig() (result PlayerConfigCommandResult) {
	defer func() {
		app.recordOperation("player-config.create", operationOutcome(result.OK, result.Canceled), nil)
	}()
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()
	if app.deps.PlayerConfigs == nil {
		return app.playerConfigFailure("player config service is unavailable", false)
	}
	if failure := app.playerConfigChangePrecondition(); failure != nil {
		return *failure
	}
	return app.installPlayerConfig(app.deps.PlayerConfigs.Create(app.contextSnapshot()), true)
}

// OpenPlayerConfig selects, associates, and installs an existing durable roster.
func (app *App) OpenPlayerConfig() (result PlayerConfigCommandResult) {
	defer func() {
		app.recordOperation("player-config.open", operationOutcome(result.OK, result.Canceled), nil)
	}()
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()
	if app.deps.PlayerConfigs == nil {
		return app.playerConfigFailure("player config service is unavailable", false)
	}
	if failure := app.playerConfigChangePrecondition(); failure != nil {
		return *failure
	}
	return app.installPlayerConfig(app.deps.PlayerConfigs.Open(app.contextSnapshot()), true)
}

func (app *App) playerConfigChangePrecondition() *PlayerConfigCommandResult {
	sessions, ok := app.deps.Sessions.(sessionPlayerConfigCommands)
	if !ok {
		result := app.playerConfigFailure("session service cannot associate player configs", false)
		return &result
	}
	active := sessions.Snapshot()
	if active.Path == "" || active.Session == nil {
		result := app.playerConfigFailure("there is no active session", false)
		return &result
	}
	app.mu.RLock()
	broadcastActive := app.coordinationState != nil && app.coordinationState.Broadcast != nil
	app.mu.RUnlock()
	if broadcastActive {
		result := app.playerConfigFailure("player config cannot change during a broadcast", false)
		return &result
	}
	return nil
}

func (app *App) installPlayerConfig(result playerconfigservice.Result, associate bool) PlayerConfigCommandResult {
	if result.Canceled {
		return app.playerConfigFailure("", true)
	}
	if !result.OK || result.Config == nil || result.FilePath == "" {
		message := result.Error
		if message == "" {
			message = "player config could not be loaded"
		}
		return app.playerConfigFailure(message, false)
	}
	coordination, ok := app.deps.Coordination.(coordinationPlayerConfigService)
	if !ok {
		return app.playerConfigFailure("coordination service cannot install player configs", false)
	}
	var associatedSession *domain.Session
	if associate {
		sessions, ok := app.deps.Sessions.(sessionPlayerConfigCommands)
		if !ok {
			return app.playerConfigFailure("session service cannot associate player configs", false)
		}
		associated := sessions.AssociatePlayerConfig(app.contextSnapshot(), result.FilePath)
		if !associated.OK {
			message := associated.Error
			if message == "" {
				message = "player config association could not be saved"
			}
			return app.playerConfigFailure(message, associated.Canceled)
		}
		associatedSession = associated.Session
	} else if sessions, ok := app.deps.Sessions.(sessionPlayerConfigCommands); ok {
		associatedSession = sessions.Snapshot().Session
	}
	handle := domain.PlayerConfigHandle{
		Path:          result.FilePath,
		Version:       result.Config.Version,
		Name:          result.Config.Name,
		ContentDigest: result.ContentDigest,
	}
	state, err := coordination.InstallPlayerConfig(handle, result.Config.Roster)
	if err != nil {
		return app.playerConfigFailure(err.Error(), false)
	}
	app.publishCoordinationState(state)
	return routePlayerConfigResult(PlayerConfigCommandResult{
		OK:      true,
		Config:  &domain.PlayerConfigMetadata{Status: "loaded", FilePath: handle.Path, Version: handle.Version, Name: handle.Name},
		Session: associatedSession,
		State:   domain.CloneMasterCoordinationState(state),
	})
}

func (app *App) playerConfigFailure(message string, canceled bool) PlayerConfigCommandResult {
	app.mu.RLock()
	state := domain.CloneMasterCoordinationState(app.coordinationState)
	app.mu.RUnlock()
	return routePlayerConfigResult(PlayerConfigCommandResult{Canceled: canceled, Error: message, State: state})
}

func (app *App) resetPlayerConfigForSession(result sessionservice.SessionResult) {
	if !result.OK {
		return
	}
	coordination, ok := app.deps.Coordination.(coordinationPlayerConfigService)
	if !ok {
		return
	}
	state, err := coordination.ClearPlayerConfig()
	if err == nil && state != nil {
		app.publishCoordinationState(state)
	}
}

// AddCharacter validates the complete trusted player profile before entering
// the coordinator and publishes only its detached authoritative projection.
func (app *App) AddCharacter(payload CharacterCreatePayload) CoordinationCommandResult {
	if err := domain.ValidateCharacterIntelligence(payload.Intelligence); err != nil {
		return app.coordinationFailure(err.Error())
	}
	payload = routeAddCharacterRequest(payload)
	name, err := domain.ValidateCharacterName(payload.Name)
	if err != nil {
		return app.coordinationFailure(err.Error())
	}
	if payload.HackerPerkAvailable == nil {
		return app.coordinationFailure("character Hacker perk availability is required")
	}
	if app.deps.Coordination == nil {
		return app.coordinationFailure("coordination service is unavailable")
	}
	state, err := app.deps.Coordination.AddCharacter(domain.CharacterCreatePayload{
		Name:                name,
		Intelligence:        payload.Intelligence,
		HackerPerkAvailable: *payload.HackerPerkAvailable,
		ExpectedRevision:    payload.ExpectedRevision,
	})
	return app.completeCoordinationCommand(state, err, "character could not be added")
}

// UpdateCharacter validates a complete trusted player profile before entering
// the coordinator transaction.
func (app *App) UpdateCharacter(payload CharacterUpdatePayload) CoordinationCommandResult {
	if err := domain.ValidateCharacterIntelligence(payload.Intelligence); err != nil {
		return app.coordinationFailure(err.Error())
	}
	payload = routeUpdateCharacterRequest(payload)
	if strings.TrimSpace(string(payload.CharacterID)) == "" {
		return app.coordinationFailure("character ID must not be blank")
	}
	name, err := domain.ValidateCharacterName(payload.Name)
	if err != nil {
		return app.coordinationFailure(err.Error())
	}
	if payload.HackerPerkAvailable == nil {
		return app.coordinationFailure("character Hacker perk availability is required")
	}
	coordination, ok := app.deps.Coordination.(coordinationCorrectionService)
	if !ok {
		return app.coordinationFailure("coordination service is unavailable")
	}
	state, commandErr := coordination.UpdateCharacter(domain.CharacterUpdatePayload{
		CharacterID:         payload.CharacterID,
		Name:                name,
		Intelligence:        payload.Intelligence,
		HackerPerkAvailable: *payload.HackerPerkAvailable,
		ExpectedRevision:    payload.ExpectedRevision,
	})
	return app.completeCoordinationCommand(state, commandErr, "character could not be updated")
}

// DeleteCharacter removes only an existing unclaimed roster identity.
func (app *App) DeleteCharacter(payload CharacterDeletePayload) CoordinationCommandResult {
	payload = routeDeleteCharacterRequest(payload)
	if strings.TrimSpace(string(payload.CharacterID)) == "" {
		return app.coordinationFailure("character ID must not be blank")
	}
	coordination, ok := app.deps.Coordination.(coordinationCorrectionService)
	if !ok {
		return app.coordinationFailure("coordination service is unavailable")
	}
	state, err := coordination.DeleteCharacter(domain.CharacterDeletePayload{
		CharacterID:      payload.CharacterID,
		ExpectedRevision: payload.ExpectedRevision,
	})
	return app.completeCoordinationCommand(state, err, "character could not be deleted")
}

// RenameLogicalSession validates and trims the private fallback label without
// exposing it to durable session persistence.
func (app *App) RenameLogicalSession(payload LogicalSessionRenamePayload) CoordinationCommandResult {
	payload = routeRenameLogicalSessionRequest(payload)
	if strings.TrimSpace(string(payload.SessionID)) == "" {
		return app.coordinationFailure("session ID must not be blank")
	}
	fallbackName, err := validatedCoordinationDisplayName(payload.FallbackName, "session fallback name")
	if err != nil {
		return app.coordinationFailure(err.Error())
	}
	coordination, ok := app.deps.Coordination.(coordinationCorrectionService)
	if !ok {
		return app.coordinationFailure("coordination service is unavailable")
	}
	state, commandErr := coordination.RenameLogicalSession(payload.SessionID, fallbackName)
	return app.completeCoordinationCommand(state, commandErr, "logical session could not be renamed")
}

// AssignCharacter installs one authoritative current-broadcast claim.
func (app *App) AssignCharacter(payload AssignmentPayload) CoordinationCommandResult {
	payload = routeAssignCharacterRequest(payload)
	if strings.TrimSpace(string(payload.SessionID)) == "" {
		return app.coordinationFailure("session ID must not be blank")
	}
	if strings.TrimSpace(string(payload.CharacterID)) == "" {
		return app.coordinationFailure("character ID must not be blank")
	}
	coordination, ok := app.deps.Coordination.(coordinationCorrectionService)
	if !ok {
		return app.coordinationFailure("coordination service is unavailable")
	}
	state, err := coordination.AssignCharacter(payload.SessionID, payload.CharacterID)
	return app.completeCoordinationCommand(state, err, "character could not be assigned")
}

// ReleaseCharacter removes one claim and leaves controller selection to an
// explicit trusted command.
func (app *App) ReleaseCharacter(sessionID string) CoordinationCommandResult {
	sessionID = routeReleaseCharacterRequest(sessionID)
	if strings.TrimSpace(sessionID) == "" {
		return app.coordinationFailure("session ID must not be blank")
	}
	coordination, ok := app.deps.Coordination.(coordinationCorrectionService)
	if !ok {
		return app.coordinationFailure("coordination service is unavailable")
	}
	state, err := coordination.ReleaseCharacter(domain.LogicalSessionID(sessionID))
	return app.completeCoordinationCommand(state, err, "character could not be released")
}

// MoveCharacter transfers a stable roster identity to one unassigned logical
// session in the coordinator's single authoritative order.
func (app *App) MoveCharacter(payload MoveCharacterPayload) CoordinationCommandResult {
	payload = routeMoveCharacterRequest(payload)
	if strings.TrimSpace(string(payload.CharacterID)) == "" {
		return app.coordinationFailure("character ID must not be blank")
	}
	if strings.TrimSpace(string(payload.ToSessionID)) == "" {
		return app.coordinationFailure("destination session ID must not be blank")
	}
	coordination, ok := app.deps.Coordination.(coordinationCorrectionService)
	if !ok {
		return app.coordinationFailure("coordination service is unavailable")
	}
	state, err := coordination.MoveCharacter(payload.CharacterID, payload.ToSessionID)
	return app.completeCoordinationCommand(state, err, "character could not be moved")
}

// SetActiveController atomically designates one connected assigned logical
// session as the sole controller for the current broadcast.
func (app *App) SetActiveController(sessionID string) CoordinationCommandResult {
	sessionID = routeSetActiveControllerRequest(sessionID)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return app.coordinationFailure("session ID must not be blank")
	}
	coordination, ok := app.deps.Coordination.(coordinationControllerService)
	if !ok {
		return app.coordinationFailure("coordination service is unavailable")
	}
	state, err := coordination.SetActiveController(domain.LogicalSessionID(sessionID))
	return app.completeCoordinationCommand(state, err, "active controller could not be changed")
}

// StartBroadcast creates a fresh broadcast epoch through the coordinator.
func (app *App) StartBroadcast() (result CoordinationCommandResult) {
	defer func() {
		app.recordOperation("broadcast.start", operationOutcome(result.OK, false), nil)
	}()
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()
	if app.deps.Coordination == nil {
		return app.coordinationFailure("coordination service is unavailable")
	}
	state, err := app.deps.Coordination.StartBroadcast()
	if err != nil {
		return app.coordinationFailure(err.Error())
	}
	if state == nil {
		return app.coordinationFailure("broadcast could not be started")
	}
	app.publishCoordinationState(state)
	return routeCoordinationResult(CoordinationCommandResult{OK: true, State: domain.CloneMasterCoordinationState(state)})
}

// EndBroadcast ends only the process-local broadcast epoch. Authored durable
// terminals and the active session document remain outside this boundary.
func (app *App) EndBroadcast() (result CoordinationCommandResult) {
	defer func() {
		app.recordOperation("broadcast.end", operationOutcome(result.OK, false), nil)
	}()
	coordination, ok := app.deps.Coordination.(coordinationBroadcastLifecycleService)
	if !ok {
		return app.coordinationFailure("coordination service is unavailable")
	}
	state, err := coordination.EndBroadcast()
	return app.completeCoordinationCommand(state, err, "broadcast could not be ended")
}

// RequestTerminalActivation validates and normalizes authored content before
// asking the coordinator to atomically select the broadcast-wide terminal.
func (app *App) RequestTerminalActivation(payload LiveTerminalPayload) TerminalSwitchCommandResult {
	payload.TerminalID = strings.TrimSpace(payload.TerminalID)
	payload.TerminalName = strings.TrimSpace(payload.TerminalName)
	if err := app.validateLiveTerminal(payload.TerminalID, payload.TerminalName, payload.Tree, payload.HackLevel, payload.IntroText); err != nil {
		return app.terminalSwitchFailure(err.Error(), nil)
	}
	payload, err := routeTerminalActivationRequest(payload, false)
	if err != nil {
		return app.terminalSwitchFailure("terminal request could not be represented by the private contract", nil)
	}
	coordination, ok := app.deps.Coordination.(coordinationTerminalService)
	if !ok {
		return app.terminalSwitchFailure("coordination service is unavailable", nil)
	}
	target := domain.TerminalTarget{
		TerminalID: payload.TerminalID, TerminalName: payload.TerminalName,
		Tree: payload.Tree, HackLevel: payload.HackLevel, IntroText: payload.IntroText,
		CommandStates: app.canonicalCommandStates(payload.TerminalID),
	}
	state, err := coordination.RequestTerminalActivation(target)
	return app.completeTerminalSwitchRequest(state, err, "activated", "terminal could not be activated")
}

// RequestTerminalClear clears only the active terminal selection. The
// broadcast, assignments, identities, and controller remain authoritative.
func (app *App) RequestTerminalClear() TerminalSwitchCommandResult {
	coordination, ok := app.deps.Coordination.(coordinationTerminalService)
	if !ok {
		return app.terminalSwitchFailure("coordination service is unavailable", nil)
	}
	state, err := coordination.RequestTerminalClear()
	return app.completeTerminalSwitchRequest(state, err, "cleared", "active terminal could not be cleared")
}

// ResolveTerminalSwitch validates the opaque decision payload and lets the
// coordinator re-evaluate the latest private source state before committing.
func (app *App) ResolveTerminalSwitch(payload TerminalSwitchDecisionPayload) TerminalSwitchCommandResult {
	if payload.SwitchID == "" {
		return app.terminalSwitchFailure("switch ID must not be blank", nil)
	}
	if payload.Decision != domain.TerminalSwitchPreserve && payload.Decision != domain.TerminalSwitchDiscard && payload.Decision != domain.TerminalSwitchCancel {
		return app.terminalSwitchFailure("terminal switch decision must be preserve, discard, or cancel", nil)
	}
	payload, err := routeTerminalSwitchDecisionRequest(payload)
	if err != nil {
		return app.terminalSwitchFailure("terminal switch decision could not be represented by the private contract", nil)
	}
	coordination, ok := app.deps.Coordination.(coordinationTerminalDecisionService)
	if !ok {
		return app.terminalSwitchFailure("coordination service is unavailable", nil)
	}
	app.mu.RLock()
	pending := domain.CloneMasterCoordinationState(app.coordinationState)
	app.mu.RUnlock()
	status := "activated"
	if payload.Decision == domain.TerminalSwitchCancel {
		status = "cancelled"
	} else if pending != nil && pending.PendingSwitch != nil && pending.PendingSwitch.SwitchID == payload.SwitchID && pending.PendingSwitch.TargetTerminalID == nil {
		status = "cleared"
	}
	state, err := coordination.ResolveTerminalSwitch(payload.SwitchID, payload.Decision)
	return app.completeTerminalSwitchCommand(state, err, status, "terminal switch could not be resolved")
}

// ResolveCommandExecution approves or rejects the exact current private
// request. A durable session event is emitted only after approve has reached
// the session store and returned a changed canonical document.
func (app *App) ResolveCommandExecution(payload CommandExecutionDecisionPayload) ResolveCommandExecutionResult {
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()

	payload.RequestID = strings.TrimSpace(payload.RequestID)
	if payload.RequestID == "" {
		return app.commandExecutionFailure("command execution request ID must not be blank", nil)
	}
	if payload.Decision != domain.CommandExecutionApprove && payload.Decision != domain.CommandExecutionReject {
		return app.commandExecutionFailure("command execution decision must be approve or reject", nil)
	}
	routed, err := routeCommandExecutionDecisionRequest(payload)
	if err != nil {
		return app.commandExecutionFailure("command execution decision could not be represented by the private contract", nil)
	}
	coordination, ok := app.deps.Coordination.(coordinationCommandExecutionService)
	if !ok {
		return app.commandExecutionFailure("coordination service is unavailable", nil)
	}

	state, mutation, facilityResult, err := coordination.ResolveCommandExecution(app.contextSnapshot(), routed.RequestID, routed.Decision)
	if state != nil {
		app.publishCoordinationStateIfNewer(state)
	}
	if facilityResult != nil {
		if facilityResult.OK && facilityResult.Changed && facilityResult.Session != nil {
			canonical := domain.CloneSession(*facilityResult.Session)
			app.acceptSessionStateRevision(facilityResult.SessionRevision)
			app.publishSessionState(SessionStateEvent{Revision: facilityResult.SessionRevision, Session: &canonical})
		}
		return routeResolveCommandExecutionResult(ResolveCommandExecutionResult{
			OK:             true,
			State:          domain.CloneMasterCoordinationState(state),
			FacilityResult: facilityResult,
		})
	}
	if err != nil {
		return app.commandExecutionFailure(commandExecutionMasterError(err), state)
	}
	if state == nil {
		return app.commandExecutionFailure("command execution could not be resolved", nil)
	}

	if mutation != nil {
		app.acceptSessionStateRevision(mutation.Revision)
		if mutation.Changed {
			session := mutation.Session
			app.publishSessionState(SessionStateEvent{Revision: mutation.Revision, Session: &session})
		}
	}
	return routeResolveCommandExecutionResult(ResolveCommandExecutionResult{
		OK: true, State: domain.CloneMasterCoordinationState(state),
	})
}

// ResolveTerminalNavigation validates and routes an exact private enum/request
// pair, then accepts only a newer coordination revision from the coordinator.
func (app *App) ResolveTerminalNavigation(payload TerminalNavigationDecisionPayload) ResolveTerminalNavigationResult {
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()

	payload.RequestID = strings.TrimSpace(payload.RequestID)
	if payload.RequestID == "" {
		return app.terminalNavigationFailure("terminal navigation request ID must not be blank", nil)
	}
	if payload.Decision != domain.TerminalNavigationApprove && payload.Decision != domain.TerminalNavigationReject {
		return app.terminalNavigationFailure("terminal navigation decision must be approve or reject", nil)
	}
	routed, err := routeTerminalNavigationDecisionRequest(payload)
	if err != nil {
		return app.terminalNavigationFailure("terminal navigation decision could not be represented by the private contract", nil)
	}
	coordination, ok := app.deps.Coordination.(coordinationTerminalNavigationService)
	if !ok {
		return app.terminalNavigationFailure("coordination service is unavailable", nil)
	}
	state, err := coordination.ResolveTerminalNavigation(routed.RequestID, routed.Decision)
	if state != nil {
		app.publishCoordinationStateIfNewer(state)
	}
	if err != nil {
		return app.terminalNavigationFailure("terminal navigation request is no longer valid", state)
	}
	if state == nil {
		return app.terminalNavigationFailure("terminal navigation could not be resolved", nil)
	}
	return routeResolveTerminalNavigationResult(ResolveTerminalNavigationResult{OK: true, State: domain.CloneMasterCoordinationState(state)})
}

func (app *App) terminalNavigationFailure(message string, state *domain.MasterCoordinationState) ResolveTerminalNavigationResult {
	if state == nil {
		app.mu.RLock()
		state = domain.CloneMasterCoordinationState(app.coordinationState)
		app.mu.RUnlock()
	}
	return routeResolveTerminalNavigationResult(ResolveTerminalNavigationResult{Error: message, State: domain.CloneMasterCoordinationState(state)})
}

func (app *App) publishCoordinationStateIfNewer(state *domain.MasterCoordinationState) {
	if state == nil {
		return
	}
	clone := domain.CloneMasterCoordinationState(state)
	app.mu.Lock()
	if app.coordinationState != nil && clone.Revision <= app.coordinationState.Revision {
		app.mu.Unlock()
		return
	}
	app.coordinationState = clone
	app.mu.Unlock()
	if app.deps.Events != nil {
		app.emitEvent(coordinationStateEvent, routeCoordinationEvent(domain.CloneMasterCoordinationState(clone)))
	}
}

func (app *App) commandExecutionFailure(message string, state *domain.MasterCoordinationState) ResolveCommandExecutionResult {
	if state == nil {
		app.mu.RLock()
		state = domain.CloneMasterCoordinationState(app.coordinationState)
		app.mu.RUnlock()
	}
	return routeResolveCommandExecutionResult(ResolveCommandExecutionResult{
		Error: message, State: domain.CloneMasterCoordinationState(state),
	})
}

func commandExecutionMasterError(err error) string {
	switch {
	case errors.Is(err, controlservice.ErrCommandExecutionStale):
		return "command execution request is no longer pending"
	case errors.Is(err, controlservice.ErrCommandExecutionPersistence):
		return "command execution could not be persisted"
	default:
		return "command execution could not be resolved"
	}
}

// UpdateLiveTerminal validates replacement content and preserves the current
// puzzle through the ordered coordinator boundary. The legacy fallback keeps
// older focused hosts compatible while production always provides coordination.
func (app *App) UpdateLiveTerminal(payload LiveUpdatePayload) CoordinationCommandResult {
	if err := app.validateLiveTerminalUpdate(payload.Tree, payload.IntroText); err != nil {
		return app.coordinationFailure(err.Error())
	}
	payload, err := routeLiveTerminalUpdateRequest(payload)
	if err != nil {
		return app.coordinationFailure("live terminal update could not be represented by the private contract")
	}
	if coordination, ok := app.deps.Coordination.(coordinationTerminalService); ok {
		state, err := coordination.UpdateLiveTerminal(payload.Tree, payload.IntroText)
		return app.completeCoordinationCommand(state, err, "no terminal is active")
	}
	if app.deps.Live == nil {
		return app.coordinationFailure("live service is unavailable")
	}
	state, ok := app.deps.Live.Update(payload.Tree, payload.IntroText)
	if !ok || state == nil {
		return app.coordinationFailure("no terminal is live")
	}
	if publisher, ok := app.deps.Player.(playerPublisher); ok {
		publisher.PublishUpdate()
	}
	app.updateHackState(state.Hack)
	app.mu.RLock()
	coordinationState := domain.CloneMasterCoordinationState(app.coordinationState)
	app.mu.RUnlock()
	return routeCoordinationResult(CoordinationCommandResult{OK: true, State: coordinationState})
}

// ResetFailedHack validates the latest authored terminal payload at the
// trusted desktop edge, then asks the coordinator to replace only the failed
// active puzzle. Player transports never receive this command surface.
func (app *App) ResetFailedHack(payload LiveTerminalPayload) CoordinationCommandResult {
	payload.TerminalID = strings.TrimSpace(payload.TerminalID)
	payload.TerminalName = strings.TrimSpace(payload.TerminalName)
	if err := app.validateLiveTerminal(payload.TerminalID, payload.TerminalName, payload.Tree, payload.HackLevel, payload.IntroText); err != nil {
		return app.coordinationFailure(err.Error())
	}
	payload, err := routeTerminalActivationRequest(payload, true)
	if err != nil {
		return app.coordinationFailure("failed hacking reset could not be represented by the private contract")
	}
	coordination, ok := app.deps.Coordination.(coordinationTerminalService)
	if !ok {
		return app.coordinationFailure("coordination service is unavailable")
	}
	target := domain.TerminalTarget{
		TerminalID: payload.TerminalID, TerminalName: payload.TerminalName,
		Tree: payload.Tree, HackLevel: payload.HackLevel, IntroText: payload.IntroText,
		CommandStates: app.canonicalCommandStates(payload.TerminalID),
	}
	state, err := coordination.ResetFailedHack(target)
	return app.completeCoordinationCommand(state, err, "failed hacking puzzle could not be reset")
}

// ResetCommandState removes one durable execution snapshot. Successful
// no-ops return the canonical document without publishing a new revision.
func (app *App) ResetCommandState(payload ResetCommandStatePayload) SessionStateResult {
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()

	payload.TerminalID = strings.TrimSpace(payload.TerminalID)
	payload.CommandID = strings.TrimSpace(payload.CommandID)
	if payload.TerminalID == "" {
		return routeSessionStateResult(SessionStateResult{Error: "terminal ID must not be blank"})
	}
	if payload.CommandID == "" {
		return routeSessionStateResult(SessionStateResult{Error: "command ID must not be blank"})
	}
	payload = routeResetCommandStateRequest(payload)
	if coordination, ok := app.deps.Coordination.(coordinationCommandStateResetService); ok {
		state, mutation, err := coordination.ResetCommandState(
			app.contextSnapshot(), payload.TerminalID, payload.CommandID,
		)
		if !errors.Is(err, controlservice.ErrCommandStateStorageUnavailable) {
			return app.completeCoordinatedCommandStateReset(state, mutation, err)
		}
	}
	commands, ok := app.deps.Sessions.(sessionCommandStateCommands)
	if !ok {
		return routeSessionStateResult(SessionStateResult{Error: "coordination service is unavailable"})
	}
	result := commands.ResetCommandState(app.contextSnapshot(), payload.TerminalID, payload.CommandID)
	return app.completeCommandStateReset(payload.TerminalID, result)
}

// ResetTerminalCommandStates removes all durable execution snapshots for one
// stable terminal ID in one session-service mutation.
func (app *App) ResetTerminalCommandStates(payload ResetTerminalCommandStatesPayload) SessionStateResult {
	app.coordinationCommandMu.Lock()
	defer app.coordinationCommandMu.Unlock()

	payload.TerminalID = strings.TrimSpace(payload.TerminalID)
	if payload.TerminalID == "" {
		return routeSessionStateResult(SessionStateResult{Error: "terminal ID must not be blank"})
	}
	payload = routeResetTerminalCommandStatesRequest(payload)
	if coordination, ok := app.deps.Coordination.(coordinationCommandStateResetService); ok {
		state, mutation, err := coordination.ResetTerminalCommandStates(app.contextSnapshot(), payload.TerminalID)
		if !errors.Is(err, controlservice.ErrCommandStateStorageUnavailable) {
			return app.completeCoordinatedCommandStateReset(state, mutation, err)
		}
	}
	commands, ok := app.deps.Sessions.(sessionCommandStateCommands)
	if !ok {
		return routeSessionStateResult(SessionStateResult{Error: "coordination service is unavailable"})
	}
	result := commands.ResetTerminalCommandStates(app.contextSnapshot(), payload.TerminalID)
	return app.completeCommandStateReset(payload.TerminalID, result)
}

func (app *App) completeCoordinatedCommandStateReset(
	state *domain.MasterCoordinationState,
	mutation *controlservice.CommandStateMutation,
	err error,
) SessionStateResult {
	if state != nil {
		app.publishCoordinationStateIfNewer(state)
	}
	if err != nil {
		return routeSessionStateResult(SessionStateResult{Error: err.Error()})
	}
	if mutation == nil {
		return routeSessionStateResult(SessionStateResult{Error: "command state reset returned no session"})
	}

	canonical := domain.CloneSession(mutation.Session)
	app.acceptSessionStateRevision(mutation.Revision)
	if mutation.Changed {
		app.publishSessionState(SessionStateEvent{Revision: mutation.Revision, Session: &canonical})
	}
	return routeSessionStateResult(SessionStateResult{
		OK: true, Revision: mutation.Revision, Session: sessionPointerForApp(canonical),
	})
}

func (app *App) completeCommandStateReset(terminalID string, result sessionservice.CommandStateResult) SessionStateResult {
	if !result.OK {
		message := result.Error
		if message == "" {
			message = "command state could not be reset"
		}
		return routeSessionStateResult(SessionStateResult{Error: message, Revision: result.Revision})
	}
	if result.Session == nil {
		return routeSessionStateResult(SessionStateResult{Error: "command state reset returned no session", Revision: result.Revision})
	}

	native := SessionStateResult{OK: true, Revision: result.Revision, Session: result.Session}
	app.acceptSessionStateRevision(result.Revision)
	if !result.Changed {
		return routeSessionStateResult(native)
	}

	refreshError := ""
	terminal := terminalForSessionState(result.Session, terminalID)
	if terminal == nil {
		refreshError = "reset terminal is missing from the canonical session"
	} else if app.isActiveTerminal(terminalID) {
		if coordination, ok := app.deps.Coordination.(coordinationTerminalCanonicalRefreshService); ok {
			state, err := coordination.RefreshActiveTerminal(domain.TerminalTarget{
				TerminalID: terminal.ID, TerminalName: terminal.Name, Tree: terminal.Root,
				CommandStates: cloneCommandExecutionStates(terminal.CommandStates),
				HackLevel:     terminal.HackLevel, IntroText: terminal.IntroText,
			})
			if err != nil || state == nil {
				refreshError = "active terminal could not be refreshed"
			} else {
				app.publishCoordinationState(state)
			}
		} else if coordination, ok := app.deps.Coordination.(coordinationTerminalService); ok {
			// Focused legacy fakes implement only the authored refresh seam. The
			// production coordinator always takes the canonical target path above.
			intro := terminal.IntroText
			state, err := coordination.UpdateLiveTerminal(terminal.Root, &intro)
			if err != nil || state == nil {
				refreshError = "active terminal could not be refreshed"
			} else {
				app.publishCoordinationState(state)
			}
		} else {
			refreshError = "coordination service is unavailable"
		}
	}

	routed := routeSessionStateResult(native)
	app.publishSessionState(SessionStateEvent{Revision: routed.Revision, Session: routed.Session})
	if refreshError != "" {
		return routeSessionStateResult(SessionStateResult{Error: refreshError, Revision: routed.Revision, Session: routed.Session})
	}
	return routed
}

func (app *App) canonicalCommandStates(terminalID string) map[string]domain.CommandExecutionState {
	sessions, ok := app.deps.Sessions.(sessionCommands)
	if !ok {
		return nil
	}
	terminal := terminalForSessionState(sessions.Snapshot().Session, terminalID)
	if terminal == nil {
		return nil
	}
	return cloneCommandExecutionStates(terminal.CommandStates)
}

func cloneCommandExecutionStates(states map[string]domain.CommandExecutionState) map[string]domain.CommandExecutionState {
	return domain.CloneCommandExecutionStates(states)
}

func terminalForSessionState(session *domain.Session, terminalID string) *domain.Terminal {
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

func (app *App) isActiveTerminal(terminalID string) bool {
	app.mu.RLock()
	defer app.mu.RUnlock()
	return app.coordinationState != nil &&
		app.coordinationState.Broadcast != nil &&
		app.coordinationState.Broadcast.ActiveTerminalID != nil &&
		*app.coordinationState.Broadcast.ActiveTerminalID == terminalID
}

func (app *App) acceptSessionStateRevision(revision uint64) {
	app.mu.Lock()
	if revision > app.requestedRevision {
		app.requestedRevision = revision
	}
	if revision > app.savedRevision {
		app.savedRevision = revision
	}
	app.saveState = string(sessionservice.SaveStateSaved)
	app.mu.Unlock()
}

func (app *App) resetSessionStateOrdering() {
	app.mu.Lock()
	app.publishedSessionRevision = 0
	app.mu.Unlock()
}

func (app *App) publishSessionState(event SessionStateEvent) {
	if event.Revision == 0 || event.Session == nil {
		return
	}
	app.mu.Lock()
	if event.Revision <= app.publishedSessionRevision {
		app.mu.Unlock()
		return
	}
	app.publishedSessionRevision = event.Revision
	app.mu.Unlock()
	if app.deps.Events != nil {
		app.emitEvent(sessionStateEvent, routeSessionStateEvent(event))
	}
}

// ForceHackSuccess completes an eligible puzzle and publishes the sanitized
// result.
func (app *App) ForceHackSuccess() CommandResult {
	if coordination, ok := app.deps.Coordination.(coordinatedHackSuccess); ok {
		state, accepted := coordination.ForceHackSuccess()
		if !accepted {
			return commandFailure("no active hacking puzzle")
		}
		app.updateHackState(state)
		return routeCommandResult(CommandResult{OK: true})
	}

	// Compatibility for partial compositions that predate the coordinator's
	// trusted operation. Production injects the ordered coordinator path.
	if app.deps.Live == nil {
		return commandFailure("live service is unavailable")
	}
	state, ok := app.deps.Live.ForceHackSuccess()
	if !ok {
		return commandFailure("no active hacking puzzle")
	}
	if publisher, ok := app.deps.Player.(playerPublisher); ok {
		publisher.PublishHack()
	}
	app.updateHackState(state)
	return routeCommandResult(CommandResult{OK: true})
}

// OpenURL validates immediately before crossing the system-browser boundary.
func (app *App) OpenURL(rawURL string) CommandResult {
	rawURL = routeOpenURLRequest(rawURL)
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return commandFailure("external URL must be an absolute HTTP or HTTPS URL")
	}
	if app.deps.Browser == nil {
		return commandFailure("external browser is unavailable")
	}
	if err := app.deps.Browser.OpenURL(parsed.String()); err != nil {
		return commandFailure("could not open the external URL")
	}
	return routeCommandResult(CommandResult{OK: true})
}

// OpenLogLocation opens the application-owned retained-log directory. The
// frontend supplies no path and cannot use this operation as a file launcher.
func (app *App) OpenLogLocation() LogAccessResult {
	result := LogAccessResult{DirectoryPath: app.deps.LogDirectory}
	if app.deps.ActiveLogPath != nil {
		result.ActiveLogPath = app.deps.ActiveLogPath()
	}
	if app.deps.LogDirectoryOpener == nil || result.DirectoryPath == "" {
		result.Error = "The application log directory is unavailable."
		return routeLogAccessResult(result)
	}
	if err := app.deps.LogDirectoryOpener.OpenDirectory(result.DirectoryPath); err != nil {
		result.Error = "Could not open the application log directory."
		return routeLogAccessResult(result)
	}
	result.OK = true
	return routeLogAccessResult(result)
}

func (app *App) failLocked(cause error) error {
	app.mu.Lock()
	app.startupError = cause.Error()
	app.phase = "failed"
	app.mu.Unlock()
	app.cancelRuntime(cause)
	cleanupContext, cancel := app.freshShutdownContext()
	defer cancel(errApplicationCleanupComplete)
	cleanupErr := app.shutdownLocked(cleanupContext, true)
	if cleanupErr != nil {
		return errors.Join(cause, cleanupErr)
	}
	return cause
}

func (app *App) shutdownLocked(ctx context.Context, preserveFailure bool) error {
	app.mu.Lock()
	if app.stopped {
		app.mu.Unlock()
		return nil
	}
	app.phase = "stopping"
	playerStarted := app.playerStarted
	desktopReady := app.desktopReady
	sessionsOpen := app.deps.Sessions != nil && !app.sessionsClosed
	publicAccessOpen := app.deps.PublicAccess != nil && !app.publicAccessClosed
	clearProcessState := !app.processStateCleared
	app.processStateCleared = true
	app.mu.Unlock()

	var cleanupErrors []error
	if publicAccessOpen {
		if err := app.deps.PublicAccess.Shutdown(ctx); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("stop embedded public access: %w", err))
		} else {
			app.mu.Lock()
			app.publicAccessClosed = true
			app.mu.Unlock()
		}
	}
	if clearProcessState && app.deps.Live != nil {
		app.deps.Live.Clear()
		app.mu.Lock()
		app.hackState = nil
		app.mu.Unlock()
	}
	if clearProcessState {
		if coordination, ok := app.deps.Coordination.(coordinationShutdownService); ok {
			coordination.Shutdown()
			app.mu.Lock()
			app.coordinationState = nil
			app.mu.Unlock()
		}
	}
	if playerStarted && app.deps.Player != nil {
		if err := app.deps.Player.Stop(ctx); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("stop player server: %w", err))
		} else {
			app.mu.Lock()
			app.playerStarted = false
			app.mu.Unlock()
		}
	}
	if sessionsOpen {
		if err := app.deps.Sessions.Shutdown(ctx); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("stop session service: %w", err))
		} else {
			app.mu.Lock()
			app.sessionsClosed = true
			app.mu.Unlock()
		}
	}
	if desktopReady && app.deps.Desktop != nil {
		if err := app.deps.Desktop.Close(ctx); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("close desktop runtime: %w", err))
		} else {
			app.mu.Lock()
			app.desktopReady = false
			app.mu.Unlock()
		}
	}

	app.mu.Lock()
	remaining := app.playerStarted || app.desktopReady ||
		(app.deps.Sessions != nil && !app.sessionsClosed) ||
		(app.deps.PublicAccess != nil && !app.publicAccessClosed)
	app.stopped = !remaining
	if !remaining {
		app.serverInfo = nil
	}
	if preserveFailure {
		app.phase = "failed"
	} else if remaining {
		app.phase = "stop-failed"
	} else {
		app.phase = "stopped"
	}
	app.mu.Unlock()
	return errors.Join(cleanupErrors...)
}

func (app *App) setPhase(phase string) {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.phase = phase
	app.stopped = false
}

func (app *App) freshShutdownContext() (context.Context, context.CancelCauseFunc) {
	return app.shutdownContext(app.root)
}

func (app *App) shutdownContext(parent context.Context) (context.Context, context.CancelCauseFunc) {
	timeout := app.deps.ShutdownTimeout
	if timeout <= 0 {
		timeout = wailsShutdownTimeout
	}
	return boundedCleanupContext(parent, timeout, errApplicationCleanupTimeout)
}

func boundedCleanupContext(parent context.Context, timeout time.Duration, timeoutCause error) (context.Context, context.CancelCauseFunc) {
	detached := context.WithoutCancel(parent)
	deadline := time.Now().Add(timeout)
	if parent.Err() == nil {
		if parentDeadline, ok := parent.Deadline(); ok && parentDeadline.Before(deadline) {
			deadline = parentDeadline
		}
	}
	deadlineContext, stopDeadline := context.WithDeadlineCause(detached, deadline, timeoutCause)
	ctx, cancel := context.WithCancelCause(deadlineContext)
	return ctx, func(cause error) {
		cancel(cause)
		stopDeadline()
	}
}

func (app *App) cancelRuntime(cause error) {
	app.mu.RLock()
	cancel := app.cancel
	app.mu.RUnlock()
	if cancel != nil {
		cancel(cause)
	}
}

func cloneServerInfo(info domain.ServerInfo) *domain.ServerInfo {
	clone := info
	return &clone
}

func cloneServerInfoPointer(info *domain.ServerInfo) *domain.ServerInfo {
	if info == nil {
		return nil
	}
	return cloneServerInfo(*info)
}

func (app *App) contextSnapshot() context.Context {
	app.mu.RLock()
	defer app.mu.RUnlock()
	return app.ctx
}

func (app *App) captureSessionStatus(commands sessionCommands) {
	app.captureActiveSessionStatus(commands.Snapshot())
}

func (app *App) captureActiveSessionStatus(snapshot sessionservice.ActiveSession) {
	app.mu.Lock()
	app.saveState = string(snapshot.SaveState)
	app.requestedRevision = snapshot.RequestedRevision
	app.savedRevision = snapshot.SavedRevision
	app.mu.Unlock()
}

// updateHackState is the trusted desktop projection boundary for accepted
// typed guess and pattern actions. Only the detached public projection enters
// RuntimeStatus or crosses the desktop event bridge.
func (app *App) updateHackState(state *domain.PublicHackState) {
	clone := clonePublicHackState(state)
	app.mu.Lock()
	app.hackState = clone
	app.mu.Unlock()
	if app.deps.Events != nil {
		app.emitEvent(hackStateEvent, routeHackStateEvent(clonePublicHackState(clone)))
	}
}

func (app *App) updateClientCount(count int) {
	if count < 0 {
		count = 0
	}
	app.mu.Lock()
	app.clientCount = count
	app.mu.Unlock()
	if app.deps.Events != nil {
		app.emitEvent(clientCountEvent, routeClientCountEvent(count))
	}
}

func (app *App) publishCoordinationState(state *domain.MasterCoordinationState) {
	clone := domain.CloneMasterCoordinationState(state)
	app.mu.Lock()
	if clone != nil && app.coordinationState != nil && clone.Revision < app.coordinationState.Revision {
		app.mu.Unlock()
		return
	}
	app.coordinationState = clone
	app.mu.Unlock()
	if app.deps.Events != nil {
		app.emitEvent(coordinationStateEvent, routeCoordinationEvent(domain.CloneMasterCoordinationState(clone)))
	}
	if app.deps.CoordinationObserver != nil {
		app.deps.CoordinationObserver.observeCoordinationState(domain.CloneMasterCoordinationState(clone))
	}
}

func (app *App) coordinationFailure(message string) CoordinationCommandResult {
	app.mu.RLock()
	state := domain.CloneMasterCoordinationState(app.coordinationState)
	app.mu.RUnlock()
	return routeCoordinationResult(CoordinationCommandResult{Error: message, State: state})
}

func (app *App) completeCoordinationCommand(state *domain.MasterCoordinationState, err error, nilStateMessage string) CoordinationCommandResult {
	if err != nil {
		if state == nil {
			return app.coordinationFailure(err.Error())
		}
		return routeCoordinationResult(CoordinationCommandResult{Error: err.Error(), State: domain.CloneMasterCoordinationState(state)})
	}
	if state == nil {
		return app.coordinationFailure(nilStateMessage)
	}
	app.publishCoordinationState(state)
	return routeCoordinationResult(CoordinationCommandResult{OK: true, State: domain.CloneMasterCoordinationState(state)})
}

func (app *App) terminalSwitchFailure(message string, state *domain.MasterCoordinationState) TerminalSwitchCommandResult {
	if state == nil {
		app.mu.RLock()
		state = domain.CloneMasterCoordinationState(app.coordinationState)
		app.mu.RUnlock()
	}
	return routeTerminalSwitchResult(TerminalSwitchCommandResult{Error: message, State: domain.CloneMasterCoordinationState(state)})
}

func (app *App) completeTerminalSwitchCommand(state *domain.MasterCoordinationState, err error, status, nilStateMessage string) TerminalSwitchCommandResult {
	if err != nil {
		return app.terminalSwitchFailure(err.Error(), state)
	}
	if state == nil {
		return app.terminalSwitchFailure(nilStateMessage, nil)
	}
	app.publishCoordinationState(state)
	return routeTerminalSwitchResult(TerminalSwitchCommandResult{
		OK: true, Status: status, State: domain.CloneMasterCoordinationState(state),
	})
}

func (app *App) completeTerminalSwitchRequest(state *domain.MasterCoordinationState, err error, completedStatus, nilStateMessage string) TerminalSwitchCommandResult {
	if err != nil {
		return app.terminalSwitchFailure(err.Error(), state)
	}
	if state == nil {
		return app.terminalSwitchFailure(nilStateMessage, nil)
	}
	status := completedStatus
	var switchID domain.SwitchID
	if state.PendingSwitch != nil {
		status = "decision-required"
		switchID = state.PendingSwitch.SwitchID
	}
	app.publishCoordinationState(state)
	return routeTerminalSwitchResult(TerminalSwitchCommandResult{
		OK: true, Status: status, SwitchID: switchID,
		State: domain.CloneMasterCoordinationState(state),
	})
}

func validatedCoordinationDisplayName(value string, label string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s must not be blank", label)
	}
	if len([]rune(value)) > 80 {
		return "", fmt.Errorf("%s must be at most 80 characters", label)
	}
	return value, nil
}

func (app *App) validateLiveTerminal(id, name string, tree domain.ContentNode, hackLevel int, intro string) error {
	validationTree := cloneTreeForBridgeValidation(tree)
	if sessions, ok := app.deps.Sessions.(sessionCommands); ok {
		active := sessions.Snapshot()
		if active.Session != nil {
			for index := range active.Session.Terminals {
				if active.Session.Terminals[index].ID != id {
					continue
				}
				terminal := active.Session.Terminals[index]
				terminal.Name = name
				terminal.HackLevel = hackLevel
				terminal.IntroText = intro
				terminal.Root = validationTree
				active.Session.Terminals[index] = terminal
				return domain.ValidateSession(*active.Session)
			}
			return fmt.Errorf("terminal %q is not part of the active session", id)
		}
	}
	return validateIsolatedLiveTerminal(id, name, validationTree, hackLevel, intro)
}

func (app *App) validateLiveTerminalUpdate(tree domain.ContentNode, intro *string) error {
	validationTree := cloneTreeForBridgeValidation(tree)
	if sessions, ok := app.deps.Sessions.(sessionCommands); ok {
		active := sessions.Snapshot()
		var state *domain.MasterCoordinationState
		if app.deps.Coordination != nil {
			state = app.deps.Coordination.Snapshot()
		}
		if active.Session != nil && state != nil && state.Broadcast != nil && state.Broadcast.ActiveTerminalID != nil {
			for index := range active.Session.Terminals {
				if active.Session.Terminals[index].ID != *state.Broadcast.ActiveTerminalID {
					continue
				}
				terminal := active.Session.Terminals[index]
				terminal.Root = validationTree
				if intro != nil {
					terminal.IntroText = *intro
				}
				active.Session.Terminals[index] = terminal
				return domain.ValidateSession(*active.Session)
			}
			return fmt.Errorf("terminal %q is not part of the active session", *state.Broadcast.ActiveTerminalID)
		}
	}
	value := ""
	if intro != nil {
		value = *intro
	}
	return validateIsolatedLiveTerminal("live-terminal", "Live Terminal", validationTree, 0, value)
}

func validateIsolatedLiveTerminal(id, name string, tree domain.ContentNode, hackLevel int, intro string) error {
	terminal := domain.Terminal{
		ID: id, Name: name, HackLevel: hackLevel, IntroText: intro, Root: tree,
	}
	return domain.ValidateSession(domain.Session{
		Version:   1,
		Name:      "Live Terminal",
		Terminals: []domain.Terminal{terminal},
	})
}

// Wails decodes an empty authored folder as either nil or an empty slice
// depending on the JavaScript payload shape. Both represent the same empty
// folder at this bridge; durable JSON decoding remains strict about arrays.
func cloneTreeForBridgeValidation(node domain.ContentNode) domain.ContentNode {
	clone := node
	if node.Type == domain.NodeFolder && node.Children == nil {
		clone.Children = []domain.ContentNode{}
	} else {
		clone.Children = make([]domain.ContentNode, len(node.Children))
	}
	for index := range node.Children {
		clone.Children[index] = cloneTreeForBridgeValidation(node.Children[index])
	}
	return clone
}

func commandFailure(message string) CommandResult {
	return routeCommandResult(CommandResult{Error: message})
}

func clonePublicHackState(state *domain.PublicHackState) *domain.PublicHackState {
	if state == nil {
		return nil
	}
	clone := *state
	if state.Log != nil {
		clone.Log = append([]string{}, state.Log...)
	}
	clone.Patterns = append([]domain.PublicHackPattern(nil), state.Patterns...)
	clone.Columns = make([]domain.HackColumn, len(state.Columns))
	for index, column := range state.Columns {
		clone.Columns[index] = column
		clone.Columns[index].Addresses = append([]string(nil), column.Addresses...)
		clone.Columns[index].Words = append([]domain.HackWord(nil), column.Words...)
	}
	return &clone
}
