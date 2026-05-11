package command

func (commands *Commands) VersionCommand() {
	commands.Println(commands.Version)
}
