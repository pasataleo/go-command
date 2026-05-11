package command

import "github.com/pasataleo/go-config/pkg/config"

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

type Command struct {
	Name        string
	Description string
	Hidden      bool

	Config interface{}

	Command func([]string) error

	Parent   *Command
	Children map[string]*Command
}

func (command *Command) Execute(args []string, config *config.Config) error {
	if command.Config != nil {
		if err := config.Assign(command.Config); err != nil {
			return err
		}
	}

	return command.Command(args)
}

func parseCommand(name string, cmd interface{}) *Command {
	if cmd == nil {
		return nil
	}

	if command, ok := cmd.(*Command); ok {
		return command
	}
	if command, ok := cmd.(Command); ok {
		return &command
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
	if withChildren, ok := cmd.(WithChildren); ok {
		for k, v := range withChildren.Children() {
			if command.Children == nil {
				command.Children = make(map[string]*Command)
			}
			command.Children[k] = parseCommands(k, v)
		}
	}

	return &command
}
