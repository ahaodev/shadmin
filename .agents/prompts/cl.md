---
description: Audit changelog entries before release
---

Audit commits since the latest release relevant to the current release line. Use the release/tag supplied in the request, or inspect reachable tags and existing `CHANGELOG.md` sections to establish the base; do not assume the largest version tag belongs to the current branch. If there is no release tag, review the available history and state that limit.

## Process

1. Read `CHANGELOG.md` and list commits after the release base with `git log <tag>..HEAD --oneline`. Inspect source changes, not just commit titles.
2. Skip pure documentation, formatting, changelog and release-housekeeping commits. Group related commits by user-visible change; significant build/tooling changes may warrant a Tooling entry.
3. Review maintained sources: `ent/schema/` and `ent/generate.go` are source, not generated output. Interpret changes to generated Ent, Swagger and the route tree through their source changes; do not count regeneration as a separate feature.
4. Report missing, inaccurate or duplicate entries. An audit request alone does not authorize editing the changelog.
5. If the user requests changelog updates as maintainer/release work, prepare the entries for the target release version and date. Update that section if it exists, otherwise add it above released history. Use release context already supplied; if the target version/date cannot be established, present the draft and ask for that missing information before writing. Do not invent a version for an ordinary task or rewrite released sections.

Follow the [shared release rules](../CONTRIBUTING.md#commits-and-releases). Existing explicit authorization is sufficient; do not ask again solely because this template mentions writing.

## Format

A release section is `## [x.y.z] - YYYY-MM-DD`. Include only populated subsections, in this order: Added, Changed, Fixed, Security, Cleanup, Tooling, Breaking.

Use one concise past-tense bullet per change, without issue or PR links. Include the migration impact for breaking changes.
