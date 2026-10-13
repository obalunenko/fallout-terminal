# Research: Overseer Runtime Logs

## Decision 1: Extend the existing logger through one retained writer

**Decision**: Keep `github.com/obalunenko/logger` as the single production logger and initialize it once with a concurrency-safe writer that mirrors the existing text-format records to standard error and an app-owned retained log. The retained side creates a random application-run identifier, writes run-and-segment-named files below application support, rotates at 5 MiB, retains at most eight 5 MiB segments, and protects the newest segment of both the current and immediately previous run. One oversized record may temporarily exceed the nominal 40 MiB boundary; the next successful write or startup pruning restores the bound.

**Rationale**: The dependency already accepts an `io.WriteCloser`, already emits readable structured key/value records, and is required by the completed application-logging feature. A small standard-library writer adds retention without replacing the logger, changing every caller, or adding a new dependency. Run-specific filenames make current and previous evidence distinguishable after both clean and unexpected stops.

**Alternatives considered**:

- Replace the logger with another logging framework: rejected because the repository pins the current logger and its field vocabulary is already tested.
- Write only to a file in packaged mode: rejected because development parity and a fallback diagnostic channel are requirements.
- Keep one unbounded `application.log`: rejected because it cannot meet bounded-retention or previous-run requirements.
- Add an external rotation dependency: rejected because the needed bounded segment policy is small, deterministic, and achievable with the standard library.

## Decision 2: Produce audit facts in coordinator order and format them at the root

**Decision**: Add a closed, safe audit-event vocabulary to `internal/control` and carry audit events on the existing detached `Effect` path. The coordinator emits connection, disconnection, role-transition, pending-command, decision, and hacking facts in the same serialized transition that commits the authoritative revision; `coordinationEffectRouter` forwards each fact to `App`, which maps it to logger fields and severity.

**Rationale**: The coordinator is the only owner that can distinguish a logical session's first/final physical connection, effective role changes, accepted versus replayed requests, the exact pending approval identifier, and private puzzle-generation identity. Emitting detached facts avoids diffing incomplete public projections, while root formatting keeps logging policy and the concrete logger out of domain and control logic.

**Alternatives considered**:

- Log transport connections in the player handler: rejected because several physical streams can represent one logical session and would create misleading connect/disconnect duplicates.
- Diff only `MasterCoordinationState` and public hacking state in `App`: rejected because decision causes, rejected player actions, puzzle generation identity, and safe pattern outcomes are not all present in those projections.
- Inject the concrete logger into `internal/control`: rejected because it would mix diagnostic formatting and I/O policy into the server-authoritative state owner.

## Decision 3: Use a strict safe event vocabulary

**Decision**: Records use stable categories—`player.connected`, `player.role_changed`, `player.disconnected`, `command.request_received`, `command.request_outcome`, `command.decision`, `hack.started`, `hack.guess`, `hack.pattern`, `hack.succeeded`, `hack.failed`, `hack.reset`, and `hack.interrupted`. Allowed fields are limited to run identity, coordination revision, stable session/terminal/command/request/puzzle identifiers, current and previous role, decision, outcome/reason category, hacking level, and attempt counts. Puzzle targets, words, board coordinates, command names, confirmation text, results, character names, and raw dependency errors never enter an audit event.

**Rationale**: A closed vocabulary makes support records searchable and testable while allowing the Overseer to correlate exactly which safe command identifier and puzzle lifecycle failed. Creating the redaction boundary before formatting prevents an accidental secret from being carried to any log destination.

**Alternatives considered**:

- Log complete runtime commands or public state snapshots: rejected because they contain player-controlled or gameplay-bearing values and are unnecessarily large.
- Redact only at the final writer: rejected because sensitive values would already have crossed intermediate diagnostic boundaries.
- Log character display names for readability: rejected because the existing logging contract explicitly excludes character names.

## Decision 4: Open only the fixed private log location

**Decision**: Add an `OpenLogLocation` method to the existing private `desktopService`. `App` receives the intended log directory and a narrow opener, asks `internal/platform.Desktop` to open that fixed directory through Wails' `BrowserManager.OpenFile`, and returns a protobuf-governed result containing success, safe error text, directory path, and active log path. The frontend provides the same action on the startup screen and normal Overseer screen so diagnostics remain reachable when application startup is degraded.

**Rationale**: Opening one application-owned directory satisfies packaged access without accepting a browser-supplied path or exposing a generic filesystem capability. Returning the intended path on failure makes the UI actionable and testable, and the active filename identifies the current run after opening the directory.

**Alternatives considered**:

- Use the existing HTTP/HTTPS `OpenURL` path: rejected because file URLs are deliberately denied and broadening that allowlist would weaken the private boundary.
- Build an embedded searchable log viewer: rejected because the requested support mechanism is satisfied by opening retained logs and a viewer would add a substantially larger parsing, pagination, and redaction surface.
- Expose the log location to the public player client: rejected because filesystem access and operational diagnostics are Overseer-only capabilities.

## Decision 5: Verify portable behavior without overstating native evidence

**Decision**: Test all supported operating-system path mappings and package separation deterministically in Go, run current-host build/package checks, and provide a matching-host macOS packaged smoke that opens retained logs without a terminal. Native evidence for unavailable Windows or Linux hosts is reported as not run rather than becoming a feature-completion or release gate.

**Rationale**: This proves the portable contract while following the constitution's explicit distinction between supported package production and optional native UI evidence. It also keeps CI deterministic and credential-free.

**Alternatives considered**:

- Require native UI automation on all five release targets: rejected because the constitution prohibits making unavailable native evidence a completion or tagged-release gate.
- Verify only development runs: rejected because packaged accessibility is the central requirement.
- Put logs inside package resources so archive inspection can find them: rejected because installed packages may be read-only and upgrades must not erase runtime diagnostics.
