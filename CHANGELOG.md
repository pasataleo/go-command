# v0.2.0

## FEATURES

- `Injector` build option, supplying the injector once at `Build` time. Documentation generated from the `Commands` alone - `ManPageTree`, which has no `Execute` call to take an injector from - now knows about the modules the program registered

## IMPROVEMENTS

- Help text, shell completion and man pages report the flags and environment variables of inject modules registered on the injector, not only those of the command's own config struct. A registered module holds flags and environment variables like any other struct, but nothing the caller owns points at it, so `Find` does not walk it; `config.AdditionalTargets` reports those structs and they are merged into the metadata every command is described from
- Man pages for subcommands are generated from the same sources as the root page. Each command in the tree was previously re-deriving its metadata from a separate, injector-less source list, so a subcommand's page could differ from the root's for reasons that had nothing to do with the subcommand
- Flag validation no longer branches on whether the command has a config struct of its own, since a registered module contributes flags either way

## BUG FIXES

- A flag belonging to a registered inject module is no longer rejected as an unknown flag. The module's fields were absent from the set of known flags, so passing one failed with `unknown flag` even though the module was there to receive it

## BREAKING CHANGES

- `Commands.Execute` takes only the arguments: `Execute(args []string) error`. The injector is supplied to `Build` with the `Injector` option instead, so there is one place it comes from rather than two. A call that passed `nil` drops the argument; a call that passed an injector moves it to `Build`

## UPGRADE NOTES

- `cmds.Execute(os.Args[1:], nil)` becomes `cmds.Execute(os.Args[1:])`
- `cmds.Execute(os.Args[1:], injector)` becomes `command.Build(name, tree, command.Injector(injector))` at construction and `cmds.Execute(os.Args[1:])` at the call

# v0.1.0

## FEATURES

- `command.Build` for constructing CLI applications from a `map[string]interface{}` command tree
- String, function, and `Command` struct shortcuts for defining commands at each level
- Interface-based commands via `Executable`, `Describable`, and `Hidden`
- Config structs populated automatically from flags, environment variables, dependency injection, and defaults — in priority order
- Struct tags: `flag`, `env`, `inject`, `default`, `required`, `usage`, `hidden`
- Bool flag negation with `--no-<flag>`
- Nested subcommand trees with `--` pass-through
- Built-in `help` / `--help` with auto-generated output listing subcommands, flags, and environment variables
- Built-in `version` / `--version` via `command.Version()` option
- `command.StdOut` and `command.StdErr` options for controlling output destinations
- `FormatErrors` for formatting unknown command/flag errors with "did you mean" suggestions and missing required value errors
- `WithChildren` interface for nesting subcommands inside a struct via a `Children() map[string]interface{}` method
- Shell completion for bash, zsh, and fish via the built-in `completion` command; scripts call back into the binary at runtime via `__complete` so they stay in sync automatically
- Man page generation via `ManPageTree(dir)` and the hidden `__man <dir>` subcommand; writes roff-format `.1` files with NAME, SYNOPSIS, COMMANDS, OPTIONS, and ENVIRONMENT sections
- Passing `nil` as the injector to `Execute` is safe
