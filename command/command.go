package command

import (
	"strings"

	"github.com/pasataleo/go-errors/errors"
	"github.com/pasataleo/go-flags/flags"
	"github.com/pasataleo/go-inject/inject"
)

type Function func(command *Command, args []string) error

type Command struct {
	Name        string
	Description string

	GlobalFlags *flags.Set
	LocalFlags  *flags.Set
	Injector    *inject.Injector

	Fn       Function
	Children map[string]*Command

	Parent *Command
}

func New(name string, description string) *Command {
	return create(name, description, inject.NewInjector(), true)
}

func create(name string, description string, injector *inject.Injector, makeHelp bool) *Command {
	cmd := &Command{
		Name:        name,
		Description: description,
		GlobalFlags: flags.NewSet(),
		LocalFlags:  flags.NewSet(),
		Injector:    injector,
		Children:    make(map[string]*Command),
	}

	if makeHelp {
		help, err := cmd.add("help", "Display usage information for this command.", false)
		if err != nil {
			// This should never happen, this is the first child we're adding so nothing should exist yet.
			panic("command: failed to create help command")
		}

		help.Fn = cmd.help()
	}

	return cmd
}

func (cmd *Command) Add(name string, description string) (*Command, error) {
	return cmd.add(name, description, true)
}

func (cmd *Command) add(name string, description string, makeHelp bool) (*Command, error) {
	if _, ok := cmd.Children[name]; ok {
		return nil, errors.Newf(nil, errors.ErrorCodeUnknown, "command %s already exists", name)
	}

	child := create(name, description, cmd.Injector, makeHelp)
	child.Parent = cmd
	cmd.Children[name] = child
	return child, nil
}

func (cmd *Command) Execute(args []string) error {

	skipNext := false
	for ix, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		// Since users could interleave flags and arguments, we need to check if the current argument is a flag.
		if strings.HasPrefix(arg, "-") {
			// If this argument is a flag, and doesn't contain a value then we assume the next entry in the args is the
			// value, and we shouldn't process it as a command.
			skipNext = !strings.Contains(arg, "=")
			continue
		}

		// Then the argument is a command, so we should see if we have a child command that matches.
		if child, ok := cmd.Children[arg]; ok {
			return child.Execute(append(args[:ix], args[ix+1:]...))
		}

		// If we don't have a child command that matches, then we should execute the current command.
		break
	}

	if cmd.Fn != nil {
		var err error

		// First, we'll parse the global flags from all our parents.
		for parent := cmd.Parent; parent != nil; parent = parent.Parent {
			var parentErr error
			args, parentErr = parent.GlobalFlags.Parse(args)
			err = errors.Append(err, parentErr)
		}

		// Next, we'll parse the local flags for this command.
		var argsErr error
		args, argsErr = cmd.LocalFlags.Parse(args)
		err = errors.Append(err, argsErr)

		if err != nil {
			return err
		}

		return cmd.Fn(cmd, args)
	} else {
		return cmd.Children["help"].Execute(args)
	}
}
