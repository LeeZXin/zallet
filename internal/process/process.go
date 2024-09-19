package process

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

type Status string

const (
	StartingStatus Status = "starting"
	RunningStatus  Status = "running"
	StoppingStatus Status = "stopping"
	StoppedStatus  Status = "stopped"
)

type Process struct {
	Cmd     *exec.Cmd
	errChan chan error
}

func (p *Process) Kill() error {
	if p.Cmd.Process != nil {
		return util.KillNegativePid(p.Cmd.Process.Pid)
	}
	return nil
}

func (p *Process) GetPid() int {
	if p == nil {
		return 0
	}
	proc := p.Cmd.Process
	if proc != nil {
		return proc.Pid
	}
	return 0
}

func (p *Process) Wait() error {
	select {
	case err, ok := <-p.errChan:
		if !ok {
			return errors.New("process is down")
		}
		return err
	}
}

func RunProcess(workDir, script string, envs []string, stdin io.Reader) (*Process, error) {
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
	cmd.Dir = workDir
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
	ret := &Process{
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
