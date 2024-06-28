package reexec

import (
	"bytes"
	"errors"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/google/uuid"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

func ExecCommand(workDir, script string, envs []string, stdin io.Reader, stdout io.Writer) error {
	if script == "" {
		return errors.New("empty script")
	}
	cmdPath := filepath.Join(workDir, uuid.NewString())
	err := os.WriteFile(cmdPath, []byte(script), os.ModePerm)
	if err != nil {
		return err
	}
	defer os.RemoveAll(cmdPath)
	cmd := exec.Command("chmod", "+x", cmdPath)
	err = cmd.Run()
	if err != nil {
		return err
	}
	cmd = exec.Command("bash", "-c", cmdPath)
	stderr := new(bytes.Buffer)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if len(envs) > 0 {
		cmd.Env = append(os.Environ(), envs...)
	} else {
		cmd.Env = os.Environ()
	}
	err = cmd.Run()
	if stderr.Len() > 0 {
		return errors.New(stderr.String())
	}
	return err
}

type AsyncCommand struct {
	Cmd              *exec.Cmd
	errChan          chan error
	closeErrChanOnce sync.Once
}

func (p *AsyncCommand) Kill() error {
	return util.KillNegativePid(p.Cmd.Process.Pid)
}

func (p *AsyncCommand) Wait() error {
	select {
	case err, _ := <-p.errChan:
		return err
	}
}

func RunAsyncCommand(workDir, script string, envs []string, stdin io.Reader, stdout io.Writer, setPgid bool) (*AsyncCommand, error) {
	if script == "" {
		return nil, errors.New("empty script")
	}
	var (
		cmd     *exec.Cmd
		cmdPath string
	)
	if strings.Count(script, "\n") > 0 {
		cmdPath = filepath.Join(workDir, uuid.NewString())
		err := os.WriteFile(cmdPath, []byte(script), os.ModePerm)
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(cmdPath)
		cmd = exec.Command("chmod", "+x", cmdPath)
		err = cmd.Run()
		if err != nil {
			return nil, err
		}
		cmd = exec.Command("bash", "-c", cmdPath)
	} else {
		fields := strings.Fields(script)
		if len(fields) > 0 {
			cmd = exec.Command(fields[0], fields[1:]...)
		} else {
			cmd = exec.Command(fields[0])
		}
	}
	if setPgid {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	}
	stderr := new(bytes.Buffer)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if len(envs) > 0 {
		cmd.Env = append(os.Environ(), envs...)
	} else {
		cmd.Env = os.Environ()
	}
	ret := &AsyncCommand{
		Cmd:     cmd,
		errChan: make(chan error),
	}
	go func() {
		defer close(ret.errChan)
		err2 := cmd.Run()
		if stderr.Len() > 0 {
			err2 = errors.New(stderr.String())
		}
		transferErr := func(e error) {
			defer func() {
				recover()
			}()
			ret.errChan <- e
		}
		transferErr(err2)
	}()
	for cmd.Process == nil {
		time.Sleep(time.Second)
	}
	return ret, nil
}
