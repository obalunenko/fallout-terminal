# Feature Specification: Interactive Approval Notifications

**Feature Directory**: `028-interactive-approval-notifications`

**Created**: 2026-08-28

**Status**: Draft

**Input**: Notify the Overseer through an interactive system notification whenever a player command requires approval, while preserving the current in-app approval flow.

## User Scenarios & Testing

### User Story 1 - Notice every approval request (Priority: P1)

As an Overseer, I receive a system notification when a player requests a command that needs approval, so I can notice the request even when the Overseer window is not focused.

**Why this priority**: A missed approval leaves every player waiting and blocks shared terminal activity.

**Independent Test**: Put the Overseer application in the background, request each supported kind of player command, and verify that one recognizable notification appears for each new pending request while the existing in-app approval remains available.

**Acceptance Scenarios**:

1. **Given** notifications are authorized and the Overseer window is not focused, **When** a player command becomes pending for approval, **Then** one system notification identifies the command and states that an Overseer decision is required.
2. **Given** notifications are authorized and the Overseer window is focused, **When** a player command becomes pending for approval, **Then** the system notification and the existing in-app approval prompt are both available.
3. **Given** the same pending request is delivered repeatedly while state synchronizes, **When** the Overseer receives those repeated updates, **Then** the application does not create additional notifications for that request.
4. **Given** a player requests an ordinary, state changing, previously completed, or terminal navigation command, **When** that request enters its existing approval lifecycle, **Then** it receives the same notification coverage.

---

### User Story 2 - Decide from the notification (Priority: P1)

As an Overseer, I can approve or reject the exact pending player command directly from the notification, so I can resolve the request without first returning to the application window.

**Why this priority**: Action buttons turn the alert into the requested interactive approval path and shorten the time players spend waiting.

**Independent Test**: Trigger a pending command, use each notification action in turn, and verify that the same authoritative outcomes appear for the Overseer and all players as when the corresponding in-app action is used.

**Acceptance Scenarios**:

1. **Given** a command is still pending, **When** the Overseer approves it from the notification, **Then** the exact request is approved through the existing decision rules and all current views reflect the approved outcome.
2. **Given** a command is still pending, **When** the Overseer rejects it from the notification, **Then** the exact request is rejected through the existing decision rules and all current views reflect the rejected outcome.
3. **Given** a notification action is being processed, **When** the Overseer also views the in-app prompt, **Then** both surfaces prevent a second effective decision and converge on the authoritative result.
4. **Given** a notification refers to a request that is no longer pending, **When** its action is delivered late or repeated, **Then** no other request is affected and no command effect is repeated.

---

### User Story 3 - Retain a dependable in-app fallback (Priority: P2)

As an Overseer, I can continue using the existing approval prompt when system notifications are denied, unavailable, or fail, so notification support never prevents play or weakens decision safety.

**Why this priority**: System notification availability varies by user choice and desktop environment, but command approval must remain reliable everywhere the application already works.

**Independent Test**: Deny permission or simulate notification delivery and response failures, then approve and reject commands through the existing prompt without restarting the player flow.

**Acceptance Scenarios**:

1. **Given** the operating system requires notification permission, **When** permission has not been granted, **Then** the application follows the system consent process without repeatedly pressuring the Overseer.
2. **Given** notification permission is denied or notification delivery is unavailable, **When** a command requires approval, **Then** the existing in-app prompt remains fully usable and the request remains pending until a valid decision is made.
3. **Given** an approval action from a notification cannot be completed, **When** the failure is known, **Then** the application does not imply success, preserves the authoritative pending state, and keeps the in-app decision path available.
4. **Given** a request is resolved through the in-app prompt, **When** its earlier system notification remains visible because of platform limitations, **Then** any later interaction with that notification is treated as stale and cannot change the resolved outcome.

### Edge Cases

- Notification authorization is undecided, denied, revoked, or changes while a request is pending.
- A compatible system notification service is absent or stops responding.
- The same pending request is published more than once during reconnect or state resynchronization.
- A notification response arrives after rejection, approval, broadcast end, terminal switch, application shutdown, or replacement by another request.
- The Overseer acts from the notification and the in-app prompt at nearly the same time.
- Delivery succeeds but the platform cannot remove or update an already delivered notification.
- Command names or confirmation text are empty, unusually long, multiline, or contain non-Latin characters.
- The application restarts while a player request was pending and the current coordination state is restored.

## Requirements

### Functional Requirements

- **FR-001**: The application MUST issue an interactive system notification whenever a new player command becomes the current pending approval request.
- **FR-002**: Notification coverage MUST include every existing player command approval mode, including terminal navigation commands that use the separate navigation approval state.
- **FR-003**: Each notification MUST identify the requested command and present the confirmation message already used by the in-app approval prompt.
- **FR-004**: Each notification MUST expose no private session or player data beyond the command information already available in the in-app approval prompt.
- **FR-005**: Each interactive approval notification MUST offer actions to approve and reject the request.
- **FR-006**: A notification action MUST resolve the exact request represented by that notification through the same authoritative validation and decision rules as the corresponding in-app action.
- **FR-007**: Only the current pending request MUST be eligible for a notification decision; stale, repeated, malformed, or mismatched responses MUST NOT affect another request or repeat a command effect.
- **FR-008**: The application MUST emit at most one newly delivered notification for each unique pending request, including across repeated state updates.
- **FR-009**: The existing in-app approval prompt MUST remain available and synchronized with decisions made from system notifications.
- **FR-010**: Resolution through either surface MUST prevent a second effective decision for the same request.
- **FR-011**: A resolved request's notification MUST become non-actionable and MUST be removed or updated where the operating system permits.
- **FR-012**: The application MUST follow the operating system's notification authorization process, respect denial or revocation, and avoid repeated unsolicited permission prompts.
- **FR-013**: Notification denial, unsupported notification behavior, delivery failure, or response failure MUST NOT cancel, approve, reject, or otherwise alter the pending command by itself.
- **FR-014**: When a notification action fails, the application MUST avoid reporting a successful decision and MUST preserve the existing in-app path to the authoritative request state.
- **FR-015**: Notification responses MUST remain confined to the trusted Overseer application boundary and MUST NOT add any player-facing or remote approval capability.
- **FR-016**: After broadcast end, terminal switch, request replacement, or application shutdown invalidates a request, any later response from its notification MUST be rejected as stale.

## Key Entities

- **Approval request**: The one current, server-owned player request awaiting an Overseer decision. It carries a unique request identity, command identity, command name, confirmation message, approval mode, and broadcast context.
- **Interactive approval notification**: The system-visible representation of one approval request, with approve and reject actions and a stable relationship to that request.
- **Approval decision**: An Overseer's approve or reject choice, accepted only for the exact request that is still current.
- **Notification authorization**: The operating system's current permission for the application to show notifications.

## Success Criteria

### Measurable Outcomes

- **SC-001**: On every supported desktop platform with notifications authorized, 100% of newly pending command requests produce exactly one notification within one second of the application receiving the pending state.
- **SC-002**: Approval and rejection from a notification produce the same observable Overseer and player outcomes as the corresponding in-app actions for every existing command approval mode.
- **SC-003**: In a test of at least 100 repeated, delayed, stale, or concurrent notification responses, no request receives more than one effective decision and no command effect is repeated.
- **SC-004**: In every denied, revoked, unavailable, delivery-failure, and response-failure scenario, the in-app approval flow remains usable and the pending request changes only after a valid Overseer decision.
- **SC-005**: Every emitted notification contains only the command identification and confirmation information already available in the existing Overseer approval prompt.
- **SC-006**: Platform acceptance checks confirm notification delivery, both decision actions, permission handling, stale-response safety, and in-app fallback on every supported desktop platform.

## Assumptions

- “Interactive notifications” means native system notifications with approve and reject actions.
- “Any command execution” includes all existing universal command approval modes and command-driven terminal navigation, which currently has a separate pending approval representation.
- The existing in-app approval prompt remains the canonical fallback and is not redesigned by this feature.
- Notification content uses the command name and confirmation message already shown to the Overseer; no player identity, session content, credentials, or command result is added.
- The operating system account and its notification access are treated as part of the trusted local Overseer environment; this feature does not add a second authentication step.
- Delivered-notification cleanup may degrade by platform, but request validation and stale-response safety are required consistently.

## Out of Scope

- Changing command execution, persistence, player presentation, or terminal navigation semantics.
- Replacing or removing the existing in-app approval prompt.
- Adding notification preferences, custom sounds, attachments, scheduling, or text replies.
- Adding approval controls to player-facing web surfaces or remote APIs.
