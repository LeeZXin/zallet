package util

import (
	"syscall"
	"time"
)

func KillPid(pid int) error {
	errChan := make(chan error)
	go func() {
		err := syscall.Kill(-pid, syscall.SIGTERM)
		defer close(errChan)
		if err != nil {
			errChan <- err
		}
	}()
	t := time.NewTimer(30 * time.Second)
	defer t.Stop()
	select {
	case <-t.C:
		return syscall.Kill(-pid, syscall.SIGKILL)
	case err := <-errChan:
		return err
	}
}
