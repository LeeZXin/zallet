package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/LeeZXin/zallet/internal/reexec"
	"github.com/LeeZXin/zallet/internal/util"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

type Service struct {
	serviceId       string
	serviceDir      string
	tempDir         string
	pid             int
	ctx             context.Context
	cancelCauseFunc context.CancelCauseFunc
	serviceCmd      atomic.Value
	probeRevision   atomic.Uint64
	statusRevision  atomic.Uint64
	httpClient      *http.Client
	startTime       int64
	ShutdownChan    chan struct{}
}

type ServiceOpts struct {
	ServiceId string            `json:"serviceId"`
	Yaml      Yaml              `json:"yaml"`
	BaseDir   string            `json:"baseDir"`
	SockFile  string            `json:"sockFile"`
	Envs      map[string]string `json:"envs"`
}

func (o *ServiceOpts) IsValid() error {
	if o.ServiceId == "" {
		return errors.New("empty serviceId")
	}
	if err := o.Yaml.IsValid(); err != nil {
		return err
	}
	if o.BaseDir == "" || !filepath.IsAbs(o.BaseDir) {
		return fmt.Errorf("invalid baseDir: %s", o.BaseDir)
	}
	if o.SockFile == "" || !filepath.IsAbs(o.SockFile) {
		return fmt.Errorf("invalid sockFile: %s", o.SockFile)
	}
	return nil
}

func RunService(opts ServiceOpts) (*Service, error) {
	if err := opts.IsValid(); err != nil {
		return nil, err
	}
	serviceDir := filepath.Join(opts.BaseDir, opts.Yaml.App)
	tempDir := filepath.Join(serviceDir, "temp")
	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		return nil, err
	}
	ctx, cancelCauseFunc := context.WithCancelCause(context.Background())
	srv := &Service{
		serviceId:       opts.ServiceId,
		serviceDir:      serviceDir,
		tempDir:         tempDir,
		pid:             os.Getpid(),
		ctx:             ctx,
		cancelCauseFunc: cancelCauseFunc,
		startTime:       time.Now().UnixMilli(),
		httpClient:      util.NewUnixHttpClient(opts.SockFile),
		ShutdownChan:    make(chan struct{}),
	}
	// 上报守护进程
	go func() {
		for ctx.Err() == nil {
			time.Sleep(10 * time.Second)
			srv.reportDaemon()
		}
	}()
	// 启动服务
	go func() {
		srv.reportStatus(StartingServiceStatus, nil)
		cmd, err2 := reexec.RunAsyncCommand(
			tempDir,
			opts.Yaml.Start,
			util.MergeEnvs(opts.Envs),
			nil,
			nil,
			false,
		)
		if err2 != nil {
			srv.reportStatus(FailedServiceStatus, err2)
		} else {
			srv.reportStatus(RunningServiceStatus, nil)
			srv.serviceCmd.Store(cmd)
			err2 = cmd.Wait()
			if err2 == nil {
				srv.reportStatus(ShutdownServiceStatus, nil)
			} else {
				srv.reportStatus(FailedServiceStatus, err2)
			}
		}
	}()
	// 心跳检查
	if opts.Yaml.Probe != nil {
		go func() {
			onFailTimes := 0
			var failed int64 = 0
			if opts.Yaml.Probe.OnFail != nil {
				onFailTimes = opts.Yaml.Probe.OnFail.Times
			}
			for ctx.Err() == nil {
				time.Sleep(10 * time.Second)
				probeRet := runProbe(opts.Yaml.Probe)
				if probeRet {
					failed = 0
				} else {
					failed += 1
				}
				srv.reportProbe(probeRet, failed)
				if onFailTimes > 0 && failed > 0 && failed%int64(onFailTimes) == 0 {
					srv.reportStatus(RestartServiceStatus, fmt.Errorf("probe failed: %v", failed))
					// 执行心跳失败脚本
					reexec.ExecCommand(
						tempDir,
						opts.Yaml.Probe.OnFail.Action,
						util.MergeEnvs(opts.Envs),
						nil,
						nil,
					)
				}
			}
		}()
	}
	return srv, nil
}

func (s *Service) reportProbe(isSuccess bool, failCount int64) {
	req := ReportProbeReq{
		ServiceId: s.serviceId,
		EventTime: time.Now().UnixMilli(),
		Revision:  s.probeRevision.Add(1),
		IsSuccess: isSuccess,
		Pid:       s.pid,
		FailCount: failCount,
	}
	m, _ := json.Marshal(req)
	resp, err := s.httpClient.Post(
		"http://fake/api/reportProbe",
		"application/json;charset=utf-8",
		bytes.NewReader(m),
	)
	if err == nil {
		resp.Body.Close()
	}
}

func (s *Service) reportDaemon() {
	req := ReportDaemonReq{
		ServiceId: s.serviceId,
		Pid:       s.pid,
		EventTime: time.Now().UnixMilli(),
	}
	m, _ := json.Marshal(req)
	resp, err := s.httpClient.Post(
		"http://fake/api/reportDaemon",
		"application/json;charset=utf-8",
		bytes.NewReader(m),
	)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return
			}
			var rdr ReportDaemonResp
			err = json.Unmarshal(body, &rdr)
			if err != nil {
				return
			}
			// 以防默认值
			if rdr.Exist == "false" {
				s.Shutdown(errors.New(rdr.Message))
				s.ShutdownChan <- struct{}{}
			}
		}
	}
}

func (s *Service) reportStatus(status string, err error) {
	req := ReportStatusReq{
		ServiceId: s.serviceId,
		Pid:       s.pid,
		EventTime: time.Now().UnixMilli(),
		Status:    status,
		Revision:  s.statusRevision.Add(1),
	}
	if err != nil {
		req.ErrLog = err.Error()
	}
	m, _ := json.Marshal(req)
	resp, err := s.httpClient.Post(
		"http://fake/api/reportStatus",
		"application/json;charset=utf-8",
		bytes.NewReader(m),
	)
	if err == nil {
		resp.Body.Close()
	}
}

func (s *Service) Shutdown(err error) {
	s.reportStatus(ShutdownServiceStatus, nil)
	s.cancelCauseFunc(err)
	srv := s.serviceCmd.Load()
	if srv != nil {
		srv.(*reexec.AsyncCommand).Kill()
	}
}
