package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// App holds the I/O environment so the flow is testable.
type App struct {
	Cwd  string
	Repo *Repo
	In   *bufio.Reader
	Out  io.Writer
	Err  io.Writer
	TTY  bool
}

func (a *App) ask(question, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(a.Out, "%s [%s]: ", question, def)
	} else {
		fmt.Fprintf(a.Out, "%s: ", question)
	}
	line, err := a.In.ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("no input")
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

func (a *App) confirm(question string) (bool, error) {
	ans, err := a.ask(question+" (y/N)", "")
	if err != nil {
		return false, err
	}
	ans = strings.ToLower(ans)
	return ans == "y" || ans == "yes", nil
}

// parseArgs parses flags interspersed with positional arguments.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func (a *App) newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// Run executes the CLI and returns the exit code.
func (a *App) Run(args []string) int {
	err := a.run(args)
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return 0
	default:
		fmt.Fprintf(a.Err, "git-release: %v\n", err)
		return 1
	}
}

func (a *App) openRepo() error {
	r, err := OpenRepo(a.Cwd)
	a.Repo = r
	return err
}

func (a *App) run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "help", "-h", "--help":
			io.WriteString(a.Out, helpText)
			return nil
		case "complete":
			return a.complete(args[1:])
		case "config":
			return a.configCmd(args[1:])
		case "undo":
			return a.undoCmd(args[1:])
		}
	}
	return a.releaseCmd(args)
}

func (a *App) releaseCmd(args []string) error {
	var o releaseOpts
	fs := a.newFlagSet("release")
	fs.BoolVar(&o.DryRun, "dry-run", false, "")
	fs.BoolVar(&o.Yes, "yes", false, "")
	fs.BoolVar(&o.Yes, "y", false, "")
	fs.BoolVar(&o.NoPre, "no-pre", false, "")
	fs.BoolVar(&o.Sign, "sign", false, "")
	fs.StringVar(&o.Branch, "branch", "", "")
	fs.StringVar(&o.Pre, "pre", "", "")
	fs.Func("prefix", "", func(s string) error {
		if s != "" && s != "v" {
			return errors.New(`must be "v" or ""`)
		}
		o.Prefix = &s
		return nil
	})
	pos, err := parseArgs(fs, args)
	if err != nil {
		return a.usageErr(err)
	}
	if len(pos) > 1 {
		return a.usageErr(fmt.Errorf("unexpected argument %q", pos[1]))
	}
	if len(pos) == 1 {
		o.Arg = pos[0]
	}
	if err := a.openRepo(); err != nil {
		return err
	}
	return a.Release(o)
}

func (a *App) usageErr(err error) error {
	return fmt.Errorf("%w (see: git release help)", err)
}

func (a *App) undoCmd(args []string) error {
	var o undoOpts
	fs := a.newFlagSet("undo")
	fs.BoolVar(&o.Yes, "yes", false, "")
	fs.BoolVar(&o.Yes, "y", false, "")
	fs.BoolVar(&o.DryRun, "dry-run", false, "")
	fs.BoolVar(&o.ForceRemote, "force-remote", false, "")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return a.usageErr(err)
	}
	if len(pos) > 1 {
		return a.usageErr(fmt.Errorf("unexpected argument %q", pos[1]))
	}
	if len(pos) == 1 {
		o.Tag = pos[0]
	}
	if err := a.openRepo(); err != nil {
		return err
	}
	return a.Undo(o)
}

func (a *App) configCmd(args []string) error {
	var initFile bool
	fs := a.newFlagSet("config")
	fs.BoolVar(&initFile, "init", false, "")
	if _, err := parseArgs(fs, args); err != nil {
		return a.usageErr(err)
	}
	if err := a.openRepo(); err != nil {
		return err
	}
	path := filepath.Join(a.Repo.Dir, ConfigFile)
	if initFile {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists", path)
		}
		if err := os.WriteFile(path, []byte(starterConfig), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(a.Out, "wrote %s (commit it)\n", path)
		return nil
	}
	cfg, exists, err := LoadConfig(a.Repo.Dir)
	if err != nil {
		return err
	}
	w := a.Out
	if exists {
		fmt.Fprintf(w, "config file: %s\n", path)
	} else {
		fmt.Fprintf(w, "config file: none (%s missing; `git release config --init` creates one)\n", ConfigFile)
	}
	branch, src := cfg.Branch, "config"
	if branch == "" {
		src = "default"
		if b, err := a.Repo.DefaultBranch(); err == nil {
			branch = b
		} else {
			branch = "(undetermined)"
		}
	}
	fmt.Fprintf(w, "branch:      %s (%s)\n", branch, src)
	tags, _ := a.Repo.SemverTags()
	switch {
	case cfg.TagPrefix != nil:
		fmt.Fprintf(w, "tag_prefix:  %q (config)\n", *cfg.TagPrefix)
	default:
		d, state := DetectPrefix(tags)
		switch state {
		case prefixClear:
			fmt.Fprintf(w, "tag_prefix:  %q (detected, saved on next release)\n", d)
		case prefixMixed:
			fmt.Fprintln(w, "tag_prefix:  mixed styles in existing tags (will ask)")
		default:
			fmt.Fprintln(w, "tag_prefix:  unset, no tags yet (will ask)")
		}
	}
	fmt.Fprintf(w, "sign:        %v\n", cfg.Sign)
	fmt.Fprintf(w, "annotated:   %v\n", cfg.annotated())
	for _, l := range []struct {
		name string
		cmds []string
	}{{"pre", cfg.Pre}, {"bump", cfg.Bump}, {"post", cfg.Post}} {
		if len(l.cmds) == 0 {
			fmt.Fprintf(w, "%-12s (none)\n", l.name+":")
			continue
		}
		fmt.Fprintf(w, "%s:\n", l.name)
		for _, c := range l.cmds {
			fmt.Fprintf(w, "  - %s\n", c)
		}
	}
	return nil
}
