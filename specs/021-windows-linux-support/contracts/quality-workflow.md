# Contract: Pull-Request and Main Quality Workflow

## Trigger boundary

The non-release quality workflow runs for:

- pull-request open, synchronize, and reopen events; and
- pushes to `main`.

It does not run because a semantic-version tag was pushed and it does not invoke any job from the five-target release matrix.

## Required quality coverage

Every quality run uses the locked repository dependencies and covers:

1. `go test ./...` through the Task graph, including the startup and repository contract tests;
2. `go vet ./...` through the Task graph;
3. Buf/protobuf formatting and linting, deterministic generation drift, generated-code compilation, and the established breaking-change gate through the Task graph;
4. a clean locked install for `frontend/`;
5. a production build of `frontend/overseer/`;
6. a production build of `frontend/client/`;
7. exact Wails v3.0.0-beta.13 runtime, CLI, and frontend pin consistency; and
8. clean deterministic Wails v3 binding generation with no unexplained drift.

The workflow may run additional repository quality checks or build the current host when useful. It does not need to package all five targets and does not make native UI, dialog, credential-store, player, lifecycle, tunnel, or signing journeys mandatory. Those journeys are optional evidence; unexecuted checks are reported as `NOT RUN` and do not change archive support or quality status.

## Permissions and outputs

- Default GitHub token permission is `contents: read`.
- No job has `contents: write` or `packages: write`.
- The workflow never invokes GoReleaser, ORAS, `gh release`, a release API mutation, or `task package:all`.
- It creates no GitHub Release and publishes zero release assets.
- Logs and ordinary test reports are quality evidence, not distribution artifacts.

## Separation from tagged releases

Quality success is not an input to a later tagged release run. A qualifying tag receives its own clean checkout and runs only the minimal five-target release contract. Branch protection may require quality checks for merging, but a tag workflow does not call or wait for this workflow.

## Static verification

Repository acceptance tests prove that the workflow:

- has only pull-request and `main` push triggers;
- contains every required quality command;
- contains the governed Buf/protobuf check and breaking-change tasks;
- contains none of the five target matrix rows;
- grants no publication permission; and
- contains no release publisher, release upload, package-registry, or local Docker aggregate invocation.
