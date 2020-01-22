package command

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/kballard/go-shellquote"
	"golang.org/x/crypto/ssh"
)

func Copy(size int64, mode os.FileMode, fileName string, contents io.Reader, destinationPath string, session *ssh.Session) error {
	return copy(size, mode, fileName, contents, destinationPath, session)
}

func CopyPath(filePath string, destinationPath string, session *ssh.Session) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	s, err := f.Stat()
	if err != nil {
		return err
	}
	return copy(s.Size(), s.Mode().Perm(), path.Base(filePath), f, destinationPath, session)
}

func copy(size int64, mode os.FileMode, fileName string, contents io.Reader, destination string, session *ssh.Session) error {
	w, err := session.StdinPipe()
	if err != nil {
		return err
	}
	w.Close()

	cmd := shellquote.Join("scp", "-t", destination)
	if err := session.Start(cmd); err != nil {
		return err
	}
	errors := make(chan error)
	go func() {
		errors <- session.Wait()
	}()
	fmt.Fprintf(w, "C%#o %d %s \n", mode, size, fileName)
	io.Copy(w, contents)
	fmt.Fprint(w, "\x00")
	return <-errors
}
