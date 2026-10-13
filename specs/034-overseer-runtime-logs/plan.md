# Implementation Plan: Overseer Runtime Logs

**Feature**: [spec.md](./spec.md)
**Date**: 2026-09-03
**Size**: Normal

## Summary

Extend the existing application logger with a bounded, per-user retained destination while preserving error-stream output and one production logger initialization. Generate safe audit facts inside the serialized coordination transitions that own player presence, roles, command approval, and hacking state, carry those facts through the existing effect router, and format them at the root logging boundary. Add one private Overseer action that opens the app-owned log directory and reports the active file path, then verify the behavior through deterministic Go tests, generated private bindings, browser coverage, target-aware path checks, and optional matching-host package smoke evidence.

## Project Structure

```text
.
├── main.go                                      # Compose retained logging once and attach the run identity
├── app.go                                       # Format safe audit records and implement log-location access
├── app_contract.go                              # Route the private log-access result through protobuf semantics
├── app_contract_test.go                         # Verify the private result contract and descriptor
├── app_test.go                                  # Verify audit formatting, redaction, access, and fallback behavior
├── desktop_service.go                           # Expose the one narrow private OpenLogLocation method
├── internal
│   ├── control
│   │   ├── service.go                           # Produce ordered safe audit facts with canonical transitions
│   │   └── service_test.go                      # Cover connection, role, command, and hacking audit events
│   ├── diagnostics
│   │   ├── retained_log.go                      # Own run identity, files, rotation, retention, and fallback writer
│   │   └── retained_log_test.go                 # Exercise lifecycle, bounds, failures, ordering, and permissions
│   ├── gen/fallout/terminal/private/v1
│   │   └── desktop.pb.go                        # Generated private result contract
│   └── platform
│       ├── desktop.go                            # Open the fixed app-owned log directory through Wails
│       ├── desktop_test.go                       # Verify allowlisting, context, and native-open failures
│       ├── paths.go                              # Resolve the log directory below application support
│       └── paths_test.go                         # Verify every supported operating-system mapping
├── proto/fallout/terminal/private/v1
│   └── desktop.proto                             # Define the private log-access result
├── frontend/overseer
│   ├── bindings/github.com/obalunenko/Fallout-Terminal/v2/
│   │   ├── desktopservice.js                     # Generated Wails method binding
│   │   └── models.js                             # Generated Wails result model
│   └── src
│       ├── desktop-api.js                        # Normalize the new private command
│       ├── index.html                            # Offer log access during startup and normal operation
│       └── overseer.js                           # Invoke access and show success or actionable failure
├── tests/browser
│   └── overseer-runtime-logs.spec.mjs            # Exercise both visible access points and feedback
├── scripts
│   └── runtime-logs-macos-smoke.sh               # Optional matching-host packaged access evidence
└── Taskfile.yml                                  # Expose the optional package smoke through the canonical task graph
```

**Structure Decision**: Keep storage and rotation in a framework-independent diagnostics package, path and native-open behavior in the platform adapter, authoritative event detection in the coordinator, and formatting plus private desktop access in root composition; the browser player receives no new capability or data.

## Constitution Check

| Principle | Assessment | Evidence |
|---|---|---|
| I. Govern the Accepted Desktop Runtime | PASS | Wails file opening remains inside `internal/platform/`; the root private service is the only frontend entrypoint, while control and diagnostics remain Wails-independent. |
| II. Make Protobuf the Application Contract Source of Truth | PASS | The new desktop result is added to the private v1 protobuf graph and routed through `app_contract.go`; structured log lines remain process diagnostics, consistent with the completed application-logging design. |
| III. Use ConnectRPC and Keep State Server-Authoritative | PASS | Player, role, approval, and hacking facts are derived from accepted coordinator transitions and carry the same revision order; no browser-local mutation or new RPC is introduced. |
| IV. Separate Public and Private Capabilities | PASS | Opening logs is a fixed-path Overseer-only method on the registered desktop service. Public player schemas, handlers, routes, and assets expose neither filesystem access nor diagnostic records. |
| V. Evolve Schemas Safely and Reproducibly | PASS | The private contract change is additive, generated artifacts remain generated, and Buf plus binding drift checks cover the update. |
| VI. Preserve Portable Session JSON Version 1 | PASS | Logs live below per-user application support and do not enter, relocate, or alter session or player-configuration JSON. |
| VII. Complete Cutovers and Remove Superseded Protocols | PASS | The existing logger remains the sole application logger; retained output extends its writer instead of creating a parallel logging protocol. |
| Secret and Credential Governance | PASS | The audit vocabulary permits only stable identifiers, roles, categories, revisions, and counts; names, content, credentials, raw errors, puzzle targets, and solutions are excluded and tested with canaries. |
| Testing and Quality Gates | PASS | The plan includes deterministic filesystem and coordinator tests, protobuf and binding checks, browser coverage, race validation, target-aware package/path checks, and honest optional native evidence. |

**Post-design recheck**: PASS. The data model and contracts preserve the same boundaries and introduce no constitutional violation.

## Delivery Phases

1. Establish retained-log path resolution and the concurrency-safe bounded writer, then compose it as the existing logger's destination with an application-run field and error-stream fallback.
2. Add safe audit events to the coordinator transaction/effect path and format them at the root logger boundary without exposing authored content or secrets.
3. Add the private protobuf-governed log-access result, platform open operation, desktop facade method, generated bindings, and always-available Overseer controls.
4. Complete redaction, rotation, cross-platform path, browser, race, build, and optional matching-host packaged verification.
