package sshagent

import (
	"bufio"
	"fmt"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/spf13/cast"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	RunningStatus = "running"
	SuccessStatus = "success"
	FailStatus    = "fail"
	TimeoutStatus = "timeout"
	CancelStatus  = "cancel"
	QueueStatus   = "queue"
	UnknownStatus = "unknown"
	UnExecuted    = "unExecuted"
)

const (
	originFileName = "origin"
	statusFileName = "status"
	beginFileName  = "begin"
	errLogFileName = "error.log"
	logFileName    = "log"
)

func toStatusMsg(status string, duration time.Duration) string {
	return fmt.Sprintf("%s %d", status, duration.Milliseconds())
}

func toStatusMsgBytes(status string, duration time.Duration) []byte {
	return []byte(toStatusMsg(status, duration))
}

type Store interface {
	IsExists() bool
	StoreStatus(string, time.Duration) error
	ReadStatus() (string, int64, error)
	StoreBeginTime(time.Time) error
	ReadBeginTime() (time.Time, error)
	StoreErrLog(error) error
	ReadErrLog() (string, error)
	StoreOrigin([]byte) error
	ReadOrigin() ([]byte, error)
	StoreLog(io.Reader) error
	ReadLog() (io.ReadCloser, error)
}

type fileStore struct {
	BaseDir string
}

func newFileStore(dir string) Store {
	return &fileStore{
		BaseDir: dir,
	}
}

func (s *fileStore) IsExists() bool {
	ret, _ := util.IsExist(s.BaseDir)
	return ret
}

func (s *fileStore) StoreStatus(status string, duration time.Duration) error {
	return os.WriteFile(filepath.Join(s.BaseDir, statusFileName),
		toStatusMsgBytes(status, duration),
		os.ModePerm,
	)
}

func (s *fileStore) ReadStatus() (string, int64, error) {
	content, err := os.ReadFile(filepath.Join(s.BaseDir, statusFileName))
	if err != nil {
		return "", 0, err
	}
	status, duration := convertStatusFileContent(content)
	return status, duration, nil
}

func (s *fileStore) StoreBeginTime(beginTime time.Time) error {
	return os.WriteFile(filepath.Join(s.BaseDir, beginFileName),
		[]byte(strconv.FormatInt(beginTime.UnixMilli(), 10)),
		os.ModePerm,
	)
}

func (s *fileStore) ReadBeginTime() (time.Time, error) {
	content, err := os.ReadFile(filepath.Join(s.BaseDir, beginFileName))
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(cast.ToInt64(string(content))), nil
}

func (s *fileStore) StoreErrLog(err error) error {
	return os.WriteFile(filepath.Join(s.BaseDir, errLogFileName),
		[]byte(err.Error()),
		os.ModePerm,
	)
}

func (s *fileStore) ReadErrLog() (string, error) {
	content, err := os.ReadFile(filepath.Join(s.BaseDir, errLogFileName))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (s *fileStore) StoreOrigin(input []byte) error {
	return os.WriteFile(filepath.Join(s.BaseDir, originFileName),
		input,
		os.ModePerm,
	)
}

func (s *fileStore) ReadOrigin() ([]byte, error) {
	return os.ReadFile(filepath.Join(s.BaseDir, originFileName))
}

func (s *fileStore) StoreLog(reader io.Reader) error {
	var logFile *os.File
	// 记录日志
	logFile, err := os.OpenFile(filepath.Join(s.BaseDir, logFileName), os.O_APPEND|os.O_WRONLY|os.O_CREATE, os.ModePerm)
	if err == nil {
		defer logFile.Close()
		// 增加缓存
		writer := bufio.NewWriter(logFile)
		defer writer.Flush()
		_, err = io.Copy(writer, reader)
	}
	return err
}

func (s *fileStore) ReadLog() (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.BaseDir, logFileName))
}

func convertStatusFileContent(content []byte) (string, int64) {
	fields := strings.Fields(strings.TrimSpace(string(content)))
	if len(fields) != 2 {
		return UnknownStatus, 0
	}
	return fields[0], cast.ToInt64(fields[1])
}
