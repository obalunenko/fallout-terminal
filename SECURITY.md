# Security Policy

Fallout Terminal is a hobby and home-use project maintained on a best-effort basis. Security
reports are still taken seriously, especially when they concern public ingress, authentication,
credential storage, filesystem access, or the separation between player and Overseer capabilities.

## Supported Versions

The latest published release and the current `main` branch receive security fixes. Older releases
may be fixed only when the change can be backported safely and without disproportionate maintenance
cost.

## Reporting a Vulnerability

Do not open a public issue for a vulnerability that could expose credentials, private session
content, filesystem access, trusted desktop operations, or a usable public-access bypass.

Use GitHub's private vulnerability reporting flow:

- [Report a vulnerability privately](https://github.com/obalunenko/Fallout-Terminal/security/advisories/new)

Include, when available:

- the affected version, commit, and operating system;
- the required configuration, including whether public access is enabled;
- minimal reproduction steps or a proof of concept;
- the security boundary that is crossed and the likely impact; and
- suggested mitigations, without including live credentials or private user data.

If private vulnerability reporting is unavailable, contact the repository owner through an
already-established private channel before sharing sensitive details. Non-sensitive hardening
suggestions may use an ordinary GitHub issue.

## Handling Reports

There is no guaranteed response or disclosure SLA. The maintainer will make a best-effort attempt
to acknowledge a complete report, reproduce it, assess supported releases, and coordinate a fix
before public disclosure.

Never send provider tokens, player passwords, session files containing private campaign material,
or other reusable secrets. Replace them with clearly marked test values.
