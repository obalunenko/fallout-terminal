#!/usr/bin/env python3
"""Small AT-SPI driver for the matching-host Linux package smoke."""

from __future__ import annotations

import argparse
import pathlib
import sys
import time

import pyatspi


def descendants(node):
    yield node
    for index in range(getattr(node, "childCount", 0)):
        try:
            child = node.getChildAtIndex(index)
        except Exception:
            continue
        if child is not None:
            yield from descendants(child)


def candidates():
    desktop = pyatspi.Registry.getDesktop(0)
    for application_index in range(desktop.childCount):
        application = desktop.getChildAtIndex(application_index)
        if application is not None:
            yield from descendants(application)


def accessible_name(node) -> str:
    try:
        return node.name or ""
    except Exception:
        return ""


def wait_for_named(name: str, prefix: bool, timeout: float):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        for node in candidates():
            candidate = accessible_name(node)
            if candidate == name or (prefix and candidate.startswith(name)):
                return node
        time.sleep(0.2)
    raise RuntimeError(f"accessible element was not observed: {name}")


def invoke_named(name: str, prefix: bool, timeout: float) -> None:
    deadline = time.monotonic() + timeout
    last_error = None
    while time.monotonic() < deadline:
        for node in candidates():
            candidate = accessible_name(node)
            if candidate == name or (prefix and candidate.startswith(name)):
                try:
                    invoke(node)
                    return
                except RuntimeError as error:
                    last_error = error
        time.sleep(0.2)
    if last_error is not None:
        raise last_error
    raise RuntimeError(f"accessible element was not observed: {name}")


def invoke(node) -> None:
    try:
        actions = node.queryAction()
    except Exception as error:
        raise RuntimeError(f"accessible element has no invokable action: {accessible_name(node)}") from error
    for index in range(actions.nActions):
        action_name = (actions.getName(index) or "").lower()
        if action_name in {"click", "press", "activate"}:
            if not actions.doAction(index):
                raise RuntimeError(f"accessible action failed: {accessible_name(node)}")
            return
    if actions.nActions > 0 and actions.doAction(0):
        return
    raise RuntimeError(f"accessible element exposed no usable action: {accessible_name(node)}")


def assert_canaries_absent(canary_file: pathlib.Path, timeout: float) -> None:
    canaries = [line for line in canary_file.read_text(encoding="ascii").splitlines() if line]
    if not canaries:
        raise RuntimeError("canary file is empty")
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        names = [accessible_name(node) for node in candidates()]
        if any(canary in name for canary in canaries for name in names):
            raise RuntimeError("secret canary appeared in public native accessibility state")
        time.sleep(0.2)


def main() -> int:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)
    invoke_parser = subparsers.add_parser("invoke")
    invoke_parser.add_argument("--name", required=True)
    invoke_parser.add_argument("--prefix", action="store_true")
    invoke_parser.add_argument("--timeout", type=float, default=15)
    name_parser = subparsers.add_parser("assert-name")
    name_parser.add_argument("--name", required=True)
    name_parser.add_argument("--prefix", action="store_true")
    name_parser.add_argument("--timeout", type=float, default=15)
    absent_parser = subparsers.add_parser("assert-canaries-absent")
    absent_parser.add_argument("--canary-file", type=pathlib.Path, required=True)
    absent_parser.add_argument("--timeout", type=float, default=1)
    args = parser.parse_args()

    if args.command == "invoke":
        invoke_named(args.name, args.prefix, args.timeout)
    elif args.command == "assert-name":
        wait_for_named(args.name, args.prefix, args.timeout)
    else:
        assert_canaries_absent(args.canary_file, args.timeout)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as error:
        print(f"native-ui-smoke: {error}", file=sys.stderr)
        raise SystemExit(1)
