package main

const helpText = `git release - tag a semver release safely

USAGE
  git release [major|minor|patch|<version>] [flags]
  git release undo [<tag>] [--yes] [--dry-run] [--force-remote]
  git release config [--init]
  git release complete [bash|zsh|fish]
  git release help

  (Use "git release help" or -h. "git release --help" is intercepted by git itself.)

WHAT IT DOES
  1. Guards: on the release branch, worktree clean, upstream set, not ahead/behind.
  2. Runs the "pre" steps (tests, lint). Stops at the first failure.
  3. Resolves the version: major|minor|patch bump the latest semver tag; or give
     1.2.3 / v1.2.3 explicitly. No argument: interactive menu on a terminal,
     failure otherwise.
  4. Runs the "bump" steps, commits their changes as "release: <tag>".
  5. Creates the (annotated) tag.
  6. Runs the "post" steps (e.g. push). Nothing is pushed unless a post step does it.

FLAGS
  --dry-run        show what would happen, change nothing
  -y, --yes        skip the confirmation prompt
  --branch NAME    release from NAME instead of the default branch
  --prefix v|""    tag style; normally auto-detected from existing tags
  --pre ID         make a pre-release: 1.3.0-ID.1, 1.3.0-ID.2, ...
  --sign           create a signed tag
  --no-pre         skip the "pre" steps

TAG STYLE
  Existing tags decide whether tags look like v1.2.3 or 1.2.3. If all agree, that style
  is used and saved to tag_prefix in .git-release.toml. If there are no tags, the style
  follows your version argument ("git release v0.1.0") or you are asked. Mixed styles
  are asked about (or need --prefix when not on a terminal).

FIRST RELEASE
  git release v0.1.0         explicit, no prompt
  git release                on a terminal: proposes 0.1.0

CONFIGURATION  .git-release.toml (repository root, commit it)
  branch     = "main"        default: remote default branch
  tag_prefix = "v"           "v" or ""
  sign       = false
  annotated  = true
  pre  = ["make test", "make lint"]
  bump = ["..."]             may edit files; tracked changes get committed
  post = ["git push origin HEAD --follow-tags"]

  Every step runs with "sh -c" in the repository root with:
    GIT_RELEASE_VERSION   1.4.0       (no prefix)
    GIT_RELEASE_TAG       v1.4.0
    GIT_RELEASE_PREVIOUS  v1.3.2      (empty on first release)
    GIT_RELEASE_BRANCH    main

  "pre" steps must leave the worktree clean. File changes belong in "bump".
  New files created by "bump" are not committed automatically: "git add" them in the step.

BUMP EXAMPLES
  package.json (jq):
    bump = ["jq '.version=\"$GIT_RELEASE_VERSION\"' package.json > package.json.tmp && mv package.json.tmp package.json"]
  package.json + lockfile (npm):
    bump = ["npm version --no-git-tag-version \"$GIT_RELEASE_VERSION\""]
  Cargo.toml (cargo-edit):
    bump = ["cargo set-version \"$GIT_RELEASE_VERSION\""]
  Cargo.toml (plain sed, first match only):
    bump = ["sed -i '0,/^version = .*/s//version = \"'\"$GIT_RELEASE_VERSION\"'\"/' Cargo.toml && cargo update -w"]
  pyproject.toml (poetry / uv):
    bump = ["poetry version \"$GIT_RELEASE_VERSION\""]
    bump = ["uv version \"$GIT_RELEASE_VERSION\""]
  pyproject.toml (sed):
    bump = ["sed -i 's/^version = .*/version = \"'\"$GIT_RELEASE_VERSION\"'\"/' pyproject.toml"]
  Maven:
    bump = ["mvn -q versions:set -DnewVersion=\"$GIT_RELEASE_VERSION\" -DgenerateBackupPoms=false"]
  Gradle (gradle.properties):
    bump = ["sed -i 's/^version=.*/version='\"$GIT_RELEASE_VERSION\"'/' gradle.properties"]
  Go: versions live in tags only; no bump needed.
  Changelog from commit messages:
    bump = ["{ echo \"## $GIT_RELEASE_TAG\"; git log --pretty='- %s' \"${GIT_RELEASE_PREVIOUS:+$GIT_RELEASE_PREVIOUS..}HEAD\"; echo; cat CHANGELOG.md; } > CHANGELOG.new && mv CHANGELOG.new CHANGELOG.md"]

POST EXAMPLES
  post = ["git push origin HEAD --follow-tags"]
  post = ["git push origin HEAD --follow-tags", "gh release create \"$GIT_RELEASE_TAG\" --generate-notes"]

UNDO
  If a step fails after changes began, the exact rollback commands are printed and, on a
  terminal, you are offered to run the local ones. Remote steps are never run automatically.

  git release undo           revert the latest release: delete local tag, drop the
                             unpushed release commit
  git release undo --force-remote
                             also delete the tag on the remote (shared history!)
  If the release commit was already pushed, undo tells you to "git revert HEAD".

SHELL COMPLETION
  bash/zsh:  . <(git release complete)       (add to ~/.bashrc or ~/.zshrc)
  fish:      git release complete fish | source
  bash needs git's own completion loaded (the "git" package ships it).

EXIT CODE
  0 on success, 1 on any failure.
`
