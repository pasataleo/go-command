package command

import (
	"fmt"
	"os"
	"time"

	"github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-config/pkg/defaults"
	"github.com/pasataleo/go-config/pkg/env"
	"github.com/pasataleo/go-config/pkg/flags"
	"github.com/pasataleo/go-config/pkg/hidden"
	"github.com/pasataleo/go-config/pkg/inject"
	"github.com/pasataleo/go-config/pkg/oneof"
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

// Version sets the version version and --version report, instead of the one
// the binary was built with.
func Version(v string) OptionsFn {
	return func(commands *Commands) {
		commands.Version = v
	}
}

// Injector sets the injector that inject tags are resolved against. It belongs
// to the Commands rather than to a call, so that documentation generated
// without running anything - ManPageTree - describes the same flags and
// environment variables that Execute would accept, modules included.
func Injector(injector *inject.Source) OptionsFn {
	return func(commands *Commands) {
		commands.Injector = injector
	}
}

type Commands struct {
	Command string
	Root    *Command

	Version string

	Injector *inject.Source

	Out *os.File
	Err *os.File
}

func Build(command string, commands interface{}, opts ...OptionsFn) *Commands {
	root := parseCommands(command, commands)
	cmd := &Commands{
		Command: command,
		Root:    root,
		Version: buildVersion(),
		Out:     os.Stdout,
		Err:     os.Stderr,
	}
	for _, opt := range opts {
		opt(cmd)
	}
	return cmd
}

func (commands *Commands) Execute(args []string) error {
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

	cfg := config.New(commands.defaultSources(flagsSource)...)

	found := findAll(cfg, command.Config)

	if isComplete {
		for _, c := range commands.complete(command, completionCurrent, found) {
			commands.Println(c)
		}
		return nil
	}

	// Only the config's flags count as claimed: a command's documented flags
	// are the ones it takes through UnclaimedFlags, so they stay unclaimed.
	knownFlags := config.FilterBySource[flags.Flag](found, "flags")
	unclaimedFlags := flagsSource.Validate(knownFlags)
	unclaimed := make(map[string][]string)
	if len(unclaimedFlags) > 0 {
		var err error
		for unclaimedFlag, suggestion := range unclaimedFlags {
			if unclaimedFlag == "help" || (unclaimedFlag == "version" && command == commands.Root) {
				continue
			}
			if command.UnclaimedFlags != nil {
				// The command takes these itself, and reports any it does not
				// recognise once it has seen them.
				unclaimed[unclaimedFlag] = values[unclaimedFlag]
				continue
			}
			err = errorsx.Append(err, unknownFlag(unclaimedFlag, suggestion))
		}
		if err != nil {
			return err
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
		return writeManPages(args[1], commands.Command, commands.Root, commands.Version, time.Now(), cfg)
	}

	if command.Command != nil {
		return command.run(args, cfg, unclaimed, flagNames(knownFlags, command.FlagDocs))
	}

	if len(args) > 0 || len(values) > 0 {
		return unknownCommand(current, command.Children)
	}

	commands.HelpCommand(command, found)
	return nil
}

func (commands *Commands) defaultSources(flags *flags.Source) []config.Source {
	return []config.Source{
		// oneof first, so it sees every field before a source claims it.
		new(oneof.Source),
		flags,
		new(env.Source),
		commands.injector(),
		new(hidden.Source),
		new(usage.Source),
		new(defaults.Source),
		new(required.Source),
	}
}

// injector returns the injector Build was given, or an empty one. The inject
// source is always in the list, so it has to be non-nil even when the program
// wires nothing up.
func (commands *Commands) injector() *inject.Source {
	if commands.Injector != nil {
		return commands.Injector
	}
	return inject.New()
}

// docConfig builds the sources Execute would, with an empty flag source, to
// report what the commands accept without running anything. Find reads struct
// tags rather than parsed values, and it never calls Start and never opens a
// scope, so no module is installed and nothing is bound.
func (commands *Commands) docConfig() *config.Config {
	return config.New(commands.defaultSources(new(flags.Source))...)
}

// findAll collects the metadata for a command's own config struct together with
// the structs the sources bring along themselves - the modules registered on the
// injector, which carry flags and environment variables of their own but which
// nothing the caller owns points at. Find does not walk those, so help text,
// completion, man pages and flag validation would all miss them.
func findAll(cfg *config.Config, target interface{}) map[string]map[string]interface{} {
	found := make(map[string]map[string]interface{})
	merge := func(m map[string]map[string]interface{}, prefix string) {
		for path, sources := range m {
			found[prefix+path] = sources
		}
	}
	if target != nil {
		merge(cfg.Find(target), "")
	}
	for i, target := range cfg.AdditionalTargets() {
		// Paths are relative to each target's own root, so they can collide
		// across targets. Only the values are ever read, never the key.
		merge(cfg.Find(target), fmt.Sprintf("%d\x00", i))
	}
	return found
}

// flagNames names every flag a command accepts: its config's, and those it
// documents itself.
func flagNames(config map[string]flags.Flag, documented []FlagDoc) []string {
	names := make([]string, 0, len(config)+len(documented))
	for _, flag := range config {
		names = append(names, flag.Name)
	}
	for _, doc := range documented {
		names = append(names, doc.Name)
	}
	return names
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
			command = &Command{Name: name}
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
