package process

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/LeeZXin/zallet/internal/global"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/shirou/gopsutil/v3/process"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Supervisor struct {
	opts           ServiceOpts
	pid            int
	procCancelFunc context.CancelFunc
	supvCancelFunc context.CancelFunc
	httpClient     *http.Client
	startTime      time.Time
	locker         sync.Mutex
	process        *Process
	processRunning bool
	isRunning      bool
	ShutdownChan   chan struct{}
}

func NewSupervisor(opts ServiceOpts) *Supervisor {
	return &Supervisor{
		opts:         opts,
		pid:          os.Getpid(),
		startTime:    time.Now(),
		httpClient:   util.NewUnixHttpClient(opts.SockFile),
		ShutdownChan: make(chan struct{}),
		isRunning:    true,
	}
}

type ServiceOpts struct {
	ServiceId string `json:"serviceId"`
	Yaml      Yaml   `json:"yaml"`
	BaseDir   string `json:"baseDir"`
	SockFile  string `json:"sockFile"`
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

func (s *Supervisor) healthCheck() bool {
	resp, err := s.httpClient.Get("http://fake/api/v1/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *Supervisor) reportStatus(status Status, err error) {
	req := global.ReportStatusReq{
		ServiceId:  s.opts.ServiceId,
		Pid:        s.pid,
		ProcessPid: s.process.GetPid(),
		EventTime:  time.Now().UnixMilli(),
		Status:     string(status),
	}
	if err != nil {
		req.ErrLog = err.Error()
	}
	if status == RunningStatus {
		pid := s.process.GetPid()
		if pid > 0 {
			pcs, err := process.NewProcess(int32(pid))
			if err == nil {
				cpuPercent, err := pcs.CPUPercent()
				if err == nil {
					req.CpuPercent = int(cpuPercent)
				}
				memPercent, err := pcs.MemoryPercent()
				if err == nil {
					req.MemPercent = int(memPercent)
				}
			}
		}
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

func (s *Supervisor) Run() error {
	var ctx context.Context
	ctx, s.supvCancelFunc = context.WithCancel(context.Background())
	// 启动心跳检查
	if s.opts.Yaml.Probe != nil {
		go s.runProbe(ctx)
	}
	// 启动后端健康检查
	go s.runHealthCheck(ctx)
	return s.startProcess()
}

func (s *Supervisor) startProcess() error {
	s.locker.Lock()
	defer s.locker.Unlock()
	if !s.isRunning {
		return errors.New("supervisor closed")
	}
	if s.processRunning {
		return nil
	}
	var ctx context.Context
	ctx, s.procCancelFunc = context.WithCancel(context.Background())
	s.reportStatus(StartingStatus, nil)
	// 执行启动命令
	proc, err := RunProcess(
		s.opts.Yaml.Workdir,
		s.opts.Yaml.Start,
		util.MergeEnvs(s.opts.Yaml.With),
		nil,
	)
	if err != nil {
		return err
	}
	s.process = proc
	s.reportStatus(RunningStatus, nil)
	s.processRunning = true
	go s.reportCpuAndMem(ctx)
	go s.waitProcessStopped(proc)
	return nil
}

func (s *Supervisor) reportCpuAndMem(ctx context.Context) {
	fn := func() {
		s.locker.Lock()
		defer s.locker.Unlock()
		if s.processRunning {
			s.reportStatus(RunningStatus, nil)
		}
	}
	time.Sleep(5 * time.Second)
	for ctx.Err() == nil {
		fn()
		time.Sleep(5 * time.Second)
	}
}

func (s *Supervisor) waitProcessStopped(process *Process) error {
	err := process.Wait()
	s.locker.Lock()
	defer s.locker.Unlock()
	// 并发问题可能导致不是原来的进程
	if process != s.process {
		return nil
	}
	if !s.isRunning {
		return errors.New("supervisor closed")
	}
	if !s.processRunning {
		return nil
	}
	s.processRunning = false
	s.process = nil
	s.procCancelFunc()
	s.reportStatus(StoppedStatus, err)
	return nil
}

func (s *Supervisor) KillProcess() error {
	s.locker.Lock()
	defer s.locker.Unlock()
	if !s.isRunning {
		return errors.New("supervisor closed")
	}
	s.killProcess()
	return nil
}

func (s *Supervisor) killProcess() {
	if s.processRunning {
		s.reportStatus(StoppingStatus, nil)
		s.process.Kill()
		s.processRunning = false
		s.process = nil
		s.procCancelFunc()
		s.reportStatus(StoppedStatus, nil)
	}
}

func (s *Supervisor) RestartProcess() error {
	err := s.KillProcess()
	if err == nil {
		return s.startProcess()
	}
	return err
}

func (s *Supervisor) Shutdown() error {
	s.locker.Lock()
	defer s.locker.Unlock()
	if !s.isRunning {
		return errors.New("supervisor closed")
	}
	s.killProcess()
	s.isRunning = false
	if s.supvCancelFunc != nil {
		s.supvCancelFunc()
	}
	return nil
}

func (s *Supervisor) runHealthCheck(ctx context.Context) {
	var failed int64 = 0
	for ctx.Err() == nil {
		if s.healthCheck() {
			failed = 0
		} else {
			failed += 1
		}
		if failed > 3 {
			log.Println("health check failed!!shutdown supervisor")
			// 关闭整个supervisor
			s.ShutdownChan <- struct{}{}
			return
		}
		time.Sleep(3 * time.Second)
	}
}

func (s *Supervisor) runProbe(ctx context.Context) {
	var failed int64 = 0
	delay, err := time.ParseDuration(s.opts.Yaml.Probe.Delay)
	if err != nil || delay < time.Second {
		delay = 10 * time.Second
	}
	interval, err := time.ParseDuration(s.opts.Yaml.Probe.Interval)
	if err != nil || interval < time.Second {
		interval = 5 * time.Second
	}
	log.Printf("%s run probe delay: %v interval: %v", s.opts.ServiceId, s.opts.Yaml.Probe.Delay, s.opts.Yaml.Probe.Interval)
	time.Sleep(delay)
	for ctx.Err() == nil {
		if s.opts.Yaml.Probe.run() {
			failed = 0
		} else {
			failed += 1
		}
		if failed > 0 && failed%3 == 0 {
			// 重启服务
			s.RestartProcess()
			failed = 0
		}
		time.Sleep(interval)
	}
}
