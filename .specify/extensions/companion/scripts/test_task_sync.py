#!/usr/bin/env python3
"""Regression tests for Companion convergence reopening and task folding."""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from spec_context import read_ctx
from task_sync import append_task_log, materialize_log, reopen_convergence


class ConvergenceReopenTest(unittest.TestCase):
    def make_feature(self, root: Path, *, status: str = "completed", pending: bool = True) -> Path:
        feature = root / "specs" / "010-example"
        feature.mkdir(parents=True)
        task_state = " " if pending else "x"
        (feature / "tasks.md").write_text(
            f"- [x] T001 Initial task\n- [{task_state}] T002 Convergence task\n",
            encoding="utf-8",
        )
        (feature / ".spec-context.json").write_text(
            json.dumps({
                "workflow": "speckit",
                "specName": "Example",
                "branch": "010-example",
                "currentStep": "implement",
                "currentTask": "T001",
                "status": status,
                "history": [
                    {"step": "implement", "substep": None, "kind": "start", "by": "extension", "at": "2026-09-03T00:00:00.000Z"},
                    {"step": "implement", "substep": None, "task": "T001", "kind": "complete", "by": "ai", "at": "2026-09-03T00:01:00.000Z"},
                    {"step": "implement", "substep": None, "kind": "complete", "by": "extension", "at": "2026-09-03T00:02:00.000Z"},
                ],
            }),
            encoding="utf-8",
        )
        return feature

    def test_reopen_and_materialize_close_numbered_convergence_round(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            feature = self.make_feature(Path(temp))

            self.assertEqual(feature / ".spec-context.json", reopen_convergence(feature, "extension"))
            self.assertEqual(feature / ".spec-context.json", reopen_convergence(feature, "extension"))
            reopened = read_ctx(feature / ".spec-context.json")
            self.assertEqual("implementing", reopened["status"])
            self.assertEqual("T002", reopened["currentTask"])
            starts = [
                event for event in reopened["history"]
                if event.get("substep") == "convergence-1" and event.get("kind") == "start"
            ]
            self.assertEqual(1, len(starts))

            self.assertIsNotNone(append_task_log(feature, "T002", "ai", "Finished convergence", ["app.go"]))
            self.assertEqual(feature / ".spec-context.json", materialize_log(feature, "ai", quiet=True))
            completed = read_ctx(feature / ".spec-context.json")
            self.assertEqual("implemented", completed["status"])
            finishes = [
                event for event in completed["history"]
                if event.get("substep") == "convergence-1" and event.get("kind") == "complete"
            ]
            self.assertEqual(1, len(finishes))
            self.assertIn("- [x] T002", (feature / "tasks.md").read_text(encoding="utf-8"))

    def test_reopen_refuses_fully_checked_or_archived_spec(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            complete = self.make_feature(root / "complete", pending=False)
            archived = self.make_feature(root / "archived", status="archived")

            self.assertIsNone(reopen_convergence(complete, "extension"))
            self.assertIsNone(reopen_convergence(archived, "extension"))
            self.assertEqual("completed", read_ctx(complete / ".spec-context.json")["status"])
            self.assertEqual("archived", read_ctx(archived / ".spec-context.json")["status"])


if __name__ == "__main__":
    unittest.main()
