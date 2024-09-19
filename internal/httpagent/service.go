package httpagent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/LeeZXin/zallet/internal/global"
	"github.com/LeeZXin/zallet/internal/process"
	"github.com/LeeZXin/zallet/internal/reexec"
	"github.com/LeeZXin/zallet/internal/servicemd"
	"github.com/LeeZXin/zallet/internal/util"
	"log"
	"time"
	"xorm.io/xorm"
)

func doReportStatus(req global.ReportStatusReq) {
	session := global.Xengine.NewSession()
	defer session.Close()
	_, err := servicemd.UpdateServiceStatus(
		session,
		req.EventTime,
		req.ServiceId,
		req.Status,
		req.ErrLog,
		req.CpuPercent,
		req.MemPercent,
	)
	if err != nil {
		log.Printf("updateServiceStatus :%v failed with err: %v", req.ServiceId, err)
	}
}

func doKillService(serviceId string) error {
	session := global.Xengine.NewSession()
	defer session.Close()
	srv, b, err := servicemd.GetServiceByServiceIdAndInstanceId(session, serviceId, global.InstanceId)
	if err != nil {
		return err
	}
	if !b {
		return fmt.Errorf("%s is not found", serviceId)
	}
	err = util.KillNegativePid(srv.Pid)
	if err == nil {
		log.Printf("kill service: %v pid: %v", serviceId, srv.Pid)
	}
	return err
}

func doDeleteService(serviceId string) (*process.Yaml, error) {
	session := global.Xengine.NewSession()
	defer session.Close()
	srv, b, err := servicemd.GetServiceByServiceIdAndInstanceId(session, serviceId, global.InstanceId)
	if err != nil {
		return nil, err
	}
	if !b {
		return nil, fmt.Errorf("%s is not found", serviceId)
	}
	_, err = servicemd.DeleteServiceByServiceId(session, serviceId)
	if err != nil {
		return nil, err
	}
	util.KillNegativePid(srv.Pid)
	log.Printf("delete service: %v pid: %v", serviceId, srv.Pid)
	return srv.AppYaml, nil
}

func doRestartService(serviceId string) error {
	appYaml, err := doDeleteService(serviceId)
	if err != nil {
		return err
	}
	if appYaml == nil {
		return fmt.Errorf("fail to restart service: %v", serviceId)
	}
	return doApplyAppYaml(*appYaml)
}

func doLsService(appId string, all bool, status string) ([]global.ServiceVO, error) {
	session := global.Xengine.NewSession()
	defer session.Close()
	if !all {
		session.Where("instance_id = ?", global.InstanceId)
	}
	if appId != "" {
		session.And("app = ?", appId)
	}
	if status != "" {
		session.And("service_status = ?", status)
	}
	ret := make([]servicemd.Service, 0)
	err := session.Desc("created").Find(&ret)
	if err != nil {
		return nil, err
	}
	voList := make([]global.ServiceVO, 0, len(ret))
	for _, md := range ret {
		voList = append(voList, global.ServiceVO{
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

func doApplyAppYaml(appYaml process.Yaml) error {
	serviceId := util.RandomUuid()[:16]
	var cmdRet *reexec.AsyncCommand
	opts := process.ServiceOpts{
		ServiceId: serviceId,
		Yaml:      appYaml,
		BaseDir:   global.BaseDir,
		SockFile:  global.SockFile,
	}
	m, _ := json.Marshal(opts)
	_, err := global.Xengine.Transaction(func(session *xorm.Session) (any, error) {
		var err2 error
		cmdRet, err2 = reexec.RunAsyncCommand(
			global.BaseDir,
			global.AppPath+" service",
			nil,
			bytes.NewReader(m),
		)
		if err2 != nil {
			return nil, err2
		}
		if cmdRet == nil {
			return nil, errors.New("run command failed")
		}
		md := &servicemd.Service{
			Pid:           cmdRet.Cmd.Process.Pid,
			ServiceId:     serviceId,
			ServiceStatus: string(process.StartingStatus),
			InstanceId:    global.InstanceId,
			App:           appYaml.App,
			AppYaml:       &appYaml,
			Env:           appYaml.Env,
			AgentHost:     global.SshHost,
			AgentToken:    global.SshToken,
			EventTime:     time.Now().UnixMilli(),
		}
		return nil, servicemd.InsertService(session, md)
	})
	if err != nil && cmdRet != nil {
		cmdRet.Kill()
	}
	return err
}
