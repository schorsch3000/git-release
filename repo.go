package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Repo runs git commands in a directory.
type Repo struct{ Dir string }

// OpenRepo locates the repository root containing dir.
func OpenRepo(dir string) (*Repo, error) {
	r := &Repo{Dir: dir}
	root, err := r.git("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, errors.New("not inside a git repository")
	}
	return &Repo{Dir: root}, nil
}

func (r *Repo) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// ok reports whether the git command exits 0.
func (r *Repo) ok(args ...string) bool {
	_, err := r.git(args...)
	return err == nil
}

func (r *Repo) CurrentBranch() (string, error) {
	b, err := r.git("symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		return "", errors.New("HEAD is detached")
	}
	return b, nil
}

// DefaultBranch finds the remote's default branch, with local fallbacks.
func (r *Repo) DefaultBranch() (string, error) {
	if s, err := r.git("symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimPrefix(s, "origin/"), nil
	}
	candidates := []string{}
	if s, err := r.git("config", "init.defaultBranch"); err == nil && s != "" {
		candidates = append(candidates, s)
	}
	candidates = append(candidates, "main", "master")
	for _, c := range candidates {
		if r.ok("show-ref", "--verify", "--quiet", "refs/heads/"+c) {
			return c, nil
		}
	}
	return "", errors.New("cannot determine default branch; set branch in " + ConfigFile)
}

func (r *Repo) Clean() (bool, string, error) {
	s, err := r.git("status", "--porcelain")
	return s == "", s, err
}

func (r *Repo) Upstream() (string, error) {
	return r.git("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
}

// Remote returns the remote of the current branch, falling back to origin.
func (r *Repo) Remote() string {
	if b, err := r.CurrentBranch(); err == nil {
		if s, err := r.git("config", "branch."+b+".remote"); err == nil && s != "" && s != "." {
			return s
		}
	}
	return "origin"
}

// AheadBehind compares HEAD with its upstream.
func (r *Repo) AheadBehind() (ahead, behind int, err error) {
	s, err := r.git("rev-list", "--left-right", "--count", "HEAD...@{u}")
	if err != nil {
		return 0, 0, err
	}
	_, err = fmt.Sscanf(s, "%d\t%d", &ahead, &behind)
	return
}

func (r *Repo) SemverTags() ([]Tag, error) {
	out, err := r.git("tag", "--list")
	if err != nil {
		return nil, err
	}
	var tags []Tag
	for _, name := range strings.Fields(out) {
		if t, ok := ParseTag(name); ok {
			tags = append(tags, t)
		}
	}
	return tags, nil
}

func (r *Repo) TagExists(name string) bool {
	return r.ok("rev-parse", "-q", "--verify", "refs/tags/"+name)
}

func (r *Repo) RemoteHasTag(remote, name string) bool {
	out, err := r.git("ls-remote", "--tags", remote, "refs/tags/"+name)
	return err == nil && out != ""
}
