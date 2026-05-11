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

<!--
## IMPROVEMENTS
Enhancements to existing functionality.
-->

<!--
## BUG FIXES
Issues that have been resolved.
-->

<!--
## SECURITY
Vulnerabilities or security-related changes addressed in this release.
-->

<!--
## DEPRECATIONS
Functionality that will be removed in a future release.
-->

<!--
## BREAKING CHANGES
Changes that are not backwards compatible and require updates from consumers.
-->

<!--
## UPGRADE NOTES
Steps required when upgrading from a previous version.
-->
