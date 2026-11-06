package main

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionCompare(t *testing.T) {
	order := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "2.0.0"}
	for i := 0; i < len(order)-1; i++ {
		a, _ := ParseVersion(order[i])
		b, _ := ParseVersion(order[i+1])
		if a.Compare(b) != -1 || b.Compare(a) != 1 {
			t.Errorf("%s should be < %s", order[i], order[i+1])
		}
	}
	for _, bad := range []string{"1.2", "01.2.3", "v1.2.3", "1.2.3+build", "a.b.c"} {
		if _, ok := ParseVersion(bad); ok {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func TestNextVersion(t *testing.T) {
	mk := func(names ...string) []Tag {
		var ts []Tag
		for _, n := range names {
			tg, _ := ParseTag(n)
			ts = append(ts, tg)
		}
		return ts
	}
	cases := []struct {
		tags      []string
		kind, pre string
		want      string
	}{
		{nil, "minor", "", "0.1.0"},
		{nil, "major", "", "1.0.0"},
		{[]string{"v1.2.3"}, "patch", "", "1.2.4"},
		{[]string{"v1.2.3"}, "minor", "", "1.3.0"},
		{[]string{"v1.2.3"}, "major", "", "2.0.0"},
		{[]string{"v1.2.3-rc.1"}, "patch", "", "1.2.3"},
		{[]string{"v1.2.3"}, "minor", "rc", "1.3.0-rc.1"},
		{[]string{"v1.2.3", "v1.3.0-rc.1", "v1.3.0-rc.2"}, "patch", "rc", "1.3.0-rc.3"},
		{[]string{"v1.3.0-rc.1"}, "patch", "rc", "1.3.0-rc.2"},
	}
	for _, c := range cases {
		tags := mk(c.tags...)
		var latest *Version
		if l, ok := LatestTag(tags); ok {
			latest = &l.V
		}
		got, err := NextVersion(latest, c.kind, c.pre, tags)
		if err != nil || got.String() != c.want {
			t.Errorf("%v %s %s: got %v %v want %s", c.tags, c.kind, c.pre, got, err, c.want)
		}
	}
}

func TestDetectPrefix(t *testing.T) {
	tg := func(n string) Tag { x, _ := ParseTag(n); return x }
	if p, s := DetectPrefix(nil); p != "" || s != prefixNone {
		t.Error("none")
	}
	if p, s := DetectPrefix([]Tag{tg("v1.0.0"), tg("v1.1.0")}); p != "v" || s != prefixClear {
		t.Error("v")
	}
	if p, s := DetectPrefix([]Tag{tg("1.0.0"), tg("1.1.0")}); p != "" || s != prefixClear {
		t.Error("plain")
	}
	if _, s := DetectPrefix([]Tag{tg("1.0.0"), tg("v1.1.0")}); s != prefixMixed {
		t.Error("mixed")
	}
}

// --- end to end, against real git ---

type env struct {
	t      *testing.T
	work   string
	remote string
}

func (e *env) git(args ...string) string {
	e.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = e.work
	out, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (e *env) write(name, content string) {
	e.t.Helper()
	if err := os.WriteFile(filepath.Join(e.work, name), []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func newEnv(t *testing.T, config string) *env {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "T")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "T")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@example.com")
	dir := t.TempDir()
	e := &env{t: t, work: filepath.Join(dir, "work"), remote: filepath.Join(dir, "remote.git")}
	for _, d := range []string{e.work, e.remote} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("git", "init", "--bare", "-b", "main", e.remote)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	e.git("init", "-b", "main")
	e.git("remote", "add", "origin", e.remote)
	e.write("VERSION", "0\n")
	if config != "" {
		e.write(ConfigFile, config)
	}
	e.git("add", "-A")
	e.git("commit", "-m", "init")
	e.git("push", "-u", "origin", "main")
	return e
}

func (e *env) run(tty bool, stdin string, args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	a := &App{Cwd: e.work, In: bufio.NewReader(strings.NewReader(stdin)), Out: &out, Err: &errb, TTY: tty}
	code := a.Run(args)
	return code, out.String(), errb.String()
}

const testConfig = `pre = ["test -f VERSION"]
bump = ["echo $GIT_RELEASE_VERSION > VERSION"]
post = ["git push origin HEAD --follow-tags"]
`

func TestReleaseFlow(t *testing.T) {
	e := newEnv(t, testConfig)

	// first release, explicit version decides prefix and is saved
	if code, out, errs := e.run(false, "", "v0.1.0"); code != 0 {
		t.Fatalf("release failed: %s %s", out, errs)
	}
	if got := e.git("tag", "--list"); got != "v0.1.0" {
		t.Fatalf("tags = %q", got)
	}
	if got := e.git("show", "HEAD:VERSION"); got != "0.1.0" {
		t.Fatalf("VERSION = %q", got)
	}
	if !strings.Contains(e.git("show", "HEAD:"+ConfigFile), `tag_prefix = "v"`) {
		t.Fatal("prefix not committed to config")
	}
	if got := e.git("ls-remote", "--tags", "origin"); !strings.Contains(got, "refs/tags/v0.1.0") {
		t.Fatalf("tag not pushed: %q", got)
	}

	// dry run changes nothing
	before := e.git("rev-parse", "HEAD")
	if code, out, errs := e.run(false, "", "minor", "--dry-run"); code != 0 || !strings.Contains(out, "v0.2.0") {
		t.Fatalf("dry-run: %d %s %s", code, out, errs)
	}
	if e.git("rev-parse", "HEAD") != before || e.git("tag", "--list") != "v0.1.0" {
		t.Fatal("dry run mutated repo")
	}

	// no argument and no tty fails
	if code, _, errs := e.run(false, ""); code == 0 || !strings.Contains(errs, "not a terminal") {
		t.Fatalf("expected failure, got %d %s", code, errs)
	}

	// contradicting prefix
	if code, _, errs := e.run(false, "", "0.2.0"); code == 0 || !strings.Contains(errs, "contradicts") {
		t.Fatalf("expected prefix error, got %d %s", code, errs)
	}
	// not greater
	if code, _, _ := e.run(false, "", "v0.1.0"); code == 0 {
		t.Fatal("expected failure for same version")
	}

	// bump keyword, flags after the argument
	if code, out, errs := e.run(false, "", "patch", "--yes"); code != 0 {
		t.Fatalf("patch: %s %s", out, errs)
	}
	if got := e.git("describe", "--tags"); got != "v0.1.1" {
		t.Fatalf("describe = %q", got)
	}

	// dirty tree refused
	e.write("dirty", "x")
	if code, _, errs := e.run(false, "", "patch"); code == 0 || !strings.Contains(errs, "not clean") {
		t.Fatalf("expected dirty failure: %d %s", code, errs)
	}
	os.Remove(filepath.Join(e.work, "dirty"))

	// wrong branch refused
	e.git("checkout", "-q", "-b", "feature")
	if code, _, errs := e.run(false, "", "patch"); code == 0 || !strings.Contains(errs, "release branch") {
		t.Fatalf("expected branch failure: %d %s", code, errs)
	}
	e.git("checkout", "-q", "main")
}

func TestUnpushedRefused(t *testing.T) {
	e := newEnv(t, testConfig)
	e.write("f", "x")
	e.git("add", "f")
	e.git("commit", "-m", "local")
	if code, _, errs := e.run(false, "", "v0.1.0"); code == 0 || !strings.Contains(errs, "unpushed") {
		t.Fatalf("expected unpushed failure: %d %s", code, errs)
	}
}

func TestPreFailureStops(t *testing.T) {
	e := newEnv(t, "pre = [\"false\"]\n")
	if code, _, errs := e.run(false, "", "v0.1.0"); code == 0 || !strings.Contains(errs, "pre step failed") {
		t.Fatalf("got %d %s", code, errs)
	}
	if e.git("tag", "--list") != "" {
		t.Fatal("tag created despite failing pre")
	}
}

func TestPostFailureOffersUndo(t *testing.T) {
	e := newEnv(t, "bump = [\"echo $GIT_RELEASE_VERSION > VERSION\"]\npost = [\"false\"]\n")
	code, _, errs := e.run(false, "", "v0.1.0")
	if code == 0 {
		t.Fatal("expected failure")
	}
	if !strings.Contains(errs, "git tag -d v0.1.0") || !strings.Contains(errs, "git reset --hard HEAD~1") {
		t.Fatalf("recipe missing:\n%s", errs)
	}
	// non-interactive: nothing rolled back
	if e.git("tag", "--list") != "v0.1.0" {
		t.Fatal("tag should remain without a tty")
	}
	// undo needs --yes without tty
	if code, _, errs := e.run(false, "", "undo"); code == 0 || !strings.Contains(errs, "--yes") {
		t.Fatalf("undo without yes: %d %s", code, errs)
	}
	if code, out, errs := e.run(false, "", "undo", "--yes"); code != 0 {
		t.Fatalf("undo: %s %s", out, errs)
	}
	if e.git("tag", "--list") != "" || e.git("show", "HEAD:VERSION") != "0" {
		t.Fatal("undo did not restore state")
	}
}

func TestUndoRemoteNeedsForce(t *testing.T) {
	e := newEnv(t, testConfig)
	if code, out, errs := e.run(false, "", "v0.1.0"); code != 0 {
		t.Fatalf("%s %s", out, errs)
	}
	if code, _, errs := e.run(false, "", "undo", "--yes"); code == 0 || !strings.Contains(errs, "--force-remote") {
		t.Fatalf("expected refusal: %d %s", code, errs)
	}
	if code, out, errs := e.run(false, "", "undo", "--yes", "--force-remote"); code != 0 {
		t.Fatalf("%s %s", out, errs)
	}
	if e.git("ls-remote", "--tags", "origin") != "" {
		t.Fatal("remote tag still there")
	}
	if e.git("tag", "--list") != "" {
		t.Fatal("local tag still there")
	}
	// release commit was pushed: kept, with revert advice
	if e.git("show", "HEAD:VERSION") != "0.1.0" {
		t.Fatal("pushed commit must not be rewritten")
	}
}

func TestInteractiveFirstRelease(t *testing.T) {
	e := newEnv(t, "")
	// prefix question: v ; version question: default 0.1.0 ; confirm: y
	code, out, errs := e.run(true, "v\n\ny\n")
	if code != 0 {
		t.Fatalf("%d %s %s", code, out, errs)
	}
	if e.git("tag", "--list") != "v0.1.0" {
		t.Fatalf("tags = %q\n%s", e.git("tag", "--list"), out)
	}
}

func TestDetectedPrefixSaved(t *testing.T) {
	e := newEnv(t, "")
	e.git("tag", "-a", "1.0.0", "-m", "x")
	e.git("push", "origin", "1.0.0")
	if code, out, errs := e.run(false, "", "minor"); code != 0 {
		t.Fatalf("%s %s", out, errs)
	}
	if e.git("describe", "--tags") != "1.1.0" {
		t.Fatalf("describe = %q", e.git("describe", "--tags"))
	}
	if !strings.Contains(e.git("show", "HEAD:"+ConfigFile), `tag_prefix = ""`) {
		t.Fatal("detected prefix not saved")
	}
}

func TestMixedPrefixNeedsAnswer(t *testing.T) {
	e := newEnv(t, "")
	e.git("tag", "1.0.0")
	e.git("tag", "v1.1.0")
	if code, _, errs := e.run(false, "", "patch"); code == 0 || !strings.Contains(errs, "prefix") {
		t.Fatalf("%d %s", code, errs)
	}
	if code, out, errs := e.run(false, "", "patch", "--prefix=v"); code != 0 {
		t.Fatalf("%s %s", out, errs)
	}
	if e.git("describe", "--tags") != "v1.1.1" {
		t.Fatal(e.git("describe", "--tags"))
	}
}

func TestCompleteAndHelp(t *testing.T) {
	e := newEnv(t, "")
	for _, sh := range []string{"bash", "zsh", "fish"} {
		code, out, _ := e.run(false, "", "complete", sh)
		if code != 0 || !strings.Contains(out, "dry-run") {
			t.Errorf("%s completion broken", sh)
		}
	}
	if code, _, _ := e.run(false, "", "complete", "tcsh"); code == 0 {
		t.Error("unsupported shell accepted")
	}
	t.Setenv("SHELL", "/bin/bash")
	if code, out, _ := e.run(false, "", "complete"); code != 0 || !strings.Contains(out, "_git_release") {
		t.Error("default shell detection broken")
	}
	if code, out, _ := e.run(false, "", "help"); code != 0 || !strings.Contains(out, "BUMP EXAMPLES") {
		t.Error("help broken")
	}
}
