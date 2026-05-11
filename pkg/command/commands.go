package command

import (
	"os"
	"time"

	"github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-config/pkg/defaults"
	"github.com/pasataleo/go-config/pkg/env"
	"github.com/pasataleo/go-config/pkg/flags"
	"github.com/pasataleo/go-config/pkg/hidden"
	"github.com/pasataleo/go-config/pkg/inject"
	"github.com/pasataleo/go-config/pkg/required"
	"github.com/pasataleo/go-config/pkg/usage"
	"github.com/pasataleo/go-errorsx/pkg/errorsx"
)

const (
	helpCommand       = "help"
	versionCommand    = "version"
	completionCommand = "completion"
)

type OptionsFn func(*Commands)

func StdOut(out *os.File) OptionsFn {
	return func(commands *Commands) {
		commands.Out = out
	}
}

func StdErr(err *os.File) OptionsFn {
	return func(commands *Commands) {
		commands.Err = err
	}
}

func Version(v string) OptionsFn {
	return func(commands *Commands) {
		commands.Version = v
	}
}

type Commands struct {
	Command string
	Root    *Command

	Version string

	Out *os.File
	Err *os.File
}

func Build(command string, commands interface{}, opts ...OptionsFn) *Commands {
	root := parseCommands(command, commands)
	cmd := &Commands{
		Command: command,
		Root:    root,
		Out:     os.Stdout,
		Err:     os.Stderr,
	}
	for _, opt := range opts {
		opt(cmd)
	}
	return cmd
}

func (commands *Commands) Execute(args []string, injector *inject.Source) error {
	// Scan for __complete before flags.Parse so a partial flag-like word isn't
	// consumed. Everything before __complete is the navigation path; the one
	// word after it (if any) is the partial word being typed.
	isComplete := false
	var completionCurrent string
	for i, arg := range args {
		if arg == "__complete" {
			isComplete = true
			if i+1 < len(args) {
				completionCurrent = args[i+1]
			}
			args = args[:i]
			break
		}
	}

	var values map[string][]string
	args, values = flags.Parse(args)
	flagsSource := flags.NewSource(values)

	command := commands.Root
	current := commands.Command
	for len(args) > 0 {
		current = args[0]
		if current == "--" {
			// the '--' indicates to stop processing the command tree, and pass the remaining values to the command.
			args = args[1:]
			break
		}
		next, ok := command.Children[current]
		if !ok {
			// then we have no subcommand by this name, so we'll stop and pass the remaining values on.
			break
		}

		args = args[1:]
		command = next
	}

	cfg := config.New(commands.defaultSources(injector, flagsSource)...)

	var found map[string]map[string]interface{}
	if command.Config != nil {
		found = cfg.Find(command.Config)
	}

	if isComplete {
		for _, c := range commands.complete(command, completionCurrent, found) {
			commands.Println(c)
		}
		return nil
	}

	if command.Config != nil {
		knownFlags := config.FilterBySource[flags.Flag](found, "flags")
		unclaimedFlags := flagsSource.Validate(knownFlags)
		if len(unclaimedFlags) > 0 {
			var err error
			for unclaimedFlag, suggestion := range unclaimedFlags {
				if unclaimedFlag == "help" || (unclaimedFlag == "version" && command == commands.Root) {
					continue
				}
				err = errorsx.Append(err, unknownFlag(unclaimedFlag, suggestion))
			}
			if err != nil {
				return err
			}
		}
	} else {
		unclaimedFlags := flagsSource.Validate(nil)
		if len(unclaimedFlags) > 0 {
			var err error
			for unclaimedFlag, suggestion := range unclaimedFlags {
				if unclaimedFlag == "help" || (unclaimedFlag == "version" && command == commands.Root) {
					continue
				}
				err = errorsx.Append(err, unknownFlag(unclaimedFlag, suggestion))
			}
			if err != nil {
				return err
			}
		}
	}

	if _, ok := values[helpCommand]; ok || (len(args) > 0 && args[0] == helpCommand) {
		commands.HelpCommand(command, found)
		return nil
	}
	if command == commands.Root && len(commands.Version) > 0 {
		if _, ok := values[versionCommand]; ok || (len(args) > 0 && args[0] == versionCommand) {
			commands.VersionCommand()
			return nil
		}
	}

	if command == commands.Root && len(args) > 0 && args[0] == completionCommand {
		if len(args) < 2 {
			commands.EPrintf("usage: %s completion [bash|zsh|fish]\n", commands.Command)
			return nil
		}
		script, err := commands.CompletionScript(args[1])
		if err != nil {
			return err
		}
		commands.Print(script)
		return nil
	}

	if command == commands.Root && len(args) > 0 && args[0] == manCommand {
		if len(args) < 2 {
			commands.EPrintf("usage: %s __man <dir>\n", commands.Command)
			return nil
		}
		if err := os.MkdirAll(args[1], 0755); err != nil {
			return err
		}
		return writeManPages(args[1], commands.Command, commands.Root, commands.Version, time.Now(), found)
	}

	if command.Command != nil {
		return command.Execute(args, cfg)
	}

	if len(args) > 0 || len(values) > 0 {
		return unknownCommand(current, command.Children)
	}

	commands.HelpCommand(command, found)
	return nil
}

func (commands *Commands) defaultSources(injector *inject.Source, flags *flags.Source) []config.Source {
	if injector == nil {
		injector = inject.New()
	}
	return []config.Source{
		flags,
		new(env.Source),
		injector,
		new(hidden.Source),
		new(usage.Source),
		new(defaults.Source),
		new(required.Source),
	}
}

func parseCommands(name string, next interface{}) *Command {
	if rest, ok := next.(map[string]interface{}); ok {
		var command *Command
		var children map[string]*Command
		for k, v := range rest {
			if k == "__complete" {
				panic("__complete is reserved for shell completion and cannot be used as a command name")
			}
			if k == "__man" {
				panic("__man is reserved for man page generation and cannot be used as a command name")
			}
			if k == "" {
				command = parseCommand(name, v)
				continue
			}
			if children == nil {
				children = make(map[string]*Command)
			}
			children[k] = parseCommands(k, v)
		}
		if command == nil {
			command = new(Command)
		}
		for _, child := range children {
			child.Parent = command
		}

		if command.Children == nil {
			command.Children = children
		} else {
			for key, child := range children {
				command.Children[key] = child
			}
		}

		return command
	}
	return parseCommand(name, next)
}
