package app

type ReportProbeReq struct {
	ServiceId string `json:"serviceId"`
	EventTime int64  `json:"eventTime"`
	IsSuccess bool   `json:"isSuccess"`
	FailCount int64  `json:"failCount"`
	Pid       int    `json:"pid"`
}

const (
	StartingServiceStatus = "starting"
	RunningServiceStatus  = "running"
	FailedServiceStatus   = "failed"
	ShutdownServiceStatus = "shutdown"
	KilledServiceStatus   = "killed"
)

type ReportStatusReq struct {
	ServiceId string `json:"serviceId"`
	Pid       int    `json:"pid"`
	EventTime int64  `json:"eventTime"`
	Status    string `json:"status"`
	Revision  uint64 `json:"revision"`
	ErrLog    string `json:"errLog"`
}

type ReportDaemonReq struct {
	ServiceId string `json:"serviceId"`
	Pid       int    `json:"pid"`
	EventTime int64  `json:"eventTime"`
}

type ReportDaemonResp struct {
	Exist   bool   `json:"exist"`
	Message string `json:"message"`
}

type ServiceVO struct {
	ServiceId     string `json:"serviceId"`
	App           string `json:"app"`
	Env           string `json:"env"`
	ServiceStatus string `json:"serviceStatus"`
	Pid           int    `json:"pid"`
	AgentHost     string `json:"agentHost"`
}
