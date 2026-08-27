---
name: code-simplification-review
description: Use when working code needs simplification and refinement — after a feature or fix works, before commit/merge, or when asked to clean up, simplify, refine, or reduce complexity of changed code. Covers duplicate/reinvented helpers, redundant state, wasted computation, and band-aid fixes. Not for finding correctness bugs. Language- and project-agnostic.
---

# Code Simplification Review

## Overview

Improve the quality of changed code without changing what it does. **Preserve functionality — never change what the code does, only how.** This is not bug hunting: correctness review is a separate pass.

Best run in a fresh context — a subagent or a separate agent session, when the runtime supports one — so the review is not biased by the context that produced the code. Otherwise run it inline.

## Scope

Review **the changed code only** — `git diff` against upstream/main plus working-tree changes, or the target the user names. Do not refactor stable surrounding code; use it as the reference the new code must fit.

For Go code, also apply `.agents/skills/go-code-quality-review/SKILL.md`, including its Google Go Style hierarchy and its rule against fixed line-length limits. Use that guidance to choose the clearer, simpler form while preserving behavior.

## The Four Angles

Review the diff through each angle; each finding names file:line, the cost (what is duplicated, wasted, or harder to maintain), and the concrete simpler form.

### 1. Reuse

New code that re-implements something the codebase already has. Search shared/utility modules and files adjacent to the change; name the existing helper to call instead. Inline copies of an existing utility (even slightly reworded) count.

### 2. Simplification

Unnecessary complexity the diff adds:
- Redundant or derivable state (a counter maintained beside the collection it counts)
- Copy-paste blocks with slight variation — extract or restructure
- Deep nesting — invert conditions, return early
- Dead code left behind (unused functions, unreachable branches)
- Comments narrating obvious code — delete them
- Verbose forms with idiomatic equivalents

Name the simpler form that does the same job.

### 3. Efficiency

Wasted work the diff introduces:
- Repeated computation or I/O inside loops (recompiling patterns, re-fetching invariants) — hoist it
- Independent operations run sequentially where concurrent execution is natural and safe — two data fetches awaited one after another with no data dependency between them is the canonical case; flag it even when each call is fast
- Blocking work added to startup or hot paths
- Long-lived objects built from closures over the enclosing scope — they keep everything captured alive; prefer a struct/class holding only the fields it needs

Name the cheaper alternative.

### 4. Altitude

Is each change implemented at the right depth, or as a fragile band-aid? Special cases layered onto shared/generic code (a hard-coded ID or name inside a general function) signal the fix isn't deep enough — prefer generalizing the underlying mechanism (config, parameter, polymorphism) over accumulating special cases.

## Applying Fixes

When asked to fix (not just report): dedup findings pointing at the same line or mechanism, then fix each remaining one directly, following the target project's own conventions (instruction files such as AGENTS.md or CLAUDE.md, lint config, surrounding style).

**Skip — and note the skip instead of forcing it — any fix that would:**
- Change intended behavior, even subtly
- Require changes well outside the reviewed diff
- Be a false positive on closer reading

## Over-Simplification Guardrails

Clarity over brevity — explicit code beats compact code. Do NOT:
- Create clever solutions that are harder to understand than the original
- Merge distinct concerns into one function to save lines
- Remove abstractions that genuinely aid organization
- Optimize for fewer lines at the cost of readability (dense one-liners, nested ternaries — prefer if/else or switch)
- Make the code harder to debug or extend

## Output

- **Report mode**: findings grouped by angle, each with file:line, cost, and the simpler form.
- **Fix mode**: apply the fixes, then a brief summary of what was fixed and what was skipped and why — or confirm the code was already clean.
