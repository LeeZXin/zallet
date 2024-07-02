package reexec

import (
	"bytes"
	"errors"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/google/uuid"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type AsyncCommand struct {
	Cmd              *exec.Cmd
	errChan          chan error
	closeErrChanOnce sync.Once
}

func (p *AsyncCommand) Kill() error {
	pro := p.Cmd.Process
	if pro != nil {
		return util.KillNegativePid(pro.Pid)
	}
	return nil
}

func (p *AsyncCommand) Wait() error {
	select {
	case err, _ := <-p.errChan:
		return err
	}
}

func RunAsyncCommand(workDir, script string, envs []string, stdin io.Reader, stdout io.WriteCloser) (*AsyncCommand, error) {
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
		defer os.Remove(cmdPath)
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
	ret := &AsyncCommand{
		Cmd:     cmd,
		errChan: make(chan error),
	}
	go func() {
		cmd.Stdout = stdout
		if stdout != nil {
			defer stdout.Close()
		}
		defer close(ret.errChan)
		err2 := cmd.Run()
		if stderr.Len() > 0 {
			err2 = errors.New(stderr.String())
		}
		if err2 != nil {
			log.Printf("run [%s] failed with err: %v", script, err2)
		}
		transferErr := func(e error) {
			defer func() {
				recover()
			}()
			ret.errChan <- e
		}
		transferErr(err2)
	}()
	for {
		select {
		case err := <-ret.errChan:
			for err == nil && cmd.Process == nil {
				time.Sleep(time.Second)
			}
			return ret, err
		default:
			if cmd.Process != nil {
				return ret, nil
			}
		}
		time.Sleep(time.Second)
	}
}
