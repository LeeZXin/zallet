package zallet

import (
	"github.com/LeeZXin/zallet/internal/app"
	"time"
	"xorm.io/xorm"
)

type ServiceModel struct {
	Id             int64     `json:"id" xorm:"pk autoincr"`
	ServiceId      string    `json:"serviceId"`
	Pid            int       `json:"pid"`
	InstanceId     string    `json:"instanceId"`
	App            string    `json:"app"`
	AppYaml        *app.Yaml `json:"appYaml"`
	ServiceStatus  string    `json:"serviceStatus"`
	StatusRevision uint64    `json:"statusRevision"`
	ErrLog         string    `json:"errLog"`
	ProbeTimestamp int64     `json:"probeTimestamp"`
	ProbeFailCount int64     `json:"probeFailCount"`
	AgentHost      string    `json:"agentHost"`
	AgentToken     string    `json:"agentToken"`
	Env            string    `json:"env"`
	CpuPercent     int       `json:"cpuPercent"`
	MemPercent     int       `json:"memPercent"`
	Created        time.Time `json:"created" xorm:"created"`
}

func (*ServiceModel) TableName() string {
	return "zallet_service"
}

func insertServiceModel(session *xorm.Session, service *ServiceModel) error {
	_, err := session.Insert(service)
	return err
}

func updateServiceStatus(session *xorm.Session, serviceId string, serviceStatus string, statusRevision uint64, errLog string) (bool, error) {
	rows, err := session.
		Where("service_id = ?", serviceId).
		And("status_revision < ?", statusRevision).
		Cols("service_status", "status_revision", "err_log").
		Update(&ServiceModel{
			ServiceStatus:  serviceStatus,
			StatusRevision: statusRevision,
			ErrLog:         errLog,
		})
	return rows == 1, err
}

func killServiceStatus(session *xorm.Session, serviceId string, statusRevision uint64) (bool, error) {
	rows, err := session.
		Where("service_id = ?", serviceId).
		And("status_revision < ?", statusRevision).
		Cols("service_status", "status_revision", "err_log", "cpu_percent", "mem_percent").
		Update(&ServiceModel{
			ServiceStatus:  app.KilledServiceStatus,
			StatusRevision: statusRevision,
			ErrLog:         "",
			CpuPercent:     0,
			MemPercent:     0,
		})
	return rows == 1, err
}

func getServiceByServiceIdAndInstanceId(session *xorm.Session, serviceId, instanceId string) (ServiceModel, bool, error) {
	var ret ServiceModel
	b, err := session.
		Where("service_id = ?", serviceId).
		And("instance_id = ?", instanceId).
		Get(&ret)
	return ret, b, err
}

func deleteServiceByServiceId(session *xorm.Session, serviceId string) (bool, error) {
	rows, err := session.
		Where("service_id = ?", serviceId).
		Delete(new(ServiceModel))
	return rows == 1, err
}

func updateServiceProbe(session *xorm.Session, serviceId string, eventTime int64, failCount int64) (bool, error) {
	rows, err := session.
		Where("service_id = ?", serviceId).
		Cols("probe_timestamp", "probe_fail_count").
		Update(&ServiceModel{
			ProbeTimestamp: eventTime,
			ProbeFailCount: failCount,
		})
	return rows == 1, err
}

func updateCpuAndMemPercent(session *xorm.Session, serviceId string, cpuPercent, memPercent int) (bool, error) {
	rows, err := session.Where("service_id = ?", serviceId).
		Cols("cpu_percent", "mem_percent").
		Update(&ServiceModel{
			CpuPercent: cpuPercent,
			MemPercent: memPercent,
		})
	return rows == 1, err
}
