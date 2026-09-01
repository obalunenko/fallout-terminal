# T174 — Final frontend reproducibility

Date: 2026-09-01 (Asia/Tbilisi)

Result: PASS.

- Self-test: two Vite trees matched and a deliberate copied-tree mismatch failed actionably with the changed asset identified.
- `task frontend:reproducible:check`: both production applications built twice with identical sorted relative paths, modes, sizes, and SHA-256 values.
- Overseer tree manifest: `ce63d205fbbbd7cbdf2d95f0e33f059fd7f1bbb757718244916050cccd07f8aa`.
- Player tree manifest: `09f77ad14d77c12592766a8b788b9bc0a7b21ccb0d42c1a50e33c9b814a3dc7f`.
- The governed scratch trees were removed by the command, and `frontend/package-lock.json` was not modified.
