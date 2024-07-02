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
	"path/filepath"
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

func (s *Server) KillService(serviceId string) error {
	session := s.xengine.NewSession()
	defer session.Close()
	srv, b, err := getServiceByServiceId(session, serviceId)
	if err != nil {
		return err
	}
	if !b {
		return fmt.Errorf("%s is not found", serviceId)
	}
	if srv.InstanceId != s.instanceId {
		return fmt.Errorf("%s belongs to instance: %s", serviceId, srv.InstanceId)
	}
	deleteServiceByServiceId(session, serviceId)
	log.Printf("kill service: %v pid: %v with err: %v", serviceId, srv.Pid, util.KillNegativePid(srv.Pid))
	return nil
}

func (s *Server) ReportDaemon(req app.ReportDaemonReq) error {
	session := s.xengine.NewSession()
	defer session.Close()
	srv, b, err := getServiceByServiceId(session, req.ServiceId)
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
	serviceId := util.RandomUuid()
	var cmdRet *reexec.AsyncCommand
	opts := app.ServiceOpts{
		ServiceId: serviceId,
		Yaml:      appYaml,
		BaseDir:   s.baseDir,
		SockFile:  s.sockFile,
		Envs:      nil,
	}
	m, _ := json.Marshal(opts)
	_, err := s.xengine.Transaction(func(session *xorm.Session) (any, error) {
		var err2 error
		cmdRet, err2 = reexec.RunAsyncCommand(
			s.tempDir,
			fmt.Sprintf("%s service", s.appPath),
			nil,
			bytes.NewReader(m),
			true,
			filepath.Join(s.logDir, serviceId+".log"),
		)
		if err2 != nil {
			return nil, err2
		}
		if cmdRet == nil {
			return nil, errors.New("run command failed")
		}
		md := &ServiceModel{
			Pid:        cmdRet.Cmd.Process.Pid,
			ServiceId:  serviceId,
			InstanceId: s.instanceId,
			App:        appYaml.App,
			AppYaml:    &appYaml,
			Env:        appYaml.Env,
			AgentHost:  s.sshHost,
			AgentToken: s.sshToken,
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
		req.Revision,
	)
	if err != nil {
		log.Printf("updateServiceStatus :%v failed with err: %v", req.ServiceId, err)
	}
}
