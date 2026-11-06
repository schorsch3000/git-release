package main

import (
	"bufio"
	"os"

	"golang.org/x/term"
)

func main() {
	cwd, _ := os.Getwd()
	app := &App{
		Cwd: cwd,
		In:  bufio.NewReader(os.Stdin),
		Out: os.Stdout,
		Err: os.Stderr,
		TTY: term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())),
	}
	os.Exit(app.Run(os.Args[1:]))
}
