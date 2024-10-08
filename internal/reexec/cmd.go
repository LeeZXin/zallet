package reexec

import (
	"bytes"
	"errors"
	"github.com/LeeZXin/zallet/internal/util"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

var (
	ClosedErr = errors.New("closed")
)

type AsyncCommand struct {
	Cmd     *exec.Cmd
	errChan chan error
}

func (p *AsyncCommand) Kill() error {
	if p.Cmd.Process != nil {
		err := util.KillNegativePid(p.Cmd.Process.Pid)
		if err == nil {
			return p.Wait()
		}
		return err
	}
	return nil
}

func (p *AsyncCommand) GetPid() int {
	if p.Cmd.Process != nil {
		return p.Cmd.Process.Pid
	}
	return 0
}

func (p *AsyncCommand) Wait() error {
	select {
	case err, ok := <-p.errChan:
		if !ok {
			return ClosedErr
		}
		return err
	}
}

func RunAsyncCommand(workdir, script string, envs []string, stdin io.Reader) (*AsyncCommand, error) {
	if script == "" {
		return nil, errors.New("empty script")
	}
	var cmd *exec.Cmd
	if strings.Count(script, "\n") > 0 {
		cmd = exec.Command("bash", "-c", script)
	} else {
		fields := strings.Fields(script)
		if len(fields) > 1 {
			cmd = exec.Command(fields[0], fields[1:]...)
		} else {
			cmd = exec.Command(script)
		}
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	stderr := new(bytes.Buffer)
	cmd.Dir = workdir
	cmd.Stdin = stdin
	cmd.Stderr = stderr
	if len(envs) > 0 {
		cmd.Env = append(os.Environ(), envs...)
	} else {
		cmd.Env = os.Environ()
	}
	// 先启动命令
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	ret := &AsyncCommand{
		Cmd:     cmd,
		errChan: make(chan error, 1),
	}
	go func() {
		defer close(ret.errChan)
		err2 := cmd.Wait()
		if err2 != nil && stderr.Len() > 0 {
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
	return ret, nil
}
