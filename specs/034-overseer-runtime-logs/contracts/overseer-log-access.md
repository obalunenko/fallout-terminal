# Contract: Overseer Log Access

## Private desktop method

`OpenLogLocation()` accepts no browser-supplied path and returns a private `LogAccessResult`.

The private v1 protobuf definition is additive:

```proto
message LogAccessResult {
  bool ok = 1;
  optional string error = 2;
  string directory_path = 3;
  optional string active_log_path = 4;
}
```

The method always attempts to open the fixed per-user directory owned by the current application composition. It never accepts or resolves a frontend path, URL, filename, environment variable, or command. Success means the operating system accepted the open request, not that a separate file-manager process was observed. Failure returns a safe message plus `directory_path`; `active_log_path` remains available when retained output was created successfully.

## Frontend integration

- The generated Wails binding is wrapped as `desktopApi.openLogLocation()`.
- The startup screen and normal Overseer layout each expose a button with `data-action="open-log-location"` and visible text `ОТКРЫТЬ ЛОГИ`.
- Both buttons call the same command and are disabled while that invocation is pending.
- A shared status element identified by `data-log-access-status` reports success or failure with polite live-region semantics; failures also use alert semantics where the existing screen feedback pattern requires it.
- Success feedback identifies the active log filename and confirms that the directory was opened.
- Failure feedback includes the exact `directoryPath` returned by the private method so the Overseer can navigate manually.
- The action remains available when startup status is degraded; it does not depend on a session, player configuration, broadcast, public access, or update state.

## Boundary rules

- `frontend/client/` receives no binding, element, event, status field, or route related to retained logs.
- The method opens only the composition-provided app-owned directory through the platform adapter.
- The directory and active file paths are not stored in browser storage, session JSON, player configuration, named public events, or reusable runtime status.
- Native open errors are mapped to stable safe text before crossing the desktop boundary.
- Generated protobuf and Wails files are regenerated from their sources and are never edited manually.

## Verification contract

- Private descriptor checks find `LogAccessResult` with field numbers 1–4 and no public-player descriptor contains it.
- Desktop service tests prove the method forwards only to the root `App` and accepts no path argument.
- Platform tests prove the fixed path reaches `BrowserManager.OpenFile`, nil/not-ready managers fail safely, and HTTP `OpenURL` validation remains unchanged.
- Browser tests exercise both `data-action="open-log-location"` controls for success, failure, pending-state de-duplication, accessible feedback, and startup-degraded availability.
- Target-aware path tests prove each supported operating system resolves below application support and outside packaged resources.
