package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

type undoStep struct {
	Desc   string
	Args   []string
	Remote bool
}

type undoPlan struct {
	Steps []undoStep
	Notes []string
}

func (p undoPlan) hasLocal() bool {
	for _, s := range p.Steps {
		if !s.Remote {
			return true
		}
	}
	return false
}

func (p undoPlan) hasRemote() bool {
	for _, s := range p.Steps {
		if s.Remote {
			return true
		}
	}
	return false
}

// undoPlan works out what is needed to revert a release of tag from the current repo state.
// restoreDirty adds a step discarding uncommitted bump changes.
func (a *App) undoPlan(tag string, restoreDirty bool) undoPlan {
	var p undoPlan
	r := a.Repo
	remote := r.Remote()
	if restoreDirty {
		if clean, _, err := r.Clean(); err == nil && !clean {
			p.Steps = append(p.Steps, undoStep{Desc: "discard uncommitted release changes", Args: []string{"restore", "--staged", "--worktree", "."}})
		}
	}
	if r.RemoteHasTag(remote, tag) {
		p.Steps = append(p.Steps, undoStep{Desc: "delete tag on " + remote, Args: []string{"push", remote, ":refs/tags/" + tag}, Remote: true})
	}
	if r.TagExists(tag) {
		p.Steps = append(p.Steps, undoStep{Desc: "delete local tag", Args: []string{"tag", "-d", tag}})
	}
	if subj, err := r.git("log", "-1", "--format=%s"); err == nil && subj == releaseSubject(tag) {
		if r.ok("merge-base", "--is-ancestor", "HEAD", "@{u}") {
			p.Notes = append(p.Notes, "release commit is already pushed; undo it with: git revert HEAD")
		} else {
			p.Steps = append(p.Steps, undoStep{Desc: "drop unpushed release commit", Args: []string{"reset", "--hard", "HEAD~1"}})
		}
	}
	return p
}

func (a *App) printPlan(w io.Writer, p undoPlan) {
	for _, s := range p.Steps {
		fmt.Fprintf(w, "  git %-40s # %s\n", strings.Join(s.Args, " "), s.Desc)
	}
	for _, n := range p.Notes {
		fmt.Fprintf(w, "  note: %s\n", n)
	}
}

func (a *App) runPlan(p undoPlan, includeRemote bool) error {
	for _, s := range p.Steps {
		if s.Remote && !includeRemote {
			continue
		}
		fmt.Fprintf(a.Out, "==> git %s\n", strings.Join(s.Args, " "))
		if _, err := a.Repo.git(s.Args...); err != nil {
			return err
		}
	}
	return nil
}

type undoOpts struct {
	Tag         string
	Yes         bool
	DryRun      bool
	ForceRemote bool
}

// Undo reverts a release (default: the latest semver tag).
func (a *App) Undo(o undoOpts) error {
	tag := o.Tag
	if tag == "" {
		tags, err := a.Repo.SemverTags()
		if err != nil {
			return err
		}
		t, ok := LatestTag(tags)
		if !ok {
			return errors.New("no version tags, nothing to undo")
		}
		tag = t.Name
	}
	plan := a.undoPlan(tag, false)
	if len(plan.Steps) == 0 {
		a.printPlan(a.Out, plan)
		return fmt.Errorf("nothing to undo for %s", tag)
	}
	fmt.Fprintf(a.Out, "Undo %s:\n", tag)
	a.printPlan(a.Out, plan)
	if o.DryRun {
		fmt.Fprintln(a.Out, "\n(dry run: nothing changed)")
		return nil
	}
	if plan.hasRemote() && !o.ForceRemote {
		return fmt.Errorf("tag %s exists on the remote; deleting it rewrites shared history. Re-run with --force-remote if you are sure", tag)
	}
	if !o.Yes {
		if !a.TTY {
			return errors.New("not a terminal: re-run with --yes to execute the steps above")
		}
		ok, err := a.confirm("Run these steps?")
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("aborted")
		}
	}
	return a.runPlan(plan, o.ForceRemote)
}
