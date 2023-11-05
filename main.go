package main

import (
	"fmt"
	"os"

	"github.com/pasataleo/go-flags/flags"

	"github.com/pasataleo/go-command/command"
)

func main() {

	cmd := command.New("gitc", "")
	branch, err := cmd.Add("branch", "Manage git branches")
	if err != nil {
		panic(err)
	}

	flags.BindString("first", "Value for branch command.", true, "first value").ToInjectorUnsafe(cmd.FlagSet, branch.Injector, "first")
	flags.BindString("second", "Value for branch command.", true, "second value").ToInjectorUnsafe(cmd.FlagSet, branch.Injector, "second")

	branch.Fn = func(cmd *command.Command, args []string) error {
		fmt.Println(cmd.Injector.GetUnsafe("second"))
		return nil
	}

	flags.BindString("first", "Value for branch command.", true, "first value").ToInjectorUnsafe(branch.FlagSet, branch.Injector, "first")
	flags.BindString("second", "Value for branch command.", true, "second value").ToInjectorUnsafe(branch.FlagSet, branch.Injector, "second")

	if err := cmd.Execute(os.Args[1:]); err != nil {
		panic(err)
	}
}
