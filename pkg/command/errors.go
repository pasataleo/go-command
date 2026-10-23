package command

import (
	"maps"

	"github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-config/pkg/didyoumean"
	"github.com/pasataleo/go-errorsx/pkg/errorsx"
)

const (
	UnknownCommand errorsx.Code = "UnknownCommand"
	UnknownFlag    errorsx.Code = "UnknownFlag"
)

// FormatErrors prints the errors it knows how to present - invalid, missing
// and conflicting values, and unknown commands and flags with a suggestion -
// to stderr, and returns every other error for the caller to report.
func (commands *Commands) FormatErrors(err error) error {
	var unknown error
	for _, err := range errorsx.Errors(err) {
		switch code := errorsx.ErrorCode(err); code {
		case config.InvalidValue, config.MissingValue, config.ConflictingValues:
			// these should have been properly formatted already by the config package
			commands.EPrintln(err)
		case UnknownCommand, UnknownFlag:
			commands.EPrintln(err)
			if suggestion, ok := errorsx.GetAnnotation(err, "suggestion"); ok {
				commands.EPrintf("did you mean: %s?\n", suggestion)
			}
		default:
			unknown = errorsx.Append(unknown, err)
		}
	}
	return unknown
}

func unknownCommand(command string, commands map[string]*Command) error {
	err := errorsx.Newf(UnknownCommand, nil, "unknown command: %s", command)
	if len(commands) > 0 {
		if suggestion := didyoumean.Suggest(command, maps.Keys(commands)); len(suggestion) > 0 {
			err = errorsx.Annotate(err, "suggestion", suggestion)
		}
	}
	return err
}

func unknownFlag(flag string, suggestion string) error {
	err := errorsx.Newf(UnknownFlag, nil, "unknown flag: %s", flag)
	if len(suggestion) > 0 {
		err = errorsx.Annotate(err, "suggestion", suggestion)
	}
	return err
}
