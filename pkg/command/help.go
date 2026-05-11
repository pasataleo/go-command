package command

import (
	"fmt"
	"sort"

	"github.com/pasataleo/go-config/pkg/flags"
)

func (commands *Commands) HelpCommand(command *Command, values map[string]map[string]interface{}) {
	// Build the path by walking up the parent chain.
	var path []string
	for current := command; current != nil; current = current.Parent {
		path = append(path, current.Name)
	}

	if len(command.Description) > 0 {
		commands.Println(command.Description)
		commands.Println()
	}

	commands.Println("Usage:")
	commands.Print(" ")
	for i := len(path) - 1; i >= 0; i-- {
		commands.Printf(" %s", path[i])
	}
	if command.Command == nil && len(command.Children) > 0 {
		commands.Print(" [command]")
	}
	commands.Println()

	// Commands
	if len(command.Children) > 0 {
		visible := map[string]string{}
		maxLen := 0
		for k, v := range command.Children {
			if v.Hidden {
				continue
			}

			visible[k] = v.Description
			if len(k) > maxLen {
				maxLen = len(k)
			}
		}

		if command == commands.Root {
			if _, ok := command.Children[versionCommand]; !ok {
				visible[versionCommand] = "Display version information"
				if len(versionCommand) > maxLen {
					maxLen = len(versionCommand)
				}
			}
			if _, ok := command.Children[completionCommand]; !ok {
				visible[completionCommand] = "Generate shell completion script"
				if len(completionCommand) > maxLen {
					maxLen = len(completionCommand)
				}
			}
		}
		if _, ok := command.Children[helpCommand]; !ok {
			visible[helpCommand] = "Display help information (i.e. print this message)"
			if len(helpCommand) > maxLen {
				maxLen = len(helpCommand)
			}
		}

		if len(visible) > 0 {
			names := make([]string, 0, len(visible))
			for k := range visible {
				names = append(names, k)
			}
			sort.Strings(names)

			commands.Println()
			commands.Println("Available subcommands:")
			for _, k := range names {
				v := visible[k]
				if len(v) > 0 {
					commands.Printf("  %-*s  %s\n", maxLen, k, v)
				} else {
					commands.Printf("  %s\n", k)
				}
			}
		}
	}

	// Flags
	{
		type flag struct {
			usage    string
			def      string
			required bool
		}

		flagsList := map[string]flag{}
		maxLen := 0
		for _, sources := range values {
			if _, ok := sources["hidden"]; ok {
				continue
			}
			flagValue, ok := sources["flags"].(flags.Flag)
			if !ok {
				continue
			}
			f := flag{}
			if u, ok := sources["usage"].(string); ok {
				f.usage = u
			}
			if d, ok := sources["defaults"].(string); ok {
				f.def = d
			}
			if _, ok := sources["required"].(string); ok {
				f.required = true
			}
			flagsList[flagValue.Name] = f
			if len(flagValue.Name) > maxLen {
				maxLen = len(flagValue.Name)
			}
		}

		if command == commands.Root {
			if _, ok := flagsList[versionCommand]; !ok {
				flagsList[versionCommand] = flag{
					usage: "Display version information",
					def:   "false",
				}
				if len(versionCommand) > maxLen {
					maxLen = len(versionCommand)
				}
			}
		}

		if _, ok := flagsList[helpCommand]; !ok {
			flagsList[helpCommand] = flag{
				usage: "Display help information (i.e. print this message)",
				def:   "false",
			}
			if len(helpCommand) > maxLen {
				maxLen = len(helpCommand)
			}
		}

		if len(flagsList) > 0 {
			flagNames := make([]string, 0, len(flagsList))
			for name := range flagsList {
				flagNames = append(flagNames, name)
			}
			sort.Strings(flagNames)

			commands.Println()
			commands.Println("Flags:")
			for _, name := range flagNames {
				f := flagsList[name]
				desc := f.usage
				if f.def != "" {
					if desc != "" {
						desc += " "
					}
					desc += fmt.Sprintf("(default: %q)", f.def)
				}
				if f.required {
					if desc != "" {
						desc += " "
					}
					desc += "(required)"
				}
				if desc != "" {
					commands.Printf("  --%-*s  %s\n", maxLen, name, desc)
				} else {
					commands.Printf("  --%s\n", name)
				}
			}
		}
	}

	// Environment variables
	{
		type envVar struct {
			name  string
			usage string
		}

		var envList []envVar
		maxLen := 0
		for _, sources := range values {
			if _, ok := sources["hidden"]; ok {
				continue
			}
			name, ok := sources["env"].(string)
			if !ok {
				continue
			}
			e := envVar{name: name}
			if u, ok := sources["usage"].(string); ok {
				e.usage = u
			}
			envList = append(envList, e)
			if len(e.name) > maxLen {
				maxLen = len(e.name)
			}
		}

		if len(envList) > 0 {
			sort.Slice(envList, func(i, j int) bool {
				return envList[i].name < envList[j].name
			})

			commands.Println()
			commands.Println("Environment variables:")
			for _, e := range envList {
				if e.usage != "" {
					commands.Printf("  %-*s  %s\n", maxLen, e.name, e.usage)
				} else {
					commands.Printf("  %s\n", e.name)
				}
			}
		}
	}
}
