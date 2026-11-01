'use strict';

const desktopAPI = window.desktopAPI;

// ── State ─────────────────────────────────────────────────
const state = {
  session:        null,   // { version, name, terminals: [{id,name,hackLevel,introText,root}] }
  filePath:       null,
  liveTerminalId: null,
  editTerminalId: null,
  selectedNodeId: null,
  expanded:       new Set(['root']),
  collapsedTerminalGroupIds: new Set(),
  liveHack:       null,   // last known hack-state of the live terminal, or null
  coordination:   null,   // authoritative roster/session/broadcast projection
};

let idCounter = 0;
function uid(prefix) {
  idCounter++;
  return `${prefix}_${Date.now().toString(36)}_${idCounter}`;
}

// ── DOM refs ──────────────────────────────────────────────
const startScreen      = document.getElementById('startScreen');
const startStatus      = document.getElementById('startStatus');
const btnOpenSession   = document.getElementById('btnOpenSession');
const btnNewSession    = document.getElementById('btnNewSession');
const logLocationButtons = Array.from(document.querySelectorAll('[data-action="open-log-location"]'));
const logAccessStatuses = Array.from(document.querySelectorAll('[data-log-access-status]'));
const mainLayout        = document.getElementById('mainLayout');
const sessionFileLabel  = document.getElementById('sessionFileLabel');
const serverUrlEl       = document.getElementById('serverUrl');
const clientCountEl     = document.getElementById('clientCount');
const termList          = document.getElementById('termList');
const saveStatus        = document.getElementById('saveStatus');
const saveStatusEvidence = document.getElementById('saveStatusEvidence');
const btnUndo           = document.getElementById('btnUndo');
const btnRedo           = document.getElementById('btnRedo');
const btnReapplyEdit    = document.getElementById('btnReapplyEdit');
const btnResetScene     = document.getElementById('btnResetScene');
const editingTermName   = document.getElementById('editingTermName');
const liveFlag          = document.getElementById('liveFlag');
const btnMakeLive       = document.getElementById('btnMakeLive');
const terminalSettingsMenu = document.getElementById('terminalSettingsMenu');
const btnReapplySettings = document.getElementById('btnReapplySettings');
const treeView          = document.getElementById('treeView');
const nodeForm          = document.getElementById('nodeForm');
const toolbarHint       = document.getElementById('toolbarHint');
const btnAddFolder      = document.getElementById('btnAddFolder');
const btnAddCommand     = document.getElementById('btnAddCommand');
const btnAddEntry       = document.getElementById('btnAddEntry');
const btnAddTerminal    = document.getElementById('btnAddTerminal');
const btnCreateTerminalGroup = document.getElementById('btnCreateTerminalGroup');
const terminalGroupError = document.getElementById('terminalGroupError');
const terminalGroupDraftDialog = document.getElementById('terminalGroupDraftDialog');
const terminalGroupDraftForm = document.getElementById('terminalGroupDraftForm');
const terminalGroupNameInput = document.getElementById('terminalGroupNameInput');
const terminalGroupTerminalChoices = document.querySelector('#terminalGroupTerminalChoices .terminal-group-terminal-choice-list');
const terminalGroupTerminalChoiceTemplate = document.getElementById('terminalGroupTerminalChoiceTemplate');
const terminalGroupDestinationSelect = document.getElementById('terminalGroupDestinationSelect');
const terminalGroupImpactDialog = document.getElementById('terminalGroupImpactDialog');
const terminalGroupImpactSummary = document.getElementById('terminalGroupImpactSummary');
const terminalGroupImpactError = document.getElementById('terminalGroupImpactError');
const amendTerminalGroupChangeButton = terminalGroupImpactDialog.querySelector('[data-action="amend-terminal-group-change"]');
const btnStopBroadcast  = document.getElementById('btnStopBroadcast');
const createTerminalDialog = document.getElementById('createTerminalDialog');
const createTerminalForm = document.getElementById('createTerminalForm');
const createTerminalName = document.getElementById('createTerminalName');
const createTerminalError = document.getElementById('createTerminalError');
const btnCancelCreateTerminal = document.getElementById('btnCancelCreateTerminal');
const btnConfirmCreateTerminal = document.getElementById('btnConfirmCreateTerminal');
const takeOffAirDialog = document.getElementById('takeOffAirDialog');
const takeOffAirError = document.getElementById('takeOffAirError');
const btnCancelTakeOffAir = document.getElementById('btnCancelTakeOffAir');
const btnConfirmTakeOffAir = document.getElementById('btnConfirmTakeOffAir');
const hackStatus        = document.getElementById('hackStatus');
const hackStatusLine    = document.getElementById('hackStatusLine');
const btnHackSuccess    = document.getElementById('btnHackSuccess');
const btnResetFailedHack = document.getElementById('btnResetFailedHack');
const hackLevelSelect   = document.getElementById('hackLevelSelect');
const introTextArea     = document.getElementById('introTextArea');
const btnApplySettings  = document.getElementById('btnApplySettings');
const btnCancelSettings = document.getElementById('btnCancelSettings');
const termSettings      = document.getElementById('termSettings');
const broadcastSummary = document.getElementById('broadcastSummary');
const coordinationPanel = document.getElementById('coordinationPanel');
const playerConfigStatus = document.getElementById('playerConfigStatus');
const playerConfigError = document.getElementById('playerConfigError');
const btnOpenPlayerConfig = document.getElementById('btnOpenPlayerConfig');
const btnNewPlayerConfig = document.getElementById('btnNewPlayerConfig');
const btnManagePlayers = document.getElementById('btnManagePlayers');
const btnStartBroadcast = document.getElementById('btnStartBroadcast');
const btnEndBroadcast = document.getElementById('btnEndBroadcast');
const endBroadcastDialog = document.getElementById('endBroadcastDialog');
const btnCancelEndBroadcast = document.getElementById('btnCancelEndBroadcast');
const btnConfirmEndBroadcast = document.getElementById('btnConfirmEndBroadcast');
const coordinationStatus = document.getElementById('coordinationStatus');
const coordinationError = document.getElementById('coordinationError');
const activeLogicalSessionCount = document.getElementById('activeLogicalSessionCount');
const btnManageLogicalSessions = document.getElementById('btnManageLogicalSessions');
const logicalSessionDialog = document.getElementById('logicalSessionDialog');
const btnCloseLogicalSessions = document.getElementById('btnCloseLogicalSessions');
const logicalSessionDialogStatus = document.getElementById('logicalSessionDialogStatus');
const logicalSessionDialogError = document.getElementById('logicalSessionDialogError');
const logicalSessionList = document.getElementById('logicalSessionList');
const logicalSessionRowTemplate = document.getElementById('logicalSessionRowTemplate');
const playerManagementDialog = document.getElementById('playerManagementDialog');
const playerManagementMode = document.getElementById('playerManagementMode');
const playerManagementRoster = document.getElementById('playerManagementRoster');
const playerManagementEmpty = document.getElementById('playerManagementEmpty');
const playerManagementAddForm = document.getElementById('playerManagementAddForm');
const playerNameInput = document.getElementById('playerNameInput');
const playerIntelligenceInput = document.getElementById('playerIntelligenceInput');
const playerHackerPerkAvailability = document.getElementById('playerHackerPerkAvailability');
const btnAddPlayer = document.getElementById('btnAddPlayer');
const playerManagementStatus = document.getElementById('playerManagementStatus');
const playerManagementError = document.getElementById('playerManagementError');
const btnClosePlayerManagement = document.getElementById('btnClosePlayerManagement');
const playerManagementRowTemplate = document.getElementById('playerManagementRowTemplate');
const playerDeleteDialog = document.getElementById('playerDeleteDialog');
const playerDeleteDialogDescription = document.getElementById('playerDeleteDialogDescription');
const btnConfirmPlayerDelete = document.getElementById('btnConfirmPlayerDelete');
const btnCancelPlayerDelete = document.getElementById('btnCancelPlayerDelete');
const terminalSwitchDialog = document.getElementById('terminalSwitchDialog');
const terminalSwitchStatus = document.getElementById('terminalSwitchStatus');
const terminalSwitchError = document.getElementById('terminalSwitchError');
const terminalSwitchButtons = Array.from(document.querySelectorAll('[data-switch-decision]'));
const publicAccessSection = document.getElementById('publicAccessSection');
const publicAccessStateRow = document.querySelector('.public-access-state-row');
const publicAccessSettingsDialog = document.getElementById('publicAccessSettingsDialog');
const publicAccessSetupRequired = document.getElementById('publicAccessSetupRequired');
const publicAccessSettingsError = document.getElementById('publicAccessSettingsError');
const publicAccessGuide = document.getElementById('publicAccessGuide');
const publicAccessForm = document.getElementById('publicAccessForm');
const publicAccessDomain = document.getElementById('publicAccessDomain');
const publicAccessUsernameSummary = document.getElementById('publicAccessUsernameSummary');
const publicAccessPasswordMask = document.getElementById('publicAccessPasswordMask');
const publicAccessProviderSetup = document.getElementById('publicAccessProviderSetup');
const publicAccessProviderConfigured = document.getElementById('publicAccessProviderConfigured');
const publicAccessProviderToken = document.getElementById('publicAccessProviderToken');
const publicAccessProviderPresence = document.getElementById('publicAccessProviderPresence');
const publicAccessPasswordPresence = document.getElementById('publicAccessPasswordPresence');
const publicAccessNgrokDocLinks = document.querySelectorAll('[data-ngrok-doc-url]');
const publicAccessProviderTokenDialog = document.getElementById('publicAccessProviderTokenDialog');
const publicAccessProviderTokenForm = document.getElementById('publicAccessProviderTokenForm');
const publicAccessReplacementProviderToken = document.getElementById('publicAccessReplacementProviderToken');
const publicAccessProviderTokenError = document.getElementById('publicAccessProviderTokenError');
const publicAccessPlayerCredentialsDialog = document.getElementById('publicAccessPlayerCredentialsDialog');
const publicAccessPlayerCredentialsForm = document.getElementById('publicAccessPlayerCredentialsForm');
const publicAccessReplacementUsername = document.getElementById('publicAccessReplacementUsername');
const publicAccessReplacementPlayerPassword = document.getElementById('publicAccessReplacementPlayerPassword');
const publicAccessReplacementPlayerPasswordHint = document.getElementById('publicAccessReplacementPlayerPasswordHint');
const publicAccessPlayerCredentialsError = document.getElementById('publicAccessPlayerCredentialsError');
const publicAccessStatus = document.getElementById('publicAccessStatus');
const publicAccessError = document.getElementById('publicAccessError');
const publicAccessURL = document.getElementById('publicAccessURL');
const publicAccessCopyStatus = document.getElementById('publicAccessCopyStatus');
const publicAccessSettingsCopyStatus = document.getElementById('publicAccessSettingsCopyStatus');
const btnSavePublicAccess = document.getElementById('btnSavePublicAccess');
const btnStartPublicAccess = document.getElementById('btnStartPublicAccess');
const btnStopPublicAccess = document.getElementById('btnStopPublicAccess');
const btnCopyPublicURL = document.getElementById('btnCopyPublicURL');
const btnOpenPublicAccessSettings = document.getElementById('btnOpenPublicAccessSettings');
const btnClosePublicAccessSettings = document.getElementById('btnClosePublicAccessSettings');
const btnCancelPublicAccessSettings = document.getElementById('btnCancelPublicAccessSettings');
const btnOpenPublicAccessProviderToken = document.getElementById('btnOpenPublicAccessProviderToken');
const btnCancelPublicAccessProviderToken = document.getElementById('btnCancelPublicAccessProviderToken');
const btnSavePublicAccessProviderToken = document.getElementById('btnSavePublicAccessProviderToken');
const btnDeletePublicAccessProviderToken = document.getElementById('btnDeletePublicAccessProviderToken');
const btnOpenPublicAccessPlayerCredentials = document.getElementById('btnOpenPublicAccessPlayerCredentials');
const btnSharePublicAccessCredentials = document.getElementById('btnSharePublicAccessCredentials');
const btnCancelPublicAccessPlayerCredentials = document.getElementById('btnCancelPublicAccessPlayerCredentials');
const btnSavePublicAccessPlayerCredentials = document.getElementById('btnSavePublicAccessPlayerCredentials');
const btnDeletePublicAccessPlayerCredentials = document.getElementById('btnDeletePublicAccessPlayerCredentials');
const btnGeneratePlayerPassword = document.getElementById('btnGeneratePlayerPassword');
const generatedPasswordDialog = document.getElementById('generatedPasswordDialog');
const generatedPasswordValue = document.getElementById('generatedPasswordValue');
const btnCopyGeneratedPassword = document.getElementById('btnCopyGeneratedPassword');
const btnDismissGeneratedPassword = document.getElementById('btnDismissGeneratedPassword');
const applicationUpdateStatusPanel = document.getElementById('applicationUpdateStatusPanel');
const applicationUpdateStatus = document.getElementById('applicationUpdateStatus');
const applicationUpdateError = document.getElementById('applicationUpdateError');
const applicationUpdateProgress = document.getElementById('applicationUpdateProgress');
const btnShowApplicationUpdate = document.getElementById('btnShowApplicationUpdate');
const applicationUpdateDialog = document.getElementById('applicationUpdateDialog');
const applicationUpdateInstalledVersion = document.getElementById('applicationUpdateInstalledVersion');
const applicationUpdateAvailableVersion = document.getElementById('applicationUpdateAvailableVersion');
const applicationUpdateReleaseNotes = document.getElementById('applicationUpdateReleaseNotes');
const btnAcceptApplicationUpdate = document.getElementById('btnAcceptApplicationUpdate');
const btnDeferApplicationUpdate = document.getElementById('btnDeferApplicationUpdate');
const applicationUpdateRestartDialog = document.getElementById('applicationUpdateRestartDialog');
const btnRestartApplicationUpdate = document.getElementById('btnRestartApplicationUpdate');
const btnPostponeApplicationUpdate = document.getElementById('btnPostponeApplicationUpdate');
const entryBlockCommandDialog = document.getElementById('entryBlockCommandDialog');
const entryBlockCommandDialogDescription = document.getElementById('entryBlockCommandDialogDescription');
const entryBlockCommandForm = document.getElementById('entryBlockCommandForm');
const entryBlockCommandModeInputs = Array.from(document.querySelectorAll('input[name="entryBlockCommandMode"]'));
const entryBlockExistingCommand = document.getElementById('entryBlockExistingCommand');
const entryBlockCompletedText = document.getElementById('entryBlockCompletedText');
const entryBlockNewCommandName = document.getElementById('entryBlockNewCommandName');
const entryBlockNewCompletedName = document.getElementById('entryBlockNewCompletedName');
const entryBlockNewConfirmationText = document.getElementById('entryBlockNewConfirmationText');
const entryBlockNewResultText = document.getElementById('entryBlockNewResultText');
const entryBlockNewDestinationFolder = document.getElementById('entryBlockNewDestinationFolder');
const entryBlockCommandError = document.getElementById('entryBlockCommandError');
const btnApplyEntryBlockCommand = entryBlockCommandDialog.querySelector('[data-entry-block-command-action="apply"]');
const btnRemoveEntryBlockCommand = entryBlockCommandDialog.querySelector('[data-entry-block-command-action="remove"]');
const btnCancelEntryBlockCommand = entryBlockCommandDialog.querySelector('[data-entry-block-command-action="cancel"]');

const terminalWorkspaceTab = document.getElementById('terminalWorkspaceTab');
const facilityWorkspaceTab = document.getElementById('facilityWorkspaceTab');
const terminalWorkspace = document.getElementById('terminalWorkspace');
const facilityWorkspace = document.getElementById('facilityWorkspace');
const facilityValidation = document.getElementById('facilityValidation');
const facilityDeviceList = document.getElementById('facilityDeviceList');
const facilityActionBindingList = document.getElementById('facilityActionBindingList');
const facilityInformationBindingList = document.getElementById('facilityInformationBindingList');
const facilityConditionList = document.getElementById('facilityConditionList');
const facilityRecoveryProgramList = document.getElementById('facilityRecoveryProgramList');
const facilitySelectionEditor = document.getElementById('facilitySelectionEditor');
const facilitySelectionStatus = document.getElementById('facilitySelectionStatus');
const facilitySelectionError = document.getElementById('facilitySelectionError');
const btnPreviewFacilityDevice = document.getElementById('btnPreviewFacilityDevice');
const btnResetFacilityDevice = document.getElementById('btnResetFacilityDevice');
const btnPreviewFacilityCondition = document.getElementById('btnPreviewFacilityCondition');
const btnAddFacilityDevice = document.getElementById('btnAddFacilityDevice');
const btnAddFacilityCondition = document.getElementById('btnAddFacilityCondition');
const btnAddFacilityRecoveryProgram = document.getElementById('btnAddFacilityRecoveryProgram');
const btnAddFacilityBinding = document.getElementById('btnAddFacilityBinding');
const btnInspectFacilityDependencies = document.getElementById('btnInspectFacilityDependencies');
const btnDeleteFacilityDevice = document.getElementById('btnDeleteFacilityDevice');
const btnFacilityDeviceMenu = document.getElementById('btnFacilityDeviceMenu');
const facilityDeviceMenu = document.getElementById('facilityDeviceMenu');
const facilityDeviceDialog = document.getElementById('facilityDeviceDialog');
const facilityDeviceForm = document.getElementById('facilityDeviceForm');
const facilityDeviceID = document.getElementById('facilityDeviceID');
const facilityDeviceName = document.getElementById('facilityDeviceName');
const facilityDeviceKind = document.getElementById('facilityDeviceKind');
const facilityDeviceTemplate = document.getElementById('facilityDeviceTemplate');
const facilityDeviceStateChips = document.getElementById('facilityDeviceStateChips');
const facilityStateEditor = document.getElementById('facilityStateEditor');
const facilityStateEditorName = document.getElementById('facilityStateEditorName');
const facilityStateUses = document.getElementById('facilityStateUses');
const facilityStateDeleteConfirm = document.getElementById('facilityStateDeleteConfirm');
const facilityTransitionMatrix = document.getElementById('facilityTransitionMatrix');
const facilityTransitionMatrixHint = document.getElementById('facilityTransitionMatrixHint');
const facilityTransitionPanel = document.getElementById('facilityTransitionPanel');
const facilityTransitionCellList = document.getElementById('facilityTransitionCellList');
const facilityTransitionUses = document.getElementById('facilityTransitionUses');
const facilityDeviceTechnical = document.getElementById('facilityDeviceTechnical');
const facilityDeviceStateIDs = document.getElementById('facilityDeviceStateIDs');
const facilityDeviceTransitionIDs = document.getElementById('facilityDeviceTransitionIDs');
const facilityDeviceStatus = document.getElementById('facilityDeviceStatus');
const facilityDeviceError = document.getElementById('facilityDeviceError');
const btnApplyFacilityDeviceTemplate = document.getElementById('btnApplyFacilityDeviceTemplate');
const btnAddFacilityState = document.getElementById('btnAddFacilityState');
const btnMakeFacilityStateInitial = document.getElementById('btnMakeFacilityStateInitial');
const btnDeleteFacilityState = document.getElementById('btnDeleteFacilityState');
const btnAddFacilityCellTransition = document.getElementById('btnAddFacilityCellTransition');
const btnCloseFacilityTransitionPanel = document.getElementById('btnCloseFacilityTransitionPanel');
const btnCancelFacilityDevice = document.getElementById('btnCancelFacilityDevice');
const facilityTransitionEditorTemplate = document.getElementById('facilityTransitionEditorTemplate');
const facilityConditionDialog = document.getElementById('facilityConditionDialog');
const facilityConditionForm = document.getElementById('facilityConditionForm');
const facilityConditionID = document.getElementById('facilityConditionID');
const facilityConditionName = document.getElementById('facilityConditionName');
const facilityConditionCategory = document.getElementById('facilityConditionCategory');
const facilityConditionScope = document.getElementById('facilityConditionScope');
const facilityConditionDevice = document.getElementById('facilityConditionDevice');
const facilityConditionTerminal = document.getElementById('facilityConditionTerminal');
const facilityConditionCapability = document.getElementById('facilityConditionCapability');
const facilityConditionInitialWorking = document.getElementById('facilityConditionInitialWorking');
const facilityConditionInitialActive = document.getElementById('facilityConditionInitialActive');
const facilityFaultAppearsList = document.getElementById('facilityFaultAppearsList');
const facilityFaultAppearsWarning = document.getElementById('facilityFaultAppearsWarning');
const facilityFaultActivationPicker = document.getElementById('facilityFaultActivationPicker');
const facilityFaultClearsList = document.getElementById('facilityFaultClearsList');
const facilityFaultClearingPicker = document.getElementById('facilityFaultClearingPicker');
const btnAddFaultActivation = document.getElementById('btnAddFaultActivation');
const btnAddFaultClearing = document.getElementById('btnAddFaultClearing');
const btnAddFaultRecoveryProgram = document.getElementById('btnAddFaultRecoveryProgram');
const facilityConditionTechnical = document.getElementById('facilityConditionTechnical');
const facilityConditionError = document.getElementById('facilityConditionError');
const btnCancelFacilityCondition = document.getElementById('btnCancelFacilityCondition');
const facilityRecoveryProgramDialog = document.getElementById('facilityRecoveryProgramDialog');
const facilityRecoveryProgramForm = document.getElementById('facilityRecoveryProgramForm');
const facilityRecoveryProgramFault = document.getElementById('facilityRecoveryProgramFault');
const facilityRecoveryProgramID = document.getElementById('facilityRecoveryProgramID');
const facilityRecoveryProgramName = document.getElementById('facilityRecoveryProgramName');
const facilityProgramTransitions = document.getElementById('facilityProgramTransitions');
const btnAddFacilityProgramTransition = document.getElementById('btnAddFacilityProgramTransition');
const facilityRecoveryProgramTechnical = document.getElementById('facilityRecoveryProgramTechnical');
const facilityRecoveryProgramError = document.getElementById('facilityRecoveryProgramError');
const btnCancelFacilityRecoveryProgram = document.getElementById('btnCancelFacilityRecoveryProgram');
const facilityBindingDialog = document.getElementById('facilityBindingDialog');
const facilityBindingForm = document.getElementById('facilityBindingForm');
const facilityRuleTabs = document.getElementById('facilityRuleTabs');
const facilityRuleSentence = document.getElementById('facilityRuleSentence');
const facilityBindingNode = document.getElementById('facilityBindingNode');
const facilityBindingDevice = document.getElementById('facilityBindingDevice');
const facilityBindingState = document.getElementById('facilityBindingState');
const facilityBindingBlock = document.getElementById('facilityBindingBlock');
const facilityBindingText = document.getElementById('facilityBindingText');
const facilityRuleConflict = document.getElementById('facilityRuleConflict');
const btnOpenConflictingFacilityRule = document.getElementById('btnOpenConflictingFacilityRule');
const facilityBindingError = document.getElementById('facilityBindingError');
const btnCancelFacilityBinding = document.getElementById('btnCancelFacilityBinding');
const facilityCommandActionDialog = document.getElementById('facilityCommandActionDialog');
const facilityCommandActionForm = document.getElementById('facilityCommandActionForm');
const facilityCommandActionCommand = document.getElementById('facilityCommandActionCommand');
const facilityCommandActionCommandFixed = document.getElementById('facilityCommandActionCommandFixed');
const facilityCommandActionMode = document.getElementById('facilityCommandActionMode');
const facilityCommandActionModeTransitions = document.getElementById('facilityCommandActionModeTransitions');
const facilityCommandActionModeProgram = document.getElementById('facilityCommandActionModeProgram');
const facilityCommandActionTransitions = document.getElementById('facilityCommandActionTransitions');
const btnAddFacilityCommandActionTransition = document.getElementById('btnAddFacilityCommandActionTransition');
const facilityCommandActionProgram = document.getElementById('facilityCommandActionProgram');
const facilityCommandActionCompletedName = document.getElementById('facilityCommandActionCompletedName');
const facilityCommandActionConfirmationText = document.getElementById('facilityCommandActionConfirmationText');
const facilityCommandActionError = document.getElementById('facilityCommandActionError');
const btnAddFacilityActionBinding = document.getElementById('btnAddFacilityActionBinding');
const facilityDependencyDialog = document.getElementById('facilityDependencyDialog');
const facilityDependencyList = document.getElementById('facilityDependencyList');
const facilityDependencyStatus = document.getElementById('facilityDependencyStatus');
const facilityDependencyError = document.getElementById('facilityDependencyError');
const btnCloseFacilityDependencies = document.getElementById('btnCloseFacilityDependencies');
const facilityRepairDialog = document.getElementById('facilityRepairDialog');
const facilityRepairError = document.getElementById('facilityRepairError');
const facilityRepairTarget = document.getElementById('facilityRepairTarget');
const facilityRepairImpact = document.getElementById('facilityRepairImpact');
const facilityRepairDependencyList = document.getElementById('facilityRepairDependencyList');
const btnValidateFacilityRepair = document.getElementById('btnValidateFacilityRepair');
const btnCancelFacilityRepair = document.getElementById('btnCancelFacilityRepair');
const btnDeleteFacilityWithoutRepair = document.getElementById('btnDeleteFacilityWithoutRepair');
const btnApplyFacilityRepair = document.getElementById('btnApplyFacilityRepair');
const facilityPreviewDialog = document.getElementById('facilityPreviewDialog');
const facilityPreviewTerminal = document.getElementById('facilityPreviewTerminal');
const facilityPreviewStateLabel = document.getElementById('facilityPreviewStateLabel');
const facilityPreviewState = document.getElementById('facilityPreviewState');
const facilityPreviewActiveLabel = document.getElementById('facilityPreviewActiveLabel');
const facilityPreviewActive = document.getElementById('facilityPreviewActive');
const facilityPreviewSummary = document.getElementById('facilityPreviewSummary');
const facilityPreviewCurrent = document.getElementById('facilityPreviewCurrent');
const facilityPreviewHypothetical = document.getElementById('facilityPreviewHypothetical');
const facilityPreviewStatus = document.getElementById('facilityPreviewStatus');
const facilityPreviewError = document.getElementById('facilityPreviewError');
const btnRefreshFacilityPreview = document.getElementById('btnRefreshFacilityPreview');
const btnCloseFacilityPreview = document.getElementById('btnCloseFacilityPreview');
const facilityDeviceResetDialog = document.getElementById('facilityDeviceResetDialog');
const btnForceFacilityDeviceState = document.getElementById('btnForceFacilityDeviceState');
const facilityForceStateDialog = document.getElementById('facilityForceStateDialog');
const facilityForceStateTarget = document.getElementById('facilityForceStateTarget');
const facilityForceStateSummary = document.getElementById('facilityForceStateSummary');
const facilityForceStateError = document.getElementById('facilityForceStateError');
const btnConfirmFacilityForceState = document.getElementById('btnConfirmFacilityForceState');
const btnCancelFacilityDeviceReset = document.getElementById('btnCancelFacilityDeviceReset');
const btnConfirmFacilityDeviceReset = document.getElementById('btnConfirmFacilityDeviceReset');
const sceneResetDialog = document.getElementById('sceneResetDialog');
const sceneResetCountsList = document.getElementById('sceneResetCounts');
const sceneResetPending = document.getElementById('sceneResetPending');
const sceneResetStatus = document.getElementById('sceneResetStatus');
const btnCancelSceneReset = document.getElementById('btnCancelSceneReset');
const btnConfirmSceneReset = document.getElementById('btnConfirmSceneReset');

let serverUrl = null;
let serverUrlTitle = '';
let newestDurableRevision = 0;
let coordinationCommandPending = false;
// facilityPlayPending is true while one Overseer play change is in flight.
let facilityPlayPending = false;
let createTerminalSubmitting = false;
let takeOffAirPending = false;
let pendingTerminalSwitch = null;
let startupStatus = null;
let publicAccessSnapshot = null;
let publicAccessCommandPending = false;
let publicAccessSettingsDialogOpener = null;
let publicAccessProviderTokenDialogOpener = null;
let publicAccessPlayerCredentialsDialogOpener = null;
let sessionStateCommandPending = false;
let commandExecutionDialogRequestID = null;
let commandExecutionDecisionRequestID = null;
let commandExecutionDialogMode = null;
let commandExecutionDialogEpoch = 0;
const resolvedCommandExecutionRequestIDs = new Set();
let terminalNavigationDialogRequestID = null;
let terminalNavigationDecisionRequestID = null;
let terminalNavigationDialogEpoch = 0;
const resolvedTerminalNavigationRequestIDs = new Set();
let logicalSessionDialogOpener = null;
let playerManagementOpener = null;
let pendingPlayerDelete = null;
let terminalGroupDraft = null;
let pendingTerminalGroupImpact = null;
let terminalGroupSubmitting = false;
let terminalGroupDialogOpener = null;
let applicationUpdateSnapshot = null;
let applicationUpdateDialogAttemptID = '';
let applicationUpdateDialogOpener = null;
let applicationUpdateCommandPending = false;
let applicationUpdateRestartDialogAttemptID = '';
let applicationUpdateRestartDialogOpener = null;
let applicationUpdateRestartCommandPending = false;
let latestRenderedApplicationUpdateRevision = -1;
let entryBlockCommandDraft = null;
let selectedFacilityDeviceID = '';
// facilityDeviceFilter is the pressed device list filter for this run: a key
// of facilityDeviceFilters (FR-033).
let facilityDeviceFilter = 'all';
let facilityPreviewSelection = null;
let facilityPreviewGeneration = 0;
let facilityOperationOpener = null;
let pendingFacilityRepair = null;
let editingFacilityDeviceID = '';
let editingFacilityConditionID = '';
// facilityFaultEdits holds the fault dialog's list edits over the newest
// session (R2). Each facilityTransitionKey maps to the effect the draft gives
// that transition: true turns the fault on, false clears it, null removes it.
let facilityFaultEdits = new Map();
let editingFacilityBinding = null;
// facilityRuleKind is the open tab of the display rule dialog.
// facilityRuleElements is the «ГДЕ» catalog built when the dialog opens; its
// indexes are the option values (R5). facilityRuleConflictBinding is the rule
// «ОТКРЫТЬ ПРАВИЛО» opens.
let facilityRuleKind = 'menu-name';
let facilityRuleElements = [];
let facilityRuleConflictBinding = null;
// facilityCommandActionFixed is the { terminalId, nodeId } of the command the
// action dialog edits, or null while «КОМАНДА» picks it from
// facilityCommandActionCommands, whose indexes are the option values (R7).
// facilityCommandActionReturn gets focus when the dialog closes.
let facilityCommandActionFixed = null;
let facilityCommandActionCommands = [];
let facilityCommandActionReturn = null;
let editingFacilityProgramID = '';
// facilityProgramReturn is the control that gets focus when the program
// dialog closes; facilityProgramFromFault marks it as stacked over the fault
// dialog (R4).
let facilityProgramReturn = null;
let facilityProgramFromFault = false;
let facilityStoryStep = 0;
// The author's explicit guide choice for this run, 'expanded' or 'collapsed';
// null lets the guide follow the device count (FR-019, FR-020).
let facilityGuideChoice = null;
let facilityGuideDeviceCount = null;
// facilityDetailTab is the open detail tab for this run.
let facilityDetailTab = 'actions';
const promptedApplicationUpdateRevisions = new Set();
const promptedApplicationUpdateRestartRevisions = new Set();
const suppressedApplicationUpdateAttempts = new Set();

const commandExecutionDialog = document.createElement('dialog');
commandExecutionDialog.className = 'terminal-switch-dialog command-execution-dialog';
commandExecutionDialog.id = 'commandExecutionDialog';
commandExecutionDialog.hidden = true;
commandExecutionDialog.setAttribute('aria-modal', 'true');
commandExecutionDialog.setAttribute('aria-labelledby', 'commandExecutionDialogTitle');
commandExecutionDialog.setAttribute('aria-describedby', 'commandExecutionDialogDescription commandExecutionFacilityImpact commandExecutionDialogStatus commandExecutionDialogError');
commandExecutionDialog.innerHTML = `
  <div class="terminal-switch-dialog-panel">
    <h2 class="terminal-switch-dialog-title" id="commandExecutionDialogTitle">ПОДТВЕРЖДЕНИЕ КОМАНДЫ</h2>
    <p class="terminal-switch-dialog-description" id="commandExecutionDialogDescription"></p>
    <div class="terminal-navigation-summary" id="commandExecutionFacilityImpact" aria-label="Влияние на локацию" hidden></div>
    <div class="terminal-switch-actions" role="group" aria-label="Решение мастера по выполнению команды" style="grid-template-columns:repeat(2,minmax(0,1fr))">
      <button class="btn btn-primary" id="btnApproveCommandExecution" type="button">ОДОБРИТЬ</button>
      <button class="btn btn-danger" id="btnRejectCommandExecution" type="button">ОТКЛОНИТЬ</button>
    </div>
    <div class="terminal-switch-status" id="commandExecutionDialogStatus" role="status" aria-live="polite" aria-atomic="true"></div>
    <div class="terminal-switch-error" id="commandExecutionDialogError" role="alert" aria-live="assertive" aria-atomic="true" hidden></div>
  </div>`;
document.body.appendChild(commandExecutionDialog);
const commandExecutionDialogDescription = document.getElementById('commandExecutionDialogDescription');
const commandExecutionFacilityImpact = document.getElementById('commandExecutionFacilityImpact');
const commandExecutionDialogStatus = document.getElementById('commandExecutionDialogStatus');
const commandExecutionDialogError = document.getElementById('commandExecutionDialogError');
const btnApproveCommandExecution = document.getElementById('btnApproveCommandExecution');
const btnRejectCommandExecution = document.getElementById('btnRejectCommandExecution');

const terminalNavigationDialog = document.createElement('dialog');
terminalNavigationDialog.className = 'terminal-switch-dialog terminal-navigation-dialog';
terminalNavigationDialog.id = 'terminalNavigationDialog';
terminalNavigationDialog.hidden = true;
terminalNavigationDialog.setAttribute('aria-modal', 'true');
terminalNavigationDialog.setAttribute('aria-labelledby', 'terminalNavigationDialogTitle');
terminalNavigationDialog.innerHTML = `
  <div class="terminal-switch-dialog-panel">
    <h2 class="terminal-switch-dialog-title" id="terminalNavigationDialogTitle">ПЕРЕХОД МЕЖДУ ТЕРМИНАЛАМИ</h2>
    <div class="terminal-navigation-summary" id="terminalNavigationSummary"></div>
    <div class="terminal-switch-actions" role="group" aria-label="Решение мастера по переходу" style="grid-template-columns:repeat(2,minmax(0,1fr))">
      <button class="btn btn-primary" id="btnApproveTerminalNavigation" type="button">ОДОБРИТЬ</button>
      <button class="btn btn-danger" id="btnRejectTerminalNavigation" type="button">ОТКЛОНИТЬ</button>
    </div>
    <div class="terminal-switch-status" id="terminalNavigationStatus" role="status" aria-live="polite"></div>
    <div class="terminal-switch-error" id="terminalNavigationError" role="alert" hidden></div>
  </div>`;
document.body.appendChild(terminalNavigationDialog);
const terminalNavigationSummary = document.getElementById('terminalNavigationSummary');
const terminalNavigationStatus = document.getElementById('terminalNavigationStatus');
const terminalNavigationError = document.getElementById('terminalNavigationError');
const btnApproveTerminalNavigation = document.getElementById('btnApproveTerminalNavigation');
const btnRejectTerminalNavigation = document.getElementById('btnRejectTerminalNavigation');

let resetConfirmationResolve = null;
const resetConfirmationDialog = document.createElement('dialog');
resetConfirmationDialog.className = 'terminal-switch-dialog command-state-reset-dialog';
resetConfirmationDialog.id = 'resetConfirmationDialog';
resetConfirmationDialog.hidden = true;
resetConfirmationDialog.setAttribute('aria-modal', 'true');
resetConfirmationDialog.setAttribute('aria-labelledby', 'resetConfirmationDialogTitle');
resetConfirmationDialog.setAttribute('aria-describedby', 'resetConfirmationDialogDescription');
resetConfirmationDialog.innerHTML = `
  <div class="terminal-switch-dialog-panel">
    <h2 class="terminal-switch-dialog-title" id="resetConfirmationDialogTitle">ПОДТВЕРЖДЕНИЕ СБРОСА</h2>
    <p class="terminal-switch-dialog-description" id="resetConfirmationDialogDescription"></p>
    <div class="terminal-switch-actions" role="group" aria-label="Подтверждение сброса состояния команды" style="grid-template-columns:repeat(2,minmax(0,1fr))">
      <button class="btn btn-danger" id="btnConfirmCommandStateReset" type="button">ПОДТВЕРДИТЬ</button>
      <button class="btn" id="btnCancelCommandStateReset" type="button">ОТМЕНИТЬ</button>
    </div>
  </div>`;
document.body.appendChild(resetConfirmationDialog);
const resetConfirmationDialogDescription = document.getElementById('resetConfirmationDialogDescription');
const btnConfirmCommandStateReset = document.getElementById('btnConfirmCommandStateReset');
const btnCancelCommandStateReset = document.getElementById('btnCancelCommandStateReset');

function finishResetConfirmation(confirmed) {
  const resolve = resetConfirmationResolve;
  resetConfirmationResolve = null;
  if (resetConfirmationDialog.open) resetConfirmationDialog.close();
  resetConfirmationDialog.hidden = true;
  if (resolve) resolve(confirmed);
}

function confirmCommandStateReset(message) {
  if (resetConfirmationResolve) return Promise.resolve(false);
  resetConfirmationDialogDescription.textContent = message;
  resetConfirmationDialog.hidden = false;
  resetConfirmationDialog.showModal();
  btnCancelCommandStateReset.focus();
  return new Promise(resolve => { resetConfirmationResolve = resolve; });
}

btnConfirmCommandStateReset.addEventListener('click', () => finishResetConfirmation(true));
btnCancelCommandStateReset.addEventListener('click', () => finishResetConfirmation(false));
resetConfirmationDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  finishResetConfirmation(false);
});

function renderStartupPresentation(status) {
  startupStatus = status && typeof status === 'object' ? status : {};
  const info = startupStatus.serverInfo && typeof startupStatus.serverInfo === 'object'
    ? startupStatus.serverInfo
    : null;
  const startupError = typeof startupStatus.startupError === 'string' ? startupStatus.startupError : '';
  const tunnelError = typeof info?.tunnelError === 'string' ? info.tunnelError : '';
  const fatal = !info && Boolean(startupError);

  btnOpenSession.disabled = fatal;
  btnNewSession.disabled = fatal;
  if (fatal) {
    startStatus.dataset.state = 'failed';
    renderOverseerError(startStatus, startupError, 'startup', { prefix: 'ЗАПУСК НЕ ЗАВЕРШЁН:' });
  } else if (info?.tunnel && info.url) {
    startStatus.dataset.state = 'ready-public';
    startStatus.textContent = `ГОТОВО · ПУБЛИЧНЫЙ И ЛОКАЛЬНЫЙ ДОСТУП${info.localUrl ? ` · ${info.localUrl}` : ''}`;
  } else if (info) {
    const warning = tunnelError || startupError;
    startStatus.dataset.state = warning ? 'warning' : 'ready-local';
    if (warning) {
      renderOverseerError(startStatus, warning, 'public-access',
        { prefix: 'ЛОКАЛЬНЫЙ РЕЖИМ ГОТОВ · ПУБЛИЧНЫЙ ДОСТУП НЕДОСТУПЕН:' });
    } else {
      startStatus.textContent = `ЛОКАЛЬНЫЙ РЕЖИМ ГОТОВ · ${info.localUrl || info.url}`;
    }
  } else {
    startStatus.dataset.state = 'starting';
    startStatus.textContent = 'ЗАПУСК ЛОКАЛЬНОГО СЕРВЕРА…';
  }
}

// ── Server info / connection count ─────────────────────────
desktopAPI.onServerInfo((info) => {
  renderStartupPresentation({ ...(startupStatus || {}), serverInfo: info });
  const publicUrl = info.tunnel && info.url ? info.url : '';
  const localUrl = info.localUrl || (!info.tunnel ? info.url : '');
  const tunnelUnavailable = info.tunnel && !publicUrl;

  serverUrl = publicUrl || localUrl || null;
  serverUrlEl.classList.toggle('server-url-error', Boolean(info.tunnelError) || tunnelUnavailable);

  if (info.tunnelError) {
    serverUrlEl.textContent = localUrl
      ? `NGROK: ОШИБКА · ЛОКАЛЬНО: ${localUrl}`
      : 'NGROK: ОШИБКА';
    serverUrlTitle = localUrl
      ? `${info.tunnelError}\nЛокальная ссылка остаётся доступна (нажмите, чтобы открыть)`
      : info.tunnelError;
  } else if (publicUrl) {
    serverUrlEl.textContent = publicUrl;
    serverUrlTitle = localUrl
      ? `Публичная ссылка (нажмите, чтобы открыть)\nЛокально: ${localUrl}`
      : 'Публичная ссылка (нажмите, чтобы открыть)';
  } else if (localUrl) {
    serverUrlEl.textContent = localUrl;
    serverUrlTitle = tunnelUnavailable
      ? 'Публичная ссылка недоступна; локальная ссылка остаётся доступна'
      : 'Локальная ссылка (нажмите, чтобы открыть)';
  } else {
    serverUrlEl.textContent = '—';
    serverUrlTitle = 'Адрес игроков пока недоступен';
  }
  serverUrlEl.title = serverUrlTitle;
});
desktopAPI.onClientCount((count) => {
  clientCountEl.textContent = count;
});
desktopAPI.onHackState((hack) => {
  state.liveHack = hack;
  renderHackStatus();
});
desktopAPI.onCoordinationState((coordination) => {
  applyCoordinationState(coordination);
  renderCoordination();
  if (!facilityWorkspace.hidden && selectedFacilityDeviceID) renderFacilityPlayFeed();
  if (sceneResetDialog.open) renderSceneResetPending();
  if (state.session) {
    renderTermList();
    renderTreeHeader();
    renderHackStatus();
  }
});
if (typeof desktopAPI.onSessionState === 'function') {
  desktopAPI.onSessionState((event) => {
    const revision = Number(event?.revision);
    if (!event?.session || !Number.isSafeInteger(revision)) return;
    saveStatus.dataset.sessionStateRevision = String(revision);
    updateSessionStateEvidenceDescription();
    if (!state.session || revision <= newestDurableRevision) return;
    applyLiveSession(event.session, revision);
    renderAll();
  });
}
void desktopAPI.getRuntimeStatus().then(renderStartupPresentation);

// ── Application update: nonblocking status and explicit offer ──
const applicationUpdateStatusLabels = Object.freeze({
  checking: 'ПРОВЕРКА ОБНОВЛЕНИЙ…',
  available: 'ДОСТУПНО ОБНОВЛЕНИЕ ПРИЛОЖЕНИЯ',
  deferred: 'ОБНОВЛЕНИЕ ОТЛОЖЕНО ДО СЛЕДУЮЩЕГО ЗАПУСКА',
  downloading: 'ЗАГРУЗКА ОБНОВЛЕНИЯ…',
  verifying: 'ПРОВЕРКА ЗАГРУЖЕННОГО ОБНОВЛЕНИЯ…',
  staging: 'ПОДГОТОВКА ОБНОВЛЕНИЯ…',
  'ready-to-restart': 'ОБНОВЛЕНИЕ ГОТОВО К ПЕРЕЗАПУСКУ',
  applying: 'ПРИМЕНЕНИЕ ОБНОВЛЕНИЯ…',
  failed: 'НЕ УДАЛОСЬ ПОДГОТОВИТЬ ОБНОВЛЕНИЕ',
});
const applicationUpdateFailureStageLabels = Object.freeze({
  check: 'ПРОВЕРКА ОБНОВЛЕНИЙ',
  download: 'ЗАГРУЗКА ОБНОВЛЕНИЯ',
  verify: 'ПРОВЕРКА ЗАГРУЖЕННОГО ОБНОВЛЕНИЯ',
  stage: 'ПОДГОТОВКА ОБНОВЛЕНИЯ',
  apply: 'ПРИМЕНЕНИЕ ОБНОВЛЕНИЯ',
  relaunch: 'ПЕРЕЗАПУСК ОБНОВЛЁННОГО ПРИЛОЖЕНИЯ',
  recovery: 'ВОССТАНОВЛЕНИЕ РАБОЧЕЙ ВЕРСИИ',
});
const MAX_APPLICATION_UPDATE_TEXT = 16_384;

function boundedApplicationUpdateText(value, fallback = '') {
  if (typeof value !== 'string') return fallback;
  if (value.length <= MAX_APPLICATION_UPDATE_TEXT) return value;
  return `${value.slice(0, MAX_APPLICATION_UPDATE_TEXT)}\n\n[Описание выпуска сокращено]`;
}

function applicationUpdatePromptKey(snapshot) {
  return `${snapshot.attemptId}:${snapshot.revision}`;
}

// renderApplicationUpdateFailure names the failed stage; the failure and
// recovery texts from Go go under the developer disclosure.
function renderApplicationUpdateFailure(snapshot) {
  const stage = applicationUpdateFailureStageLabels[snapshot.failedStage]
    || 'ОБНОВЛЕНИЕ ПРИЛОЖЕНИЯ';
  const original = [snapshot.errorMessage, snapshot.recoveryAction]
    .map(value => boundedApplicationUpdateText(value))
    .filter(Boolean)
    .join('\n');
  renderOverseerError(applicationUpdateError, original, 'update', { prefix: `ЭТАП: ${stage}.` });
}

function restoreApplicationUpdateFocus() {
  const opener = applicationUpdateDialogOpener;
  applicationUpdateDialogOpener = null;
  if (opener?.isConnected && typeof opener.focus === 'function') opener.focus();
}

function closeApplicationUpdateDialog() {
  applicationUpdateDialogAttemptID = '';
  if (applicationUpdateDialog.open) applicationUpdateDialog.close();
  applicationUpdateDialog.hidden = true;
  queueMicrotask(restoreApplicationUpdateFocus);
}

function restoreApplicationUpdateRestartFocus() {
  const opener = applicationUpdateRestartDialogOpener;
  applicationUpdateRestartDialogOpener = null;
  if (opener?.isConnected && typeof opener.focus === 'function') opener.focus();
}

function closeApplicationUpdateRestartDialog() {
  applicationUpdateRestartDialogAttemptID = '';
  if (applicationUpdateRestartDialog.open) applicationUpdateRestartDialog.close();
  applicationUpdateRestartDialog.hidden = true;
  queueMicrotask(restoreApplicationUpdateRestartFocus);
}

function renderApplicationUpdateOffer(snapshot) {
  applicationUpdateInstalledVersion.textContent = boundedApplicationUpdateText(snapshot.installedVersion, '—');
  applicationUpdateAvailableVersion.textContent = boundedApplicationUpdateText(snapshot.availableVersion, '—');
  applicationUpdateReleaseNotes.textContent = boundedApplicationUpdateText(
    snapshot.releaseNotes,
    'Для этого выпуска описание не предоставлено.',
  );
}

function showApplicationUpdateOffer(snapshot, { automatic = false } = {}) {
  if (!snapshot?.attemptId || snapshot.state !== 'available') return;
  if (suppressedApplicationUpdateAttempts.has(snapshot.attemptId)) return;

  renderApplicationUpdateOffer(snapshot);
  if (applicationUpdateDialog.open) {
    applicationUpdateDialogAttemptID = snapshot.attemptId;
    return;
  }
  if (automatic && document.querySelector('dialog[open]')) {
    btnShowApplicationUpdate.hidden = false;
    return;
  }

  const key = applicationUpdatePromptKey(snapshot);
  if (automatic && promptedApplicationUpdateRevisions.has(key)) return;
  promptedApplicationUpdateRevisions.add(key);
  applicationUpdateDialogAttemptID = snapshot.attemptId;
  applicationUpdateDialogOpener = document.activeElement;
  applicationUpdateDialog.hidden = false;
  applicationUpdateDialog.showModal();
  btnShowApplicationUpdate.hidden = true;
  btnDeferApplicationUpdate.focus();
}

function showApplicationUpdateRestart(snapshot, { automatic = false } = {}) {
  if (!snapshot?.attemptId || snapshot.state !== 'ready-to-restart') return;
  if (applicationUpdateRestartDialog.open) {
    applicationUpdateRestartDialogAttemptID = snapshot.attemptId;
    return;
  }
  if (automatic && document.querySelector('dialog[open]')) {
    btnShowApplicationUpdate.hidden = false;
    return;
  }

  const key = applicationUpdatePromptKey(snapshot);
  if (automatic && promptedApplicationUpdateRestartRevisions.has(key)) return;
  promptedApplicationUpdateRestartRevisions.add(key);
  applicationUpdateRestartDialogAttemptID = snapshot.attemptId;
  applicationUpdateRestartDialogOpener = document.activeElement;
  applicationUpdateRestartDialog.hidden = false;
  applicationUpdateRestartDialog.showModal();
  btnShowApplicationUpdate.hidden = true;
  btnPostponeApplicationUpdate.focus();
}

function renderApplicationUpdateProgress(snapshot, stateName) {
  const progressVisible = stateName === 'downloading'
    || stateName === 'verifying'
    || stateName === 'staging';
  applicationUpdateProgress.hidden = !progressVisible;
  applicationUpdateProgress.removeAttribute('max');
  applicationUpdateProgress.removeAttribute('value');
  if (!progressVisible) return;

  if (stateName === 'downloading'
    && Number.isSafeInteger(snapshot.downloadSize)
    && snapshot.downloadSize > 0) {
    const downloaded = Number.isSafeInteger(snapshot.bytesDownloaded)
      ? Math.max(0, Math.min(snapshot.bytesDownloaded, snapshot.downloadSize))
      : 0;
    applicationUpdateProgress.max = snapshot.downloadSize;
    applicationUpdateProgress.value = downloaded;
    applicationUpdateProgress.setAttribute(
      'aria-label',
      `Загрузка обновления: ${downloaded} из ${snapshot.downloadSize} байт`,
    );
    return;
  }

  applicationUpdateProgress.setAttribute(
    'aria-label',
    stateName === 'verifying'
      ? 'Проверка загруженного обновления'
      : stateName === 'staging'
        ? 'Подготовка обновления к перезапуску'
        : 'Загрузка обновления',
  );
}

function renderApplicationUpdateSnapshot(snapshot, { prompt = true } = {}) {
  if (!snapshot || typeof snapshot !== 'object') return;
  const revision = Number(snapshot.revision);
  if (!Number.isSafeInteger(revision) || revision < 0) return;
  if (revision < latestRenderedApplicationUpdateRevision) return;
  if (revision === latestRenderedApplicationUpdateRevision
    && applicationUpdateSnapshot
    && (snapshot.attemptId !== applicationUpdateSnapshot.attemptId
      || snapshot.state !== applicationUpdateSnapshot.state)) return;
  latestRenderedApplicationUpdateRevision = revision;
  applicationUpdateSnapshot = snapshot;
  const stateName = typeof snapshot.state === 'string' ? snapshot.state : '';
  const silent = stateName === '' || stateName === 'disabled' || stateName === 'idle' || stateName === 'current';
  const attemptSuppressed = suppressedApplicationUpdateAttempts.has(snapshot.attemptId);

  applicationUpdateStatusPanel.hidden = silent;
  applicationUpdateStatusPanel.dataset.state = stateName;
  applicationUpdateStatus.textContent = applicationUpdateStatusLabels[stateName] || '';
  renderApplicationUpdateProgress(snapshot, stateName);
  if (stateName === 'failed') renderApplicationUpdateFailure(snapshot);
  else renderOverseerError(applicationUpdateError, null, 'update');
  const offerCanOpen = stateName === 'available' && !attemptSuppressed;
  const restartCanOpen = stateName === 'ready-to-restart';
  btnShowApplicationUpdate.hidden = (!offerCanOpen && !restartCanOpen)
    || applicationUpdateDialog.open
    || applicationUpdateRestartDialog.open;
  btnShowApplicationUpdate.setAttribute(
    'aria-controls',
    restartCanOpen ? 'applicationUpdateRestartDialog' : 'applicationUpdateDialog',
  );

  if (!offerCanOpen && applicationUpdateDialog.open) closeApplicationUpdateDialog();
  if (!restartCanOpen && applicationUpdateRestartDialog.open) closeApplicationUpdateRestartDialog();

  if (offerCanOpen) {
    renderApplicationUpdateOffer(snapshot);
    if (prompt) showApplicationUpdateOffer(snapshot, { automatic: true });
  } else if (restartCanOpen && prompt) {
    showApplicationUpdateRestart(snapshot, { automatic: true });
  }
}

function setApplicationUpdateDecisionPending(pending) {
  applicationUpdateCommandPending = pending;
  btnAcceptApplicationUpdate.disabled = pending;
  btnDeferApplicationUpdate.disabled = pending;
  applicationUpdateDialog.setAttribute('aria-busy', String(pending));
}

async function resolveApplicationUpdateOffer(decision) {
  if (applicationUpdateCommandPending) return;
  const snapshot = applicationUpdateSnapshot;
  if (!snapshot?.attemptId || snapshot.attemptId !== applicationUpdateDialogAttemptID) return;

  setApplicationUpdateDecisionPending(true);
  const result = await desktopAPI.resolveApplicationUpdateOffer({
    attemptId: snapshot.attemptId,
    decision,
  });
  setApplicationUpdateDecisionPending(false);

  if (result?.ok !== true) {
    renderApplicationUpdateSnapshot(result?.snapshot || snapshot, { prompt: false });
    renderOverseerError(applicationUpdateError, result, 'update');
    btnDeferApplicationUpdate.focus();
    return;
  }

  suppressedApplicationUpdateAttempts.add(snapshot.attemptId);
  closeApplicationUpdateDialog();
  const resultSnapshot = result?.snapshot;
  if (resultSnapshot
    && (resultSnapshot.revision > snapshot.revision
      || resultSnapshot.state !== snapshot.state)) {
    renderApplicationUpdateSnapshot(resultSnapshot, { prompt: false });
    return;
  }
  if (decision === 'defer') {
    renderApplicationUpdateSnapshot({ ...snapshot, state: 'deferred' }, { prompt: false });
    return;
  }
  applicationUpdateStatus.textContent = 'ПОДГОТОВКА ОБНОВЛЕНИЯ ЗАПРОШЕНА…';
}

function setApplicationUpdateRestartPending(pending) {
  applicationUpdateRestartCommandPending = pending;
  btnRestartApplicationUpdate.disabled = pending;
  btnPostponeApplicationUpdate.disabled = pending;
  applicationUpdateRestartDialog.setAttribute('aria-busy', String(pending));
}

async function resolveApplicationUpdateRestart(decision) {
  if (applicationUpdateRestartCommandPending) return;
  const snapshot = applicationUpdateSnapshot;
  if (!snapshot?.attemptId || snapshot.attemptId !== applicationUpdateRestartDialogAttemptID) return;

  setApplicationUpdateRestartPending(true);
  const result = await desktopAPI.resolveApplicationUpdateRestart({
    attemptId: snapshot.attemptId,
    decision,
  });
  setApplicationUpdateRestartPending(false);

  if (result?.ok !== true) {
    renderApplicationUpdateSnapshot(result?.snapshot || snapshot, { prompt: false });
    renderOverseerError(applicationUpdateError, result, 'update');
    btnPostponeApplicationUpdate.focus();
    return;
  }

  closeApplicationUpdateRestartDialog();
  renderApplicationUpdateSnapshot(result?.snapshot || snapshot, { prompt: false });
}

btnShowApplicationUpdate.addEventListener('click', () => {
  if (applicationUpdateSnapshot?.state === 'ready-to-restart') {
    showApplicationUpdateRestart(applicationUpdateSnapshot);
    return;
  }
  showApplicationUpdateOffer(applicationUpdateSnapshot);
});
btnAcceptApplicationUpdate.addEventListener('click', () => {
  void resolveApplicationUpdateOffer('accept');
});
btnDeferApplicationUpdate.addEventListener('click', () => {
  void resolveApplicationUpdateOffer('defer');
});
applicationUpdateDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  void resolveApplicationUpdateOffer('defer');
});
applicationUpdateDialog.addEventListener('close', () => {
  applicationUpdateDialog.hidden = true;
});
btnRestartApplicationUpdate.addEventListener('click', () => {
  void resolveApplicationUpdateRestart('restart');
});
btnPostponeApplicationUpdate.addEventListener('click', () => {
  void resolveApplicationUpdateRestart('postpone');
});
applicationUpdateRestartDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  void resolveApplicationUpdateRestart('postpone');
});
applicationUpdateRestartDialog.addEventListener('close', () => {
  applicationUpdateRestartDialog.hidden = true;
});
if (typeof desktopAPI.onApplicationUpdateStatus === 'function') {
  desktopAPI.onApplicationUpdateStatus(renderApplicationUpdateSnapshot);
}

serverUrlEl.addEventListener('click', async () => {
  const requestedUrl = serverUrl;
  if (!requestedUrl) return;

  // The frontend never opens a URL directly. The Go command parses the final
  // value and rejects malformed or non-HTTP(S) protocols at the privilege edge.
  const result = await desktopAPI.openUrl(requestedUrl);
  if (requestedUrl === serverUrl && result && result.ok === false) {
    serverUrlEl.title = `${serverUrlTitle}\n${overseerErrorMessage(result, 'links').message}`;
  }
});

// ── Public access: trusted settings and explicit lifecycle ──
const publicAccessStateLabels = Object.freeze({
  stopped: 'ОСТАНОВЛЕН',
  starting: 'ЗАПУСК…',
  ready: 'ГОТОВ',
  stopping: 'ОСТАНОВКА…',
  error: 'ОШИБКА',
});

const secureCredentialStoreFailures = Object.freeze({
  secret_store_locked: 'Unlock the secure credential store and try again.',
  secret_store_denied: 'Allow secure credential store access and try again.',
  secret_store_unavailable: 'The secure credential store is unavailable; local access remains available.',
});

function publicAccessFailureMessage(status) {
  return secureCredentialStoreFailures[status.errorCategory] || status.errorMessage || '';
}

function renderSecretPresence(element, presence, presentLabel = 'СОХРАНЕН') {
  element.dataset.presence = presence;
  element.textContent = presence === 'present'
    ? presentLabel
    : presence === 'absent' ? 'НЕ СОХРАНЕН' : 'НЕДОСТУПЕН';
}

function publicAccessDisplayURL(preferences, status) {
  if (status.state === 'ready' && status.publicUrl) return status.publicUrl;
  if (status.state === 'error') return '';
  const reservedDomain = String(preferences.reservedDomain || '').trim();
  if (!reservedDomain) return '';
  return /^https?:\/\//i.test(reservedDomain) ? reservedDomain : `https://${reservedDomain}`;
}

function syncPublicAccessPlayerCredentialControls() {
  const configured = publicAccessSnapshot?.playerPasswordPresence === 'present';
  const transitioning = publicAccessSnapshot?.status?.state === 'starting' ||
    publicAccessSnapshot?.status?.state === 'stopping';
  const disabled = publicAccessCommandPending || transitioning;
  const username = publicAccessReplacementUsername.value.trim();
  const password = publicAccessReplacementPlayerPassword.value;
  const passwordValid = password === '' || Array.from(password).length >= 8;
  publicAccessReplacementUsername.disabled = disabled;
  publicAccessReplacementPlayerPassword.disabled = disabled;
  btnGeneratePlayerPassword.disabled = disabled;
  btnCancelPublicAccessPlayerCredentials.disabled = publicAccessCommandPending;
  btnDeletePublicAccessPlayerCredentials.hidden = !configured;
  btnDeletePublicAccessPlayerCredentials.disabled = disabled || !configured;
  btnSavePublicAccessPlayerCredentials.disabled = disabled || username === '' ||
    !passwordValid || (!configured && password === '');
  publicAccessReplacementPlayerPasswordHint.textContent = configured
    ? 'Оставьте пустым, чтобы сохранить текущий пароль.'
    : 'Введите пароль не короче 8 символов или сгенерируйте новый.';
}

function renderPublicAccess(snapshot) {
  if (!snapshot || typeof snapshot !== 'object') return;
  if (publicAccessSnapshot) {
    const candidateGeneration = Number(snapshot.status?.generation || 0);
    const currentGeneration = Number(publicAccessSnapshot.status?.generation || 0);
    const candidateRevision = Number(snapshot.status?.settingsRevision || snapshot.preferences?.revision || 0);
    const currentRevision = Number(publicAccessSnapshot.status?.settingsRevision || publicAccessSnapshot.preferences?.revision || 0);
    if (candidateGeneration < currentGeneration ||
      (candidateGeneration === currentGeneration && candidateRevision < currentRevision)) return;
  }
  publicAccessSection.hidden = false;
  publicAccessSnapshot = snapshot;
  const preferences = snapshot.preferences || {};
  const status = snapshot.status || {};
  publicAccessDomain.value = preferences.reservedDomain || '';
  publicAccessUsernameSummary.textContent = preferences.username || 'players';
  const providerTokenConfigured = snapshot.providerTokenPresence === 'present';
  const playerPasswordConfigured = snapshot.playerPasswordPresence === 'present';
  publicAccessProviderSetup.hidden = providerTokenConfigured;
  publicAccessProviderConfigured.hidden = !providerTokenConfigured;
  if (providerTokenConfigured) publicAccessProviderToken.value = '';
  renderSecretPresence(publicAccessProviderPresence, snapshot.providerTokenPresence, 'НАСТРОЕН');
  renderSecretPresence(publicAccessPasswordPresence, snapshot.playerPasswordPresence);
  publicAccessPasswordPresence.hidden = playerPasswordConfigured;
  publicAccessPasswordMask.hidden = !playerPasswordConfigured;
  publicAccessPasswordMask.textContent = playerPasswordConfigured ? '*****' : '';
  publicAccessStatus.textContent = publicAccessStateLabels[status.state] || 'ЗАГРУЗКА…';
  publicAccessStatus.dataset.state = status.state || 'loading';
  publicAccessStateRow.dataset.state = status.state || 'loading';
  publicAccessStatus.dataset.generation = String(Number(status.generation || 0));
  publicAccessStatus.dataset.settingsRevision = String(Number(status.settingsRevision || preferences.revision || 0));
  renderOverseerError(publicAccessError, status.state === 'error' ? publicAccessFailureMessage(status) : null,
    'public-access', { prefix: 'ЛОКАЛЬНЫЙ РЕЖИМ ПРОДОЛЖАЕТ РАБОТАТЬ' });
  const displayURL = publicAccessDisplayURL(preferences, status);
  publicAccessURL.textContent = displayURL || 'ПОЯВИТСЯ ПОСЛЕ ЗАПУСКА';
  publicAccessURL.dataset.available = String(Boolean(displayURL));
  btnCopyPublicURL.hidden = !displayURL;
  const transitioning = status.state === 'starting' || status.state === 'stopping';
  const disabled = publicAccessCommandPending || transitioning;
  for (const control of [
    publicAccessDomain, publicAccessProviderToken, publicAccessReplacementProviderToken,
    btnSavePublicAccess,
    btnOpenPublicAccessProviderToken, btnSavePublicAccessProviderToken,
    btnDeletePublicAccessProviderToken, btnOpenPublicAccessPlayerCredentials,
  ]) control.disabled = disabled;
  const stopping = status.state === 'ready' || status.state === 'stopping';
  btnStartPublicAccess.hidden = stopping;
  btnStopPublicAccess.hidden = !stopping;
  btnStartPublicAccess.textContent = status.state === 'starting' ? 'ЗАПУСК…' : 'ВКЛЮЧИТЬ ДОСТУП';
  btnStopPublicAccess.textContent = status.state === 'stopping' ? 'ОСТАНОВКА…' : 'ОСТАНОВИТЬ ДОСТУП';
  btnStartPublicAccess.disabled = disabled || status.state === 'ready';
  btnStopPublicAccess.disabled = disabled || status.state === 'stopped';
  btnOpenPublicAccessSettings.disabled = disabled;
  btnClosePublicAccessSettings.disabled = publicAccessCommandPending;
  btnCancelPublicAccessSettings.disabled = publicAccessCommandPending;
  btnCancelPublicAccessProviderToken.disabled = publicAccessCommandPending;
  btnSavePublicAccessProviderToken.disabled = disabled || publicAccessReplacementProviderToken.value.trim() === '';
  btnSharePublicAccessCredentials.disabled = disabled || !playerPasswordConfigured ||
    publicAccessUsernameSummary.textContent.trim() === '';
  syncPublicAccessPlayerCredentialControls();
}

function publicAccessRevision() {
  return Number(publicAccessSnapshot?.preferences?.revision || 0);
}

async function copyTransientText(value, successMessage, statusElement = publicAccessCopyStatus) {
  if (!value) return false;
  let copied = false;
  try {
    if (typeof navigator.clipboard?.writeText === 'function') {
      await navigator.clipboard.writeText(value);
      copied = true;
    }
  } catch {
    // Packaged WebViews may deny the browser Clipboard API. The native Wails
    // runtime is the bounded fallback and returns no copy of the value.
  }
  if (!copied) {
    copied = await desktopAPI.writeClipboardText(value);
  }
  if (copied) {
    statusElement.textContent = successMessage;
    return true;
  }
  statusElement.textContent = 'НЕ УДАЛОСЬ СКОПИРОВАТЬ';
  return false;
}

function showPublicAccessSettings({ setupRequired = false } = {}) {
  if (publicAccessCommandPending) return;
  publicAccessSettingsDialogOpener = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : btnOpenPublicAccessSettings;
  publicAccessSetupRequired.hidden = !setupRequired;
  if (setupRequired) publicAccessGuide.open = true;
  publicAccessSettingsError.textContent = '';
  publicAccessSettingsError.hidden = true;
  setOverseerSinkText(publicAccessSettingsCopyStatus, '');
  publicAccessSettingsDialog.hidden = false;
  if (typeof publicAccessSettingsDialog.showModal === 'function' && !publicAccessSettingsDialog.open) {
    publicAccessSettingsDialog.showModal();
  } else {
    publicAccessSettingsDialog.setAttribute('open', '');
  }
  queueMicrotask(() => btnClosePublicAccessSettings.focus());
}

function hidePublicAccessSettings() {
  if (publicAccessCommandPending) return;
  if (typeof publicAccessSettingsDialog.close === 'function' && publicAccessSettingsDialog.open) {
    publicAccessSettingsDialog.close();
  } else {
    publicAccessSettingsDialog.removeAttribute('open');
  }
  publicAccessSettingsDialog.hidden = true;
  publicAccessSetupRequired.hidden = true;
  publicAccessSettingsError.textContent = '';
  publicAccessSettingsError.hidden = true;
  setOverseerSinkText(publicAccessSettingsCopyStatus, '');
  publicAccessProviderToken.value = '';
  renderPublicAccess(publicAccessSnapshot);
  const opener = publicAccessSettingsDialogOpener;
  publicAccessSettingsDialogOpener = null;
  if (opener?.isConnected) opener.focus();
}

function publicAccessNonSecretDraft() {
  return {
    reservedDomain: publicAccessDomain.value,
  };
}

function restorePublicAccessNonSecretDraft(draft) {
  publicAccessDomain.value = draft.reservedDomain;
}

function confirmActivePublicAccessChange() {
  return publicAccessSnapshot?.status?.state !== 'ready' || window.confirm(
    'ПУБЛИЧНЫЙ ДОСТУП АКТИВЕН. ИЗМЕНЕНИЕ ОСТАНОВИТ И ПЕРЕЗАПУСТИТ ССЫЛКУ. ПРОДОЛЖИТЬ?',
  );
}

function showPublicAccessProviderTokenDialog() {
  if (publicAccessCommandPending || publicAccessSnapshot?.providerTokenPresence !== 'present') return;
  publicAccessProviderTokenDialogOpener = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : btnOpenPublicAccessProviderToken;
  publicAccessReplacementProviderToken.value = '';
  publicAccessProviderTokenError.textContent = '';
  publicAccessProviderTokenError.hidden = true;
  publicAccessProviderTokenDialog.hidden = false;
  if (typeof publicAccessProviderTokenDialog.showModal === 'function' && !publicAccessProviderTokenDialog.open) {
    publicAccessProviderTokenDialog.showModal();
  } else {
    publicAccessProviderTokenDialog.setAttribute('open', '');
  }
  queueMicrotask(() => publicAccessReplacementProviderToken.focus());
}

function hidePublicAccessProviderTokenDialog() {
  if (publicAccessCommandPending) return;
  if (typeof publicAccessProviderTokenDialog.close === 'function' && publicAccessProviderTokenDialog.open) {
    publicAccessProviderTokenDialog.close();
  } else {
    publicAccessProviderTokenDialog.removeAttribute('open');
  }
  publicAccessProviderTokenDialog.hidden = true;
  publicAccessReplacementProviderToken.value = '';
  publicAccessProviderTokenError.textContent = '';
  publicAccessProviderTokenError.hidden = true;
  const opener = publicAccessProviderTokenDialogOpener;
  publicAccessProviderTokenDialogOpener = null;
  if (opener?.isConnected && !opener.closest('[hidden]')) {
    opener.focus();
  } else {
    publicAccessProviderToken.focus();
  }
}

async function runPublicAccessProviderTokenMutation({ replacementProviderToken = '', deleteProviderToken = false }) {
  if (publicAccessCommandPending || !confirmActivePublicAccessChange()) return;
  const preferences = publicAccessSnapshot?.preferences || {};
  const draft = publicAccessNonSecretDraft();
  const request = {
    expectedRevision: publicAccessRevision(),
    enabledPreference: false,
    reservedDomain: preferences.reservedDomain || '',
    username: preferences.username || 'players',
    replacementProviderToken,
    deleteProviderToken,
    replacementPlayerPassword: '',
    deletePlayerPassword: false,
  };
  publicAccessProviderTokenError.textContent = '';
  publicAccessProviderTokenError.hidden = true;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const pending = desktopAPI.savePublicAccessSettings(request);
  publicAccessReplacementProviderToken.value = '';
  request.replacementProviderToken = '';
  const result = await pending;
  publicAccessCommandPending = false;
  renderPublicAccess(result.snapshot || publicAccessSnapshot);
  restorePublicAccessNonSecretDraft(draft);
  if (!result.ok) {
    renderOverseerError(publicAccessProviderTokenError, result, 'credentials');
    return;
  }
  hidePublicAccessProviderTokenDialog();
}

function showPublicAccessPlayerCredentialsDialog() {
  if (publicAccessCommandPending) return;
  publicAccessPlayerCredentialsDialogOpener = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : btnOpenPublicAccessPlayerCredentials;
  publicAccessReplacementUsername.value = publicAccessSnapshot?.preferences?.username || 'players';
  publicAccessReplacementPlayerPassword.value = '';
  publicAccessPlayerCredentialsError.textContent = '';
  publicAccessPlayerCredentialsError.hidden = true;
  syncPublicAccessPlayerCredentialControls();
  publicAccessPlayerCredentialsDialog.hidden = false;
  if (typeof publicAccessPlayerCredentialsDialog.showModal === 'function' && !publicAccessPlayerCredentialsDialog.open) {
    publicAccessPlayerCredentialsDialog.showModal();
  } else {
    publicAccessPlayerCredentialsDialog.setAttribute('open', '');
  }
  queueMicrotask(() => publicAccessReplacementUsername.focus());
}

function hidePublicAccessPlayerCredentialsDialog() {
  if (publicAccessCommandPending) return;
  if (typeof publicAccessPlayerCredentialsDialog.close === 'function' && publicAccessPlayerCredentialsDialog.open) {
    publicAccessPlayerCredentialsDialog.close();
  } else {
    publicAccessPlayerCredentialsDialog.removeAttribute('open');
  }
  publicAccessPlayerCredentialsDialog.hidden = true;
  publicAccessReplacementPlayerPassword.value = '';
  publicAccessPlayerCredentialsError.textContent = '';
  publicAccessPlayerCredentialsError.hidden = true;
  const opener = publicAccessPlayerCredentialsDialogOpener;
  publicAccessPlayerCredentialsDialogOpener = null;
  if (opener?.isConnected && !opener.closest('[hidden]')) {
    opener.focus();
  } else {
    btnOpenPublicAccessPlayerCredentials.focus();
  }
}

async function runPublicAccessPlayerCredentialMutation({
  username,
  replacementPlayerPassword = '',
  deletePlayerPassword = false,
}) {
  if (publicAccessCommandPending || !confirmActivePublicAccessChange()) return;
  const preferences = publicAccessSnapshot?.preferences || {};
  const mainDraft = publicAccessNonSecretDraft();
  const request = {
    expectedRevision: publicAccessRevision(),
    enabledPreference: false,
    reservedDomain: preferences.reservedDomain || '',
    username,
    replacementProviderToken: '',
    deleteProviderToken: false,
    replacementPlayerPassword,
    deletePlayerPassword,
  };
  publicAccessPlayerCredentialsError.textContent = '';
  publicAccessPlayerCredentialsError.hidden = true;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const pending = desktopAPI.savePublicAccessSettings(request);
  publicAccessReplacementPlayerPassword.value = '';
  request.replacementPlayerPassword = '';
  const result = await pending;
  publicAccessCommandPending = false;
  renderPublicAccess(result.snapshot || publicAccessSnapshot);
  restorePublicAccessNonSecretDraft(mainDraft);
  if (!result.ok) {
    publicAccessReplacementUsername.value = username;
    renderOverseerError(publicAccessPlayerCredentialsError, result, 'credentials');
    syncPublicAccessPlayerCredentialControls();
    return;
  }
  hidePublicAccessPlayerCredentialsDialog();
}

function showGeneratedPassword(oneTimeValue) {
  let transientValue = oneTimeValue;
  generatedPasswordValue.textContent = transientValue;
  const clearAndClose = () => {
    transientValue = '';
    generatedPasswordValue.textContent = '';
    btnCopyGeneratedPassword.onclick = null;
    btnDismissGeneratedPassword.onclick = null;
    generatedPasswordDialog.oncancel = null;
    if (generatedPasswordDialog.open) generatedPasswordDialog.close();
    btnGeneratePlayerPassword.focus();
  };
  btnCopyGeneratedPassword.onclick = async () => {
    await copyTransientText(transientValue, 'ПАРОЛЬ СКОПИРОВАН', publicAccessSettingsCopyStatus);
    clearAndClose();
  };
  btnDismissGeneratedPassword.onclick = clearAndClose;
  generatedPasswordDialog.oncancel = (event) => {
    event.preventDefault();
    clearAndClose();
  };
  generatedPasswordDialog.showModal();
  btnCopyGeneratedPassword.focus();
}

// The facade owns exact `public-access-status` event ordering and stale-snapshot suppression.
desktopAPI.onPublicAccessStatus(renderPublicAccess);

publicAccessForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  if (publicAccessCommandPending) return;
  if (!confirmActivePublicAccessChange()) return;
  const request = {
    expectedRevision: publicAccessRevision(),
    enabledPreference: false,
    reservedDomain: publicAccessDomain.value,
    username: publicAccessSnapshot?.preferences?.username || 'players',
    replacementProviderToken: publicAccessProviderToken.value,
    deleteProviderToken: false,
    replacementPlayerPassword: '',
    deletePlayerPassword: false,
  };
  publicAccessSettingsError.textContent = '';
  publicAccessSettingsError.hidden = true;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const pending = desktopAPI.savePublicAccessSettings(request);
  publicAccessProviderToken.value = '';
  request.replacementProviderToken = '';
  const result = await pending;
  publicAccessCommandPending = false;
  renderPublicAccess(result.snapshot || publicAccessSnapshot);
  if (!result.ok) {
    publicAccessDomain.value = request.reservedDomain;
    renderOverseerError(publicAccessSettingsError, result, 'public-access');
    return;
  }
  hidePublicAccessSettings();
});

async function runPublicAccessLifecycle(command) {
  if (publicAccessCommandPending) return;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const result = await command({ expectedRevision: publicAccessRevision() });
  publicAccessCommandPending = false;
  renderPublicAccess(result.snapshot || publicAccessSnapshot);
  if (!result.ok) {
    renderOverseerError(publicAccessError, result, 'public-access');
  }
}

btnStartPublicAccess.addEventListener('click', () => {
  const configured = publicAccessSnapshot?.providerTokenPresence === 'present'
    && publicAccessSnapshot?.playerPasswordPresence === 'present';
  if (!configured) {
    showPublicAccessSettings({ setupRequired: true });
    return;
  }
  void runPublicAccessLifecycle(desktopAPI.startPublicAccess);
});
btnStopPublicAccess.addEventListener('click', () => runPublicAccessLifecycle(desktopAPI.stopPublicAccess));
btnOpenPublicAccessSettings.addEventListener('click', () => showPublicAccessSettings());
btnClosePublicAccessSettings.addEventListener('click', hidePublicAccessSettings);
btnCancelPublicAccessSettings.addEventListener('click', hidePublicAccessSettings);
publicAccessSettingsDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  hidePublicAccessSettings();
});
btnOpenPublicAccessProviderToken.addEventListener('click', showPublicAccessProviderTokenDialog);
btnCancelPublicAccessProviderToken.addEventListener('click', hidePublicAccessProviderTokenDialog);
publicAccessProviderTokenDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  hidePublicAccessProviderTokenDialog();
});
publicAccessProviderTokenForm.addEventListener('submit', (event) => {
  event.preventDefault();
  const replacementProviderToken = publicAccessReplacementProviderToken.value;
  if (!replacementProviderToken.trim()) return;
  void runPublicAccessProviderTokenMutation({ replacementProviderToken });
});
btnDeletePublicAccessProviderToken.addEventListener('click', () => {
  void runPublicAccessProviderTokenMutation({ deleteProviderToken: true });
});
publicAccessReplacementProviderToken.addEventListener('input', () => {
  btnSavePublicAccessProviderToken.disabled = publicAccessCommandPending ||
    publicAccessReplacementProviderToken.value.trim() === '';
});
btnOpenPublicAccessPlayerCredentials.addEventListener('click', showPublicAccessPlayerCredentialsDialog);
btnCancelPublicAccessPlayerCredentials.addEventListener('click', hidePublicAccessPlayerCredentialsDialog);
publicAccessPlayerCredentialsDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  hidePublicAccessPlayerCredentialsDialog();
});
publicAccessPlayerCredentialsForm.addEventListener('submit', (event) => {
  event.preventDefault();
  const username = publicAccessReplacementUsername.value.trim();
  const replacementPlayerPassword = publicAccessReplacementPlayerPassword.value;
  const configured = publicAccessSnapshot?.playerPasswordPresence === 'present';
  if (!username) return;
  if ((!configured && replacementPlayerPassword === '') ||
    (replacementPlayerPassword !== '' && Array.from(replacementPlayerPassword).length < 8)) {
    publicAccessPlayerCredentialsError.textContent = 'ПАРОЛЬ ДОЛЖЕН СОДЕРЖАТЬ НЕ МЕНЕЕ 8 СИМВОЛОВ';
    publicAccessPlayerCredentialsError.hidden = false;
    return;
  }
  void runPublicAccessPlayerCredentialMutation({ username, replacementPlayerPassword });
});
btnDeletePublicAccessPlayerCredentials.addEventListener('click', () => {
  void runPublicAccessPlayerCredentialMutation({ username: 'players', deletePlayerPassword: true });
});
publicAccessReplacementUsername.addEventListener('input', syncPublicAccessPlayerCredentialControls);
publicAccessReplacementPlayerPassword.addEventListener('input', syncPublicAccessPlayerCredentialControls);
btnGeneratePlayerPassword.addEventListener('click', async () => {
  if (publicAccessCommandPending) return;
  if (!confirmActivePublicAccessChange()) return;
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const result = await desktopAPI.generatePlayerPassword({ expectedRevision: publicAccessRevision() });
  publicAccessCommandPending = false;
  if (!result.ok || !result.generatedPassword) {
    renderPublicAccess(publicAccessSnapshot);
    renderOverseerError(publicAccessPlayerCredentialsError, result, 'credentials');
    return;
  }
  showGeneratedPassword(result.generatedPassword);
  const refreshed = await desktopAPI.getPublicAccess();
  renderPublicAccess(refreshed);
});
btnCopyPublicURL.addEventListener('click', () => {
  if (publicAccessURL.dataset.available !== 'true') return;
  void copyTransientText(publicAccessURL.textContent, 'URL СКОПИРОВАН');
});
btnSharePublicAccessCredentials.addEventListener('click', async () => {
  if (publicAccessCommandPending || publicAccessSnapshot?.playerPasswordPresence !== 'present') return;
  setOverseerSinkText(publicAccessSettingsCopyStatus, '');
  publicAccessCommandPending = true;
  renderPublicAccess(publicAccessSnapshot);
  const result = await desktopAPI.copyPublicAccessCredentials();
  publicAccessCommandPending = false;
  renderPublicAccess(publicAccessSnapshot);
  if (result.ok) setOverseerSinkText(publicAccessSettingsCopyStatus, 'ЛОГИН И ПАРОЛЬ СКОПИРОВАНЫ');
  else renderOverseerError(publicAccessSettingsCopyStatus, result, 'credentials');
});
for (const link of publicAccessNgrokDocLinks) {
  link.addEventListener('click', () => {
    void desktopAPI.openUrl(link.dataset.ngrokDocUrl);
  });
}

let logAccessPending = false;
const safeLogAccessErrors = new Set([
  'Could not open the application log directory.',
  'The application log directory is unavailable.',
]);

function renderLogAccessStatus(status, result) {
  const value = result && typeof result === 'object' ? result : {};
  const directoryPath = typeof value.directoryPath === 'string' ? value.directoryPath : '';
  const activeLogPath = typeof value.activeLogPath === 'string' ? value.activeLogPath : '';
  // An absent active file means retained output is degraded and records reach only standard error.
  const retention = activeLogPath ? '' : 'ЗАПИСЬ ЛОГОВ В ФАЙЛ НЕДОСТУПНА';
  if (value.ok !== true) {
    renderOverseerError(status, safeLogAccessErrors.has(value.error) ? value.error : '', 'logs');
    // A failed open names the directory so the Overseer can navigate there manually.
    const location = [directoryPath, retention].filter(Boolean).join(' · ');
    if (location) {
      const line = document.createElement('p');
      line.className = 'log-access-location';
      line.textContent = location;
      status.querySelector('.overseer-error-message').after(line);
    }
    return;
  }
  setOverseerSinkText(status, ['ЛОГИ ОТКРЫТЫ', activeLogPath || directoryPath, retention].filter(Boolean).join(' · '));
}

for (const button of logLocationButtons) {
  button.addEventListener('click', async () => {
    if (logAccessPending) return;
    logAccessPending = true;
    logLocationButtons.forEach(control => { control.disabled = true; });
    logAccessStatuses.forEach(status => setOverseerSinkText(status, 'ОТКРЫТИЕ ПАПКИ ЛОГОВ…'));
    try {
      const result = await desktopAPI.openLogLocation();
      logAccessStatuses.forEach(status => renderLogAccessStatus(status, result));
    } finally {
      logLocationButtons.forEach(control => { control.disabled = false; });
      logAccessPending = false;
    }
  });
}
// ── Start screen: open / new session ───────────────────────
btnOpenSession.addEventListener('click', async () => {
  const res = await desktopAPI.openSession();
  if (!res.ok) {
    if (res.error) renderOverseerError(startStatus, res, 'session');
    return;
  }
  await loadSession(res.session, res.filePath, res.authoringRevision);
});

btnNewSession.addEventListener('click', async () => {
  const res = await desktopAPI.newSession();
  if (!res.ok) {
    if (res.error) renderOverseerError(startStatus, res, 'session');
    return;
  }
  await loadSession(res.session, res.filePath, res.authoringRevision);
});

// ── What players see (R11) ──────────────────────────────────
// playerView caches the server's explanation of what players see, keyed by
// terminal and node. The newest request wins.
const playerView = { requested: 0, applied: 0, inFlight: 0, nodes: new Map() };

function playerViewNode(terminalID, nodeID) {
  return playerView.nodes.get(`${terminalID}/${nodeID}`) || null;
}

async function refreshPlayerView() {
  if (!state.session || !state.filePath) return;
  const request = ++playerView.requested;
  const generation = authoring.generation;
  playerView.inFlight++;
  renderCommandRules();
  const result = await desktopAPI.describePresentation({ expectedAuthoringRevision: 0 });
  playerView.inFlight--;
  if (result.ok && generation === authoring.generation && request > playerView.applied) {
    playerView.applied = request;
    playerView.nodes = new Map();
    for (const terminal of result.terminals) {
      for (const node of terminal.nodes || []) playerView.nodes.set(`${terminal.terminalId}/${node.nodeId}`, node);
    }
    renderTree();
  }
  renderCommandRules();
}

// ── Command form lists (FR-008–FR-010) ──────────────────────
const PRESENTATION_RULE_LABELS = {
  'authored-name': 'ИСХОДНОЕ НАЗВАНИЕ',
  'name-after-execution': 'НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ',
  'state-name': 'НАЗВАНИЕ ПО СОСТОЯНИЮ',
  'state-visibility': 'ВИДИМОСТЬ ПО СОСТОЯНИЮ',
  'diagnostic-forced-visibility': 'ВИДИМОСТЬ ПРИ НЕИСПРАВНОСТИ',
  'state-availability': 'ДОСТУПНОСТЬ ПО СОСТОЯНИЮ',
  'diagnostic-capability-block': 'БЛОКИРОВКА НЕИСПРАВНОСТЬЮ',
  'initial-text': 'ИСХОДНЫЙ ТЕКСТ',
  'text-after-execution': 'ТЕКСТ ПОСЛЕ ВЫПОЛНЕНИЯ',
  'state-text': 'ТЕКСТ ПО СОСТОЯНИЮ',
  'diagnostic-substitution': 'ЗАМЕНА ПРИ НЕИСПРАВНОСТИ',
};

const PRESENTATION_RULE_BINDINGS = {
  'state-name': 'menu-name', 'state-visibility': 'visibility',
  'state-availability': 'availability', 'state-text': 'entry-content',
};

function facilityTransitionEffect(request) {
  const device = facilityDeviceByID(request.deviceId);
  const transition = device?.transitions?.find(value => value.id === request.transitionId);
  if (!transition) return `${device?.name || request.deviceId}: переход не найден`;
  return `${device.name}: ${facilityStateName(device, transition.sourceStateId)} → ${facilityStateName(device, transition.destinationStateId)}`;
}

// commandEffectItems lists what approving the authored command changes.
function commandEffectItems(term, node) {
  if (node.terminalTransition) {
    const targetID = node.terminalTransition.targetTerminalId;
    const target = state.session.terminals.find(candidate => candidate.id === targetID);
    return [{ kind: 'terminal-transition', text: `ПЕРЕХОД: ${target?.name || targetID}` }];
  }
  const change = node.stateChange;
  if (!change) return [];
  const items = [];
  const action = change.facilityAction;
  for (const request of action?.transitions?.transitions || []) {
    items.push({ kind: 'transition', text: facilityTransitionEffect(request) });
  }
  if (action?.recoveryProgramId) {
    const program = authoredFacility().recoveryPrograms.find(value => value.id === action.recoveryProgramId);
    items.push({ kind: 'recovery', text: `ПРОГРАММА ВОССТАНОВЛЕНИЯ: ${program?.name || action.recoveryProgramId}` });
    for (const request of program?.transitions || []) items.push({ kind: 'transition', text: facilityTransitionEffect(request) });
  }
  if (change.completedName) items.push({ kind: 'completed-name', text: `НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ: ${change.completedName}` });
  const blockChange = change.entryContentChange;
  if (blockChange?.blockId) {
    const location = entryBlockLocation(term, blockChange.blockId);
    items.push({
      kind: 'entry-block',
      text: `${location?.entry.name || 'Запись'} · блок ${(location?.index ?? 0) + 1}: ${blockChange.completedText}`,
    });
  }
  return items;
}

function presentationConditionText(term, condition) {
  if (condition?.deviceState) {
    const device = facilityDeviceByID(condition.deviceState.deviceId);
    return `КОГДА ${device?.name || condition.deviceState.deviceId} = ${facilityStateName(device, condition.deviceState.stateId)}`;
  }
  if (condition?.commandExecuted) {
    const command = locateNode(term.root, condition.commandExecuted.commandId)?.node;
    return command?.id === state.selectedNodeId ? 'ПОСЛЕ ВЫПОЛНЕНИЯ' : `ПОСЛЕ ВЫПОЛНЕНИЯ «${command?.name || condition.commandExecuted.commandId}»`;
  }
  if (condition?.diagnosticActive) {
    const id = condition.diagnosticActive.conditionId;
    const fault = authoredFacility().conditions.find(value => value.id === id);
    return `КОГДА АКТИВНА «${fault?.name || id}»`;
  }
  return 'ВСЕГДА';
}

function presentationRuleValue(rule) {
  const effects = { visible: 'ПОКАЗАТЬ', available: 'РАЗРЕШИТЬ', unavailable: 'ЗАПРЕТИТЬ' };
  return effects[rule.effect] || rule.text || '';
}

// openPresentationRuleEditor opens where rule is authored: a node form
// field, the display rule dialog, or the fault dialog.
function openPresentationRuleEditor(term, ownerID, blockID, rule) {
  const condition = rule.condition || {};
  if (condition.diagnosticActive) {
    openFacilityConditionDialog(condition.diagnosticActive.conditionId);
    return;
  }
  const property = PRESENTATION_RULE_BINDINGS[rule.kind];
  if (property) {
    const binding = facilityBindingCatalog().find(value => value.terminal.id === term.id
      && value.node.id === ownerID && value.property === property && (value.block?.id || '') === blockID
      && value.variant.when?.deviceId === condition.deviceState?.deviceId
      && value.variant.when?.stateId === condition.deviceState?.stateId
      && (!['menu-name', 'entry-content'].includes(property) || value.variant.text === rule.text));
    if (binding) openFacilityBindingDialog(binding);
    return;
  }
  const fieldNodeID = rule.kind === 'text-after-execution' ? condition.commandExecuted?.commandId : ownerID;
  if (fieldNodeID && fieldNodeID !== state.selectedNodeId) {
    state.selectedNodeId = fieldNodeID;
    renderTree();
    renderNodeForm();
    renderToolbarHint();
  }
  const fields = {
    'authored-name': 'fldName', 'name-after-execution': 'fldCompletedName',
    'text-after-execution': 'fldCompletedEntryBlockContent',
  };
  document.getElementById(fields[rule.kind] || '')?.focus();
}

// renderCommandRules fills #commandRules from the cached explanation for
// the selected command, grouped by target, lowest precedence first.
function renderCommandRules() {
  const section = document.getElementById('commandRules');
  if (!section) return;
  section.setAttribute('aria-busy', String(playerView.inFlight > 0));
  const term = getEditTerminal();
  const node = term && state.selectedNodeId ? locateNode(term.root, state.selectedNodeId)?.node : null;
  const seen = node ? playerViewNode(term.id, node.id) : null;
  const targets = seen ? [
    ['name', node.id, '', 'НАЗВАНИЕ', seen.nameRules],
    ['visibility', node.id, '', 'ВИДИМОСТЬ', seen.visibilityRules],
    ['availability', node.id, '', 'ДОСТУПНОСТЬ', seen.availabilityRules],
  ] : [];
  const blockID = node?.stateChange?.entryContentChange?.blockId;
  const location = blockID ? entryBlockLocation(term, blockID) : null;
  if (location) {
    const block = playerViewNode(term.id, location.entry.id)?.blocks?.find(value => value.blockId === blockID);
    targets.push([`block:${blockID}`, location.entry.id, blockID, `${location.entry.name} · БЛОК ${location.index + 1}`, block?.textRules]);
  }
  const container = section.querySelector('.command-rule-targets');
  const groups = [];
  for (const [target, ownerID, ownerBlockID, heading, rules] of targets) {
    if (!rules?.length) continue;
    const ordered = [...rules].sort((left, right) => left.precedence - right.precedence);
    const group = document.createElement('section');
    group.className = 'command-rule-target';
    group.dataset.target = target;
    const title = document.createElement('div');
    title.className = 'command-rule-target-heading';
    title.textContent = heading;
    const list = document.createElement('ul');
    for (const rule of ordered) {
      const item = document.createElement('li');
      item.className = 'command-rule';
      item.dataset.kind = rule.kind;
      const text = document.createElement('span');
      text.textContent = `${PRESENTATION_RULE_LABELS[rule.kind] || rule.kind} · ${presentationConditionText(term, rule.condition)} · ${presentationRuleValue(rule)}`;
      item.appendChild(text);
      if (rule.playersSee) {
        const mark = document.createElement('span');
        mark.className = 'rule-players-see';
        mark.textContent = 'ВИДЯТ ИГРОКИ';
        item.appendChild(mark);
      }
      const lower = [...new Set(ordered.filter(value => value.precedence < rule.precedence)
        .map(value => PRESENTATION_RULE_LABELS[value.kind] || value.kind))];
      if (rule.conditionHolds && lower.length) {
        const overrides = document.createElement('span');
        overrides.className = 'rule-overrides';
        overrides.textContent = `ПЕРЕКРЫВАЕТ: ${lower.join(', ')}`;
        item.appendChild(overrides);
      }
      const edit = document.createElement('button');
      edit.className = 'btn btn-mini btn-secondary';
      edit.type = 'button';
      edit.textContent = 'ИЗМЕНИТЬ';
      edit.addEventListener('click', () => openPresentationRuleEditor(term, ownerID, ownerBlockID, rule));
      item.appendChild(edit);
      list.appendChild(item);
    }
    group.append(title, list);
    groups.push(group);
  }
  if (!groups.length && playerView.inFlight > 0) return;
  if (!groups.length) {
    const empty = document.createElement('div');
    empty.className = 'command-mode-hint';
    empty.textContent = seen ? 'ИГРОКИ ВИДЯТ ИСХОДНОЕ НАЗВАНИЕ' : 'ПРАВИЛА ПОЯВЯТСЯ ПОСЛЕ СОХРАНЕНИЯ';
    groups.push(empty);
  }
  container.replaceChildren(...groups);
}

async function loadSession(session, filePath, authoringRevision) {
  newestDurableRevision = 0;
  delete saveStatus.dataset.savedRevision;
  state.session        = session;
  resetAuthoring(session, authoringRevision);
  renderSaveStatus();
  resetFacilitySelection();
  state.filePath        = filePath;
  state.liveTerminalId  = state.coordination?.broadcast?.activeTerminalId || null;
  state.editTerminalId  = (session.terminals[0] && session.terminals[0].id) || null;
  state.selectedNodeId  = null;
  state.expanded         = new Set(['root']);
  state.collapsedTerminalGroupIds = new Set();
  state.liveHack         = null;

  sessionFileLabel.textContent = filePath;
  startScreen.style.display = 'none';
  mainLayout.style.display  = 'flex';
  playerView.nodes = new Map();
  renderAll();
  void refreshPlayerView();

  if (session.playerConfig) {
    await runPlayerConfigCommand(
      () => desktopAPI.loadReferencedPlayerConfig(),
      'КОНФИГУРАЦИЯ ИГРОКОВ ЗАГРУЖЕНА'
    );
  } else {
    setPlayerConfigError('ВЫБЕРИТЕ ИЛИ СОЗДАЙТЕ КОНФИГУРАЦИЮ ИГРОКОВ');
  }
}

// ── Authoring: one save model ───────────────────────────────
// Every applied edit is one SaveAuthoring request. Requests go one at a time
// and in order; a rejected edit reverts only itself. The server keeps every
// live value (executed commands, device states, fault activity), so authoring
// never overwrites live play.
const AUTHORING_HISTORY_LIMIT = 20;
const authoring = {
  generation: 0,
  revision: 0,
  lastSaved: null,
  lastSavedAt: null,
  queue: [],
  inFlight: null,
  held: [],
  error: null,
  historyError: null,
  historyTrimmed: false,
  undo: [],
  redo: [],
  nextID: 0,
  idleWaiters: [],
};

function resetAuthoring(session, authoringRevision) {
  authoring.generation++;
  authoring.revision = Number(authoringRevision) || 1;
  authoring.lastSaved = session ? structuredClone(session) : null;
  authoring.lastSavedAt = null;
  authoring.queue = [];
  authoring.inFlight = null;
  authoring.held = [];
  authoring.error = null;
  authoring.historyError = null;
  authoring.historyTrimmed = false;
  authoring.undo = [];
  authoring.redo = [];
  unappliedInputs.clear();
  for (const resolve of authoring.idleWaiters.splice(0)) resolve();
}

function authoringBusy() {
  return Boolean(authoring.inFlight || authoring.queue.length);
}

// authoringIdle resolves once no save is queued or in flight.
function authoringIdle() {
  if (!authoringBusy()) return Promise.resolve();
  return new Promise(resolve => authoring.idleWaiters.push(resolve));
}

// mergeLiveValues copies the server-owned live values of source into target.
function mergeLiveValues(target, source) {
  if (!target || !source) return target;
  const sourceTerminals = new Map((source.terminals || []).map(terminal => [terminal.id, terminal]));
  for (const terminal of target.terminals || []) {
    const states = sourceTerminals.get(terminal.id)?.commandStates;
    if (states && Object.keys(states).length) terminal.commandStates = structuredClone(states);
    else delete terminal.commandStates;
  }
  if (target.facility && source.facility) {
    target.facility.revision = source.facility.revision;
    const devices = new Map((source.facility.devices || []).map(device => [device.id, device]));
    for (const device of target.facility.devices || []) {
      if (devices.has(device.id)) device.currentStateId = devices.get(device.id).currentStateId;
    }
    const conditions = new Map((source.facility.conditions || []).map(condition => [condition.id, condition]));
    for (const condition of target.facility.conditions || []) {
      if (conditions.has(condition.id)) condition.currentActive = conditions.get(condition.id).currentActive;
    }
  }
  return target;
}

// authoredKey is a stable comparison key for the authored configuration: the
// session without live values or terminal groups, with sorted keys and
// without empty values, so local and server documents compare equal.
function authoredKey(session) {
  if (!session) return '';
  const copy = structuredClone(session);
  delete copy.terminalGroups;
  for (const terminal of copy.terminals || []) delete terminal.commandStates;
  if (copy.facility) {
    delete copy.facility.revision;
    for (const device of copy.facility.devices || []) delete device.currentStateId;
    for (const condition of copy.facility.conditions || []) delete condition.currentActive;
  }
  const normalize = value => {
    if (Array.isArray(value)) return value.map(normalize);
    if (!value || typeof value !== 'object') return value;
    const result = {};
    for (const key of Object.keys(value).sort()) {
      const item = normalize(value[key]);
      const empty = item === undefined || item === null || item === ''
        || (Array.isArray(item) && !item.length) || (typeof item === 'object' && !Object.keys(item).length);
      if (!empty) result[key] = item;
    }
    return result;
  };
  return JSON.stringify(normalize(copy));
}

// replaceAuthoredContent swaps the authored configuration of session for that
// of source while keeping session's live values and terminal groups.
function replaceAuthoredContent(session, source) {
  const replacement = mergeLiveValues(structuredClone(source), session);
  replacement.terminalGroups = session.terminalGroups;
  for (const key of Object.keys(session)) delete session[key];
  Object.assign(session, replacement);
}

// applyLiveSession installs a server result that changed live values only.
function applyLiveSession(session, revision = 0) {
  if (!session || !state.session) return;
  mergeLiveValues(state.session, session);
  if (authoring.lastSaved) mergeLiveValues(authoring.lastSaved, session);
  // A live change may make a rejected undo or redo possible again.
  authoring.historyError = null;
  newestDurableRevision = Math.max(newestDurableRevision, Number(revision) || 0);
  saveStatus.dataset.savedRevision = String(newestDurableRevision);
  void refreshPlayerView();
}

// applyAuthoredSession installs a server result that changed the authored
// configuration outside the save pipeline (groups, player configuration).
function applyAuthoredSession(session, authoringRevision, revision = 0) {
  if (!session) return;
  if (Number(authoringRevision) > 0) authoring.revision = Number(authoringRevision);
  authoring.lastSaved = structuredClone(session);
  if (authoringBusy()) mergeLiveValues(state.session, session);
  else state.session = structuredClone(session);
  newestDurableRevision = Math.max(newestDurableRevision, Number(revision) || 0);
  saveStatus.dataset.savedRevision = String(newestDurableRevision);
  void refreshPlayerView();
}

// commitAuthoringEdit applies one edit to the working copy and queues its
// save. mutate(session) must find what it changes by ID so it can run again
// on a newer session; reopen(reason, result) brings the editor back after a
// failure.
function commitAuthoringEdit({ label, scope = 'terminal', mutate, reopen = null, kind = 'edit', entry = null }) {
  if (!state.session || !state.filePath) return false;
  const before = authoredKey(state.session);
  mutate(state.session);
  if (authoredKey(state.session) === before) {
    renderSaveStatus();
    return true;
  }
  if (kind === 'edit') authoring.redo = [];
  authoring.error = null;
  authoring.historyError = null;
  authoring.queue.push({
    id: ++authoring.nextID, label, scope, mutate, reopen, kind, entry,
    candidate: structuredClone(state.session),
  });
  renderSaveStatus();
  void pumpAuthoringQueue();
  return true;
}

async function pumpAuthoringQueue() {
  if (authoring.inFlight) return;
  if (!authoring.queue.length) {
    for (const resolve of authoring.idleWaiters.splice(0)) resolve();
    return;
  }
  const generation = authoring.generation;
  const edit = authoring.queue.shift();
  authoring.inFlight = edit;
  renderSaveStatus();
  const result = await desktopAPI.saveAuthoring({
    session: edit.candidate,
    expectedAuthoringRevision: authoring.revision,
    correlationId: `authoring-${generation}-${edit.id}`,
  });
  if (generation !== authoring.generation) return;
  authoring.inFlight = null;
  if (result.ok) acceptAuthoringSave(edit, result);
  else if (result.failure === 'stale-revision') holdStaleAuthoringEdits(edit, result);
  else rejectAuthoringEdit(edit, result);
  renderAll();
  void pumpAuthoringQueue();
}

function acceptAuthoringSave(edit, result) {
  const previous = authoring.lastSaved;
  authoring.revision = result.authoringRevision;
  authoring.lastSaved = structuredClone(result.session);
  authoring.lastSavedAt = new Date();
  newestDurableRevision = Math.max(newestDurableRevision, result.sessionRevision);
  saveStatus.dataset.savedRevision = String(newestDurableRevision);
  if (result.changed) {
    if (edit.kind === 'edit') {
      authoring.undo.push({ label: edit.label, before: previous, after: authoring.lastSaved });
      if (authoring.undo.length > AUTHORING_HISTORY_LIMIT) {
        authoring.undo.shift();
        authoring.historyTrimmed = true;
      }
    } else if (edit.kind === 'undo') {
      authoring.undo = authoring.undo.filter(entry => entry !== edit.entry);
      authoring.redo.push(edit.entry);
    } else if (edit.kind === 'redo') {
      authoring.redo = authoring.redo.filter(entry => entry !== edit.entry);
      authoring.undo.push(edit.entry);
    }
  }
  if (authoring.queue.length) mergeLiveValues(state.session, result.session);
  else state.session = structuredClone(result.session);
  if (result.changed) void refreshPlayerView();
}

// rebuildWorkingCopy replays the queued edits on the last saved session.
function rebuildWorkingCopy(liveSource) {
  state.session = structuredClone(authoring.lastSaved);
  if (liveSource) mergeLiveValues(state.session, liveSource);
  authoring.queue = authoring.queue.filter(edit => {
    try {
      edit.mutate(state.session);
      edit.candidate = structuredClone(state.session);
      return true;
    } catch {
      return false;
    }
  });
}

function rejectAuthoringEdit(edit, result) {
  rebuildWorkingCopy(result.session);
  // The rebuilt working copy lacks what the edit added, so the message also
  // looks entity names up in the rejected candidate.
  const failure = { ...result, session: edit.candidate };
  const message = facilityIssueMessage(failure);
  authoring.error = { message, label: edit.label };
  if (edit.kind !== 'edit') authoring.historyError = { kind: edit.kind, message };
  edit.reopen?.(message, failure);
}

function holdStaleAuthoringEdits(edit, result) {
  if (Number(result.authoringRevision) > 0) authoring.revision = result.authoringRevision;
  if (result.session) authoring.lastSaved = structuredClone(result.session);
  authoring.held = [edit, ...authoring.queue].filter(held => held.kind === 'edit');
  authoring.queue = [];
  state.session = structuredClone(authoring.lastSaved);
  authoring.error = { message: 'СЕССИЯ ИЗМЕНИЛАСЬ ДРУГИМ ДЕЙСТВИЕМ — ПРАВКА НЕ ПРИМЕНЕНА', label: edit.label, stale: true };
}

// reapplyHeldEdits runs the edits rejected as stale again, in order, on the
// newer session («ПРИМЕНИТЬ СНОВА»).
function reapplyHeldEdits() {
  const held = authoring.held;
  authoring.held = [];
  authoring.error = null;
  for (const edit of held) {
    commitAuthoringEdit({ label: edit.label, scope: edit.scope, mutate: edit.mutate, reopen: edit.reopen });
  }
  renderAll();
}

function renderSaveStatus() {
  let saveState = 'idle';
  let text = '';
  if (authoring.error) {
    saveState = 'error';
    text = `ОШИБКА СОХРАНЕНИЯ: ${authoring.error.message}`;
  } else if (authoringBusy()) {
    saveState = 'saving';
    text = 'СОХРАНЕНИЕ…';
  } else if (authoring.lastSavedAt) {
    saveState = 'saved';
    text = `СОХРАНЕНО · ${authoring.lastSavedAt.toLocaleTimeString('ru-RU')}`;
  }
  const unapplied = unappliedInputLabels();
  if (unapplied.length) text = [text, `НЕ ПРИМЕНЕНО: ${unapplied.join(', ')}`].filter(Boolean).join(' · ');
  saveStatus.textContent = text;
  saveStatus.dataset.state = saveState;
  saveStatus.dataset.unappliedCount = String(unapplied.length);
  saveStatus.classList.toggle('err', saveState === 'error');
  renderAuthoringControls();
}

// ── Unapplied input (R7) ────────────────────────────────────
// unappliedInputs keeps form input that is not applied yet, keyed by surface
// and entity, so a tab switch or a live re-render shows it again.
const unappliedInputs = new Map();
// unappliedSurfaces maps each tracked form to the entity it shows now.
const unappliedSurfaces = new Map();
let unappliedRestoring = false;

function fieldValue(field) {
  return field.type === 'checkbox' ? field.checked : field.value;
}

function fieldValues(container) {
  const values = {};
  for (const field of container.querySelectorAll('input[id], select[id], textarea[id]')) {
    values[field.id] = fieldValue(field);
  }
  return values;
}

function surfaceFields(container, surface) {
  return [...container.querySelectorAll('input[id], select[id], textarea[id]')]
    .filter(field => Object.hasOwn(surface.applied, field.id));
}

// showUnappliedInput fills container's fields for surface: staged input where
// there is some, the applied values elsewhere. surface is { key, label,
// exists, applied, blocks? }, or null when the form shows no entity.
function showUnappliedInput(container, surface) {
  if (!surface) unappliedSurfaces.delete(container);
  else unappliedSurfaces.set(container, surface);
  const staged = surface && unappliedInputs.get(surface.key);
  const restored = [];
  for (const field of surface ? surfaceFields(container, surface) : []) {
    const value = staged && Object.hasOwn(staged.values, field.id) ? staged.values[field.id] : surface.applied[field.id];
    if (fieldValue(field) === value) continue;
    if (field.type === 'checkbox') field.checked = value;
    else field.value = value;
    if (staged) restored.push(field);
  }
  // Restored fields announce their value so dependent controls follow it.
  unappliedRestoring = true;
  try {
    restored.forEach(field => field.dispatchEvent(new Event('change', { bubbles: true })));
  } finally {
    unappliedRestoring = false;
  }
  markUnappliedFields(container);
  return staged || null;
}

function stageUnappliedInput(container) {
  const surface = unappliedSurfaces.get(container);
  if (!surface || unappliedRestoring) return;
  const values = {};
  let changed = false;
  for (const field of surfaceFields(container, surface)) {
    values[field.id] = fieldValue(field);
    if (values[field.id] !== surface.applied[field.id]) changed = true;
  }
  const blocks = surface.blocks?.() || null;
  if (changed || blocks) unappliedInputs.set(surface.key, { label: surface.label, exists: surface.exists, values, blocks });
  else unappliedInputs.delete(surface.key);
  markUnappliedFields(container);
  renderSaveStatus();
}

function markUnappliedFields(container) {
  const surface = unappliedSurfaces.get(container);
  const staged = Boolean(surface && unappliedInputs.has(surface.key));
  for (const field of container.querySelectorAll('input[id], select[id], textarea[id]')) {
    if (staged && Object.hasOwn(surface.applied, field.id) && fieldValue(field) !== surface.applied[field.id]) {
      field.dataset.unapplied = 'true';
    } else {
      delete field.dataset.unapplied;
    }
  }
  container.querySelectorAll('[data-unapplied-cancel]').forEach(button => { button.hidden = !staged; });
}

// takeUnappliedInput removes and returns the staged input of container, so
// an apply can hand it back if the server rejects the edit.
function takeUnappliedInput(container) {
  const surface = unappliedSurfaces.get(container);
  if (!surface) return null;
  const staged = unappliedInputs.get(surface.key) || null;
  unappliedInputs.delete(surface.key);
  markUnappliedFields(container);
  const key = surface.key;
  return staged && (() => { unappliedInputs.set(key, staged); renderAll(); });
}

function unappliedInputLabels() {
  return [...unappliedInputs.values()].map(entry => (entry.exists()
    ? entry.label : `${entry.label} — ИЗМЕНЁННЫЙ ЭЛЕМЕНТ БОЛЬШЕ НЕ СУЩЕСТВУЕТ`));
}

// explainControl blocks control with reason, or leaves it runnable with an
// optional description. A blocked control stays focusable; its
// aria-describedby element and title carry the text (FR-005).
function explainControl(control, reason, description = '') {
  if (reason) control.setAttribute('aria-disabled', 'true');
  else control.removeAttribute('aria-disabled');
  const text = reason || description;
  const reasonElement = document.getElementById(control.getAttribute('aria-describedby') || '');
  if (reasonElement) reasonElement.textContent = text;
  control.title = text;
}

function controlBlocked(control) {
  return control.getAttribute('aria-disabled') === 'true';
}

// historyBlockReason says why the newest entry of stack cannot be undone
// (kind 'undo') or redone (kind 'redo'), or returns ''.
function historyBlockReason(kind) {
  if (!state.session || !state.filePath) return 'НЕТ ОТКРЫТОЙ СЕССИИ';
  if (authoringBusy()) return 'ДОЖДИТЕСЬ СОХРАНЕНИЯ';
  const stack = kind === 'undo' ? authoring.undo : authoring.redo;
  const entry = stack.at(-1);
  if (!entry) {
    if (kind === 'undo' && authoring.historyTrimmed) return 'ИСТОРИЯ ХРАНИТ 20 ПОСЛЕДНИХ ПРАВОК';
    return kind === 'undo' ? 'НЕЧЕГО ОТМЕНЯТЬ' : 'НЕЧЕГО ПОВТОРЯТЬ';
  }
  if (authoring.historyError?.kind === kind) return authoring.historyError.message;
  const expected = kind === 'undo' ? entry.after : entry.before;
  if (authoredKey(state.session) !== authoredKey(expected)) return 'СЕССИЯ ИЗМЕНИЛАСЬ ПОСЛЕ ЭТОЙ ПРАВКИ';
  return '';
}

// runHistoryEntry saves the snapshot on the other side of the newest undo or
// redo entry through the save pipeline; the server keeps current live values.
function runHistoryEntry(kind) {
  const control = kind === 'undo' ? btnUndo : btnRedo;
  if (controlBlocked(control)) return;
  const entry = (kind === 'undo' ? authoring.undo : authoring.redo).at(-1);
  const target = kind === 'undo' ? entry.before : entry.after;
  commitAuthoringEdit({
    label: entry.label, kind, entry,
    mutate: session => replaceAuthoredContent(session, target),
  });
  renderAll();
}

function renderAuthoringControls() {
  explainControl(btnUndo, historyBlockReason('undo'),
    authoring.undo.length ? `ОТМЕНИТЬ: ${authoring.undo.at(-1).label}` : '');
  explainControl(btnRedo, historyBlockReason('redo'),
    authoring.redo.length ? `ПОВТОРИТЬ: ${authoring.redo.at(-1).label}` : '');
  btnReapplyEdit.hidden = !authoring.held.length;
  explainControl(btnResetScene, controlBlockReason(btnResetScene));
}

// ── Helpers ─────────────────────────────────────────────────
// ROOT_FOLDER_LABEL names a terminal's root folder in Overseer copy; the
// folder's stored name stays as authored.
const ROOT_FOLDER_LABEL = 'ГЛАВНОЕ МЕНЮ';

function escHtml(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}
function escAttr(s) {
  return escHtml(s).replace(/"/g, '&quot;');
}

function getEditTerminal() {
  if (!state.session) return null;
  return state.session.terminals.find(t => t.id === state.editTerminalId) || null;
}

function locateNode(root, id) {
  function walk(node, parent) {
    if (node.id === id) return { node, parent };
    if (node.children) {
      for (const child of node.children) {
        const res = walk(child, node);
        if (res) return res;
      }
    }
    return null;
  }
  return walk(root, null);
}

function commandExecutionState(term, commandID) {
  const commandStates = term?.commandStates;
  if (!commandStates || typeof commandStates !== 'object') return null;
  const snapshot = commandStates[commandID];
  return snapshot && typeof snapshot === 'object' ? snapshot : null;
}

function nodeKindLabel(node) {
  return node.type === 'folder' ? 'Папка' : node.type === 'command' ? 'Команда' : 'Запись';
}

function effectiveNodeName(term, node) {
  if (node?.type !== 'command') return node?.name || '';
  const completedName = commandExecutionState(term, node.id)?.completedName;
  return typeof completedName === 'string' && completedName ? completedName : node.name;
}

function visitContentNodes(node, visit) {
  if (!node) return;
  visit(node);
  for (const child of node.children ?? []) visitContentNodes(child, visit);
}

function entryBlockCatalog(term) {
  const catalog = [];
  visitContentNodes(term?.root, (node) => {
    if (node.type !== 'entry') return;
    for (const [index, block] of (node.blocks ?? []).entries()) {
      catalog.push({ entry: node, block, index });
    }
  });
  return catalog;
}

function entryBlockOwners(term) {
  const owners = new Map();
  visitContentNodes(term?.root, (node) => {
    const blockID = node.type === 'command'
      ? node.stateChange?.entryContentChange?.blockId
      : '';
    if (blockID && !owners.has(blockID)) owners.set(blockID, node);
  });
  return owners;
}

function entryBlockLocation(term, blockID) {
  return entryBlockCatalog(term).find(({ block }) => block.id === blockID) || null;
}

function entryBlockCommandCandidates(term, blockID, currentOwnerID = '') {
  const candidates = [];
  visitContentNodes(term?.root, (node) => {
    if (node.type !== 'command' || !node.stateChange) return;
    const targetBlockID = node.stateChange.entryContentChange?.blockId || '';
    if (targetBlockID && targetBlockID !== blockID) return;
    if (commandExecutionState(term, node.id) && node.id !== currentOwnerID) return;
    candidates.push(node);
  });
  return candidates;
}

function entryBlockFolderCatalog(term) {
  const folders = [];
  function walk(node, path) {
    if (node.type !== 'folder') return;
    const name = node.id === 'root' ? ROOT_FOLDER_LABEL : node.name;
    const nextPath = [...path, name];
    folders.push({ folder: node, label: nextPath.join(' › ') });
    for (const child of node.children ?? []) {
      if (child.type === 'folder') walk(child, nextPath);
    }
  }
  if (term?.root) walk(term.root, []);
  return folders;
}

function setEntryBlockCommandError(message, field = null) {
  entryBlockCommandError.textContent = message;
  entryBlockCommandError.hidden = !message;
  field?.focus();
}

function selectedEntryBlockCommandMode() {
  return entryBlockCommandModeInputs.find(input => input.checked)?.value || 'existing';
}

function updateEntryBlockCommandMode() {
  const mode = selectedEntryBlockCommandMode();
  entryBlockCommandDialog.querySelectorAll('[data-entry-block-command-fields]').forEach((fields) => {
    fields.hidden = fields.dataset.entryBlockCommandFields !== mode;
  });
  setEntryBlockCommandError('');
}

function closeEntryBlockCommandDialog({ restoreFocus = true } = {}) {
  const draft = entryBlockCommandDraft;
  if (entryBlockCommandDialog.open && typeof entryBlockCommandDialog.close === 'function') {
    entryBlockCommandDialog.close();
  } else {
    entryBlockCommandDialog.removeAttribute('open');
  }
  entryBlockCommandDialog.hidden = true;
  entryBlockCommandDraft = null;
  if (!restoreFocus || !draft) return;
  const trigger = nodeForm.querySelector(
    `.entry-content-block[data-block-id="${CSS.escape(draft.blockID)}"] [data-entry-block-action="configure-command"]`,
  );
  trigger?.focus();
}

function openEntryBlockCommandDialog(term, blockID) {
  const location = entryBlockLocation(term, blockID);
  if (!location) return;
  const owner = entryBlockOwners(term).get(blockID) || null;
  const completedOwner = owner ? commandExecutionState(term, owner.id) : null;
  const locked = Boolean(completedOwner);
  const candidates = entryBlockCommandCandidates(term, blockID, owner?.id);

  entryBlockCommandDraft = { terminalID: term.id, blockID };
  entryBlockCommandForm.reset();
  entryBlockCommandDialogDescription.textContent = entryBlockTargetLabel(
    location.entry,
    location.block,
    location.index,
  );
  if (completedOwner) {
    entryBlockCommandDialogDescription.textContent += ` · ${effectiveNodeName(term, owner)} · СНАЧАЛА СБРОСЬТЕ СОСТОЯНИЕ КОМАНДЫ`;
  }

  entryBlockExistingCommand.innerHTML = '<option value="">ВЫБЕРИТЕ КОМАНДУ</option>'
    + candidates.map(command =>
      `<option value="${escAttr(command.id)}">${escHtml(effectiveNodeName(term, command))}</option>`
    ).join('');
  entryBlockExistingCommand.value = owner?.id || '';
  entryBlockCompletedText.value = completedOwner?.entryContentChange?.completedText
    ?? owner?.stateChange?.entryContentChange?.completedText
    ?? '';
  entryBlockNewDestinationFolder.innerHTML = '<option value="">ВЫБЕРИТЕ ПАПКУ</option>'
    + entryBlockFolderCatalog(term).map(({ folder, label }) =>
      `<option value="${escAttr(folder.id)}">${escHtml(label)}</option>`
    ).join('');

  entryBlockCommandModeInputs.forEach(input => { input.disabled = locked; });
  entryBlockExistingCommand.disabled = locked;
  entryBlockCompletedText.disabled = locked;
  entryBlockCommandDialog.querySelectorAll('[data-entry-block-command-fields="new"] input, [data-entry-block-command-fields="new"] textarea, [data-entry-block-command-fields="new"] select')
    .forEach(control => { control.disabled = locked; });
  btnApplyEntryBlockCommand.disabled = locked;
  btnRemoveEntryBlockCommand.disabled = locked || !owner;
  setEntryBlockCommandError('');
  updateEntryBlockCommandMode();

  showEntryBlockCommandDialog();
  (completedOwner ? btnCancelEntryBlockCommand : entryBlockExistingCommand).focus();
}

function showEntryBlockCommandDialog() {
  entryBlockCommandDialog.hidden = false;
  if (typeof entryBlockCommandDialog.showModal === 'function' && !entryBlockCommandDialog.open) {
    entryBlockCommandDialog.showModal();
  } else {
    entryBlockCommandDialog.setAttribute('open', '');
  }
}

// commitTerminalEdit queues an edit of one terminal, found again by ID
// whenever the edit is replayed on a newer session.
function commitTerminalEdit(label, terminalID, mutate, reopen = null) {
  return commitAuthoringEdit({
    label,
    reopen,
    mutate: session => {
      const term = session.terminals.find(terminal => terminal.id === terminalID);
      if (!term) throw new Error('terminal removed');
      mutate(term);
    },
  });
}

// commitEntryBlockCommandEdit saves the dialog's edit and closes it; a
// rejected save reopens it with the author's input and the reason.
function commitEntryBlockCommandEdit(label, draft, mutate) {
  commitTerminalEdit(label, draft.terminalID, mutate, reason => {
    entryBlockCommandDraft = draft;
    showEntryBlockCommandDialog();
    setEntryBlockCommandError(reason);
  });
  renderTree();
  renderNodeForm();
  renderToolbarHint();
  closeEntryBlockCommandDialog();
}

function applyEntryBlockCommandDialog() {
  const draft = entryBlockCommandDraft;
  if (!draft || !state.session) return;
  const term = state.session.terminals.find(candidate => candidate.id === draft.terminalID);
  const location = entryBlockLocation(term, draft.blockID);
  if (!location) {
    setEntryBlockCommandError('БЛОК БОЛЬШЕ НЕ СУЩЕСТВУЕТ');
    return;
  }
  const owner = entryBlockOwners(term).get(draft.blockID) || null;
  if (owner && commandExecutionState(term, owner.id)) {
    setEntryBlockCommandError('СНАЧАЛА СБРОСЬТЕ СОСТОЯНИЕ КОМАНДЫ');
    return;
  }

  const mode = selectedEntryBlockCommandMode();
  let selectedCommandID = '';
  let destinationFolderID = '';
  if (mode === 'existing') {
    selectedCommandID = entryBlockExistingCommand.value;
    const selected = selectedCommandID ? locateNode(term.root, selectedCommandID)?.node : null;
    const targetBlockID = selected?.stateChange?.entryContentChange?.blockId || '';
    if (!selected || selected.type !== 'command' || !selected.stateChange
      || commandExecutionState(term, selected.id)
      || (targetBlockID && targetBlockID !== draft.blockID)) {
      setEntryBlockCommandError('ВЫБЕРИТЕ ДОСТУПНУЮ КОМАНДУ', entryBlockExistingCommand);
      return;
    }
  } else {
    const required = [
      [entryBlockNewCommandName, 'УКАЖИТЕ НАЗВАНИЕ ДО ВЫПОЛНЕНИЯ'],
      [entryBlockNewCompletedName, 'УКАЖИТЕ НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ'],
      [entryBlockNewConfirmationText, 'УКАЖИТЕ ТЕКСТ ПОДТВЕРЖДЕНИЯ'],
      [entryBlockNewResultText, 'УКАЖИТЕ РЕЗУЛЬТАТ КОМАНДЫ'],
    ];
    for (const [field, message] of required) {
      if (!field.value.trim()) {
        setEntryBlockCommandError(message, field);
        return;
      }
    }
    destinationFolderID = entryBlockNewDestinationFolder.value;
    const destination = destinationFolderID ? locateNode(term.root, destinationFolderID)?.node : null;
    if (!destination || destination.type !== 'folder') {
      setEntryBlockCommandError('ВЫБЕРИТЕ ПАПКУ ЭТОГО ТЕРМИНАЛА', entryBlockNewDestinationFolder);
      return;
    }
  }

  const entryContentChange = { blockId: draft.blockID, completedText: entryBlockCompletedText.value };
  let newCommand = null;
  if (mode !== 'existing') {
    let commandID;
    do commandID = uid('n'); while (locateNode(term.root, commandID));
    selectedCommandID = commandID;
    newCommand = {
      id: commandID,
      type: 'command',
      name: entryBlockNewCommandName.value.trim(),
      text: entryBlockNewResultText.value,
      stateChange: {
        completedName: entryBlockNewCompletedName.value,
        confirmationText: entryBlockNewConfirmationText.value,
        entryContentChange,
      },
    };
  }

  commitEntryBlockCommandEdit('СВЯЗЬ БЛОКА С КОМАНДОЙ', draft, candidateTerm => {
    const candidateOwner = entryBlockOwners(candidateTerm).get(draft.blockID) || null;
    if (candidateOwner && candidateOwner.id !== selectedCommandID) {
      delete candidateOwner.stateChange.entryContentChange;
    }
    if (newCommand) {
      const destination = locateNode(candidateTerm.root, destinationFolderID)?.node;
      if (!destination) throw new Error('folder removed');
      destination.children ??= [];
      if (!destination.children.some(child => child.id === newCommand.id)) {
        destination.children.push(structuredClone(newCommand));
      }
      return;
    }
    const selected = locateNode(candidateTerm.root, selectedCommandID)?.node;
    if (!selected?.stateChange) throw new Error('command removed');
    selected.stateChange.entryContentChange = { ...entryContentChange };
  });
}

function removeEntryBlockCommand() {
  const draft = entryBlockCommandDraft;
  if (!draft || !state.session) return;
  const term = state.session.terminals.find(candidate => candidate.id === draft.terminalID);
  if (!entryBlockLocation(term, draft.blockID)) {
    setEntryBlockCommandError('БЛОК БОЛЬШЕ НЕ СУЩЕСТВУЕТ');
    return;
  }
  const owner = entryBlockOwners(term).get(draft.blockID) || null;
  if (!owner) {
    setEntryBlockCommandError('СВЯЗЬ БОЛЬШЕ НЕ СУЩЕСТВУЕТ');
    return;
  }
  if (commandExecutionState(term, owner.id)) {
    setEntryBlockCommandError('СНАЧАЛА СБРОСЬТЕ СОСТОЯНИЕ КОМАНДЫ');
    return;
  }
  commitEntryBlockCommandEdit('СВЯЗЬ БЛОКА С КОМАНДОЙ', draft, candidateTerm => {
    const candidateOwner = entryBlockOwners(candidateTerm).get(draft.blockID);
    if (candidateOwner) delete candidateOwner.stateChange.entryContentChange;
  });
}

entryBlockCommandForm.addEventListener('submit', (event) => event.preventDefault());
entryBlockCommandModeInputs.forEach(input => input.addEventListener('change', updateEntryBlockCommandMode));
btnApplyEntryBlockCommand.addEventListener('click', applyEntryBlockCommandDialog);
btnRemoveEntryBlockCommand.addEventListener('click', removeEntryBlockCommand);
btnCancelEntryBlockCommand.addEventListener('click', closeEntryBlockCommandDialog);
entryBlockCommandDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  closeEntryBlockCommandDialog();
});

function normalizedEntryBlockPreview(value) {
  const normalized = String(value ?? '').trim().replace(/\s+/gu, ' ');
  if (!normalized) return 'ПУСТО';
  const codePoints = Array.from(normalized);
  return codePoints.length > 48 ? `${codePoints.slice(0, 48).join('')}…` : normalized;
}

function entryBlockTargetLabel(entry, block, index) {
  return `${entry.name} · БЛОК ${index + 1} · ${normalizedEntryBlockPreview(block.initialText)}`;
}

function entryBlockTargetOptions(term, selectedBlockID) {
  return entryBlockCatalog(term).map(({ entry, block, index }) =>
    `<option value="${escAttr(block.id)}"${block.id === selectedBlockID ? ' selected' : ''}>${escHtml(entryBlockTargetLabel(entry, block, index))}</option>`
  ).join('');
}

function commandOwningEntryBlock(term, blockID, excludedCommandID = '') {
  const owner = entryBlockOwners(term).get(blockID);
  return owner && owner.id !== excludedCommandID ? owner : null;
}

function targetingCommandsWithinNode(term, node) {
  const blockIDs = new Set();
  visitContentNodes(node, (candidate) => {
    if (candidate.type !== 'entry') return;
    for (const block of candidate.blocks ?? []) blockIDs.add(block.id);
  });
  const commands = [];
  const seen = new Set();
  const owners = entryBlockOwners(term);
  for (const blockID of blockIDs) {
    const command = owners.get(blockID);
    if (command && !seen.has(command.id)) {
      seen.add(command.id);
      commands.push(command);
    }
  }
  return commands;
}

function entryBlockDeletionError(term, commands) {
  const names = commands.map(command => effectiveNodeName(term, command)).join(', ');
  return `НЕЛЬЗЯ УДАЛИТЬ: БЛОК ИЗМЕНЯЕТСЯ КОМАНДОЙ: ${names}. СНАЧАЛА СБРОСЬТЕ, ПЕРЕНАЗНАЧЬТЕ ИЛИ УДАЛИТЕ КОМАНДУ.`;
}

function uniqueEntryBlockID(term) {
  const existing = new Set(entryBlockCatalog(term).map(({ block }) => block.id));
  let id;
  do id = uid('b'); while (existing.has(id));
  return id;
}

function entryContentBlockMarkup(term, block, index, { legacyDraft = false, owners = entryBlockOwners(term) } = {}) {
  const owner = block.id ? owners.get(block.id) : null;
  const ownerMarkup = owner
    ? `<div class="entry-content-block-owner" data-entry-block-owner data-command-id="${escAttr(owner.id)}">ИЗМЕНЯЕТСЯ КОМАНДОЙ: ${escHtml(effectiveNodeName(term, owner))}</div>`
    : '';
  return `<div class="entry-content-block" data-block-id="${legacyDraft ? '' : escAttr(block.id)}"${legacyDraft ? ' data-legacy-draft="true"' : ''}>
    <div class="entry-content-block-heading">
      <span class="entry-content-block-index">БЛОК ${index + 1}</span>
      <span class="entry-content-block-actions" role="group" aria-label="Действия блока ${index + 1}">
        <button class="btn btn-mini" type="button" data-entry-block-action="configure-command" aria-label="Настроить команду для блока ${index + 1}" aria-haspopup="dialog" aria-controls="entryBlockCommandDialog"${legacyDraft ? ' disabled title="Сначала примените блок"' : ''}>КОМАНДА…</button>
        <button class="btn btn-mini" type="button" data-entry-block-action="move-up" aria-label="Переместить блок ${index + 1} вверх">↑</button>
        <button class="btn btn-mini" type="button" data-entry-block-action="move-down" aria-label="Переместить блок ${index + 1} вниз">↓</button>
        <button class="btn btn-mini btn-danger" type="button" data-entry-block-action="delete" aria-label="Удалить блок ${index + 1}">×</button>
      </span>
    </div>
    <textarea class="field-textarea entry-content-block-text" data-entry-block-content aria-label="ИСХОДНЫЙ ТЕКСТ БЛОКА ${index + 1}">${escHtml(block.initialText)}</textarea>
    ${ownerMarkup}
  </div>`;
}

function renderSessionStateResult(result, successMessage, acceptsCanonicalResult = null) {
  if (!result?.ok || !result.session) {
    setCoordinationError(result, 'commands', { prefix: 'ОШИБКА ИЗМЕНЕНИЯ СОСТОЯНИЯ:' });
    return false;
  }
  if (typeof acceptsCanonicalResult === 'function' && !acceptsCanonicalResult(result)) {
    setCoordinationStatus('ОШИБКА ИЗМЕНЕНИЯ СОСТОЯНИЯ: сброс не подтверждён. Проверьте состояние команды и повторите сброс.', true);
    return false;
  }

  applyLiveSession(result.session, Number(result.revision || result.savedRevision || 0));
  setCoordinationStatus(successMessage);
  renderAll();
  return true;
}

function updateSessionStateEvidenceDescription() {
  const evidence = [];
  if (saveStatus.dataset.wailsCommand) {
    evidence.push(`Wails command ${saveStatus.dataset.wailsCommand} ${saveStatus.dataset.wailsResult || 'unknown'}`);
  }
  if (saveStatus.dataset.wailsTerminalId) evidence.push(`terminal ${saveStatus.dataset.wailsTerminalId}`);
  if (saveStatus.dataset.wailsRevision) evidence.push(`document revision ${saveStatus.dataset.wailsRevision}`);
  if (saveStatus.dataset.sessionStateRevision) {
    evidence.push(`session-state revision ${saveStatus.dataset.sessionStateRevision}`);
  }
  // The evidence is visually hidden text that describes the status, so the
  // status keeps its visible Russian text as its name. Native smoke checks
  // read the hidden text, because WebKit exposes no description for it.
  saveStatusEvidence.textContent = evidence.join('; ');
}

async function runSessionStateCommand(command, successMessage, acceptsCanonicalResult = null) {
  if (sessionStateCommandPending) return;
  sessionStateCommandPending = true;
  renderSettingsPanel();
  renderNodeForm();
  try {
    const result = await command();
    renderSessionStateResult(result, successMessage, acceptsCanonicalResult);
  } catch (error) {
    setCoordinationError(error instanceof Error ? error.message : String(error), 'commands',
      { prefix: 'ОШИБКА ИЗМЕНЕНИЯ СОСТОЯНИЯ:' });
  } finally {
    sessionStateCommandPending = false;
    renderSettingsPanel();
    renderNodeForm();
    renderSaveStatus();
  }
}

function currentAddTarget() {
  const term = getEditTerminal();
  if (!term) return null;
  if (!state.selectedNodeId) return term.root;
  const loc = locateNode(term.root, state.selectedNodeId);
  if (!loc) return term.root;
  return loc.node.type === 'folder' ? loc.node : (loc.parent || term.root);
}

// ── Facility authoring ──────────────────────────────────────
function emptyFacility() {
  return { revision: 0, devices: [], conditions: [], recoveryPrograms: [] };
}

function resetFacilitySelection() {
  selectedFacilityDeviceID = '';
  pendingFacilityRepair = null;
}

const EMPTY_FACILITY = Object.freeze({
  revision: 0, devices: Object.freeze([]), conditions: Object.freeze([]), recoveryPrograms: Object.freeze([]),
});

// authoredFacility is the working copy's facility. A session without one reads
// as empty; only an edit creates it (ensureFacility), so saving a legacy
// session never adds a facility to it.
function authoredFacility() {
  if (!state.session) return null;
  return state.session.facility || EMPTY_FACILITY;
}

function ensureFacility(session) {
  if (!session.facility || typeof session.facility !== 'object') session.facility = emptyFacility();
  for (const key of ['devices', 'conditions', 'recoveryPrograms']) session.facility[key] ||= [];
  return session.facility;
}

// commitFacilityEdit saves one facility edit and redraws the workspace.
function commitFacilityEdit(label, mutate, reopen = null) {
  commitAuthoringEdit({ label, scope: 'facility', mutate, reopen });
  renderFacilityWorkspace();
}

function setFacilityFeedback(element, message = '', field = null) {
  element.textContent = message;
  element.hidden = !message;
  const dialog = element.closest('dialog');
  dialog?.querySelectorAll('[aria-invalid="true"]').forEach(control => control.removeAttribute('aria-invalid'));
  if (!message || !field) return;
  for (let parent = field.parentElement; parent; parent = parent.parentElement) {
    if (parent instanceof HTMLDetailsElement) parent.open = true;
  }
  field.setAttribute('aria-invalid', 'true');
  field.focus();
}

// setFacilityFailure shows a rejected facility request in element, clearing
// the field marks of its dialog as setFacilityFeedback does.
function setFacilityFailure(element, result) {
  setFacilityFeedback(element);
  renderOverseerError(element, result, 'facility');
}

function showFacilityDialog(dialog, focusTarget) {
  dialog.facilityReturnToStory = Boolean(document.activeElement?.closest('#facilityStoryActions'));
  dialog.hidden = false;
  if (typeof dialog.showModal === 'function' && !dialog.open) dialog.showModal();
  focusTarget?.focus();
}

function hideFacilityDialog(dialog, focusTarget = null) {
  if (dialog.open && typeof dialog.close === 'function') dialog.close();
  else dialog.removeAttribute('open');
  dialog.hidden = true;
  if (dialog.facilityReturnToStory && document.getElementById('facilityStoryGuide').open) {
    document.getElementById('facilityStoryTitle').focus();
  } else {
    focusTarget?.focus();
  }
  delete dialog.facilityReturnToStory;
}

function populateSelect(select, values, selected = '', placeholder = '') {
  select.replaceChildren();
  if (placeholder) {
    const option = document.createElement('option');
    option.value = '';
    option.textContent = placeholder;
    select.appendChild(option);
  }
  for (const value of values) {
    const option = document.createElement('option');
    option.value = value.id;
    option.textContent = value.name || value.id;
    option.title = value.id;
    select.appendChild(option);
  }
  if ([...select.options].some(option => option.value === selected)) select.value = selected;
}

// FACILITY_DEVICE_TEMPLATES suggests states and transitions for every device
// kind except custom (R9). Transitions name their states. The device dialog
// offers a template and never stores one.
const FACILITY_DEVICE_TEMPLATES = {
  door: {
    states: ['Заперта', 'Открыта'],
    initial: 'Заперта',
    transitions: [
      { name: 'Открыть', from: 'Заперта', to: 'Открыта' },
      { name: 'Запереть', from: 'Открыта', to: 'Заперта' },
    ],
  },
  turret: {
    states: ['Безопасный режим', 'Сопровождение'],
    initial: 'Безопасный режим',
    transitions: [
      { name: 'Включить сопровождение', from: 'Безопасный режим', to: 'Сопровождение' },
      { name: 'Вернуть безопасный режим', from: 'Сопровождение', to: 'Безопасный режим' },
    ],
  },
  'power-grid': {
    states: ['Нет питания', 'В работе'],
    initial: 'Нет питания',
    transitions: [
      { name: 'Запустить', from: 'Нет питания', to: 'В работе' },
      { name: 'Отключить', from: 'В работе', to: 'Нет питания' },
    ],
  },
  reactor: {
    states: ['Аварийная остановка', 'Дежурный режим'],
    initial: 'Аварийная остановка',
    transitions: [
      { name: 'Перевести в дежурный режим', from: 'Аварийная остановка', to: 'Дежурный режим' },
      { name: 'Остановить', from: 'Дежурный режим', to: 'Аварийная остановка' },
    ],
  },
  ventilation: {
    states: ['Загрязнён', 'Очищен'],
    initial: 'Загрязнён',
    transitions: [
      { name: 'Продуть', from: 'Загрязнён', to: 'Очищен' },
      { name: 'Смоделировать загрязнение', from: 'Очищен', to: 'Загрязнён' },
    ],
  },
  alarm: {
    states: ['Тишина', 'Эвакуация'],
    initial: 'Тишина',
    transitions: [
      { name: 'Подать сигнал', from: 'Тишина', to: 'Эвакуация' },
      { name: 'Отключить сигнал', from: 'Эвакуация', to: 'Тишина' },
    ],
  },
  'robot-pod': {
    states: ['Зарядка', 'Патруль'],
    initial: 'Зарядка',
    transitions: [
      { name: 'Отправить в патруль', from: 'Зарядка', to: 'Патруль' },
      { name: 'Вернуть на зарядку', from: 'Патруль', to: 'Зарядка' },
    ],
  },
  elevator: {
    states: ['Нижний уровень', 'Верхний уровень'],
    initial: 'Нижний уровень',
    transitions: [
      { name: 'Поднять кабину', from: 'Нижний уровень', to: 'Верхний уровень' },
      { name: 'Опустить кабину', from: 'Верхний уровень', to: 'Нижний уровень' },
    ],
  },
  'network-segment': {
    states: ['Изолирована', 'Подключена'],
    initial: 'Изолирована',
    transitions: [
      { name: 'Подключить', from: 'Изолирована', to: 'Подключена' },
      { name: 'Изолировать', from: 'Подключена', to: 'Изолирована' },
    ],
  },
};
const CYRILLIC_TO_LATIN = {
  а: 'a', б: 'b', в: 'v', г: 'g', д: 'd', е: 'e', ё: 'yo', ж: 'zh', з: 'z', и: 'i', й: 'y', к: 'k',
  л: 'l', м: 'm', н: 'n', о: 'o', п: 'p', р: 'r', с: 's', т: 't', у: 'u', ф: 'f', х: 'kh', ц: 'ts',
  ч: 'ch', ш: 'sh', щ: 'shch', ъ: '', ы: 'y', ь: '', э: 'e', ю: 'yu', я: 'ya',
  і: 'i', ї: 'yi', є: 'ye', ґ: 'g', ў: 'u',
};
const FACILITY_ID_LIMIT = 256;

// Suggests an identifier for a new facility entity. Transliteration runs before NFKD, which would
// otherwise split й and ё; uniqueness is checked against the final values in existingIDs.
function suggestFacilityID(prefix, name, existingIDs, keepPrefix = true) {
  const stem = [...String(name || '').toLowerCase()]
    .map(character => CYRILLIC_TO_LATIN[character] ?? character)
    .join('')
    .normalize('NFKD')
    .replace(/\p{M}/gu, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  const base = !stem ? prefix : keepPrefix && !stem.startsWith(`${prefix}-`) ? `${prefix}-${stem}` : stem;
  for (let attempt = 1; ; attempt++) {
    const suffix = stem && attempt === 1 ? '' : `-${attempt}`;
    const candidate = base.slice(0, FACILITY_ID_LIMIT - suffix.length).replace(/-+$/, '') + suffix;
    if (!existingIDs.has(candidate)) return candidate;
  }
}

function facilityDeviceByID(deviceID, session = state.session) {
  return session?.facility?.devices?.find(device => device.id === deviceID) || null;
}

function facilityNodeCatalog(session = state.session) {
  const values = [];
  for (const terminal of session?.terminals || []) {
    visitContentNodes(terminal.root, node => values.push({ terminal, node }));
  }
  return values;
}

function facilityBindingCatalog(session = state.session) {
  const bindings = [];
  for (const { terminal, node } of facilityNodeCatalog(session)) {
    for (const [index, variant] of (node.facilityNameVariants || []).entries()) {
      bindings.push({ terminal, node, variant, property: 'menu-name', index });
    }
    for (const block of node.blocks || []) {
      for (const [index, variant] of (block.facilityTextVariants || []).entries()) {
        bindings.push({ terminal, node, block, variant, property: 'entry-content', index });
      }
    }
    if (node.visibleWhen) {
      bindings.push({ terminal, node, variant: { when: node.visibleWhen }, property: 'visibility', index: 0 });
    }
    if (node.availableWhen) {
      bindings.push({ terminal, node, variant: { when: node.availableWhen }, property: 'availability', index: 0 });
    }
    const requests = node.stateChange?.facilityAction?.transitions?.transitions || [];
    if (requests.length) {
      bindings.push({
        terminal, node, requests,
        variant: { text: 'ДЕЙСТВИЕ КОМАНДЫ' },
        property: 'command-action', index: 0,
      });
    }
    const programID = node.stateChange?.facilityAction?.recoveryProgramId;
    if (programID) {
      const program = session.facility?.recoveryPrograms?.find(value => value.id === programID);
      bindings.push({ terminal, node, programID, requests: program?.transitions || [],
        variant: { text: program?.name || programID }, property: 'recovery-action', index: 0 });
    }
  }
  return bindings;
}

function facilityListRow(kind, id, heading, detail = '') {
  const row = document.createElement(kind === 'device' || kind === 'condition' ? 'button' : 'div');
  if (row instanceof HTMLButtonElement) row.type = 'button';
  row.dataset.facilityKind = kind;
  row.dataset.facilityId = id;
  if (!(row instanceof HTMLButtonElement)) row.setAttribute('role', 'listitem');
  const title = document.createElement('strong');
  title.textContent = heading || id;
  row.appendChild(title);
  if (detail) {
    const description = document.createElement('div');
    description.textContent = detail;
    row.appendChild(description);
  }
  return row;
}

function facilityConditionActivity(condition) {
  return `СЕЙЧАС: ${condition.currentActive ? 'АКТИВНА' : 'НЕАКТИВНА'} · В НАЧАЛЕ: ${condition.initialActive ? 'АКТИВНА' : 'ИСПРАВНО'}`;
}

function facilityStateName(device, stateID) {
  return device?.states?.find(value => value.id === stateID)?.name || stateID || 'Не задано';
}

function facilityEqualitySummary(equality) {
  const device = facilityDeviceByID(equality?.deviceId);
  return `${device?.name || equality?.deviceId || 'Устройство не найдено'}: ${facilityStateName(device, equality?.stateId)}`;
}

function facilityTransitionSummary(request) {
  const device = facilityDeviceByID(request.deviceId);
  const transition = device?.transitions?.find(value => value.id === request.transitionId);
  if (!transition) return `${device?.name || request.deviceId}: переход не найден (${request.transitionId})`;
  const requirement = (transition.preconditions || []).map(facilityEqualitySummary).join('; ');
  return `${device.name} — ${transition.name}: ${facilityStateName(device, transition.sourceStateId)} → ${facilityStateName(device, transition.destinationStateId)}${requirement ? ` · Требуется: ${requirement}` : ''}`;
}

function facilityCommandActionSummary(action) {
  const requests = action?.transitions?.transitions || [];
  if (requests.length) return requests.map(facilityTransitionSummary).join('\n');
  const programID = action?.recoveryProgramId;
  if (!programID) return 'Действие команды не настроено.';
  const program = authoredFacility()?.recoveryPrograms?.find(value => value.id === programID);
  const heading = `Программа восстановления: ${program?.name || programID}`;
  const transitions = (program?.transitions || []).map(facilityTransitionSummary).join('\n');
  return transitions ? `${heading}\n${transitions}` : heading;
}

// FACILITY_BINDING_PURPOSES names each binding property in Overseer copy.
const FACILITY_BINDING_PURPOSES = {
  'menu-name': 'Название в меню', 'entry-content': 'Текст блока записи',
  visibility: 'Показывать элемент', availability: 'Разрешить команду',
  'command-action': 'Действие команды', 'recovery-action': 'Программа восстановления',
};

function facilityBindingSummary(binding) {
  if (Array.isArray(binding.requests)) {
    const heading = binding.property === 'recovery-action'
      ? `Программа: ${binding.variant.text}\n` : 'ДЕЙСТВИЕ КОМАНДЫ\n';
    return `${heading}${binding.requests.map(facilityTransitionSummary).join('\n')}`;
  }
  const when = facilityEqualitySummary(binding.variant.when);
  return `${FACILITY_BINDING_PURPOSES[binding.property]} · Когда ${when}${['menu-name', 'entry-content'].includes(binding.property) ? `\n${binding.variant.text}` : ''}`;
}

function facilityBindingUsesDevice(binding, deviceID) {
  return Array.isArray(binding.requests)
    ? binding.requests.some(value => value.deviceId === deviceID)
    : binding.variant.when.deviceId === deviceID;
}

function facilityTechnicalDetails(parent, text, summaryText = 'подробности для разработчика') {
  const details = document.createElement('details');
  details.className = 'facility-technical-details';
  const summary = document.createElement('summary');
  summary.textContent = summaryText;
  const content = document.createElement('p');
  content.textContent = text;
  details.append(summary, content);
  parent.appendChild(details);
}

function facilityEmptyList(list, message) {
  if (list.children.length) return;
  const empty = document.createElement('p');
  empty.className = 'facility-empty';
  empty.textContent = message;
  list.appendChild(empty);
}

// facilityRowActions adds «Изменить» and «Удалить» to row. When editDialog
// is set, «Изменить» names the dialog it opens.
function facilityRowActions(row, name, edit, remove, editDialog = '') {
  const actions = document.createElement('div');
  actions.className = 'facility-related-actions';
  for (const [label, callback] of [['Изменить', edit], ['Удалить', remove]]) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = `btn btn-mini ${label === 'Удалить' ? 'btn-danger' : 'btn-secondary'}`;
    button.textContent = label;
    button.setAttribute('aria-label', `${label}: ${name}`);
    if (label === 'Изменить' && editDialog) {
      button.setAttribute('aria-haspopup', 'dialog');
      button.setAttribute('aria-controls', editDialog);
    }
    button.addEventListener('click', callback);
    actions.appendChild(button);
  }
  row.appendChild(actions);
}

function facilityBindingKey(binding) {
  return {
    terminalID: binding.terminal.id, nodeID: binding.node.id, blockID: binding.block?.id || '',
    property: binding.property, index: binding.index,
  };
}

// removeFacilityBinding removes the binding named by key from session.
function removeFacilityBinding(session, key) {
  const terminal = session.terminals.find(candidate => candidate.id === key.terminalID);
  const node = terminal ? locateNode(terminal.root, key.nodeID)?.node : null;
  if (!node) return;
  const block = node.blocks?.find(candidate => candidate.id === key.blockID);
  if (key.property === 'menu-name') node.facilityNameVariants?.splice(key.index, 1);
  else if (key.property === 'entry-content') block?.facilityTextVariants?.splice(key.index, 1);
  else if (key.property === 'visibility') delete node.visibleWhen;
  else if (key.property === 'availability') delete node.availableWhen;
  else if (node.stateChange) delete node.stateChange.facilityAction;
}

// removeFacilityProgram deletes a program no command runs. Faults whose
// recovery entries name it lose those entries in the same edit (R2).
function removeFacilityProgram(program) {
  const commands = facilityNodeCatalog()
    .filter(({ node }) => node.stateChange?.facilityAction?.recoveryProgramId === program.id).map(({ node }) => node.name);
  if (commands.length) {
    setFacilityFeedback(facilityValidation, `Программа «${program.name}» используется: ${commands.join('; ')}. Сначала измените связанные команды.`);
    return;
  }
  commitFacilityEdit(`Программа «${program.name}»`, session => {
    const facility = ensureFacility(session);
    pruneFacilityRecovery(facility, () => {
      facility.recoveryPrograms = facility.recoveryPrograms.filter(value => value.id !== program.id);
    });
  });
}

const facilityStorySteps = [
  {
    label: 'УСТРОЙСТВО',
    title: '1. Что может измениться в сцене?',
    explanation: 'Создайте устройство — предмет или систему, важную для истории. Назовите его состояния и выберите начальное. Если устройство уже есть, выберите его слева и продолжайте настройку.',
    example: 'Пример: «Дверь в убежище». Состояния: «Заперта» и «Открыта». Начальное: «Заперта». В форме также добавьте переход «Открыть дверь»: «Заперта» → «Открыта».',
  },
  {
    label: 'ДЕЙСТВИЕ',
    title: '2. Как игроки изменят ситуацию?',
    explanation: 'Переход описывает изменение устройства. Чтобы игроки могли его запросить, свяжите переход с командой терминала. Если команды ещё нет, создайте её на вкладке «ТЕРМИНАЛЫ», затем вернитесь сюда.',
    example: 'Пример: команда «Открыть дверь» запускает переход двери «Заперта» → «Открыта». Игрок запрашивает действие, Смотритель подтверждает. Если нужно питание, добавьте предусловие «Электросеть: В работе» в переход.',
  },
  {
    label: 'РЕЗУЛЬТАТ',
    title: '3. Как игроки узнают о результате?',
    explanation: 'Добавьте правило отображения: текст записи, название в меню, видимость элемента или доступность команды. Оно проверяет состояние устройства, но само его не меняет. Исходный текст остаётся для остальных состояний.',
    example: 'Пример: в записи «Состояние шлюза» исходный текст — «Дверь заперта». Для состояния «Открыта» задайте текст блока «Путь свободен». Отдельным правилом можно показать новый маршрут или разрешить команду.',
  },
  {
    label: 'ОСЛОЖНЕНИЯ',
    title: '4. Что может пойти не так? · Необязательно',
    explanation: 'Простая сцена уже обходится без неисправностей и программ. Добавляйте их, если истории нужны препятствие и способ его устранить. Неисправность задаёт ограничение, эффект перехода включает или снимает её.',
    example: 'Пример: «Привод двери повреждён» мешает выполнить команду. В окне неисправности укажите, какой переход её включает и какой устраняет. Смотритель может снять её вручную во время игры, а программа объединяет несколько переходов.',
  },
  {
    label: 'ПРОВЕРКА',
    title: '5. Проверьте сцену',
    explanation: 'Каждая правка сохраняется сразу и сразу доходит до игроков. «ОТМЕНИТЬ» на общей панели возвращает последнюю правку. Сравните состояния в предпросмотре — он не меняет мир для игроков.',
    example: 'Проверка двери: выберите в предпросмотре «Открыта» и нужный терминал. Видно ли «Путь свободен»? После этого проверьте реальный запрос команды игроком и подтверждение Смотрителем. Предпросмотр проверяет отображение, а не выполнение команды.',
  },
];

function openFacilityStoryTerminals() {
  selectWorkspace('terminal');
  terminalWorkspaceTab.focus();
}

function setFacilityStoryStep(step, focusHeading = false) {
  facilityStoryStep = Math.max(0, Math.min(step, facilityStorySteps.length - 1));
  renderFacilityStoryGuide();
  if (focusHeading) document.getElementById('facilityStoryTitle').focus();
}

// renderFacilityGuideOpen applies the guide rule: the author's explicit
// choice for this run, or else open exactly while the session has no devices.
// The first saved device clears the choice, so the guide collapses then.
function renderFacilityGuideOpen(facility) {
  const guide = document.getElementById('facilityStoryGuide');
  const count = facility.devices.length;
  if (facilityGuideDeviceCount === 0 && count > 0) facilityGuideChoice = null;
  facilityGuideDeviceCount = count;
  const open = facilityGuideChoice ? facilityGuideChoice === 'expanded' : count === 0;
  // Collapsing hides whatever had focus inside the guide; keep focus on its summary.
  const focusInside = guide.open && !open && guide.contains(document.activeElement);
  guide.open = open;
  if (focusInside) document.getElementById('facilityStoryGuideSummary').focus();
  renderFacilityGuideSummary();
}

function renderFacilityGuideSummary() {
  const step = facilityStorySteps[facilityStoryStep];
  document.getElementById('facilityStoryGuideStep').textContent =
    `КАК СОБРАТЬ СЦЕНАРИЙ · ШАГ ${facilityStoryStep + 1} ИЗ ${facilityStorySteps.length}: ${step.label}`;
  document.getElementById('facilityStoryGuideToggle').textContent =
    document.getElementById('facilityStoryGuide').open ? 'СКРЫТЬ' : 'ПОКАЗАТЬ';
}

function renderFacilityStoryGuide() {
  const facility = authoredFacility();
  if (!facility) return;
  renderFacilityGuideOpen(facility);
  const device = facilityDeviceByID(selectedFacilityDeviceID);
  const bindings = facilityBindingCatalog().filter(binding => device && facilityBindingUsesDevice(binding, device.id));
  const actions = bindings.filter(binding => Array.isArray(binding.requests));
  const information = bindings.filter(binding => !Array.isArray(binding.requests));
  const step = facilityStorySteps[facilityStoryStep];
  document.getElementById('facilityStoryTitle').textContent = step.title;
  document.getElementById('facilityStoryExplanation').textContent = step.explanation;
  document.getElementById('facilityStoryExample').textContent = step.example;
  for (const button of document.querySelectorAll('[data-story-step]')) {
    if (Number(button.dataset.storyStep) === facilityStoryStep) button.setAttribute('aria-current', 'step');
    else button.removeAttribute('aria-current');
  }
  document.getElementById('btnFacilityStoryBack').disabled = facilityStoryStep === 0;
  document.getElementById('btnFacilityStoryNext').disabled = facilityStoryStep === facilityStorySteps.length - 1;
  const context = document.getElementById('facilityStoryContext');
  context.textContent = device
    ? `Настраиваем «${device.name}». Состояний: ${device.states.length}; переходов: ${device.transitions?.length || 0}; команд: ${actions.length}; правил отображения: ${information.length}.`
    : `Устройств в сессии: ${facility.devices.length}. Выберите устройство слева или создайте первое для своей сцены.`;
  const controls = document.getElementById('facilityStoryActions');
  controls.replaceChildren();
  const addAction = (label, action, primary = false) => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = `btn ${primary ? 'btn-primary' : 'btn-secondary'}`;
    button.textContent = label;
    button.addEventListener('click', action);
    controls.appendChild(button);
    return button;
  };
  const addTerminalsAction = (primary = false) => {
    addAction('ОТКРЫТЬ ТЕРМИНАЛЫ', openFacilityStoryTerminals, primary);
  };
  // addCommandAction opens «ДЕЙСТВИЕ КОМАНДЫ» on target's command, which the
  // author may change there when pick is set.
  const addCommandAction = (label, target, pick, primary = false) => {
    const button = addAction(label,
      () => openFacilityCommandActionDialog(target.terminal.id, target.node.id, { pick }), primary);
    button.setAttribute('aria-haspopup', 'dialog');
    button.setAttribute('aria-controls', 'facilityCommandActionDialog');
  };
  const editDevice = () => openFacilityDeviceDialog(device?.id || '');
  if (!device) {
    addAction('СОЗДАТЬ УСТРОЙСТВО', () => openFacilityDeviceDialog(), true);
    if (facility.devices.length) addAction('ВЫБРАТЬ УСТРОЙСТВО', () => {
      document.getElementById('facilityDeviceSearch').focus();
    });
    if (facilityStoryStep !== 4) return;
  } else if (facilityStoryStep === 0) {
    addAction('ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ', editDevice, true);
    addAction('СОЗДАТЬ ЕЩЁ УСТРОЙСТВО', () => openFacilityDeviceDialog());
  } else if (facilityStoryStep === 1) {
    if (!device.transitions?.length) {
      context.textContent += ' Сначала добавьте переход: из какого состояния и в какое ведёт действие.';
      addAction('ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ', editDevice, true);
      return;
    }
    const existing = actions[0];
    const freeCommand = facilityActionCommands().find(({ node }) => !node.stateChange?.facilityAction);
    const target = existing || freeCommand;
    if (target) {
      addCommandAction(existing ? 'ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ' : 'СВЯЗАТЬ С КОМАНДОЙ', target, !existing, true);
      if (existing && freeCommand) addCommandAction('СВЯЗАТЬ С ДРУГОЙ КОМАНДОЙ', freeCommand, true);
    } else {
      context.textContent += ' Нет свободной команды. Создайте команду в нужном терминале и вернитесь на этот шаг.';
      addTerminalsAction(true);
    }
    addAction('ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ', editDevice);
  } else if (facilityStoryStep === 2) {
    const existing = information[0];
    const nodes = facilityNodeCatalog().filter(({ node }) => node.id !== 'root');
    const target = existing || nodes.find(({ node }) => node.type === 'entry' && node.blocks?.length) || nodes[0];
    if (target) {
      addAction(existing ? 'ИЗМЕНИТЬ ПРАВИЛО ОТОБРАЖЕНИЯ' : 'НАСТРОИТЬ РЕЗУЛЬТАТ НА ТЕРМИНАЛЕ',
        () => openFacilityBindingDialog(existing, target), true);
      addAction('ДОБАВИТЬ ДРУГОЕ ПРАВИЛО', () => openFacilityBindingDialog(null, target));
    } else {
      context.textContent += ' Сначала создайте запись или другой элемент в терминале: здесь будет показан результат.';
      addTerminalsAction(true);
    }
  } else if (facilityStoryStep === 3) {
    addAction('ДОБАВИТЬ НЕИСПРАВНОСТЬ', () => openFacilityConditionDialog(), true);
    if (device.transitions?.length) {
      addAction('СОБРАТЬ ПРОГРАММУ ВОССТАНОВЛЕНИЯ', () => openFacilityRecoveryProgramDialog());
    } else {
      context.textContent += ' Если нужна программа восстановления, сначала добавьте переход устройства: программа запускает готовые переходы. Для простой сцены этот шаг можно пропустить.';
      addAction('ИЗМЕНИТЬ СОСТОЯНИЯ И ПЕРЕХОДЫ', editDevice);
    }
    // Which transitions activate and clear a fault is set in its dialog (FR-017).
    const fault = (facility.conditions || []).find(value => value.device?.deviceId === device.id);
    if (fault) addAction('НАСТРОИТЬ АКТИВАЦИЮ И СНЯТИЕ', () => openFacilityConditionDialog(fault.id));
  }
  if (facilityStoryStep === 4) {
    if (device) {
      context.textContent += ' Все правки уже сохранены. Сравните результат в нужном терминале.';
      addAction('ПРОВЕРИТЬ В ПРЕДПРОСМОТРЕ', () => openFacilityPreview('device', controls.firstElementChild), true);
    }
    addTerminalsAction();
  }
}

function renderFacilityWorkspace() {
  const facility = authoredFacility();
  if (!facility) return;
  const search = document.getElementById('facilityDeviceSearch')?.value.trim().toLocaleLowerCase() || '';
  const bindings = facilityBindingCatalog();
  renderFacilityStoryGuide();
  renderFacilityDeviceFilters(facility);
  facilityDeviceList.replaceChildren();
  let selectionHidden = false;
  for (const device of facility.devices || []) {
    const matches = facilityDeviceFilters[facilityDeviceFilter](device, facility)
      && (!search || [device.name, facilityStateName(device, device.currentStateId)]
        .some(text => text.toLocaleLowerCase().includes(search)));
    if (!matches) {
      selectionHidden ||= device.id === selectedFacilityDeviceID;
      continue;
    }
    const row = facilityDeviceRow(device, facility);
    row.dataset.facilitySelected = String(device.id === selectedFacilityDeviceID);
    row.setAttribute('aria-pressed', String(device.id === selectedFacilityDeviceID));
    row.addEventListener('click', () => {
      if (device.id !== selectedFacilityDeviceID) setFacilityPlayStatus('');
      selectedFacilityDeviceID = device.id;
      facilitySelectionStatus.textContent = '';
      setFacilityFeedback(facilitySelectionError);
      renderFacilityWorkspace();
      rowByFacilityIdentity('device', device.id)?.focus({ preventScroll: true });
    });
    facilityDeviceList.appendChild(row);
  }
  // The selected device's details stay open when its row is hidden (FR-033).
  document.getElementById('facilityDeviceHiddenSelection').hidden = !selectionHidden;
  if (facility.devices?.length && !facilityDeviceList.children.length) {
    facilityEmptyList(facilityDeviceList, 'Ни одно устройство не подходит под фильтр и поиск.');
    const reset = document.createElement('button');
    reset.type = 'button';
    reset.className = 'btn btn-secondary';
    reset.id = 'btnFacilityDeviceFilterReset';
    reset.textContent = 'ВСЕ';
    reset.addEventListener('click', resetFacilityDeviceFilter);
    facilityDeviceList.appendChild(reset);
  }
  facilityEmptyList(facilityDeviceList, 'Добавьте первое устройство, затем задайте его состояния и переходы.');
  renderFacilityTerminalFaults(facility);

  facilityConditionList.replaceChildren();
  for (const condition of facility.conditions || []) {
    if (selectedFacilityDeviceID && condition.device?.deviceId !== selectedFacilityDeviceID) continue;
    const scope = facilityDeviceByID(condition.device?.deviceId)?.name
      || state.session.terminals.find(value => value.id === condition.terminal?.terminalId)?.name || 'Вся сессия';
    const row = facilityListRow(
      'condition', condition.id, condition.name, `${facilityConditionActivity(condition)} · ${scope}`,
    );
    row.setAttribute('aria-haspopup', 'dialog');
    row.addEventListener('click', () => openFacilityConditionDialog(condition.id));
    facilityConditionList.appendChild(row);
  }
  facilityEmptyList(facilityConditionList, 'Неисправности не настроены для выбранного устройства.');

  facilityRecoveryProgramList.replaceChildren();
  for (const program of facility.recoveryPrograms || []) {
    if (selectedFacilityDeviceID && !program.transitions?.some(value => value.deviceId === selectedFacilityDeviceID)) continue;
    const row = facilityListRow(
      'recovery-program', program.id, program.name, (program.transitions || []).map(facilityTransitionSummary).join('\n'),
    );
    facilityRowActions(row, program.name, () => openFacilityRecoveryProgramDialog(program.id), () => removeFacilityProgram(program));
    facilityRecoveryProgramList.appendChild(row);
  }
  facilityEmptyList(facilityRecoveryProgramList, 'Программы восстановления не настроены.');

  facilityActionBindingList.replaceChildren();
  facilityInformationBindingList.replaceChildren();
  for (const binding of bindings) {
    if (selectedFacilityDeviceID && !facilityBindingUsesDevice(binding, selectedFacilityDeviceID)) continue;
    const id = `${binding.terminal.id}/${binding.block?.id || binding.node.id}/${binding.property}/${binding.index}`;
    const row = facilityListRow(
      'binding', id, `${facilityNodePath(binding.terminal, binding.node.id)}${binding.block ? ` › Блок ${binding.node.blocks.indexOf(binding.block) + 1}` : ''}`,
      facilityBindingSummary(binding),
    );
    // A command's action, program-backed or not, opens «ДЕЙСТВИЕ КОМАНДЫ» (R7).
    const action = Array.isArray(binding.requests);
    facilityRowActions(row, `${binding.node.name} · ${FACILITY_BINDING_PURPOSES[binding.property]}`,
      event => (action
        ? openFacilityCommandActionDialog(binding.terminal.id, binding.node.id, { opener: event.currentTarget })
        : openFacilityBindingDialog(binding)),
      () => {
        const key = facilityBindingKey(binding);
        commitFacilityEdit(`${action ? 'Действие команды' : 'Правило'} «${binding.node.name}»`,
          session => removeFacilityBinding(session, key));
      }, action ? 'facilityCommandActionDialog' : '');
    (action ? facilityActionBindingList : facilityInformationBindingList).appendChild(row);
  }
  renderFacilityDetailTabs();
  facilityEmptyList(facilityActionBindingList, 'Команды ещё не связаны. Откройте шаг 2 «Действие» в инструкции.');
  facilityEmptyList(facilityInformationBindingList, 'Правила отображения ещё не настроены. Откройте шаг 3 «Результат» в инструкции.');

  const selected = facilityDeviceByID(selectedFacilityDeviceID);
  // A device removed by an edit, an undo, or a live change clears the selection.
  if (selectedFacilityDeviceID && !selected) selectedFacilityDeviceID = '';
  facilitySelectionEditor.hidden = !selected;
  document.getElementById('facilitySelectionEmpty').hidden = Boolean(selected);
  if (selected) {
    renderFacilityDeviceHeader(selected, facility, bindings);
    renderFacilityPlayCard(selected, facility);
  }
  if (!selected) closeFacilityDeviceMenu();
  explainControl(btnPreviewFacilityDevice, controlBlockReason(btnPreviewFacilityDevice));
  if (facilityConditionDialog.open) renderFacilityFaultDraft();
}

// renderFacilityDeviceHeader shows the selected device's identity and live
// state first in the detail area (FR-017).
function renderFacilityDeviceHeader(device, facility, bindings) {
  const kind = facilityDeviceKindLabel(device);
  const terminals = new Set(bindings.filter(value => facilityBindingUsesDevice(value, device.id)).map(value => value.terminal.id));
  const activeFaults = facilityActiveFaultCount(facility, device.id);
  const set = (id, text, data = {}) => {
    const element = document.getElementById(id);
    element.textContent = text;
    Object.assign(element.dataset, data);
  };
  set('facilityDeviceHeaderName', device.name);
  set('facilityDeviceHeaderKind', kind.toLocaleUpperCase('ru'), { kind: device.kind });
  set('facilityDeviceHeaderCurrent', `СЕЙЧАС: ${facilityStateName(device, device.currentStateId)}`, { stateId: device.currentStateId });
  set('facilityDeviceHeaderInitial', `В НАЧАЛЕ: ${facilityStateName(device, device.initialStateId)}`, { stateId: device.initialStateId });
  set('facilityDeviceHeaderFaults', `АКТИВНЫХ НЕИСПРАВНОСТЕЙ: ${activeFaults}`, { count: String(activeFaults) });
  set('facilityDeviceHeaderTerminals', `ТЕРМИНАЛЫ: ${terminals.size}`, { count: String(terminals.size) });
  const technical = document.getElementById('facilityDeviceHeaderTechnical');
  technical.replaceChildren();
  facilityTechnicalDetails(technical, `${device.id} · ${device.kind} · ${device.currentStateId}`);
}

// facilityTransitionVerdict applies the server's order to one transition: the
// source state first, then each precondition in authored order. It drives the
// display only; the server's result decides (FR-002, SC-002).
function facilityTransitionVerdict(facility, deviceID, transition) {
  const devices = new Map((facility.devices || []).map(device => [device.id, device]));
  const preconditions = (transition.preconditions || []).map(precondition => ({
    ...precondition,
    met: devices.get(precondition.deviceId)?.currentStateId === precondition.stateId,
  }));
  const sourceMatches = devices.get(deviceID)?.currentStateId === transition.sourceStateId;
  const firstUnmet = preconditions.find(precondition => !precondition.met) || null;
  return { available: sourceMatches && !firstUnmet, sourceMatches, preconditions, firstUnmet };
}

// facilityEqualityLabel reads «<device> = <state>», with raw IDs for anything missing.
function facilityEqualityLabel(equality) {
  const device = facilityDeviceByID(equality.deviceId);
  return `${device?.name || equality.deviceId} = ${facilityStateName(device, equality.stateId)}`;
}

// facilityPlayBlockReason says why no play control can run now, or returns ''.
function facilityPlayBlockReason() {
  if (!state.session || !state.filePath) return 'НЕТ ОТКРЫТОЙ СЕССИИ';
  if (facilityPlayPending || coordinationCommandPending || sessionStateCommandPending) return 'ВЫПОЛНЯЕТСЯ ДРУГАЯ КОМАНДА';
  // An authoring save in flight would make the play change stale.
  if (authoringBusy()) return 'ДОЖДИТЕСЬ СОХРАНЕНИЯ';
  return '';
}

// renderFacilityPlayCard lists the transitions that leave the device's current
// state, with their preconditions and verdicts (FR-001, FR-002).
function renderFacilityPlayCard(device, facility) {
  const blockReason = facilityPlayBlockReason();
  const list = document.getElementById('facilityPlayTransitions');
  list.replaceChildren();
  const transitions = (device.transitions || []).filter(value => value.sourceStateId === device.currentStateId);
  transitions.forEach((transition, index) => {
    const verdict = facilityTransitionVerdict(facility, device.id, transition);
    const item = document.createElement('li');
    item.className = 'facility-play-transition';
    item.dataset.transitionId = transition.id;
    item.dataset.available = String(verdict.available);
    const run = document.createElement('button');
    run.type = 'button';
    run.className = 'btn btn-primary facility-play-transition-run';
    run.textContent = `${transition.name} → ${facilityStateName(device, transition.destinationStateId)}`;
    const reason = document.createElement('span');
    reason.className = 'control-reason';
    reason.id = `facilityPlayTransitionReason${index}`;
    run.setAttribute('aria-describedby', reason.id);
    const preconditions = document.createElement('ul');
    preconditions.className = 'facility-play-preconditions';
    for (const precondition of verdict.preconditions) {
      const row = document.createElement('li');
      row.className = 'facility-play-precondition';
      row.dataset.deviceId = precondition.deviceId;
      row.dataset.stateId = precondition.stateId;
      row.dataset.met = String(precondition.met);
      row.textContent = `${facilityEqualityLabel(precondition)} · ${precondition.met ? 'ВЫПОЛНЕНО' : 'НЕ ВЫПОЛНЕНО'}`;
      preconditions.appendChild(row);
    }
    if (!verdict.preconditions.length) {
      const row = document.createElement('li');
      row.className = 'facility-play-precondition';
      row.textContent = 'БЕЗ ПРЕДУСЛОВИЙ';
      preconditions.appendChild(row);
    }
    item.append(run, reason, preconditions);
    list.appendChild(item);
    const unmet = verdict.firstUnmet && `НЕ ВЫПОЛНЕНО: ${facilityEqualityLabel(verdict.firstUnmet)}`;
    explainControl(run, blockReason || unmet || '');
  });
  list.hidden = !transitions.length;
  document.getElementById('facilityPlayTransitionsEmpty').hidden = Boolean(transitions.length);
  explainControl(btnForceFacilityDeviceState,
    blockReason || (device.states.length < 2 ? 'У УСТРОЙСТВА ОДНО СОСТОЯНИЕ' : ''));
  renderFacilityPlayFaults(device, facility, blockReason);
  explainControl(btnResetFacilityDevice, blockReason);
  renderFacilityPlayFeed();
}

const FACILITY_REFUSAL_TEXT = Object.freeze({
  declined: 'Смотрителем',
  'precondition-failed': 'предусловие не выполнено',
  'transition-unavailable': 'переход больше недоступен',
  'fault-blocked': 'мешает неисправность',
  stale: 'сцена изменилась',
  withdrawn: 'отозван после действия Смотрителя',
  failed: 'не удалось выполнить',
  unspecified: 'причина неизвестна',
});

// renderFacilityPlayFeed lists the selected device's recent changes in this
// run, newest first. Entries carry IDs; names come from the current session,
// and a missing one shows its ID (FR-012 to FR-014).
function renderFacilityPlayFeed() {
  const list = document.getElementById('facilityPlayFeedList');
  const device = facilityDeviceByID(selectedFacilityDeviceID, state.session);
  const changes = (state.coordination?.facilityChanges || [])
    .filter(change => change.deviceId === device?.id)
    .sort((left, right) => right.sequence - left.sequence)
    .slice(0, 5);
  list.replaceChildren(...changes.map(change => facilityFeedEntry(device, change)));
  list.hidden = !changes.length;
  document.getElementById('facilityPlayFeedEmpty').hidden = Boolean(changes.length);
}

function facilityFeedEntry(device, change) {
  const item = document.createElement('li');
  item.className = 'facility-feed-entry';
  item.dataset.sequence = String(change.sequence);
  item.dataset.kind = change.kind;
  item.dataset.author = change.author;
  if (change.kind === 'player-refusal') item.dataset.reason = change.reason;
  const at = new Date(change.occurredAtUnixMs);
  const time = document.createElement('time');
  time.dateTime = at.toISOString();
  time.textContent = `${String(at.getHours()).padStart(2, '0')}:${String(at.getMinutes()).padStart(2, '0')}`;
  item.append(time, ` · ${facilityFeedText(device, change)}`);
  return item;
}

function facilityFeedText(device, change) {
  const stateName = id => facilityStateName(device, id);
  const move = `${stateName(change.fromStateId)} → ${stateName(change.toStateId)}`;
  const terminal = state.session?.terminals?.find(value => value.id === change.terminalId);
  const command = terminal && locateNode(terminal.root, change.commandId)?.node;
  const commandName = `«${command?.name || change.commandId}»`;
  const author = change.author === 'terminal' ? terminal?.name || change.terminalId : 'Смотритель';
  const restored = change.fromStateId === change.toStateId ? 'неисправности восстановлены' : move;
  switch (change.kind) {
    case 'transition': {
      const transition = device.transitions?.find(value => value.id === change.transitionId);
      return `«${transition?.name || change.transitionId}»: ${move} · ${author}`;
    }
    case 'forced':
      return `принудительно: ${move} · ${author}`;
    case 'fault': {
      const fault = state.session?.facility?.conditions?.find(value => value.id === change.conditionId);
      return `неисправность «${fault?.name || change.conditionId}» ${change.active ? 'активирована' : 'снята'} · ${author}`;
    }
    case 'reset':
      return `сброс к начальному: ${restored} · ${author}`;
    case 'scene-reset':
      return `сброс сцены: ${restored} · ${author}`;
    case 'player-command':
      return `${commandName}: ${move} · ${author}`;
    default:
      return `${commandName} · запрос игрока отклонён: ${FACILITY_REFUSAL_TEXT[change.reason] || FACILITY_REFUSAL_TEXT.unspecified} · ${author}`;
  }
}

// renderFacilityPlayFaults lists every fault scoped to the device with a
// switch. The operator-recovery setting never blocks it (FR-009, FR-010).
function renderFacilityPlayFaults(device, facility, blockReason) {
  const list = document.getElementById('facilityPlayFaults');
  list.replaceChildren();
  const faults = (facility.conditions || []).filter(value => value.device?.deviceId === device.id);
  faults.forEach((fault, index) => {
    const { item, toggle, reason } = facilityFaultSwitchItem(fault, 'facility-play-fault', `facilityPlayFaultReason${index}`);
    const initial = document.createElement('span');
    initial.className = 'facility-play-fault-initial';
    initial.textContent = fault.initialActive ? 'В НАЧАЛЕ ИГРЫ: НЕИСПРАВНОСТЬ АКТИВНА' : 'В НАЧАЛЕ ИГРЫ: ИСПРАВНО';
    item.append(initial, reason);
    list.appendChild(item);
    explainControl(toggle, blockReason);
  });
  list.hidden = !faults.length;
  document.getElementById('facilityPlayFaultsEmpty').hidden = Boolean(faults.length);
}

// renderFacilityTerminalFaults lists every terminal-scoped fault under the
// device list, whatever the selection, search, or filter (FR-021).
function renderFacilityTerminalFaults(facility) {
  const blockReason = facilityPlayBlockReason();
  const list = document.getElementById('facilityTerminalFaultList');
  list.replaceChildren();
  const faults = (facility.conditions || []).filter(value => value.terminal);
  faults.forEach((fault, index) => {
    const { item, toggle, reason } = facilityFaultSwitchItem(fault, 'facility-terminal-fault', `facilityTerminalFaultReason${index}`);
    item.dataset.facilityKind = 'terminal-condition';
    item.dataset.facilityId = fault.id;
    const terminal = document.createElement('span');
    terminal.className = 'facility-terminal-fault-terminal';
    terminal.textContent = `ТЕРМИНАЛ: ${terminalName(fault.terminal.terminalId)}`;
    const open = document.createElement('button');
    open.type = 'button';
    open.className = 'btn btn-mini btn-secondary facility-terminal-fault-open';
    open.textContent = 'ИЗМЕНИТЬ';
    open.setAttribute('aria-label', `Изменить неисправность: ${fault.name}`);
    open.setAttribute('aria-haspopup', 'dialog');
    item.append(terminal, open, reason);
    list.appendChild(item);
    explainControl(toggle, blockReason);
  });
  list.hidden = !faults.length;
  document.getElementById('facilityTerminalFaultsEmpty').hidden = Boolean(faults.length);
}

// facilityFaultSwitchItem builds a fault's list item with the 040 switch and
// its current position. The caller appends its details, then reason.
function facilityFaultSwitchItem(fault, className, reasonID) {
  const item = document.createElement('li');
  item.className = className;
  item.dataset.conditionId = fault.id;
  item.dataset.active = String(Boolean(fault.currentActive));
  const toggle = document.createElement('button');
  toggle.type = 'button';
  toggle.className = 'btn btn-secondary facility-play-fault-switch';
  toggle.setAttribute('role', 'switch');
  toggle.setAttribute('aria-checked', String(Boolean(fault.currentActive)));
  toggle.textContent = fault.name;
  const reason = document.createElement('span');
  reason.className = 'control-reason';
  reason.id = reasonID;
  toggle.setAttribute('aria-describedby', reason.id);
  const current = document.createElement('span');
  current.className = 'facility-play-fault-current';
  current.textContent = fault.currentActive ? 'АКТИВНА' : 'НЕАКТИВНА';
  item.append(toggle, current);
  return { item, toggle, reason };
}

// runFacilityFaultSwitch turns a fault on or off. A terminal fault reports to
// its group, which stays visible without a selected device (FR-021).
function runFacilityFaultSwitch(conditionID, active) {
  const fault = state.session?.facility?.conditions?.find(value => value.id === conditionID);
  const statusID = fault?.terminal ? 'facilityTerminalFaultStatus' : 'facilityPlayStatus';
  return runFacilityPlayChange(() => desktopAPI.setFacilityConditionActive({
    conditionId: conditionID,
    active,
    expectedFacilityRevision: facilityRevision(),
    correlationId: uid('facility-condition-active'),
  }), active ? 'НЕИСПРАВНОСТЬ АКТИВИРОВАНА' : 'НЕИСПРАВНОСТЬ СНЯТА', statusID);
}

function setFacilityPlayStatus(text, outcome, statusID = 'facilityPlayStatus') {
  const status = document.getElementById(statusID);
  status.textContent = text;
  if (outcome) status.dataset.state = outcome;
  else delete status.dataset.state;
}

// runFacilityPlayChange sends one play change and reconciles its result in
// the status statusID. A stale rejection installs the state that won before
// it says so (FR-004).
async function runFacilityPlayChange(call, successMessage, statusID = 'facilityPlayStatus') {
  facilityPlayPending = true;
  setFacilityPlayStatus('', '', statusID);
  renderFacilityWorkspace();
  let result;
  try {
    result = await call();
  } finally {
    facilityPlayPending = false;
  }
  const stale = result?.failure === 'stale-revision' && result.session;
  if ((result?.ok || stale) && result.session) applyLiveSession(result.session, result.sessionRevision);
  if (result?.ok) {
    setFacilityPlayStatus(successMessage, 'ok', statusID);
  } else {
    setFacilityPlayStatus('', stale ? 'stale' : 'error', statusID);
    renderOverseerError(document.getElementById(statusID), result, 'facility');
  }
  renderAll();
  return result;
}

// facilityForceRequest is captured when the forced-change dialog opens, so a
// scene that changed while it was open is rejected as stale (FR-007, FR-008).
let facilityForceRequest = null;

function openFacilityForceStateDialog() {
  const device = facilityDeviceByID(selectedFacilityDeviceID, state.session);
  if (!device) return;
  facilityForceRequest = {
    deviceId: device.id, fromStateId: device.currentStateId, expectedFacilityRevision: facilityRevision(),
  };
  facilityForceStateTarget.replaceChildren(...device.states
    .filter(value => value.id !== device.currentStateId)
    .map(value => new Option(value.name, value.id)));
  renderFacilityForceStateSummary();
  setFacilityFeedback(facilityForceStateError);
  btnConfirmFacilityForceState.disabled = false;
  facilityOperationOpener = btnForceFacilityDeviceState;
  showFacilityDialog(facilityForceStateDialog, facilityForceStateTarget);
}

function renderFacilityForceStateSummary() {
  const device = facilityDeviceByID(facilityForceRequest?.deviceId, state.session);
  if (!device) return;
  facilityForceStateSummary.textContent = `${device.name}: ${facilityStateName(device, facilityForceRequest.fromStateId)} → ${facilityStateName(device, facilityForceStateTarget.value)}`;
}

function closeFacilityForceStateDialog() {
  facilityForceRequest = null;
  closeFacilityConfirmation(facilityForceStateDialog);
}

async function confirmFacilityForceState() {
  const request = facilityForceRequest;
  if (!request || btnConfirmFacilityForceState.disabled) return;
  btnConfirmFacilityForceState.disabled = true;
  const result = await runFacilityPlayChange(() => desktopAPI.forceFacilityDeviceState({
    deviceId: request.deviceId,
    stateId: facilityForceStateTarget.value,
    expectedFacilityRevision: request.expectedFacilityRevision,
    correlationId: uid('facility-device-force'),
  }), 'СОСТОЯНИЕ ИЗМЕНЕНО ПРИНУДИТЕЛЬНО');
  // A non-stale failure keeps the dialog open with its reason.
  if (result?.ok || result?.failure === 'stale-revision') {
    closeFacilityForceStateDialog();
    return;
  }
  btnConfirmFacilityForceState.disabled = false;
  setFacilityFailure(facilityForceStateError, result);
}

function runFacilityTransition(deviceID, transitionID) {
  return runFacilityPlayChange(() => desktopAPI.applyFacilityDeviceTransition({
    deviceId: deviceID,
    transitionId: transitionID,
    expectedFacilityRevision: facilityRevision(),
    correlationId: uid('facility-device-transition'),
  }), 'ПЕРЕХОД ВЫПОЛНЕН');
}

// facilityDeviceRow shows a device's name over its type, its current state as
// a badge framed when it differs from the starting state, and, when any are
// active, its fault count «● 1» (FR-030).
function facilityDeviceRow(device, facility) {
  const activeFaults = facilityActiveFaultCount(facility, device.id);
  const row = facilityListRow('device', device.id, device.name);
  row.dataset.currentStateId = device.currentStateId;
  row.dataset.activeFaults = String(activeFaults);
  const text = document.createElement('span');
  text.className = 'facility-device-row-text';
  const kind = document.createElement('span');
  kind.className = 'facility-device-row-kind';
  kind.textContent = facilityDeviceKindLabel(device);
  text.append(row.querySelector('strong'), kind);
  const badge = document.createElement('span');
  badge.className = 'facility-state-badge';
  badge.dataset.changed = String(device.currentStateId !== device.initialStateId);
  badge.textContent = facilityStateName(device, device.currentStateId);
  row.append(text, badge);
  if (activeFaults) {
    const mark = document.createElement('span');
    mark.className = 'facility-fault-mark';
    mark.setAttribute('aria-label', `Активных неисправностей: ${activeFaults}`);
    mark.textContent = `● ${activeFaults}`;
    row.appendChild(mark);
  }
  return row;
}

// facilityDeviceFilters keeps, per device list filter, the devices it lists
// (FR-031).
const facilityDeviceFilters = {
  all: () => true,
  changed: device => device.currentStateId !== device.initialStateId,
  faulty: (device, facility) => facilityActiveFaultCount(facility, device.id) > 0,
};

// renderFacilityDeviceFilters presses the current filter and counts each
// filter's devices over the whole session, ignoring the search (FR-031).
function renderFacilityDeviceFilters(facility) {
  for (const button of document.querySelectorAll('#facilityDeviceFilters [data-filter]')) {
    const filter = button.dataset.filter;
    const count = (facility.devices || []).filter(device => facilityDeviceFilters[filter](device, facility)).length;
    button.setAttribute('aria-pressed', String(filter === facilityDeviceFilter));
    button.dataset.count = String(count);
    button.querySelector('.facility-filter-count').textContent = String(count);
  }
}

// resetFacilityDeviceFilter lists every device again from the empty list.
function resetFacilityDeviceFilter() {
  facilityDeviceFilter = 'all';
  document.getElementById('facilityDeviceSearch').value = '';
  renderFacilityWorkspace();
  document.getElementById('facilityDeviceFilterAll').focus();
}

const facilityDetailTabs = [
  ['actions', 'facilityTabActions', 'facilityTabPanelActions', () => facilityActionBindingList],
  ['information', 'facilityTabInformation', 'facilityTabPanelInformation', () => facilityInformationBindingList],
  ['conditions', 'facilityTabConditions', 'facilityTabPanelConditions', () => facilityConditionList],
  ['programs', 'facilityTabPrograms', 'facilityTabPanelPrograms', () => facilityRecoveryProgramList],
];

// renderFacilityDetailTabs shows only the open tab's panel and counts each
// tab's rows under the current selection (FR-024). Call it before the empty
// placeholders are added.
function renderFacilityDetailTabs() {
  for (const [id, tabID, panelID, list] of facilityDetailTabs) {
    const tab = document.getElementById(tabID);
    const open = id === facilityDetailTab;
    const count = list().querySelectorAll(':scope > [data-facility-kind]').length;
    tab.setAttribute('aria-selected', String(open));
    tab.tabIndex = open ? 0 : -1;
    tab.dataset.count = String(count);
    tab.querySelector('.facility-tab-count').textContent = String(count);
    document.getElementById(panelID).hidden = !open;
  }
}

function selectFacilityDetailTab(id, focus = false) {
  facilityDetailTab = id;
  renderFacilityDetailTabs();
  if (focus) document.getElementById(facilityDetailTabs.find(([tab]) => tab === id)[1]).focus();
}

function openFacilityDeviceMenu() {
  facilityDeviceMenu.hidden = false;
  btnFacilityDeviceMenu.setAttribute('aria-expanded', 'true');
  btnDeleteFacilityDevice.focus();
}

function closeFacilityDeviceMenu(restoreFocus = false) {
  if (facilityDeviceMenu.hidden) return;
  facilityDeviceMenu.hidden = true;
  btnFacilityDeviceMenu.setAttribute('aria-expanded', 'false');
  if (restoreFocus) btnFacilityDeviceMenu.focus();
}

function facilityDeviceKindLabel(device) {
  return [...facilityDeviceKind.options].find(option => option.value === device.kind)?.textContent || device.kind;
}

// facilityActiveFaultCount counts the device-scoped faults that are active now.
function facilityActiveFaultCount(facility, deviceID) {
  return (facility.conditions || []).filter(value => value.device?.deviceId === deviceID && value.currentActive).length;
}

// controlBlockReason says why a reset or preview control cannot run now, or
// returns '' (FR-005).
function controlBlockReason(control) {
  if (!state.session || !state.filePath) return 'НЕТ ОТКРЫТОЙ СЕССИИ';
  if (coordinationCommandPending || sessionStateCommandPending) return 'ВЫПОЛНЯЕТСЯ ДРУГАЯ КОМАНДА';
  // A scene reset counts against the saved session; a preview waits instead.
  if (control === btnResetScene && authoringBusy()) return 'ДОЖДИТЕСЬ СОХРАНЕНИЯ';
  if (control === btnResetScene && sceneResetCounts(state.session).total === 0) return 'СЦЕНА УЖЕ В НАЧАЛЬНОМ СОСТОЯНИИ';
  return '';
}

function rowByFacilityIdentity(kind, id) {
  return document.querySelector(`[data-facility-kind="${CSS.escape(kind)}"][data-facility-id="${CSS.escape(id)}"]`);
}

function selectWorkspace(kind) {
  const facilitySelected = kind === 'facility';
  terminalWorkspace.hidden = facilitySelected;
  terminalWorkspace.style.display = facilitySelected ? 'none' : 'flex';
  facilityWorkspace.hidden = !facilitySelected;
  terminalWorkspaceTab.setAttribute('aria-selected', String(!facilitySelected));
  facilityWorkspaceTab.setAttribute('aria-selected', String(facilitySelected));
  terminalWorkspaceTab.tabIndex = facilitySelected ? -1 : 0;
  facilityWorkspaceTab.tabIndex = facilitySelected ? 0 : -1;
  if (facilitySelected) renderFacilityWorkspace();
}

function facilityIssueMessage(result) {
  return overseerErrorMessage(result, 'facility').message;
}


function facilityRevision() {
  return Number(state.session?.facility?.revision || 0);
}

function facilityPreviewNodeMap(node, result = new Map()) {
  if (!node || typeof node !== 'object') return result;
  result.set(node.id, node);
  for (const child of node.children || []) facilityPreviewNodeMap(child, result);
  return result;
}

const PREVIEW_DIFFERENCE_LABELS = {
  'name-changed': 'ПЕРЕИМЕНУЕТСЯ',
  'text-changed': 'ТЕКСТ ИЗМЕНИТСЯ',
  appears: 'ПОЯВИТСЯ',
  disappears: 'ИСЧЕЗНЕТ',
  'becomes-available': 'СТАНЕТ ДОСТУПНО',
  'becomes-unavailable': 'СТАНЕТ НЕДОСТУПНО',
};

// renderFacilityPreviewNode draws one projected node; kinds maps node IDs to
// the difference kinds that mark them in both panes, in summary order.
function renderFacilityPreviewNode(node, parent, kinds) {
  if (!node || typeof node !== 'object') return;
  const item = document.createElement('section');
  item.className = 'facility-preview-node';
  item.dataset.previewNodeId = node.id || '';
  const nodeKinds = kinds.get(node.id) || [];
  const heading = document.createElement('strong');
  heading.textContent = node.id === 'root' ? ROOT_FOLDER_LABEL : (node.name || node.id || '—');
  item.appendChild(heading);
  if (nodeKinds.length) item.dataset.kind = nodeKinds.join(' ');
  for (const kind of nodeKinds) {
    const status = document.createElement('span');
    status.className = 'facility-preview-node-status';
    status.textContent = PREVIEW_DIFFERENCE_LABELS[kind];
    item.appendChild(status);
  }
  const texts = node.description ? [node.description] : (node.blocks || []).map(block => block.initialText || '');
  for (const value of texts) {
    const text = document.createElement('p');
    text.textContent = value;
    item.appendChild(text);
  }
  parent.appendChild(item);
  for (const child of node.children || []) renderFacilityPreviewNode(child, item, kinds);
}

function clearFacilityPreview(summary = '') {
  facilityPreviewSummary.textContent = summary;
  facilityPreviewCurrent.replaceChildren();
  facilityPreviewHypothetical.replaceChildren();
}

function closeFacilityPreview() {
  const opener = facilityOperationOpener;
  facilityPreviewGeneration++;
  facilityPreviewSelection = null;
  facilityPreviewHypothetical.removeAttribute('aria-busy');
  facilityOperationOpener = null;
  hideFacilityDialog(facilityPreviewDialog, opener);
}

// openFacilityPreview previews the selected device, or the fault open in the
// fault dialog.
function openFacilityPreview(kind, opener) {
  const device = facilityDeviceByID(selectedFacilityDeviceID, state.session);
  const condition = state.session?.facility?.conditions?.find(value => value.id === editingFacilityConditionID) || null;
  if ((kind === 'device' && !device) || (kind === 'condition' && !condition)) return;
  facilityPreviewSelection = { kind, id: kind === 'device' ? device.id : condition.id };
  facilityOperationOpener = opener;
  populateSelect(facilityPreviewTerminal,
    (state.session?.terminals || []).map(terminal => ({ id: terminal.id, name: terminal.name })),
    state.session?.terminals?.[0]?.id || '', 'ВЫБЕРИТЕ ТЕРМИНАЛ');
  const deviceMode = kind === 'device';
  document.getElementById('facilityPreviewDialogTitle').textContent =
    deviceMode ? 'ПРЕДПРОСМОТР УСТРОЙСТВА' : 'ПРЕДПРОСМОТР НЕИСПРАВНОСТИ';
  facilityPreviewStateLabel.hidden = !deviceMode;
  facilityPreviewState.hidden = !deviceMode;
  facilityPreviewActiveLabel.hidden = deviceMode;
  if (deviceMode) {
    const different = (device.states || []).find(value => value.id !== device.currentStateId);
    populateSelect(facilityPreviewState, device.states || [], different?.id || device.currentStateId);
  } else {
    facilityPreviewActive.checked = !condition.currentActive;
  }
  clearFacilityPreview();
  facilityPreviewStatus.textContent = '';
  setFacilityFeedback(facilityPreviewError);
  showFacilityDialog(facilityPreviewDialog, deviceMode ? facilityPreviewState : facilityPreviewActive);
  void refreshFacilityPreview();
}

// refreshFacilityPreview asks, once the save queue is idle, for the current
// and the previewed projection of the applied configuration in one request;
// a stale authoring revision is retried once (FR-016–FR-020).
async function refreshFacilityPreview(retried = false) {
  if (!facilityPreviewSelection || !facilityPreviewTerminal.value) return;
  const generation = ++facilityPreviewGeneration;
  const selection = { ...facilityPreviewSelection };
  setFacilityFeedback(facilityPreviewError);
  const payload = { terminalId: facilityPreviewTerminal.value };
  let currentLabel;
  let previewLabel;
  if (selection.kind === 'device') {
    const device = facilityDeviceByID(selection.id, state.session);
    if (!device) return;
    if ((device.states || []).length < 2) {
      facilityPreviewStatus.textContent = '';
      clearFacilityPreview('НЕЧЕГО СРАВНИВАТЬ: У УСТРОЙСТВА ОДНО СОСТОЯНИЕ');
      return;
    }
    currentLabel = `${device.name}: ${facilityStateName(device, device.currentStateId)}`;
    previewLabel = `${device.name}: ${facilityStateName(device, facilityPreviewState.value)}`;
    payload.deviceState = { deviceId: selection.id, stateId: facilityPreviewState.value };
  } else {
    const condition = state.session?.facility?.conditions?.find(value => value.id === selection.id);
    if (!condition) return;
    const activity = active => (active ? 'Активно' : 'Неактивно');
    currentLabel = `${condition.name}: ${activity(condition.currentActive)}`;
    previewLabel = `${condition.name}: ${activity(facilityPreviewActive.checked)}`;
    payload.condition = { conditionId: selection.id, active: facilityPreviewActive.checked };
  }
  facilityPreviewStatus.textContent = 'ПОДГОТОВКА ПРЕДПРОСМОТРА…';
  facilityPreviewHypothetical.setAttribute('aria-busy', 'true');
  await authoringIdle();
  if (generation !== facilityPreviewGeneration) return;
  payload.expectedFacilityRevision = facilityRevision();
  payload.expectedAuthoringRevision = authoring.revision;
  const result = await desktopAPI.previewFacility(payload);
  if (generation !== facilityPreviewGeneration || facilityPreviewSelection?.kind !== selection.kind
      || facilityPreviewSelection.id !== selection.id) return;
  facilityPreviewHypothetical.removeAttribute('aria-busy');
  if (!result.ok && result.failure === 'stale-revision' && !retried) {
    void refreshFacilityPreview(true);
    return;
  }
  if (!result.ok || !result.terminal || !result.current) {
    facilityPreviewStatus.textContent = '';
    setFacilityFailure(facilityPreviewError, result);
    return;
  }
  const kinds = new Map();
  for (const { nodeId, kind } of result.differences) {
    const nodeKinds = kinds.get(nodeId) || [];
    if (!nodeKinds.includes(kind)) kinds.set(nodeId, [...nodeKinds, kind]);
  }
  document.getElementById('facilityPreviewCurrentHeading').textContent = `СЕЙЧАС: ${currentLabel}`;
  document.getElementById('facilityPreviewHypotheticalHeading').textContent = `ЕСЛИ: ${previewLabel}`;
  clearFacilityPreview();
  renderFacilityPreviewNode(result.current.tree, facilityPreviewCurrent, kinds);
  renderFacilityPreviewNode(result.terminal.tree, facilityPreviewHypothetical, kinds);
  const nodes = new Map([...facilityPreviewNodeMap(result.current.tree), ...facilityPreviewNodeMap(result.terminal.tree)]);
  if (!result.differences.length) {
    facilityPreviewSummary.textContent = 'ИГРОКИ НЕ УВИДЯТ ИЗМЕНЕНИЙ';
  } else {
    const heading = document.createElement('div');
    heading.textContent = `ИЗМЕНЕНИЙ: ${result.differences.length}`;
    const list = document.createElement('ul');
    // The player projection joins an entry's blocks into one text, so block
    // numbers come from the authored terminal.
    const authored = state.session?.terminals?.find(terminal => terminal.id === payload.terminalId);
    for (const difference of result.differences) {
      const node = nodes.get(difference.nodeId);
      const block = difference.blockId ? entryBlockLocation(authored, difference.blockId) : null;
      const item = document.createElement('li');
      item.dataset.kind = difference.kind;
      item.textContent = `${PREVIEW_DIFFERENCE_LABELS[difference.kind] || difference.kind}: ${node?.name || difference.nodeId}`
        + (block ? ` · блок ${block.index + 1}` : '');
      list.appendChild(item);
    }
    facilityPreviewSummary.replaceChildren(heading, list);
  }
  facilityPreviewStatus.textContent = 'ПРЕДПРОСМОТР ГОТОВ · МИР ДЛЯ ИГРОКОВ НЕ ИЗМЕНЁН';
}

function openFacilityConfirmation(dialog, opener, focusTarget) {
  const description = dialog.querySelector('p');
  const device = facilityDeviceByID(selectedFacilityDeviceID, state.session);
  if (dialog === facilityDeviceResetDialog && device) {
    description.textContent = `${device.name}: ${facilityStateName(device, device.currentStateId)} → ${facilityStateName(device, device.initialStateId)}. Неисправности этого устройства также вернутся к начальным значениям. Изменение сразу сохранится и станет видно игрокам.`;
  }
  facilityOperationOpener = opener;
  showFacilityDialog(dialog, focusTarget);
}

function closeFacilityConfirmation(dialog) {
  const opener = facilityOperationOpener;
  facilityOperationOpener = null;
  hideFacilityDialog(dialog, opener);
}

async function resetSelectedFacilityDevice() {
  const deviceID = selectedFacilityDeviceID;
  closeFacilityConfirmation(facilityDeviceResetDialog);
  await runFacilityPlayChange(() => desktopAPI.resetFacilityDevice({
    deviceId: deviceID,
    expectedFacilityRevision: facilityRevision(),
    correlationId: uid('facility-device-reset'),
  }), 'УСТРОЙСТВО СБРОШЕНО');
}

// ── Scene reset (FR-014) ────────────────────────────────────
let sceneResetCounted = null;
let sceneResetSubmitting = false;

// sceneResetCounts counts what a scene reset changes in session.
function sceneResetCounts(session) {
  const facility = session?.facility;
  const devices = (facility?.devices || []).filter(device => device.currentStateId !== device.initialStateId).length;
  const faults = (facility?.conditions || [])
    .filter(condition => Boolean(condition.currentActive) !== Boolean(condition.initialActive)).length;
  const terminals = (session?.terminals || [])
    .map(terminal => ({ name: terminal.name, count: Object.keys(terminal.commandStates || {}).length }))
    .filter(terminal => terminal.count > 0);
  const commands = terminals.reduce((sum, terminal) => sum + terminal.count, 0);
  return { devices, faults, commands, terminals, total: devices + faults + commands };
}

// renderSceneResetDialog lists the counts and records the revisions they
// were counted against.
function renderSceneResetDialog(sessionRevision = currentSessionRevision(), facilityRevisionValue = facilityRevision()) {
  const counts = sceneResetCounts(state.session);
  const perTerminal = counts.terminals.map(terminal => `${terminal.name}: ${terminal.count}`).join(', ');
  sceneResetCountsList.replaceChildren(...[
    `УСТРОЙСТВА: ${counts.devices}`,
    `НЕИСПРАВНОСТИ: ${counts.faults}`,
    `ВЫПОЛНЕННЫЕ КОМАНДЫ: ${counts.commands}${perTerminal ? ` (${perTerminal})` : ''}`,
  ].map(text => {
    const item = document.createElement('li');
    item.textContent = text;
    return item;
  }));
  renderSceneResetPending();
  sceneResetCounted = { sessionRevision, facilityRevision: facilityRevisionValue };
}

// renderSceneResetPending says what the reset does to a pending player
// request; it follows coordination changes while the dialog is open.
function renderSceneResetPending() {
  const pending = state.coordination?.pendingCommandExecution;
  if (pending?.requestId) {
    const terminal = state.session?.terminals.find(candidate => candidate.id === pending.terminalId);
    const rejected = Boolean(pending.facilityAction) || Object.hasOwn(terminal?.commandStates || {}, pending.commandId);
    sceneResetPending.textContent = `ОЖИДАЮЩИЙ ЗАПРОС ИГРОКА: ${pending.commandName || pending.commandId} — ${rejected ? 'БУДЕТ ОТКЛОНЁН' : 'ОСТАНЕТСЯ В ОЖИДАНИИ'}`;
  }
  sceneResetPending.hidden = !pending?.requestId;
}

function setSceneResetStatus(message = '') {
  sceneResetStatus.textContent = message;
  sceneResetStatus.hidden = !message;
}

function openSceneResetDialog() {
  if (controlBlocked(btnResetScene)) return;
  setSceneResetStatus();
  renderSceneResetDialog();
  showFacilityDialog(sceneResetDialog, btnCancelSceneReset);
}

function closeSceneResetDialog() {
  if (!sceneResetSubmitting) hideFacilityDialog(sceneResetDialog, btnResetScene);
}

async function confirmSceneReset() {
  if (sceneResetSubmitting || !sceneResetCounted) return;
  sceneResetSubmitting = true;
  btnConfirmSceneReset.disabled = true;
  btnCancelSceneReset.disabled = true;
  const result = await desktopAPI.resetScene({
    expectedSessionRevision: sceneResetCounted.sessionRevision,
    expectedFacilityRevision: sceneResetCounted.facilityRevision,
    correlationId: uid('scene-reset'),
  });
  sceneResetSubmitting = false;
  btnConfirmSceneReset.disabled = false;
  btnCancelSceneReset.disabled = false;
  saveStatus.dataset.wailsCommand = 'ResetScene';
  saveStatus.dataset.wailsResult = result.ok ? 'ok' : 'error';
  saveStatus.dataset.wailsRevision = String(Number(result.sessionRevision || 0));
  delete saveStatus.dataset.wailsTerminalId;
  updateSessionStateEvidenceDescription();
  if (result.ok && result.session) {
    applyLiveSession(result.session, result.sessionRevision);
    closeSceneResetDialog();
    setCoordinationStatus('СЦЕНА СБРОШЕНА');
    renderAll();
    return;
  }
  if (result.failure === 'stale-revision' && result.session) {
    applyLiveSession(result.session, result.sessionRevision);
    renderAll();
    renderSceneResetDialog(result.sessionRevision, result.previousFacilityRevision);
    setSceneResetStatus('СЦЕНА ИЗМЕНИЛАСЬ — ПРОВЕРЬТЕ СПИСОК');
    return;
  }
  renderOverseerError(sceneResetStatus, result, 'facility', { prefix: 'СЦЕНА НЕ СБРОШЕНА:' });
}

// The device dialog keeps one draft in its markup (R8). Each state is a
// .facility-state-row in #facilityDeviceTechnical that holds its identifier,
// with its name in facilityName. Each transition is a .facility-transition-row
// in #facilityTransitionPanel, with its identifier row in facilityIDRow. A
// transition points at its state rows, so it follows a state whose identifier
// changes.
const FACILITY_TRANSITION_ENDS = [
  ['.facility-transition-source', 'facilitySource'],
  ['.facility-transition-destination', 'facilityDestination'],
];

function facilityDraftStateRows() {
  return [...facilityDeviceStateIDs.querySelectorAll('.facility-state-row')];
}

function facilityDraftTransitionRows() {
  return [...facilityTransitionPanel.querySelectorAll('.facility-transition-row')];
}

function facilityDraftStateID(row) {
  return row?.querySelector('.facility-state-id').value.trim() || '';
}

function facilityDraftTransitionID(row) {
  return row.facilityIDRow.querySelector('.facility-transition-id').value.trim();
}

function facilityDraftStateLabel(row) {
  if (!row) return '—';
  return row.facilityName.trim() || 'Без названия';
}

function facilityDraftTransitionLabel(row) {
  return row.querySelector('.facility-transition-name').value.trim() || 'Без названия';
}

function facilityDraftCellRows(source, destination) {
  return facilityDraftTransitionRows().filter(row => row.facilitySource === source
    && row.facilityDestination === destination);
}

// facilityIDField builds one row of «ТЕХНИЧЕСКИЕ ИДЕНТИФИКАТОРЫ». A saved
// identifier is read-only (FR-007); a new one follows its name until the
// author edits it.
function facilityIDField(className, inputClass, id = '') {
  const row = document.createElement('div');
  row.className = className;
  const label = document.createElement('label');
  label.className = 'facility-technical-label';
  const input = document.createElement('input');
  input.className = `field-input ${inputClass}`;
  input.type = 'text';
  input.maxLength = FACILITY_ID_LIMIT;
  input.autocomplete = 'off';
  input.required = true;
  input.value = id;
  input.readOnly = Boolean(id);
  if (id) input.dataset.manual = 'true';
  input.addEventListener('input', () => { input.dataset.manual = 'true'; });
  label.append(document.createElement('span'), input);
  row.appendChild(label);
  return row;
}

// appendFacilityStateRow adds a draft state. A value with an id is a saved state.
function appendFacilityStateRow(value = {}) {
  const row = facilityIDField('facility-state-row', 'facility-state-id', value.id);
  row.facilityOriginal = structuredClone(value);
  row.facilityName = value.name || '';
  row.dataset.initial = 'false';
  facilityDeviceStateIDs.appendChild(row);
  return row;
}

// addFacilityDraftState appends a new state. It becomes the starting state
// when the draft has none (FR-003).
function addFacilityDraftState(name) {
  const starred = facilityDraftStateRows().some(row => row.dataset.initial === 'true');
  const row = appendFacilityStateRow();
  row.dataset.initial = String(!starred);
  renameFacilityDraftState(row, name);
  return row;
}

function renameFacilityDraftState(row, name) {
  row.facilityName = name;
  const input = row.querySelector('.facility-state-id');
  if (input.dataset.manual === 'true') return;
  const existing = new Set(facilityDraftStateRows().filter(other => other !== row)
    .map(facilityDraftStateID).filter(Boolean));
  input.value = suggestFacilityID('state', name, existing, false);
}

function suggestFacilityDraftTransitionID(row) {
  const input = row.facilityIDRow.querySelector('.facility-transition-id');
  if (input.dataset.manual === 'true') return;
  const existing = new Set(facilityDraftTransitionRows().filter(other => other !== row)
    .map(facilityDraftTransitionID).filter(Boolean));
  input.value = suggestFacilityID('transition', row.querySelector('.facility-transition-name').value, existing, false);
}

// appendFacilityTransitionRow adds a draft transition to the panel, hidden
// until it is chosen, and its identifier row to the technical block.
function appendFacilityTransitionRow(value = {}) {
  const row = facilityTransitionEditorTemplate.content.firstElementChild.cloneNode(true);
  row.facilityOriginal = structuredClone(value);
  const states = facilityDraftStateRows();
  row.facilitySource = states.find(state => facilityDraftStateID(state) === value.sourceStateId) || null;
  row.facilityDestination = states.find(state => facilityDraftStateID(state) === value.destinationStateId) || null;
  row.facilityIDRow = facilityIDField('facility-transition-id-row', 'facility-transition-id', value.id);
  const name = row.querySelector('.facility-transition-name');
  name.value = value.name || '';
  name.addEventListener('input', () => suggestFacilityDraftTransitionID(row));
  for (const [selector, end] of FACILITY_TRANSITION_ENDS) {
    const select = row.querySelector(selector);
    select.addEventListener('input', () => { row[end] = facilityDraftStateRows()[select.selectedIndex - 1] || null; });
  }
  const addPrecondition = row.querySelector('.facility-add-precondition');
  addPrecondition.addEventListener('click', () => {
    const precondition = appendFacilityTransitionPrecondition(row);
    renderFacilityDeviceDraft();
    precondition.querySelector('.facility-precondition-device').focus();
  });
  for (const precondition of value.preconditions || []) appendFacilityTransitionPrecondition(row, precondition);
  listFacilityTransitionFaultEffects(row);
  row.querySelector('[data-action="remove-facility-transition"]')
    .addEventListener('click', () => removeFacilityDraftTransition(row));
  facilityTransitionUses.before(row);
  facilityDeviceTransitionIDs.appendChild(row.facilityIDRow);
  return row;
}

// addFacilityDraftTransition appends a new transition between two draft
// states, named after its destination unless name is given.
function addFacilityDraftTransition(source, destination, name = `Перевести в «${facilityDraftStateLabel(destination)}»`) {
  const row = appendFacilityTransitionRow({ preconditions: [], conditionEffects: [] });
  row.facilitySource = source;
  row.facilityDestination = destination;
  row.querySelector('.facility-transition-name').value = name;
  suggestFacilityDraftTransitionID(row);
  return row;
}

// facilityDraftTemplate is the template of the chosen device type. «ДРУГОЙ»
// and no type have none.
function facilityDraftTemplate() {
  return FACILITY_DEVICE_TEMPLATES[facilityDeviceKind.value];
}

// facilityDraftStateNamed finds the draft state with a template state's name.
// Like the server, it compares trimmed names case-insensitively.
function facilityDraftStateNamed(name) {
  return facilityDraftStateRows().find(row => row.facilityName.trim().toLowerCase() === name.toLowerCase());
}

// facilityDraftTemplateAdditions lists what applying template would add: the
// states the draft has no namesake for, and the transitions that no draft
// transition of the same name has between the same named states (R9).
function facilityDraftTemplateAdditions(template) {
  const states = template.states.filter(name => !facilityDraftStateNamed(name));
  const transitions = template.transitions.filter(({ name, from, to }) => {
    const source = facilityDraftStateNamed(from);
    const destination = facilityDraftStateNamed(to);
    return !source || !destination || !facilityDraftCellRows(source, destination)
      .some(row => row.querySelector('.facility-transition-name').value.trim().toLowerCase() === name.toLowerCase());
  });
  return { states, transitions };
}

// facilityDraftTemplateTransitionName is the template's name for a new
// transition between two draft states, matched through their names.
function facilityDraftTemplateTransitionName(source, destination) {
  return facilityDraftTemplate()?.transitions.find(({ from, to }) => facilityDraftStateNamed(from) === source
    && facilityDraftStateNamed(to) === destination)?.name;
}

// applyFacilityDraftTemplate adds the template's missing states and
// transitions, and stars its starting state when the draft has none. Nothing
// is renamed or removed (FR-011).
function applyFacilityDraftTemplate(template) {
  const { states, transitions } = facilityDraftTemplateAdditions(template);
  const starred = facilityDraftStateRows().some(row => row.dataset.initial === 'true');
  for (const name of states) renameFacilityDraftState(appendFacilityStateRow(), name);
  if (!starred) facilityDraftStateNamed(template.initial).dataset.initial = 'true';
  for (const { name, from, to } of transitions) {
    addFacilityDraftTransition(facilityDraftStateNamed(from), facilityDraftStateNamed(to), name);
  }
  facilityDeviceStatus.textContent = `ИЗ ШАБЛОНА ДОБАВЛЕНО: СОСТОЯНИЙ — ${states.length}, ПЕРЕХОДОВ — ${transitions.length}`;
  renderFacilityDeviceDraft();
}

// renderFacilityDeviceTemplate offers the chosen type's template and names
// what it holds (FR-010). The offer is blocked when applying would add
// nothing.
function renderFacilityDeviceTemplate() {
  const template = facilityDraftTemplate();
  facilityDeviceTemplate.hidden = !template;
  if (!template) return;
  document.getElementById('facilityDeviceTemplateLabel').textContent =
    `ШАБЛОН: ${facilityDeviceKind.selectedOptions[0].textContent}`;
  document.getElementById('facilityDeviceTemplateSummary').textContent =
    `${template.states.join(', ')} · ${template.transitions.map(transition => transition.name).join(', ')}`;
  const { states, transitions } = facilityDraftTemplateAdditions(template);
  explainControl(btnApplyFacilityDeviceTemplate, states.length || transitions.length ? '' : 'ВСЁ ИЗ ШАБЛОНА УЖЕ ЕСТЬ');
}

// removeFacilityDraftTransition deletes a transition from the draft, or lists
// the outside uses that refuse the delete under the panel.
function removeFacilityDraftTransition(row) {
  const device = facilityDeviceByID(editingFacilityDeviceID);
  const { id, name } = row.facilityOriginal;
  const uses = device && id ? facilityTransitionOutsideUses(device, new Map([[id, name]])) : [];
  if (uses.length) {
    clearFacilityUses(facilityTransitionUses);
    facilityTransitionUses.facilityTransition = row;
    showFacilityUses(facilityTransitionUses, uses);
    return;
  }
  const { facilitySource: source, facilityDestination: destination } = row;
  row.facilityIDRow.remove();
  row.remove();
  if (source && destination && facilityDraftCellRows(source, destination).length) {
    chooseFacilityTransitionCell(source, destination);
  } else {
    closeFacilityTransitionPanel();
  }
}

// facilityPreconditionDevices lists the devices a precondition of transition
// may name: every other device that no other precondition of the transition
// uses, as the domain requires (validate.go).
function facilityPreconditionDevices(transition, row) {
  const used = new Set([...transition.querySelectorAll('.facility-precondition-row')]
    .filter(other => other !== row)
    .map(other => other.querySelector('.facility-precondition-device').value));
  return authoredFacility().devices
    .filter(device => device.id !== editingFacilityDeviceID && !used.has(device.id))
    .map(device => ({ id: device.id, name: device.name }));
}

function appendFacilityTransitionPrecondition(transition, value = {}) {
  const row = document.createElement('div');
  row.facilityOriginal = structuredClone(value);
  row.className = 'facility-transition-reference-row facility-precondition-row';
  const device = document.createElement('select');
  device.className = 'field-input facility-precondition-device';
  device.setAttribute('aria-label', 'УСТРОЙСТВО ПРЕДУСЛОВИЯ');
  const equals = document.createElement('span');
  equals.textContent = '=';
  const stateSelect = document.createElement('select');
  stateSelect.className = 'field-input facility-precondition-state';
  stateSelect.setAttribute('aria-label', 'СОСТОЯНИЕ ПРЕДУСЛОВИЯ');
  const remove = document.createElement('button');
  remove.className = 'btn btn-danger btn-mini';
  remove.type = 'button';
  remove.textContent = 'УДАЛИТЬ';
  remove.setAttribute('aria-label', 'Удалить предусловие');
  const addPrecondition = transition.querySelector('.facility-add-precondition');
  remove.addEventListener('click', () => {
    row.remove();
    renderFacilityDeviceDraft();
    addPrecondition.focus();
  });
  row.append(device, equals, stateSelect, remove);
  addPrecondition.before(row);
  populateSelect(device, facilityPreconditionDevices(transition, row), value.deviceId || '', 'ВЫБЕРИТЕ УСТРОЙСТВО');
  populateSelect(stateSelect, facilityDeviceByID(device.value)?.states || [], value.stateId || '', 'ВЫБЕРИТЕ СОСТОЯНИЕ');
  return row;
}

function refreshFacilityPreconditionChoices(transition) {
  for (const row of transition.querySelectorAll('.facility-precondition-row')) {
    const device = row.querySelector('.facility-precondition-device');
    const stateSelect = row.querySelector('.facility-precondition-state');
    populateSelect(device, facilityPreconditionDevices(transition, row), device.value, 'ВЫБЕРИТЕ УСТРОЙСТВО');
    populateSelect(stateSelect, facilityDeviceByID(device.value)?.states || [], stateSelect.value, 'ВЫБЕРИТЕ СОСТОЯНИЕ');
  }
}

// listFacilityTransitionFaultEffects shows the faults a transition turns on
// or clears. Only the fault dialog edits them (FR-017).
function listFacilityTransitionFaultEffects(row) {
  const section = row.querySelector('.facility-transition-fault-effects');
  const list = section.querySelector('ul');
  const effects = row.facilityOriginal.conditionEffects || [];
  for (const effect of effects) {
    const item = document.createElement('li');
    item.className = 'facility-transition-fault-effect';
    item.dataset.conditionId = effect.conditionId;
    item.dataset.active = String(effect.active === true);
    const fault = authoredFacility().conditions.find(value => value.id === effect.conditionId);
    const text = document.createElement('span');
    text.textContent = `${effect.active === true ? 'Включает' : 'Снимает'} «${fault?.name || effect.conditionId}»`;
    item.appendChild(text);
    facilityDraftLink(item, 'К НЕИСПРАВНОСТИ', () => openFacilityConditionDialog(effect.conditionId))
      .classList.add('facility-fault-link');
    list.appendChild(item);
  }
  list.hidden = !effects.length;
  section.querySelector('.facility-empty').hidden = Boolean(effects.length);
}

// facilityDraftLink appends a button that leaves the device dialog for
// another editor. renderFacilityDeviceDraft blocks it while the draft has
// unsaved changes.
function facilityDraftLink(parent, label, open) {
  const link = document.createElement('button');
  link.type = 'button';
  link.className = 'btn btn-secondary btn-mini facility-draft-link';
  link.textContent = label;
  const reason = document.createElement('span');
  reason.className = 'control-reason';
  reason.id = uid('facilityDraftLinkReason');
  link.setAttribute('aria-describedby', reason.id);
  link.addEventListener('click', () => {
    if (controlBlocked(link)) return;
    closeFacilityDeviceDialog();
    open();
  });
  parent.append(link, reason);
  return link;
}

// facilityTransitionCellSummary is the line under a transition's name in its
// matrix cell, for example «1 предусловие · снимает 1 неисправность».
function facilityTransitionCellSummary(row) {
  const preconditions = row.querySelectorAll('.facility-precondition-row').length;
  const effects = row.facilityOriginal.conditionEffects || [];
  const activates = effects.filter(effect => effect.active === true).length;
  const clears = effects.length - activates;
  const faults = count => `${count} ${russianPlural(count, 'неисправность', 'неисправности', 'неисправностей')}`;
  return [
    preconditions && `${preconditions} ${russianPlural(preconditions, 'предусловие', 'предусловия', 'предусловий')}`,
    activates && `включает ${faults(activates)}`,
    clears && `снимает ${faults(clears)}`,
  ].filter(Boolean).join(' · ');
}

function facilityTransitionCell(source, destination, rows) {
  const button = document.createElement('button');
  button.type = 'button';
  button.className = 'facility-transition-cell';
  button.facilitySource = source;
  button.facilityDestination = destination;
  button.dataset.from = facilityDraftStateID(source);
  button.dataset.to = facilityDraftStateID(destination);
  button.dataset.count = String(rows.length);
  button.setAttribute('aria-controls', 'facilityTransitionPanel');
  if (rows.some(row => row.facilityProblem)) button.dataset.problem = 'true';
  const pair = `${facilityDraftStateLabel(source)} → ${facilityDraftStateLabel(destination)}`;
  if (!rows.length) {
    button.textContent = '+';
    button.setAttribute('aria-label', `Новый переход: ${pair}`);
    return button;
  }
  for (const row of rows) {
    const name = document.createElement('span');
    name.className = 'facility-transition-cell-name';
    name.textContent = facilityDraftTransitionLabel(row);
    const summary = document.createElement('span');
    summary.className = 'facility-transition-cell-summary';
    summary.textContent = facilityTransitionCellSummary(row);
    button.append(name, summary);
  }
  button.setAttribute('aria-label', `${pair}: ${rows.map(facilityDraftTransitionLabel).join(', ')}`);
  return button;
}

function facilityTransitionCellButton(cell) {
  return [...facilityTransitionMatrix.querySelectorAll('.facility-transition-cell')]
    .find(button => button.facilitySource === cell?.source && button.facilityDestination === cell?.destination) || null;
}

function facilityMatrixHeader(state, scope) {
  const header = document.createElement('th');
  header.scope = scope;
  header.dataset.stateId = facilityDraftStateID(state);
  header.textContent = facilityDraftStateLabel(state);
  return header;
}

function renderFacilityStateChips(states) {
  const focused = document.activeElement?.closest('.facility-state-chip')?.facilityState;
  facilityDeviceStateChips.replaceChildren();
  for (const row of states) {
    const item = document.createElement('span');
    item.setAttribute('role', 'listitem');
    const chip = document.createElement('button');
    chip.type = 'button';
    chip.className = 'facility-state-chip';
    chip.facilityState = row;
    chip.dataset.stateId = facilityDraftStateID(row);
    chip.dataset.initial = row.dataset.initial;
    chip.setAttribute('aria-expanded', String(facilityStateEditor.facilityState === row));
    chip.setAttribute('aria-controls', 'facilityStateEditor');
    if (row.facilityProblem) chip.dataset.problem = 'true';
    const name = document.createElement('span');
    name.className = 'facility-state-chip-name';
    name.textContent = facilityDraftStateLabel(row);
    if (row.dataset.initial === 'true') {
      const star = document.createElement('span');
      star.className = 'facility-state-star';
      star.setAttribute('aria-hidden', 'true');
      star.textContent = '★';
      chip.appendChild(star);
      chip.setAttribute('aria-label', `${name.textContent}, начальное состояние`);
    }
    chip.appendChild(name);
    item.appendChild(chip);
    facilityDeviceStateChips.appendChild(item);
    if (row === focused) chip.focus();
  }
}

function renderFacilityStateEditor() {
  const row = facilityStateEditor.facilityState;
  facilityStateEditor.hidden = !row;
  if (!row) return;
  const initial = row.dataset.initial === 'true';
  const current = Boolean(row.facilityOriginal.id)
    && row.facilityOriginal.id === facilityDeviceByID(editingFacilityDeviceID)?.currentStateId;
  explainControl(btnMakeFacilityStateInitial, initial ? 'УЖЕ НАЧАЛЬНОЕ' : '');
  explainControl(btnDeleteFacilityState, initial ? 'НАЧАЛЬНОЕ СОСТОЯНИЕ: СНАЧАЛА ОТМЕТЬТЕ ЗВЕЗДОЙ ДРУГОЕ'
    : current ? 'УСТРОЙСТВО СЕЙЧАС В ЭТОМ СОСТОЯНИИ: СНАЧАЛА ПЕРЕВЕДИТЕ ЕГО В ДРУГОЕ' : '');
  if (controlBlocked(btnDeleteFacilityState)) resetFacilityStateDelete();
}

function renderFacilityTransitionMatrix(states, transitions) {
  const focused = document.activeElement?.closest('.facility-transition-cell');
  const focusedCell = focused && { source: focused.facilitySource, destination: focused.facilityDestination };
  const open = facilityTransitionPanel.hidden ? null : facilityTransitionPanel.facilityCell;
  const head = document.createElement('tr');
  head.appendChild(document.createElement('td'));
  for (const state of states) head.appendChild(facilityMatrixHeader(state, 'col'));
  facilityTransitionMatrix.tHead.replaceChildren(head);
  const body = facilityTransitionMatrix.tBodies[0];
  body.replaceChildren();
  for (const source of states) {
    const line = document.createElement('tr');
    line.appendChild(facilityMatrixHeader(source, 'row'));
    for (const destination of states) {
      const item = document.createElement('td');
      if (source === destination) {
        item.className = 'facility-transition-cell-unavailable';
        item.dataset.from = facilityDraftStateID(source);
        item.dataset.to = facilityDraftStateID(destination);
        item.textContent = '—';
      } else {
        const cell = facilityTransitionCell(source, destination, transitions
          .filter(row => row.facilitySource === source && row.facilityDestination === destination));
        cell.setAttribute('aria-expanded', String(open?.source === source && open?.destination === destination));
        item.appendChild(cell);
      }
      line.appendChild(item);
    }
    body.appendChild(line);
  }
  facilityTransitionMatrix.closest('.facility-transition-matrix-frame').hidden = !states.length;
  facilityTransitionMatrixHint.hidden = states.length > 1;
  facilityTransitionMatrixHint.textContent = states.length
    ? 'Для перехода нужно второе состояние.' : 'Добавьте состояния, чтобы задать переходы.';
  if (focusedCell) facilityTransitionCellButton(focusedCell)?.focus();
}

function renderFacilityTransitionPanel(states, transitions) {
  const open = transitions.find(row => !row.hidden);
  // The refused delete's uses stay only while their transition is open.
  if (facilityTransitionPanel.hidden || facilityTransitionUses.facilityTransition !== open) {
    clearFacilityUses(facilityTransitionUses);
  }
  if (open) facilityTransitionPanel.facilityCell = { source: open.facilitySource, destination: open.facilityDestination };
  const cell = facilityTransitionPanel.facilityCell;
  if (facilityTransitionPanel.hidden || !cell) return;
  document.getElementById('facilityTransitionPanelHeading').textContent =
    `ПЕРЕХОД: ${facilityDraftStateLabel(cell.source)} → ${facilityDraftStateLabel(cell.destination)}`;
  const cellRows = cell.source && cell.destination && cell.source !== cell.destination
    ? transitions.filter(row => row.facilitySource === cell.source && row.facilityDestination === cell.destination) : [];
  const focused = document.activeElement?.closest('.facility-transition-choice')?.facilityTransition;
  facilityTransitionCellList.replaceChildren();
  if (cellRows.length > 1) {
    for (const row of cellRows) {
      const item = document.createElement('li');
      const choice = document.createElement('button');
      choice.type = 'button';
      choice.className = 'btn btn-secondary btn-mini facility-transition-choice';
      choice.facilityTransition = row;
      choice.dataset.transitionId = facilityDraftTransitionID(row);
      choice.setAttribute('aria-pressed', String(row === open));
      choice.textContent = facilityDraftTransitionLabel(row);
      item.appendChild(choice);
      facilityTransitionCellList.appendChild(item);
      if (row === focused) choice.focus();
    }
  }
  facilityTransitionCellList.hidden = cellRows.length < 2;
  btnAddFacilityCellTransition.hidden = !cellRows.length;
  if (!open) return;
  const choices = states.map(state => ({ id: facilityDraftStateID(state), name: facilityDraftStateLabel(state) }));
  for (const [selector, end] of FACILITY_TRANSITION_ENDS) {
    const select = open.querySelector(selector);
    populateSelect(select, choices, '', 'ВЫБЕРИТЕ СОСТОЯНИЕ');
    select.selectedIndex = states.indexOf(open[end]) + 1;
  }
  refreshFacilityPreconditionChoices(open);
}

// renderFacilityDeviceDraft redraws everything derived from the draft: the
// identifier labels, the template offer, the chips and their editor, the
// matrix, the panel, and whether the dialog's links are blocked.
function renderFacilityDeviceDraft() {
  const states = facilityDraftStateRows();
  const transitions = facilityDraftTransitionRows();
  for (const row of states) {
    row.dataset.stateId = facilityDraftStateID(row);
    row.querySelector('label > span').textContent = facilityDraftStateLabel(row);
  }
  for (const row of transitions) {
    const id = facilityDraftTransitionID(row);
    row.dataset.transitionId = id;
    row.facilityIDRow.dataset.transitionId = id;
    row.facilityIDRow.querySelector('label > span').textContent = `${facilityDraftTransitionLabel(row)}: `
      + `${facilityDraftStateLabel(row.facilitySource)} → ${facilityDraftStateLabel(row.facilityDestination)}`;
  }
  renderFacilityDeviceTemplate();
  renderFacilityStateChips(states);
  renderFacilityStateEditor();
  renderFacilityTransitionPanel(states, transitions);
  renderFacilityTransitionMatrix(states, transitions);
  const changed = facilityDeviceDraftKey() !== facilityDeviceForm.facilityOpenedKey;
  for (const link of facilityDeviceForm.querySelectorAll('.facility-draft-link')) {
    explainControl(link, changed ? 'СНАЧАЛА НАЖМИТЕ «ГОТОВО»' : '');
  }
}

function openFacilityStateEditor(row, { focus = true, select = false } = {}) {
  facilityStateEditor.facilityState = row;
  facilityStateEditorName.value = row.facilityName;
  resetFacilityStateDelete();
  renderFacilityStateEditorProblem();
  renderFacilityDeviceDraft();
  if (focus) facilityStateEditorName.focus();
  if (select) facilityStateEditorName.select();
}

function closeFacilityStateEditor() {
  facilityStateEditor.facilityState = null;
  resetFacilityStateDelete();
  renderFacilityDeviceDraft();
}

function resetFacilityStateDelete() {
  clearFacilityUses(facilityStateUses);
  facilityStateDeleteConfirm.hidden = true;
}

// facilityStateTransitions lists the draft transitions that start or end in
// a state. They are deleted with it.
function facilityStateTransitions(row) {
  return facilityDraftTransitionRows().filter(transition => transition.facilitySource === row
    || transition.facilityDestination === row);
}

// facilityRuleUseSentence words a display rule that reads equality. A rule
// whose block is unknown leaves the block number out.
function facilityRuleUseSentence(binding, equality) {
  const name = binding.node.name;
  if (binding.property === 'menu-name') return `Название «${name}» в меню меняется, когда ${equality}.`;
  if (binding.property === 'entry-content') {
    const block = binding.block ? ` (блок ${binding.node.blocks.indexOf(binding.block) + 1})` : '';
    return `Текст записи «${name}»${block} меняется, когда ${equality}.`;
  }
  if (binding.property === 'visibility') return `Элемент «${name}» виден, только когда ${equality}.`;
  return `Команду «${name}» можно запросить, только когда ${equality}.`;
}

// FACILITY_DESTINATION_LABELS names the button that leads to each kind of
// destination, in a state's uses and in the dependency dialog (FR-037).
const FACILITY_DESTINATION_LABELS = Object.freeze({
  device: 'К УСТРОЙСТВУ',
  condition: 'К НЕИСПРАВНОСТИ',
  program: 'К ПРОГРАММЕ',
  command: 'К КОМАНДЕ',
  rule: 'К ПРАВИЛУ ОТОБРАЖЕНИЯ',
});

// facilityStateOutsideUses lists what outside the edited device names a
// state, directly or through the transitions that go with it (FR-008, R10).
// Only saved states and transitions have such uses.
function facilityStateOutsideUses(row, transitions) {
  const device = facilityDeviceByID(editingFacilityDeviceID);
  if (!device) return [];
  const facility = authoredFacility();
  const stateID = row.facilityOriginal.id;
  const equality = `${device.name} = ${facilityStateName(device, stateID)}`;
  const uses = [];
  if (stateID) {
    for (const other of facility.devices) {
      for (const transition of other.transitions || []) {
        if (!(transition.preconditions || []).some(value => value.deviceId === device.id && value.stateId === stateID)) continue;
        uses.push({
          destination: 'device',
          sentence: `Переход «${transition.name}» устройства «${other.name}» требует: ${equality}.`,
          open: () => openFacilityDeviceTransition(other.id, transition.id),
        });
      }
    }
    for (const binding of facilityBindingCatalog()) {
      if (binding.variant.when?.deviceId !== device.id || binding.variant.when.stateId !== stateID) continue;
      uses.push({
        destination: 'rule',
        sentence: facilityRuleUseSentence(binding, equality),
        open: () => openFacilityBindingDialog(binding),
      });
    }
  }
  const names = new Map(transitions.filter(transition => transition.facilityOriginal.id)
    .map(transition => [transition.facilityOriginal.id, transition.facilityOriginal.name]));
  uses.push(...facilityTransitionOutsideUses(device, names));
  return uses;
}

// facilityTransitionOutsideUses lists the command actions, recovery programs
// and fault recovery entries that name a saved transition of device. names
// maps each transition ID to its saved name.
function facilityTransitionOutsideUses(device, names) {
  const facility = authoredFacility();
  const uses = [];
  const named = request => request?.deviceId === device.id && names.has(request.transitionId);
  for (const { terminal, node } of facilityNodeCatalog()) {
    for (const request of node.stateChange?.facilityAction?.transitions?.transitions || []) {
      if (!named(request)) continue;
      uses.push({
        destination: 'command',
        sentence: `Команда «${node.name}» запускает переход «${names.get(request.transitionId)}».`,
        open: () => showFacilityCommand(terminal.id, node.id),
      });
    }
  }
  for (const program of facility.recoveryPrograms) {
    for (const request of program.transitions || []) {
      if (!named(request)) continue;
      uses.push({
        destination: 'program',
        sentence: `Программа восстановления «${program.name}» запускает переход «${names.get(request.transitionId)}».`,
        open: () => openFacilityRecoveryProgramDialog(program.id),
      });
    }
  }
  for (const condition of facility.conditions) {
    for (const reference of condition.recovery || []) {
      if (!named(reference.transition)) continue;
      uses.push({
        destination: 'condition',
        sentence: `Неисправность «${condition.name}» устраняется переходом «${names.get(reference.transition.transitionId)}».`,
        open: () => openFacilityConditionDialog(condition.id),
      });
    }
  }
  return uses;
}

// showFacilityUses lists the outside uses that block a delete, each with its
// destination button, and moves focus to the first one.
function showFacilityUses(list, uses) {
  for (const use of uses) {
    const item = document.createElement('li');
    item.className = 'facility-state-use';
    item.dataset.destination = use.destination;
    const sentence = document.createElement('span');
    sentence.textContent = use.sentence;
    item.appendChild(sentence);
    facilityDraftLink(item, FACILITY_DESTINATION_LABELS[use.destination], use.open);
    list.appendChild(item);
  }
  list.hidden = false;
  renderFacilityDeviceDraft();
  list.querySelector('button').focus();
}

function clearFacilityUses(list) {
  list.replaceChildren();
  list.hidden = true;
}

function removeFacilityDraftState(row) {
  for (const transition of facilityStateTransitions(row)) {
    transition.facilityIDRow.remove();
    transition.remove();
  }
  row.remove();
  const cell = facilityTransitionPanel.facilityCell;
  if (cell?.source === row || cell?.destination === row) facilityTransitionPanel.hidden = true;
  facilityStateEditor.facilityState = null;
  resetFacilityStateDelete();
  renderFacilityDeviceDraft();
  btnAddFacilityState.focus();
}

// openFacilityTransitionPanel shows one draft transition alone in the panel.
function openFacilityTransitionPanel(row, { focus = true, select = false } = {}) {
  for (const other of facilityDraftTransitionRows()) other.hidden = other !== row;
  facilityTransitionPanel.hidden = false;
  renderFacilityDeviceDraft();
  const name = row.querySelector('.facility-transition-name');
  if (focus) name.focus();
  if (select) name.select();
}

// chooseFacilityTransitionCell opens a matrix cell: an empty cell starts a
// transition between its states, a single transition opens, and several are
// listed first (FR-005).
function chooseFacilityTransitionCell(source, destination) {
  const rows = facilityDraftCellRows(source, destination);
  if (!rows.length) {
    const name = facilityDraftTemplateTransitionName(source, destination);
    openFacilityTransitionPanel(addFacilityDraftTransition(source, destination, name), { select: true });
    return;
  }
  if (rows.length === 1) {
    openFacilityTransitionPanel(rows[0]);
    return;
  }
  for (const row of facilityDraftTransitionRows()) row.hidden = true;
  facilityTransitionPanel.facilityCell = { source, destination };
  facilityTransitionPanel.hidden = false;
  renderFacilityDeviceDraft();
  facilityTransitionCellList.querySelector('button').focus();
}

function closeFacilityTransitionPanel() {
  const cell = facilityTransitionPanel.facilityCell;
  facilityTransitionPanel.hidden = true;
  for (const row of facilityDraftTransitionRows()) row.hidden = true;
  renderFacilityDeviceDraft();
  (facilityTransitionCellButton(cell) || btnAddFacilityState).focus();
}

function openFacilityDeviceDialog(deviceID = '') {
  facilityDeviceForm.reset();
  facilityDeviceStateIDs.replaceChildren();
  facilityDeviceTransitionIDs.replaceChildren();
  for (const row of facilityDraftTransitionRows()) row.remove();
  facilityStateEditor.facilityState = null;
  clearFacilityRowProblem(facilityStateEditor);
  resetFacilityStateDelete();
  facilityTransitionPanel.hidden = true;
  facilityTransitionPanel.facilityCell = null;
  facilityDeviceTechnical.open = false;
  facilityDeviceStatus.textContent = '';
  editingFacilityDeviceID = deviceID;
  delete facilityDeviceID.dataset.manual;
  const device = facilityDeviceByID(deviceID);
  facilityDeviceID.readOnly = Boolean(device);
  if (device) {
    facilityDeviceID.dataset.manual = 'true';
    facilityDeviceID.value = device.id;
    facilityDeviceName.value = device.name;
    facilityDeviceKind.value = device.kind;
    for (const value of device.states || []) {
      appendFacilityStateRow(value).dataset.initial = String(value.id === device.initialStateId);
    }
    for (const transition of device.transitions || []) appendFacilityTransitionRow(transition);
  }
  facilityDeviceForm.facilityOpenedKey = facilityDeviceDraftKey();
  renderFacilityDeviceDraft();
  setFacilityFeedback(facilityDeviceError);
  showFacilityDialog(facilityDeviceDialog, facilityDeviceName);
}

// openFacilityDeviceTransition opens a device's dialog with one of its saved
// transitions chosen in the panel and focus on its name (R13).
function openFacilityDeviceTransition(deviceID, transitionID) {
  selectedFacilityDeviceID = deviceID;
  renderFacilityWorkspace();
  openFacilityDeviceDialog(deviceID);
  const row = facilityDraftTransitionRows().find(value => value.facilityOriginal.id === transitionID);
  if (row) openFacilityTransitionPanel(row);
}

function closeFacilityDeviceDialog(focusTarget = null) {
  editingFacilityDeviceID = '';
  hideFacilityDialog(facilityDeviceDialog, focusTarget);
}

function readFacilityDeviceDraft() {
  const stateRows = facilityDraftStateRows();
  const states = stateRows.map(row => ({
    ...row.facilityOriginal,
    id: facilityDraftStateID(row),
    name: row.facilityName.trim(),
  }));
  const transitions = facilityDraftTransitionRows().map(row => ({
    ...row.facilityOriginal,
    id: facilityDraftTransitionID(row),
    name: row.querySelector('.facility-transition-name').value.trim(),
    sourceStateId: facilityDraftStateID(row.facilitySource),
    destinationStateId: facilityDraftStateID(row.facilityDestination),
    preconditions: [...row.querySelectorAll('.facility-precondition-row')].map(reference => ({
      ...reference.facilityOriginal,
      deviceId: reference.querySelector('.facility-precondition-device').value,
      stateId: reference.querySelector('.facility-precondition-state').value,
    })),
  }));
  const initialStateId = facilityDraftStateID(stateRows.find(row => row.dataset.initial === 'true'));
  const saved = facilityDeviceByID(editingFacilityDeviceID);
  const device = {
    ...structuredClone(saved || {}),
    id: facilityDeviceID.value.trim(), name: facilityDeviceName.value.trim(), kind: facilityDeviceKind.value,
    initialStateId, currentStateId: saved?.currentStateId || initialStateId, states, transitions,
  };
  if (device.kind === 'custom') device.customKind ||= device.id;
  else delete device.customKind;
  return device;
}

// facilityDeviceDraftKey covers the authored part of the draft, so a play
// change to the open device is not an unsaved change.
function facilityDeviceDraftKey() {
  const { id, name, kind, initialStateId, states, transitions } = readFacilityDeviceDraft();
  return JSON.stringify({ id, name, kind, initialStateId, states, transitions });
}

// Returns at most one problem per state or transition, in reading order.
function facilityDeviceRowProblems() {
  const problems = [];
  const check = (owner, rules) => {
    const failed = rules.find(([invalid]) => invalid);
    if (failed) problems.push({ owner, control: failed[1], message: failed[2] });
  };
  const stateIDs = new Set();
  const stateNames = new Set();
  for (const row of facilityDraftStateRows()) {
    const id = row.querySelector('.facility-state-id');
    const value = id.value.trim();
    const name = row.facilityName.trim();
    // The server compares state names case-insensitively after trimming.
    const normalizedName = name.toLowerCase();
    check(row, [
      [!normalizedName, facilityStateEditorName, 'УКАЖИТЕ НАЗВАНИЕ СОСТОЯНИЯ'],
      [stateNames.has(normalizedName), facilityStateEditorName, `НАЗВАНИЕ УЖЕ ИСПОЛЬЗУЕТСЯ: ${name}`],
      [!value, id, 'УКАЖИТЕ ИДЕНТИФИКАТОР СОСТОЯНИЯ'],
      [stateIDs.has(value), id, `ИДЕНТИФИКАТОР УЖЕ ИСПОЛЬЗУЕТСЯ: ${value}`],
    ]);
    if (value) stateIDs.add(value);
    if (normalizedName) stateNames.add(normalizedName);
  }
  const transitionIDs = new Set();
  for (const row of facilityDraftTransitionRows()) {
    const id = row.facilityIDRow.querySelector('.facility-transition-id');
    const name = row.querySelector('.facility-transition-name');
    const destination = row.querySelector('.facility-transition-destination');
    const value = id.value.trim();
    check(row, [
      [!name.value.trim(), name, 'УКАЖИТЕ НАЗВАНИЕ ПЕРЕХОДА'],
      [!value, id, 'УКАЖИТЕ ИДЕНТИФИКАТОР ПЕРЕХОДА'],
      [transitionIDs.has(value), id, `ИДЕНТИФИКАТОР УЖЕ ИСПОЛЬЗУЕТСЯ: ${value}`],
      [!row.facilitySource, row.querySelector('.facility-transition-source'), 'ВЫБЕРИТЕ СОСТОЯНИЕ «ИЗ»'],
      [!row.facilityDestination, destination, 'ВЫБЕРИТЕ СОСТОЯНИЕ «В»'],
      [row.facilitySource === row.facilityDestination, destination, 'СОСТОЯНИЯ «ИЗ» И «В» СОВПАДАЮТ'],
    ]);
    if (value) transitionIDs.add(value);
  }
  return problems;
}

function clearFacilityRowProblem(row) {
  row.querySelectorAll('.facility-row-error').forEach(note => note.remove());
  for (const control of row.querySelectorAll('[aria-invalid="true"]')) {
    control.removeAttribute('aria-invalid');
    control.removeAttribute('aria-describedby');
  }
}

function clearFacilityDeviceProblems() {
  facilityDeviceForm.querySelectorAll('.facility-state-row, .facility-transition-id-row, .facility-transition-row')
    .forEach(clearFacilityRowProblem);
  clearFacilityRowProblem(facilityStateEditor);
  for (const row of [...facilityDraftStateRows(), ...facilityDraftTransitionRows()]) delete row.facilityProblem;
  renderFacilityDeviceDraft();
}

function markFacilityRowProblem(container, control, message, noteID) {
  const note = document.createElement('p');
  note.className = 'facility-row-error';
  note.id = noteID;
  note.textContent = message;
  container.appendChild(note);
  control.setAttribute('aria-invalid', 'true');
  control.setAttribute('aria-describedby', note.id);
}

// The state editor serves every chip, so it shows the name problem of the
// state it is open on.
function renderFacilityStateEditorProblem() {
  clearFacilityRowProblem(facilityStateEditor);
  const problem = facilityStateEditor.facilityState?.facilityProblem;
  if (problem?.control !== facilityStateEditorName) return;
  markFacilityRowProblem(facilityStateEditor, facilityStateEditorName, problem.message, 'facilityStateEditorProblem');
}

// showFacilityRowProblems marks each problem field. Its chip or matrix cell
// keeps data-problem until the author edits the row.
function showFacilityRowProblems(problems) {
  if (!problems.length) return;
  problems.forEach(({ owner, control, message }, index) => {
    owner.facilityProblem = { control, message };
    if (control === facilityStateEditorName) return;
    const container = control.closest('.facility-state-row, .facility-transition-id-row, .facility-transition-row');
    markFacilityRowProblem(container, control, message, `facilityRowProblem${index}`);
    container.addEventListener('input', () => {
      clearFacilityRowProblem(container);
      delete owner.facilityProblem;
    }, { once: true });
  });
  renderFacilityStateEditorProblem();
  renderFacilityDeviceDraft();
}

// revealFacilityDeviceField opens the chip editor or the transition panel
// that holds field. setFacilityFeedback opens the technical block itself.
function revealFacilityDeviceField(field, owner) {
  if (field === facilityStateEditorName) openFacilityStateEditor(owner, { focus: false });
  const transition = field.closest('.facility-transition-row');
  if (transition) openFacilityTransitionPanel(transition, { focus: false });
}

function validateFacilityDeviceDraft(device) {
  if (!device.name) return ['УКАЖИТЕ НАЗВАНИЕ УСТРОЙСТВА', facilityDeviceName];
  if (!device.kind) return ['ВЫБЕРИТЕ ТИП УСТРОЙСТВА', facilityDeviceKind];
  if (!device.states.length) return ['ДОБАВЬТЕ ХОТЯ БЫ ОДНО СОСТОЯНИЕ', btnAddFacilityState];
  const problems = facilityDeviceRowProblems();
  if (problems.length) return [`ИСПРАВЬТЕ ОТМЕЧЕННОЕ: ${problems.length}`, problems[0].control, problems];
  if (!device.id) return ['УКАЖИТЕ СТАБИЛЬНЫЙ ИДЕНТИФИКАТОР УСТРОЙСТВА', facilityDeviceID];
  if (authoredFacility().devices.some(candidate => candidate.id === device.id && candidate.id !== editingFacilityDeviceID)) {
    return ['Такой идентификатор устройства уже используется', facilityDeviceID];
  }
  const stateIDs = new Set(device.states.map(stateValue => stateValue.id));
  if (!stateIDs.has(device.initialStateId)) {
    return ['ОТМЕТЬТЕ ЗВЕЗДОЙ НАЧАЛЬНОЕ СОСТОЯНИЕ', facilityDeviceStateChips.querySelector('.facility-state-chip')];
  }
  if (!stateIDs.has(device.currentStateId)) return ['НЕЛЬЗЯ УДАЛИТЬ ТЕКУЩЕЕ СОСТОЯНИЕ УСТРОЙСТВА', btnAddFacilityState];
  for (const row of facilityTransitionPanel.querySelectorAll('.facility-precondition-row')) {
    const deviceSelect = row.querySelector('.facility-precondition-device');
    const stateSelect = row.querySelector('.facility-precondition-state');
    const target = facilityDeviceByID(deviceSelect.value);
    if (!target) return ['ВЫБЕРИТЕ УСТРОЙСТВО ПРЕДУСЛОВИЯ', deviceSelect];
    if (!target.states.some(value => value.id === stateSelect.value)) {
      return ['ВЫБЕРИТЕ СОСТОЯНИЕ ПРЕДУСЛОВИЯ', stateSelect];
    }
  }
  return null;
}

function populateFacilityDeviceSelect(select, selected = '') {
  populateSelect(select, (authoredFacility()?.devices || []).map(device => ({ id: device.id, name: device.name })),
    selected, 'ВЫБЕРИТЕ УСТРОЙСТВО');
}

// facilityTransitionKey names one transition of one device. IDs may contain
// any character, so the pair is encoded rather than joined.
function facilityTransitionKey(deviceID, transitionID) {
  return JSON.stringify([deviceID, transitionID]);
}

// facilityFaultEffect is the effect the fault draft gives transition of
// deviceID: true turns the fault on, false clears it, null is none.
function facilityFaultEffect(deviceID, transition) {
  const key = facilityTransitionKey(deviceID, transition.id);
  if (facilityFaultEdits.has(key)) return facilityFaultEdits.get(key);
  const effect = editingFacilityConditionID
    && transition.conditionEffects?.find(value => value.conditionId === editingFacilityConditionID);
  return effect ? effect.active === true : null;
}

// facilityFaultTransitions lists, in device and transition order, every
// transition whose draft effect is active: true for «КАК ПОЯВЛЯЕТСЯ», false
// for «КАК УСТРАНЯЕТСЯ» (R2).
function facilityFaultTransitions(active) {
  const ways = [];
  for (const device of authoredFacility().devices) {
    for (const transition of device.transitions || []) {
      if (facilityFaultEffect(device.id, transition) === active) ways.push({ device, transition });
    }
  }
  return ways;
}

function facilityFaultWayKeys(ways) {
  return new Set(ways.map(({ device, transition }) => facilityTransitionKey(device.id, transition.id)));
}

// facilityActionPartners maps each transition key to the keys of the
// transitions that run with it in one command action or recovery program.
function facilityActionPartners() {
  const partners = new Map();
  const actions = [
    ...facilityNodeCatalog().map(({ node }) => node.stateChange?.facilityAction?.transitions?.transitions || []),
    ...authoredFacility().recoveryPrograms.map(program => program.transitions || []),
  ];
  for (const requests of actions) {
    const keys = requests.map(request => facilityTransitionKey(request.deviceId, request.transitionId));
    for (const key of keys) {
      if (!partners.has(key)) partners.set(key, new Set());
      for (const other of keys) if (other !== key) partners.get(key).add(other);
    }
  }
  return partners;
}

// populateFacilityFaultPicker offers every transition that neither list
// holds and that runs with no transition of the opposite effect, so one
// transition or one action never both turns the fault on and clears it (R3).
function populateFacilityFaultPicker(picker, listed, opposite, partners) {
  picker.replaceChildren(new Option('ВЫБЕРИТЕ ПЕРЕХОД', ''));
  let index = 0;
  for (const device of authoredFacility().devices) {
    const group = document.createElement('optgroup');
    group.label = device.name;
    for (const transition of device.transitions || []) {
      const key = facilityTransitionKey(device.id, transition.id);
      if (listed.has(key) || [...(partners.get(key) || [])].some(other => opposite.has(other))) continue;
      const option = new Option(`${device.name} — ${transition.name}`, String(++index));
      option.dataset.deviceId = device.id;
      option.dataset.transitionId = transition.id;
      group.appendChild(option);
    }
    if (group.children.length) picker.appendChild(group);
  }
}

function facilityFaultWay(kind, name, data = {}) {
  const item = document.createElement('li');
  item.className = 'facility-fault-way';
  Object.assign(item.dataset, { kind, ...data });
  const label = document.createElement('span');
  label.className = 'facility-fault-way-name';
  label.textContent = name;
  item.appendChild(label);
  return item;
}

// facilityFaultTransitionWay is a transition row of either list. Removing it
// removes the transition's effect from the draft and returns focus to add.
function facilityFaultTransitionWay({ device, transition }, add) {
  const name = `${device.name} — ${transition.name}`;
  const item = facilityFaultWay('transition', name, { deviceId: device.id, transitionId: transition.id });
  const remove = document.createElement('button');
  remove.type = 'button';
  remove.className = 'btn btn-secondary btn-mini facility-fault-way-remove';
  remove.textContent = 'УБРАТЬ';
  remove.setAttribute('aria-label', `Убрать: ${name}`);
  remove.addEventListener('click', () => {
    facilityFaultEdits.set(facilityTransitionKey(device.id, transition.id), null);
    renderFacilityFaultDraft();
    add.focus();
  });
  item.appendChild(remove);
  return item;
}

// facilityFaultProgramWay follows from a clearing transition, so it opens
// its program instead of offering a remove.
function facilityFaultProgramWay(program) {
  const item = facilityFaultWay('program', program.name, { programId: program.id });
  const kind = document.createElement('span');
  kind.className = 'facility-fault-way-kind';
  kind.textContent = 'программа восстановления';
  const open = document.createElement('button');
  open.type = 'button';
  open.className = 'btn btn-secondary btn-mini facility-fault-way-open';
  open.textContent = 'К ПРОГРАММЕ';
  open.setAttribute('aria-haspopup', 'dialog');
  open.addEventListener('click', () => openFacilityRecoveryProgramDialog(program.id, { opener: open }));
  item.append(kind, open);
  return item;
}

function facilityFaultOverseerWay(terminalScope) {
  const item = facilityFaultWay('overseer', 'Смотритель вручную');
  const hint = document.createElement('span');
  hint.className = 'facility-fault-way-hint';
  hint.textContent = terminalScope
    ? 'во время игры — переключатель в списке «Неисправности терминалов»'
    : 'во время игры — переключатель на карточке устройства';
  item.appendChild(hint);
  return item;
}

// replaceFacilityFaultWays redraws list only when its rows change, so a
// newer session leaves focus on an unchanged row.
function replaceFacilityFaultWays(list, rows) {
  const key = JSON.stringify(rows.map(row => [{ ...row.dataset }, row.textContent]));
  if (list.facilityKey === key) return;
  list.facilityKey = key;
  list.replaceChildren(...rows);
}

// facilityConditionDraftKey covers everything the fault dialog edits, so the
// preview waits for «ГОТОВО» once the draft differs from the session it
// opened on.
function facilityConditionDraftKey() {
  const ways = active => [...facilityFaultWayKeys(facilityFaultTransitions(active))];
  return JSON.stringify([[...new FormData(facilityConditionForm)], ways(true), ways(false)]);
}

// renderFacilityFaultDraft redraws what the fault dialog derives from its
// draft over the newest session: the scope fields, «КАК ПОЯВЛЯЕТСЯ» and «КАК
// УСТРАНЯЕТСЯ» with their pickers, and the preview control (R2).
function renderFacilityFaultDraft() {
  const terminalScope = facilityConditionScope.value === 'terminal';
  for (const [control, hidden] of [[facilityConditionDevice, terminalScope], [facilityConditionTerminal, !terminalScope]]) {
    control.hidden = hidden;
    facilityConditionForm.querySelector(`label[for="${control.id}"]`).hidden = hidden;
  }
  const initialActive = facilityConditionInitialActive.checked;
  const appears = facilityFaultTransitions(true);
  const clears = facilityFaultTransitions(false);
  const appearing = facilityFaultWayKeys(appears);
  const clearing = facilityFaultWayKeys(clears);
  document.getElementById('facilityFaultAppearsInitial').hidden = !initialActive;
  replaceFacilityFaultWays(facilityFaultAppearsList, appears.map(way => facilityFaultTransitionWay(way, btnAddFaultActivation)));
  facilityFaultAppearsWarning.hidden = appears.length > 0;
  facilityFaultAppearsWarning.textContent = initialActive
    ? 'Ни один переход не включает эту неисправность снова. После устранения её можно включить только вручную.'
    : 'Ни один переход не включает эту неисправность. Сейчас её можно включить только вручную.';
  const programs = authoredFacility().recoveryPrograms.filter(program => (program.transitions || [])
    .some(request => clearing.has(facilityTransitionKey(request.deviceId, request.transitionId))));
  replaceFacilityFaultWays(facilityFaultClearsList, [
    ...clears.map(way => facilityFaultTransitionWay(way, btnAddFaultClearing)),
    ...programs.map(facilityFaultProgramWay),
    facilityFaultOverseerWay(terminalScope),
  ]);
  const listed = new Set([...appearing, ...clearing]);
  const partners = facilityActionPartners();
  populateFacilityFaultPicker(facilityFaultActivationPicker, listed, clearing, partners);
  populateFacilityFaultPicker(facilityFaultClearingPicker, listed, appearing, partners);
  const saved = Boolean(editingFacilityConditionID);
  btnPreviewFacilityCondition.hidden = !saved;
  document.getElementById('btnPreviewFacilityConditionReason').hidden = !saved;
  const changed = facilityConditionDraftKey() !== facilityConditionForm.facilityOpenedKey;
  explainControl(btnPreviewFacilityCondition,
    changed ? 'СНАЧАЛА НАЖМИТЕ «ГОТОВО»' : controlBlockReason(btnPreviewFacilityCondition));
}

function openFacilityConditionDialog(conditionID = '') {
  editingFacilityConditionID = conditionID;
  facilityFaultEdits = new Map();
  const condition = authoredFacility().conditions.find(value => value.id === conditionID);
  facilityConditionForm.reset();
  facilityConditionTechnical.open = false;
  facilityFaultActivationPicker.hidden = true;
  facilityFaultClearingPicker.hidden = true;
  delete facilityConditionID.dataset.manual;
  facilityConditionID.readOnly = Boolean(condition);
  facilityConditionID.value = condition?.id || '';
  facilityConditionName.value = condition?.name || '';
  facilityConditionCategory.value = condition?.category || '';
  if (condition) facilityConditionID.dataset.manual = 'true';
  facilityConditionScope.value = condition?.terminal ? 'terminal' : 'device';
  populateFacilityDeviceSelect(facilityConditionDevice, condition?.device?.deviceId || selectedFacilityDeviceID);
  populateSelect(facilityConditionTerminal,
    (state.session?.terminals || []).map(terminal => ({ id: terminal.id, name: terminal.name })),
    condition?.terminal?.terminalId || '', 'ВЫБЕРИТЕ ТЕРМИНАЛ');
  facilityConditionCapability.value = condition?.effects
    ?.find(effect => effect.capabilityBlock)?.capabilityBlock?.capability || '';
  (condition?.initialActive ? facilityConditionInitialActive : facilityConditionInitialWorking).checked = true;
  facilityConditionForm.facilityOpenedKey = facilityConditionDraftKey();
  renderFacilityFaultDraft();
  setFacilityFeedback(facilityConditionError);
  showFacilityDialog(facilityConditionDialog, facilityConditionName);
}

function closeFacilityConditionDialog() {
  editingFacilityConditionID = '';
  hideFacilityDialog(facilityConditionDialog, btnAddFacilityCondition);
}

// facilityConditionScopeConflict names a stored effect that changes a
// terminal other than terminalID; a terminal fault acts on its own terminal.
function facilityConditionScopeConflict(effects, terminalID) {
  for (const effect of effects) {
    const target = effect.diagnosticPath || effect.recordSubstitution;
    if (!target || target.terminalId === terminalID) continue;
    const terminal = state.session.terminals.find(value => value.id === target.terminalId);
    const node = effect.diagnosticPath
      ? terminal && locateNode(terminal.root, target.nodeId)?.node
      : terminal && entryBlockLocation(terminal, target.blockId)?.entry;
    return `Неисправность меняет «${node?.name || target.nodeId || target.blockId}» на терминале `
      + `«${terminal?.name || target.terminalId}». Выберите этот терминал в поле «ГДЕ ДЕЙСТВУЕТ».`;
  }
  return '';
}

// applyFacilityFaultEdits writes the fault dialog's list edits as effects on
// conditionID. An effect that keeps its value keeps its stored object and
// position; a transition that no longer exists is left to the server.
function applyFacilityFaultEdits(facility, conditionID, edits) {
  for (const [key, active] of edits) {
    const [deviceID, transitionID] = JSON.parse(key);
    const transition = facility.devices.find(device => device.id === deviceID)
      ?.transitions?.find(value => value.id === transitionID);
    if (!transition) continue;
    const effects = transition.conditionEffects || [];
    const index = effects.findIndex(effect => effect.conditionId === conditionID);
    if (index >= 0 && (effects[index].active === true) === active) continue;
    if (index >= 0) effects.splice(index, 1);
    if (active !== null) effects.push({ conditionId: conditionID, active });
    if (effects.length) transition.conditionEffects = effects;
    else delete transition.conditionEffects;
  }
}

// facilityRecoveryStanding says whether a stored recovery entry of
// conditionID still resolves and still clears the fault. A transition entry
// stands on its transition, a program entry on its program, which must hold
// only recovery transitions. A privateOverseerAction entry always stands.
function facilityRecoveryStanding(facility, reference, conditionID) {
  const transition = request => facility.devices.find(device => device.id === request.deviceId)
    ?.transitions?.find(value => value.id === request.transitionId);
  const clears = request => Boolean(transition(request)?.conditionEffects
    ?.some(effect => effect.conditionId === conditionID && effect.active !== true));
  if (reference.transition) {
    return { resolves: Boolean(transition(reference.transition)), clears: clears(reference.transition) };
  }
  if (reference.recoveryProgramId === undefined) return { resolves: true, clears: true };
  const requests = facility.recoveryPrograms.find(program => program.id === reference.recoveryProgramId)?.transitions;
  return {
    resolves: Boolean(requests?.every(request => transition(request)?.recovery === true)),
    clears: Boolean(requests?.some(clears)),
  };
}

// pruneFacilityRecovery runs edit on facility, then drops each stored
// recovery entry that the edit stopped resolving or clearing its fault. It
// never adds an entry and never touches privateOverseerAction (R2).
function pruneFacilityRecovery(facility, edit) {
  const standing = condition => (condition.recovery || [])
    .map(reference => facilityRecoveryStanding(facility, reference, condition.id));
  const before = new Map(facility.conditions.map(condition => [condition.id, standing(condition)]));
  edit();
  for (const condition of facility.conditions) {
    const was = before.get(condition.id);
    if (!was?.length) continue;
    const now = standing(condition);
    condition.recovery = condition.recovery
      .filter((_reference, index) => !(was[index].resolves && !now[index].resolves) && !(was[index].clears && !now[index].clears));
  }
}

// facilityNodePath names node by its terminal and folders, for example
// «Терминал смотрителя › Директивы Vault-Tec › Сводка эвакуации».
function facilityNodePath(terminal, nodeID) {
  const walk = (node, names) => {
    if (node.id === nodeID) return names;
    for (const child of node.children || []) {
      const found = walk(child, [...names, child.name]);
      if (found) return found;
    }
    return null;
  };
  return [terminal.name, ...(walk(terminal.root, []) || [])].join(' › ');
}

// renderFacilityProgramCommands names the commands whose action runs the
// program, or warns that players have no way to run it (FR-018).
function renderFacilityProgramCommands(programID) {
  const commands = programID ? facilityNodeCatalog()
    .filter(({ node }) => node.stateChange?.facilityAction?.recoveryProgramId === programID) : [];
  document.getElementById('facilityProgramCommandList').replaceChildren(...commands.map(({ terminal, node }) => {
    const item = document.createElement('li');
    item.textContent = facilityNodePath(terminal, node.id);
    return item;
  }));
  document.getElementById('facilityProgramCommandsWarning').hidden = commands.length > 0;
}

// openFacilityRecoveryProgramDialog opens a saved program, or a new one when
// programID is empty. Over the fault dialog (R4), opener gets focus back,
// faultName names the fault, and a new program starts from request, the
// fault's first clearing transition, or with no row. From anywhere else a
// new program starts with one empty row.
function openFacilityRecoveryProgramDialog(programID = '', { opener = null, faultName = '', request = null } = {}) {
  editingFacilityProgramID = programID;
  facilityProgramFromFault = facilityConditionDialog.open;
  facilityProgramReturn = opener || btnAddFacilityRecoveryProgram;
  const program = authoredFacility().recoveryPrograms.find(value => value.id === programID);
  facilityRecoveryProgramForm.reset();
  facilityRecoveryProgramTechnical.open = false;
  delete facilityRecoveryProgramID.dataset.manual;
  facilityRecoveryProgramID.readOnly = Boolean(program);
  facilityRecoveryProgramID.value = program?.id || '';
  facilityRecoveryProgramName.value = program?.name || '';
  if (program) facilityRecoveryProgramID.dataset.manual = 'true';
  facilityRecoveryProgramFault.hidden = !faultName;
  facilityRecoveryProgramFault.textContent = faultName ? `Для неисправности «${faultName}»` : '';
  facilityProgramTransitions.replaceChildren();
  const fresh = facilityProgramFromFault ? [request].filter(Boolean) : [{}];
  for (const value of program?.transitions || fresh) appendFacilityActionRequest(value, facilityProgramTransitions);
  renderFacilityProgramCommands(program?.id || '');
  setFacilityFeedback(facilityRecoveryProgramError);
  showFacilityDialog(facilityRecoveryProgramDialog, facilityRecoveryProgramName);
}

// closeFacilityRecoveryProgramDialog returns focus to the control that
// opened the dialog. A fault list row redrawn meanwhile hands focus to its
// replacement, or to «+ ПРОГРАММА ВОССТАНОВЛЕНИЯ…».
function closeFacilityRecoveryProgramDialog() {
  let target = facilityProgramReturn;
  if (!target?.isConnected && facilityProgramFromFault) {
    const programID = target?.closest('[data-program-id]')?.dataset.programId;
    target = (programID && facilityFaultClearsList.querySelector(
      `[data-program-id="${CSS.escape(programID)}"] .facility-fault-way-open`)) || btnAddFaultRecoveryProgram;
  }
  editingFacilityProgramID = '';
  hideFacilityDialog(facilityRecoveryProgramDialog, target);
}

// FACILITY_RULE_TYPES describes the display rule tabs in tab order (R5): the
// type's name in sentences, the clauses it shows, the elements «ГДЕ» offers,
// and what players see in the device's other states (FR-027).
const FACILITY_RULE_TYPES = {
  'entry-content': {
    name: 'Текст записи', clauses: ['when', 'where', 'block', 'text'],
    offers: element => element.type === 'entry' && element.blocks.length > 0,
    otherStates: 'В остальных состояниях останется исходный текст блока.',
  },
  'menu-name': {
    name: 'Название в меню', clauses: ['when', 'where', 'text'],
    offers: element => !element.isRoot,
    otherStates: 'В остальных состояниях останется исходное название.',
  },
  visibility: {
    name: 'Показать элемент', clauses: ['when', 'where'],
    offers: element => !element.isRoot,
    otherStates: 'В остальных состояниях элемент скрыт.',
  },
  availability: {
    name: 'Разрешить команду', clauses: ['when', 'where'],
    offers: element => element.type === 'command',
    otherStates: 'В остальных состояниях команда видна, но её нельзя запросить.',
  },
};

// facilityRuleElementCatalog lists every element of every terminal for
// «ГДЕ», labelled by its path, with a record's blocks numbered from 1 (R5).
function facilityRuleElementCatalog() {
  return facilityNodeCatalog().map(({ terminal, node }) => ({
    terminalId: terminal.id, nodeId: node.id, type: node.type, name: node.name,
    label: facilityNodePath(terminal, node.id), isRoot: node === terminal.root,
    blocks: (node.blocks || []).map((block, index) => ({
      id: block.id, name: `${index + 1} · «${normalizedEntryBlockPreview(block.initialText)}»`,
    })),
  }));
}

// facilityRuleElement is the element chosen in «ГДЕ», or null.
function facilityRuleElement() {
  return facilityBindingNode.value ? facilityRuleElements[Number(facilityBindingNode.value)] : null;
}

// updateFacilityBindingNodes lists in «ГДЕ» the elements the open tab applies
// to (FR-024). The selected element stays while the tab offers it (FR-026).
function updateFacilityBindingNodes(selected = facilityBindingNode.value) {
  const { offers } = FACILITY_RULE_TYPES[facilityRuleKind];
  facilityBindingNode.replaceChildren(new Option('ВЫБЕРИТЕ ЭЛЕМЕНТ', ''));
  for (const [index, element] of facilityRuleElements.entries()) {
    if (!offers(element)) continue;
    const option = new Option(element.label, String(index));
    option.dataset.terminalId = element.terminalId;
    option.dataset.nodeId = element.nodeId;
    facilityBindingNode.appendChild(option);
  }
  if ([...facilityBindingNode.options].some(option => option.value === selected)) facilityBindingNode.value = selected;
}

// updateFacilityBindingBlocks lists the chosen record's blocks in «БЛОК»;
// the selected block stays while the element does.
function updateFacilityBindingBlocks(selected = facilityBindingBlock.value) {
  populateSelect(facilityBindingBlock, facilityRuleElement()?.blocks || [], selected, 'ВЫБЕРИТЕ БЛОК');
}

function updateFacilityBindingStates() {
  const device = facilityDeviceByID(facilityBindingDevice.value);
  populateSelect(facilityBindingState, device?.states || [], facilityBindingState.value, 'ВЫБЕРИТЕ СОСТОЯНИЕ');
}

// renderFacilityRuleTabs shows the open tab with its clauses and its line
// about the other states. An existing rule keeps its type, so its other
// tabs are blocked (FR-026).
function renderFacilityRuleTabs() {
  const type = FACILITY_RULE_TYPES[facilityRuleKind];
  const reason = editingFacilityBinding ? 'ЧТОБЫ СМЕНИТЬ ТИП, СОЗДАЙТЕ НОВОЕ ПРАВИЛО' : '';
  for (const tab of facilityRuleTabs.querySelectorAll('[role="tab"]')) {
    const open = tab.dataset.kind === facilityRuleKind;
    tab.setAttribute('aria-selected', String(open));
    tab.tabIndex = open ? 0 : -1;
    if (reason && !open) tab.setAttribute('aria-describedby', 'facilityRuleTabReason');
    else tab.removeAttribute('aria-describedby');
    explainControl(tab, open ? '' : reason);
    if (open) facilityRuleSentence.setAttribute('aria-labelledby', tab.id);
  }
  document.getElementById('facilityRuleTabReason').textContent = reason;
  for (const clause of facilityRuleSentence.querySelectorAll('.facility-rule-clause')) {
    clause.hidden = !type.clauses.includes(clause.dataset.clause);
  }
  document.getElementById('facilityRuleOtherStates').textContent = type.otherStates;
}

// selectFacilityRuleTab opens the tab of kind on a new rule. The device, the
// state, and the text stay, and so does an element the tab offers (FR-026).
function selectFacilityRuleTab(kind, focus = false) {
  const tab = facilityRuleTabs.querySelector(`[data-kind="${kind}"]`);
  if (focus) tab.focus();
  if (controlBlocked(tab) || kind === facilityRuleKind) return;
  facilityRuleKind = kind;
  renderFacilityRuleTabs();
  updateFacilityBindingNodes();
  updateFacilityBindingBlocks();
  showFacilityRuleConflict(null);
  setFacilityFeedback(facilityBindingError);
}

// openFacilityBindingDialog opens the display rule dialog on binding, or on
// a new rule for target's element. A new rule opens on «ТЕКСТ ЗАПИСИ» when
// that element is a record with blocks, and on «НАЗВАНИЕ В МЕНЮ» otherwise.
function openFacilityBindingDialog(binding = null, target = binding) {
  editingFacilityBinding = binding;
  facilityBindingForm.reset();
  facilityRuleElements = facilityRuleElementCatalog();
  const index = target ? facilityRuleElements.findIndex(element => element.terminalId === target.terminal.id
    && element.nodeId === target.node.id) : -1;
  const record = index >= 0 && FACILITY_RULE_TYPES['entry-content'].offers(facilityRuleElements[index]);
  facilityRuleKind = binding?.property || (record ? 'entry-content' : 'menu-name');
  renderFacilityRuleTabs();
  updateFacilityBindingNodes(index >= 0 ? String(index) : '');
  updateFacilityBindingBlocks(binding?.block?.id || '');
  populateFacilityDeviceSelect(facilityBindingDevice, binding?.variant.when.deviceId || selectedFacilityDeviceID);
  updateFacilityBindingStates();
  if (binding) facilityBindingState.value = binding.variant.when.stateId;
  facilityBindingText.value = binding?.variant.text || '';
  showFacilityRuleConflict(null);
  setFacilityFeedback(facilityBindingError);
  showFacilityDialog(facilityBindingDialog, facilityRuleTabs.querySelector('[aria-selected="true"]'));
}

// findFacilityRuleConflict finds the saved rule the draft contradicts
// (FR-028, R6): a name or block text list driven by another device, a
// repeated state, or an element that already has the equality. The edited
// rule is left out.
function findFacilityRuleConflict(key, deviceID, stateID) {
  const editing = editingFacilityBinding && JSON.stringify(facilityBindingKey(editingFacilityBinding));
  const rules = facilityBindingCatalog().filter(binding => binding.terminal.id === key.terminalID
    && binding.node.id === key.nodeID && binding.property === key.property
    && (binding.block?.id || '') === key.blockID && JSON.stringify(facilityBindingKey(binding)) !== editing);
  const equality = ({ deviceId, stateId }) => {
    const device = facilityDeviceByID(deviceId);
    return `${device?.name || deviceId} = ${facilityStateName(device, stateId)}`;
  };
  if (['visibility', 'availability'].includes(key.property)) {
    const [existing] = rules;
    return existing ? {
      binding: existing,
      message: `У элемента уже есть правило «${FACILITY_RULE_TYPES[key.property].name}»: ${equality(existing.variant.when)}. Измените его вместо нового.`,
    } : null;
  }
  const other = rules.find(binding => binding.variant.when.deviceId !== deviceID);
  if (other) {
    const device = facilityDeviceByID(other.variant.when.deviceId)?.name || other.variant.when.deviceId;
    return {
      binding: other,
      message: key.property === 'menu-name'
        ? `Название этого элемента уже зависит от устройства «${device}». Все правила одного элемента должны зависеть от одного устройства.`
        : `Текст этого блока уже зависит от устройства «${device}». Все правила одного блока должны зависеть от одного устройства.`,
    };
  }
  const repeated = rules.find(binding => binding.variant.when.stateId === stateID);
  return repeated ? {
    binding: repeated, message: `Для «${equality(repeated.variant.when)}» у этого элемента уже есть правило.`,
  } : null;
}

// showFacilityRuleConflict names the conflicting rule and offers to open it,
// or clears the notice when conflict is null.
function showFacilityRuleConflict(conflict) {
  facilityRuleConflictBinding = conflict?.binding || null;
  facilityRuleConflict.textContent = conflict?.message || '';
  facilityRuleConflict.hidden = !conflict;
  btnOpenConflictingFacilityRule.hidden = !conflict;
}

// saveFacilityRule writes value as the rule key names in session. An edited
// rule that stays in its list is replaced at its index, so the variant order
// stays (R5); one moved to another element leaves its old list.
function saveFacilityRule(session, key, editing, value) {
  const inPlace = Boolean(editing) && editing.terminalID === key.terminalID
    && editing.nodeID === key.nodeID && editing.blockID === key.blockID;
  if (editing && !inPlace) removeFacilityBinding(session, editing);
  const terminal = session.terminals.find(candidate => candidate.id === key.terminalID);
  const node = terminal ? locateNode(terminal.root, key.nodeID)?.node : null;
  if (!node) return;
  const rule = structuredClone(value);
  if (key.property === 'visibility') {
    node.visibleWhen = rule;
  } else if (key.property === 'availability') {
    node.availableWhen = rule;
  } else {
    const owner = key.property === 'menu-name' ? node : node.blocks?.find(block => block.id === key.blockID);
    if (!owner) return;
    const variants = key.property === 'menu-name'
      ? (owner.facilityNameVariants ||= []) : (owner.facilityTextVariants ||= []);
    if (inPlace) variants.splice(editing.index, 1, rule);
    else variants.push(rule);
  }
}

// facilityActionCommands lists the commands an action can belong to: every
// command without a terminal transition (R7).
function facilityActionCommands() {
  return facilityNodeCatalog().filter(({ node }) => node.type === 'command' && !node.terminalTransition);
}

// facilityCommandActionChoice is the command the action dialog edits, as
// { terminal, node } in the newest session, or null.
function facilityCommandActionChoice() {
  const choice = facilityCommandActionFixed || (facilityCommandActionCommand.value
    ? facilityCommandActionCommands[Number(facilityCommandActionCommand.value)] : null);
  const terminal = choice && state.session.terminals.find(candidate => candidate.id === choice.terminalId);
  const node = terminal ? locateNode(terminal.root, choice.nodeId)?.node : null;
  return node?.type === 'command' ? { terminal, node } : null;
}

// openFacilityCommandActionDialog opens «ДЕЙСТВИЕ КОМАНДЫ» on the command
// nodeID of terminalID (R7). The command is fixed unless pick is set, as it is
// when no command is given; «КОМАНДА» then starts on nodeID, or on the first
// command without an action. texts prefill the player texts that a command
// without stateChange needs, and opener gets focus back on close.
function openFacilityCommandActionDialog(terminalID = '', nodeID = '', {
  pick = !nodeID, texts = {}, opener = btnAddFacilityActionBinding,
} = {}) {
  facilityCommandActionForm.reset();
  facilityCommandActionReturn = opener;
  facilityCommandActionFixed = pick ? null : { terminalId: terminalID, nodeId: nodeID };
  const commands = pick ? facilityActionCommands() : [];
  facilityCommandActionCommands = commands.map(({ terminal, node }) => ({ terminalId: terminal.id, nodeId: node.id }));
  facilityCommandActionCommand.replaceChildren(new Option('ВЫБЕРИТЕ КОМАНДУ', ''));
  for (const [index, { terminal, node }] of commands.entries()) {
    const option = new Option(facilityNodePath(terminal, node.id), String(index));
    option.dataset.terminalId = terminal.id;
    option.dataset.nodeId = node.id;
    facilityCommandActionCommand.appendChild(option);
  }
  const start = commands.findIndex(({ terminal, node }) => (nodeID
    ? terminal.id === terminalID && node.id === nodeID : !node.stateChange?.facilityAction));
  if (start >= 0) facilityCommandActionCommand.value = String(start);
  facilityCommandActionCompletedName.value = texts.completedName || '';
  facilityCommandActionConfirmationText.value = texts.confirmationText || '';
  const choice = facilityCommandActionChoice();
  facilityCommandActionCommandFixed.textContent = choice && !pick ? facilityNodePath(choice.terminal, nodeID) : '';
  facilityCommandActionCommandFixed.hidden = pick;
  facilityCommandActionCommand.hidden = !pick;
  facilityCommandActionForm.querySelector('label[for="facilityCommandActionCommand"]').hidden = !pick;
  showFacilityCommandAction(choice?.node.stateChange?.facilityAction);
  setFacilityFeedback(facilityCommandActionError);
  showFacilityDialog(facilityCommandActionDialog,
    pick ? facilityCommandActionCommand : facilityCommandActionMode.querySelector(':checked'));
}

// showFacilityCommandAction fills the draft from action, the chosen
// command's stored action: its mode, its transition rows and its program.
function showFacilityCommandAction(action) {
  const programID = action?.recoveryProgramId || '';
  (programID ? facilityCommandActionModeProgram : facilityCommandActionModeTransitions).checked = true;
  facilityCommandActionTransitions.replaceChildren();
  for (const value of action?.transitions?.transitions || []) {
    appendFacilityActionRequest(value, facilityCommandActionTransitions);
  }
  populateSelect(facilityCommandActionProgram, authoredFacility().recoveryPrograms, programID, 'ВЫБЕРИТЕ ПРОГРАММУ');
  renderFacilityCommandActionDraft();
}

// renderFacilityCommandActionDraft shows the fields of the chosen mode, with
// one row for the selected device in an empty transition list, and the
// player texts while the chosen command has no stateChange yet.
function renderFacilityCommandActionDraft() {
  const transitions = facilityCommandActionModeTransitions.checked;
  if (transitions && !facilityCommandActionTransitions.children.length) {
    appendFacilityActionRequest({ deviceId: selectedFacilityDeviceID }, facilityCommandActionTransitions);
  }
  facilityCommandActionTransitions.hidden = !transitions;
  btnAddFacilityCommandActionTransition.hidden = !transitions;
  const choice = facilityCommandActionChoice();
  const texts = Boolean(choice) && !choice.node.stateChange;
  for (const [control, shown] of [[facilityCommandActionProgram, !transitions],
    [facilityCommandActionCompletedName, texts], [facilityCommandActionConfirmationText, texts]]) {
    control.hidden = !shown;
    facilityCommandActionForm.querySelector(`label[for="${control.id}"]`).hidden = !shown;
  }
}

// closeFacilityCommandActionDialog returns focus to the opener, to the
// opener the command form redrew under the same id, or to «ДОБАВИТЬ ДЕЙСТВИЕ
// КОМАНДЫ».
function closeFacilityCommandActionDialog() {
  const opener = facilityCommandActionReturn;
  const target = opener?.isConnected ? opener : (opener?.id && document.getElementById(opener.id));
  hideFacilityDialog(facilityCommandActionDialog, target || btnAddFacilityActionBinding);
}

// FACILITY_DEPENDENCY_KINDS gives each dependency kind the destination of its
// button and, for a display rule, its facilityBindingCatalog property (R13).
// Any other kind has no destination.
const FACILITY_DEPENDENCY_KINDS = Object.freeze({
  'transition-precondition': { destination: 'device' },
  'transition-condition-effect': { destination: 'device' },
  'recovery-reference': { destination: 'condition' },
  'diagnostic-scope': { destination: 'condition' },
  'diagnostic-effect': { destination: 'condition' },
  'recovery-program-transition': { destination: 'program' },
  'command-action': { destination: 'command' },
  'name-variant': { destination: 'rule', property: 'menu-name' },
  'entry-content-variant': { destination: 'rule', property: 'entry-content' },
  visibility: { destination: 'rule', property: 'visibility' },
  availability: { destination: 'rule', property: 'availability' },
});

// facilityDependencyIndex reads the list index in a dependency's property,
// such as 1 in «preconditions[1].deviceId», or 0 when it has none.
function facilityDependencyIndex(dependency) {
  return Number(/\[(\d+)\]/.exec(dependency.property)?.[1] || 0);
}

// facilityDependencySource finds what a dependency starts from in session:
// a transition with its device, a fault, a program, a command with its
// terminal, or a display rule as its facilityBindingCatalog entry. It
// returns {} when the source is gone.
function facilityDependencySource(dependency, session = state.session) {
  const { destination, property } = FACILITY_DEPENDENCY_KINDS[dependency.kind] || {};
  const facility = session?.facility;
  const terminal = session?.terminals?.find(value => value.id === dependency.terminalId);
  if (destination === 'device') {
    // IDs may contain '/', so the source is matched rather than split.
    for (const device of facility?.devices || []) {
      const transition = device.transitions?.find(value => `${device.id}/${value.id}` === dependency.sourceId);
      if (transition) return { device, transition };
    }
  } else if (destination === 'condition') {
    const condition = facility?.conditions?.find(value => value.id === dependency.sourceId);
    if (condition) return { condition };
  } else if (destination === 'program') {
    const program = facility?.recoveryPrograms?.find(value => value.id === dependency.sourceId);
    if (program) return { program };
  } else if (destination === 'command') {
    const node = terminal && locateNode(terminal.root, dependency.sourceId)?.node;
    if (node) return { terminal, node };
  } else if (destination === 'rule' && terminal) {
    // A block text variant names its block, not its record.
    const location = property === 'entry-content' ? entryBlockLocation(terminal, dependency.sourceId) : null;
    const nodeID = location ? location.entry.id : dependency.sourceId;
    const index = facilityDependencyIndex(dependency);
    const binding = facilityBindingCatalog(session).find(value => value.terminal === terminal
      && value.node.id === nodeID && value.block === location?.block
      && value.property === property && value.index === index);
    if (binding) return { binding };
  }
  return {};
}

// describeFacilityDependency words one link to the inspected device, the
// dependency's target (FR-037, R13). sentence says what depends on what,
// place says where by name, and destination is where its button leads. A
// name that is gone falls back to its raw ID, and then place is empty.
function describeFacilityDependency(dependency, session = state.session) {
  const { destination = 'none', property } = FACILITY_DEPENDENCY_KINDS[dependency.kind] || {};
  const { device, transition, condition, program, terminal, node, binding } = facilityDependencySource(dependency, session);
  const index = facilityDependencyIndex(dependency);
  const inspected = facilityDeviceByID(dependency.targetId, session);
  const equality = when => `${inspected?.name || dependency.targetId} = ${facilityStateName(inspected, when?.stateId)}`;
  const name = value => value?.name || dependency.sourceId;
  const transitionName = request => facilityDeviceByID(request?.deviceId, session)?.transitions
    ?.find(value => value.id === request.transitionId)?.name || request?.transitionId || '—';
  const described = (sentence, place) => ({ sentence, place: place || '', destination });
  const transitionSource = `Переход «${name(transition)}» устройства «${name(device)}»`;
  const transitionPlace = transition && `${device.name} › ${transition.name}`;
  switch (dependency.kind) {
    case 'transition-precondition':
      return described(`${transitionSource} требует: ${equality(transition?.preconditions?.[index])}.`, transitionPlace);
    case 'transition-condition-effect': {
      const fault = session?.facility?.conditions?.find(value => value.id === dependency.targetId);
      return described(`${transitionSource} включает или снимает неисправность «${fault?.name || dependency.targetId}».`,
        transitionPlace);
    }
    case 'recovery-reference': {
      const request = condition?.recovery?.[index]?.transition;
      return described(`Неисправность «${name(condition)}» устраняется переходом «${transitionName(request)}».`, condition?.name);
    }
    case 'diagnostic-scope':
      return described(`Неисправность «${name(condition)}» действует на это устройство.`, condition?.name);
    case 'diagnostic-effect':
      return described(`Неисправность «${name(condition)}» меняет работу терминала.`, condition?.name);
    case 'recovery-program-transition': {
      const request = program?.transitions?.[index];
      return described(`Программа восстановления «${name(program)}» запускает переход «${transitionName(request)}».`,
        program?.name);
    }
    case 'command-action': {
      const request = node?.stateChange?.facilityAction?.transitions?.transitions?.[index];
      return described(`Команда «${name(node)}» запускает переход «${transitionName(request)}».`,
        node && facilityNodePath(terminal, node.id));
    }
    default: {
      if (!property) return described('Другая ссылка на устройство.');
      const rule = binding || { property, node: { name: dependency.sourceId } };
      const block = binding?.block ? ` › блок ${binding.node.blocks.indexOf(binding.block) + 1}` : '';
      return described(facilityRuleUseSentence(rule, equality(binding?.variant.when)),
        binding && `${facilityNodePath(binding.terminal, binding.node.id)}${block}`);
    }
  }
}

// facilityDependencyRow builds one link row: the sentence, its place, a
// button named for its destination, and the technical path under a collapsed
// disclosure. The button leaves dialog, or says in error that the source is
// gone.
function facilityDependencyRow(dependency, dialog, error) {
  const { sentence, place, destination } = describeFacilityDependency(dependency);
  const row = document.createElement('div');
  row.className = 'facility-dependency-row';
  row.setAttribute('role', 'listitem');
  row.dataset.kind = dependency.kind;
  row.dataset.destination = destination;
  const text = document.createElement('span');
  text.className = 'facility-dependency-sentence';
  text.textContent = sentence;
  row.appendChild(text);
  if (place) {
    const where = document.createElement('span');
    where.className = 'facility-dependency-place';
    where.textContent = place;
    row.appendChild(where);
  }
  if (destination !== 'none') {
    const { transition, condition, program, node, binding } = facilityDependencySource(dependency);
    const source = (transition || condition || program || node || binding?.node)?.name || dependency.sourceId;
    const navigate = document.createElement('button');
    navigate.type = 'button';
    navigate.className = 'btn btn-mini btn-secondary facility-dependency-navigation';
    navigate.textContent = FACILITY_DESTINATION_LABELS[destination];
    navigate.setAttribute('aria-label', `${navigate.textContent}: ${source}`);
    navigate.addEventListener('click', () => navigateToFacilityDependency(dependency, dialog, error));
    row.appendChild(navigate);
  }
  const onTerminal = dependency.terminalId ? ` · терминал ${dependency.terminalId}` : '';
  facilityTechnicalDetails(row,
    `${dependency.kind} · ${dependency.sourceId} → ${dependency.targetId} · ${dependency.property}${onTerminal}`);
  return row;
}

function renderFacilityDependencies(report) {
  const dependencies = report?.dependencies || [];
  facilityDependencyList.replaceChildren(...dependencies
    .map(dependency => facilityDependencyRow(dependency, facilityDependencyDialog, facilityDependencyError)));
  facilityDependencyStatus.textContent = dependencies.length
    ? `НАЙДЕНО ССЫЛОК: ${dependencies.length}` : 'ССЫЛКИ НЕ НАЙДЕНЫ';
}

// navigateToFacilityDependency follows a link's button out of dialog in the
// newest session. A source that is gone keeps dialog open and says in error
// what to do next.
function navigateToFacilityDependency(dependency, dialog, error) {
  const { device, transition, condition, program, terminal, node, binding } = facilityDependencySource(dependency);
  const open = {
    device: transition && (() => openFacilityDeviceTransition(device.id, transition.id)),
    condition: condition && (() => openFacilityConditionDialog(condition.id)),
    program: program && (() => openFacilityRecoveryProgramDialog(program.id)),
    command: node && (() => showFacilityCommand(terminal.id, node.id)),
    rule: binding && (() => openFacilityBindingDialog(binding)),
  }[FACILITY_DEPENDENCY_KINDS[dependency.kind]?.destination];
  if (!open) {
    setFacilityFeedback(error, 'Связанный элемент больше не существует. Закройте окно и откройте зависимости снова.');
    return;
  }
  hideFacilityDialog(dialog);
  open();
}

// showFacilityCommand opens the «ТЕРМИНАЛЫ» workspace on a command, with focus
// on the control that edits its «ДЕЙСТВИЕ КОМАНДЫ».
function showFacilityCommand(terminalID, nodeID) {
  state.editTerminalId = terminalID;
  state.selectedNodeId = nodeID;
  selectWorkspace('terminal');
  renderAll();
  document.getElementById('btnEditCommandFacilityAction')?.focus();
}

async function inspectSelectedFacilityDevice() {
  const device = facilityDeviceByID(selectedFacilityDeviceID, state.session);
  if (!device) return null;
  document.getElementById('facilityDependencyDialogDescription').textContent =
    `Что в локации зависит от устройства «${device.name}».`;
  facilityDependencyStatus.textContent = 'ПОИСК ЗАВИСИМОСТЕЙ…';
  setFacilityFeedback(facilityDependencyError);
  showFacilityDialog(facilityDependencyDialog, btnCloseFacilityDependencies);
  const result = await desktopAPI.inspectFacilityDependencies({
    target: { kind: 'device', entityId: device.id },
    expectedSessionRevision: currentSessionRevision(),
    expectedFacilityRevision: Number(state.session.facility?.revision || 0),
  });
  if (!result?.ok || !result.report) {
    facilityDependencyStatus.textContent = '';
    setFacilityFailure(facilityDependencyError, result);
    return null;
  }
  renderFacilityDependencies(result.report);
  return result.report;
}

function replacementTransitionID(device, transitionID) {
  return device.transitions?.some(transition => transition.id === transitionID)
    ? transitionID : (device.transitions?.[0]?.id || transitionID);
}

function replacementStateID(device, stateID) {
  return device.states?.some(value => value.id === stateID) ? stateID : device.initialStateId;
}

function rewriteFacilityDeviceReferences(session, oldID, replacementID) {
  const replacement = facilityDeviceByID(replacementID, session);
  let changes = 0;
  const equality = value => {
    if (value?.deviceId !== oldID) return;
    value.deviceId = replacementID;
    value.stateId = replacementStateID(replacement, value.stateId);
    changes++;
  };
  const request = value => {
    if (value?.deviceId !== oldID) return;
    value.deviceId = replacementID;
    value.transitionId = replacementTransitionID(replacement, value.transitionId);
    changes++;
  };
  for (const device of session.facility.devices || []) {
    for (const transition of device.transitions || []) {
      (transition.preconditions || []).forEach(equality);
    }
  }
  for (const condition of session.facility.conditions || []) {
    if (condition.device?.deviceId === oldID) {
      condition.device.deviceId = replacementID;
      changes++;
    }
    for (const recovery of condition.recovery || []) request(recovery.transition);
  }
  for (const program of session.facility.recoveryPrograms || []) (program.transitions || []).forEach(request);
  for (const { node } of facilityNodeCatalog(session)) {
    (node.facilityNameVariants || []).forEach(variant => equality(variant.when));
    equality(node.visibleWhen);
    equality(node.availableWhen);
    for (const block of node.blocks || []) {
      (block.facilityTextVariants || []).forEach(variant => equality(variant.when));
    }
    const action = node.stateChange?.facilityAction;
    (action?.transitions?.transitions || []).forEach(request);
  }
  return changes;
}

function prepareFacilityRepair(report) {
  const source = facilityDeviceByID(selectedFacilityDeviceID);
  if (!source) return;
  const dependencies = report?.dependencies || [];
  populateSelect(facilityRepairTarget,
    authoredFacility().devices.filter(device => device.id !== source.id), '', 'ВЫБЕРИТЕ УСТРОЙСТВО');
  pendingFacilityRepair = { sourceID: source.id, dependencies, candidate: null };
  setFacilityFeedback(facilityRepairError, dependencies.length
    ? `Устройство используется: ${dependencies.length} ссылок. Выполните полное переназначение.`
    : 'ССЫЛКИ НЕ НАЙДЕНЫ. УСТРОЙСТВО МОЖНО УДАЛИТЬ.');
  facilityRepairDependencyList.replaceChildren(...dependencies
    .map(dependency => facilityDependencyRow(dependency, facilityRepairDialog, facilityRepairError)));
  facilityRepairDependencyList.hidden = !dependencies.length;
  facilityRepairImpact.textContent = '';
  btnDeleteFacilityWithoutRepair.disabled = dependencies.length > 0;
  btnApplyFacilityRepair.disabled = true;
  btnApplyFacilityRepair.hidden = true;
  hideFacilityDialog(facilityDependencyDialog);
  showFacilityDialog(facilityRepairDialog, dependencies.length ? facilityRepairTarget : btnDeleteFacilityWithoutRepair);
}

async function openFacilityRepair() {
  const report = await inspectSelectedFacilityDevice();
  if (report) prepareFacilityRepair(report);
}

function validateFacilityRepair() {
  if (!pendingFacilityRepair) return;
  const replacementID = facilityRepairTarget.value;
  if (!replacementID) {
    facilityRepairImpact.textContent = 'ВЫБЕРИТЕ УСТРОЙСТВО ДЛЯ ПЕРЕНАЗНАЧЕНИЯ';
    btnApplyFacilityRepair.disabled = true;
    btnApplyFacilityRepair.hidden = true;
    return;
  }
  const candidate = structuredClone(state.session);
  const changed = rewriteFacilityDeviceReferences(candidate, pendingFacilityRepair.sourceID, replacementID);
  pendingFacilityRepair.replacementID = replacementID;
  const { sourceID } = pendingFacilityRepair;
  const name = id => `«${facilityDeviceByID(id)?.name || id}»`;
  facilityRepairImpact.replaceChildren(
    `ПЕРЕНАЗНАЧЕНО ССЫЛОК: ${changed} · УДАЛЯЕТСЯ ${name(sourceID)} · НАЗНАЧАЕТСЯ ${name(replacementID)}`);
  facilityTechnicalDetails(facilityRepairImpact, `${sourceID} → ${replacementID}`);
  btnApplyFacilityRepair.disabled = false;
  btnApplyFacilityRepair.hidden = false;
}

// applyFacilityDeletion deletes the repaired device, first moving its
// references to replacementID when one is given.
function applyFacilityDeletion(replacementID) {
  const sourceID = pendingFacilityRepair?.sourceID;
  if (!sourceID) return;
  const name = facilityDeviceByID(sourceID)?.name || sourceID;
  selectedFacilityDeviceID = '';
  pendingFacilityRepair = null;
  hideFacilityDialog(facilityRepairDialog, btnAddFacilityDevice);
  commitFacilityEdit(`Удаление «${name}»`, session => {
    if (replacementID) rewriteFacilityDeviceReferences(session, sourceID, replacementID);
    const facility = ensureFacility(session);
    facility.devices = facility.devices.filter(device => device.id !== sourceID);
  });
}

// appendFacilityActionRequest adds one device and transition row to
// container, the list of transitions one action runs together. The add
// button follows container, and a removed row hands it focus.
function appendFacilityActionRequest(value, container) {
  const row = document.createElement('div');
  row.facilityOriginal = structuredClone(value);
  row.className = 'facility-transition-row facility-action-request-row';
  row.setAttribute('role', 'listitem');
  const deviceLabel = document.createElement('label');
  deviceLabel.textContent = 'УСТРОЙСТВО';
  const device = document.createElement('select');
  device.className = 'field-input facility-action-device';
  populateFacilityDeviceSelect(device, value.deviceId || '');
  deviceLabel.appendChild(device);
  const transitionLabel = document.createElement('label');
  transitionLabel.textContent = 'ПЕРЕХОД';
  const transition = document.createElement('select');
  transition.className = 'field-input facility-action-transition';
  populateSelect(transition, facilityDeviceByID(device.value)?.transitions || [], value.transitionId || '', 'ВЫБЕРИТЕ ПЕРЕХОД');
  transitionLabel.appendChild(transition);
  const remove = document.createElement('button');
  remove.className = 'btn btn-danger facility-action-remove';
  remove.type = 'button';
  remove.textContent = 'УДАЛИТЬ';
  remove.addEventListener('click', () => {
    row.remove();
    refreshFacilityActionRows(container);
    container.nextElementSibling?.focus();
  });
  row.append(deviceLabel, transitionLabel, remove);
  row.addEventListener('change', () => refreshFacilityActionRows(container));
  container.appendChild(row);
  refreshFacilityActionRows(container);
  return row;
}

// refreshFacilityActionRows offers each row only the devices no other row
// uses, since one action moves a device once, and names each row on its
// remove button.
function refreshFacilityActionRows(container) {
  const rows = [...container.querySelectorAll('.facility-action-request-row')];
  const chosen = rows.map(row => row.querySelector('.facility-action-device').value);
  rows.forEach((row, index) => {
    const device = row.querySelector('.facility-action-device');
    const transition = row.querySelector('.facility-action-transition');
    const used = new Set(chosen.filter((id, other) => id && other !== index));
    populateSelect(device, (authoredFacility()?.devices || []).filter(value => !used.has(value.id))
      .map(value => ({ id: value.id, name: value.name })), device.value, 'ВЫБЕРИТЕ УСТРОЙСТВО');
    const selected = facilityDeviceByID(device.value);
    populateSelect(transition, selected?.transitions || [], transition.value, 'ВЫБЕРИТЕ ПЕРЕХОД');
    const named = selected?.transitions?.find(value => value.id === transition.value);
    row.querySelector('.facility-action-remove').setAttribute('aria-label',
      named ? `Удалить переход: ${selected.name} — ${named.name}` : 'Удалить переход');
  });
}

// validateFacilityActionRows checks what the rows leave open: a chosen
// device and transition in every row, and no fault that one row turns on and
// another clears.
function validateFacilityActionRows(container) {
  const effects = new Map();
  for (const row of container.querySelectorAll('.facility-action-request-row')) {
    const device = row.querySelector('.facility-action-device');
    const transition = row.querySelector('.facility-action-transition');
    const target = facilityDeviceByID(device.value);
    if (!target) return ['ВЫБЕРИТЕ УСТРОЙСТВО ПЕРЕХОДА', device];
    const chosen = (target.transitions || []).find(value => value.id === transition.value);
    if (!chosen) return ['ВЫБЕРИТЕ ПЕРЕХОД', transition];
    for (const effect of chosen.conditionEffects || []) {
      const active = effect.active === true;
      if (effects.has(effect.conditionId) && effects.get(effect.conditionId) !== active) {
        const fault = authoredFacility().conditions.find(value => value.id === effect.conditionId);
        return [`Один переход включает неисправность «${fault?.name || effect.conditionId}», а другой снимает её. Уберите один из них.`, transition];
      }
      effects.set(effect.conditionId, active);
    }
  }
  return null;
}

function readFacilityActionRows(container) {
  return [...container.querySelectorAll('.facility-action-request-row')].map(row => ({
    ...row.facilityOriginal,
    deviceId: row.querySelector('.facility-action-device').value,
    transitionId: row.querySelector('.facility-action-transition').value,
  }));
}

for (const button of document.querySelectorAll('[data-story-step]')) {
  button.addEventListener('click', () => setFacilityStoryStep(Number(button.dataset.storyStep)));
}
// Only the author's own toggle is a choice; the rule's toggles fire no click.
document.getElementById('facilityStoryGuideSummary').addEventListener('click', () => {
  facilityGuideChoice = document.getElementById('facilityStoryGuide').open ? 'collapsed' : 'expanded';
});
document.getElementById('facilityStoryGuide').addEventListener('toggle', renderFacilityGuideSummary);
document.getElementById('btnFacilityStoryBack').addEventListener('click', () => setFacilityStoryStep(facilityStoryStep - 1, true));
document.getElementById('btnFacilityStoryNext').addEventListener('click', () => setFacilityStoryStep(facilityStoryStep + 1, true));

terminalWorkspaceTab.addEventListener('click', () => selectWorkspace('terminal'));
facilityWorkspaceTab.addEventListener('click', () => selectWorkspace('facility'));
document.getElementById('facilityDeviceSearch')?.addEventListener('input', renderFacilityWorkspace);
document.getElementById('facilityDeviceFilters').addEventListener('click', event => {
  const filter = event.target.closest('[data-filter]');
  if (!filter) return;
  facilityDeviceFilter = filter.dataset.filter;
  renderFacilityWorkspace();
});
// The device card and the terminal group share the fault switch (FR-021).
for (const list of ['facilityPlayFaults', 'facilityTerminalFaultList']) {
  document.getElementById(list).addEventListener('click', event => {
    const toggle = event.target.closest('.facility-play-fault-switch');
    if (!toggle || controlBlocked(toggle)) return;
    const item = toggle.closest('li[data-condition-id]');
    void runFacilityFaultSwitch(item.dataset.conditionId, item.dataset.active !== 'true');
  });
}
document.getElementById('facilityTerminalFaultList').addEventListener('click', event => {
  const open = event.target.closest('.facility-terminal-fault-open');
  if (open) openFacilityConditionDialog(open.closest('.facility-terminal-fault').dataset.conditionId);
});
document.getElementById('facilityPlayTransitions').addEventListener('click', event => {
  const run = event.target.closest('.facility-play-transition-run');
  if (!run || controlBlocked(run)) return;
  void runFacilityTransition(selectedFacilityDeviceID, run.closest('.facility-play-transition').dataset.transitionId);
});
document.getElementById('btnClearFacilitySelection')?.addEventListener('click', () => {
  selectedFacilityDeviceID = '';
  renderFacilityWorkspace();
});
for (const tab of [terminalWorkspaceTab, facilityWorkspaceTab]) {
  tab.addEventListener('keydown', event => {
    if (!['ArrowLeft', 'ArrowRight'].includes(event.key)) return;
    event.preventDefault();
    const next = tab === terminalWorkspaceTab ? facilityWorkspaceTab : terminalWorkspaceTab;
    selectWorkspace(next === facilityWorkspaceTab ? 'facility' : 'terminal');
    next.focus();
  });
}

btnAddFacilityDevice.addEventListener('click', () => openFacilityDeviceDialog());
btnApplyFacilityDeviceTemplate.addEventListener('click', () => {
  if (controlBlocked(btnApplyFacilityDeviceTemplate)) return;
  applyFacilityDraftTemplate(facilityDraftTemplate());
});
btnAddFacilityState.addEventListener('click', () => {
  const names = new Set(facilityDraftStateRows().map(row => row.facilityName.trim().toLowerCase()));
  let name = 'Новое состояние';
  for (let number = 2; names.has(name.toLowerCase()); number++) name = `Новое состояние ${number}`;
  openFacilityStateEditor(addFacilityDraftState(name), { select: true });
});
facilityDeviceStateChips.addEventListener('click', event => {
  const chip = event.target.closest('.facility-state-chip');
  if (!chip) return;
  if (facilityStateEditor.facilityState === chip.facilityState) closeFacilityStateEditor();
  else openFacilityStateEditor(chip.facilityState);
});
facilityStateEditorName.addEventListener('input', () => {
  const row = facilityStateEditor.facilityState;
  renameFacilityDraftState(row, facilityStateEditorName.value);
  if (row.facilityProblem?.control === facilityStateEditorName) delete row.facilityProblem;
  clearFacilityRowProblem(facilityStateEditor);
});
btnMakeFacilityStateInitial.addEventListener('click', () => {
  if (controlBlocked(btnMakeFacilityStateInitial)) return;
  for (const row of facilityDraftStateRows()) row.dataset.initial = String(row === facilityStateEditor.facilityState);
  renderFacilityDeviceDraft();
});
// A delete press removes an unused state at once, asks before taking this
// device's transitions with it, and lists any outside use instead (R10).
btnDeleteFacilityState.addEventListener('click', () => {
  if (controlBlocked(btnDeleteFacilityState)) return;
  const row = facilityStateEditor.facilityState;
  const transitions = facilityStateTransitions(row);
  const uses = facilityStateOutsideUses(row, transitions);
  resetFacilityStateDelete();
  if (uses.length) {
    showFacilityUses(facilityStateUses, uses);
    return;
  }
  if (!transitions.length) {
    removeFacilityDraftState(row);
    return;
  }
  document.getElementById('facilityStateDeleteSummary').textContent = 'Вместе с состоянием удалятся переходы: '
    + `${transitions.map(transition => `«${facilityDraftTransitionLabel(transition)}»`).join(', ')}.`;
  facilityStateDeleteConfirm.hidden = false;
  document.getElementById('btnConfirmDeleteFacilityState').focus();
});
document.getElementById('btnConfirmDeleteFacilityState').addEventListener('click', () => {
  removeFacilityDraftState(facilityStateEditor.facilityState);
});
document.getElementById('btnCancelDeleteFacilityState').addEventListener('click', () => {
  facilityStateDeleteConfirm.hidden = true;
  btnDeleteFacilityState.focus();
});
facilityTransitionMatrix.addEventListener('click', event => {
  const cell = event.target.closest('.facility-transition-cell');
  if (cell) chooseFacilityTransitionCell(cell.facilitySource, cell.facilityDestination);
});
facilityTransitionCellList.addEventListener('click', event => {
  const choice = event.target.closest('.facility-transition-choice');
  if (choice) openFacilityTransitionPanel(choice.facilityTransition);
});
btnAddFacilityCellTransition.addEventListener('click', () => {
  const { source, destination } = facilityTransitionPanel.facilityCell;
  openFacilityTransitionPanel(addFacilityDraftTransition(source, destination), { select: true });
});
btnCloseFacilityTransitionPanel.addEventListener('click', closeFacilityTransitionPanel);
facilityDeviceForm.addEventListener('input', renderFacilityDeviceDraft);
// facilityDeviceDialogReturn is where focus goes when the device dialog
// closes: the edit button for an existing device, or the add button.
function facilityDeviceDialogReturn() {
  const edit = document.getElementById('btnEditFacilityDevice');
  return editingFacilityDeviceID && !facilitySelectionEditor.hidden ? edit : btnAddFacilityDevice;
}
btnCancelFacilityDevice.addEventListener('click', () => closeFacilityDeviceDialog(facilityDeviceDialogReturn()));
facilityDeviceDialog.addEventListener('cancel', event => {
  event.preventDefault();
  closeFacilityDeviceDialog(facilityDeviceDialogReturn());
});
facilityDeviceForm.addEventListener('submit', event => {
  event.preventDefault();
  clearFacilityDeviceProblems();
  const device = readFacilityDeviceDraft();
  const invalid = validateFacilityDeviceDraft(device);
  if (invalid) {
    const [message, field, problems = []] = invalid;
    revealFacilityDeviceField(field, problems[0]?.owner);
    setFacilityFeedback(facilityDeviceError, message, field);
    showFacilityRowProblems(problems);
    return;
  }
  const editingID = editingFacilityDeviceID;
  const opener = facilityDeviceDialogReturn();
  selectedFacilityDeviceID = device.id;
  closeFacilityDeviceDialog(opener);
  commitFacilityEdit(`Устройство «${device.name}»`, session => {
    const facility = ensureFacility(session);
    const index = editingID ? facility.devices.findIndex(candidate => candidate.id === editingID) : -1;
    if (index >= 0) facility.devices.splice(index, 1, structuredClone(device));
    else facility.devices.push(structuredClone(device));
  }, (_reason, result) => {
    editingFacilityDeviceID = editingID;
    showFacilityDialog(facilityDeviceDialog);
    setFacilityFailure(facilityDeviceError, result);
  });
});
facilityDeviceName.addEventListener('input', () => {
  if (facilityDeviceID.dataset.manual === 'true') return;
  facilityDeviceID.value = suggestFacilityID('device', facilityDeviceName.value,
    new Set((authoredFacility()?.devices || []).map(device => device.id)));
});
facilityDeviceID.addEventListener('input', () => { facilityDeviceID.dataset.manual = 'true'; });

btnAddFacilityCondition.addEventListener('click', () => openFacilityConditionDialog());
btnCancelFacilityCondition.addEventListener('click', closeFacilityConditionDialog);
facilityConditionDialog.addEventListener('cancel', event => {
  event.preventDefault();
  closeFacilityConditionDialog();
});
// A picker keeps its own redraw, so a choice it is making stays selected.
facilityConditionForm.addEventListener('input', event => {
  if (![facilityFaultActivationPicker, facilityFaultClearingPicker].includes(event.target)) renderFacilityFaultDraft();
});
for (const [add, picker, active] of [
  [btnAddFaultActivation, facilityFaultActivationPicker, true],
  [btnAddFaultClearing, facilityFaultClearingPicker, false],
]) {
  add.addEventListener('click', () => {
    picker.value = '';
    picker.hidden = false;
    picker.focus();
  });
  picker.addEventListener('change', () => {
    const option = picker.selectedOptions[0];
    if (!option?.value) return;
    facilityFaultEdits.set(facilityTransitionKey(option.dataset.deviceId, option.dataset.transitionId), active);
    picker.hidden = true;
    renderFacilityFaultDraft();
    add.focus();
  });
  // Escape closes the picker, not the dialog.
  picker.addEventListener('keydown', event => {
    if (event.key !== 'Escape') return;
    event.preventDefault();
    picker.hidden = true;
    add.focus();
  });
}
btnAddFaultRecoveryProgram.addEventListener('click', () => {
  const [first] = facilityFaultTransitions(false);
  openFacilityRecoveryProgramDialog('', {
    opener: btnAddFaultRecoveryProgram,
    faultName: facilityConditionName.value.trim(),
    request: first && { deviceId: first.device.id, transitionId: first.transition.id },
  });
});
// One «ГОТОВО» saves the fault, the effects of every transition its lists
// touched, and the recovery entries those effects stop backing (R2, R3).
facilityConditionForm.addEventListener('submit', event => {
  event.preventDefault();
  const id = facilityConditionID.value.trim();
  const name = facilityConditionName.value.trim();
  const original = authoredFacility().conditions.find(condition => condition.id === editingFacilityConditionID);
  if (!name) return setFacilityFeedback(facilityConditionError, 'УКАЖИТЕ НАЗВАНИЕ НЕИСПРАВНОСТИ', facilityConditionName);
  if (!facilityConditionCategory.value) {
    return setFacilityFeedback(facilityConditionError, 'ВЫБЕРИТЕ КАТЕГОРИЮ НЕИСПРАВНОСТИ', facilityConditionCategory);
  }
  const terminalScope = facilityConditionScope.value === 'terminal';
  if ((!terminalScope && !facilityConditionDevice.value) || (terminalScope && !facilityConditionTerminal.value)) {
    return setFacilityFeedback(facilityConditionError, terminalScope ? 'ВЫБЕРИТЕ ТЕРМИНАЛ' : 'ВЫБЕРИТЕ УСТРОЙСТВО',
      terminalScope ? facilityConditionTerminal : facilityConditionDevice);
  }
  const effects = structuredClone(original?.effects || []);
  const capabilityIndex = effects.findIndex(effect => effect.capabilityBlock);
  if (facilityConditionCapability.value) {
    const previous = capabilityIndex >= 0 ? effects[capabilityIndex] : {};
    const capability = {
      ...previous,
      capabilityBlock: {
        ...previous.capabilityBlock,
        capability: facilityConditionCapability.value,
      },
    };
    if (capabilityIndex >= 0) effects.splice(capabilityIndex, 1, capability);
    else effects.push(capability);
  } else if (capabilityIndex >= 0) {
    effects.splice(capabilityIndex, 1);
  }
  if (!effects.length) {
    return setFacilityFeedback(facilityConditionError, 'ВЫБЕРИТЕ, ЧТО БЛОКИРУЕТ НЕИСПРАВНОСТЬ', facilityConditionCapability);
  }
  const conflict = terminalScope ? facilityConditionScopeConflict(effects, facilityConditionTerminal.value) : '';
  if (conflict) return setFacilityFeedback(facilityConditionError, conflict, facilityConditionTerminal);
  if (!id || authoredFacility().conditions.some(condition => condition.id === id && condition.id !== editingFacilityConditionID)) {
    return setFacilityFeedback(facilityConditionError, 'УКАЖИТЕ УНИКАЛЬНЫЙ ИДЕНТИФИКАТОР НЕИСПРАВНОСТИ', facilityConditionID);
  }
  const condition = {
    ...structuredClone(original || {}),
    id, name, category: facilityConditionCategory.value, effects,
    initialActive: facilityConditionInitialActive.checked,
  };
  // Only a new fault starts in its starting activity; an existing one keeps its live value.
  if (!original) condition.currentActive = condition.initialActive;
  if (condition.category === 'custom') condition.customCategory ||= condition.id;
  else delete condition.customCategory;
  if (terminalScope) condition.terminal = { ...original?.terminal, terminalId: facilityConditionTerminal.value };
  else condition.device = { ...original?.device, deviceId: facilityConditionDevice.value };
  if (terminalScope) delete condition.device;
  else delete condition.terminal;
  const edits = new Map(facilityFaultEdits);
  const originalID = original?.id || '';
  const editingID = editingFacilityConditionID;
  closeFacilityConditionDialog();
  commitFacilityEdit(`Неисправность «${condition.name}»`, session => {
    const facility = ensureFacility(session);
    const index = originalID ? facility.conditions.findIndex(candidate => candidate.id === originalID) : -1;
    // The dialog never edits recovery entries, so the newest ones stay.
    const saved = { ...structuredClone(condition), recovery: facility.conditions[index]?.recovery };
    if (!saved.recovery) delete saved.recovery;
    if (index >= 0) facility.conditions.splice(index, 1, saved);
    else facility.conditions.push(saved);
    pruneFacilityRecovery(facility, () => applyFacilityFaultEdits(facility, condition.id, edits));
  }, (_reason, result) => {
    editingFacilityConditionID = editingID;
    showFacilityDialog(facilityConditionDialog);
    renderFacilityFaultDraft();
    setFacilityFailure(facilityConditionError, result);
  });
});
facilityConditionName.addEventListener('input', () => {
  if (facilityConditionID.dataset.manual === 'true') return;
  facilityConditionID.value = suggestFacilityID('condition', facilityConditionName.value,
    new Set((authoredFacility()?.conditions || []).map(condition => condition.id)));
});
facilityConditionID.addEventListener('input', () => { facilityConditionID.dataset.manual = 'true'; });

btnAddFacilityRecoveryProgram.addEventListener('click', () => openFacilityRecoveryProgramDialog());
btnAddFacilityProgramTransition.addEventListener('click', () => {
  appendFacilityActionRequest({}, facilityProgramTransitions).querySelector('.facility-action-device').focus();
});
btnCancelFacilityRecoveryProgram.addEventListener('click', closeFacilityRecoveryProgramDialog);
facilityRecoveryProgramDialog.addEventListener('cancel', event => {
  event.preventDefault();
  closeFacilityRecoveryProgramDialog();
});
// A program saves as its own edit, also over the fault dialog, whose lists
// then re-derive (R4). Faults whose recovery entries named the program lose
// the entries the edit stops backing (R2).
facilityRecoveryProgramForm.addEventListener('submit', event => {
  event.preventDefault();
  const id = facilityRecoveryProgramID.value.trim();
  const name = facilityRecoveryProgramName.value.trim();
  if (!name) return setFacilityFeedback(facilityRecoveryProgramError, 'УКАЖИТЕ НАЗВАНИЕ ПРОГРАММЫ', facilityRecoveryProgramName);
  if (!facilityProgramTransitions.children.length) {
    return setFacilityFeedback(facilityRecoveryProgramError, 'ДОБАВЬТЕ ХОТЯ БЫ ОДИН ПЕРЕХОД', btnAddFacilityProgramTransition);
  }
  const invalid = validateFacilityActionRows(facilityProgramTransitions);
  if (invalid) return setFacilityFeedback(facilityRecoveryProgramError, ...invalid);
  if (!id || authoredFacility().recoveryPrograms.some(program => program.id === id && program.id !== editingFacilityProgramID)) {
    return setFacilityFeedback(facilityRecoveryProgramError, 'УКАЖИТЕ УНИКАЛЬНЫЙ ИДЕНТИФИКАТОР ПРОГРАММЫ', facilityRecoveryProgramID);
  }
  const original = authoredFacility().recoveryPrograms.find(value => value.id === editingFacilityProgramID);
  const program = {
    ...structuredClone(original || {}),
    id, name, transitions: readFacilityActionRows(facilityProgramTransitions),
  };
  const originalID = original?.id || '';
  const editingID = editingFacilityProgramID;
  const opener = facilityProgramReturn;
  const fromFault = facilityProgramFromFault;
  commitFacilityEdit(`Программа «${program.name}»`, session => {
    const facility = ensureFacility(session);
    pruneFacilityRecovery(facility, () => {
      const index = originalID ? facility.recoveryPrograms.findIndex(candidate => candidate.id === originalID) : -1;
      if (index >= 0) facility.recoveryPrograms.splice(index, 1, structuredClone(program));
      else facility.recoveryPrograms.push(structuredClone(program));
    });
  }, (_reason, result) => {
    editingFacilityProgramID = editingID;
    facilityProgramReturn = opener;
    facilityProgramFromFault = fromFault;
    showFacilityDialog(facilityRecoveryProgramDialog);
    setFacilityFailure(facilityRecoveryProgramError, result);
  });
  closeFacilityRecoveryProgramDialog();
});
facilityRecoveryProgramName.addEventListener('input', () => {
  if (facilityRecoveryProgramID.dataset.manual === 'true') return;
  facilityRecoveryProgramID.value = suggestFacilityID('program', facilityRecoveryProgramName.value,
    new Set((authoredFacility()?.recoveryPrograms || []).map(program => program.id)));
});
facilityRecoveryProgramID.addEventListener('input', () => { facilityRecoveryProgramID.dataset.manual = 'true'; });

btnAddFacilityBinding.addEventListener('click', () => openFacilityBindingDialog());
facilityRuleTabs.addEventListener('click', event => {
  const tab = event.target.closest('[role="tab"]');
  if (tab) selectFacilityRuleTab(tab.dataset.kind);
});
facilityRuleTabs.addEventListener('keydown', event => {
  const tabs = [...facilityRuleTabs.querySelectorAll('[role="tab"]')];
  const index = tabs.indexOf(event.target.closest('[role="tab"]'));
  const next = {
    ArrowRight: (index + 1) % tabs.length,
    ArrowLeft: (index - 1 + tabs.length) % tabs.length,
    Home: 0,
    End: tabs.length - 1,
  }[event.key];
  if (next === undefined) return;
  event.preventDefault();
  selectFacilityRuleTab(tabs[next].dataset.kind, true);
});
facilityBindingDevice.addEventListener('change', updateFacilityBindingStates);
facilityBindingNode.addEventListener('change', () => updateFacilityBindingBlocks());
// A conflict notice describes the inputs it was found for; any edit clears it.
document.getElementById('facilityRuleSentence').addEventListener('input', () => showFacilityRuleConflict(null));
btnOpenConflictingFacilityRule.addEventListener('click', () => openFacilityBindingDialog(facilityRuleConflictBinding));
btnCancelFacilityBinding.addEventListener('click', () => hideFacilityDialog(facilityBindingDialog, btnAddFacilityBinding));
facilityBindingDialog.addEventListener('cancel', event => {
  event.preventDefault();
  hideFacilityDialog(facilityBindingDialog, btnAddFacilityBinding);
});
facilityBindingForm.addEventListener('submit', event => {
  event.preventDefault();
  showFacilityRuleConflict(null);
  const kind = facilityRuleKind;
  const { clauses } = FACILITY_RULE_TYPES[kind];
  const device = facilityDeviceByID(facilityBindingDevice.value);
  if (!device) return setFacilityFeedback(facilityBindingError, 'ВЫБЕРИТЕ УСТРОЙСТВО', facilityBindingDevice);
  const stateID = facilityBindingState.value;
  if (!device.states.some(value => value.id === stateID)) {
    return setFacilityFeedback(facilityBindingError, 'ВЫБЕРИТЕ СОСТОЯНИЕ', facilityBindingState);
  }
  const element = facilityRuleElement();
  if (!element) return setFacilityFeedback(facilityBindingError, 'ВЫБЕРИТЕ ЭЛЕМЕНТ', facilityBindingNode);
  const terminal = state.session.terminals.find(candidate => candidate.id === element.terminalId);
  const node = terminal ? locateNode(terminal.root, element.nodeId)?.node : null;
  if (!node) {
    return setFacilityFeedback(facilityBindingError,
      `Элемент «${element.name}» удалён. Выберите другой элемент в «ГДЕ».`, facilityBindingNode);
  }
  const blockID = clauses.includes('block') ? facilityBindingBlock.value : '';
  if (clauses.includes('block') && !node.blocks?.some(block => block.id === blockID)) {
    return setFacilityFeedback(facilityBindingError, 'ВЫБЕРИТЕ БЛОК', facilityBindingBlock);
  }
  const text = facilityBindingText.value.trim();
  if (clauses.includes('text') && !text) return setFacilityFeedback(facilityBindingError, 'ВВЕДИТЕ ТЕКСТ', facilityBindingText);
  setFacilityFeedback(facilityBindingError);
  const key = { terminalID: terminal.id, nodeID: node.id, blockID, property: kind };
  const conflict = findFacilityRuleConflict(key, device.id, stateID);
  if (conflict) return showFacilityRuleConflict(conflict);
  // A field the tab does not show is not saved; variant extras are kept.
  const editing = editingFacilityBinding;
  const when = { ...editing?.variant.when, deviceId: device.id, stateId: stateID };
  const value = clauses.includes('text') ? { ...structuredClone(editing?.variant || {}), when, text } : when;
  const editingKey = editing ? facilityBindingKey(editing) : null;
  hideFacilityDialog(facilityBindingDialog, btnAddFacilityBinding);
  commitFacilityEdit(`Правило «${node.name}»`, session => saveFacilityRule(session, key, editingKey, value), (_reason, result) => {
    showFacilityDialog(facilityBindingDialog);
    setFacilityFailure(facilityBindingError, result);
  });
  renderNodeForm();
});

btnAddFacilityActionBinding.addEventListener('click', () => openFacilityCommandActionDialog());
// Choosing another command shows its action, or starts a new one.
facilityCommandActionCommand.addEventListener('change', () => {
  showFacilityCommandAction(facilityCommandActionChoice()?.node.stateChange?.facilityAction);
  setFacilityFeedback(facilityCommandActionError);
});
facilityCommandActionMode.addEventListener('change', renderFacilityCommandActionDraft);
btnAddFacilityCommandActionTransition.addEventListener('click', () => {
  appendFacilityActionRequest({}, facilityCommandActionTransitions).querySelector('.facility-action-device').focus();
});
document.getElementById('btnCancelFacilityCommandAction').addEventListener('click', closeFacilityCommandActionDialog);
facilityCommandActionDialog.addEventListener('cancel', event => {
  event.preventDefault();
  closeFacilityCommandActionDialog();
});
// One «ГОТОВО» writes the command's action, and the player texts of a
// command without stateChange. The other stateChange fields stay.
facilityCommandActionForm.addEventListener('submit', event => {
  event.preventDefault();
  const choice = facilityCommandActionChoice();
  const commandField = facilityCommandActionFixed ? null : facilityCommandActionCommand;
  if (!choice) {
    return setFacilityFeedback(facilityCommandActionError,
      facilityCommandActionFixed ? 'Команда больше не существует. Закройте окно.' : 'ВЫБЕРИТЕ КОМАНДУ', commandField);
  }
  if (choice.node.terminalTransition) {
    return setFacilityFeedback(facilityCommandActionError,
      'Команда переходит в другой терминал. Сначала примените режим «ИЗМЕНЯЕТ СОСТОЯНИЕ».', commandField);
  }
  const program = facilityCommandActionModeProgram.checked;
  const programID = facilityCommandActionProgram.value;
  if (program && !authoredFacility().recoveryPrograms.some(value => value.id === programID)) {
    return setFacilityFeedback(facilityCommandActionError, 'ВЫБЕРИТЕ ПРОГРАММУ', facilityCommandActionProgram);
  }
  if (!program && !facilityCommandActionTransitions.children.length) {
    return setFacilityFeedback(facilityCommandActionError, 'ДОБАВЬТЕ ХОТЯ БЫ ОДИН ПЕРЕХОД', btnAddFacilityCommandActionTransition);
  }
  const invalid = program ? null : validateFacilityActionRows(facilityCommandActionTransitions);
  if (invalid) return setFacilityFeedback(facilityCommandActionError, ...invalid);
  const completedName = facilityCommandActionCompletedName.value.trim();
  const confirmationText = facilityCommandActionConfirmationText.value.trim();
  if (!choice.node.stateChange && (!completedName || !confirmationText)) {
    return setFacilityFeedback(facilityCommandActionError, 'ЗАПОЛНИТЕ НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ И ЗАПРОС ПОДТВЕРЖДЕНИЯ',
      completedName ? facilityCommandActionConfirmationText : facilityCommandActionCompletedName);
  }
  const action = program ? { recoveryProgramId: programID }
    : { transitions: { transitions: readFacilityActionRows(facilityCommandActionTransitions) } };
  const terminalID = choice.terminal.id;
  const nodeID = choice.node.id;
  const fixed = facilityCommandActionFixed;
  const commands = facilityCommandActionCommands;
  const opener = facilityCommandActionReturn;
  commitFacilityEdit(`Действие команды «${choice.node.name}»`, session => {
    const terminal = session.terminals.find(candidate => candidate.id === terminalID);
    const node = terminal ? locateNode(terminal.root, nodeID)?.node : null;
    if (!node) return;
    node.stateChange ||= { completedName, confirmationText };
    node.stateChange.facilityAction = structuredClone(action);
  }, (_reason, result) => {
    facilityCommandActionFixed = fixed;
    facilityCommandActionCommands = commands;
    facilityCommandActionReturn = opener;
    showFacilityDialog(facilityCommandActionDialog);
    setFacilityFailure(facilityCommandActionError, result);
  });
  renderNodeForm();
  closeFacilityCommandActionDialog();
});

btnInspectFacilityDependencies.addEventListener('click', () => { void inspectSelectedFacilityDevice(); });
btnCloseFacilityDependencies.addEventListener('click', () => hideFacilityDialog(facilityDependencyDialog, btnInspectFacilityDependencies));
facilityDependencyDialog.addEventListener('cancel', event => {
  event.preventDefault();
  hideFacilityDialog(facilityDependencyDialog, btnInspectFacilityDependencies);
});
btnDeleteFacilityDevice.addEventListener('click', () => {
  closeFacilityDeviceMenu();
  void openFacilityRepair();
});
document.getElementById('btnEditFacilityDevice').addEventListener('click', () => openFacilityDeviceDialog(selectedFacilityDeviceID));
btnFacilityDeviceMenu.addEventListener('click', () => {
  if (facilityDeviceMenu.hidden) openFacilityDeviceMenu();
  else closeFacilityDeviceMenu();
});
btnFacilityDeviceMenu.addEventListener('keydown', event => {
  if (event.key !== 'ArrowDown') return;
  event.preventDefault();
  openFacilityDeviceMenu();
});
facilityDeviceMenu.addEventListener('keydown', event => {
  if (event.key === 'Escape') {
    event.preventDefault();
    closeFacilityDeviceMenu(true);
  } else if (event.key === 'Tab') {
    closeFacilityDeviceMenu();
  }
});
document.addEventListener('click', event => {
  if (!event.target.closest('.facility-device-menu')) closeFacilityDeviceMenu();
});
document.getElementById('facilityDetailTabs').addEventListener('click', event => {
  const tab = event.target.closest('[role="tab"]');
  if (tab) selectFacilityDetailTab(tab.dataset.tab);
});
document.getElementById('facilityDetailTabs').addEventListener('keydown', event => {
  const ids = facilityDetailTabs.map(([id]) => id);
  const index = ids.indexOf(facilityDetailTab);
  const next = {
    ArrowRight: (index + 1) % ids.length,
    ArrowLeft: (index - 1 + ids.length) % ids.length,
    Home: 0,
    End: ids.length - 1,
  }[event.key];
  if (next === undefined) return;
  event.preventDefault();
  selectFacilityDetailTab(ids[next], true);
});
btnPreviewFacilityDevice.addEventListener('click', () => {
  if (!controlBlocked(btnPreviewFacilityDevice)) openFacilityPreview('device', btnPreviewFacilityDevice);
});
btnPreviewFacilityCondition.addEventListener('click', () => {
  if (!controlBlocked(btnPreviewFacilityCondition)) openFacilityPreview('condition', btnPreviewFacilityCondition);
});
btnRefreshFacilityPreview.addEventListener('click', () => { void refreshFacilityPreview(); });
facilityPreviewTerminal.addEventListener('change', () => { void refreshFacilityPreview(); });
facilityPreviewState.addEventListener('change', () => { void refreshFacilityPreview(); });
facilityPreviewActive.addEventListener('change', () => { void refreshFacilityPreview(); });
btnCloseFacilityPreview.addEventListener('click', closeFacilityPreview);
facilityPreviewDialog.addEventListener('cancel', event => {
  event.preventDefault();
  closeFacilityPreview();
});
btnResetFacilityDevice.addEventListener('click', () => {
  if (controlBlocked(btnResetFacilityDevice)) return;
  openFacilityConfirmation(facilityDeviceResetDialog, btnResetFacilityDevice, btnCancelFacilityDeviceReset);
});
btnCancelFacilityDeviceReset.addEventListener('click', () => closeFacilityConfirmation(facilityDeviceResetDialog));
btnForceFacilityDeviceState.addEventListener('click', () => {
  if (controlBlocked(btnForceFacilityDeviceState)) return;
  openFacilityForceStateDialog();
});
facilityForceStateTarget.addEventListener('change', renderFacilityForceStateSummary);
document.getElementById('btnCancelFacilityForceState').addEventListener('click', closeFacilityForceStateDialog);
btnConfirmFacilityForceState.addEventListener('click', () => { void confirmFacilityForceState(); });
facilityForceStateDialog.addEventListener('cancel', event => {
  event.preventDefault();
  closeFacilityForceStateDialog();
});
btnConfirmFacilityDeviceReset.addEventListener('click', () => { void resetSelectedFacilityDevice(); });
facilityDeviceResetDialog.addEventListener('cancel', event => {
  event.preventDefault();
  closeFacilityConfirmation(facilityDeviceResetDialog);
});
btnResetScene.addEventListener('click', openSceneResetDialog);
btnCancelSceneReset.addEventListener('click', closeSceneResetDialog);
btnConfirmSceneReset.addEventListener('click', () => { void confirmSceneReset(); });
sceneResetDialog.addEventListener('cancel', event => {
  event.preventDefault();
  closeSceneResetDialog();
});
btnCancelFacilityRepair.addEventListener('click', () => {
  pendingFacilityRepair = null;
  setFacilityFeedback(facilityRepairError);
  hideFacilityDialog(facilityRepairDialog, btnDeleteFacilityDevice);
});
facilityRepairDialog.addEventListener('cancel', event => {
  event.preventDefault();
  pendingFacilityRepair = null;
  setFacilityFeedback(facilityRepairError);
  hideFacilityDialog(facilityRepairDialog, btnDeleteFacilityDevice);
});
facilityRepairTarget.addEventListener('change', () => {
  pendingFacilityRepair && (pendingFacilityRepair.replacementID = '');
  btnApplyFacilityRepair.disabled = true;
  btnApplyFacilityRepair.hidden = true;
  facilityRepairImpact.textContent = '';
});
btnValidateFacilityRepair.addEventListener('click', validateFacilityRepair);
btnApplyFacilityRepair.addEventListener('click', () => {
  if (pendingFacilityRepair?.replacementID) applyFacilityDeletion(pendingFacilityRepair.replacementID);
});
btnDeleteFacilityWithoutRepair.addEventListener('click', () => {
  if (!pendingFacilityRepair || pendingFacilityRepair.dependencies.length) return;
  applyFacilityDeletion('');
});

// ── Render: everything ──────────────────────────────────────
function renderAll() {
  renderTermList();
  renderTreeHeader();
  renderSettingsPanel();
  renderToolbarState();
  renderTree();
  renderNodeForm();
  renderToolbarHint();
  renderHackStatus();
  renderCoordination();
  if (!facilityWorkspace.hidden) renderFacilityWorkspace();
  renderSaveStatus();
}

// ── Render: authoritative roster and broadcast state ────────
function renderCoordination() {
  const coordination = state.coordination;
  const roster = Array.isArray(coordination?.roster) ? coordination.roster : [];
  const sessions = Array.isArray(coordination?.sessions) ? coordination.sessions : [];
  const broadcast = coordination?.broadcast || null;
  const playerConfig = coordination?.playerConfig || null;
  const availableCharacters = roster.filter(character => !character.claimedBySessionId);
  const unassignedSessions = sessions.filter(session => !session.character);

  broadcastSummary.textContent = broadcast
    ? (broadcast.activeTerminalId
      ? `ТРАНСЛЯЦИЯ АКТИВНА · ТЕРМИНАЛ: ${terminalName(broadcast.activeTerminalId)}`
      : 'ТРАНСЛЯЦИЯ АКТИВНА · ОЖИДАНИЕ ТЕРМИНАЛА')
    : 'ТРАНСЛЯЦИЯ НЕ ЗАПУЩЕНА';
  broadcastSummary.classList.toggle('is-live', Boolean(broadcast));
  coordinationPanel.dataset.playerConfigActive = String(Boolean(playerConfig));
  playerConfigStatus.dataset.active = String(Boolean(playerConfig));
  playerConfigStatus.textContent = playerConfig
    ? `${playerConfig.name} · ${playerConfig.filePath}`
    : 'НЕ ВЫБРАНА · СОЗДАЙТЕ ИЛИ ВЫБЕРИТЕ ФАЙЛ';
  btnOpenPlayerConfig.disabled = coordinationCommandPending || Boolean(broadcast);
  btnNewPlayerConfig.disabled = coordinationCommandPending || Boolean(broadcast);
  btnManagePlayers.disabled = !playerConfig;
  btnStartBroadcast.disabled = coordinationCommandPending || Boolean(broadcast) || !playerConfig;
  btnEndBroadcast.hidden = !broadcast;
  btnEndBroadcast.disabled = coordinationCommandPending || !broadcast;
  activeLogicalSessionCount.textContent = String(sessions.filter(session => Boolean(session.connected)).length);

  renderPlayerManagementRoster(roster, Boolean(broadcast), Boolean(playerConfig));

  logicalSessionList.replaceChildren();
  if (!sessions.length) {
    const empty = document.createElement('div');
    empty.className = 'session-empty';
    empty.setAttribute('role', 'listitem');
    empty.textContent = 'СЕССИИ НЕ ПОДКЛЮЧЕНЫ';
    logicalSessionList.appendChild(empty);
    return;
  }

  for (const session of sessions) {
    const fragment = logicalSessionRowTemplate.content.cloneNode(true);
    const row = fragment.querySelector('.session-row');
    const assigned = Boolean(session.character);
    const role = session.role || 'unassigned';
    row.dataset.sessionId = session.id;
    row.dataset.connected = String(Boolean(session.connected));
    row.dataset.role = role;
    row.querySelector('.session-primary-name').textContent = assigned
      ? session.character.name
      : session.fallbackName;
    const presence = row.querySelector('.session-presence');
    presence.dataset.presence = session.connected ? 'connected' : 'disconnected';
    presence.textContent = session.connected ? 'ПОДКЛЮЧЕН' : 'ОТКЛЮЧЕН';
    const roleLabel = row.querySelector('.session-role');
    roleLabel.dataset.sessionRole = role;
    roleLabel.textContent = role === 'active'
      ? (session.connected ? 'УПРАВЛЯЮЩИЙ' : 'УПРАВЛЯЮЩИЙ · НЕТ СВЯЗИ')
      : role === 'observer' ? 'НАБЛЮДАТЕЛЬ' : 'БЕЗ РОЛИ';
    row.querySelector('.session-character-name').textContent = assigned
      ? `ПЕРСОНАЖ: ${session.character.name}`
      : 'ПЕРСОНАЖ НЕ НАЗНАЧЕН';
    row.querySelector('.session-fallback-label').textContent = `СЕССИЯ: ${session.fallbackName}`;

    const nameInput = row.querySelector('.session-name-input');
    nameInput.value = session.fallbackName || '';
    const renameButton = row.querySelector('.session-rename');
    const assignmentControls = row.querySelector('.session-assignment-controls');
    const characterSelect = row.querySelector('.session-character-select');
    const assignButton = row.querySelector('.session-assign');
    const claimedControls = row.querySelector('.session-claimed-controls');
    const releaseButton = row.querySelector('.session-release');
    const controllerButton = row.querySelector('.session-controller');
    const moveSelect = row.querySelector('.session-move-session-select');
    const moveButton = row.querySelector('.session-move');
    for (const control of row.querySelectorAll('input, select, button')) {
      control.disabled = coordinationCommandPending;
    }
    renameButton.addEventListener('click', () => {
      const currentSession = findLogicalSession(session.id);
      if (coordinationCommandPending || !currentSession) return;
      const fallbackName = nameInput.value.trim();
      if (!fallbackName) return setCoordinationStatus('УКАЖИТЕ МЕТКУ СЕССИИ', true);
      runCoordinationCommand(
        () => desktopAPI.renameLogicalSession({ sessionId: session.id, fallbackName }),
        'МЕТКА СЕССИИ ОБНОВЛЕНА',
        'ПЕРЕИМЕНОВАНИЕ СЕССИИ...',
        'players'
      );
    });

    assignmentControls.hidden = assigned || !broadcast;
    fillSelect(characterSelect, availableCharacters, character => character.id, character => character.name, 'НЕТ ДОСТУПНЫХ ПЕРСОНАЖЕЙ');
    assignButton.disabled = coordinationCommandPending || assigned || !broadcast || !characterSelect.value;
    assignButton.addEventListener('click', () => {
      const currentSession = findLogicalSession(session.id);
      const currentCharacter = findRosterCharacter(characterSelect.value);
      if (coordinationCommandPending || !state.coordination?.broadcast || currentSession?.character ||
          !currentCharacter || currentCharacter.claimedBySessionId) return;
      runCoordinationCommand(
        () => desktopAPI.assignCharacter({ sessionId: session.id, characterId: characterSelect.value }),
        'ПЕРСОНАЖ НАЗНАЧЕН',
        'НАЗНАЧЕНИЕ ПЕРСОНАЖА...',
        'players'
      );
    });

    claimedControls.hidden = !assigned;
    releaseButton.disabled = coordinationCommandPending || !assigned;
    releaseButton.addEventListener('click', () => {
      if (coordinationCommandPending || !findLogicalSession(session.id)?.character) return;
      runCoordinationCommand(
        () => desktopAPI.releaseCharacter(session.id),
        'ПЕРСОНАЖ ОСВОБОЖДЁН',
        'ОСВОБОЖДЕНИЕ ПЕРСОНАЖА...',
        'players'
      );
    });
    controllerButton.disabled = true;
    controllerButton.hidden = role === 'active';
    if (assigned && session.connected && role !== 'active') {
      controllerButton.disabled = coordinationCommandPending;
      controllerButton.addEventListener('click', () => {
        const currentSession = findLogicalSession(session.id);
        if (coordinationCommandPending || !currentSession?.character || !currentSession.connected ||
            currentSession.role === 'active') return;
        runCoordinationCommand(
          () => desktopAPI.setActiveController(session.id),
          'УПРАВЛЕНИЕ ПЕРЕДАНО',
          'ПЕРЕДАЧА УПРАВЛЕНИЯ...',
          'players'
        );
      });
    }

    fillSelect(moveSelect, unassignedSessions, candidate => candidate.id, candidate => sessionLabel(candidate), 'НЕТ СВОБОДНЫХ СЕССИЙ');
    moveSelect.disabled = coordinationCommandPending || !assigned || !playerConfig || !moveSelect.value;
    moveButton.disabled = coordinationCommandPending || !assigned || !playerConfig || !moveSelect.value;
    moveButton.addEventListener('click', () => {
      const currentSession = findLogicalSession(session.id);
      const destination = findLogicalSession(moveSelect.value);
      if (coordinationCommandPending || !state.coordination?.playerConfig ||
          !currentSession?.character || destination?.character || !destination) return;
      runCoordinationCommand(
        () => desktopAPI.moveCharacter({
          characterId: currentSession.character.id,
          toSessionId: destination.id,
        }),
        'НАЗНАЧЕНИЕ ПЕРЕМЕЩЕНО',
        'ПЕРЕМЕЩЕНИЕ НАЗНАЧЕНИЯ...',
        'players'
      );
    });
    logicalSessionList.appendChild(fragment);
  }
}

function findLogicalSession(sessionId) {
  const sessions = Array.isArray(state.coordination?.sessions) ? state.coordination.sessions : [];
  return sessions.find(session => session.id === sessionId) || null;
}

function findRosterCharacter(characterId) {
  const roster = Array.isArray(state.coordination?.roster) ? state.coordination.roster : [];
  return roster.find(character => character.id === characterId) || null;
}

function renderPlayerManagementRoster(roster, broadcastActive, playerConfigActive) {
  const readOnly = broadcastActive || !playerConfigActive;
  playerManagementDialog.setAttribute('aria-readonly', String(readOnly));
  playerManagementMode.textContent = broadcastActive
    ? 'ТРАНСЛЯЦИЯ АКТИВНА · ПРОСМОТР БЕЗ РЕДАКТИРОВАНИЯ'
    : playerConfigActive ? 'РЕДАКТИРОВАНИЕ ДОСТУПНО' : 'КОНФИГУРАЦИЯ ИГРОКОВ НЕ ВЫБРАНА';

  const addDisabled = coordinationCommandPending || readOnly;
  for (const control of playerManagementAddForm.elements) control.disabled = addDisabled;
  syncPlayerDeleteDialogControls();
  playerManagementRoster.replaceChildren();
  playerManagementEmpty.hidden = roster.length > 0;

  for (const character of roster) {
    const fragment = playerManagementRowTemplate.content.cloneNode(true);
    const row = fragment.querySelector('.player-management-row');
    const nameInput = row.querySelector('.player-name-input');
    const intelligenceInput = row.querySelector('.player-intelligence-input');
    const hackerInput = row.querySelector('.player-hacker-perk-availability');
    row.dataset.characterId = character.id;
    nameInput.value = character.name || '';
    intelligenceInput.value = String(Number.isInteger(character.intelligence) ? character.intelligence : 1);
    hackerInput.value = character.hackerPerkAvailable === true ? 'true' : 'false';
    const saveButton = row.querySelector('.player-save');
    const deleteButton = row.querySelector('.player-delete');
    for (const control of row.querySelectorAll('input, select, button')) {
      control.disabled = coordinationCommandPending || readOnly;
    }
    saveButton.addEventListener('click', async () => {
      if (!playerMutationAllowed()) return;
      const name = nameInput.value.trim();
      const intelligence = intelligenceInput.valueAsNumber;
      const hackerChoice = hackerInput.value;
      if (!name) {
        setPlayerManagementFeedback('УКАЖИТЕ ИМЯ ИГРОКА', true);
        nameInput.focus();
        return;
      }
      if (!Number.isInteger(intelligence) || intelligence < 1 || intelligence > 10) {
        setPlayerManagementFeedback('ИНТЕЛЛЕКТ ДОЛЖЕН БЫТЬ ЦЕЛЫМ ЧИСЛОМ ОТ 1 ДО 10', true);
        intelligenceInput.focus();
        return;
      }
      if (hackerChoice !== 'true' && hackerChoice !== 'false') {
        setPlayerManagementFeedback('ВЫБЕРИТЕ ДОСТУПНОСТЬ ПЕРКА «ХАКЕР»', true);
        hackerInput.focus();
        return;
      }
      await runPlayerManagementMutation(
        () => desktopAPI.updateCharacter({
          characterId: character.id,
          name,
          intelligence,
          hackerPerkAvailable: hackerChoice === 'true',
          expectedRevision: coordinationRevision(state.coordination),
        }),
        'ПРОФИЛЬ ИГРОКА СОХРАНЁН',
        'СОХРАНЕНИЕ ПРОФИЛЯ ИГРОКА...'
      );
    });
    deleteButton.addEventListener('click', () => {
      if (!playerMutationAllowed()) return;
      showPlayerDeleteConfirmation(character.id, character.name || '', deleteButton);
    });
    playerManagementRoster.appendChild(fragment);
  }
}

function playerMutationAllowed() {
  return !coordinationCommandPending &&
    !state.coordination?.broadcast &&
    Boolean(state.coordination?.playerConfig);
}

function syncPlayerDeleteDialogControls() {
  if (!pendingPlayerDelete) return;
  const active = Boolean(state.coordination?.broadcast);
  const configMissing = !state.coordination?.playerConfig;
  btnConfirmPlayerDelete.disabled = coordinationCommandPending || active || configMissing;
  btnCancelPlayerDelete.disabled = coordinationCommandPending;
}

function showPlayerDeleteConfirmation(characterId, name, opener) {
  if (!playerMutationAllowed()) return;
  pendingPlayerDelete = { characterId, opener };
  playerDeleteDialogDescription.textContent = `Удалить игрока «${name}»? Это действие нельзя отменить.`;
  playerDeleteDialog.hidden = false;
  syncPlayerDeleteDialogControls();
  if (!playerDeleteDialog.open) playerDeleteDialog.showModal();
  btnCancelPlayerDelete.focus();
}

function hidePlayerDeleteConfirmation(restoreFocus = true) {
  const pending = pendingPlayerDelete;
  pendingPlayerDelete = null;
  if (playerDeleteDialog.open) playerDeleteDialog.close();
  playerDeleteDialog.hidden = true;
  btnConfirmPlayerDelete.disabled = false;
  btnCancelPlayerDelete.disabled = false;
  if (!restoreFocus) return;
  const currentDelete = pending?.characterId
    ? playerManagementRoster.querySelector(`[data-character-id="${CSS.escape(pending.characterId)}"] .player-delete`)
    : null;
  if (currentDelete instanceof HTMLElement) currentDelete.focus();
  else if (pending?.opener?.isConnected) pending.opener.focus();
}

async function runPlayerManagementMutation(command, successMessage, pendingMessage) {
  if (!playerMutationAllowed()) return null;
  coordinationCommandPending = true;
  setPlayerManagementFeedback(pendingMessage);
  setCoordinationStatus(pendingMessage);
  renderCoordination();

  let result;
  try {
    result = await command();
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
  coordinationCommandPending = false;
  if (result?.state) applyCoordinationState(result.state);
  renderCoordination();

  if (!result?.ok) {
    setPlayerManagementError(result);
    setCoordinationError(result, 'players');
    return result;
  }

  setPlayerManagementFeedback(successMessage);
  setCoordinationStatus(successMessage);
  return result;
}

function setPlayerManagementFeedback(message = '', isError = false) {
  playerManagementStatus.textContent = isError ? '' : message;
  playerManagementError.textContent = isError ? message : '';
  playerManagementError.hidden = !isError || !message;
}

function setPlayerManagementError(result) {
  playerManagementStatus.textContent = '';
  renderOverseerError(playerManagementError, result, 'players');
}

function showLogicalSessionManagement() {
  logicalSessionDialogOpener = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : btnManageLogicalSessions;
  setLogicalSessionDialogFeedback('');
  logicalSessionDialog.hidden = false;
  if (typeof logicalSessionDialog.showModal === 'function' && !logicalSessionDialog.open) {
    logicalSessionDialog.showModal();
  } else {
    logicalSessionDialog.setAttribute('open', '');
  }
  queueMicrotask(() => btnCloseLogicalSessions.focus());
}

function hideLogicalSessionManagement() {
  if (typeof logicalSessionDialog.close === 'function' && logicalSessionDialog.open) {
    logicalSessionDialog.close();
  } else {
    logicalSessionDialog.removeAttribute('open');
  }
  logicalSessionDialog.hidden = true;
  const opener = logicalSessionDialogOpener;
  logicalSessionDialogOpener = null;
  if (opener?.isConnected) opener.focus();
}

function showPlayerManagement() {
  if (!state.coordination?.playerConfig || btnManagePlayers.disabled) return;
  playerManagementOpener = document.activeElement instanceof HTMLElement
    ? document.activeElement
    : btnManagePlayers;
  setPlayerManagementFeedback('');
  playerManagementDialog.hidden = false;
  if (!playerManagementDialog.open) playerManagementDialog.showModal();
  queueMicrotask(() => btnClosePlayerManagement.focus());
}

function hidePlayerManagement() {
  if (pendingPlayerDelete) hidePlayerDeleteConfirmation(false);
  if (playerManagementDialog.open) playerManagementDialog.close();
  playerManagementDialog.hidden = true;
  const opener = playerManagementOpener;
  playerManagementOpener = null;
  if (opener?.isConnected) opener.focus();
}

async function addPlayerProfile({ name, intelligence, hackerPerkAvailable }) {
  if (coordinationCommandPending || state.coordination?.broadcast || !state.coordination?.playerConfig) return null;
  coordinationCommandPending = true;
  setPlayerManagementFeedback('ДОБАВЛЕНИЕ ИГРОКА...');
  setCoordinationStatus('ДОБАВЛЕНИЕ ПЕРСОНАЖА...');
  renderCoordination();

  let result;
  try {
    result = await desktopAPI.addCharacter({
      name,
      intelligence,
      hackerPerkAvailable,
      expectedRevision: coordinationRevision(state.coordination),
    });
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
  coordinationCommandPending = false;
  applyCoordinationState(result?.state || state.coordination);

  if (!result?.ok) {
    setPlayerManagementError(result);
    setCoordinationError(result, 'players');
    renderCoordination();
    return result;
  }

  playerManagementAddForm.reset();
  setPlayerManagementFeedback('ИГРОК ДОБАВЛЕН');
  setCoordinationStatus('ПЕРСОНАЖ ДОБАВЛЕН');
  renderCoordination();
  return result;
}

function setPlayerConfigError(message = '') {
  setOverseerSinkText(playerConfigError, message);
  playerConfigError.hidden = !message;
}

async function runPlayerConfigCommand(command, successMessage) {
  if (coordinationCommandPending || state.coordination?.broadcast) return null;
  coordinationCommandPending = true;
  setPlayerConfigError('');
  setCoordinationStatus('ЗАГРУЗКА КОНФИГУРАЦИИ ИГРОКОВ...');
  renderCoordination();
  let result;
  try {
    result = await command();
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
  coordinationCommandPending = false;
  if (result?.canceled) {
    setCoordinationStatus('ВЫБОР КОНФИГУРАЦИИ ОТМЕНЁН');
    setPlayerConfigError('');
    renderCoordination();
    return result;
  }
  if (!result?.ok) {
    setCoordinationError(result, 'player-config');
    renderOverseerError(playerConfigError, result, 'player-config');
    renderCoordination();
    return result;
  }
  if (result.session) applyAuthoredSession(result.session, result.authoringRevision);
  applyCoordinationState(result.state || state.coordination);
  setCoordinationStatus(successMessage);
  setPlayerConfigError('');
  renderAll();
  return result;
}

btnOpenPlayerConfig.addEventListener('click', () => runPlayerConfigCommand(
  () => desktopAPI.openPlayerConfig(),
  'КОНФИГУРАЦИЯ ИГРОКОВ ВЫБРАНА'
));

btnNewPlayerConfig.addEventListener('click', () => runPlayerConfigCommand(
  () => desktopAPI.newPlayerConfig(),
  'КОНФИГУРАЦИЯ ИГРОКОВ СОЗДАНА'
));

function coordinationRevision(coordination) {
  const revision = Number(coordination?.revision || 0);
  return Number.isSafeInteger(revision) && revision >= 0 ? revision : 0;
}

function facilityImpactIDs(values) {
  if (!Array.isArray(values)) return [];
  return values.filter(value => typeof value === 'string' && value.trim()).map(value => value.trim());
}

function renderCommandExecutionFacilityImpact(summary) {
  commandExecutionFacilityImpact.replaceChildren();
  commandExecutionFacilityImpact.hidden = !summary || typeof summary !== 'object';
  if (commandExecutionFacilityImpact.hidden) return;

  const revision = Number(summary.expectedFacilityRevision);
  const rows = [
    ['ОЖИДАЕМАЯ РЕВИЗИЯ ЛОКАЦИИ', Number.isSafeInteger(revision) && revision >= 0 ? String(revision) : '—'],
    ['УСТРОЙСТВА', facilityImpactIDs(summary.deviceIds).join(' · ') || '—'],
    ['НЕИСПРАВНОСТИ', facilityImpactIDs(summary.conditionIds).join(' · ') || '—'],
  ];
  if (typeof summary.recoveryProgramId === 'string' && summary.recoveryProgramId.trim()) {
    rows.push(['ПРОГРАММА ВОССТАНОВЛЕНИЯ', summary.recoveryProgramId.trim()]);
  }
  for (const [label, value] of rows) {
    const row = document.createElement('div');
    row.textContent = `${label}: ${value}`;
    commandExecutionFacilityImpact.appendChild(row);
  }
}

// ── Errors (R12) ────────────────────────────────────────────
// Go failure texts stay English for the logs. The Overseer shows each failure
// in Russian with a next step and keeps the original under a disclosure.
const facilityFailureMessages = Object.freeze({
  'missing-reference': 'Ссылка ведёт на удалённый элемент локации. Выберите существующий элемент и повторите.',
  'invalid-transition': 'Переход недоступен из текущего состояния устройства. Выберите другой переход или переведите устройство вручную.',
  'precondition-failed': 'Предусловие перехода не выполнено. Сначала переведите нужное устройство в требуемое состояние.',
  'stale-revision': 'Локация изменилась, пока выполнялся запрос. Проверьте новое состояние и повторите.',
  conflict: 'Правка противоречит текущим данным локации. Проверьте связанные элементы и повторите.',
  duplicate: 'Этот запрос уже выполнен. Повторять его не нужно.',
  'invalid-configuration': 'Настройка локации не прошла проверку. Исправьте последнюю правку или отмените её кнопкой «Отменить».',
  'persistence-failed': 'Не удалось записать файл сессии. Проверьте, что диск доступен и файл не защищён от записи, затем повторите.',
  'runtime-context-ended': 'Сессия закрылась до сохранения. Откройте её снова и повторите.',
  rejected: 'Смотритель отклонил запрос. Повторять его не нужно.',
});

// OVERSEER_ERROR_MESSAGES keys byText by the exact Go failure text. The Go
// catalog tests read this block as text, so keep one entry per line.
const OVERSEER_ERROR_MESSAGES = Object.freeze({
  'player-config': Object.freeze({
    general: 'Не удалось выполнить действие с файлом игроков. Выберите файл заново или создайте новый.',
    byText: Object.freeze({
      'new player config was canceled': 'Создание файла игроков прервано. Нажмите «Создать файл» ещё раз.',
      'new player config dialog is unavailable': 'Окно выбора файла недоступно. Перезапустите приложение и создайте файл игроков снова.',
      'could not choose a new player config destination': 'Не удалось выбрать место для файла игроков. Нажмите «Создать файл» и выберите другую папку.',
      'could not verify the new player config contract': 'Не удалось проверить новый файл игроков. Создайте его снова; если ошибка повторится, сохраните журнал.',
      'could not write the new player config': 'Не удалось записать файл игроков. Выберите папку, доступную для записи, и создайте файл снова.',
      'open player config was canceled': 'Выбор файла игроков прерван. Нажмите «Выбрать файл» ещё раз.',
      'open player config dialog is unavailable': 'Окно выбора файла недоступно. Перезапустите приложение и выберите файл игроков снова.',
      'could not choose a player config': 'Не удалось выбрать файл игроков. Нажмите «Выбрать файл» и повторите выбор.',
      'active session path is invalid': 'Путь к файлу сессии недействителен. Откройте сессию заново, затем выберите файл игроков.',
      'player config reference must be a relative file path': 'Сессия ссылается на файл игроков по неверному пути. Выберите файл игроков заново.',
      'could not read the player config': 'Не удалось прочитать файл игроков. Выберите его заново или создайте новый.',
      'the selected file is not a valid version-1 player config': 'Выбранный файл не является файлом игроков версии 1. Выберите другой файл или создайте новый.',
      'the selected file does not satisfy the player config contract': 'Выбранный файл игроков повреждён или заполнен неверно. Выберите другой файл или создайте новый.',
      'player config storage is unavailable': 'Хранилище файлов игроков недоступно. Перезапустите приложение и повторите.',
    }),
  }),
  'session': Object.freeze({
    general: 'Не удалось выполнить действие с файлом сессии. Повторите попытку. Если ошибка повторится, сохраните журнал.',
    byText: Object.freeze({
      'new session is unavailable': 'Создать сессию сейчас нельзя. Перезапустите приложение и повторите.',
      'new session dialog is unavailable': 'Окно выбора файла недоступно. Перезапустите приложение и создайте сессию снова.',
      'could not choose a new session destination': 'Не удалось выбрать место для новой сессии. Нажмите «Новая сессия» и выберите другую папку.',
      'new session was canceled': 'Создание сессии прервано. Нажмите «Новая сессия» ещё раз.',
      'could not verify the new session contract': 'Не удалось проверить новую сессию. Создайте её снова; если ошибка повторится, сохраните журнал.',
      'could not prepare the new session': 'Не удалось подготовить новую сессию. Создайте её снова; если ошибка повторится, сохраните журнал.',
      'session storage is unavailable': 'Хранилище сессий недоступно. Перезапустите приложение и повторите.',
      'could not write the new session': 'Не удалось записать файл сессии. Выберите папку, доступную для записи, и создайте сессию снова.',
      'open session is unavailable': 'Открыть сессию сейчас нельзя. Перезапустите приложение и повторите.',
      'open session dialog is unavailable': 'Окно выбора файла недоступно. Перезапустите приложение и откройте сессию снова.',
      'could not choose a session to open': 'Не удалось выбрать файл сессии. Нажмите «Открыть сессию» и повторите выбор.',
      'open session was canceled': 'Открытие сессии прервано. Нажмите «Открыть сессию» ещё раз.',
      'the bundled demo is read-only; copy it before editing': 'Встроенное демо открыто только для чтения. Создайте его копию и редактируйте её.',
      'could not read the selected session': 'Не удалось прочитать файл сессии. Проверьте, что файл доступен, и откройте его снова.',
      'the selected file is not a valid version-1 session': 'Выбранный файл не является сессией версии 1. Выберите другой файл.',
      'the selected file does not satisfy the session contract': 'Файл сессии повреждён или заполнен неверно. Выберите другой файл или восстановите его из копии.',
      'demo copy is unavailable': 'Скопировать демо сейчас нельзя. Перезапустите приложение и повторите.',
      'demo copy dialog is unavailable': 'Окно выбора файла недоступно. Перезапустите приложение и скопируйте демо снова.',
      'could not choose a demo destination': 'Не удалось выбрать место для копии демо. Повторите копирование и выберите другую папку.',
      'demo copy was canceled': 'Копирование демо прервано. Повторите копирование.',
      'could not read the bundled demo': 'Не удалось прочитать встроенное демо. Переустановите приложение.',
      'the bundled demo is not a valid version-1 session': 'Встроенное демо повреждено. Переустановите приложение.',
      'the bundled demo does not satisfy the session contract': 'Встроенное демо заполнено неверно. Переустановите приложение.',
      'the bundled demo player config reference is invalid': 'Встроенное демо ссылается на файл игроков неверно. Переустановите приложение.',
      'could not read the bundled demo player config': 'Не удалось прочитать файл игроков встроенного демо. Переустановите приложение.',
      'the bundled demo player config is not valid version 1': 'Файл игроков встроенного демо повреждён. Переустановите приложение.',
      'could not create a writable demo player config copy': 'Не удалось записать копию файла игроков демо. Выберите папку, доступную для записи, и повторите.',
      'could not create a writable demo copy': 'Не удалось записать копию демо. Выберите папку, доступную для записи, и повторите.',
      'player config association was canceled': 'Сохранение выбора файла игроков прервано. Выберите файл игроков ещё раз.',
      'there is no active session': 'Сессия не открыта. Откройте или создайте сессию.',
      'player config path must be absolute': 'Путь к файлу игроков недействителен. Выберите файл игроков заново.',
      'could not create a relative player config reference': 'Не удалось сохранить путь к файлу игроков относительно сессии. Выберите файл игроков рядом с файлом сессии.',
      'could not verify the player config association': 'Не удалось проверить выбор файла игроков. Выберите файл ещё раз; если ошибка повторится, сохраните журнал.',
      'could not prepare the player config association': 'Не удалось подготовить сохранение выбора файла игроков. Выберите файл ещё раз; если ошибка повторится, сохраните журнал.',
      'could not save the player config association': 'Не удалось записать выбор файла игроков в файл сессии. Проверьте, что файл сессии доступен для записи, и повторите.',
      'active session changed while saving the player config association': 'Сессия изменилась во время сохранения выбора файла игроков. Выберите файл игроков ещё раз.',
      'could not save the session': 'Не удалось сохранить файл сессии. Проверьте, что файл доступен для записи, и повторите.',
      'session service is shut down': 'Приложение завершает работу. Перезапустите его и повторите.',
    }),
  }),
  'facility': Object.freeze({
    general: 'Не удалось изменить локацию. Повторите попытку. Если ошибка повторится, сохраните журнал.',
  }),
  'public-access': Object.freeze({
    general: 'Не удалось изменить публичный доступ. Повторите попытку. Если ошибка повторится, сохраните журнал.',
  }),
  'credentials': Object.freeze({
    general: 'Не удалось сохранить данные доступа. Проверьте введённые значения и повторите попытку. Если ошибка повторится, сохраните журнал.',
  }),
  'update': Object.freeze({
    general: 'Не удалось выполнить обновление приложения. Повторите попытку позже. Если ошибка повторится, сохраните журнал.',
  }),
  'players': Object.freeze({
    general: 'Не удалось изменить список игроков. Повторите попытку. Если ошибка повторится, сохраните журнал.',
  }),
  'terminal-groups': Object.freeze({
    general: 'Не удалось изменить группы терминалов. Повторите попытку. Если ошибка повторится, сохраните журнал.',
  }),
  'broadcast': Object.freeze({
    general: 'Не удалось изменить трансляцию. Повторите попытку. Если ошибка повторится, сохраните журнал.',
  }),
  'startup': Object.freeze({
    general: 'Сервер для игроков не запустился. Закройте приложение и запустите его снова. Если ошибка повторится, сохраните журнал.',
  }),
  'commands': Object.freeze({
    general: 'Не удалось обработать запрос игрока. Повторите решение. Если ошибка повторится, сохраните журнал.',
  }),
  'logs': Object.freeze({
    general: 'Не удалось открыть журналы. Откройте папку журналов вручную.',
  }),
  'links': Object.freeze({
    general: 'Не удалось открыть ссылку. Скопируйте адрес и откройте его в браузере.',
  }),
});

// facilityIssueEntityPhrase names the entity of a facility issue the way the
// author knows it, or returns '' when session has no such entity.
function facilityIssueEntityPhrase(issue, session) {
  const facility = session?.facility;
  const named = (values, label, suffix = '') => {
    const entity = values?.find(value => value.id === issue.entityId);
    return entity ? `${label} «${entity.name}»${suffix}` : '';
  };
  const owner = issue?.referenceKind === 'owner' ? facilityDeviceByID(issue.referenceId, session) : null;
  const ownerSuffix = owner ? ` устройства «${owner.name}»` : '';
  switch (issue?.entityKind) {
    case 'device': return named(facility?.devices, 'устройство');
    case 'device-state': return named(owner?.states, 'состояние', ownerSuffix);
    case 'device-transition': return named(owner?.transitions, 'переход', ownerSuffix);
    case 'condition': return named(facility?.conditions, 'неисправность');
    case 'recovery-program': return named(facility?.recoveryPrograms, 'программа восстановления');
    default: return '';
  }
}

// overseerErrorArea returns the catalog of area; an unknown area is a
// programming error.
function overseerErrorArea(area) {
  if (!Object.hasOwn(OVERSEER_ERROR_MESSAGES, area)) throw new Error(`unknown Overseer error area: ${area}`);
  return OVERSEER_ERROR_MESSAGES[area];
}

// overseerErrorMessage picks the Russian message for a desktop result, a raw
// error text, or null: a facility failure code first, then the exact error
// text in any byText table, then the area's general message. key is what
// matched, or 'general'; original is the raw text for the disclosure.
function overseerErrorMessage(result, area) {
  const { general } = overseerErrorArea(area);
  const failure = typeof result?.failure === 'string' ? result.failure : '';
  if (failure) {
    const issue = result.issues?.[0];
    const known = Object.hasOwn(facilityFailureMessages, failure);
    const message = known ? facilityFailureMessages[failure] : general;
    // The result's session names an entity that the working copy no longer
    // holds.
    const entity = facilityIssueEntityPhrase(issue, state.session) || facilityIssueEntityPhrase(issue, result.session);
    return {
      message: entity ? `${message} Затронуто: ${entity}.` : message,
      key: known ? failure : 'general',
      original: [
        `failure: ${failure}`,
        [issue?.entityKind, issue?.entityId].filter(Boolean).join(' '),
        [issue?.referenceKind, issue?.referenceId].filter(Boolean).join(' '),
      ].filter(Boolean).join(' · '),
    };
  }
  const text = typeof result === 'string' ? result : (typeof result?.error === 'string' ? result.error : '');
  for (const { byText } of Object.values(OVERSEER_ERROR_MESSAGES)) {
    if (byText && Object.hasOwn(byText, text)) return { message: byText[text], key: text, original: text };
  }
  return { message: general, key: 'general', original: text };
}

// setOverseerSinkText writes plain text into an error sink and drops the
// attributes a previous renderOverseerError left on it.
function setOverseerSinkText(sink, text) {
  sink.textContent = text;
  delete sink.dataset.errorArea;
  delete sink.dataset.errorKey;
}

// renderOverseerError replaces sink's content with the message for result,
// after an optional prefix, and puts the original text under a collapsed
// «подробности для разработчика». A null result empties and hides sink.
function renderOverseerError(sink, result, area, { prefix = '' } = {}) {
  if (result === null) {
    overseerErrorArea(area);
    sink.replaceChildren();
    sink.hidden = true;
    delete sink.dataset.errorArea;
    delete sink.dataset.errorKey;
    return;
  }
  const { message, key, original } = overseerErrorMessage(result, area);
  const content = [];
  if (prefix) {
    const label = document.createElement('span');
    label.className = 'overseer-error-prefix';
    label.textContent = prefix;
    content.push(label);
  }
  const text = document.createElement('p');
  text.className = 'overseer-error-message';
  text.textContent = message;
  content.push(text);
  if (original) {
    const details = document.createElement('details');
    details.className = 'overseer-error-details';
    const summary = document.createElement('summary');
    summary.textContent = 'подробности для разработчика';
    const raw = document.createElement('p');
    raw.className = 'overseer-error-original';
    raw.textContent = original;
    details.append(summary, raw);
    content.push(details);
  }
  sink.replaceChildren(...content);
  sink.hidden = false;
  sink.dataset.errorArea = area;
  sink.dataset.errorKey = key;
}

function rememberResolvedCommandExecution(requestID) {
  resolvedCommandExecutionRequestIDs.add(requestID);
  if (resolvedCommandExecutionRequestIDs.size <= 128) return;
  const oldest = resolvedCommandExecutionRequestIDs.values().next().value;
  resolvedCommandExecutionRequestIDs.delete(oldest);
}

function hideCommandExecutionDialog() {
  // Any authoritative close invalidates the promise currently resolving this
  // dialog. Its eventual callback must not overwrite a newer lifecycle state
  // or dismiss a different request shown in the meantime.
  commandExecutionDialogEpoch += 1;
  commandExecutionDecisionRequestID = null;
  commandExecutionDialogRequestID = null;
  commandExecutionDialogMode = null;
  commandExecutionDialog.hidden = true;
  if (typeof commandExecutionDialog.close === 'function' && commandExecutionDialog.open) {
    commandExecutionDialog.close();
  } else {
    commandExecutionDialog.removeAttribute('open');
  }
  btnApproveCommandExecution.disabled = false;
  btnRejectCommandExecution.disabled = false;
  renderCommandExecutionFacilityImpact(null);
  commandExecutionDialogStatus.textContent = '';
  commandExecutionDialogError.textContent = '';
  commandExecutionDialogError.hidden = true;
}

function showCommandExecutionDialog(pending) {
  commandExecutionDialogEpoch += 1;
  commandExecutionDecisionRequestID = null;
  commandExecutionDialogRequestID = pending.requestId;
  commandExecutionDialogMode = pending.mode || null;
  commandExecutionDialogDescription.textContent = pending.confirmationText;
  renderCommandExecutionFacilityImpact(pending.facilityAction);
  const modeLabels = {
    ordinary: 'ОБЫЧНАЯ',
    'state-change': 'ИЗМЕНЕНИЕ СОСТОЯНИЯ',
    'completed-state-change': 'ЗАВЕРШЁННОЕ ИЗМЕНЕНИЕ СОСТОЯНИЯ',
  };
  const mode = modeLabels[pending.mode] || pending.mode || 'НЕИЗВЕСТЕН';
  const commandName = pending.commandName || pending.commandId || '—';
  commandExecutionDialogStatus.textContent = `ЗАПРОС: ${pending.requestId} · РЕЖИМ: ${mode} · КОМАНДА: ${commandName}`;
  commandExecutionDialogError.textContent = '';
  commandExecutionDialogError.hidden = true;
  btnApproveCommandExecution.disabled = false;
  btnRejectCommandExecution.disabled = false;
  commandExecutionDialog.hidden = false;
  if (typeof commandExecutionDialog.showModal === 'function' && !commandExecutionDialog.open) {
    commandExecutionDialog.showModal();
  } else {
    commandExecutionDialog.setAttribute('open', '');
  }
  btnApproveCommandExecution.focus();
}

function syncCommandExecutionDialog(coordination) {
  const pending = coordination?.pendingCommandExecution;
  const requestID = typeof pending?.requestId === 'string' ? pending.requestId : '';
  if (!requestID) {
    if (commandExecutionDialogRequestID) hideCommandExecutionDialog();
    return;
  }
  if (requestID === commandExecutionDecisionRequestID || resolvedCommandExecutionRequestIDs.has(requestID)) {
    return;
  }
  if (requestID === commandExecutionDialogRequestID) return;
  if (commandExecutionDialogRequestID) hideCommandExecutionDialog();
  showCommandExecutionDialog(pending);
}

function rememberResolvedTerminalNavigation(requestID) {
  resolvedTerminalNavigationRequestIDs.add(requestID);
  if (resolvedTerminalNavigationRequestIDs.size > 128) {
    resolvedTerminalNavigationRequestIDs.delete(resolvedTerminalNavigationRequestIDs.values().next().value);
  }
}

function hideTerminalNavigationDialog() {
  terminalNavigationDialogEpoch += 1;
  terminalNavigationDecisionRequestID = null;
  terminalNavigationDialogRequestID = null;
  terminalNavigationDialog.hidden = true;
  if (terminalNavigationDialog.open) terminalNavigationDialog.close();
  else terminalNavigationDialog.removeAttribute('open');
  btnApproveTerminalNavigation.disabled = false;
  btnRejectTerminalNavigation.disabled = false;
  terminalNavigationStatus.textContent = '';
  terminalNavigationError.hidden = true;
}

function showTerminalNavigationDialog(pending) {
  terminalNavigationDialogEpoch += 1;
  terminalNavigationDecisionRequestID = null;
  terminalNavigationDialogRequestID = pending.requestId;
  const direction = pending.direction === 'return' ? 'ВОЗВРАТ' : 'ПЕРЕХОД';
  terminalNavigationSummary.innerHTML = `
    <div>ЗАПРОС: ${escHtml(pending.requestId)}</div>
    <div class="terminal-navigation-direction">${direction}</div>
    <div>ИЗ: ${escHtml(pending.sourceTerminalName || pending.sourceTerminalId || '—')}</div>
    <div>КОМАНДА: ${escHtml(pending.commandName || pending.commandId || '—')}</div>
    <div>В: ${escHtml(pending.targetTerminalName || pending.targetTerminalId || '—')}</div>`;
  terminalNavigationError.hidden = true;
  terminalNavigationStatus.textContent = 'ИСХОДНЫЙ ТЕРМИНАЛ ОСТАЁТСЯ АКТИВНЫМ ДО РЕШЕНИЯ';
  terminalNavigationDialog.hidden = false;
  if (!terminalNavigationDialog.open) terminalNavigationDialog.showModal();
  btnApproveTerminalNavigation.focus();
}

function syncTerminalNavigationDialog(coordination) {
  const pending = coordination?.pendingTerminalNavigation;
  const requestID = typeof pending?.requestId === 'string' ? pending.requestId : '';
  if (!requestID) {
    if (terminalNavigationDialogRequestID) hideTerminalNavigationDialog();
    return;
  }
  if (requestID === terminalNavigationDecisionRequestID || resolvedTerminalNavigationRequestIDs.has(requestID)) return;
  if (requestID === terminalNavigationDialogRequestID) return;
  if (terminalNavigationDialogRequestID) hideTerminalNavigationDialog();
  showTerminalNavigationDialog(pending);
}

function applyCoordinationState(coordination) {
  if (coordination && state.coordination &&
      coordinationRevision(coordination) <= coordinationRevision(state.coordination)) {
    return false;
  }
  state.coordination = coordination || null;
  state.liveTerminalId = coordination?.broadcast?.activeTerminalId || null;
  syncCommandExecutionDialog(coordination);
  syncTerminalNavigationDialog(coordination);
  syncTerminalNavigationNotice(coordination);
  return true;
}

function syncTerminalNavigationNotice(coordination) {
  const notice = coordination?.terminalNavigationNotice;
  if (!notice) {
    if (coordinationError.dataset.kind === 'terminal-navigation') {
      coordinationError.dataset.kind = '';
      setCoordinationStatus('');
    }
    return;
  }
  const labels = {
    'target-missing': 'ЦЕЛЕВОЙ ТЕРМИНАЛ ПЕРЕХОДА БОЛЬШЕ НЕ СУЩЕСТВУЕТ',
    'self-target': 'КОМАНДА ПЕРЕХОДА НЕ МОЖЕТ ССЫЛАТЬСЯ НА ТЕКУЩИЙ ТЕРМИНАЛ',
    'command-stale': 'КОМАНДА ПЕРЕХОДА БЫЛА ИЗМЕНЕНА ИЛИ УДАЛЕНА',
    'target-changed': 'ЦЕЛЬ КОМАНДЫ ПЕРЕХОДА ИЗМЕНИЛАСЬ',
  };
  const detail = [notice.sourceTerminalId, notice.commandId, notice.targetTerminalId].filter(Boolean).join(' · ');
  coordinationError.dataset.kind = 'terminal-navigation';
  setCoordinationStatus(`${labels[notice.reason] || 'ПЕРЕХОД БОЛЬШЕ НЕ ДЕЙСТВИТЕЛЕН'}${detail ? ` · ${detail}` : ''}`, true);
}

async function resolveTerminalNavigation(decision) {
  const requestID = terminalNavigationDialogRequestID;
  if (!requestID || terminalNavigationDecisionRequestID) return null;
  terminalNavigationDecisionRequestID = requestID;
  const epoch = terminalNavigationDialogEpoch;
  btnApproveTerminalNavigation.disabled = true;
  btnRejectTerminalNavigation.disabled = true;
  terminalNavigationStatus.textContent = decision === 'approve' ? 'ВЫПОЛНЕНИЕ ПЕРЕХОДА...' : 'ОТКЛОНЕНИЕ ПЕРЕХОДА...';
  let result;
  try {
    result = await desktopAPI.resolveTerminalNavigation({ requestId: requestID, decision });
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
  if (epoch !== terminalNavigationDialogEpoch || terminalNavigationDecisionRequestID !== requestID) return result;
  terminalNavigationDecisionRequestID = null;
  rememberResolvedTerminalNavigation(requestID);
  if (result?.state) applyCoordinationState(result.state);
  if (terminalNavigationDialogRequestID === requestID) hideTerminalNavigationDialog();
  if (result?.ok) {
    setCoordinationStatus(decision === 'approve' ? 'ПЕРЕХОД ВЫПОЛНЕН' : 'ПЕРЕХОД ОТКЛОНЁН');
  } else if (result?.state?.terminalNavigationNotice) {
    syncTerminalNavigationNotice(result.state);
  } else {
    setCoordinationError(result, 'commands');
  }
  renderCoordination();
  return result;
}

btnApproveTerminalNavigation.addEventListener('click', () => { void resolveTerminalNavigation('approve'); });
btnRejectTerminalNavigation.addEventListener('click', () => { void resolveTerminalNavigation('reject'); });
terminalNavigationDialog.addEventListener('keydown', (event) => {
  if (event.key === 'ArrowLeft') {
    event.preventDefault();
    btnApproveTerminalNavigation.focus();
  } else if (event.key === 'ArrowRight') {
    event.preventDefault();
    btnRejectTerminalNavigation.focus();
  }
});
terminalNavigationDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  void resolveTerminalNavigation('reject');
});

async function resolveCommandExecution(decision) {
  const requestID = commandExecutionDialogRequestID;
  if (!requestID || commandExecutionDecisionRequestID) return null;

  const commandMode = commandExecutionDialogMode;
  commandExecutionDecisionRequestID = requestID;
  const epoch = commandExecutionDialogEpoch;
  const startingRevision = coordinationRevision(state.coordination);
  btnApproveCommandExecution.disabled = true;
  btnRejectCommandExecution.disabled = true;
  commandExecutionDialogStatus.textContent = decision === 'approve'
    ? (commandMode === 'state-change' ? 'СОХРАНЕНИЕ И ВЫПОЛНЕНИЕ...' : 'ВЫПОЛНЕНИЕ КОМАНДЫ...')
    : 'ОТКЛОНЕНИЕ ЗАПРОСА...';
  commandExecutionDialogError.textContent = '';
  commandExecutionDialogError.hidden = true;

  let result;
  try {
    result = await desktopAPI.resolveCommandExecution({ requestId: requestID, decision });
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }

  if (epoch !== commandExecutionDialogEpoch || commandExecutionDecisionRequestID !== requestID) {
    return result;
  }
  commandExecutionDecisionRequestID = null;
  const resultRevision = coordinationRevision(result?.state);
  if (resultRevision > 0 && resultRevision < coordinationRevision(state.coordination)) {
    return result;
  }
  if (!result?.state && coordinationRevision(state.coordination) > startingRevision &&
      state.coordination?.pendingCommandExecution?.requestId !== requestID) {
    return result;
  }

  rememberResolvedCommandExecution(requestID);
  if (result?.state) applyCoordinationState(result.state);
  if (commandExecutionDialogRequestID === requestID) hideCommandExecutionDialog();

  if (!result?.ok) {
    if (result?.facilityResult?.failure) setCoordinationError(result.facilityResult, 'facility');
    else setCoordinationError(result, 'commands');
  } else if (decision === 'approve') {
    setCoordinationStatus(commandMode === 'state-change'
      ? 'КОМАНДА ВЫПОЛНЕНА И СОХРАНЕНА'
      : 'КОМАНДА ВЫПОЛНЕНА');
  } else {
    setCoordinationStatus('ЗАПРОС ОТКЛОНЁН');
  }
  renderCoordination();
  return result;
}

btnApproveCommandExecution.addEventListener('click', () => {
  void resolveCommandExecution('approve');
});
btnRejectCommandExecution.addEventListener('click', () => {
  void resolveCommandExecution('reject');
});
commandExecutionDialog.addEventListener('keydown', (event) => {
  if (event.key === 'ArrowLeft') {
    event.preventDefault();
    btnApproveCommandExecution.focus();
  } else if (event.key === 'ArrowRight') {
    event.preventDefault();
    btnRejectCommandExecution.focus();
  }
});
commandExecutionDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  void resolveCommandExecution('reject');
});

function setCoordinationStatus(message, isError = false) {
  coordinationStatus.textContent = isError ? '' : (message || '');
  coordinationStatus.classList.remove('err');
  setOverseerSinkText(coordinationError, isError ? (message || '') : '');
  coordinationError.hidden = !isError || !message;
  setLogicalSessionDialogFeedback(message, isError);
}

// setCoordinationError shows a desktop failure in the coordination panel and
// in the logical session dialog, which mirrors it.
function setCoordinationError(result, area, options) {
  coordinationStatus.textContent = '';
  coordinationStatus.classList.remove('err');
  logicalSessionDialogStatus.textContent = '';
  renderOverseerError(coordinationError, result, area, options);
  renderOverseerError(logicalSessionDialogError, result, area, options);
}

function setLogicalSessionDialogFeedback(message = '', isError = false) {
  logicalSessionDialogStatus.textContent = isError ? '' : (message || '');
  logicalSessionDialogError.textContent = isError ? (message || '') : '';
  logicalSessionDialogError.hidden = !isError || !message;
}

function fillSelect(select, values, valueOf, labelOf, emptyLabel) {
  select.replaceChildren();
  for (const value of values) {
    const option = document.createElement('option');
    option.value = valueOf(value);
    option.textContent = labelOf(value);
    select.appendChild(option);
  }
  if (!values.length) {
    const option = document.createElement('option');
    option.value = '';
    option.textContent = emptyLabel;
    select.appendChild(option);
  }
}

function sessionLabel(session) {
  const character = session.character ? ` · ${session.character.name}` : '';
  return `${session.fallbackName}${character}`;
}

async function runCoordinationCommand(command, successMessage, pendingMessage, area) {
  if (coordinationCommandPending) return null;
  coordinationCommandPending = true;
  setCoordinationStatus(pendingMessage || 'ВЫПОЛНЕНИЕ ОПЕРАЦИИ...');
  renderCoordination();
  if (state.session) renderTreeHeader();
  renderHackStatus();
  let result;
  try {
    result = await command();
  } catch (error) {
    result = { ok: false, error: error instanceof Error ? error.message : String(error) };
  }
  coordinationCommandPending = false;
  renderHackStatus();
  if (state.session) renderTreeHeader();
  if (!result?.ok) {
    if (result?.state) applyCoordinationState(result.state);
    setCoordinationError(result, area);
    renderCoordination();
    return result;
  }
  if (result.state) applyCoordinationState(result.state);
  setCoordinationStatus(successMessage || 'ОПЕРАЦИЯ ВЫПОЛНЕНА');
  renderCoordination();
  return result;
}

function showTerminalSwitchDecision(result) {
  pendingTerminalSwitch = result?.switchId || null;
  if (!pendingTerminalSwitch) return;
  terminalSwitchStatus.textContent = 'ИСХОДНЫЙ ТЕРМИНАЛ ОСТАЁТСЯ АКТИВНЫМ ДО ВЫБОРА';
  terminalSwitchError.textContent = '';
  terminalSwitchError.hidden = true;
  terminalSwitchDialog.hidden = false;
  if (typeof terminalSwitchDialog.showModal === 'function' && !terminalSwitchDialog.open) {
    terminalSwitchDialog.showModal();
  } else {
    terminalSwitchDialog.setAttribute('open', '');
  }
  terminalSwitchButtons[0]?.focus();
}

function hideTerminalSwitchDecision() {
  pendingTerminalSwitch = null;
  terminalSwitchDialog.hidden = true;
  if (typeof terminalSwitchDialog.close === 'function' && terminalSwitchDialog.open) {
    terminalSwitchDialog.close();
  } else {
    terminalSwitchDialog.removeAttribute('open');
  }
}

function showEndBroadcastConfirmation() {
  endBroadcastDialog.hidden = false;
  if (typeof endBroadcastDialog.showModal === 'function' && !endBroadcastDialog.open) {
    endBroadcastDialog.showModal();
  } else {
    endBroadcastDialog.setAttribute('open', '');
  }
  btnCancelEndBroadcast.focus();
}

function hideEndBroadcastConfirmation({ restoreFocus = true } = {}) {
  endBroadcastDialog.hidden = true;
  if (typeof endBroadcastDialog.close === 'function' && endBroadcastDialog.open) {
    endBroadcastDialog.close();
  } else {
    endBroadcastDialog.removeAttribute('open');
  }
  btnCancelEndBroadcast.disabled = false;
  btnConfirmEndBroadcast.disabled = false;
  if (restoreFocus && !btnEndBroadcast.hidden) btnEndBroadcast.focus();
}

function showTakeOffAirConfirmation() {
  if (takeOffAirPending || !state.coordination?.broadcast || !state.liveTerminalId) return;
  takeOffAirError.textContent = '';
  takeOffAirError.hidden = true;
  btnCancelTakeOffAir.disabled = false;
  btnConfirmTakeOffAir.disabled = false;
  takeOffAirDialog.hidden = false;
  if (typeof takeOffAirDialog.showModal === 'function' && !takeOffAirDialog.open) {
    takeOffAirDialog.showModal();
  } else {
    takeOffAirDialog.setAttribute('open', '');
  }
  btnCancelTakeOffAir.focus();
}

function hideTakeOffAirConfirmation({ restoreFocus = true } = {}) {
  if (typeof takeOffAirDialog.close === 'function' && takeOffAirDialog.open) {
    takeOffAirDialog.close();
  } else {
    takeOffAirDialog.removeAttribute('open');
  }
  takeOffAirDialog.hidden = true;
  takeOffAirPending = false;
  btnCancelTakeOffAir.disabled = false;
  btnConfirmTakeOffAir.disabled = false;
  if (restoreFocus && !btnStopBroadcast.hidden) btnStopBroadcast.focus();
}

async function runTerminalSwitchRequest(command, completedMessage, pendingMessage) {
  const result = await runCoordinationCommand(command, completedMessage, pendingMessage, 'broadcast');
  if (result?.ok && result.status === 'decision-required' && result.switchId) {
    showTerminalSwitchDecision(result);
  }
  return result;
}

// ── Render: terminal list ────────────────────────────────────
function inboundTerminalTransitions(targetTerminalID) {
  const inbound = [];
  const visit = (source, node) => {
    if (node?.terminalTransition?.targetTerminalId === targetTerminalID) {
      inbound.push(`${source.name}: ${node.name}`);
    }
    for (const child of node?.children ?? []) visit(source, child);
  };
  for (const source of state.session?.terminals ?? []) visit(source, source.root);
  return inbound;
}

function renderTermList() {
  termList.innerHTML = '';
  if (!state.session.terminals.length) {
    termList.innerHTML = '<div class="tree-empty-hint">Нет терминалов</div>';
    return;
  }

  ensureSessionTerminalGroups();
  const currentGroupIDs = new Set(state.session.terminalGroups.map(group => group.id));
  for (const groupID of state.collapsedTerminalGroupIds) {
    if (!currentGroupIDs.has(groupID)) state.collapsedTerminalGroupIds.delete(groupID);
  }
  const terminalsByID = new Map(state.session.terminals.map(term => [term.id, term]));
  state.session.terminalGroups.forEach((group, groupIndex) => {
    const collapsed = state.collapsedTerminalGroupIds.has(group.id);
    const groupEl = document.createElement('section');
    groupEl.className = 'terminal-group' + (group.terminalIds.length === 1 ? ' is-singleton' : '');
    groupEl.dataset.groupId = group.id;
    groupEl.dataset.singleton = String(group.terminalIds.length === 1);
    groupEl.dataset.collapsed = String(collapsed);
    groupEl.setAttribute('role', 'listitem');

    const header = document.createElement('header');
    header.className = 'terminal-group-header';

    const members = document.createElement('div');
    members.className = 'terminal-group-members';
    members.id = `terminal-group-members-${groupIndex}`;
    members.hidden = collapsed;
    members.setAttribute('role', 'list');
    members.setAttribute('aria-label', `Терминалы группы ${group.name}`);

    const toggle = document.createElement('button');
    toggle.type = 'button';
    toggle.className = 'terminal-group-toggle';
    toggle.dataset.action = 'toggle-terminal-group';
    toggle.setAttribute('aria-controls', members.id);
    toggle.setAttribute('aria-expanded', String(!collapsed));
    toggle.setAttribute('aria-label', `${collapsed ? 'Развернуть' : 'Свернуть'} группу ${group.name}`);

    const caret = document.createElement('span');
    caret.className = 'terminal-group-caret';
    caret.setAttribute('aria-hidden', 'true');
    caret.textContent = collapsed ? '▸' : '▾';
    const heading = document.createElement('span');
    heading.className = 'terminal-group-name';
    heading.textContent = group.name;
    toggle.append(caret, heading);
    toggle.addEventListener('click', () => {
      const nextCollapsed = !state.collapsedTerminalGroupIds.has(group.id);
      if (nextCollapsed) state.collapsedTerminalGroupIds.add(group.id);
      else state.collapsedTerminalGroupIds.delete(group.id);
      members.hidden = nextCollapsed;
      groupEl.dataset.collapsed = String(nextCollapsed);
      caret.textContent = nextCollapsed ? '▸' : '▾';
      toggle.setAttribute('aria-expanded', String(!nextCollapsed));
      toggle.setAttribute('aria-label', `${nextCollapsed ? 'Развернуть' : 'Свернуть'} группу ${group.name}`);
    });

    const memberCount = document.createElement('span');
    memberCount.className = 'terminal-group-member-count';
    memberCount.textContent = terminalCountLabel(group.terminalIds.length);

    const groupActions = terminalActionMenu({
      scope: 'terminal-group',
      ownerID: group.id,
      ownerName: group.name,
      items: [
        { label: 'ПЕРЕИМЕНОВАТЬ ГРУППУ', action: 'rename-terminal-group', handler: () => showTerminalGroupRename(group.id) },
        { label: 'ПЕРЕМЕСТИТЬ ГРУППУ ВВЕРХ', action: 'move-terminal-group-up', handler: () => prepareTerminalGroupOrder(group.id, -1), disabled: groupIndex === 0 },
        { label: 'ПЕРЕМЕСТИТЬ ГРУППУ ВНИЗ', action: 'move-terminal-group-down', handler: () => prepareTerminalGroupOrder(group.id, 1), disabled: groupIndex === state.session.terminalGroups.length - 1 },
        { label: 'РАСФОРМИРОВАТЬ ГРУППУ', action: 'dissolve-terminal-group', handler: () => prepareTerminalGroupDissolution(group.id), destructive: true },
      ],
    });
    header.append(toggle, memberCount, groupActions);
    groupEl.append(header, members);

    group.terminalIds.forEach((terminalID, memberIndex) => {
      const term = terminalsByID.get(terminalID);
      if (term) members.appendChild(buildTerminalRow(term, group, memberIndex));
    });
    termList.appendChild(groupEl);
  });
}

// russianPlural picks the noun form for count: one (1, 21), few (2 to 4,
// 22 to 24) or many (5 to 20, 25 to 30).
function russianPlural(count, one, few, many) {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few;
  return many;
}

function terminalCountLabel(count) {
  return `${count} ${russianPlural(count, 'ТЕРМИНАЛ', 'ТЕРМИНАЛА', 'ТЕРМИНАЛОВ')}`;
}

function terminalActionMenu({ scope, ownerID, ownerName, items }) {
  const menu = document.createElement('details');
  menu.className = 'terminal-action-menu';
  menu.addEventListener('click', event => event.stopPropagation());
  menu.addEventListener('toggle', () => {
    if (!menu.open) return;
    for (const other of termList.querySelectorAll('.terminal-action-menu[open]')) {
      if (other !== menu) other.open = false;
    }
  });

  const trigger = document.createElement('summary');
  trigger.className = 'terminal-action-menu-trigger';
  trigger.dataset.actionMenuTrigger = scope;
  trigger.dataset.actionMenuOwnerId = ownerID;
  trigger.setAttribute('aria-label', `Действия: ${ownerName}`);
  trigger.textContent = '•••';

  const panel = document.createElement('div');
  panel.className = 'terminal-action-menu-panel';
  panel.setAttribute('role', 'menu');
  panel.setAttribute('aria-label', `Действия: ${ownerName}`);
  for (const item of items) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'btn btn-mini terminal-action-menu-item'
      + (item.destructive ? ' btn-danger terminal-action-menu-destructive' : '');
    button.textContent = item.label;
    button.dataset.action = item.action;
    button.disabled = Boolean(item.disabled);
    button.setAttribute('role', 'menuitem');
    button.addEventListener('click', event => {
      event.stopPropagation();
      terminalGroupDialogOpener = { scope, ownerID };
      menu.open = false;
      item.handler();
      if (!terminalGroupDraftDialog.open && !terminalGroupImpactDialog.open) {
        terminalGroupDialogOpener = null;
        if (document.activeElement === button) trigger.focus();
      }
    });
    panel.appendChild(button);
  }
  menu.append(trigger, panel);
  return menu;
}

function closeTerminalActionMenus({ restoreFocus = false } = {}) {
  const openMenu = termList.querySelector('.terminal-action-menu[open]');
  if (!openMenu) return false;
  const trigger = openMenu.querySelector('.terminal-action-menu-trigger');
  openMenu.open = false;
  if (restoreFocus) trigger?.focus();
  return true;
}

document.addEventListener('click', event => {
  if (!event.target.closest('.terminal-action-menu')) closeTerminalActionMenus();
});
document.addEventListener('keydown', event => {
  if (event.key === 'Escape' && closeTerminalActionMenus({ restoreFocus: true })) event.preventDefault();
});

function ensureSessionTerminalGroups(session = state.session) {
  if (!session) return;
  const terminalIDs = new Set(session.terminals.map(term => term.id));
  const represented = new Set();
  const groups = [];
  for (const group of session.terminalGroups ?? []) {
    const members = (group.terminalIds ?? []).filter(id => terminalIDs.has(id) && !represented.has(id));
    members.forEach(id => represented.add(id));
    if (members.length) groups.push({ id: group.id, name: group.name, terminalIds: members });
  }
  const usedNames = new Set(groups.map(group => group.name.trim().toLocaleLowerCase()));
  for (const term of session.terminals) {
    if (represented.has(term.id)) continue;
    let name = term.name.trim() || 'Terminal';
    let suffix = 2;
    while (usedNames.has(name.toLocaleLowerCase())) name = `${term.name.trim() || 'Terminal'} (${suffix++})`;
    usedNames.add(name.toLocaleLowerCase());
    groups.push({ id: uid('g'), name, terminalIds: [term.id] });
  }
  session.terminalGroups = groups;
}

function setTerminalGroupError(message = '') {
  terminalGroupError.textContent = message;
  terminalGroupError.hidden = !message;
}

function setTerminalGroupImpactError(message = '', canAmend = false) {
  terminalGroupImpactError.textContent = message;
  terminalGroupImpactError.hidden = !message;
  amendTerminalGroupChangeButton.hidden = !canAmend;
}

function showTerminalGroupDialog(dialog, focusTarget) {
  if (!terminalGroupDialogOpener) {
    terminalGroupDialogOpener = { element: document.activeElement };
  }
  dialog.hidden = false;
  if (typeof dialog.showModal === 'function' && !dialog.open) dialog.showModal();
  else dialog.setAttribute('open', '');
  focusTarget?.focus();
}

function hideTerminalGroupDialog(dialog) {
  if (dialog.open && typeof dialog.close === 'function') dialog.close();
  else dialog.removeAttribute('open');
  dialog.hidden = true;
}

function restoreTerminalGroupDialogFocus() {
  const opener = terminalGroupDialogOpener;
  terminalGroupDialogOpener = null;
  if (!opener) return;
  if (opener.element?.isConnected) {
    opener.element.focus();
    return;
  }
  const trigger = [...termList.querySelectorAll(`[data-action-menu-trigger="${opener.scope}"]`)]
    .find(candidate => candidate.dataset.actionMenuOwnerId === opener.ownerID);
  trigger?.focus();
}

function closeTerminalGroupDraft({ restoreFocus = true } = {}) {
  terminalGroupDraft = null;
  terminalGroupDraftForm.reset();
  hideTerminalGroupDialog(terminalGroupDraftDialog);
  if (restoreFocus) restoreTerminalGroupDialogFocus();
}

function closeTerminalGroupImpact({ restoreFocus = true } = {}) {
  pendingTerminalGroupImpact = null;
  hideTerminalGroupDialog(terminalGroupImpactDialog);
  if (restoreFocus) restoreTerminalGroupDialogFocus();
}

function populateTerminalChoices(selectedIDs = []) {
  terminalGroupTerminalChoices.innerHTML = '';
  const selected = new Set(selectedIDs);
  for (const terminal of state.session.terminals) {
    const fragment = terminalGroupTerminalChoiceTemplate.content.cloneNode(true);
    const input = fragment.querySelector('input[name="terminalIds"]');
    input.value = terminal.id;
    input.checked = selected.has(terminal.id);
    fragment.querySelector('.terminal-group-terminal-choice-name').textContent = terminal.name;
    terminalGroupTerminalChoices.appendChild(fragment);
  }
}

function configureTerminalGroupDraft(mode) {
  const terminalChoicesFieldset = document.getElementById('terminalGroupTerminalChoices');
  const destinationLabel = document.querySelector('label[for="terminalGroupDestinationSelect"]');
  const reviewButton = terminalGroupDraftDialog.querySelector('[data-action="review-terminal-group-change"]');
  const renameButton = terminalGroupDraftDialog.querySelector('[data-action="save-terminal-group-rename"]');
  terminalChoicesFieldset.hidden = mode !== 'create';
  terminalGroupNameInput.hidden = mode === 'move';
  document.querySelector('label[for="terminalGroupNameInput"]').hidden = mode === 'move';
  terminalGroupDestinationSelect.hidden = mode !== 'move';
  destinationLabel.hidden = mode !== 'move';
  reviewButton.hidden = mode === 'rename';
  renameButton.hidden = mode !== 'rename';
}

function showTerminalGroupCreate() {
  if (!state.session) return;
  setTerminalGroupError();
  terminalGroupDraftForm.reset();
  terminalGroupDraft = { kind: 'create' };
  configureTerminalGroupDraft('create');
  populateTerminalChoices();
  showTerminalGroupDialog(terminalGroupDraftDialog, terminalGroupNameInput);
}

function showTerminalGroupRename(groupID) {
  const group = state.session.terminalGroups.find(candidate => candidate.id === groupID);
  if (!group) return;
  setTerminalGroupError();
  terminalGroupDraftForm.reset();
  terminalGroupDraft = { kind: 'rename', groupID };
  configureTerminalGroupDraft('rename');
  terminalGroupNameInput.value = group.name;
  showTerminalGroupDialog(terminalGroupDraftDialog, terminalGroupNameInput);
}

function showTerminalMoveDraft(terminalID) {
  const source = state.session.terminalGroups.find(group => group.terminalIds.includes(terminalID));
  if (!source) return;
  setTerminalGroupError();
  terminalGroupDraftForm.reset();
  terminalGroupDraft = { kind: 'move', terminalID, sourceGroupID: source.id };
  configureTerminalGroupDraft('move');
  terminalGroupDestinationSelect.innerHTML = '<option value="">ВЫБЕРИТЕ ГРУППУ</option>';
  if (source.terminalIds.length > 1) {
    const singletonOption = document.createElement('option');
    singletonOption.value = 'new-singleton';
    singletonOption.dataset.newSingleton = 'true';
    singletonOption.textContent = 'НОВАЯ ОДИНОЧНАЯ ГРУППА';
    terminalGroupDestinationSelect.appendChild(singletonOption);
  }
  for (const group of state.session.terminalGroups) {
    if (group.id === source.id) continue;
    const option = document.createElement('option');
    option.value = group.id;
    option.textContent = group.name;
    terminalGroupDestinationSelect.appendChild(option);
  }
  showTerminalGroupDialog(terminalGroupDraftDialog, terminalGroupDestinationSelect);
}

function terminalName(terminalID) {
  return state.session?.terminals.find(terminal => terminal.id === terminalID)?.name || terminalID;
}

function groupName(groups, groupID) {
  return groups.find(group => group.id === groupID)?.name || groupID || '—';
}

function uniqueLocalGroupName(base, groups) {
  const used = new Set(groups.map(group => group.name.trim().toLocaleLowerCase()));
  const encoder = new TextEncoder();
  const truncate = (value, byteLimit) => {
    let result = '';
    let byteCount = 0;
    for (const symbol of value) {
      const symbolBytes = encoder.encode(symbol).length;
      if (byteCount + symbolBytes > byteLimit) break;
      result += symbol;
      byteCount += symbolBytes;
    }
    return result;
  };
  const rawBase = String(base || 'Terminal').trim() || 'Terminal';
  let candidate = truncate(rawBase, 256);
  let suffix = 2;
  while (used.has(candidate.toLocaleLowerCase())) {
    const ending = ` (${suffix++})`;
    candidate = truncate(rawBase, 256 - encoder.encode(ending).length) + ending;
  }
  return candidate;
}

function newLocalSingletonGroup(terminalID, groups) {
  return {
    id: uid('group'),
    name: uniqueLocalGroupName(terminalName(terminalID), groups),
    terminalIds: [terminalID],
  };
}

function reviewTerminalGroupDraft() {
  if (!terminalGroupDraft || !state.session) return;
  const before = structuredClone(state.session.terminalGroups);
  if (terminalGroupDraft.kind === 'create') {
    const name = terminalGroupNameInput.value.trim();
    const terminalIDs = [...terminalGroupDraftDialog.querySelectorAll('input[name="terminalIds"]:checked')]
      .map(input => input.value);
    if (!name || terminalIDs.length < 2) {
      setTerminalGroupError('УКАЖИТЕ УНИКАЛЬНОЕ НАЗВАНИЕ И ВЫБЕРИТЕ НЕ МЕНЕЕ ДВУХ ТЕРМИНАЛОВ');
      return;
    }
    const selected = new Set(terminalIDs);
    const sourceNames = before.filter(group => group.terminalIds.some(id => selected.has(id))).map(group => group.name);
    const candidate = before
      .map(group => ({ ...group, terminalIds: group.terminalIds.filter(id => !selected.has(id)) }))
      .filter(group => group.terminalIds.length);
    if (candidate.some(group => group.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase())) {
      setTerminalGroupError('ГРУППА С ТАКИМ НАЗВАНИЕМ УЖЕ СУЩЕСТВУЕТ');
      return;
    }
    const reusableGroup = before.find(group =>
      group.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase()
      && group.terminalIds.every(id => selected.has(id)));
    candidate.push({ id: reusableGroup?.id || uid('group'), name, terminalIds: terminalIDs });
    showTerminalGroupImpact({
      kind: 'СОЗДАНИЕ ГРУППЫ', candidate, affectedGroupNames: [...new Set([...sourceNames, name])],
      affectedTerminalIDs: terminalIDs, membership: `${name}: ${terminalIDs.map(terminalName).join(' → ')}`,
      orderBefore: before.flatMap(group => group.terminalIds).map(terminalName),
      orderAfter: candidate.flatMap(group => group.terminalIds).map(terminalName),
    });
    return;
  }
  if (terminalGroupDraft.kind === 'move') {
    const destinationGroupID = terminalGroupDestinationSelect.value;
    const sourceGroupID = terminalGroupDraft.sourceGroupID;
    const terminalID = terminalGroupDraft.terminalID;
    if (!destinationGroupID) {
      setTerminalGroupError('ВЫБЕРИТЕ ГРУППУ НАЗНАЧЕНИЯ');
      return;
    }
    const candidate = before
      .map(group => ({ ...group, terminalIds: group.terminalIds.filter(id => id !== terminalID) }))
      .filter(group => group.terminalIds.length);
    const splitToSingleton = terminalGroupDestinationSelect.selectedOptions[0]?.dataset.newSingleton === 'true';
    if (splitToSingleton) {
      const singleton = newLocalSingletonGroup(terminalID, candidate);
      candidate.push(singleton);
      const source = candidate.find(group => group.id === sourceGroupID);
      showTerminalGroupImpact({
        kind: 'ОТДЕЛЕНИЕ ТЕРМИНАЛА', candidate,
        affectedGroupNames: [groupName(before, sourceGroupID), singleton.name],
        affectedTerminalIDs: [terminalID], sourceGroupID, destinationGroupID: singleton.id,
        destinationGroupName: singleton.name,
        membership: `${source.name}: ${source.terminalIds.map(terminalName).join(' → ')} · ${singleton.name}: ${terminalName(terminalID)}`,
        orderBefore: before.find(group => group.id === sourceGroupID)?.terminalIds.map(terminalName) || [],
        orderAfter: [...source.terminalIds, terminalID].map(terminalName),
      });
      return;
    }
    const destination = candidate.find(group => group.id === destinationGroupID);
    if (!destination) {
      setTerminalGroupError('ВЫБЕРИТЕ ГРУППУ НАЗНАЧЕНИЯ');
      return;
    }
    destination.terminalIds.push(terminalID);
    showTerminalGroupImpact({
      kind: 'ПЕРЕМЕЩЕНИЕ ТЕРМИНАЛА', candidate,
      affectedGroupNames: [groupName(before, sourceGroupID), groupName(before, destinationGroupID)],
      affectedTerminalIDs: [terminalID], sourceGroupID, destinationGroupID,
      membership: `${groupName(before, destinationGroupID)}: ${destination.terminalIds.map(terminalName).join(' → ')}`,
      orderBefore: before.find(group => group.id === sourceGroupID)?.terminalIds.map(terminalName) || [],
      orderAfter: destination.terminalIds.map(terminalName),
    });
  }
}

function prepareTerminalMemberOrder(groupID, terminalID, delta) {
  const before = structuredClone(state.session.terminalGroups);
  const candidate = structuredClone(before);
  const group = candidate.find(item => item.id === groupID);
  const index = group?.terminalIds.indexOf(terminalID) ?? -1;
  const next = index + delta;
  if (!group || index < 0 || next < 0 || next >= group.terminalIds.length) return;
  [group.terminalIds[index], group.terminalIds[next]] = [group.terminalIds[next], group.terminalIds[index]];
  showTerminalGroupImpact({
    kind: 'ИЗМЕНЕНИЕ ПОРЯДКА', candidate, affectedGroupNames: [group.name],
    affectedTerminalIDs: [terminalID, group.terminalIds[index]],
    membership: `${group.name}: ${group.terminalIds.map(terminalName).join(' → ')}`,
    orderBefore: before.find(item => item.id === groupID).terminalIds.map(terminalName),
    orderAfter: group.terminalIds.map(terminalName),
  });
}

function prepareTerminalGroupOrder(groupID, delta) {
  const before = structuredClone(state.session.terminalGroups);
  const candidate = structuredClone(before);
  const index = candidate.findIndex(group => group.id === groupID);
  const next = index + delta;
  if (index < 0 || next < 0 || next >= candidate.length) return;
  [candidate[index], candidate[next]] = [candidate[next], candidate[index]];
  showTerminalGroupImpact({
    kind: 'ИЗМЕНЕНИЕ ПОРЯДКА', candidate,
    affectedGroupNames: [candidate[index].name, candidate[next].name], affectedTerminalIDs: [],
    membership: candidate.map(group => group.name).join(' → '),
    orderBefore: before.map(group => group.name), orderAfter: candidate.map(group => group.name),
  });
}

function prepareTerminalGroupDissolution(groupID) {
  const before = structuredClone(state.session.terminalGroups);
  const group = before.find(candidate => candidate.id === groupID);
  if (!group) return;
  if (group.terminalIds.length === 1) {
    setTerminalGroupError('ОДИНОЧНУЮ ГРУППУ НЕЛЬЗЯ РАСФОРМИРОВАТЬ: ТЕРМИНАЛ ДОЛЖЕН ОСТАТЬСЯ В ГРУППЕ');
    return;
  }
  const candidate = before.filter(candidate => candidate.id !== groupID);
  const resultantSingletons = [];
  for (const terminalID of group.terminalIds) {
    const singleton = newLocalSingletonGroup(terminalID, candidate);
    candidate.push(singleton);
    resultantSingletons.push(singleton);
  }
  showTerminalGroupImpact({
    kind: 'РАСФОРМИРОВАНИЕ ГРУППЫ', candidate,
    affectedGroupNames: [group.name, ...resultantSingletons.map(singleton => singleton.name)],
    affectedTerminalIDs: group.terminalIds, sourceGroupID: group.id,
    membership: resultantSingletons
      .map(singleton => `${terminalName(singleton.terminalIds[0])} → ${singleton.name}`)
      .join(' · '),
    orderBefore: group.terminalIds.map(terminalName), orderAfter: group.terminalIds.map(terminalName),
  });
}

function currentSessionRevision() {
  return Number(saveStatus.dataset.savedRevision || startupStatus?.savedRevision || newestDurableRevision || 0);
}

function terminalGroupCandidateMembership(groups) {
  return groups
    .map(group => `${group.name}: ${group.terminalIds.map(terminalName).join(' → ')}`)
    .join(' · ');
}

function authoredTransitionRejections(message) {
  const pattern = /terminal transition command "([^"]+)" in terminal "([^"]+)" targets terminal "([^"]+)" and crosses groups "([^"]+)" and "([^"]+)"/g;
  return [...String(message || '').matchAll(pattern)].map(match => ({
    commandID: match[1],
    sourceTerminalID: match[2],
    targetTerminalID: match[3],
    sourceGroupID: match[4],
    targetGroupID: match[5],
  }));
}

function actionableTransitionRejection(rejections, candidate) {
  const edges = rejections.map(rejection =>
    `КОМАНДА «${rejection.commandID}»: «${terminalName(rejection.sourceTerminalID)}» → «${terminalName(rejection.targetTerminalID)}» ОСТАЁТСЯ МЕЖДУ ГРУППАМИ «${groupName(candidate, rejection.sourceGroupID)}» И «${groupName(candidate, rejection.targetGroupID)}»`);
  return `${edges.join(' · ')}. ДОБАВЬТЕ СВЯЗАННЫЕ ТЕРМИНАЛЫ В ОДНО ПРЕДЛОЖЕНИЕ И ПРОВЕРЬТЕ ЕГО ПЕРЕД ПРИМЕНЕНИЕМ.`;
}

function amendRejectedTerminalGroupImpact() {
  const impact = pendingTerminalGroupImpact;
  if (!impact?.rejections?.length) return;
  const involvedGroupIDs = new Set(impact.rejections.flatMap(rejection =>
    [rejection.sourceGroupID, rejection.targetGroupID]));
  const selectedTerminalIDs = impact.candidate
    .filter(group => involvedGroupIDs.has(group.id))
    .flatMap(group => group.terminalIds);
  const preferredGroup = impact.candidate.find(group => group.id === impact.destinationGroupID)
    || impact.candidate.find(group => group.id === impact.rejections[0].sourceGroupID);

  pendingTerminalGroupImpact = null;
  hideTerminalGroupDialog(terminalGroupImpactDialog);
  setTerminalGroupError();
  terminalGroupDraftForm.reset();
  terminalGroupDraft = { kind: 'create' };
  configureTerminalGroupDraft('create');
  populateTerminalChoices(selectedTerminalIDs);
  terminalGroupNameInput.value = preferredGroup?.name || '';
  showTerminalGroupDialog(terminalGroupDraftDialog, terminalGroupNameInput);
}

function showTerminalGroupImpact(impact) {
  setTerminalGroupError();
  setTerminalGroupImpactError();
  closeTerminalGroupDraft({ restoreFocus: false });
  pendingTerminalGroupImpact = {
    ...impact,
    expectedSessionRevision: currentSessionRevision(),
    expectedCoordinationRevision: coordinationRevision(state.coordination),
  };
  const values = {
    kind: impact.kind,
    groups: impact.affectedGroupNames.join(' · ') || '—',
    terminals: impact.affectedTerminalIDs.map(terminalName).join(' · ') || '—',
    'source-group': groupName(state.session.terminalGroups, impact.sourceGroupID),
    'destination-group': impact.destinationGroupName || groupName(state.session.terminalGroups, impact.destinationGroupID),
    membership: terminalGroupCandidateMembership(impact.candidate) || '—',
    'order-before': (impact.orderBefore || []).join(' → ') || '—',
    'order-after': (impact.orderAfter || []).join(' → ') || '—',
  };
  for (const [key, value] of Object.entries(values)) {
    terminalGroupImpactSummary.querySelector(`[data-impact="${key}"]`).textContent = value;
  }
  showTerminalGroupDialog(
    terminalGroupImpactDialog,
    terminalGroupImpactDialog.querySelector('[data-action="cancel-terminal-group-change"]'),
  );
}

async function submitTerminalGroupCandidate(candidate, expectedSessionRevision, expectedCoordinationRevision) {
  if (terminalGroupSubmitting) return false;
  terminalGroupSubmitting = true;
  const controls = terminalGroupImpactDialog.querySelectorAll('button');
  controls.forEach(control => { control.disabled = true; });
  const result = await desktopAPI.replaceTerminalGroups({
    terminalGroups: structuredClone(candidate), expectedSessionRevision, expectedCoordinationRevision,
  });
  terminalGroupSubmitting = false;
  controls.forEach(control => { control.disabled = false; });
  const revision = Number(result?.sessionRevision || 0);
  if (result?.session) applyAuthoredSession(result.session, result.authoringRevision, revision);
  if (result?.coordinationState) applyCoordinationState(result.coordinationState);
  if (!result?.ok) {
    renderOverseerError(terminalGroupError, result, 'terminal-groups');
    const rejections = authoredTransitionRejections(result?.error);
    if (pendingTerminalGroupImpact && rejections.length) {
      pendingTerminalGroupImpact.rejections = rejections;
      setTerminalGroupImpactError(
        actionableTransitionRejection(rejections, pendingTerminalGroupImpact.candidate),
        true,
      );
    }
    renderAll();
    return false;
  }
  setTerminalGroupError();
  setCoordinationStatus('ГРУППЫ СОХРАНЕНЫ');
  renderAll();
  return true;
}

async function saveTerminalGroupRename() {
  const group = state.session.terminalGroups.find(candidate => candidate.id === terminalGroupDraft?.groupID);
  const name = terminalGroupNameInput.value.trim();
  if (!group || !name) {
    setTerminalGroupError('НАЗВАНИЕ ГРУППЫ НЕ ДОЛЖНО БЫТЬ ПУСТЫМ');
    return;
  }
  if (state.session.terminalGroups.some(candidate => candidate.id !== group.id && candidate.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase())) {
    setTerminalGroupError('ГРУППА С ТАКИМ НАЗВАНИЕМ УЖЕ СУЩЕСТВУЕТ');
    return;
  }
  const candidate = structuredClone(state.session.terminalGroups);
  candidate.find(item => item.id === group.id).name = name;
  const ok = await submitTerminalGroupCandidate(candidate, currentSessionRevision(), coordinationRevision(state.coordination));
  if (ok) closeTerminalGroupDraft();
}

btnCreateTerminalGroup.addEventListener('click', showTerminalGroupCreate);
terminalGroupDraftDialog.querySelector('[data-action="review-terminal-group-change"]').addEventListener('click', reviewTerminalGroupDraft);
terminalGroupDraftDialog.querySelector('[data-action="save-terminal-group-rename"]').addEventListener('click', () => { void saveTerminalGroupRename(); });
for (const action of ['close-terminal-group-draft', 'cancel-terminal-group-draft']) {
  terminalGroupDraftDialog.querySelector(`[data-action="${action}"]`).addEventListener('click', () => closeTerminalGroupDraft());
}
for (const action of ['close-terminal-group-change', 'cancel-terminal-group-change']) {
  terminalGroupImpactDialog.querySelector(`[data-action="${action}"]`).addEventListener('click', () => closeTerminalGroupImpact());
}
terminalGroupImpactDialog.querySelector('[data-action="confirm-terminal-group-change"]').addEventListener('click', async () => {
  const impact = pendingTerminalGroupImpact;
  if (!impact || terminalGroupSubmitting) return;
  const ok = await submitTerminalGroupCandidate(impact.candidate, impact.expectedSessionRevision, impact.expectedCoordinationRevision);
  if (ok || !impact.rejections?.length) closeTerminalGroupImpact();
  else amendTerminalGroupChangeButton.focus();
});
amendTerminalGroupChangeButton.addEventListener('click', amendRejectedTerminalGroupImpact);
terminalGroupDraftDialog.addEventListener('cancel', event => { event.preventDefault(); closeTerminalGroupDraft(); });
terminalGroupImpactDialog.addEventListener('cancel', event => { event.preventDefault(); closeTerminalGroupImpact(); });

function buildTerminalRow(term, group, memberIndex) {
  const row = document.createElement('div');
  const selected = term.id === state.editTerminalId;
  row.className = 'term-row'
    + (selected ? ' editing' : '')
    + (term.id === state.liveTerminalId ? ' is-live' : '');
  row.dataset.terminalId = term.id;
  row.tabIndex = 0;
  row.setAttribute('role', 'listitem');
  row.setAttribute('aria-current', String(selected));

  const nameRow = document.createElement('div');
  nameRow.className = 'term-row-name';
  nameRow.textContent = term.name;
  row.appendChild(nameRow);

  const metaRow = document.createElement('div');
  metaRow.className = 'term-row-meta';
  metaRow.textContent = '● В ЭФИРЕ';
  row.appendChild(metaRow);

  const actions = terminalActionMenu({
    scope: 'terminal',
    ownerID: term.id,
    ownerName: term.name,
    items: [
      { label: 'ПЕРЕИМЕНОВАТЬ ТЕРМИНАЛ', action: 'rename-terminal', handler: () => startRenameTerminal(term, nameRow) },
      { label: 'ПЕРЕМЕСТИТЬ В ДРУГУЮ ГРУППУ', action: 'move-terminal', handler: () => showTerminalMoveDraft(term.id) },
      { label: 'ПЕРЕМЕСТИТЬ ТЕРМИНАЛ ВВЕРХ', action: 'move-terminal-up', handler: () => prepareTerminalMemberOrder(group.id, term.id, -1), disabled: memberIndex === 0 },
      { label: 'ПЕРЕМЕСТИТЬ ТЕРМИНАЛ ВНИЗ', action: 'move-terminal-down', handler: () => prepareTerminalMemberOrder(group.id, term.id, 1), disabled: memberIndex === group.terminalIds.length - 1 },
      { label: 'УДАЛИТЬ ТЕРМИНАЛ', action: 'delete-terminal', handler: () => deleteTerminal(term), destructive: true },
    ],
  });
  row.appendChild(actions);

  const selectTerminal = () => {
    if (state.editTerminalId === term.id) return;
    state.editTerminalId = term.id;
    state.selectedNodeId = null;
    state.expanded = new Set(['root']);
    renderAll();
  };
  row.addEventListener('click', selectTerminal);
  row.addEventListener('keydown', event => {
    if (event.target !== row || (event.key !== 'Enter' && event.key !== ' ')) return;
    event.preventDefault();
    selectTerminal();
  });
  return row;
}

function deleteTerminal(term) {
  if (state.liveTerminalId === term.id || state.coordination?.pendingSwitch?.sourceTerminalId === term.id) {
    setCoordinationStatus('АКТИВНЫЙ ИЛИ СОХРАНЁННЫЙ ТЕРМИНАЛ НЕЛЬЗЯ УДАЛИТЬ', true);
    return;
  }
  const inbound = inboundTerminalTransitions(term.id);
  if (inbound.length) {
    setCoordinationStatus(`ТЕРМИНАЛ ИСПОЛЬЗУЕТСЯ КОМАНДАМИ ПЕРЕХОДА: ${inbound.join(', ')}`, true);
    return;
  }
  if (!window.confirm(`Удалить терминал "${term.name}" целиком?`)) return;
  const terminalID = term.id;
  commitAuthoringEdit({
    label: `УДАЛЕНИЕ ТЕРМИНАЛА «${term.name}»`,
    mutate: session => {
      session.terminals = session.terminals.filter(candidate => candidate.id !== terminalID);
      session.terminalGroups = (session.terminalGroups ?? [])
        .map(group => ({ ...group, terminalIds: group.terminalIds.filter(id => id !== terminalID) }))
        .filter(group => group.terminalIds.length > 0);
    },
  });
  if (state.editTerminalId === terminalID) {
    state.editTerminalId = state.session.terminals[0]?.id || null;
    state.selectedNodeId = null;
    state.expanded = new Set(['root']);
  }
  renderAll();
}

function startRenameTerminal(term, nameRow) {
  const input = document.createElement('input');
  input.className = 'field-input';
  input.value = term.name;
  nameRow.replaceWith(input);
  input.focus();
  input.select();

  const commit = () => {
    const val = input.value.trim();
    if (val) {
      commitTerminalEdit('НАЗВАНИЕ ТЕРМИНАЛА', term.id, target => { target.name = val; });
    }
    renderAll();
  };
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') input.blur();
    if (e.key === 'Escape') renderAll();
  });
  input.addEventListener('blur', commit);
}

// ── Render: tree header + toolbar state ──────────────────────
function renderTreeHeader() {
  const term = getEditTerminal();
  editingTermName.textContent = term ? term.name : '—';
  const isLive = !!term && term.id === state.liveTerminalId;
  const broadcastActive = Boolean(state.coordination?.broadcast);
  liveFlag.hidden = !isLive;
  btnMakeLive.hidden = isLive;
  btnMakeLive.textContent = 'СДЕЛАТЬ АКТИВНЫМ';
  btnMakeLive.disabled = !term || !broadcastActive || coordinationCommandPending;
  terminalSettingsMenu.hidden = !isLive;
  if (!isLive) terminalSettingsMenu.open = false;
  btnReapplySettings.disabled = !isLive || coordinationCommandPending;
  btnStopBroadcast.hidden = !broadcastActive || !state.liveTerminalId;
  btnStopBroadcast.disabled = !broadcastActive || !state.liveTerminalId || coordinationCommandPending;
}

// ── Render: per-terminal settings (hack level / intro text) ──
function renderSettingsPanel() {
  const term = getEditTerminal();
  hackLevelSelect.disabled = !term;
  introTextArea.disabled   = !term;
  btnApplySettings.disabled = !term;
  const applied = { hackLevelSelect: term ? String(term.hackLevel || 0) : '0', introTextArea: term ? (term.introText || '') : '' };
  if (!term) {
    hackLevelSelect.value = applied.hackLevelSelect;
    introTextArea.value = applied.introTextArea;
  }
  const terminalID = term?.id;
  showUnappliedInput(termSettings, term && {
    key: `settings:${terminalID}`,
    label: `Настройки «${term.name}»`,
    exists: () => Boolean(state.session?.terminals.some(candidate => candidate.id === terminalID)),
    applied,
  });
}


// ── Render: live hack status (term panel footer) ──────────────
function renderHackStatus() {
  const liveTerm = state.session && state.liveTerminalId
    ? state.session.terminals.find(t => t.id === state.liveTerminalId)
    : null;

  if (!liveTerm || !liveTerm.hackLevel || !state.liveHack) {
    hackStatus.style.display = 'none';
    btnResetFailedHack.hidden = true;
    return;
  }

  hackStatus.style.display = '';
  const h = state.liveHack;
  if (h.solved) {
    hackStatusLine.textContent = 'ВЗЛОМ: ПРОЙДЕН';
  } else if (h.failed) {
    hackStatusLine.textContent = 'ВЗЛОМ: ЗАБЛОКИРОВАН';
  } else {
    hackStatusLine.textContent = `ВЗЛОМ: осталось попыток ${h.attemptsLeft}/${h.attemptsMax}`;
  }
  btnHackSuccess.disabled = h.solved || h.failed;
  btnHackSuccess.hidden = h.failed;
  btnResetFailedHack.hidden = !h.failed;
  btnResetFailedHack.disabled = !h.failed || coordinationCommandPending;
}

function renderToolbarState() {
  const term = getEditTerminal();
  const disabled = !term;
  btnAddFolder.disabled  = disabled;
  btnAddCommand.disabled = disabled;
  btnAddEntry.disabled   = disabled;
}

function renderToolbarHint() {
  const target = currentAddTarget();
  toolbarHint.textContent = target ? `Добавление в: ${target.id === 'root' ? ROOT_FOLDER_LABEL : target.name}` : '';
}

// ── Render: tree view ─────────────────────────────────────────
function renderTree() {
  const term = getEditTerminal();
  treeView.innerHTML = '';
  if (!term) {
    treeView.innerHTML = '<div class="tree-empty-hint">Нет терминала — создайте его слева</div>';
    return;
  }
  treeView.appendChild(renderNode(term.root, true));
}

function renderNode(node, isRoot) {
  const term = getEditTerminal();
  const wrap = document.createElement('div');
  wrap.className = 'tree-node';

  const row = document.createElement('div');
  const completed = commandExecutionState(term, node.id);
  row.className = 'tree-row'
    + (state.selectedNodeId === node.id ? ' selected' : '')
    + (completed ? ' command-completed' : '');

  const hasChildren = node.type === 'folder' && node.children && node.children.length > 0;
  const isExpanded   = state.expanded.has(node.id);

  const caret = document.createElement('span');
  caret.className = 'tree-caret';
  if (node.type === 'folder') {
    caret.textContent = hasChildren ? (isExpanded ? '▾' : '▸') : '·';
    caret.addEventListener('click', (e) => {
      e.stopPropagation();
      if (!hasChildren) return;
      if (isExpanded) state.expanded.delete(node.id); else state.expanded.add(node.id);
      renderTree();
    });
  }
  row.appendChild(caret);

  const icon = document.createElement('span');
  icon.className = 'tree-icon ' + node.type;
  icon.textContent = node.type === 'folder' ? '[ПАПКА]' : node.type === 'command' ? '[КОМАНДА]' : '[ЗАПИСЬ]';
  row.appendChild(icon);

  const label = document.createElement('span');
  label.className = 'tree-label';
  const seen = isRoot ? null : playerViewNode(term.id, node.id);
  label.textContent = isRoot ? ROOT_FOLDER_LABEL : (seen?.playerName || effectiveNodeName(term, node));
  if (!isRoot) {
    label.dataset.authoredName = node.name;
    if (seen && seen.playerName !== node.name) label.title = `В РЕДАКТОРЕ: ${node.name}`;
  }
  row.appendChild(label);
  const addBadge = (className, text) => {
    const badge = document.createElement('span');
    badge.className = `tree-badge ${className}`;
    badge.textContent = text;
    row.appendChild(badge);
  };
  if (seen && !seen.visible) addBadge('tree-badge-hidden', 'СКРЫТО ОТ ИГРОКОВ');
  if (seen?.available === false) addBadge('tree-badge-unavailable', 'НЕДОСТУПНО');

  row.addEventListener('click', () => {
    state.selectedNodeId = node.id;
    if (node.type === 'folder') state.expanded.add(node.id);
    renderTree();
    renderNodeForm();
    renderToolbarHint();
  });

  wrap.appendChild(row);

  if (node.type === 'folder' && isExpanded) {
    const childrenWrap = document.createElement('div');
    childrenWrap.className = 'tree-children';
    if (!node.children || node.children.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'tree-empty-hint';
      empty.textContent = '(пусто)';
      childrenWrap.appendChild(empty);
    } else {
      node.children.forEach(child => childrenWrap.appendChild(renderNode(child, false)));
    }
    wrap.appendChild(childrenWrap);
  }

  return wrap;
}

// ── Render: node property form ────────────────────────────────
function renderNodeForm() {
  const term = getEditTerminal();
  const loc = term && state.selectedNodeId ? locateNode(term.root, state.selectedNodeId) : null;
  if (!loc) {
    showUnappliedInput(nodeForm, null);
    nodeForm.innerHTML = '<div class="node-empty">Выберите узел дерева слева</div>';
    return;
  }
  const node = loc.node;

  if (node.id === 'root') {
    showUnappliedInput(nodeForm, null);
    nodeForm.innerHTML = `
      <div class="node-type-label">КОРЕНЬ ТЕРМИНАЛА</div>
      <div class="node-empty">Это главный экран терминала. Добавляйте папки, команды и записи через панель инструментов сверху.</div>`;
    return;
  }
  const unappliedKey = `node:${term.id}:${node.id}`;
  const stagedBlocks = unappliedInputs.get(unappliedKey)?.blocks || null;

  const typeLabel = node.type === 'folder' ? 'ПАПКА' : node.type === 'command' ? 'КОМАНДА' : 'ЗАПИСЬ';
  const snapshot = commandExecutionState(term, node.id);
  const hasExplicitEntryBlocks = node.type === 'entry' && Array.isArray(node.blocks) && node.blocks.length > 0;
  const hasLegacyEntryDescription = node.type === 'entry' && !hasExplicitEntryBlocks && typeof node.description === 'string' && node.description.length > 0;
  const entryBlocks = stagedBlocks || (hasExplicitEntryBlocks
    ? node.blocks
    : hasLegacyEntryDescription
      ? [{ id: '', initialText: node.description }]
      : []);
  const blockOwners = node.type === 'entry' ? entryBlockOwners(term) : null;
  let entryBlocksChanged = Boolean(stagedBlocks);
  let html = `<div class="node-type-label">${typeLabel}</div>
    <label class="field-label" for="fldName">${node.type === 'command' ? 'ИСХОДНОЕ НАЗВАНИЕ' : 'НАЗВАНИЕ'}</label>
    <input class="field-input" id="fldName" value="${escAttr(node.name)}">`;

  if (node.type === 'command') {
    const commandMode = node.stateChange
      ? 'state-change'
      : node.terminalTransition
        ? 'terminal-transition'
        : 'ordinary';
    const authoredGroup = state.session.terminalGroups?.find(group => group.terminalIds.includes(term.id));
    const eligibleTransitionIDs = new Set(authoredGroup?.terminalIds ?? []);
    const transitionOptions = state.session.terminals
      .filter(candidate => candidate.id !== term.id && eligibleTransitionIDs.has(candidate.id))
      .map(candidate => `<option value="${escAttr(candidate.id)}"${node.terminalTransition?.targetTerminalId === candidate.id ? ' selected' : ''}>${escHtml(candidate.name)}</option>`)
      .join('');
    const configuredEntryChange = snapshot?.entryContentChange ?? node.stateChange?.entryContentChange ?? null;
    const selectedBlockID = configuredEntryChange?.blockId || '';
    const completedBlockText = configuredEntryChange?.completedText ?? '';
    const entryTargetLocked = Boolean(snapshot);
    const facilityAction = node.stateChange?.facilityAction;
    const facilityActionSummary = facilityCommandActionSummary(facilityAction);
    const commandEffects = commandEffectItems(term, node);
    html += `
      <label class="field-label" for="fldCommandMode">РЕЖИМ КОМАНДЫ</label>
      <select class="field-input command-mode-select" id="fldCommandMode"${snapshot ? ' disabled' : ''}>
        <option value="ordinary"${commandMode === 'ordinary' ? ' selected' : ''}>ОБЫЧНАЯ КОМАНДА</option>
        <option value="state-change"${commandMode === 'state-change' ? ' selected' : ''}>ИЗМЕНЯЕТ СОСТОЯНИЕ</option>
        <option value="terminal-transition"${commandMode === 'terminal-transition' ? ' selected' : ''}>ПЕРЕХОД В ДРУГОЙ ТЕРМИНАЛ</option>
      </select>
      ${snapshot ? '<div class="command-mode-hint">Сначала сбросьте выполненное состояние команды, чтобы изменить режим.</div>' : ''}
      <div id="commandModeDecision" role="status" data-has-action="${facilityAction ? 'true' : 'false'}" hidden>
        <div data-decision-message></div>
        <div class="command-mode-decision-summary">${escHtml(facilityActionSummary)}</div>
        <button class="btn btn-secondary" id="btnKeepStateChangeMode" type="button">ОСТАВИТЬ «ИЗМЕНЯЕТ СОСТОЯНИЕ»</button>
        <button class="btn btn-danger" id="btnRemoveDeviceAction" type="button">УДАЛИТЬ ДЕЙСТВИЕ И ПЕРЕКЛЮЧИТЬ</button>
      </div>
      <div class="state-change-fields" id="stateChangeFields"${commandMode === 'state-change' ? '' : ' hidden'}>
        <label class="field-label" for="fldCompletedName">НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ</label>
        <input class="field-input" id="fldCompletedName" value="${escAttr(node.stateChange?.completedName || '')}">
        <label class="field-label" for="fldConfirmationText">ТЕКСТ ЗАПРОСА ПОДТВЕРЖДЕНИЯ</label>
        <textarea class="field-textarea state-change-textarea" id="fldConfirmationText">${escHtml(node.stateChange?.confirmationText || '')}</textarea>
        <label class="field-label" for="fldEntryBlockTarget">БЛОК ЗАПИСИ</label>
        <select class="field-input entry-block-target" id="fldEntryBlockTarget"${entryTargetLocked ? ' disabled' : ''}>
          <option value="">НЕ ИЗМЕНЯТЬ СОДЕРЖИМОЕ ЗАПИСИ</option>
          ${entryBlockTargetOptions(term, selectedBlockID)}
        </select>
        <label class="field-label" for="fldCompletedEntryBlockContent">ТЕКСТ БЛОКА ПОСЛЕ ВЫПОЛНЕНИЯ</label>
        <textarea class="field-textarea state-change-textarea" id="fldCompletedEntryBlockContent"${entryTargetLocked || !selectedBlockID ? ' disabled' : ''}>${escHtml(completedBlockText)}</textarea>
        ${entryTargetLocked ? '<div class="command-mode-hint">Сначала сбросьте выполненное состояние команды, чтобы изменить блок и его итоговый текст.</div>' : ''}
      </div>
      <div class="state-change-fields terminal-transition-fields" id="terminalTransitionFields"${commandMode === 'terminal-transition' ? '' : ' hidden'}>
        <label class="field-label" for="fldTerminalTransitionTarget">ЦЕЛЕВОЙ ТЕРМИНАЛ</label>
        <select class="field-input" id="fldTerminalTransitionTarget">
          <option value="">ВЫБЕРИТЕ ТЕРМИНАЛ</option>${transitionOptions}
        </select>
      </div>
      <label class="field-label" for="fldText">ТЕКСТ УСПЕШНОГО ВЫПОЛНЕНИЯ</label>
      <textarea class="field-textarea" id="fldText">${escHtml(node.text || '')}</textarea>
      <section class="command-facility-action" aria-labelledby="commandFacilityActionHeading"${commandMode === 'state-change' ? '' : ' hidden'}>
        <div class="field-label" id="commandFacilityActionHeading">ДЕЙСТВИЕ КОМАНДЫ</div>
        <div class="command-facility-action-summary">${escHtml(facilityActionSummary)}</div>
        <button class="btn btn-secondary" id="btnEditCommandFacilityAction" type="button" aria-haspopup="dialog" aria-controls="facilityCommandActionDialog">
          ${facilityAction ? 'ИЗМЕНИТЬ ДЕЙСТВИЕ КОМАНДЫ' : 'ДОБАВИТЬ ДЕЙСТВИЕ КОМАНДЫ'}
        </button>
      </section>
      <section class="command-effects" id="commandEffects" aria-labelledby="commandEffectsHeading"${commandEffects.length ? '' : ' hidden'}>
        <div class="field-label" id="commandEffectsHeading">ПРИ ОДОБРЕНИИ</div>
        <ul>${commandEffects.map(effect => `<li class="command-effect" data-kind="${effect.kind}">${escHtml(effect.text)}</li>`).join('')}</ul>
      </section>
      <section class="command-rules" id="commandRules" aria-labelledby="commandRulesHeading">
        <div class="field-label" id="commandRulesHeading">ЧТО ВИДЯТ ИГРОКИ</div>
        <div class="command-rule-targets"></div>
      </section>`;
    if (snapshot) {
      html += `
        <div class="command-execution-snapshot" role="status" aria-label="СОХРАНЁННОЕ СОСТОЯНИЕ КОМАНДЫ">
          <div class="command-execution-heading">ВЫПОЛНЕНО</div>
          <div class="command-execution-label">ЗАФИКСИРОВАННОЕ НАЗВАНИЕ</div>
          <div class="command-execution-value">${escHtml(snapshot.completedName || '')}</div>
          <div class="command-execution-label">ЗАФИКСИРОВАННЫЙ РЕЗУЛЬТАТ</div>
          <div class="command-execution-value command-execution-result">${escHtml(snapshot.resultText || '')}</div>
          ${snapshot.entryContentChange ? `<div class="command-execution-label">ЗАФИКСИРОВАННЫЙ ТЕКСТ БЛОКА</div>
          <div class="command-execution-value command-execution-result">${escHtml(snapshot.entryContentChange.completedText)}</div>` : ''}
        </div>`;
    }
  } else if (node.type === 'entry') {
    html += `<div class="field-label">СОДЕРЖИМОЕ ЗАПИСИ</div>
      <div class="entry-content-blocks" id="entryContentBlocks" aria-label="БЛОКИ СОДЕРЖИМОГО ЗАПИСИ">
        ${entryBlocks.map((block, index) => entryContentBlockMarkup(term, block, index, {
          legacyDraft: hasLegacyEntryDescription,
          owners: blockOwners,
        })).join('')}
      </div>
      <button class="btn btn-mini entry-content-block-add" id="btnAddEntryContentBlock" type="button">+ ДОБАВИТЬ БЛОК</button>`;
  } else if (node.type === 'folder') {
    const count = node.children ? node.children.length : 0;
    html += `<div class="field-label">СОДЕРЖИМОЕ</div><div class="node-empty">${count} элемент(ов)</div>`;
  }

  html += '<div class="node-validation-error" id="nodeValidationError" role="alert" hidden></div>';
  html += `<div class="node-actions">
      <button class="btn btn-primary" id="btnApplyNode">ПРИМЕНИТЬ</button>
      <button class="btn btn-secondary" id="btnCancelNodeInput" type="button" data-unapplied-cancel hidden>ОТМЕНА</button>
      ${snapshot ? '<button class="btn btn-secondary" id="btnResetCommandState" type="button" aria-describedby="btnResetCommandStateReason">СБРОСИТЬ СОСТОЯНИЕ</button>' : ''}
      <button class="btn btn-danger" id="btnDeleteNode">УДАЛИТЬ</button>
    </div>
    ${snapshot ? '<span class="control-reason" id="btnResetCommandStateReason"></span>' : ''}`;

  nodeForm.innerHTML = html;
  renderCommandRules();
  const appliedNodeFields = fieldValues(nodeForm);
  // currentEntryBlocks returns the edited block rows, or null when the blocks
  // are unchanged.
  const currentEntryBlocks = () => (node.type === 'entry' && entryBlocksChanged
    ? Array.from(nodeForm.querySelectorAll('#entryContentBlocks .entry-content-block')).map((row) => ({
      id: row.dataset.blockId || uniqueEntryBlockID(term),
      initialText: row.querySelector('[data-entry-block-content]').value,
    }))
    : null);

  const validationError = document.getElementById('nodeValidationError');
  const showValidationError = (message, field) => {
    validationError.textContent = message;
    validationError.hidden = false;
    field?.focus();
  };

  if (node.type === 'entry') {
    const blockList = document.getElementById('entryContentBlocks');
    const refreshBlockPositions = () => {
      const rows = Array.from(blockList.querySelectorAll('.entry-content-block'));
      rows.forEach((row, index) => {
        const position = index + 1;
        row.querySelector('.entry-content-block-index').textContent = `БЛОК ${position}`;
        const content = row.querySelector('[data-entry-block-content]');
        content.setAttribute('aria-label', `ИСХОДНЫЙ ТЕКСТ БЛОКА ${position}`);
        const actions = row.querySelector('.entry-content-block-actions');
        actions.setAttribute('aria-label', `Действия блока ${position}`);
        const moveUp = row.querySelector('[data-entry-block-action="move-up"]');
        const moveDown = row.querySelector('[data-entry-block-action="move-down"]');
        const remove = row.querySelector('[data-entry-block-action="delete"]');
        const configure = row.querySelector('[data-entry-block-action="configure-command"]');
        moveUp.disabled = index === 0;
        moveDown.disabled = index === rows.length - 1;
        moveUp.setAttribute('aria-label', `Переместить блок ${position} вверх`);
        moveDown.setAttribute('aria-label', `Переместить блок ${position} вниз`);
        remove.setAttribute('aria-label', `Удалить блок ${position}`);
        configure.setAttribute('aria-label', `Настроить команду для блока ${position}`);
      });
    };
    const bindBlockRow = (row) => {
      row.querySelector('[data-entry-block-content]').addEventListener('input', () => {
        entryBlocksChanged = true;
        validationError.hidden = true;
      });
      row.querySelectorAll('[data-entry-block-action]').forEach((button) => {
        button.addEventListener('click', () => {
          const action = button.dataset.entryBlockAction;
          if (action === 'configure-command') {
            const blockID = row.dataset.blockId;
            if (blockID) openEntryBlockCommandDialog(term, blockID);
            return;
          }
          if (action === 'delete') {
            const owner = row.querySelector('[data-entry-block-owner]');
            if (owner) {
              const command = locateNode(term.root, owner.dataset.commandId)?.node;
              showValidationError(entryBlockDeletionError(term, command ? [command] : []), button);
              return;
            }
            if (!window.confirm('Удалить этот блок содержимого?')) return;
            row.remove();
          } else if (action === 'move-up' && row.previousElementSibling) {
            row.previousElementSibling.before(row);
          } else if (action === 'move-down' && row.nextElementSibling) {
            row.nextElementSibling.after(row);
          } else {
            return;
          }
          entryBlocksChanged = true;
          validationError.hidden = true;
          refreshBlockPositions();
          stageUnappliedInput(nodeForm);
        });
      });
    };
    blockList.querySelectorAll('.entry-content-block').forEach(bindBlockRow);
    document.getElementById('btnAddEntryContentBlock').addEventListener('click', () => {
      const block = { id: uniqueEntryBlockID(term), initialText: '' };
      blockList.insertAdjacentHTML('beforeend', entryContentBlockMarkup(
        term,
        block,
        blockList.querySelectorAll('.entry-content-block').length,
        { owners: blockOwners },
      ));
      const row = blockList.lastElementChild;
      bindBlockRow(row);
      entryBlocksChanged = true;
      validationError.hidden = true;
      refreshBlockPositions();
      stageUnappliedInput(nodeForm);
      row.querySelector('[data-entry-block-content]').focus();
    });
    refreshBlockPositions();
  }

  if (node.type === 'command') {
    const mode = document.getElementById('fldCommandMode');
    const fields = document.getElementById('stateChangeFields');
    const transitionFields = document.getElementById('terminalTransitionFields');
    const actionSection = nodeForm.querySelector('.command-facility-action');
    const decision = document.getElementById('commandModeDecision');
    const applyButton = document.getElementById('btnApplyNode');
    let removeDeviceAction = false;
    const updateModePresentation = () => {
      fields.hidden = mode.value !== 'state-change';
      transitionFields.hidden = mode.value !== 'terminal-transition';
      actionSection.hidden = mode.value !== 'state-change';
      const deciding = decision.dataset.hasAction === 'true' && mode.value !== 'state-change';
      if (!deciding) removeDeviceAction = false;
      decision.hidden = !deciding;
      decision.querySelector('[data-decision-message]').textContent = removeDeviceAction
        ? 'ДЕЙСТВИЕ КОМАНДЫ БУДЕТ УДАЛЕНО, КОГДА ВЫ ПРИМЕНИТЕ ФОРМУ.'
        : 'ВЫБЕРИТЕ, ЧТО СДЕЛАТЬ С ДЕЙСТВИЕМ КОМАНДЫ. ОНО РАБОТАЕТ ТОЛЬКО В РЕЖИМЕ «ИЗМЕНЯЕТ СОСТОЯНИЕ».';
      document.getElementById('btnRemoveDeviceAction').hidden = removeDeviceAction;
      applyButton.disabled = deciding && !removeDeviceAction;
    };
    mode.addEventListener('change', () => {
      updateModePresentation();
      validationError.hidden = true;
      validationError.textContent = '';
    });
    document.getElementById('btnKeepStateChangeMode').addEventListener('click', () => {
      mode.value = 'state-change';
      updateModePresentation();
      // A scripted value fires no input event, so restage the form here:
      // the mode is back to its applied value and nothing is unapplied.
      stageUnappliedInput(nodeForm);
      mode.focus();
    });
    document.getElementById('btnRemoveDeviceAction').addEventListener('click', () => {
      removeDeviceAction = true;
      updateModePresentation();
      mode.focus();
    });
    updateModePresentation();
    const entryTarget = document.getElementById('fldEntryBlockTarget');
    const completedBlockContent = document.getElementById('fldCompletedEntryBlockContent');
    entryTarget.addEventListener('change', () => {
      completedBlockContent.disabled = !entryTarget.value;
      const owner = commandOwningEntryBlock(term, entryTarget.value, node.id);
      if (owner) {
        showValidationError(
          `ЭТОТ БЛОК УЖЕ ИЗМЕНЯЕТСЯ КОМАНДОЙ: ${effectiveNodeName(term, owner)}`,
          entryTarget,
        );
      } else {
        validationError.hidden = true;
        validationError.textContent = '';
      }
    });
    // The dialog starts a new stateChange's player texts from this form.
    const editAction = document.getElementById('btnEditCommandFacilityAction');
    editAction.addEventListener('click', () => openFacilityCommandActionDialog(term.id, node.id, {
      opener: editAction,
      texts: {
        completedName: document.getElementById('fldCompletedName').value,
        confirmationText: document.getElementById('fldConfirmationText').value,
      },
    }));
  }

  document.getElementById('btnApplyNode').addEventListener('click', () => {
    const nameEl = document.getElementById('fldName');
    const name = nameEl.value.trim();
    if (!name) {
      showValidationError(
        node.type === 'command' ? 'УКАЖИТЕ ИСХОДНОЕ НАЗВАНИЕ КОМАНДЫ' : 'УКАЖИТЕ НАЗВАНИЕ',
        nameEl
      );
      return;
    }

    let commandEdit = null;
    if (node.type === 'command') {
      const commandMode = document.getElementById('fldCommandMode').value;
      const textEl = document.getElementById('fldText');
      let nextStateChange = null;
      let nextTerminalTransition = null;
      if (commandMode === 'state-change') {
        const completedNameEl = document.getElementById('fldCompletedName');
        const confirmationTextEl = document.getElementById('fldConfirmationText');
        if (!completedNameEl.value.trim()) {
          showValidationError('УКАЖИТЕ НАЗВАНИЕ ПОСЛЕ ВЫПОЛНЕНИЯ', completedNameEl);
          return;
        }
        if (!confirmationTextEl.value.trim()) {
          showValidationError('УКАЖИТЕ ТЕКСТ ЗАПРОСА ПОДТВЕРЖДЕНИЯ', confirmationTextEl);
          return;
        }
        if (!textEl.value.trim()) {
          showValidationError('УКАЖИТЕ ТЕКСТ УСПЕШНОГО РЕЗУЛЬТАТА', textEl);
          return;
        }
        nextStateChange = {
          completedName: completedNameEl.value,
          confirmationText: confirmationTextEl.value,
        };
        const entryTargetEl = document.getElementById('fldEntryBlockTarget');
        const targetBlockID = entryTargetEl.value;
        if (targetBlockID) {
          const targetExists = entryBlockCatalog(term).some(({ block }) => block.id === targetBlockID);
          if (!targetExists) {
            showValidationError('ВЫБЕРИТЕ СУЩЕСТВУЮЩИЙ БЛОК ЗАПИСИ ЭТОГО ТЕРМИНАЛА', entryTargetEl);
            return;
          }
          const owner = commandOwningEntryBlock(term, targetBlockID, node.id);
          if (owner) {
            showValidationError(
              `ЭТОТ БЛОК УЖЕ ИЗМЕНЯЕТСЯ КОМАНДОЙ: ${effectiveNodeName(term, owner)}`,
              entryTargetEl,
            );
            return;
          }
          nextStateChange.entryContentChange = {
            blockId: targetBlockID,
            completedText: document.getElementById('fldCompletedEntryBlockContent').value,
          };
        }
      }
      if (commandMode === 'terminal-transition') {
        const targetEl = document.getElementById('fldTerminalTransitionTarget');
        const targetID = targetEl.value;
        if (!targetID || targetID === term.id || !state.session.terminals.some(candidate => candidate.id === targetID)) {
          showValidationError('ВЫБЕРИТЕ ДРУГОЙ СУЩЕСТВУЮЩИЙ ТЕРМИНАЛ', targetEl);
          return;
        }
        nextTerminalTransition = { targetTerminalId: targetID };
      }
      commandEdit = { stateChange: nextStateChange, terminalTransition: nextTerminalTransition, text: textEl.value };
    }

    const blocks = currentEntryBlocks();
    const nodeID = node.id;
    commitTerminalEdit(`${nodeKindLabel(node)} «${name}»`, term.id, candidateTerm => {
      const target = locateNode(candidateTerm.root, nodeID)?.node;
      if (!target) throw new Error('node removed');
      if (commandEdit) {
        const stateChange = commandEdit.stateChange && structuredClone(commandEdit.stateChange);
        if (stateChange && target.stateChange?.facilityAction) {
          stateChange.facilityAction = structuredClone(target.stateChange.facilityAction);
        }
        if (stateChange) target.stateChange = stateChange;
        else delete target.stateChange;
        if (commandEdit.terminalTransition) target.terminalTransition = { ...commandEdit.terminalTransition };
        else delete target.terminalTransition;
        target.text = commandEdit.text;
      }
      target.name = name;
      if (blocks) {
        if (blocks.length > 0) target.blocks = structuredClone(blocks);
        else delete target.blocks;
        delete target.description;
      }
    }, takeUnappliedInput(nodeForm));
    renderTree();
    renderNodeForm();
    renderToolbarHint();
  });

  document.getElementById('btnCancelNodeInput').addEventListener('click', () => {
    takeUnappliedInput(nodeForm);
    renderNodeForm();
    renderSaveStatus();
  });

  const btnResetCommandState = document.getElementById('btnResetCommandState');
  if (btnResetCommandState) {
    // The form re-renders when a session-state command starts and ends.
    explainControl(btnResetCommandState, sessionStateCommandPending ? 'ВЫПОЛНЯЕТСЯ ДРУГАЯ КОМАНДА' : '');
    btnResetCommandState.addEventListener('click', async () => {
      if (controlBlocked(btnResetCommandState)) return;
      const displayedName = snapshot?.completedName || node.name;
      if (!await confirmCommandStateReset(`Сбросить выполненное состояние команды "${displayedName}"?`)) return;
      const revisionBeforeReset = newestDurableRevision;
      runSessionStateCommand(
        () => desktopAPI.resetCommandState({ terminalId: term.id, commandId: node.id }),
        'СОСТОЯНИЕ КОМАНДЫ СБРОШЕНО',
        (result) => {
          const revision = Number(result?.revision || 0);
          const canonicalTerminal = result?.session?.terminals?.find(candidate => candidate.id === term.id);
          return revision > revisionBeforeReset && canonicalTerminal &&
            !Object.hasOwn(canonicalTerminal.commandStates ?? {}, node.id);
        }
      );
    });
  }

  document.getElementById('btnDeleteNode').addEventListener('click', () => {
    const targetingCommands = targetingCommandsWithinNode(term, node);
    if (targetingCommands.length > 0) {
      showValidationError(entryBlockDeletionError(term, targetingCommands));
      return;
    }
    const childCount = (node.type === 'folder' && node.children) ? node.children.length : 0;
    const msg = childCount > 0
      ? `Удалить "${node.name}" вместе со всем содержимым (${childCount} элемент(ов))?`
      : `Удалить "${node.name}"?`;
    if (!window.confirm(msg)) return;
    const nodeID = node.id;
    commitTerminalEdit(`УДАЛЕНИЕ «${node.name}»`, term.id, candidateTerm => {
      const parent = locateNode(candidateTerm.root, nodeID)?.parent;
      if (parent) parent.children = parent.children.filter(child => child.id !== nodeID);
    });
    state.selectedNodeId = null;
    renderTree();
    renderNodeForm();
    renderToolbarHint();
  });

  const terminalID = term.id;
  const nodeID = node.id;
  showUnappliedInput(nodeForm, {
    key: unappliedKey,
    label: `${nodeKindLabel(node)} «${node.name}»`,
    exists: () => {
      const terminal = state.session?.terminals.find(candidate => candidate.id === terminalID);
      return Boolean(terminal && locateNode(terminal.root, nodeID));
    },
    applied: appliedNodeFields,
    blocks: currentEntryBlocks,
  });
}

// ── Toolbar: add nodes ────────────────────────────────────────
function addNode(type) {
  const term = getEditTerminal();
  const target = currentAddTarget();
  if (!term || !target) return;

  const node = {
    id:   uid('n'),
    type,
    name: type === 'folder' ? 'Новая папка' : type === 'command' ? 'Новая команда' : 'Новая запись',
  };
  if (type === 'folder')  node.children = [];
  if (type === 'command') node.text = '';
  if (type === 'entry')   node.description = '';

  const targetID = target.id;
  commitTerminalEdit(`НОВЫЙ ЭЛЕМЕНТ «${node.name}»`, term.id, candidateTerm => {
    const parent = locateNode(candidateTerm.root, targetID)?.node;
    if (!parent) throw new Error('folder removed');
    parent.children ??= [];
    parent.children.push(structuredClone(node));
  });
  state.expanded.add(targetID);
  state.selectedNodeId = node.id;

  renderTree();
  renderNodeForm();
  renderToolbarHint();
}

btnAddFolder.addEventListener('click', () => addNode('folder'));
btnAddCommand.addEventListener('click', () => addNode('command'));
btnAddEntry.addEventListener('click', () => addNode('entry'));

// ── Player roster and broadcast management ──────────────────
btnManageLogicalSessions.addEventListener('click', showLogicalSessionManagement);
btnCloseLogicalSessions.addEventListener('click', hideLogicalSessionManagement);
logicalSessionDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  hideLogicalSessionManagement();
});
btnManagePlayers.addEventListener('click', showPlayerManagement);
btnClosePlayerManagement.addEventListener('click', hidePlayerManagement);
playerManagementDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  hidePlayerManagement();
});
btnCancelPlayerDelete.addEventListener('click', () => hidePlayerDeleteConfirmation(true));
playerDeleteDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  if (!coordinationCommandPending) hidePlayerDeleteConfirmation(true);
});
btnConfirmPlayerDelete.addEventListener('click', async () => {
  const characterId = pendingPlayerDelete?.characterId || '';
  if (!characterId || !playerMutationAllowed()) return;
  const result = await runPlayerManagementMutation(
    () => desktopAPI.deleteCharacter({
      characterId,
      expectedRevision: coordinationRevision(state.coordination),
    }),
    'ИГРОК УДАЛЁН',
    'УДАЛЕНИЕ ИГРОКА...'
  );
  hidePlayerDeleteConfirmation(!result?.ok);
});

playerManagementAddForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  if (!playerManagementAddForm.checkValidity()) {
    playerManagementAddForm.reportValidity();
    return;
  }

  const name = playerNameInput.value.trim();
  const intelligence = playerIntelligenceInput.valueAsNumber;
  const hackerChoice = playerHackerPerkAvailability.value;
  if (!name) {
    setPlayerManagementFeedback('УКАЖИТЕ ИМЯ ИГРОКА', true);
    playerNameInput.focus();
    return;
  }
  if (!Number.isInteger(intelligence) || intelligence < 1 || intelligence > 10) {
    setPlayerManagementFeedback('ИНТЕЛЛЕКТ ДОЛЖЕН БЫТЬ ЦЕЛЫМ ЧИСЛОМ ОТ 1 ДО 10', true);
    playerIntelligenceInput.focus();
    return;
  }
  if (hackerChoice !== 'true' && hackerChoice !== 'false') {
    setPlayerManagementFeedback('ВЫБЕРИТЕ ДОСТУПНОСТЬ ПЕРКА «ХАКЕР»', true);
    playerHackerPerkAvailability.focus();
    return;
  }

  await addPlayerProfile({
    name,
    intelligence,
    hackerPerkAvailable: hackerChoice === 'true',
  });
});

btnStartBroadcast.addEventListener('click', async () => {
  if (coordinationCommandPending || state.coordination?.broadcast || !state.coordination?.playerConfig) return;
  coordinationCommandPending = true;
  setCoordinationStatus('ЗАПУСК ТРАНСЛЯЦИИ...');
  renderCoordination();
  const result = await desktopAPI.startBroadcast();
  coordinationCommandPending = false;
  if (!result?.ok) {
    setCoordinationError(result, 'broadcast');
    renderCoordination();
    return;
  }

  state.coordination = result.state || state.coordination;
  setCoordinationStatus('ТРАНСЛЯЦИЯ ЗАПУЩЕНА');
  renderCoordination();
  renderTreeHeader();
});

btnEndBroadcast.addEventListener('click', () => {
  if (coordinationCommandPending || !state.coordination?.broadcast) return;
  showEndBroadcastConfirmation();
});

btnCancelEndBroadcast.addEventListener('click', () => hideEndBroadcastConfirmation());
endBroadcastDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  if (!coordinationCommandPending) hideEndBroadcastConfirmation();
});

btnConfirmEndBroadcast.addEventListener('click', async () => {
  if (coordinationCommandPending || !state.coordination?.broadcast) return;
  btnCancelEndBroadcast.disabled = true;
  btnConfirmEndBroadcast.disabled = true;
  const result = await runCoordinationCommand(
    () => desktopAPI.endBroadcast(),
    'ТРАНСЛЯЦИЯ ЗАВЕРШЕНА · СЕССИИ И ПЕРСОНАЖИ СОХРАНЕНЫ',
    'ЗАВЕРШЕНИЕ ТРАНСЛЯЦИИ...',
    'broadcast'
  );
  if (!result?.ok) {
    hideEndBroadcastConfirmation();
    return;
  }
  if (!result.state || result.state.broadcast) {
    setCoordinationStatus('Завершение трансляции не подтверждено. Проверьте состояние трансляции и завершите её снова.', true);
    renderCoordination();
    hideEndBroadcastConfirmation();
    return;
  }
  hideEndBroadcastConfirmation({ restoreFocus: false });
  hideTerminalSwitchDecision();
  state.liveHack = null;
  renderAll();
  btnStartBroadcast.focus();
});

// ── Terminal management ───────────────────────────────────────
function showCreateTerminalDialog() {
  if (!state.session || createTerminalSubmitting) return;
  createTerminalName.value = '';
  createTerminalError.textContent = '';
  createTerminalError.hidden = true;
  btnCancelCreateTerminal.disabled = false;
  btnConfirmCreateTerminal.disabled = false;
  createTerminalDialog.hidden = false;
  if (typeof createTerminalDialog.showModal === 'function' && !createTerminalDialog.open) {
    createTerminalDialog.showModal();
  } else {
    createTerminalDialog.setAttribute('open', '');
  }
  createTerminalName.focus();
}

function hideCreateTerminalDialog({ restoreFocus = true } = {}) {
  if (typeof createTerminalDialog.close === 'function' && createTerminalDialog.open) {
    createTerminalDialog.close();
  } else {
    createTerminalDialog.removeAttribute('open');
  }
  createTerminalDialog.hidden = true;
  createTerminalSubmitting = false;
  btnCancelCreateTerminal.disabled = false;
  btnConfirmCreateTerminal.disabled = false;
  if (restoreFocus && !btnAddTerminal.hidden) btnAddTerminal.focus();
}

btnAddTerminal.addEventListener('click', showCreateTerminalDialog);
btnReapplyEdit.addEventListener('click', reapplyHeldEdits);
for (const container of [nodeForm, termSettings]) {
  container.addEventListener('input', () => stageUnappliedInput(container));
  container.addEventListener('change', () => stageUnappliedInput(container));
}
btnUndo.addEventListener('click', () => runHistoryEntry('undo'));
btnRedo.addEventListener('click', () => runHistoryEntry('redo'));

btnCancelCreateTerminal.addEventListener('click', () => {
  if (!createTerminalSubmitting) hideCreateTerminalDialog();
});

createTerminalDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  if (!createTerminalSubmitting) hideCreateTerminalDialog();
});

createTerminalForm.addEventListener('submit', (event) => {
  event.preventDefault();
  if (!state.session || createTerminalSubmitting) return;
  const name = createTerminalName.value.trim();
  if (!name) {
    createTerminalError.textContent = 'УКАЖИТЕ НАЗВАНИЕ ТЕРМИНАЛА';
    createTerminalError.hidden = false;
    createTerminalName.focus();
    return;
  }

  createTerminalSubmitting = true;
  btnCancelCreateTerminal.disabled = true;
  btnConfirmCreateTerminal.disabled = true;
  const term = {
    id:        uid('t'),
    name,
    hackLevel: 0,
    introText: '',
    root:      { id: 'root', type: 'folder', name: 'ROOT', children: [] },
  };
  commitAuthoringEdit({
    label: `НОВЫЙ ТЕРМИНАЛ «${name}»`,
    mutate: session => {
      session.terminals.push(structuredClone(term));
      ensureSessionTerminalGroups(session);
    },
    reopen: reason => {
      if (state.editTerminalId === term.id) state.editTerminalId = state.session.terminals[0]?.id || null;
      showCreateTerminalDialog();
      createTerminalName.value = name;
      createTerminalError.textContent = reason;
      createTerminalError.hidden = false;
    },
  });
  state.editTerminalId = term.id;
  state.selectedNodeId = null;
  state.expanded = new Set(['root']);
  renderAll();
  hideCreateTerminalDialog({ restoreFocus: false });
  hackLevelSelect.focus();
});

btnApplySettings.addEventListener('click', () => {
  const term = getEditTerminal();
  if (!term) return;
  const hackLevel = Number(hackLevelSelect.value) || 0;
  const introText = introTextArea.value;
  // The save reaches players at once; a new hack level applies on the next
  // activation so it never disrupts an in-progress hack.
  commitTerminalEdit('НАСТРОЙКИ ТЕРМИНАЛА', term.id, target => {
    target.hackLevel = hackLevel;
    target.introText = introText;
  }, takeUnappliedInput(termSettings));
});

btnCancelSettings.addEventListener('click', () => {
  takeUnappliedInput(termSettings);
  renderSettingsPanel();
  renderSaveStatus();
});

function terminalActivationRequest(term) {
  return desktopAPI.requestTerminalActivation({
    terminalId: term.id,
    terminalName: term.name,
    tree: term.root,
    hackLevel: term.hackLevel || 0,
    introText: term.introText || '',
  });
}

btnMakeLive.addEventListener('click', async () => {
  const term = getEditTerminal();
  if (!term || term.id === state.liveTerminalId || !state.coordination?.broadcast || coordinationCommandPending) return;
  const result = await runTerminalSwitchRequest(
    () => terminalActivationRequest(term),
    'АКТИВНЫЙ ТЕРМИНАЛ ВЫБРАН',
    'ПЕРЕКЛЮЧЕНИЕ АКТИВНОГО ТЕРМИНАЛА...'
  );
  if (result?.ok && result.status === 'activated') state.liveHack = null;
  renderTermList();
  renderTreeHeader();
  renderHackStatus();
});

btnReapplySettings.addEventListener('click', async () => {
  const term = getEditTerminal();
  if (!term || term.id !== state.liveTerminalId || !state.coordination?.broadcast || coordinationCommandPending) return;
  terminalSettingsMenu.open = false;
  const result = await runTerminalSwitchRequest(
    () => terminalActivationRequest(term),
    'НАСТРОЙКИ АКТИВНОГО ТЕРМИНАЛА ПЕРЕПРИМЕНЕНЫ',
    'ПЕРЕПРИМЕНЕНИЕ НАСТРОЕК...'
  );
  if (result?.ok && result.status === 'activated') state.liveHack = null;
  renderTermList();
  renderTreeHeader();
  renderHackStatus();
});


btnStopBroadcast.addEventListener('click', showTakeOffAirConfirmation);

btnCancelTakeOffAir.addEventListener('click', () => {
  if (!takeOffAirPending) hideTakeOffAirConfirmation();
});

takeOffAirDialog.addEventListener('cancel', (event) => {
  event.preventDefault();
  if (!takeOffAirPending) hideTakeOffAirConfirmation();
});

btnConfirmTakeOffAir.addEventListener('click', async () => {
  if (takeOffAirPending || coordinationCommandPending || !state.coordination?.broadcast || !state.liveTerminalId) return;
  takeOffAirPending = true;
  takeOffAirError.textContent = '';
  takeOffAirError.hidden = true;
  btnCancelTakeOffAir.disabled = true;
  btnConfirmTakeOffAir.disabled = true;
  const result = await runCoordinationCommand(
    () => desktopAPI.requestTerminalClear(),
    'АКТИВНЫЙ ТЕРМИНАЛ УБРАН · ТРАНСЛЯЦИЯ ПРОДОЛЖАЕТСЯ',
    'ОЧИСТКА АКТИВНОГО ТЕРМИНАЛА...',
    'broadcast'
  );
  takeOffAirPending = false;
  if (!result?.ok) {
    renderOverseerError(takeOffAirError, result, 'broadcast');
    btnCancelTakeOffAir.disabled = false;
    btnConfirmTakeOffAir.disabled = false;
    btnConfirmTakeOffAir.focus();
    return;
  }
  if (result.status === 'decision-required' && result.switchId) {
    hideTakeOffAirConfirmation({ restoreFocus: false });
    showTerminalSwitchDecision(result);
    return;
  }
  if (result.status !== 'cleared') {
    takeOffAirError.textContent = 'СНЯТИЕ С ЭФИРА НЕ ПОДТВЕРЖДЕНО';
    takeOffAirError.hidden = false;
    btnCancelTakeOffAir.disabled = false;
    btnConfirmTakeOffAir.disabled = false;
    btnConfirmTakeOffAir.focus();
    return;
  }
  state.liveHack = null;
  hideTakeOffAirConfirmation({ restoreFocus: false });
  renderTermList();
  renderTreeHeader();
  renderHackStatus();
  if (!btnEndBroadcast.hidden) btnEndBroadcast.focus();
});

for (const button of terminalSwitchButtons) {
  button.addEventListener('click', async () => {
    if (!pendingTerminalSwitch || coordinationCommandPending) return;
    const decision = button.dataset.switchDecision;
    terminalSwitchButtons.forEach(control => { control.disabled = true; });
    terminalSwitchStatus.textContent = 'ПРИМЕНЕНИЕ РЕШЕНИЯ...';
    terminalSwitchError.hidden = true;
    const result = await runCoordinationCommand(
      () => desktopAPI.resolveTerminalSwitch({ switchId: pendingTerminalSwitch, decision }),
      decision === 'cancel' ? 'ПЕРЕКЛЮЧЕНИЕ ОТМЕНЕНО' : 'РЕШЕНИЕ ПРИМЕНЕНО',
      'ПРИМЕНЕНИЕ РЕШЕНИЯ...',
      'broadcast'
    );
    terminalSwitchButtons.forEach(control => { control.disabled = false; });
    if (!result?.ok) {
      renderOverseerError(terminalSwitchError, result, 'broadcast');
      terminalSwitchStatus.textContent = 'ИСХОДНЫЙ ТЕРМИНАЛ ОСТАЁТСЯ АКТИВНЫМ';
      return;
    }
    if (result.status === 'activated' || result.status === 'cleared') state.liveHack = null;
    hideTerminalSwitchDecision();
    renderAll();
  });
}

btnHackSuccess.addEventListener('click', () => {
  if (!state.liveHack || state.liveHack.solved || state.liveHack.failed) return;
  desktopAPI.forceHackSuccess();
});

btnResetFailedHack.addEventListener('click', async () => {
  const term = state.session && state.liveTerminalId
    ? state.session.terminals.find(candidate => candidate.id === state.liveTerminalId)
    : null;
  if (!term || !state.liveHack?.failed || coordinationCommandPending) return;
  const result = await runCoordinationCommand(
    () => desktopAPI.resetFailedHack({
      terminalId: term.id,
      terminalName: term.name,
      tree: term.root,
      hackLevel: term.hackLevel || 0,
      introText: term.introText || '',
    }),
    'СОЗДАНА НОВАЯ ГОЛОВОЛОМКА',
    'ПОДГОТОВКА НОВОЙ ГОЛОВОЛОМКИ...',
    'broadcast'
  );
  if (!result?.ok) renderHackStatus();
});
