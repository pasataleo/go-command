package command

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pasataleo/go-config/pkg/config"
)

const manCommand = "__man"

// ManPageTree generates man pages for all commands in the tree and writes them
// to dir. Files are named using hyphen-separated command paths, e.g.
// myapp.1, myapp-db.1, myapp-db-migrate.1.
func (commands *Commands) ManPageTree(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return writeManPages(dir, commands.Command, commands.Root, commands.Version, time.Now(), commands.docConfig())
}

// writeManPages takes the config rather than a prepared map so that every
// command in the tree derives its own metadata the same way, modules included.
func writeManPages(dir, prefix string, cmd *Command, version string, date time.Time, cfg *config.Config) error {
	content := manPage(prefix, cmd, version, date, findAll(cfg, cmd.Config))
	path := filepath.Join(dir, prefix+".1")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	for name, child := range cmd.Children {
		if !child.Hidden {
			if err := writeManPages(dir, prefix+"-"+name, child, version, date, cfg); err != nil {
				return err
			}
		}
	}
	return nil
}

func manPage(name string, cmd *Command, version string, date time.Time, found map[string]map[string]interface{}) string {
	var sb strings.Builder

	if version == "" {
		version = "dev"
	}

	// Title line
	fmt.Fprintf(&sb, ".TH %s 1 \"%s\" \"%s\" \"%s Manual\"\n",
		strings.ToUpper(strings.ReplaceAll(name, "-", "\\-")),
		date.Format("2006-01-02"),
		version,
		strings.ToUpper(strings.ReplaceAll(name, "-", "\\-")),
	)

	// NAME
	fmt.Fprintf(&sb, ".SH NAME\n")
	if cmd.Description != "" {
		fmt.Fprintf(&sb, "%s \\- %s\n", roffEscape(name), roffEscape(cmd.Description))
	} else {
		fmt.Fprintf(&sb, "%s\n", roffEscape(name))
	}

	// SYNOPSIS
	fmt.Fprintf(&sb, ".SH SYNOPSIS\n")
	fmt.Fprintf(&sb, ".B %s\n", roffEscape(name))
	if len(cmd.Children) > 0 {
		fmt.Fprintf(&sb, "[\\fIOPTIONS\\fR] [\\fICOMMAND\\fR] [\\fIARGS\\fR...]\n")
	} else {
		fmt.Fprintf(&sb, "[\\fIOPTIONS\\fR] [\\fIARGS\\fR...]\n")
	}

	// COMMANDS
	var childNames []string
	for n, child := range cmd.Children {
		if !child.Hidden {
			childNames = append(childNames, n)
		}
	}
	sort.Strings(childNames)

	if len(childNames) > 0 {
		fmt.Fprintf(&sb, ".SH COMMANDS\n")
		for _, n := range childNames {
			child := cmd.Children[n]
			fmt.Fprintf(&sb, ".TP\n")
			fmt.Fprintf(&sb, ".B %s\n", roffEscape(n))
			if child.Description != "" {
				fmt.Fprintf(&sb, "%s\n", roffEscape(child.Description))
			}
		}
	}

	// OPTIONS
	flagList := collectCompletionFlags(found)
	if len(flagList) > 0 {
		fmt.Fprintf(&sb, ".SH OPTIONS\n")
		for _, flag := range flagList {
			fmt.Fprintf(&sb, ".TP\n")
			fmt.Fprintf(&sb, "\\fB\\-\\-%s\\fR\n", roffEscape(flag.name))
			desc := manAnnotate(flag.desc, flag.def, flag.required)
			if desc != "" {
				fmt.Fprintf(&sb, "%s\n", roffEscape(desc))
			}
		}
	}

	// ENVIRONMENT
	envVars := collectCompletionEnvVars(found)
	if len(envVars) > 0 {
		fmt.Fprintf(&sb, ".SH ENVIRONMENT\n")
		for _, ev := range envVars {
			fmt.Fprintf(&sb, ".TP\n")
			fmt.Fprintf(&sb, "\\fB%s\\fR\n", roffEscape(ev.name))
			desc := manAnnotate(ev.desc, ev.def, ev.required)
			if desc != "" {
				fmt.Fprintf(&sb, "%s\n", roffEscape(desc))
			}
		}
	}

	return sb.String()
}

func manAnnotate(desc, def string, required bool) string {
	if def != "" {
		if desc != "" {
			desc += " "
		}
		desc += fmt.Sprintf("(default: %q)", def)
	}
	if required {
		if desc != "" {
			desc += " "
		}
		desc += "(required)"
	}
	return desc
}

func roffEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "-", "\\-")
	return s
}
