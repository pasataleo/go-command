# v0.2.0

## FEATURES

- `Injector` build option, so man pages generated without an `Execute` call know about registered modules.
- `oneof` groups on required fields, through go-config's `oneof` source.

## IMPROVEMENTS

- Help, completion and man pages include the flags and environment variables of registered inject modules.
- Man pages for subcommands are generated from the same sources as the root page.
- Help and man pages show a `oneof` field as `(required: one of --port, --socket)`.
- `FormatErrors` prints `ConflictingValues` errors.

## BUG FIXES

- A flag belonging to a registered inject module is no longer rejected as unknown.
- Help's usage line includes the program name and every parent command, even without a `""` entry.
- `FormatErrors` returns every error it does not print, whatever its code.

<!--
## SECURITY
Vulnerabilities or security-related changes addressed in this release.
-->

<!--
## DEPRECATIONS
Functionality that will be removed in a future release.
-->

## BREAKING CHANGES

- `Commands.Execute` takes only the arguments; the injector moves to the `Injector` build option.

## UPGRADE NOTES

- `cmds.Execute(args, nil)` becomes `cmds.Execute(args)`.
- `cmds.Execute(args, injector)` becomes `command.Build(name, tree, command.Injector(injector))` and `cmds.Execute(args)`.

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
