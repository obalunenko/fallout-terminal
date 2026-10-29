#!/usr/bin/env bash

# Rewrites Spec Kit skill invocations in .claude/skills to the Claude Code
# slash form. Extension skills reference dotted `/speckit.ext.name` ids, and
# older renders use the `$speckit-name` syntax, while the skills are
# registered as `speckit-ext-name`. Both forms become `/speckit-ext-name`.
# File paths such as `commands/speckit.ext.name.md` are left untouched. The
# rewrite is idempotent.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly repo_root

shopt -s nullglob
skill_files=("${repo_root}"/.claude/skills/speckit-*/SKILL.md)
if ((${#skill_files[@]} == 0)); then
	printf 'No Spec Kit skills found in %s/.claude/skills\n' "${repo_root}" >&2
	exit 1
fi

perl -pi -e '
	s{(?<![\w/.-])[\$/]speckit([.-][a-z0-9][a-z0-9-]*(?:\.[a-z0-9][a-z0-9-]*)*)(?!\.md)}{
		my $name = $1;
		$name =~ tr/./-/;
		"/speckit$name";
	}ge;
' "${skill_files[@]}"

printf 'Normalized skill invocations in %d Spec Kit skills.\n' "${#skill_files[@]}"
