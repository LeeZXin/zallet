package util

import (
	"syscall"
	"time"
)

func KillNegativePid(pid int) error {
	errChan := make(chan error)
	go func() {
		err := syscall.Kill(-pid, syscall.SIGTERM)
		defer close(errChan)
		if err != nil {
			errChan <- err
		}
	}()
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
		return syscall.Kill(-pid, syscall.SIGKILL)
	case err := <-errChan:
		return err
	}
}
