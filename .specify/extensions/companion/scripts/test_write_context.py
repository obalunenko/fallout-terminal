#!/usr/bin/env python3
"""Focused regression tests for guarded Companion bugfix reopen transitions."""

from __future__ import annotations

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("write-context.py")
SPEC = importlib.util.spec_from_file_location("write_context", SCRIPT)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError(f"could not load {SCRIPT}")
write_context = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(write_context)


class BugfixReopenTest(unittest.TestCase):
    def make_feature(self, root: Path, *, patched: bool = True) -> Path:
        feature = root / "specs" / "020-example"
        (feature / "bugs").mkdir(parents=True)
        status = "Patched" if patched else "Open"
        (feature / "bugs" / "BUG-002.md").write_text(
            f"# Bug Report\n\n**Status**: {status}\n**Patched**: 2026-08-25\n",
            encoding="utf-8",
        )
        (feature / "tasks.md").write_text(
            "- [x] **T001** Existing work\n"
            "- [ ] **T002** Reopened work (reopened — BUG-002)\n"
            "## Phase 2: BUG-002 — Repair\n"
            "- [ ] **T003** New repair work\n",
            encoding="utf-8",
        )
        history = [
            {
                "step": "implement", "substep": None, "task": "T001",
                "kind": "complete", "by": "ai", "at": "2026-08-25T00:00:00.000Z",
            },
            {
                "step": "implement", "substep": None, "kind": "complete",
                "by": "extension", "at": "2026-08-25T00:00:01.000Z",
            },
        ]
        (feature / ".spec-context.json").write_text(
            json.dumps({"status": "completed", "currentStep": "implement", "history": history}),
            encoding="utf-8",
        )
        return feature

    def test_reopen_journal_materialize_and_complete(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            feature = self.make_feature(Path(temp))
            original = write_context.read_ctx(feature / ".spec-context.json")["history"]

            target = write_context.reopen_for_bugfix(feature, "BUG-002", "ai")

            self.assertEqual(feature / ".spec-context.json", target)
            reopened = write_context.read_ctx(target)
            self.assertEqual("implementing", reopened["status"])
            self.assertEqual("T002", reopened["currentTask"])
            self.assertEqual(original, reopened["history"][:len(original)])
            self.assertEqual("bugfix-BUG-002", reopened["history"][-1]["substep"])
            self.assertEqual(["T002", "T003"], reopened["history"][-1]["tasks"])

            for task_id in ("T002", "T003"):
                write_context.append_task_log(feature, task_id, "ai")
                write_context.materialize_log(feature, "ai", quiet=True)
            write_context.mark_spec_complete(feature, "ai")

            completed = write_context.read_ctx(target)
            self.assertEqual("completed", completed["status"])
            self.assertIsNone(write_context._open_bugfix_substep(completed["history"]))
            self.assertNotIn("[ ]", (feature / "tasks.md").read_text(encoding="utf-8"))

    def test_reopen_requires_a_patched_report(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            feature = self.make_feature(Path(temp), patched=False)

            self.assertIsNone(write_context.reopen_for_bugfix(feature, "BUG-002", "ai"))
            self.assertEqual(
                "completed",
                write_context.read_ctx(feature / ".spec-context.json")["status"],
            )

    def test_open_substep_pairs_repeated_bugfix_epochs_in_order(self) -> None:
        history = [
            {"step": "implement", "substep": "bugfix-BUG-002", "kind": "start"},
            {"step": "implement", "substep": "bugfix-BUG-002", "kind": "complete"},
            {"step": "implement", "substep": "bugfix-BUG-002", "kind": "start"},
        ]

        self.assertEqual(
            "bugfix-BUG-002",
            write_context._open_bugfix_substep(history),
        )


if __name__ == "__main__":
    unittest.main()
