package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/LeeZXin/zallet/internal/reexec"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/shirou/gopsutil/v3/process"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

type Service struct {
	opts            ServiceOpts
	serviceId       string
	serviceDir      string
	logDir          string
	tempDir         string
	pid             int
	ctx             context.Context
	cancelCauseFunc context.CancelCauseFunc
	serviceCmd      atomic.Value
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
	logDir := filepath.Join(serviceDir, "log")
	err := os.MkdirAll(logDir, os.ModePerm)
	if err != nil {
		return nil, err
	}
	tempDir := filepath.Join(serviceDir, "temp")
	err = os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		return nil, err
	}
	ctx, cancelCauseFunc := context.WithCancelCause(context.Background())
	srv := &Service{
		opts:            opts,
		serviceId:       opts.ServiceId,
		serviceDir:      serviceDir,
		logDir:          logDir,
		tempDir:         tempDir,
		pid:             os.Getpid(),
		ctx:             ctx,
		cancelCauseFunc: cancelCauseFunc,
		startTime:       time.Now().UnixMilli(),
		httpClient:      util.NewUnixHttpClient(opts.SockFile),
		ShutdownChan:    make(chan struct{}),
	}
	// 上报守护进程
	go srv.runDaemon()
	// 启动服务
	go srv.start()
	// 心跳检查
	if opts.Yaml.Probe != nil {
		go srv.runProbe()
	}
	go srv.runStat()
	return srv, nil
}

func (s *Service) runDaemon() {
	for s.ctx.Err() == nil {
		time.Sleep(2 * time.Second)
		s.reportDaemon()
	}
}

func (s *Service) runStat() {
	for s.ctx.Err() == nil {
		time.Sleep(time.Second)
		s.reportStat()
	}
}

func (s *Service) reportStat() {
	req := ReportStatReq{
		ServiceId: s.serviceId,
	}
	val := s.serviceCmd.Load()
	if val != nil {
		cmd := val.(*reexec.AsyncCommand)
		if cmd.Cmd.Process != nil {
			pcs, err := process.NewProcess(int32(cmd.Cmd.Process.Pid))
			if err == nil {
				cpuPercent, err := pcs.CPUPercent()
				if err == nil {
					req.CpuPercent = int(cpuPercent * 100)
				}
				memPercent, err := pcs.MemoryPercent()
				if err == nil {
					req.MemPercent = int(memPercent * 100)
				}
			}
		}
	}
	m, _ := json.Marshal(req)
	resp, err := s.httpClient.Post(
		"http://fake/api/v1/reportStat",
		"application/json;charset=utf-8",
		bytes.NewReader(m),
	)
	if err == nil {
		resp.Body.Close()
	} else {
		log.Printf("reportStat failed with err: %v", err)
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
		"http://fake/api/v1/reportDaemon",
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
			if !rdr.Exist {
				s.Shutdown(errors.New(rdr.Message), true)
				s.ShutdownChan <- struct{}{}
			}
		}
	} else {
		log.Printf("reportDaemon failed with err: %v", err)
	}
}

func (s *Service) reportProbe(isSuccess bool, failCount int64) {
	req := ReportProbeReq{
		ServiceId: s.serviceId,
		EventTime: time.Now().UnixMilli(),
		IsSuccess: isSuccess,
		Pid:       s.pid,
		FailCount: failCount,
	}
	m, _ := json.Marshal(req)
	resp, err := s.httpClient.Post(
		"http://fake/api/v1/reportProbe",
		"application/json;charset=utf-8",
		bytes.NewReader(m),
	)
	if err == nil {
		resp.Body.Close()
	} else {
		log.Printf("reportProbe failed with err: %v", err)
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
		"http://fake/api/v1/reportStatus",
		"application/json;charset=utf-8",
		bytes.NewReader(m),
	)
	if err == nil {
		resp.Body.Close()
	}
}

func (s *Service) start() {
	s.reportStatus(StartingServiceStatus, nil)
	logger, _ := os.Create(filepath.Join(s.logDir, s.serviceId+".log"))
	cmd, err2 := reexec.RunAsyncCommand(
		s.tempDir,
		s.opts.Yaml.Start,
		util.MergeEnvs(s.opts.Envs),
		nil,
		logger,
	)
	if err2 != nil {
		s.reportStatus(FailedServiceStatus, err2)
	} else {
		s.reportStatus(RunningServiceStatus, nil)
		s.serviceCmd.Store(cmd)
		err2 = cmd.Wait()
		if err2 == nil {
			s.reportStatus(ShutdownServiceStatus, nil)
		} else {
			s.reportStatus(FailedServiceStatus, err2)
		}
	}
}

func (s *Service) Shutdown(err error, shouldKill bool) {
	s.reportStatus(ShutdownServiceStatus, err)
	s.cancelCauseFunc(err)
	if shouldKill {
		srv := s.serviceCmd.Load()
		if srv != nil {
			srv.(*reexec.AsyncCommand).Kill()
		}
	}
}

func (s *Service) runProbe() {
	var failed int64 = 0
	for s.ctx.Err() == nil {
		time.Sleep(2 * time.Second)
		probeRet := runProbe(s.opts.Yaml.Probe)
		if probeRet {
			failed = 0
		} else {
			failed += 1
		}
		s.reportProbe(probeRet, failed)
		if failed > 0 && failed%5 == 0 {
			// 重启服务
			s.restart()
			failed = 0
		}
	}
}

func (s *Service) restart() {
	srv := s.serviceCmd.Load()
	if srv != nil {
		s.serviceCmd.Store((*reexec.AsyncCommand)(nil))
		srv.(*reexec.AsyncCommand).Kill()
	}
	s.start()
}
