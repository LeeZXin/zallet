package zallet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/LeeZXin/zallet/internal/app"
	"github.com/LeeZXin/zallet/internal/reexec"
	"github.com/LeeZXin/zallet/internal/util"
	"log"
	"os"
	"path/filepath"
	"time"
	"xorm.io/xorm"
)

func (s *Server) ReportStatus(req app.ReportStatusReq) {
	session := s.xengine.NewSession()
	defer session.Close()
	_, err := updateServiceStatus(
		session,
		req.ServiceId,
		req.Status,
		req.Revision,
		req.ErrLog,
	)
	if err != nil {
		log.Printf("updateServiceStatus :%v failed with err: %v", req.ServiceId, err)
	}
}

func (s *Server) ReportStat(req app.ReportStatReq) {
	session := s.xengine.NewSession()
	defer session.Close()
	_, err := updateCpuAndMemPercent(
		session,
		req.ServiceId,
		req.CpuPercent,
		req.MemPercent,
	)
	if err != nil {
		log.Printf("updateCpuAndMemPercent :%v failed with err: %v", req.ServiceId, err)
	}
}

func (s *Server) KillService(serviceId string) error {
	session := s.xengine.NewSession()
	defer session.Close()
	srv, b, err := getServiceByServiceIdAndInstanceId(session, serviceId, s.instanceId)
	if err != nil {
		return err
	}
	if !b {
		return fmt.Errorf("%s is not found", serviceId)
	}
	r := srv.StatusRevision
	for {
		r++
		b, err = killServiceStatus(session, serviceId, r)
		if err != nil {
			log.Printf("updateServiceStatus srv: %s failed with err: %v", serviceId, err)
			return err
		}
		if b {
			log.Printf("kill service: %v pid: %v with err: %v", serviceId, srv.Pid, util.KillNegativePid(srv.Pid))
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *Server) DeleteService(serviceId string) (*app.Yaml, error) {
	session := s.xengine.NewSession()
	defer session.Close()
	srv, b, err := getServiceByServiceIdAndInstanceId(session, serviceId, s.instanceId)
	if err != nil {
		return nil, err
	}
	if !b {
		return nil, fmt.Errorf("%s is not found", serviceId)
	}
	_, err = deleteServiceByServiceId(session, serviceId)
	if err != nil {
		return nil, err
	}
	log.Printf("kill service: %v pid: %v with err: %v", serviceId, srv.Pid, util.KillNegativePid(srv.Pid))
	return srv.AppYaml, nil
}

func (s *Server) RestartService(serviceId string) error {
	appYaml, err := s.DeleteService(serviceId)
	if err != nil {
		return err
	}
	if appYaml == nil {
		return fmt.Errorf("fail to restart service: %v", serviceId)
	}
	return s.ApplyAppYaml(*appYaml)
}

func (s *Server) LsService(appId string, global bool, status string) ([]app.ServiceVO, error) {
	session := s.xengine.NewSession()
	defer session.Close()
	if !global {
		session.Where("instance_id = ?", s.instanceId)
	}
	if appId != "" {
		session.And("app = ?", appId)
	}
	if status != "" {
		session.And("service_status = ?", status)
	}
	ret := make([]ServiceModel, 0)
	err := session.Desc("created").Find(&ret)
	if err != nil {
		return nil, err
	}
	voList := make([]app.ServiceVO, 0, len(ret))
	for _, md := range ret {
		voList = append(voList, app.ServiceVO{
			ServiceId:     md.ServiceId,
			App:           md.App,
			Env:           md.Env,
			ServiceStatus: md.ServiceStatus,
			Pid:           md.Pid,
			AgentHost:     md.AgentHost,
		})
	}
	return voList, nil
}

func (s *Server) ReportDaemon(req app.ReportDaemonReq) error {
	session := s.xengine.NewSession()
	defer session.Close()
	srv, b, err := getServiceByServiceIdAndInstanceId(session, req.ServiceId, s.instanceId)
	// 数据库的错误忽略
	if err != nil {
		log.Printf("updateServiceStatus :%v failed with err: %v", req.ServiceId, err)
		return nil
	}
	if !b || srv.Pid != req.Pid {
		return fmt.Errorf("unknown service id: %s", req.ServiceId)
	}
	return nil
}

func (s *Server) ApplyAppYaml(appYaml app.Yaml) error {
	now := time.Now().Format("20060102150405")
	serviceId := now + util.RandomUuid()[14:]
	var cmdRet *reexec.AsyncCommand
	opts := app.ServiceOpts{
		ServiceId: serviceId,
		Yaml:      appYaml,
		BaseDir:   s.baseDir,
		SockFile:  s.sockFile,
		Envs:      nil,
	}
	m, _ := json.Marshal(opts)
	dir := filepath.Join(s.logDir, now[:4], now[4:6], now[6:8])
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return err
	}
	logger, err := os.Create(filepath.Join(dir, serviceId+".log"))
	if err != nil {
		return err
	}
	_, err = s.xengine.Transaction(func(session *xorm.Session) (any, error) {
		var err2 error
		cmdRet, err2 = reexec.RunAsyncCommand(
			s.tempDir,
			fmt.Sprintf("%s service", s.appPath),
			nil,
			bytes.NewReader(m),
			logger,
		)
		if err2 != nil {
			return nil, err2
		}
		if cmdRet == nil {
			return nil, errors.New("run command failed")
		}
		md := &ServiceModel{
			Pid:           cmdRet.Cmd.Process.Pid,
			ServiceId:     serviceId,
			ServiceStatus: app.PendingServiceStatus,
			InstanceId:    s.instanceId,
			App:           appYaml.App,
			AppYaml:       &appYaml,
			Env:           appYaml.Env,
			AgentHost:     s.sshHost,
			AgentToken:    s.sshToken,
		}
		return nil, insertServiceModel(session, md)
	})
	if err != nil && cmdRet != nil {
		cmdRet.Kill()
	}
	return err
}

func (s *Server) ReportProbe(req app.ReportProbeReq) {
	session := s.xengine.NewSession()
	defer session.Close()
	var probeTs int64
	if req.IsSuccess {
		probeTs = req.EventTime
	}
	_, err := updateServiceProbe(
		session,
		req.ServiceId,
		probeTs,
		req.FailCount,
	)
	if err != nil {
		log.Printf("updateServiceStatus :%v failed with err: %v", req.ServiceId, err)
	}
}
