package command

import "fmt"

func (cmd *Command) Println(args ...any) {
	fmt.Fprintln(cmd.Stdout, args...)
}

func (cmd *Command) Printf(format string, args ...any) {
	fmt.Fprintf(cmd.Stdout, format, args...)
}

func (cmd *Command) Print(args ...any) {
	fmt.Fprint(cmd.Stdout, args...)
}

func (cmd *Command) Eprintln(args ...any) {
	fmt.Fprintln(cmd.Stderr, args...)
}

func (cmd *Command) Eprintf(format string, args ...any) {
	fmt.Fprintf(cmd.Stderr, format, args...)
}

func (cmd *Command) Eprint(args ...any) {
	fmt.Fprint(cmd.Stderr, args...)
}
