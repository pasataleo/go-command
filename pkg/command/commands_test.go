package command

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/pasataleo/go-config/pkg/flags"
	"github.com/pasataleo/go-config/pkg/inject"
	"github.com/pasataleo/go-errorsx/pkg/errorsx"
	"github.com/pasataleo/go-testingx/pkg/testingx"
)

// --- String shortcut: a plain string creates a description-only command ---
//
// Passing a string as the root command means the tree has no executable. Execute
// should succeed (printing help) without error.
func TestStringShortcut(t *testing.T) {
	cmds := Build("app", map[string]interface{}{
		"": "My CLI tool",
	})

	testingx.Call(t, cmds.Execute, []string{}).
		NoError()
}

// --- Function shortcut: a bare function is used as the command handler ---
//
// A func([]string) error can be passed directly as a  It receives the
// remaining positional args and its return value becomes the Execute result.
func TestFunctionShortcut(t *testing.T) {
	var called bool
	var gotArgs []string

	cmds := Build("app", map[string]interface{}{
		"": func(args []string) error {
			called = true
			gotArgs = args
			return nil
		},
	})

	testingx.Call(t, cmds.Execute, []string{"extra", "args"}).
		NoError()

	testingx.Capture(t, called).Equal(true)
	testingx.Capture(t, gotArgs).Equal([]string{"extra", "args"})
}

// --- Function shortcut with error: returned errors propagate ---
func TestFunctionShortcutError(t *testing.T) {
	cmds := Build("app", map[string]interface{}{
		"": func(args []string) error {
			return fmt.Errorf("something went wrong")
		},
	})

	testingx.Call(t, cmds.Execute, []string{}).
		MatchesError("something went wrong")
}

// --- Subcommand routing: positional args select nested commands ---
//
// The first positional args are consumed to walk the command tree. Once a leaf
// command is found, remaining args are passed to its handler.
func TestSubcommandRouting(t *testing.T) {
	var leaf string

	cmds := Build("app", map[string]interface{}{
		"": "root",
		"parent": map[string]interface{}{
			"": "parent group",
			"child": func(args []string) error {
				leaf = "child"
				return nil
			},
		},
		"other": func(args []string) error {
			leaf = "other"
			return nil
		},
	})

	leaf = ""
	testingx.Call(t, cmds.Execute, []string{"parent", "child"}).
		NoError()
	testingx.Capture(t, leaf).Equal("child")

	leaf = ""
	testingx.Call(t, cmds.Execute, []string{"other"}).
		NoError()
	testingx.Capture(t, leaf).Equal("other")
}

// --- Flag parsing: flags are parsed and assigned to struct fields ---
//
// The "flag" struct tag binds a CLI flag to a field. Supports --flag=value and
// --flag value syntax.
func TestFlagParsing(t *testing.T) {
	type Config struct {
		Name string `flag:"name"`
		Port int    `flag:"port"`
	}

	cfg := new(Config)
	cmds := Build("app", map[string]interface{}{
		"": &Command{
			Config: cfg,
			Command: func(args []string) error {
				return nil
			},
		},
	})

	testingx.Call(t, cmds.Execute, []string{"--name", "alice", "--port=8080"}).
		NoError()

	testingx.Capture(t, cfg.Name).Equal("alice")
	testingx.Capture(t, cfg.Port).Equal(8080)
}

// --- Default values: the "default" tag provides fallback values ---
//
// When no flag or env var sets a field, the default tag value is used.
func TestDefaultValues(t *testing.T) {
	type Config struct {
		Host string `flag:"host" default:"localhost"`
		Port int    `flag:"port" default:"3000"`
	}

	cfg := new(Config)
	cmds := Build("app", map[string]interface{}{
		"": &Command{
			Config: cfg,
			Command: func(args []string) error {
				return nil
			},
		},
	})

	testingx.Call(t, cmds.Execute, []string{}).
		NoError()

	testingx.Capture(t, cfg.Host).Equal("localhost")
	testingx.Capture(t, cfg.Port).Equal(3000)
}

// --- Flags override defaults: explicit flags take priority ---
func TestFlagsOverrideDefaults(t *testing.T) {
	type Config struct {
		Host string `flag:"host" default:"localhost"`
	}

	cfg := new(Config)
	cmds := Build("app", map[string]interface{}{
		"": &Command{
			Config: cfg,
			Command: func(args []string) error {
				return nil
			},
		},
	})

	testingx.Call(t, cmds.Execute, []string{"--host", "example.com"}).
		NoError()

	testingx.Capture(t, cfg.Host).Equal("example.com")
}

// --- Environment variables: the "env" tag reads from the environment ---
func TestEnvironmentVariables(t *testing.T) {
	type Config struct {
		Token string `env:"TEST_COMMAND_TOKEN"`
	}

	cfg := new(Config)
	cmds := Build("app", map[string]interface{}{
		"": &Command{
			Config: cfg,
			Command: func(args []string) error {
				return nil
			},
		},
	})

	t.Setenv("TEST_COMMAND_TOKEN", "secret123")

	testingx.Call(t, cmds.Execute, []string{}).
		NoError()

	testingx.Capture(t, cfg.Token).Equal("secret123")
}

// --- Dependency injection: the injector populates fields by tag ---
//
// Values bound to the injector with a tag name are matched to struct fields
// that have a corresponding "inject" tag.
func TestDependencyInjection(t *testing.T) {
	type Config struct {
		DB string `inject:"database-url"`
	}

	injector := inject.New()
	injector.Bind("postgres://localhost/mydb", "database-url")

	cfg := new(Config)
	cmds := Build("app", map[string]interface{}{
		"": &Command{
			Config: cfg,
			Command: func(args []string) error {
				return nil
			},
		},
	}, Injector(injector))

	testingx.Call(t, cmds.Execute, []string{}).
		NoError()

	testingx.Capture(t, cfg.DB).Equal("postgres://localhost/mydb")
}

// --- Required validation: missing required fields produce an error ---
//
// The "required" tag value is included in the error message so users know
// which flag or field they need to provide.
func TestRequiredValidation(t *testing.T) {
	type Config struct {
		Name string `flag:"name" required:"--name"`
	}

	cmds := Build("app", map[string]interface{}{
		"": &Command{
			Config:  new(Config),
			Command: func(args []string) error { return nil },
		},
	})

	testingx.Call(t, cmds.Execute, []string{}).
		HasError("required value not set: --name")
}

// --- Required passes when set: a required field with a value is fine ---
func TestRequiredPassesWhenSet(t *testing.T) {
	type Config struct {
		Name string `flag:"name" required:"--name"`
	}

	cfg := new(Config)
	cmds := Build("app", map[string]interface{}{
		"": &Command{
			Config:  cfg,
			Command: func(args []string) error { return nil },
		},
	})

	testingx.Call(t, cmds.Execute, []string{"--name", "alice"}).
		NoError()

	testingx.Capture(t, cfg.Name).Equal("alice")
}

// --- Bool negation: --no-flag sets a bool field to false ---
//
// When a bool field has a true default, passing --no-<flag> explicitly sets
// it to false.
func TestBoolNegation(t *testing.T) {
	type Config struct {
		Color bool `flag:"color" default:"true"`
	}

	cfg := new(Config)
	cmds := Build("app", map[string]interface{}{
		"": &Command{
			Config:  cfg,
			Command: func(args []string) error { return nil },
		},
	})

	testingx.Call(t, cmds.Execute, []string{"--no-color"}).
		NoError()

	testingx.Capture(t, cfg.Color).Equal(false)
}

// --- Hidden commands: hidden commands are executable but omitted from help ---
//
// A hidden command can still be invoked directly. This test verifies it runs.
func TestHiddenCommandExecutes(t *testing.T) {
	var called bool

	cmds := Build("app", map[string]interface{}{
		"": "root",
		"debug": &Command{
			Hidden: true,
			Command: func(args []string) error {
				called = true
				return nil
			},
		},
	})

	testingx.Call(t, cmds.Execute, []string{"debug"}).
		NoError()

	testingx.Capture(t, called).Equal(true)
}

// --- Double dash stops subcommand routing ---
//
// A "--" stops subcommand resolution. Everything after it (including things
// that look like subcommand names) is passed as positional args.
func TestDoubleDashStopsRouting(t *testing.T) {
	var gotArgs []string

	cmds := Build("app", map[string]interface{}{
		"": func(args []string) error {
			gotArgs = args
			return nil
		},
		"child": func(args []string) error {
			t.Fatal("child should not be reached")
			return nil
		},
	})

	testingx.Call(t, cmds.Execute, []string{"--", "child", "extra"}).
		NoError()

	testingx.Capture(t, gotArgs).Contains("child")
	testingx.Capture(t, gotArgs).Contains("extra")
}

// --- Command struct directly: full control over command definition ---
//
// Using Command{} lets you set Config, Command, Description, and
// Hidden without implementing any interfaces.
func TestCommandStructDirectly(t *testing.T) {
	type Config struct {
		Name string `flag:"name" default:"world"`
	}

	cfg := new(Config)
	var greeting string

	cmds := Build("app", map[string]interface{}{
		"": &Command{
			Description: "A greeting tool.",
			Config:      cfg,
			Command: func(args []string) error {
				greeting = fmt.Sprintf("Hello, %s!", cfg.Name)
				return nil
			},
		},
	})

	testingx.Call(t, cmds.Execute, []string{"--name", "Alice"}).
		NoError()

	testingx.Capture(t, greeting).Equal("Hello, Alice!")
}

// --- Source priority: flags beat env which beats defaults ---
//
// When multiple sources provide a value for the same field, the highest
// priority source wins: flags > env > inject > defaults.
func TestSourcePriority(t *testing.T) {
	type Config struct {
		Value string `flag:"value" env:"TEST_COMMAND_VALUE" default:"from-default"`
	}

	// Only default.
	cfg1 := new(Config)
	cmds1 := Build("app", &Command{Config: cfg1, Command: func(args []string) error { return nil }})
	testingx.Call(t, cmds1.Execute, []string{}).NoError()
	testingx.Capture(t, cfg1.Value).Equal("from-default")

	// Env overrides default.
	t.Setenv("TEST_COMMAND_VALUE", "from-env")
	cfg2 := new(Config)
	cmds2 := Build("app", &Command{Config: cfg2, Command: func(args []string) error { return nil }})
	testingx.Call(t, cmds2.Execute, []string{}).NoError()
	testingx.Capture(t, cfg2.Value).Equal("from-env")

	// Flag overrides env.
	cfg3 := new(Config)
	cmds3 := Build("app", &Command{Config: cfg3, Command: func(args []string) error { return nil }})
	testingx.Call(t, cmds3.Execute, []string{"--value", "from-flag"}).NoError()
	testingx.Capture(t, cfg3.Value).Equal("from-flag")
}

// --- Empty args: Execute handles empty args gracefully ---
func TestEmptyArgs(t *testing.T) {
	cmds := Build("app", map[string]interface{}{
		"": "root description",
	})

	testingx.Call(t, cmds.Execute, []string{}).
		NoError()
}

// --- Struct with interfaces: the classic approach ---
//
// Implementing Executable, Describable, etc. on a struct is the standard way
// to define commands with full config support.
func TestStructWithInterfaces(t *testing.T) {
	cfg := new(greetCommand)
	cmds := Build("app", map[string]interface{}{
		"": cfg,
	})

	testingx.Call(t, cmds.Execute, []string{"--message", "hi"}).
		NoError()

	testingx.Capture(t, cfg.Message).Equal("hi")
	testingx.Capture(t, cfg.executed).Equal(true)
}

// --- Unmatched positional args are passed through ---
//
// Args that don't match a subcommand name stop the tree walk and are passed
// to the resolved command's handler.
func TestUnmatchedArgsPassThrough(t *testing.T) {
	var gotArgs []string

	cmds := Build("app", map[string]interface{}{
		"": func(args []string) error {
			gotArgs = args
			return nil
		},
	})

	testingx.Call(t, cmds.Execute, []string{"foo", "bar"}).
		NoError()

	testingx.Capture(t, gotArgs).Equal([]string{"foo", "bar"})
}

// --- Mixed flags and subcommands ---
//
// Flags can appear anywhere in the args. Subcommand routing uses only
// positional args (non-flag arguments).
func TestFlagsWithSubcommands(t *testing.T) {
	type ChildConfig struct {
		Name string `flag:"name" default:"world"`
	}

	cfg := new(ChildConfig)
	var called bool

	cmds := Build("app", map[string]interface{}{
		"": "root",
		"greet": &Command{
			Config: cfg,
			Command: func(args []string) error {
				called = true
				return nil
			},
		},
	})

	testingx.Call(t, cmds.Execute, []string{"greet", "--name", "Alice"}).
		NoError()

	testingx.Capture(t, called).Equal(true)
	testingx.Capture(t, cfg.Name).Equal("Alice")
}

func capture(t *testing.T, fn func(opts OptionsFn)) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	fn(StdOut(w))
	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// --- Help flag: --help prints help output ---
func TestHelpFlag(t *testing.T) {
	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"":    "My CLI tool",
			"run": "Run something",
		}, opts)

		testingx.Call(t, cmds.Execute, []string{"--help"}).
			NoError()
	})

	testingx.Capture(t, output).Equal(`My CLI tool

Usage:
  app [command]

Available subcommands:
  completion  Generate shell completion script
  help        Display help information (i.e. print this message)
  run         Run something
  version     Display version information

Flags:
  --help     Display help information (i.e. print this message) (default: "false")
  --version  Display version information (default: "false")
`)
}

// --- Help arg: "help" as a positional arg prints help output ---
func TestHelpArg(t *testing.T) {
	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"":    "My CLI tool",
			"run": "Run something",
		}, opts)

		testingx.Call(t, cmds.Execute, []string{"help"}).
			NoError()
	})

	testingx.Capture(t, output).Equal(`My CLI tool

Usage:
  app [command]

Available subcommands:
  completion  Generate shell completion script
  help        Display help information (i.e. print this message)
  run         Run something
  version     Display version information

Flags:
  --help     Display help information (i.e. print this message) (default: "false")
  --version  Display version information (default: "false")
`)
}

// --- Help usage line: commands without a "" entry still have their name ---
//
// A map level without a "" entry used to get an unnamed command, so the usage
// line lost the program name, and a group's name was missing from the paths
// of the commands under it.
func TestHelpUsageLine(t *testing.T) {
	tree := map[string]interface{}{
		"run": "Run something",
		"db": map[string]interface{}{
			"migrate": "Migrate the database",
		},
	}

	tests := map[string]struct {
		args []string
		want string
	}{
		"root":       {[]string{"--help"}, "  app [command]"},
		"subcommand": {[]string{"run", "--help"}, "  app run"},
		"group":      {[]string{"db", "--help"}, "  app db [command]"},
		"nested":     {[]string{"db", "migrate", "--help"}, "  app db migrate"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			output := capture(t, func(opts OptionsFn) {
				testingx.Call(t, Build("app", tree, opts).Execute, tt.args).
					NoError()
			})

			lines := strings.Split(output, "\n")
			usage := slices.Index(lines, "Usage:")
			if usage < 0 || usage+1 >= len(lines) {
				t.Fatalf("no usage line in:\n%s", output)
			}
			testingx.Capture(t, lines[usage+1]).Equal(tt.want)
		})
	}
}

// --- Help shows config flags: flags from Config structs appear in help ---
//
// This verifies that flags defined via struct tags on a command's Config are
// included in the help output alongside the synthetic help/version flags.
func TestHelpShowsConfigFlags(t *testing.T) {
	type Config struct {
		Host string `flag:"host" default:"localhost" usage:"Server host"`
		Port int    `flag:"port" default:"3000"`
	}

	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"": &Command{
				Description: "My server",
				Config:      new(Config),
				Command:     func(args []string) error { return nil },
			},
		}, opts)

		testingx.Call(t, cmds.Execute, []string{"--help"}).
			NoError()
	})

	testingx.Capture(t, output).Contains("--host")
	testingx.Capture(t, output).Contains("Server host")
	testingx.Capture(t, output).Contains(`(default: "localhost")`)
	testingx.Capture(t, output).Contains("--port")
	testingx.Capture(t, output).Contains(`(default: "3000")`)
}

// --- Version flag: --version prints the version string ---
func TestVersionFlag(t *testing.T) {
	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"": "My CLI tool",
		}, opts, Version("1.2.3"))

		testingx.Call(t, cmds.Execute, []string{"--version"}).
			NoError()
	})

	testingx.Capture(t, output).Equal("1.2.3\n")
}

// --- Version default: without the option, the version the binary was built at ---
//
// A test binary has no version stamped into it, so it reports "dev".
func TestVersionDefault(t *testing.T) {
	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"": "My CLI tool",
		}, opts)

		testingx.Call(t, cmds.Execute, []string{"--version"}).
			NoError()
	})

	testingx.Capture(t, output).Equal("dev\n")
}

// --- Version arg: "version" as a positional arg prints the version string ---
func TestVersionArg(t *testing.T) {
	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"": "My CLI tool",
		}, opts, Version("1.2.3"))

		testingx.Call(t, cmds.Execute, []string{"version"}).
			NoError()
	})

	testingx.Capture(t, output).Equal("1.2.3\n")
}

// --- Version only at root: --version on a subcommand does not print version ---
func TestVersionOnlyAtRoot(t *testing.T) {
	var called bool

	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"": "root",
			"child": func(args []string) error {
				called = true
				return nil
			},
		}, opts, Version("1.2.3"))
		cmds.Err = cmds.Out

		err := testingx.CallAs[error](t, cmds.Execute, []string{"child", "--version"}).
			Capture()
		testingx.Call(t, cmds.FormatErrors, err).NoError()
	})

	testingx.Capture(t, output).Contains("unknown flag: version")
	testingx.Capture(t, called).Equal(false)
}

// --- Hidden commands omitted from help ---
func TestHelpOmitsHiddenCommands(t *testing.T) {
	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"":       "My CLI tool",
			"public": "A visible command",
			"secret": &Command{
				Hidden:      true,
				Description: "A hidden command",
				Command:     func(args []string) error { return nil },
			},
		}, opts)

		testingx.Call(t, cmds.Execute, []string{"--help"}).
			NoError()
	})

	testingx.Capture(t, strings.Contains(output, "public")).Equal(true)
	testingx.Capture(t, strings.Contains(output, "secret")).Equal(false)
}

// --- Unknown command with suggestion: typos produce a "did you mean" hint ---
func TestUnknownCommandSuggestion(t *testing.T) {
	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"":        "My CLI tool",
			"apply":   "Apply changes",
			"destroy": "Destroy resources",
		}, opts)
		cmds.Err = cmds.Out

		err := testingx.CallAs[error](t, cmds.Execute, []string{"destory"}).Capture() // nolint misspell
		testingx.Call(t, cmds.FormatErrors, err).NoError()
	})

	testingx.Capture(t, output).Contains("unknown command: destory") // nolint misspell
	testingx.Capture(t, output).Contains("did you mean: destroy?")
}

// --- Unknown command without suggestion: no match produces no hint ---
func TestUnknownCommandNoSuggestion(t *testing.T) {
	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"":      "My CLI tool",
			"apply": "Apply changes",
		}, opts)
		cmds.Err = cmds.Out

		err := testingx.CallAs[error](t, cmds.Execute, []string{"xyz"}).
			Capture()
		testingx.Call(t, cmds.FormatErrors, err).NoError()
	})

	testingx.Capture(t, output).Contains("unknown command: xyz")
	testingx.Capture(t, output).NotContains("did you mean")
}

// --- Unknown flag with suggestion: typo flags produce a "did you mean" hint ---
func TestUnknownFlagSuggestion(t *testing.T) {
	type Config struct {
		Destroy bool `flag:"destroy"`
	}

	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"": &Command{
				Config:  new(Config),
				Command: func(args []string) error { return nil },
			},
		}, opts)
		cmds.Err = cmds.Out

		err := testingx.CallAs[error](t, cmds.Execute, []string{"--destory"}).Capture() // nolint misspell
		testingx.Call(t, cmds.FormatErrors, err).NoError()
	})

	testingx.Capture(t, output).Contains("unknown flag: destory") // nolint misspell
	testingx.Capture(t, output).Contains("did you mean: destroy?")
}

// --- Unknown flag without suggestion: no close match produces no hint ---
func TestUnknownFlagNoSuggestion(t *testing.T) {
	type Config struct {
		Name string `flag:"name"`
	}

	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"": &Command{
				Config:  new(Config),
				Command: func(args []string) error { return nil },
			},
		}, opts)
		cmds.Err = cmds.Out

		err := testingx.CallAs[error](t, cmds.Execute, []string{"--xyz", "value"}).
			Capture()
		testingx.Call(t, cmds.FormatErrors, err).NoError()
	})

	testingx.Capture(t, output).Contains("unknown flag: xyz")
	testingx.Capture(t, output).NotContains("did you mean")
}

// --- Valid flags do not trigger unknown flag errors ---
func TestValidFlagsNoError(t *testing.T) {
	type Config struct {
		Name string `flag:"name"`
		Port int    `flag:"port" default:"8080"`
	}

	cfg := new(Config)
	var called bool

	cmds := Build("app", map[string]interface{}{
		"": &Command{
			Config: cfg,
			Command: func(args []string) error {
				called = true
				return nil
			},
		},
	})

	testingx.Call(t, cmds.Execute, []string{"--name", "alice", "--port", "3000"}).
		NoError()

	testingx.Capture(t, called).Equal(true)
	testingx.Capture(t, cfg.Name).Equal("alice")
	testingx.Capture(t, cfg.Port).Equal(3000)
}

type dbCommands struct{}

func (d *dbCommands) Description() string { return "Database commands" }

func (d *dbCommands) Children() map[string]interface{} {
	return map[string]interface{}{
		"migrate": func(args []string) error { return nil },
		"seed":    func(args []string) error { return nil },
	}
}

func TestReservedCompleteCommand(t *testing.T) {
	testingx.Panics(t, nil, Build, "__complete", map[string]interface{}{
		"__complete": func(args []string) error { return nil },
	})
}

// A program that wires nothing up supplies no Injector option, and the inject
// source is filled in with an empty one.
func TestNoInjector(t *testing.T) {
	cmds := Build("app", map[string]interface{}{
		"": func(args []string) error { return nil },
	})
	testingx.Call(t, cmds.Execute, []string{}).NoError()
}

func TestWithChildren_Routing(t *testing.T) {
	cmds := Build("app", map[string]interface{}{
		"":   "root",
		"db": &dbCommands{},
	})

	testingx.Call(t, cmds.Execute, []string{"db", "migrate"}).NoError()
	testingx.Call(t, cmds.Execute, []string{"db", "seed"}).NoError()
}

func TestWithChildren_UnknownChild(t *testing.T) {
	cmds := Build("app", map[string]interface{}{
		"":   "root",
		"db": &dbCommands{},
	})

	testingx.Call(t, cmds.Execute, []string{"db", "unknown"}).Error()
}

// greetCommand is a test helper that implements the interface-based approach.
type greetCommand struct {
	Message  string `flag:"message" default:"hello"`
	executed bool
}

func (c *greetCommand) Description() string {
	return "Greet someone."
}

func (c *greetCommand) Execute(args []string) error {
	c.executed = true
	return nil
}

// --- Registered modules: flags and env vars a module brings along ---
//
// A module registered on the injector is not reachable from the command's own
// config struct, so Find does not walk it. Its flags and environment variables
// still have to show up in help, completion and man pages, and still have to be
// accepted rather than rejected as unknown.
type dbModule struct {
	Host string `flag:"db-host" env:"DB_HOST" default:"localhost" usage:"Database host"`
}

func (m *dbModule) Install(s *inject.Source) error {
	s.Bind(m.Host, "database-url")
	return nil
}

func TestRegisteredModule_FlagInHelp(t *testing.T) {
	injector := inject.New()
	injector.Register(&dbModule{})

	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"": "My CLI tool",
		}, Injector(injector), opts)

		testingx.Call(t, cmds.Execute, []string{"--help"}).NoError()
	})

	help := testingx.Capture(t, output)
	help.Contains("--db-host")
	help.Contains("Database host")
	help.Contains(`(default: "localhost")`)
}

func TestRegisteredModule_FlagAccepted(t *testing.T) {
	injector := inject.New()
	injector.Register(&dbModule{})

	cmds := Build("app", map[string]interface{}{
		"": func(args []string) error { return nil },
	}, Injector(injector))

	testingx.Call(t, cmds.Execute, []string{"--db-host", "example.com"}).NoError()
}

// The module's fields belong to every command, so they are documented on every
// page: the subcommand accepts --db-host exactly as the root does.
func TestRegisteredModule_InManPages(t *testing.T) {
	injector := inject.New()
	injector.Register(&dbModule{})

	cmds := Build("app", map[string]interface{}{
		"":     "My CLI tool",
		"sync": "Sync something",
	}, Injector(injector))

	dir := t.TempDir()
	testingx.Call(t, cmds.ManPageTree, dir).NoError()

	for _, name := range []string{"app.1", "app-sync.1"} {
		content := testingx.CallAs[[]byte](t, os.ReadFile, filepath.Join(dir, name)).NoError().Capture()

		page := testingx.Capture(t, string(content))
		page.Contains("db\\-host")
		page.Contains("Database host")
		page.Contains("DB_HOST")
	}
}

// --- oneof: exactly one of a group of required flags ---

type listenConfig struct {
	Port   int    `flag:"port" required:"port" oneof:"listen" usage:"Port to listen on"`
	Socket string `flag:"socket" required:"socket" oneof:"listen" usage:"Socket to listen on"`
}

func TestOneOf(t *testing.T) {
	tcs := map[string]struct {
		args    []string
		wantErr string
	}{
		"port":          {args: []string{"--port", "8080"}},
		"socket":        {args: []string{"--socket", "/tmp/app.sock"}},
		"port set to 0": {args: []string{"--port", "0"}},
		"neither":       {wantErr: "required value not set: one of port, socket"},
		"both": {
			args:    []string{"--port", "8080", "--socket", "/tmp/app.sock"},
			wantErr: "only one of port, socket can be set, got port, socket",
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			cmds := Build("app", map[string]interface{}{
				"": &Command{
					Config:  new(listenConfig),
					Command: func(args []string) error { return nil },
				},
			})

			if tc.wantErr == "" {
				testingx.Call(t, cmds.Execute, tc.args).NoError()
				return
			}
			testingx.Call(t, cmds.Execute, tc.args).HasError(tc.wantErr)
		})
	}
}

// portOnlyModule holds one member of a group whose other member is on the
// command's own config, so the group spans the module's walk and the
// command's.
type portOnlyModule struct {
	Port int `flag:"port" required:"port" oneof:"listen" usage:"Port to listen on"`
}

func (m *portOnlyModule) Install(*inject.Source) error {
	return nil
}

func TestOneOf_acrossModule(t *testing.T) {
	type Config struct {
		Socket string `flag:"socket" required:"socket" oneof:"listen" usage:"Socket to listen on"`
	}

	build := func() *Commands {
		injector := inject.New()
		injector.Register(new(portOnlyModule))
		return Build("app", map[string]interface{}{
			"": &Command{
				Config:  new(Config),
				Command: func(args []string) error { return nil },
			},
		}, Injector(injector))
	}

	testingx.Call(t, build().Execute, []string{"--port", "8080"}).NoError()
	testingx.Call(t, build().Execute, []string{"--socket", "/tmp/app.sock"}).NoError()
	testingx.Call(t, build().Execute, []string{}).HasError("required value not set: one of port, socket")
}

func TestOneOf_help(t *testing.T) {
	output := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{
			"": &Command{
				Config:  new(listenConfig),
				Command: func(args []string) error { return nil },
			},
		}, opts)

		testingx.Call(t, cmds.Execute, []string{"--help"}).NoError()
	})

	out := testingx.Capture(t, output)
	out.Contains("--port     Port to listen on (required: one of --port, --socket)")
	out.Contains("--socket   Socket to listen on (required: one of --port, --socket)")
}

func TestOneOf_manPage(t *testing.T) {
	cmds := Build("app", map[string]interface{}{
		"": &Command{
			Config:  new(listenConfig),
			Command: func(args []string) error { return nil },
		},
	})

	dir := t.TempDir()
	testingx.Call(t, cmds.ManPageTree, dir).NoError()

	content := testingx.CallAs[[]byte](t, os.ReadFile, filepath.Join(dir, "app.1")).NoError().Capture()
	testingx.Capture(t, string(content)).Contains("Port to listen on (required: one of \\-\\-port, \\-\\-socket)")
}

// FormatErrors prints a conflicting-values error, rather than dropping it as
// an unknown code.
func TestFormatErrors_conflictingValues(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	cmds := Build("app", map[string]interface{}{
		"": &Command{
			Config:  new(listenConfig),
			Command: func(args []string) error { return nil },
		},
	}, StdErr(w))

	execErr := cmds.Execute([]string{"--port", "8080", "--socket", "/tmp/app.sock"})
	remaining := cmds.FormatErrors(execErr)
	w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	testingx.Capture(t, string(out)).Contains("only one of port, socket can be set")
	if remaining != nil {
		t.Errorf("FormatErrors returned %v, want the error handled", remaining)
	}
}

// FormatErrors returns every error it does not present itself, whatever its
// code, so the caller can report it. A command failing with a coded error used
// to exit with nothing printed at all.
func TestFormatErrors_returnsOtherErrors(t *testing.T) {
	tcs := map[string]error{
		"coded":   errorsx.New(errorsx.NotFound, nil, "no such user"),
		"uncoded": errors.New("something broke"),
	}
	for name, cause := range tcs {
		t.Run(name, func(t *testing.T) {
			cmds := Build("app", map[string]interface{}{
				"": func(args []string) error { return cause },
			})

			testingx.Call(t, cmds.FormatErrors, cmds.Execute(nil)).ErrorContains(cause.Error())
		})
	}
}

// Errors it presents are printed and not returned, while the rest of an
// aggregate is returned.
func TestFormatErrors_mixed(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	cmds := Build("app", map[string]interface{}{
		"": func(args []string) error { return nil },
	}, StdErr(w))

	remaining := cmds.FormatErrors(errorsx.Append(
		errorsx.New(UnknownFlag, nil, "unknown flag: --nme"),
		errorsx.New(errorsx.Unavailable, nil, "listening on 127.0.0.1:8080"),
	))
	w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	testingx.Capture(t, string(out)).Contains("unknown flag: --nme")
	testingx.Capture(t, remaining).ErrorContains("listening on 127.0.0.1:8080")
	if strings.Contains(remaining.Error(), "unknown flag") {
		t.Errorf("FormatErrors returned the error it printed: %v", remaining)
	}
}

// --- Unclaimed flags: a command can take flags beyond its config ---

// requestCommand takes request fields as flags beyond its config, the way a
// command built from a method does: they are not config fields, so it
// documents them and receives them itself.
type requestCommand struct {
	Verbose bool `flag:"verbose" usage:"Print more"`

	received    map[string][]string
	sawVerbose  bool
	ran         bool
	failWith    error
	rejectNames []string
}

func (c *requestCommand) Execute([]string) error {
	c.ran = true
	return nil
}

func (c *requestCommand) FlagDocs() []FlagDoc {
	return []FlagDoc{
		{Flag: flags.Flag{Name: "name", Type: reflect.TypeFor[string]()}, Usage: "Who to greet"},
		{Flag: flags.Flag{Name: "address.city", Type: reflect.TypeFor[string]()}, Usage: "Where they live"},
		{Flag: flags.Flag{Name: "loud", Type: reflect.TypeFor[bool]()}, Usage: "Shout"},
	}
}

func (c *requestCommand) UnclaimedFlags(flags map[string][]string) ([]string, error) {
	c.received = flags
	c.sawVerbose = c.Verbose
	if c.failWith != nil {
		return nil, c.failWith
	}
	return c.rejectNames, nil
}

func TestUnclaimedFlags_received(t *testing.T) {
	cmd := new(requestCommand)
	cmds := Build("app", map[string]interface{}{"greet": cmd})

	testingx.Call(t, cmds.Execute, []string{"greet", "--verbose", "--name", "world", "--address.city", "London", "--address.city", "Paris", "--no-loud"}).
		NoError()

	testingx.Capture(t, cmd.received).Equal(map[string][]string{
		"name":         {"world"},
		"address.city": {"London", "Paris"},
		"no-loud":      {""},
	})
	// The config is assigned before the command sees its unclaimed flags.
	testingx.Capture(t, cmd.sawVerbose).True()
	testingx.Capture(t, cmd.ran).True()
}

func TestUnclaimedFlags_unknownNamesReported(t *testing.T) {
	cmd := &requestCommand{rejectNames: []string{"nme"}}
	cmds := Build("app", map[string]interface{}{"greet": cmd})

	err := cmds.Execute([]string{"greet", "--nme", "world"})
	testingx.Capture(t, err).ErrorCode(UnknownFlag)
	testingx.Capture(t, err).ErrorContains("unknown flag: nme")
	testingx.Call(t, errorsx.GetAnnotation, err, "suggestion").True().Equal(any("name"))
	testingx.Capture(t, cmd.ran).False()
}

func TestUnclaimedFlags_errorStopsCommand(t *testing.T) {
	cmd := &requestCommand{failWith: errorsx.New(errorsx.InvalidArgument, nil, "--name must not be empty")}
	cmds := Build("app", map[string]interface{}{"greet": cmd})

	testingx.Call(t, cmds.Execute, []string{"greet", "--name", ""}).ErrorCode(errorsx.InvalidArgument)
	testingx.Capture(t, cmd.ran).False()
}

func TestUnclaimedFlags_commandFields(t *testing.T) {
	var received map[string][]string
	cmds := Build("app", map[string]interface{}{
		"greet": &Command{
			FlagDocs: []FlagDoc{{Flag: flags.Flag{Name: "name", Type: reflect.TypeFor[string]()}}},
			UnclaimedFlags: func(flags map[string][]string) ([]string, error) {
				received = flags
				return nil, nil
			},
			Command: func([]string) error { return nil },
		},
	})

	testingx.Call(t, cmds.Execute, []string{"greet", "--name", "world"}).NoError()
	testingx.Capture(t, received).Equal(map[string][]string{"name": {"world"}})
}

// A command without UnclaimedFlags is as strict as it always was.
func TestUnclaimedFlags_strictWithout(t *testing.T) {
	type Config struct {
		Name string `flag:"name"`
	}
	cmds := Build("app", map[string]interface{}{
		"greet": &Command{Config: new(Config), Command: func([]string) error { return nil }},
	})

	testingx.Call(t, cmds.Execute, []string{"greet", "--nme", "world"}).ErrorCode(UnknownFlag)
}

func TestFlagDocs_withoutUnclaimedFlagsPanics(t *testing.T) {
	testingx.Panics(t, nil, func() {
		Build("app", map[string]interface{}{
			"greet": &Command{
				FlagDocs: []FlagDoc{{Flag: flags.Flag{Name: "name", Type: reflect.TypeFor[string]()}}},
				Command:  func([]string) error { return nil },
			},
		})
	})
}

func TestFlagDocs_inHelpCompletionAndMan(t *testing.T) {
	help := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{"greet": new(requestCommand)}, opts)
		testingx.Call(t, cmds.Execute, []string{"greet", "--help"}).NoError()
	})
	out := testingx.Capture(t, help)
	out.Contains("--name          Who to greet")
	out.Contains("--address.city  Where they live")
	out.Contains("--loud          Shout")
	out.Contains("--verbose       Print more")

	completion := capture(t, func(opts OptionsFn) {
		cmds := Build("app", map[string]interface{}{"greet": new(requestCommand)}, opts)
		testingx.Call(t, cmds.Execute, []string{"greet", "__complete", "--"}).NoError()
	})
	testingx.Capture(t, completion).Contains("--address.city")

	dir := t.TempDir()
	cmds := Build("app", map[string]interface{}{"greet": new(requestCommand)})
	testingx.Call(t, cmds.ManPageTree, dir).NoError()
	page := testingx.CallAs[[]byte](t, os.ReadFile, filepath.Join(dir, "app-greet.1")).NoError().Capture()
	testingx.Capture(t, string(page)).Contains("Who to greet")
}

// A documented flag's default and requirement are shown the way a config
// flag's are.
func TestFlagDocs_defaultAndRequirement(t *testing.T) {
	build := func(opts ...OptionsFn) *Commands {
		return Build("app", map[string]interface{}{
			"greet": &Command{
				FlagDocs: []FlagDoc{
					{Flag: flags.Flag{Name: "name", Type: reflect.TypeFor[string]()}, Usage: "Who to greet", Default: "world"},
					{Flag: flags.Flag{Name: "id", Type: reflect.TypeFor[int]()}, Usage: "Which one", Requirement: "required"},
				},
				UnclaimedFlags: func(map[string][]string) ([]string, error) { return nil, nil },
				Command:        func([]string) error { return nil },
			},
		}, opts...)
	}

	help := capture(t, func(opts OptionsFn) {
		testingx.Call(t, build(opts).Execute, []string{"greet", "--help"}).NoError()
	})
	out := testingx.Capture(t, help)
	out.Contains(`--name  Who to greet (default: "world")`)
	out.Contains(`--id    Which one (required)`)

	dir := t.TempDir()
	testingx.Call(t, build().ManPageTree, dir).NoError()
	page := testingx.CallAs[[]byte](t, os.ReadFile, filepath.Join(dir, "app-greet.1")).NoError().Capture()
	testingx.Capture(t, string(page)).Contains("Which one (required)")
}
