package command

type Command interface {
	BaseCommand
	GetURL() string
	GetIP() string
	RunTransaction(do func(tx BaseCommand) error) error
}

type BaseCommand interface {
	RunCommand(cmd string) (string, error)
	RunCommands(cmds ...string) ([]string, error)
	RunShell(filePath string, remotePath string, params ...string) (outputs []string, err error)
	SendFile(localPath string, remotePath string) error
	SendDir(localDir, remoteDir string) error
}
