package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type releaseOpts struct {
	Arg    string
	DryRun bool
	Yes    bool
	NoPre  bool
	Sign   bool
	Branch string
	Prefix *string
	Pre    string
}

func (a *App) guards(branch string) error {
	cur, err := a.Repo.CurrentBranch()
	if err != nil {
		return err
	}
	if cur != branch {
		return fmt.Errorf("on branch %q but release branch is %q (switch, or use --branch / branch in %s)", cur, branch, ConfigFile)
	}
	clean, status, err := a.Repo.Clean()
	if err != nil {
		return err
	}
	if !clean {
		return fmt.Errorf("worktree is not clean (commit or stash first; a new %s must be committed):\n%s", ConfigFile, status)
	}
	up, err := a.Repo.Upstream()
	if err != nil {
		return fmt.Errorf("branch %q has no upstream (git push -u)", cur)
	}
	if _, err := a.Repo.git("fetch", "--quiet", a.Repo.Remote()); err != nil {
		return err
	}
	ahead, behind, err := a.Repo.AheadBehind()
	if err != nil {
		return err
	}
	if behind > 0 {
		return fmt.Errorf("%s is %d commit(s) behind %s (pull first)", cur, behind, up)
	}
	if ahead > 0 {
		return fmt.Errorf("%s has %d unpushed commit(s) relative to %s (push first)", cur, ahead, up)
	}
	return nil
}

// explicitVersion parses a version argument with optional v prefix.
func explicitVersion(arg string) (v Version, prefix string, ok bool) {
	t, ok := ParseTag(arg)
	return t.V, t.Prefix, ok
}

// resolvePrefix picks the tag prefix. save reports that it should be written to the config.
func (a *App) resolvePrefix(o releaseOpts, cfg Config, tags []Tag) (prefix string, save bool, err error) {
	if o.Prefix != nil {
		return *o.Prefix, false, nil
	}
	if cfg.TagPrefix != nil {
		return *cfg.TagPrefix, false, nil
	}
	detected, state := DetectPrefix(tags)
	if state == prefixClear {
		return detected, true, nil
	}
	if _, argPrefix, ok := explicitVersion(o.Arg); ok && state == prefixNone {
		return argPrefix, true, nil
	}
	if !a.TTY {
		what := "no existing version tags"
		if state == prefixMixed {
			what = "existing tags mix plain and v-prefixed styles"
		}
		return "", false, fmt.Errorf("cannot determine tag prefix: %s (use --prefix or tag_prefix in %s)", what, ConfigFile)
	}
	ans, err := a.ask("Tag prefix, 'v' (v1.2.3) or 'plain' (1.2.3)", "v")
	if err != nil {
		return "", false, err
	}
	switch strings.ToLower(ans) {
	case "v":
		return "v", true, nil
	case "plain", "":
		return "", true, nil
	}
	return "", false, fmt.Errorf("invalid answer %q", ans)
}

func (a *App) resolveVersion(o releaseOpts, tags []Tag, prefix string) (Version, error) {
	var latestV *Version
	if t, ok := LatestTag(tags); ok {
		latestV = &t.V
	}
	arg := o.Arg
	if arg == "" {
		if !a.TTY {
			return Version{}, errors.New("no version argument (major|minor|patch|<version>) and not a terminal")
		}
		var err error
		if arg, err = a.pickVersion(latestV, o.Pre, tags, prefix); err != nil {
			return Version{}, err
		}
	}
	switch arg {
	case "major", "minor", "patch":
		return NextVersion(latestV, arg, o.Pre, tags)
	}
	v, argPrefix, ok := explicitVersion(arg)
	if !ok {
		return Version{}, fmt.Errorf("%q is not major|minor|patch or a semantic version", arg)
	}
	if argPrefix != prefix {
		return Version{}, fmt.Errorf("version %q contradicts tag style %q (drop the prefix or use --prefix)", arg, prefix+"X.Y.Z")
	}
	if latestV != nil && v.Compare(*latestV) <= 0 {
		return Version{}, fmt.Errorf("version %s is not greater than latest %s", v, latestV)
	}
	return v, nil
}

func (a *App) pickVersion(latest *Version, pre string, tags []Tag, prefix string) (string, error) {
	if latest == nil {
		fmt.Fprintln(a.Out, "No version tags yet.")
		ans, err := a.ask("First version", "0.1.0")
		return withPrefix(ans, prefix), err
	}
	fmt.Fprintf(a.Out, "Latest: %s%s\n", prefix, latest)
	for i, kind := range []string{"patch", "minor", "major"} {
		if v, err := NextVersion(latest, kind, pre, tags); err == nil {
			fmt.Fprintf(a.Out, "  %d) %-5s %s%s\n", i+1, kind, prefix, v)
		}
	}
	ans, err := a.ask("Choose 1-3 or type a version", "1")
	if err != nil {
		return "", err
	}
	switch ans {
	case "1":
		return "patch", nil
	case "2":
		return "minor", nil
	case "3":
		return "major", nil
	}
	return withPrefix(ans, prefix), nil
}

// withPrefix lets interactive answers omit the tag prefix.
func withPrefix(ans, prefix string) string {
	if _, ok := ParseVersion(ans); ok {
		return prefix + ans
	}
	return ans
}

// Release runs the whole flow.
func (a *App) Release(o releaseOpts) error {
	cfg, _, err := LoadConfig(a.Repo.Dir)
	if err != nil {
		return err
	}
	branch := o.Branch
	if branch == "" {
		branch = cfg.Branch
	}
	if branch == "" {
		if branch, err = a.Repo.DefaultBranch(); err != nil {
			return err
		}
	}
	if err := a.guards(branch); err != nil {
		return err
	}
	tags, err := a.Repo.SemverTags()
	if err != nil {
		return err
	}
	prefix, savePrefix, err := a.resolvePrefix(o, cfg, tags)
	if err != nil {
		return err
	}
	version, err := a.resolveVersion(o, tags, prefix)
	if err != nil {
		return err
	}
	tag := prefix + version.String()
	if a.Repo.TagExists(tag) {
		return fmt.Errorf("tag %s already exists", tag)
	}
	previous := ""
	if t, ok := LatestTag(tags); ok {
		previous = t.Name
	}
	env := []string{
		"GIT_RELEASE_VERSION=" + version.String(),
		"GIT_RELEASE_TAG=" + tag,
		"GIT_RELEASE_PREVIOUS=" + previous,
		"GIT_RELEASE_BRANCH=" + branch,
	}

	a.summary(tag, previous, branch, cfg, o, savePrefix, prefix)
	if o.DryRun {
		fmt.Fprintln(a.Out, "\n(dry run: nothing changed)")
		return nil
	}
	if a.TTY && !o.Yes {
		ok, err := a.confirm("Proceed?")
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("aborted")
		}
	}

	if !o.NoPre {
		if err := a.runHooks("pre", cfg.Pre, env); err != nil {
			return err
		}
		if clean, status, err := a.Repo.Clean(); err != nil {
			return err
		} else if !clean {
			return fmt.Errorf("pre steps modified the worktree (use bump for file changes):\n%s", status)
		}
	}

	if err := a.runHooks("bump", cfg.Bump, env); err != nil {
		return a.failed(err, tag, true)
	}
	if savePrefix {
		if err := SaveTagPrefix(a.Repo.Dir, prefix); err != nil {
			return a.failed(err, tag, true)
		}
	}
	if err := a.commitRelease(tag, savePrefix); err != nil {
		return a.failed(err, tag, true)
	}

	tagArgs := []string{"tag"}
	switch {
	case o.Sign || cfg.Sign:
		tagArgs = append(tagArgs, "-s", "-m", "Release "+tag)
	case cfg.annotated():
		tagArgs = append(tagArgs, "-a", "-m", "Release "+tag)
	}
	if _, err := a.Repo.git(append(tagArgs, tag)...); err != nil {
		return a.failed(err, tag, false)
	}
	fmt.Fprintf(a.Out, "==> tagged %s\n", tag)

	if err := a.runHooks("post", cfg.Post, env); err != nil {
		return a.failed(err, tag, false)
	}
	fmt.Fprintf(a.Out, "==> released %s\n", tag)
	return nil
}

func (a *App) summary(tag, previous, branch string, cfg Config, o releaseOpts, savePrefix bool, prefix string) {
	w := a.Out
	fmt.Fprintf(w, "Release %s on %s (previous: %s)\n", tag, branch, orDash(previous))
	if savePrefix {
		fmt.Fprintf(w, "  will write tag_prefix = %q to %s\n", prefix, ConfigFile)
	}
	list := func(name string, cmds []string) {
		for _, c := range cmds {
			fmt.Fprintf(w, "  %-4s %s\n", name, c)
		}
	}
	if !o.NoPre {
		list("pre", cfg.Pre)
	}
	list("bump", cfg.Bump)
	fmt.Fprintf(w, "  tag  %s\n", tag)
	list("post", cfg.Post)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func (a *App) runHooks(stage string, cmds []string, env []string) error {
	for _, c := range cmds {
		fmt.Fprintf(a.Out, "==> %s: %s\n", stage, c)
		cmd := exec.Command("sh", "-c", c)
		cmd.Dir = a.Repo.Dir
		cmd.Env = append(os.Environ(), env...)
		cmd.Stdout, cmd.Stderr = a.Out, a.Err
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s step failed: %s: %w", stage, c, err)
		}
	}
	return nil
}

// commitRelease commits tracked changes (and the config file if written) as the release commit.
func (a *App) commitRelease(tag string, withConfig bool) error {
	if _, err := a.Repo.git("add", "-u"); err != nil {
		return err
	}
	if withConfig {
		if _, err := a.Repo.git("add", "--", ConfigFile); err != nil {
			return err
		}
	}
	if a.Repo.ok("diff", "--cached", "--quiet") {
		return nil
	}
	if _, err := a.Repo.git("commit", "--quiet", "-m", releaseSubject(tag)); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "==> committed %q\n", releaseSubject(tag))
	return nil
}

func releaseSubject(tag string) string { return "release: " + tag }

// failed reports the failure, shows how to roll back and, on a TTY, offers to do the local part.
// dirty means bump changes may be uncommitted in the worktree.
func (a *App) failed(cause error, tag string, dirty bool) error {
	fmt.Fprintf(a.Err, "\ngit-release: failed: %v\n", cause)
	plan := a.undoPlan(tag, dirty)
	if len(plan.Steps) == 0 && len(plan.Notes) == 0 {
		return cause
	}
	fmt.Fprintln(a.Err, "To roll back:")
	a.printPlan(a.Err, plan)
	if a.TTY && plan.hasLocal() {
		if ok, err := a.confirm("Run the local rollback steps now?"); err == nil && ok {
			if err := a.runPlan(plan, false); err != nil {
				fmt.Fprintf(a.Err, "git-release: rollback failed: %v\n", err)
			}
		}
	}
	return cause
}
