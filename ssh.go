package shell

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type sshMode int

const (
	CertPassword sshMode = iota
	CertPublicKeyFile

	DefaultTimeout = 60
)

type SSHClient struct {
	IP   string
	User string
	Cert string
	Port int
	Mode sshMode
}

func NewSSHClientWithPassword(ip string, user string, password string) *SSHClient {
	return &SSHClient{
		IP:   ip,
		User: user,
		Port: 22,
		Cert: password,
		Mode: CertPassword,
	}
}

func (me *SSHClient) GetURL() string {
	return fmt.Sprintf("%s:%d", me.IP, me.Port)
}

func (me *SSHClient) GetIP() string {
	return fmt.Sprintf("%s", me.IP)
}

func (me *SSHClient) readPublicKeyFile(file string) ssh.AuthMethod {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil
	}
	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}

func (me *SSHClient) connect(do func(cli *ssh.Client) error) error {
	var cfg *ssh.ClientConfig
	var auth []ssh.AuthMethod
	if me.Mode == CertPassword {
		auth = []ssh.AuthMethod{ssh.Password(me.Cert)}
	} else if me.Mode == CertPublicKeyFile {
		auth = []ssh.AuthMethod{me.readPublicKeyFile(me.Cert)}
	} else {
		return errors.New(fmt.Sprintf("does not support mode: %d", me.Mode))
	}
	cfg = &ssh.ClientConfig{
		User: me.User,
		Auth: auth,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: time.Second * DefaultTimeout,
	}
	client, err := ssh.Dial("tcp", fmt.Sprint("%s:%d", me.IP, me.Port), cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	return do(client)
}

func (me *SSHClient) RunCommand(cmd string) (string, error) {
	output := ""
	err := me.connect(func(cli *ssh.Client) error {
		temp, err := runCommand(cli, cmd)
		output = temp
		return err
	})
	return output, err
}

func (me *SSHClient) RunCommands(cmds ...string) ([]string, error) {
	outputs := []string{}
	err := me.connect(func(cli *ssh.Client) error {
		var err error
		outputs, err = runCommands(cli, cmds...)
		return err
	})
	return outputs, err
}

// 发送单个文件
// 注意路径必须带文件名
func (me *SSHClient) SendFile(localPath string, remotePath string) error {
	return me.connect(func(cli *ssh.Client) error {
		remotePath = filepath.ToSlash(remotePath)
		dir := remotePath[:strings.LastIndex(remotePath, "/")]
		_, err := runCommand(cli, "mkdir -p "+dir)
		if err != nil {
			return err
		}
		return sendFile(cli, localPath, remotePath)
	})
}

// 发送整个目录
// 注意 远程目录需要自己指定，函数不回将原目录发送，路径必须带 '/'
// 例如 ../scrpt， ./不会在远程目录生成script目录
func (me *SSHClient) SendDir(localDir string, remoteDir string) error {
	return me.connect(func(cli *ssh.Client) error {
		return sendDir(cli, localDir, remoteDir)
	})
}

func (me *SSHClient) RunShell(filePath string, remotePath string, params ...string) (outputs []string, err error) {
	me.connect(func(cli *ssh.Client) error {
		outputs, err = runShell(cli, filePath, remotePath, params...)
		return nil
	})
	return
}

func (me *SSHClient) RunTransaction(do func(ctx TransactionContext) error) error {
	return me.connect(func(cli *ssh.Client) error {
		return do(TransactionContext{cli: cli})
	})
}

func runSession(cli *ssh.Client, do func(ss *ssh.Session) error) error {
	ss, err := cli.NewSession()
	defer cli.Close()
	if err != nil {
		return err
	}
	return do(ss)
}

func runCommand(cli *ssh.Client, cmd string) (string, error) {
	output := ""
	err := runSession(cli, func(ss *ssh.Session) error {
		r, err := ss.CombinedOutput(cmd)
		if err != nil {
			return err
		}
		output = string(r)
		return nil
	})
	return output, err
}

func runCommands(cli *ssh.Client, cmds ...string) ([]string, error) {
	outputs := []string{}
	for _, cmd := range cmds {
		output, err := runCommand(cli, cmd)
		if err != nil {
			return outputs, err
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

func sendFile(cli *ssh.Client, localPath string, remotePath string) error {
	return runSession(cli, func(ss *ssh.Session) error {
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			return errors.New(fmt.Sprintf("no such file or directory: %s", localPath))
		}
		return CopyPath(localPath, remotePath, ss)
	})
}

func sendDir(cli *ssh.Client, localDir string, remoteDir string) error {
	absPath, fileList, err := getPathList(localDir)
	if err != nil {
		return err
	}
	lastDirIndex := len(absPath)
	for _, v := range fileList {
		remotePath := remoteDir + v.Path[lastDirIndex:]
		// 注意不用系统之间的路径符不同，需要转换
		remotePath = filepath.ToSlash(remotePath)
		if v.IsDir {
			_, err = runCommand(cli, "mkdir -p "+remotePath)
		} else {
			err = sendFile(cli, v.Path, remotePath)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func runShell(cli *ssh.Client, filePath string, remotePath string, params ...string) ([]string, error) {
	// 检查后缀名
	if !strings.HasSuffix(filePath, ".sh") {
		return nil, errors.New("not .sh suffix")
	}
	remotePath = filepath.ToSlash(remotePath)
	remotePath = remotePath + filepath.Base(filePath)
	outputs := []string{}
	err := sendFile(cli, filePath, remotePath)
	if err != nil {
		return outputs, err
	}
	cmd := remotePath
	for _, v := range params {
		cmd = cmd + " " + v
	}
	outputs, err = runCommands(cli,
		"chmod a+x "+remotePath,       //给文件权限
		"sed -i 's/\r$//'"+remotePath, //处理换行符
		cmd,
		"rm -f "+remotePath, // 执行后删除
	)
	return outputs, err
}

type TransactionContext struct {
	cli *ssh.Client
}

func (me *TransactionContext) RunCommand(cmd string) (string, error) {
	return runCommand(me.cli, cmd)
}

func (me *TransactionContext) RunCommands(cmds ...string) ([]string, error) {
	return runCommands(me.cli, cmds...)
}

func (me *TransactionContext) RunShell(filePath string, remotePath string, params ...string) (outputs []string, err error) {
	return runShell(me.cli, filePath, remotePath, params...)
}

func (me *TransactionContext) SendFile(localPath string, remotePath string) error {
	remotePath = filepath.ToSlash(remotePath)
	dir := remotePath[:strings.LastIndex(remotePath, "/")]
	_, err := runCommand(me.cli, "mkdir -p "+dir)
	if err != nil {
		return err
	}
	return sendFile(me.cli, localPath, remotePath)
}

func (me *TransactionContext) SendDir(localDir, remoteDir string) error {
	return sendDir(me.cli, localDir, remoteDir)
}

// 获取目录下所有路径
func getPathList(dir string) (string, []Path, error) {
	sli := []Path{}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", nil, err
	}
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return err
		}
		if info.IsDir() {
			sli = append(sli, Path{path, true})
		} else {
			sli = append(sli, Path{path, false})
		}
		return nil
	})
	return dir, sli, err
}

type Path struct {
	Path  string
	IsDir bool
}
