package shell

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"runtime"
)

type platform int8

const (
	windows platform = iota
	darwin
	linux
)

type Local struct {
	platform platform
}

func NewLocal() *Local {
	pf := windows
	switch runtime.GOOS {
	case "darwin":
		pf = darwin
	case "linux":
		pf = linux
	case "windows":
		pf = windows
	}
	return &Local{platform: pf}
}

func (me *Local) GetURL() string {
	return "localhost"
}

func (me *Local) GetIP() string {
	return "localhost"
}

func (me *Local) runCommand(cmd string) (string, error) {
	if me.platform == windows {
		bytes, err := exec.Command("powershell", cmd).CombinedOutput()
		return string(bytes), err
	} else {
		bytes, err := exec.Command("bash", "-c", cmd).CombinedOutput()
		return string(bytes), err
	}
}

func (me *Local) RunCommand(cmd string) (string, error) {
	return me.runCommand(cmd)
}

func (me *Local) RunCommands(cmds ...string) ([]string, error) {
	outputs := []string{}
	for _, cmd := range cmds {
		output, err := me.runCommand(cmd)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

// 发送单个文件
// 注意路径必须带文件名
func (me *Local) SendFile(localPath string, remotePath string) error {
	localPath = filepath.Join(localPath)
	remotePath = filepath.Join(remotePath)

	inputBytes, err := ioutil.ReadFile(localPath)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(remotePath, inputBytes, 0644)
}

// 不实现
func (me *Local) RunShell(filePath string, remotePath string, params ...string) (outputs []string, err error) {
	return nil, nil
}

// 不实现
func (me *Local) SendDir(localDir, remoteDir string) error {
	return nil
}

// 不实现
func (me *Local) RunTransaction(do func(tx BaseCommand) error) error {
	return nil
}
