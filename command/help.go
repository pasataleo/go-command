package command

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/pasataleo/go-flags/flags"
)

func (cmd *Command) help() Function {
	return func(_ *Command, _ []string) error {
		var buffer bytes.Buffer

		usage := cmd.Name
		for parent := cmd.Parent; parent != nil; parent = parent.Parent {
			usage = parent.Name + " " + usage
		}

		buffer.WriteString(fmt.Sprintf("Usage: %s", usage))
		if len(cmd.LocalFlags.Flags) > 0 {
			buffer.WriteString(fmt.Sprintln(" [flags]"))
		} else {
			buffer.WriteString(fmt.Sprintln())
		}

		if len(cmd.Description) > 0 {
			buffer.WriteString(fmt.Sprintln())
			buffer.WriteString(fmt.Sprintln(cmd.Description))
		}

		buffer.WriteString(cmd.localFlags())
		buffer.WriteString(cmd.globalFlags())
		buffer.WriteString(cmd.commands())

		fmt.Println(buffer.String())
		return nil
	}
}

func (cmd *Command) globalFlags() string {
	var buffer bytes.Buffer

	names, length := cmd.globalFlagsMeta()
	sort.Strings(names)
	for _, name := range names {
		buffer.WriteString(flag(length, name, cmd.globalFlag(name)))
	}

	globalFlags := buffer.String()
	if len(globalFlags) > 0 {
		return fmt.Sprintf("\nGlobal flags:\n%s", globalFlags)
	}
	return ""
}

func (cmd *Command) globalFlag(name string) *flags.Flag[any] {
	if flag, ok := cmd.GlobalFlags.Flags[name]; ok {
		return flag
	}

	if cmd.Parent != nil {
		return cmd.Parent.globalFlag(name)
	}

	return nil
}

func (cmd *Command) globalFlagsMeta() (names []string, length int) {
	for name, flag := range cmd.GlobalFlags.Flags {
		rendered := fmt.Sprintf("--%s=%T", name, flag.Default)
		if len(rendered) > length {
			length = len(rendered)
		}
		names = append(names, name)
	}

	if cmd.Parent != nil {
		parentNames, parentLength := cmd.Parent.globalFlagsMeta()
		if parentLength > length {
			length = parentLength
		}
		names = append(names, parentNames...)
	}

	return names, length
}

func (cmd *Command) localFlags() string {
	var buffer bytes.Buffer

	if len(cmd.LocalFlags.Flags) > 0 {
		buffer.WriteString(fmt.Sprintln())
		buffer.WriteString(fmt.Sprintln("Local flags:"))

		length := 0
		var names []string
		for name, flag := range cmd.LocalFlags.Flags {
			rendered := fmt.Sprintf("--%s=%T", name, flag.Default)
			if len(rendered) > length {
				length = len(rendered)
			}
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			buffer.WriteString(flag(length, name, cmd.LocalFlags.Flags[name]))
		}
	}

	return buffer.String()
}

func (cmd *Command) commands() string {
	var buffer bytes.Buffer

	if len(cmd.Children) > 0 {
		buffer.WriteString(fmt.Sprintln())
		buffer.WriteString(fmt.Sprintln("Commands:"))

		length := 0
		var names []string
		for name := range cmd.Children {
			if len(name) > length {
				length = len(name)
			}
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			child := cmd.Children[name]
			buffer.WriteString(fmt.Sprintf("  %-*s", length, name))
			if len(child.Description) > 0 {
				buffer.WriteString(fmt.Sprintf(" %s", child.Description))
			}
			buffer.WriteString(fmt.Sprintln())
		}
	}

	return buffer.String()
}

func flag(length int, name string, flag *flags.Flag[any]) string {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("  %-*s", length, fmt.Sprintf("--%s=%T", name, flag.Default)))
	if len(flag.Description) > 0 {
		buffer.WriteString(fmt.Sprintf(" %s", flag.Description))
	}
	if len(flag.Aliases) > 0 {
		buffer.WriteString(fmt.Sprintf(", aliases: %s", strings.Join(flag.Aliases, ", ")))
	}
	if flag.Optional {
		buffer.WriteString(fmt.Sprintf(", default: %v", flag.Default))
	}
	buffer.WriteString(fmt.Sprintln())
	return buffer.String()
}
