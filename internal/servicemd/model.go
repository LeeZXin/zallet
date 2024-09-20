package servicemd

import (
	"github.com/LeeZXin/zallet/internal/process"
	"time"
	"xorm.io/xorm"
)

type Service struct {
	Id            int64         `json:"id" xorm:"pk autoincr"`
	ServiceId     string        `json:"serviceId"`
	Pid           int           `json:"pid"`
	InstanceId    string        `json:"instanceId"`
	App           string        `json:"app"`
	AppYaml       *process.Yaml `json:"appYaml"`
	ServiceStatus string        `json:"serviceStatus"`
	ErrLog        string        `json:"errLog"`
	AgentHost     string        `json:"agentHost"`
	AgentToken    string        `json:"agentToken"`
	Env           string        `json:"env"`
	CpuPercent    int           `json:"cpuPercent"`
	MemPercent    int           `json:"memPercent"`
	EventTime     int64         `json:"eventTime"`
	Created       time.Time     `json:"created" xorm:"created"`
}

func (*Service) TableName() string {
	return "zallet_service"
}

func InsertService(session *xorm.Session, service *Service) error {
	_, err := session.Insert(service)
	return err
}

func UpdateServiceStatus(session *xorm.Session, eventTime int64, serviceId string, serviceStatus string, errLog string, cpuPercent, memPercent int) (bool, error) {
	rows, err := session.
		Where("service_id = ?", serviceId).
		And("event_time < ?", eventTime).
		Cols("service_status", "err_log", "cpu_percent", "mem_percent", "event_time").
		Update(&Service{
			ServiceStatus: serviceStatus,
			ErrLog:        errLog,
			CpuPercent:    cpuPercent,
			MemPercent:    memPercent,
			EventTime:     eventTime,
		})
	return rows == 1, err
}

func GetServiceByServiceIdAndInstanceId(session *xorm.Session, serviceId, instanceId string) (Service, bool, error) {
	var ret Service
	b, err := session.
		Where("service_id = ?", serviceId).
		And("instance_id = ?", instanceId).
		Get(&ret)
	return ret, b, err
}

func DeleteServiceByServiceId(session *xorm.Session, serviceId string) (bool, error) {
	rows, err := session.
		Where("service_id = ?", serviceId).
		Delete(new(Service))
	return rows == 1, err
}
