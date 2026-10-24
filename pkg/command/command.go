package command

import (
	"slices"

	"github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-config/pkg/didyoumean"
	"github.com/pasataleo/go-config/pkg/flags"
	"github.com/pasataleo/go-errorsx/pkg/errorsx"
)

type Executable interface {
	Execute(args []string) error
}

type Hidden interface {
	Hidden() bool
}

type Describable interface {
	Description() string
}

type WithChildren interface {
	Children() map[string]interface{}
}

// WithUnclaimedFlags is implemented by a command that accepts flags beyond its
// config. Flags no config field claims are not rejected; they are handed to
// UnclaimedFlags, by name with every value given, once the config is assigned
// and before the command runs. It returns the names it does not recognise
// either, which are reported as unknown flags, and an error stops the command.
type WithUnclaimedFlags interface {
	UnclaimedFlags(flags map[string][]string) (unknown []string, err error)
}

// WithFlagDocs documents the flags a command accepts beyond its config, for
// help, completion and man pages, and as "did you mean" candidates. Like
// Describable, it is called once, when the command is built.
type WithFlagDocs interface {
	FlagDocs() []FlagDoc
}

// FlagDoc is how a flag is documented in help, completion and man pages. It
// embeds the flags.Flag go-config reports for a config's flags: Name without
// the leading --, and Type, where a bool takes no value.
//
// go-command builds one for every config flag from its tags. A command
// documents flags beyond its config with WithFlagDocs, and for those Default
// and Requirement are documentation only: the command handles the flags
// itself, so it is the one that applies a default or enforces a requirement.
type FlagDoc struct {
	flags.Flag
	Usage       string
	Default     string // shown as (default: "...")
	Requirement string // shown as (...): "required", or "required: one of --port, --socket"
}

type Command struct {
	Name        string
	Description string
	Hidden      bool

	Config interface{}

	Command func([]string) error

	// UnclaimedFlags and FlagDocs are the fields WithUnclaimedFlags and
	// WithFlagDocs fill. FlagDocs needs UnclaimedFlags: a documented flag with
	// nowhere to go would be accepted and dropped.
	UnclaimedFlags func(map[string][]string) ([]string, error)
	FlagDocs       []FlagDoc

	Parent   *Command
	Children map[string]*Command
}

func (command *Command) Execute(args []string, config *config.Config) error {
	return command.run(args, config, nil, nil)
}

// run assigns the config, hands the command the flags its config left
// unclaimed, and runs it. known names every flag the command accepts, as
// candidates for suggesting a name the command did not recognise.
func (command *Command) run(args []string, config *config.Config, unclaimed map[string][]string, known []string) error {
	if command.Config != nil {
		if err := config.Assign(command.Config); err != nil {
			return err
		}
	}

	if command.UnclaimedFlags != nil {
		unknown, err := command.UnclaimedFlags(unclaimed)
		if err != nil {
			return err
		}

		var errs error
		for _, name := range unknown {
			errs = errorsx.Append(errs, unknownFlag(name, didyoumean.Suggest(name, slices.Values(known))))
		}
		if errs != nil {
			return errs
		}
	}

	return command.Command(args)
}

// check panics on a command that cannot work as built.
func (command *Command) check() *Command {
	if command != nil && len(command.FlagDocs) > 0 && command.UnclaimedFlags == nil {
		panic("command " + command.Name + " documents flags with FlagDocs but has no UnclaimedFlags to receive them")
	}
	return command
}

func parseCommand(name string, cmd interface{}) *Command {
	if cmd == nil {
		return nil
	}

	if command, ok := cmd.(*Command); ok {
		return command.check()
	}
	if command, ok := cmd.(Command); ok {
		return command.check()
	}

	if description, ok := cmd.(string); ok {
		return &Command{
			Name:        name,
			Description: description,
		}
	}

	if fn, ok := cmd.(func([]string) error); ok {
		return &Command{
			Name:    name,
			Command: fn,
		}
	}

	command := Command{
		Name:   name,
		Config: cmd,
	}

	if executable, ok := cmd.(Executable); ok {
		command.Command = executable.Execute
	}
	if description, ok := cmd.(Describable); ok {
		command.Description = description.Description()
	}
	if hidden, ok := cmd.(Hidden); ok {
		command.Hidden = hidden.Hidden()
	}
	if receiver, ok := cmd.(WithUnclaimedFlags); ok {
		command.UnclaimedFlags = receiver.UnclaimedFlags
	}
	if docs, ok := cmd.(WithFlagDocs); ok {
		command.FlagDocs = docs.FlagDocs()
	}
	if withChildren, ok := cmd.(WithChildren); ok {
		for k, v := range withChildren.Children() {
			if command.Children == nil {
				command.Children = make(map[string]*Command)
			}
			command.Children[k] = parseCommands(k, v)
		}
	}

	return command.check()
}
