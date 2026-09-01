# Valid task-plan governance fixture

Prose may mention `! rg`; executable absence uses safe forms.

- [ ] **T001** Complete the governed fixture.
  - Outcome: exact fixture passes every applicable validator class and names frontend:build, frontend:build:client, frontend:build:overseer, frontend:boundary:check, frontend:compatibility:check, frontend:policy:check, frontend:reproducible:check, frontend:typecheck, frontend:typecheck:client, and frontend:typecheck:overseer. · Files: `fixture.md` · Read-only: `input.md` · Depends: none · Coverage: FR-001, FR-002, FR-003, FR-004, FR-005, FR-006, FR-007, FR-008, FR-009, FR-010, FR-011, FR-012, FR-013, FR-014, FR-015, FR-016, FR-017, FR-018, FR-019, FR-020, FR-021, FR-022, FR-023, FR-024, FR-025, FR-026, FR-027, FR-028, FR-029, FR-030, FR-031, FR-032, FR-033, FR-034, FR-035, FR-036, FR-037, FR-038, FR-039, FR-040, FR-041, FR-042, FR-043, FR-044, FR-045, FR-046, FR-047, FR-048, FR-049, FR-050, FR-051; SC-001, SC-002, SC-003, SC-004, SC-005, SC-006, SC-007, SC-008, SC-009, SC-010, SC-011, SC-012; CHK001, CHK002, CHK003, CHK004, CHK005, CHK006, CHK007, CHK008, CHK009, CHK010, CHK011, CHK012, CHK013, CHK014, CHK015, CHK016, CHK017, CHK018, CHK019, CHK020, CHK021, CHK022, CHK023, CHK024, CHK025, CHK026, CHK027, CHK028, CHK029, CHK030, CHK031, CHK032, CHK033, CHK034, CHK035, CHK036, CHK037, CHK038, CHK039, CHK040 · Verify: safe governed helpers; local command: `test ! -e removed.js && scripts/frontend-assert-no-match.sh 'legacy' active.ts && scripts/frontend-focused-browser-check.sh tests/browser/example.spec.mjs 'literal suffix'` · Evidence: exact passing fixture · Temporary: all 18 rows closed · Go: not applicable.

| Overseer mechanism 1 | closed |
| Overseer mechanism 2 | closed |
| Overseer mechanism 3 | closed |
| Overseer mechanism 4 | closed |
| Overseer mechanism 5 | closed |
| Overseer mechanism 6 | closed |
| Overseer mechanism 7 | closed |
| Overseer mechanism 8 | closed |
| Player mechanism 1 | closed |
| Player mechanism 2 | closed |
| Player mechanism 3 | closed |
| Player mechanism 4 | closed |
| Player mechanism 5 | closed |
| Player mechanism 6 | closed |
| Player mechanism 7 | closed |
| Player mechanism 8 | closed |
| Player mechanism 9 | closed |
| Deliberate protobuf drift mutation | closed |
