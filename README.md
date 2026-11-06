# git-release

Tag semver releases safely: `git release minor`.

It checks that you are on the release branch with a clean, pushed worktree, runs your checks, bumps
version files, commits, tags, and runs follow-up commands such as pushing. If something fails midway,
it tells you how to roll back.

## Install

```sh
go install github.com/schorsch3000/git-release@latest
```

Make sure `git-release` is on your `PATH`; git then picks it up as `git release`. Prebuilt binaries
for Linux and macOS are attached to each GitHub release.

Shell completion (bash, zsh, fish):

```sh
. <(git release complete)                 # bash/zsh, put in your rc file
git release complete fish | source        # fish
```

## Usage

```sh
git release patch            # or minor, major
git release 1.4.0            # explicit version (v1.4.0 if your tags use the v prefix)
git release minor --pre rc   # 1.5.0-rc.1
git release minor --dry-run  # show the plan, change nothing
git release                  # on a terminal: pick interactively
git release undo             # revert the latest release
git release help             # full help with hook examples
```

Before anything happens it requires: current branch is the release branch (the remote default
branch, or `branch` in the config), worktree clean, upstream set and in sync.

Tag style (`v1.2.3` vs `1.2.3`) is detected from existing tags and saved. With no tags it follows the
version you pass (`git release v0.1.0`) or asks. Only semver is supported.

## Configuration

Optional `.git-release.toml` in the repo root, committed:

```toml
branch = "main"
pre  = ["make test", "make lint"]                      # checks, stop at first failure
bump = ["jq '.version=\"$GIT_RELEASE_VERSION\"' package.json > package.json.tmp && mv package.json.tmp package.json"]
post = ["git push origin HEAD --follow-tags"]          # nothing is pushed unless you say so
```

Steps run with `sh -c` and get `GIT_RELEASE_VERSION`, `GIT_RELEASE_TAG`, `GIT_RELEASE_PREVIOUS` and
`GIT_RELEASE_BRANCH`. Changes made by `bump` are committed as `release: <tag>`.
`git release config` shows the effective settings, `git release config --init` writes a starter file.
More `bump` examples (npm, cargo, poetry, maven, changelog) are in `git release help`.

## Undo

Failures after the first change print the exact rollback commands, and on a terminal offer to run the
local ones. `git release undo` removes the local tag and an unpushed release commit. Deleting a pushed
tag needs `--force-remote`; a pushed release commit is never rewritten.

See [SPEC.md](SPEC.md) for the full behaviour.
