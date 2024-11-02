package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/pasataleo/go-errors/errors"
	"github.com/pasataleo/go-flags/flags"
	"github.com/pasataleo/go-inject/inject"

	"github.com/pasataleo/go-command/command"
)

func main() {
	cmd := command.New("example", "This command demonstrates the use of the go-command library.")

	createEchoCommand(cmd)

	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func createEchoCommand(parent *command.Command) {
	var lower bool

	cmd, err := parent.Add("echo", "Echo a string.")
	if err != nil {
		panic(fmt.Errorf("failed to create echo command: %w", err))
	}

	cmd.Fn = func(command *command.Command, args []string) error {
		upper := cmd.GetUnsafe("upper").(bool)

		if upper && lower {
			return errors.New(nil, errors.ErrorCodeUnknown, "cannot set both upper and lower")
		}

		switch {
		case upper:
			command.Println(strings.ToUpper(args[0]))
		case lower:
			command.Println(strings.ToLower(args[0]))
		default:
			command.Println(args[0])
		}

		return nil
	}

	flags.BindBoolean("upper", "Convert returned string to upper case.", true, false).ToFunction(cmd.LocalFlags, inject.DirectBinder[bool](cmd.Injector))
	flags.BindBoolean("lower", "Convert returned string to lower case.", true, false).ToValue(cmd.LocalFlags, &lower)
}
