---
name: clog
description: Manage changelog fragments with the `clog` tool. Use when asked to add/record changelog changes for a change, preview the next release entry, validate fragments, or cut a release. Invoke as `/clog add changelog changes for this change`, `/clog preview`, `/clog validate`, or `/clog release`.
---

# clog — changelog fragment manager

This repo records changelog entries as YAML **fragments** in `changelog.d/<branch>.yaml`, not by editing `CHANGELOG.md`. The `clog` binary (on PATH) merges all fragments into `CHANGELOG.md` at release time. `clog` has **no command for adding entries** — entries are added by editing the fragment YAML, which is what this skill does.

`$ARGUMENTS` is the user's request. Pick the action from it; default to **add** when it describes changes to record (e.g. "add changelog changes for this change").

## Fragment schema

A fragment is YAML with eight category keys, each a list of strings. Unused categories keep a single empty-string placeholder. Real entries replace the placeholder:

```yaml
deployment:
  - ""
added:
  - "New `wf prune` command removes worktrees whose branches were deleted"
changed:
  - ""
deprecated:
  - ""
removed:
  - ""
fixed:
  - "Status refresh no longer panics when a worktree path is missing"
security:
  - ""
yanked:
  - ""
```

Categories: `deployment`, `added`, `changed`, `deprecated`, `removed`, `fixed`, `security`, `yanked`. Map changes by intent — new capability → `added`; behavior change to existing capability → `changed`; bug fix → `fixed`; removed feature → `removed`; security fix → `security`.

## Action: add (default)

1. **Find/create the fragment.** Get the current branch (`git rev-parse --abbrev-ref HEAD`). Look for its fragment in `changelog.d/` (ignore `sample.yaml`). If none exists, run `clog new` to create it, then locate the new file (the non-`sample` `.yaml` that appeared).
2. **Understand the change at a feature level.** Use the user's description in `$ARGUMENTS`. If it's thin or absent, read `git diff` / `git diff --staged` / recent commits to figure out *what the change accomplishes for a user*. Describe outcomes — "Added X", "Fixed Y" — **never** file-by-file mechanics like "edited cli.go".
3. **Add as many entries as the change warrants.** One change often spans categories — e.g. a PR that adds a feature *and* fixes a bug gets one `added` entry and one `fixed` entry. Split distinct features into separate entries rather than one run-on line.
4. **Edit the fragment YAML.** Replace the `- ""` placeholder under each category you're filling with your entries (one list item per entry). Leave untouched categories exactly as they are (keep their `- ""`). Keep entries concise, imperative-past, and user-facing.
5. **Validate.** Run `clog validate` and fix any error it reports.
6. **Report** which categories/entries you added, and the fragment path. Do not commit unless asked.

## Action: preview

Run `clog preview` and show the output — this is what the next release entry will look like.

## Action: validate

Run `clog validate` and report results; fix schema problems if asked.

## Action: release (destructive — confirm first)

`clog release` merges all fragments into `CHANGELOG.md` and **deletes** them. Before running it, confirm with the user. Then follow the repo's tag-driven flow: `clog release` → set `VERSION` to the new version → commit → `git tag vX.Y.Z` → `git push origin vX.Y.Z`. CI fails if `VERSION` doesn't match the tag.
