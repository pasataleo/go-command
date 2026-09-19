# go-command

A Go library for building CLI applications with subcommands, flags, environment variables, dependency injection, and auto-generated help.

## Installation

```sh
go get github.com/pasataleo/go-command
```

## Quick start

```go
package main

import (
    "fmt"
    "os"

    "github.com/pasataleo/go-command/pkg/command"
)

func main() {
    cmds := command.Build("myapp", map[string]interface{}{
        "": "My CLI tool",
        "greet": &command.Command{
            Description: "Greet someone",
            Config: &struct {
                Name string `flag:"name" default:"world" usage:"Who to greet"`
            }{},
            Command: func(args []string) error {
                fmt.Println("Hello!")
                return nil
            },
        },
    }, command.Version("0.1.0"))

    if err := cmds.Execute(os.Args[1:]); err != nil {
        if err = cmds.FormatErrors(err); err != nil {
            fmt.Fprintln(os.Stderr, err)
        }
        os.Exit(1)
    }
}
```

## Defining commands

Commands are defined as a `map[string]interface{}` tree. The empty key `""` defines the command at the current level; all other keys define subcommands.

### String shortcut

A plain string creates a description-only command (used for groups that just show help):

```go
command.Build("app", map[string]interface{}{
    "": "My CLI tool",
})
```

### Function shortcut

A `func([]string) error` is used directly as the command handler:

```go
command.Build("app", map[string]interface{}{
    "": func(args []string) error {
        fmt.Println("Hello!")
        return nil
    },
})
```

### Command struct

Use `command.Command` for full control over config, description, and visibility:

```go
command.Build("app", map[string]interface{}{
    "": &command.Command{
        Description: "A greeting tool",
        Config:      cfg,
        Command:     func(args []string) error { return nil },
        Hidden:      true,
    },
})
```

### Interface-based commands

Implement `Executable`, `Describable`, and/or `Hidden` on a struct:

```go
type myCommand struct {
    Name string `flag:"name" default:"world"`
}

func (c *myCommand) Execute(args []string) error { return nil }
func (c *myCommand) Description() string         { return "My command" }
```

Implement `WithChildren` to nest subcommands inside a struct rather than in the raw map:

```go
type dbCommands struct{}

func (d *dbCommands) Description() string { return "Database commands" }

func (d *dbCommands) Children() map[string]interface{} {
    return map[string]interface{}{
        "migrate": func(args []string) error { return nil },
        "seed":    func(args []string) error { return nil },
    }
}

command.Build("app", map[string]interface{}{
    "":   "My CLI tool",
    "db": &dbCommands{},
})
```

## Config

Config structs are populated automatically from multiple sources, in priority order:

1. **Flags** (`flag` tag) — `--name value` or `--name=value`
2. **Environment variables** (`env` tag)
3. **Dependency injection** (`inject` tag)
4. **Defaults** (`default` tag)

Additional tags:

- `required:"--name"` — fail with an error if the value is not set; the tag value is included in the error message
- `usage:"description"` — description shown in help output
- `hidden:""` — hide the field from help output

```go
type Config struct {
    Host  string `flag:"host" env:"HOST" default:"localhost" usage:"Server host"`
    Port  int    `flag:"port" env:"PORT" default:"3000" usage:"Server port"`
    Token string `env:"TOKEN" required:"TOKEN env var" hidden:""`
}
```

### Bool negation

Bool flags with a `true` default can be negated with `--no-<flag>`:

```sh
myapp --no-color
```

## Dependency injection

Fields tagged `inject` are populated from an `*inject.Source`, supplied once with the `Injector` option:

```go
injector := inject.New()
injector.Bind("postgres://localhost/mydb", "database-url")

cmds := command.Build("app", tree, command.Injector(injector))

type Config struct {
    DB string `inject:"database-url"`
}
```

A program that injects nothing omits the option.

### Modules

A module registered on the injector is a struct that configures itself and then binds what it built. Its own fields are populated like any other config struct, so it can carry flags and environment variables of its own:

```go
type DatabaseModule struct {
    Host string `flag:"db-host" env:"DB_HOST" default:"localhost" usage:"Database host"`
}

func (m *DatabaseModule) Install(s *inject.Source) error {
    s.Bind(openDB(m.Host), "database")
    return nil
}

injector := inject.New()
injector.Register(&DatabaseModule{})
```

Registered modules are wiring for the whole program rather than for one command, so `--db-host` is accepted by every command and documented on every help page and man page — nothing in the command tree has to point at the module for that to happen.

Because the injector is what knows about the modules, `ManPageTree` needs the `Injector` option to document them; generating pages from a `Commands` built without it will describe only the commands' own config structs.

## Subcommands

Nest `map[string]interface{}` to create subcommand trees:

```go
command.Build("app", map[string]interface{}{
    "": "My CLI tool",
    "db": map[string]interface{}{
        "": "Database commands",
        "migrate": func(args []string) error { return nil },
        "seed":    func(args []string) error { return nil },
    },
})
```

Use `--` to stop subcommand routing and pass remaining args to the current command.

## Built-in commands

- `help` / `--help` — auto-generated help output showing subcommands, flags, and environment variables
- `version` / `--version` — prints the version string (root command only, requires `command.Version()` option)
- `completion [bash|zsh|fish]` — prints a shell completion script to stdout
- `__man <dir>` — writes roff-format man pages to `dir` (hidden from help; see [Man pages](#man-pages))

## Shell completion

Run `myapp completion bash` (or `zsh`, `fish`) and source the output to enable tab completion. The script is a thin wrapper that calls back into the binary at runtime, so it stays in sync with your commands automatically — no need to regenerate after adding subcommands.

```sh
# bash — add to ~/.bashrc or ~/.bash_profile
source <(myapp completion bash)

# zsh — save to a directory on $fpath
myapp completion zsh > ~/.zsh/completions/_myapp

# fish — save to completions directory
myapp completion fish > ~/.config/fish/completions/myapp.fish
```

The completion mechanism works by appending `__complete <partial>` to the already-typed navigation words. The binary resolves the subcommand path using its normal routing and returns matching subcommand names and flag names for the shell to filter.

## Man pages

`ManPageTree` writes a flat directory of roff-format man pages, one per command, named with hyphen-separated paths (`myapp.1`, `myapp-db.1`, `myapp-db-migrate.1`):

```go
if err := cmds.ManPageTree("/usr/local/share/man/man1"); err != nil {
    log.Fatal(err)
}
```

Each page includes NAME, SYNOPSIS, COMMANDS, OPTIONS, and ENVIRONMENT sections derived from the command tree, struct tags, and descriptions. The hidden `__man <dir>` subcommand exposes the same functionality at runtime for local use or CI pipelines:

```sh
myapp __man ./man
```

## Options

```go
command.Build("app", tree,
    command.Version("1.0.0"),
    command.StdOut(os.Stdout),
    command.StdErr(os.Stderr),
    command.Injector(injector),
)
```

- `Version` — version string for `version` / `--version`
- `StdOut` / `StdErr` — where output is written; both default to the process streams
- `Injector` — the `*inject.Source` to resolve `inject` tags against (see [Dependency injection](#dependency-injection)). Omit it and commands run against an empty injector

## Error handling

`FormatErrors` formats known error types (unknown commands/flags with "did you mean" suggestions, missing required values) to stderr and returns any remaining unhandled errors:

```go
if err := cmds.Execute(os.Args[1:]); err != nil {
    if err = cmds.FormatErrors(err); err != nil {
        fmt.Fprintln(os.Stderr, err)
    }
    os.Exit(1)
}
```
