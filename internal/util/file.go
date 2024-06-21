package util

import (
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const windowsSharingViolationError syscall.Errno = 32

func IsExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil || os.IsExist(err) {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// RemoveAll removes the named file or (empty) directory with at most 5 attempts.
func RemoveAll(name string) error {
	b, err := IsExist(name)
	if err != nil {
		return err
	}
	if !b {
		return nil
	}
	for i := 0; i < 5; i++ {
		err = os.RemoveAll(name)
		if err == nil {
			break
		}
		unwrapped := err.(*os.PathError).Err
		if unwrapped == syscall.EBUSY || unwrapped == syscall.ENOTEMPTY || unwrapped == syscall.EPERM || unwrapped == syscall.EMFILE || unwrapped == syscall.ENFILE {
			// try again
			<-time.After(100 * time.Millisecond)
			continue
		}
		if unwrapped == windowsSharingViolationError && runtime.GOOS == "windows" {
			// try again
			<-time.After(100 * time.Millisecond)
			continue
		}
		if unwrapped == syscall.ENOENT {
			// it's already gone
			return nil
		}
	}
	return err
}

func CutEnv(envs []string) map[string]string {
	ret := make(map[string]string, len(envs))
	for _, env := range envs {
		k, v, b := strings.Cut(env, "=")
		if b {
			ret[k] = v
		}
	}
	return ret
}
