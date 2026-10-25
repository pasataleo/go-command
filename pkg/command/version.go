package command

import "runtime/debug"

// buildVersion is the version the binary was built at: go build stamps it from
// version control, and go install from the version installed. A binary with
// neither, such as one from go run, is "dev".
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}

func (commands *Commands) VersionCommand() {
	commands.Println(commands.Version)
}
