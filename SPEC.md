# git-release

A command line tool, invoked as `git release [args]` (the binary is `git-release` on `PATH`), that
performs a typical release: run checks, optionally bump version files, commit, create a semver tag,
and run follow-up commands such as pushing.

Written in Go. Shells out to the `git` binary instead of using a Go git library, so behaviour
matches the user's git.

## Principles

- Safe by default: refuses to release from a dirty tree, the wrong branch, or an unpushed/behind state.
- Semver only. No CalVer.
- Configuration lives in one committed file, `.git-release.toml`. No git-config keys, no TUI.
- Interactive when attached to a TTY, strictly non-interactive (fail instead of prompt) otherwise.
- Every failure after the first mutation prints, and on a TTY offers, how to roll back.

## Configuration: `.git-release.toml`

Located at the repository root. All keys optional. Unknown keys are an error (catches typos).

```toml
branch     = "main"   # default: the remote's default branch (origin/HEAD), then main/master
tag_prefix = "v"      # "v" or "". Auto-detected from existing tags and written here when unambiguous
sign       = false    # create a signed tag (git tag -s)
annotated  = true      # annotated tag (ignored when sign = true)

pre  = ["make test", "make lint"]   # checks; stop at first failure; must not modify the tree
bump = [                            # may modify files; changes are committed
  "jq '.version=\"$GIT_RELEASE_VERSION\"' package.json > package.json.tmp && mv package.json.tmp package.json",
]
post = ["git push origin HEAD --follow-tags"]
```

Each entry is run with `sh -c`, in the repository root, with this environment:

| variable                | value                                          |
|-------------------------|------------------------------------------------|
| `GIT_RELEASE_VERSION`   | new version without prefix, e.g. `1.4.0`       |
| `GIT_RELEASE_TAG`       | new tag name, e.g. `v1.4.0`                    |
| `GIT_RELEASE_PREVIOUS`  | previous tag name, empty on first release      |
| `GIT_RELEASE_BRANCH`    | release branch                                 |

## Release flow

`git release [major|minor|patch|<version>] [flags]`

1. **Guards** (always, also with `--dry-run`)
   - current branch equals the release branch (`branch` config, `--branch`, or default branch)
   - worktree clean (no staged, unstaged or untracked changes)
   - branch has an upstream; `git fetch` it; local is neither ahead nor behind
2. **Tag prefix**: resolved in order `--prefix`, `tag_prefix` config, detection from existing semver
   tags. All `v`-prefixed or all plain: use it and save it. No tags: take it from the version argument
   (`v0.1.0` vs `0.1.0`), else on a TTY ask (default `v`). Mixed: on a TTY ask, else fail.
   A version argument whose prefix contradicts the resolved prefix is an error.
3. **Version**: `major|minor|patch` bump the latest semver tag; an explicit `1.2.3` / `v1.2.3` must be
   greater than the latest tag. No argument: on a TTY show a menu (patch/minor/major or typed
   version), otherwise fail.
   - First release (no tags): `major` = 1.0.0, `minor` = 0.1.0, `patch` = 0.0.1. On a TTY without an
     argument the proposal is `0.1.0`.
   - `patch` on a pre-release (`1.2.3-rc.1`) releases the core (`1.2.3`).
   - `--pre rc` appends `-rc.N`, N one higher than the highest existing `-rc.N` of that core version.
4. **Confirm** summary (TTY, unless `--yes`).
5. **pre** steps (skipped by `--no-pre`). The tree must still be clean afterwards.
6. **bump** steps. Tracked-file changes (`git add -u`) plus a newly written `.git-release.toml` are
   committed as `release: <tag>`. Nothing changed means no commit.
7. **Tag**: annotated by default (`-s` if `sign`).
8. **post** steps.

Nothing is pushed unless a `post` step does it. Recommended: `git push origin HEAD --follow-tags`
pushes the release commit and the tag together.

### Dry run

`--dry-run` runs the guards, resolves prefix and version, prints every step that would run, and
changes nothing (no hooks, no commit, no tag, no config write).

## Undo

Failure after step 6 begins: print the exact commands to roll back; on a TTY offer to run the local
ones. Remote steps are only ever printed, never run automatically.

`git release undo [<tag>] [--yes] [--dry-run] [--force-remote]` (default tag: the latest semver tag)

1. delete the tag on the remote (`git push <remote> :refs/tags/<tag>`) — only with `--force-remote`
2. delete the local tag
3. if `HEAD` is the unpushed commit `release: <tag>`: `git reset --hard HEAD~1`; if it was already
   pushed: print `git revert HEAD` advice and leave it alone

If the tag exists on the remote and `--force-remote` is absent, `undo` refuses. On a TTY it asks for
confirmation; without a TTY it prints the plan and requires `--yes`.

## Other subcommands

- `git release config` prints the effective configuration and where each value comes from.
  `--init` writes a commented starter `.git-release.toml`.
- `git release help` prints detailed help including hook examples (`jq`/package.json, `cargo`,
  `poetry`, `sed`, `mvn`, `npm`, changelog). Note: `git release --help` is intercepted by git (it looks
  for a man page); use `git release help` or `-h`.
- `git release complete [bash|zsh|fish]` prints a shell completion script. Shell defaults to
  `$SHELL`. Use `. <(git release complete)` in bash/zsh, `git release complete fish | source` in fish.

## Flags

`--dry-run`, `--yes`, `--branch <name>`, `--prefix <v|"">`, `--pre <id>`, `--sign`, `--no-pre`.
Flags may appear before or after the positional argument.

## Exit codes

0 success, 1 any failure (message on stderr, prefixed `git-release:`).

## Out of scope

Changelog generation (use a `bump` hook), editing version files natively, TUI configuration,
CalVer, hosting-provider releases (GitHub/GitLab). A `post` hook can call `gh release create`.
