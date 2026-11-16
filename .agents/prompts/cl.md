---
description: Audit changelog entries before release
---
Audit changelog entries for all commits since the last release.

## Process

1. **Find the last release tag:**
   ```bash
   git tag --sort=-version:refname | head -1
   ```

2. **List all commits since that tag:**
   ```bash
   git log <tag>..HEAD --oneline
   ```

3. **Read `CHANGELOG.md` at the repository root.**

4. **For each commit, check:**
   - Skip: changelog updates, doc-only changes, release housekeeping
   - Skip: generated files (`ent/**`, `docs/swagger.json`, `docs/swagger.yaml`, `docs/docs.go`) unless accompanied by an intentional change in non-generated source
   - Determine which surface the commit affects (use `git show <hash> --stat`): backend, `frontend/`, `cli/`, or docs and agent instructions
   - Verify a changelog entry exists

5. **If there are unreleased changes, add a new version section** `## [x.y.z] - YYYY-MM-DD` above the latest one in `CHANGELOG.md`. Propose the summary to the user for confirmation before writing it.

6. **Report:**
   - List commits with missing entries
   - List entries that need correction
   - Add any missing entries directly

## Changelog Format Reference

A release section is `## [x.y.z] - YYYY-MM-DD`, with subsections in this order:

- `### Added` - New features
- `### Changed` - Changes to existing functionality
- `### Fixed` - Bug fixes
- `### Security` - Security fixes
- `### Cleanup` - Dead code and duplication removal
- `### Tooling` - Build, checks, agent instructions
- `### Breaking` - API changes requiring migration

Entries are single bullets in the past tense, without issue or PR links.
