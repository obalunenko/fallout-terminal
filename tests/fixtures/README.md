# Test Fixtures

## `session-05-cold-storage.json`

This is a content-minimized equivalent of the authored campaign session used to
reproduce BUG-003 and BUG-004. It preserves the complete input projection used
by legacy terminal-group normalization and replacement validation:

- no persisted `terminalGroups` field;
- session version, name, terminal order, terminal IDs, terminal names, and hack
  levels;
- transition `svc-access-admin` named `ВХОД АДМИНИСТРАТОРА` from
  `t-krel-service` to `t-krel-admin`; and
- transition `adm-emergency` named `АВАРИЙНОЕ УПРАВЛЕНИЕ` from
  `t-krel-admin` to `t-krel-emergency`.

The unchanged source file inspected for BUG-004 has 19,710 bytes and SHA-256
`b4ca8b89b7d7af32e05a9b598a007e36a747ef59ce3e2bd15a60d0b3f0ec9438`.

The checked-in fixture intentionally differs only outside that projection:

- `playerConfig` is omitted so the fixture does not reference campaign files
  outside this repository;
- all three `introText` values are blank;
- non-transition folders, entries, ordinary commands, and state-changing
  commands are removed (the source contains 13, 17, and 9 total content nodes
  for the service, admin, and emergency terminals; the fixture contains 2, 2,
  and 1 respectively);
- the admin terminal's completed `adm-security-revoke` and
  `adm-security-return` command-state snapshots are omitted with their removed
  commands; and
- whitespace and object-key ordering are not retained.

Those fields are preserved by persistence tests elsewhere but are not read by
legacy singleton normalization, terminal-to-group indexing, authored-transition
edge enumeration, or complete-candidate classification. The exact source and
this fixture decode to the same terminal-group validation projection: three
ordered singleton inputs and the ordered edges
`svc-access-admin:t-krel-service->t-krel-admin` and
`adm-emergency:t-krel-admin->t-krel-emergency`. Focused domain tests compare
that projection directly when the exact authored source is available and run
the same partial/full candidate matrix for both inputs.
