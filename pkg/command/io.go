package command

import (
	"fmt"
)

func (commands *Commands) Print(message ...any) {
	_, _ = fmt.Fprint(commands.Out, message...)
}

func (commands *Commands) Println(message ...any) {
	_, _ = fmt.Fprintln(commands.Out, message...)
}

func (commands *Commands) Printf(format string, args ...any) {
	_, _ = fmt.Fprintf(commands.Out, format, args...)
}

func (commands *Commands) EPrint(message ...any) {
	_, _ = fmt.Fprint(commands.Err, message...)
}

func (commands *Commands) EPrintf(format string, args ...any) {
	_, _ = fmt.Fprintf(commands.Err, format, args...)
}

func (commands *Commands) EPrintln(message ...any) {
	_, _ = fmt.Fprintln(commands.Err, message...)
}
