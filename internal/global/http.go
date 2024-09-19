package global

type ReportStatusReq struct {
	ServiceId  string `json:"serviceId"`
	Pid        int    `json:"pid"`
	ProcessPid int    `json:"processPid"`
	EventTime  int64  `json:"eventTime"`
	Status     string `json:"status"`
	ErrLog     string `json:"errLog"`
	CpuPercent int    `json:"cpuPercent"`
	MemPercent int    `json:"memPercent"`
}

type ServiceVO struct {
	ServiceId     string `json:"serviceId"`
	App           string `json:"app"`
	Env           string `json:"env"`
	ServiceStatus string `json:"serviceStatus"`
	Pid           int    `json:"pid"`
	AgentHost     string `json:"agentHost"`
}
