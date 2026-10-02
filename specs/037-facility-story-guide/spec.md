# Feature Specification: Facility Story Guide

**Created**: 2026-09-05
**Input**: The Overseer still does not explain what to do or how to configure a narrative; provide instructions, steps, and meaningful grouping.

## User Scenarios & Testing

### User Story 1 - Build a story in a guided order (Priority: P1)

As an Overseer preparing a session, I can follow a short, in-application route from a story object to an action players can request and the information they will see.

**Why this priority**: A usable form is insufficient when the operator does not know which forms to use or in what order.

**Independent Test**: Follow the guide for a door with closed/open states, an opening action, a player command, and a changed status message.

**Acceptance Scenarios**:
1. **Given** an empty or existing facility, **When** I open its workspace, **Then** I can discover an ordered setup guide without leaving the application.
2. **Given** a guide step, **When** I read it, **Then** I see its story purpose, a concrete example, and an action opening the relevant existing editor.
3. **Given** a missing prerequisite such as a device or transition, **When** I reach a dependent step, **Then** it explains what to create first and offers that action.
4. **Given** existing content, **When** I select a device or return from a form, **Then** guidance uses that device and the current draft rather than assuming a blank session.

### User Story 2 - Understand how rules fit together (Priority: P1)

As an Overseer, I can distinguish what changes the world, what players see, and what optional faults or recovery add to the story.

**Why this priority**: Technical collections do not explain cause and effect or which parts are optional.

**Independent Test**: Inspect a populated device and distinguish its command actions, display rules, faults, and recovery programs without consulting repository documentation.

**Acceptance Scenarios**:
1. **Given** mixed relationships, **When** I inspect a device, **Then** actions and displayed information appear in named groups with short explanations.
2. **Given** a simple story, **When** I encounter faults and recovery, **Then** they are marked optional and explained with the same running example.
3. **Given** a configuration form, **When** I enter states, transitions, prerequisites, bindings, or recovery, **Then** nearby instructions explain their role and how to connect the result to a player command.

### User Story 3 - Know what remains and test safely (Priority: P2)

As an Overseer, I can inspect what is configured, save deliberately, and preview before trying the story with players.

**Why this priority**: A checklist should help preparation without claiming that a story is correct merely because some fields exist.

**Independent Test**: Stage changes, inspect the final step, save once, preview, and navigate the guide using a narrow window and keyboard.

**Acceptance Scenarios**:
1. **Given** draft changes, **When** I reach the final step, **Then** it explains apply versus save and offers explicit save before preview.
2. **Given** saved configuration, **When** I inspect the final step, **Then** it offers preview and explains that actual player commands require a terminal command and Overseer approval.
3. **Given** a small window or keyboard use, **When** I navigate or collapse the guide, **Then** its controls remain reachable and existing authoring controls remain usable.

## Edge Cases

- No devices, a selected device with no transitions, no suitable terminal commands, no display rules.
- Existing demo sessions and selection changes; staged additions and discard.
- Optional faults/programs absent; never label these as errors or fabricate a readiness score.
- Dirty or stale drafts; guidance must use existing save conflict protection.
- Long Russian names and 800×700 windows; guidance must not consume the fixed workspace height.

## Requirements

### Functional Requirements

- **FR-001**: The facility workspace MUST expose a collapsible, ordered setup guide for objects/states, actions/commands, player information, optional complications/recovery, and save/preview.
- **FR-002**: Every step MUST explain its narrative purpose and the same concrete door example in Russian, with an action leading to the relevant configuration surface.
- **FR-003**: Guide actions MUST use the selected device and explain missing prerequisites with a usable route to create them.
- **FR-004**: The guide MUST reflect current draft data after form application, device selection, save, and discard; counts MUST describe configured data rather than certify narrative correctness.
- **FR-005**: Relationship lists MUST distinguish command actions from player-visible rules with readable group explanations and preserve edit/remove access.
- **FR-006**: Fault and recovery sections MUST explain their optional role; the command-action form MUST allow selecting an existing recovery program, and configuration forms MUST provide contextual examples for states, transitions, prerequisites, bindings, and recovery.
- **FR-007**: Guide navigation MUST cause no durable writes; publication MUST continue through an explicitly labeled save with existing conflict protection, and preview MUST remain detached.
- **FR-008**: The guide MUST be usable with keyboard controls and at 800×700 and 1440×900 without horizontal page overflow or hiding existing authoring actions.

## Key Entities

- **Story object**: An existing device whose states describe a change in the scene.
- **Story action**: An existing transition connected to a player command; prerequisites explain when it can occur.
- **Player information**: Existing display rules showing the consequence of a state.
- **Setup step**: Temporary navigation and guidance derived from the currently authored configuration.

## Success Criteria

- **SC-001**: A user can reach device, command-action, display-rule, optional-fault, recovery, save, and preview controls from the guide.
- **SC-002**: The continuous door example explains an object, two states, an action, its player command, visible consequence, and optional complication/recovery.
- **SC-003**: Navigating the guide and applying forms performs zero durable writes until one explicit save; existing conflict guards remain effective.
- **SC-004**: Existing and empty configurations pass relevant browser checks; both target window sizes pass visual inspection and frontend builds pass.

## Assumptions

- Grouping is by narrative purpose in the UI, not new persisted scene/location entities.
- Reuse feature 036 editing, draft, preview, and private authority boundaries; preserve its uncommitted implementation.
- Keep instructions in the private Russian Overseer UI; do not add tutorial copy to player content.
- No new persistence fields, transport, dependencies, or automatic scenario creation.

## Approach

- Extend `frontend/overseer/src/index.html`, `overseer.css`, and `overseer.js` with a collapsible guide inside the scrollable detail panel; use native buttons and one current-step panel.
- Derive prerequisite feedback and configuration counts from the existing selected device and facility draft. Route guide actions to existing dialogs and the explicit save/preview boundary. No second draft store or persisted completion flags.
- Group the existing binding catalog into command actions and player information while retaining its containing element and existing controls. Explain optional sections and add contextual form copy.
- Extend `tests/browser/facility-authoring-ux.spec.mjs` for empty/existing guide journeys, draft effects, keyboard and responsive capture; update `sessions/demo-capability-paths.md` with the narrative route.
- Run Taskfile focused browser tests and frontend build under `.nvmrc`; inspect desktop/narrow screenshots and record actual results in `validation.md`. Reuse the previous repository-wide checks where no new affected backend surface exists.
- Constitution I–IX: existing private desktop transport, generated contracts, server authority, private capability boundary, unchanged schema, portable session preservation, one active authoring path, unchanged audit boundary, and private-only demo guidance remain intact.
