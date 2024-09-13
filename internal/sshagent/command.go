package sshagent

import (
	"context"
	"fmt"
	"github.com/gliderlabs/ssh"
	"github.com/kballard/go-shellquote"
	"io"
	"os"
	"os/exec"
	"syscall"
)

func executeCommand(line string, session ssh.Session, workdir string) error {
	cmd, err := newCommand(session.Context(), line, session, session.Stderr(), workdir, session.Environ())
	if err != nil {
		return err
	}
	return cmd.Run()
}

func newCommand(ctx context.Context, line string, stdout, stderr io.Writer, workdir string, envs []string) (*exec.Cmd, error) {
	fields, err := shellquote.Split(line)
	if err != nil {
		return nil, err
	}
	var cmd *exec.Cmd
	if len(fields) > 1 {
		cmd = exec.CommandContext(ctx, fields[0], fields[1:]...)
	} else if len(fields) == 1 {
		cmd = exec.CommandContext(ctx, fields[0])
	} else {
		return nil, fmt.Errorf("empty command")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Env = append(os.Environ(), envs...)
	cmd.Dir = workdir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd, nil
}

type command struct {
	Args      map[string]string
	Operation string
}

func splitCommand(cmd string) (command, error) {
	fields, err := shellquote.Split(cmd)
	if err != nil {
		return command{}, err
	}
	flen := len(fields)
	if flen == 0 {
		return command{}, fmt.Errorf("unregonized command: %v", cmd)
	}
	args := make(map[string]string)
	ret := command{
		Operation: fields[0],
		Args:      args,
	}
	i := 1
	for i < flen {
		if fields[i][0] == '-' && i < flen-1 && fields[i+1][0] != '-' {
			args[fields[i][1:]] = fields[i+1]
			i++
		}
		i++
	}
	return ret, nil
}
