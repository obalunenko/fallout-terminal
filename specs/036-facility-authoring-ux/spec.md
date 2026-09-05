# Feature Specification: Facility Authoring UX

**Created**: 2026-09-05

**Status**: Specified
**Input**: Implement all seven findings from the screenshot/source review of feature 035 through Spec Kit.

## User Scenarios & Testing

### User Story 1 - Edit safely and save predictably (Priority: P1)

As an Overseer, I can revise a device and its rules without losing existing relationships, and know exactly when changes reach my session.

**Why this priority**: Accidental reference loss or unexpected publication undermines every authoring task.

**Independent Test**: Open a multi-transition device, rename a state, add a transition, stage edits to another entity, discard once, then save and reopen.

**Acceptance Scenarios**:
1. **Given** transitions with prerequisites and fault effects, **When** I open or edit states or add transitions, **Then** existing references and unrelated authored attributes remain intact.
2. **Given** an incomplete reference row, **When** I apply the form, **Then** the relevant control receives an actionable validation error and the row is not silently dropped.
3. **Given** edits in device, binding, or recovery forms, **When** I apply a form, **Then** it updates a clearly identified draft without saving the session.
4. **Given** a changed draft, **When** I save the session changes, **Then** all staged edits save together once; a failure retains the draft for correction.
5. **Given** a changed draft, **When** I discard it, **Then** the saved configuration is restored without changing live facility state.

### User Story 2 - Inspect the facility during play (Priority: P1)

As an Overseer, I can find a device, read its current state and faults, and reach its controls without losing my place.

**Why this priority**: The feature must work while running a tabletop session.

**Independent Test**: Use the full demo at desktop and narrow sizes, search for a device, select it, and inspect its status and related rules.

**Acceptance Scenarios**:
1. **Given** many devices and bindings, **When** I select a device, **Then** its details appear beside the list on desktop without jumping to a distant name field.
2. **Given** named states and faults, **When** I inspect the overview, **Then** readable names, current versus initial state, active versus inactive faults, and affected terminals are distinguishable.
3. **Given** a reset or recovery action, **When** I inspect its confirmation, **Then** the affected device or facility and consequence are explicit.

### User Story 3 - Follow and edit relationships (Priority: P1)

As an Overseer, I can understand and maintain rules from their list, dependencies, or the relevant command.

**Why this priority**: Adding rules without a reliable correction path is insufficient authoring.

**Independent Test**: Edit and remove a binding and recovery program, follow node/block/transition references, and edit a command's multi-device action from command properties.

**Acceptance Scenarios**:
1. **Given** a dependency, **When** I inspect it, **Then** readable entity names and purpose appear, technical details are secondary, and Open reaches the actual target or explains a missing target.
2. **Given** an existing binding or recovery program, **When** I edit or remove it, **Then** the draft reflects the requested change without duplicate rules or loss of unrelated values.
3. **Given** a facility-backed command, **When** I select it in the terminal editor, **Then** I can inspect and edit device source/destination changes and prerequisites there.

### User Story 4 - Understand transitions and preview consequences (Priority: P2)

As an Overseer, I can read a transition as an action and see how its hypothetical state affects terminal content.

**Why this priority**: Clear explanation reduces preparation effort once edits are trustworthy.

**Independent Test**: Expand a transition's advanced rules, select an explicit fault effect, and preview two states with different content and command availability.

**Acceptance Scenarios**:
1. **Given** a transition, **When** I inspect it, **Then** its name and source-to-destination summary are readable, optional rules can be expanded, and fault activation/removal is explicit.
2. **Given** a long form, **When** I scroll, **Then** apply/cancel controls remain reachable and labels remain readable without horizontal overflow.
3. **Given** a preview, **When** I change its selections, **Then** its device/condition context, saved-data basis, detached nature, changed content, hidden items, and unavailable commands are clear.

## Edge Cases

- Deleted or renamed reference targets; duplicate state IDs; incomplete prerequisites and effects.
- A device has several transitions whose prerequisites reference other devices' states.
- Unsaved facility edits coexist with terminal edits or a newer saved session revision.
- Reset/recovery while a draft is dirty; failed publication; a new unsaved device cannot be previewed from saved data.
- Empty facility/search results; long Russian names; many bindings; narrow windows; keyboard navigation.
- Dependencies refer to a block, recovery program, or a device/transition composite identity.
- Preview selections change before an older response completes.

## Requirements

### Functional Requirements

- **FR-001**: Device edits MUST preserve existing references, current state, and unedited authored attributes.
- **FR-002**: Incomplete reference rows MUST produce focused validation errors instead of silent removal.
- **FR-003**: Every facility form MUST stage changes into one explicit draft, with one session-wide save action and a discard action.
- **FR-004**: Failed saves and intervening session updates MUST NOT silently erase or overwrite pending facility edits.
- **FR-005**: Selecting a device MUST show adjacent details on desktop and a usable responsive equivalent without a distant focus jump.
- **FR-006**: The workspace MUST support device search and show readable current/initial states, active fault status, and target names.
- **FR-007**: Reset and recovery controls MUST name their scope and explain their effect before confirmation.
- **FR-008**: Dependencies MUST explain relationships in readable language and navigate to nodes, blocks, transitions, conditions, and recovery programs or report a missing target.
- **FR-009**: Bindings and recovery programs MUST support editing and removal while preserving unrelated authored values.
- **FR-010**: Command properties MUST expose facility transition summaries, prerequisites, and contextual editing.
- **FR-011**: Transition forms MUST provide readable summaries, expandable advanced rules, explicit activate/remove-fault choices, and visible apply/cancel controls.
- **FR-012**: Preview MUST identify the selected entity, explain that it uses saved data without affecting players, and distinguish changed content, hidden items, and unavailable commands.
- **FR-013**: Technical identifiers MUST remain available as secondary details instead of replacing authored names in primary summaries.
- **FR-014**: The delivered workflows MUST pass browser regression checks for authoring preservation, save/discard, relationship editing/navigation, preview, keyboard access, and responsive layout.

## Key Entities

- **Facility draft**: The staged device definitions, faults, programs, and terminal rules awaiting one explicit save.
- **Device detail**: A selected device's readable state, initial value, transitions, faults, and references.
- **Relationship**: A binding, command action, prerequisite, fault scope, or recovery reference connecting authored entities.
- **Detached preview**: Hypothetical terminal content resolved from the saved facility with a selected state or fault override.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Opening, editing, saving, and reopening the multi-transition regression fixture retains every untouched reference and attribute.
- **SC-002**: Applying or discarding any facility form causes zero durable writes; saving the full draft causes one write.
- **SC-003**: At 1440×900, selected device details remain visible next to a searchable list even with the full demo's bindings; at 800×700 there is no horizontal page overflow.
- **SC-004**: Every supported dependency target in the regression fixture opens its authoring destination, and every binding/program can be edited and removed through the UI.
- **SC-005**: Preview tests show changed and unavailable content without any saved-state mutation or player publication.
- **SC-006**: All affected browser tests and frontend build checks pass, with validation evidence recorded in this feature directory.

## Assumptions

- This is a follow-up to implemented feature 035; its atomic persistence, private authority, player behavior, and portable version-1 model remain authoritative.
- Preserve the Russian operator UI and Fallout visual style. Use existing browser controls and authored names.
- Preview operates on saved configuration because that is the existing private preview contract; clearly label and guard unsaved previews.
- No new player capabilities, network contracts, persistence format, dependencies, release publication, or unrelated UI redesign are in scope.
- The branch starts at feature 035's tip, which contains three prerequisite commits absent from local develop; rebasing away those prerequisites would remove the feature being improved.
