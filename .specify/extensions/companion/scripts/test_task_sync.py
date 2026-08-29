#!/usr/bin/env python3
"""Regression tests for Companion task journaling and materialization."""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

import task_sync


class TaskSyncTest(unittest.TestCase):
    def make_feature(
        self,
        root: Path,
        *,
        status: str,
        tasks: str,
        history: list[dict] | None = None,
    ) -> Path:
        feature = root / "specs" / "010-example"
        feature.mkdir(parents=True)
        (feature / "tasks.md").write_text(tasks, encoding="utf-8")
        (feature / ".spec-context.json").write_text(
            json.dumps(
                {
                    "workflow": "speckit",
                    "specName": "Example",
                    "branch": "010-example",
                    "currentStep": "implement",
                    "status": status,
                    "history": history or [],
                }
            ),
            encoding="utf-8",
        )
        return feature

    def test_materialize_checks_appended_tasks_and_closes_implement(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            feature = self.make_feature(
                Path(temp),
                status="implementing",
                tasks="- [ ] **T001** First\n- [ ] **T002** Second\n",
                history=[
                    {
                        "step": "implement",
                        "substep": None,
                        "kind": "start",
                        "by": "ai",
                        "at": "2026-08-28T00:00:00.000Z",
                    }
                ],
            )

            task_sync.append_task_log(feature, "T001", "ai")
            task_sync.materialize_log(feature, "ai", quiet=True)
            self.assertEqual(
                (["T001", "T002"], ["T001"]),
                task_sync.parse_task_markers(feature / "tasks.md"),
            )

            task_sync.append_task_log(feature, "T002", "ai")
            task_sync.materialize_log(feature, "ai", quiet=True)

            ctx = task_sync.read_ctx(feature / ".spec-context.json")
            self.assertEqual("implemented", ctx["status"])
            self.assertEqual(
                (["T001", "T002"], ["T001", "T002"]),
                task_sync.parse_task_markers(feature / "tasks.md"),
            )
            self.assertTrue(
                any(
                    entry.get("step") == "implement"
                    and entry.get("kind") == "complete"
                    and entry.get("task") is None
                    for entry in ctx["history"]
                )
            )

    def test_late_convergence_does_not_recheck_historically_finished_task(self) -> None:
        old_history = [
            {
                "step": "implement",
                "substep": None,
                "task": task_id,
                "kind": "complete",
                "by": "ai",
                "at": f"2026-08-27T00:00:0{index}.000Z",
            }
            for index, task_id in enumerate(("T001", "T002"), start=1)
        ]
        old_history.append(
            {
                "step": "implement",
                "substep": None,
                "kind": "complete",
                "by": "extension",
                "at": "2026-08-27T00:00:03.000Z",
            }
        )

        with tempfile.TemporaryDirectory() as temp:
            feature = self.make_feature(
                Path(temp),
                status="completed",
                tasks=(
                    "- [ ] **T001** Reopened but unfinished\n"
                    "- [ ] **T002** Reopened and finished\n"
                ),
                history=old_history,
            )

            task_sync.append_task_log(feature, "T002", "ai")
            task_sync.materialize_log(feature, "ai", quiet=True)

            self.assertEqual(
                (["T001", "T002"], ["T002"]),
                task_sync.parse_task_markers(feature / "tasks.md"),
            )
            ctx = task_sync.read_ctx(feature / ".spec-context.json")
            self.assertEqual("completed", ctx["status"])
            self.assertTrue((feature / ".spec-context.events.jsonl").is_file())


if __name__ == "__main__":
    unittest.main()
