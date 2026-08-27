# Data Model: Go Code Quality Refactor

No persisted or transport entity changes. The following in-memory concepts are clarified or reshaped.

## Detached Content Node

Represents a deep process-local copy of authored terminal content.

- Copies scalar identity, type, text, and presentation fields by value.
- Copies state-change and terminal-transition configurations when present.
- Copies raw extension bytes and recursively copies children.
- Preserves nil versus allocated-empty collections.
- Shares no mutable nested storage with its source.

## Control Failure Identity

Represents a stable category attached to a human-readable error.

- **Stale decision**: the pending command no longer matches the request or current authored state.
- **Persistence failure**: durable execution is unavailable, fails, or returns state that cannot prove completion.
- Identities survive one or more wrapping layers.
- Application-facing copy remains intentionally generic and does not expose internal causes.

## Tunnel Lifecycle Iteration

Represents one pass through lifecycle ownership checks.

- Inputs: caller context, expected revision, and for reconfiguration an owned secret mutation buffer.
- Wait states: another reconfiguration, cleanup, or stop operation owns the transition.
- Terminal states: conflict, canceled/timed out, joined result, or ownership acquired.
- A completed wait returns to the ownership checks without adding a call frame.
- Owned secret buffers are cleared exactly once when the outer operation ends.

## Player Action Transaction Context

Represents values resolved once inside the existing atomic commit.

- Connection and logical session identity.
- Request fingerprint and cached-result status.
- Active broadcast and terminal eligibility.
- Selected authored command or return route.
- Accepted or rejected result plus ordered effects.

The context does not escape the transaction and does not become persisted state.

## Character Value Rules

- Display name is trimmed, required, and limited to the canonical character count.
- Intelligence remains within the canonical inclusive range.
- Boundary-specific presence, identity, revision, active-session, and authorization checks remain outside these reusable value rules.

## Extracted Manifest Validation Context

- Selected candidate identity and target.
- Parsed manifest identity and ordered file records.
- Actual extracted regular-file evidence keyed according to target case rules.
- Required application package shape.

Validation proceeds from root safety to manifest availability, decoding, identity, inventory shape, and exact file evidence. No partially validated result is exposed.
