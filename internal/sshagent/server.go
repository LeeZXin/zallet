package sshagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/LeeZXin/zallet/internal/action"
	"github.com/LeeZXin/zallet/internal/executor"
	"github.com/LeeZXin/zallet/internal/global"
	"github.com/LeeZXin/zallet/internal/hashset"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/LeeZXin/zallet/internal/zssh"
	"github.com/gliderlabs/ssh"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	validWorkflowTaskIdRegexp *regexp.Regexp
	validStageTaskIdRegexp    *regexp.Regexp
)

type handler func(ssh.Session, map[string]string)

type Server struct {
	srv              *zssh.Server
	Token            string
	graphMap         *graphMap
	handlerMap       map[string]handler
	workflowDir      string
	servicesDir      string
	cmdMap           *cmdMap
	workflowExecutor *executor.Executor
	serviceExecutor  *executor.Executor
}

func (s *Server) GetWorkflowBaseDir(taskId string) string {
	yearStr := taskId[:4]
	monthStr := taskId[4:6]
	dayStr := taskId[6:8]
	hourStr := taskId[8:10]
	id := taskId[10:]
	return filepath.Join(s.workflowDir, "action", yearStr, monthStr, dayStr, hourStr, id)
}

func (s *Server) Shutdown() {
	s.srv.Close()
	graphs := s.graphMap.GetAll()
	for _, graph := range graphs {
		graph.Cancel(action.TaskCancelErr)
	}
	cmds := s.cmdMap.GetAll()
	for _, cmd := range cmds {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	time.Sleep(time.Second)
}

type BaseStatus struct {
	Status    Status `json:"status"`
	Duration  int64  `json:"duration"`
	ErrLog    string `json:"errLog"`
	BeginTime int64  `json:"beginTime"`
}

type TaskStatus struct {
	BaseStatus
	JobStatus []JobStatus `json:"jobStatus"`
}

type JobStatus struct {
	JobName string `json:"jobName"`
	BaseStatus
	Steps []StepStatus `json:"steps"`
}

type StepStatus struct {
	StepName string `json:"stepName"`
	BaseStatus
}

type TaskStatusCallbackReq struct {
	Status   Status      `json:"status"`
	Duration int64       `json:"duration"`
	Task     *TaskStatus `json:"task,omitempty"`
}

func getBaseStatus(store Store) BaseStatus {
	var (
		ret BaseStatus
		err error
	)
	ret.Status, ret.Duration, err = store.ReadStatus()
	if err != nil {
		ret.Status = UnknownStatus
	} else {
		if ret.Status == FailStatus {
			content, _ := store.ReadErrLog()
			if len(content) > 0 {
				ret.ErrLog = content
			}
		}
	}
	beginTime, err := store.ReadBeginTime()
	if err == nil {
		ret.BeginTime = beginTime.UnixMilli()
	}
	return ret
}

func getJobStatus(baseDir string, jobName string, jobCfg action.JobCfg) JobStatus {
	jobDir := filepath.Join(baseDir, jobName)
	store := newFileStore(jobDir)
	if !store.IsExists() {
		return JobStatus{
			JobName: jobName,
			BaseStatus: BaseStatus{
				Status: UnExecuted,
			},
		}
	}
	var ret JobStatus
	ret.JobName = jobName
	ret.BaseStatus = getBaseStatus(store)
	ret.Steps = make([]StepStatus, 0, len(jobCfg.Steps))
	for i, step := range jobCfg.Steps {
		ret.Steps = append(ret.Steps, getStepStatus(baseDir, jobName, i, step.Name))
	}
	return ret
}

func getStepStatus(baseDir string, jobName string, index int, stepName string) StepStatus {
	stepDir := filepath.Join(baseDir, jobName, strconv.Itoa(index))
	store := newFileStore(stepDir)
	if !store.IsExists() {
		return StepStatus{
			StepName: stepName,
		}
	}
	var ret StepStatus
	ret.StepName = stepName
	ret.BaseStatus = getBaseStatus(store)
	return ret
}

func getTaskStatus(baseDir string) TaskStatus {
	var ret TaskStatus
	store := newFileStore(baseDir)
	origin, err := store.ReadOrigin()
	if err == nil {
		var p action.GraphCfg
		// 解析yaml
		err = yaml.Unmarshal(origin, &p)
		if err != nil || p.IsValid() != nil {
			return TaskStatus{}
		}
		jobsMap := make(map[string]action.JobCfg, len(p.Jobs))
		for name, cfg := range p.Jobs {
			jobsMap[name] = cfg
		}
		jobsInDfsOrder := dfsOrder(p)
		ret.JobStatus = make([]JobStatus, 0, len(p.Jobs))
		for _, jobName := range jobsInDfsOrder {
			ret.JobStatus = append(ret.JobStatus, getJobStatus(baseDir, jobName, jobsMap[jobName]))
		}
	}
	ret.BaseStatus = getBaseStatus(store)
	return ret
}

// 返回深度优先遍历顺序
func dfsOrder(p action.GraphCfg) []string {
	type jobNode struct {
		name  string
		next  *hashset.HashSet[string]
		needs *hashset.HashSet[string]
	}
	nodesMap := make(map[string]*jobNode, len(p.Jobs))
	// 初始化nodesMap
	for name, cfg := range p.Jobs {
		nodesMap[name] = &jobNode{
			name:  name,
			next:  hashset.NewHashSet[string](),
			needs: hashset.NewHashSet[string](cfg.Needs...),
		}
	}
	// 补充next
	for name, cfg := range p.Jobs {
		for _, need := range cfg.Needs {
			n, b := nodesMap[need]
			if b {
				n.next.Add(name)
			}
		}
	}
	var dfs func(...*jobNode)
	ret := make([]string, 0, len(p.Jobs))
	visited := make(map[string]bool, len(p.Jobs))
	noNeedsLayers := make([]*jobNode, 0)
	for _, node := range nodesMap {
		if node.needs.Size() == 0 {
			noNeedsLayers = append(noNeedsLayers, node)
		}
	}
	dfs = func(nodes ...*jobNode) {
		for _, node := range nodes {
			if visited[node.name] {
				continue
			}
			ret = append(ret, node.name)
			visited[node.name] = true
			if node.next.Size() > 0 {
				nextNodes := make([]*jobNode, 0, node.next.Size())
				node.next.Range(func(n string) {
					nextNodes = append(nextNodes, nodesMap[n])
				})
				nextNodes = util.Filter(nextNodes, func(n *jobNode) bool {
					return n != nil
				})
				if len(nextNodes) > 0 {
					dfs(nextNodes...)
				}
			}
		}
	}
	dfs(noNeedsLayers...)
	return ret
}

func mkdir(dir string) bool {
	return os.MkdirAll(dir, os.ModePerm) == nil
}

func notifyCallback(callbackUrl, token, taskId string, req any) {
	if callbackUrl == "" {
		return
	}
	m, _ := json.Marshal(req)
	request, err := http.NewRequest(http.MethodPost, callbackUrl+"?taskId="+taskId, bytes.NewReader(m))
	if err == nil {
		request.Header.Set("Content-Type", "application/json;charset=utf-8")
		request.Header.Set("Authorization", token)
		resp, err := http.DefaultClient.Do(request)
		if err == nil {
			defer resp.Body.Close()
		} else {
			log.Println(err)
		}
	} else {
		log.Println(err)
	}
}

func StartServer() *Server {
	validWorkflowTaskIdRegexp = regexp.MustCompile(`^\d{10}\S+$`)
	validStageTaskIdRegexp = regexp.MustCompile(`^\S{32}$`)
	agent := new(Server)
	poolSize := global.Viper.GetInt("ssh.agent.workflow.poolSize")
	if poolSize <= 0 {
		poolSize = 10
	}
	queueSize := global.Viper.GetInt("ssh.agent.workflow.queueSize")
	if queueSize <= 0 {
		queueSize = 1024
	}
	agent.workflowExecutor, _ = executor.NewExecutor(poolSize, queueSize, time.Minute, executor.AbortStrategy)
	poolSize = global.Viper.GetInt("ssh.agent.service.poolSize")
	if poolSize <= 0 {
		poolSize = 10
	}
	queueSize = global.Viper.GetInt("ssh.agent.service.queueSize")
	if queueSize <= 0 {
		queueSize = 1024
	}
	agent.serviceExecutor, _ = executor.NewExecutor(poolSize, queueSize, time.Minute, executor.AbortStrategy)
	agent.Token = global.Viper.GetString("ssh.agent.Token")
	agent.graphMap = newGraphMap()
	agent.cmdMap = newCmdMap()
	agent.workflowDir = filepath.Join(global.BaseDir, "workflow")
	agent.servicesDir = filepath.Join(global.BaseDir, "services")
	agent.handlerMap = map[string]handler{
		"getWorkflowStepLog": func(session ssh.Session, args map[string]string) {
			taskId := args["i"]
			if !validWorkflowTaskIdRegexp.MatchString(taskId) {
				returnErrMsg(session, "invalid id")
				return
			}
			jobName := args["j"]
			if !action.ValidJobNameRegexp.MatchString(jobName) {
				returnErrMsg(session, "invalid job name")
				return
			}
			index := args["n"]
			if index == "" {
				returnErrMsg(session, "invalid index")
				return
			}
			stepDir := filepath.Join(agent.GetWorkflowBaseDir(taskId), jobName, index)
			exist, _ := util.IsExist(stepDir)
			if !exist {
				returnErrMsg(session, "unknown step")
				return
			}
			readCloser, err := newFileStore(stepDir).ReadLog()
			if err == nil {
				defer readCloser.Close()
				io.Copy(session, readCloser)
			}
			session.Exit(0)
		},
		"getWorkflowTaskOrigin": func(session ssh.Session, args map[string]string) {
			taskId := args["i"]
			if !validWorkflowTaskIdRegexp.MatchString(taskId) {
				returnErrMsg(session, "invalid id")
				return
			}
			baseDir := agent.GetWorkflowBaseDir(taskId)
			origin, err := newFileStore(baseDir).ReadOrigin()
			if err != nil {
				returnErrMsg(session, "unknown id")
				return
			}
			session.Write(origin)
			session.Exit(0)
		},
		"getWorkflowTaskStatus": func(session ssh.Session, args map[string]string) {
			taskId := args["i"]
			if !validWorkflowTaskIdRegexp.MatchString(taskId) {
				returnErrMsg(session, "invalid id")
				return
			}
			taskStatus := getTaskStatus(agent.GetWorkflowBaseDir(taskId))
			m, _ := json.Marshal(taskStatus)
			fmt.Fprint(session, string(m)+"\n")
			session.Exit(0)
		},
		"executeWorkflow": func(session ssh.Session, args map[string]string) {
			taskId := args["i"]
			if !validWorkflowTaskIdRegexp.MatchString(taskId) {
				returnErrMsg(session, "invalid task id")
				return
			}
			input, err := io.ReadAll(session)
			if err != nil {
				returnErrMsg(session, err.Error())
				return
			}
			var p action.GraphCfg
			// 解析yaml
			err = yaml.Unmarshal(input, &p)
			if err != nil {
				returnErrMsg(session, err.Error())
				return
			}
			graph, err := p.ConvertToGraph()
			if err != nil {
				returnErrMsg(session, err.Error())
				return
			}
			// 环境变量
			envs := util.CutEnv(session.Environ())
			callbackUrl := envs[action.EnvCallBackUrl]
			token := envs[action.EnvCallBackToken]
			now := time.Now()
			logDir := agent.GetWorkflowBaseDir(taskId)
			exist, err := util.IsExist(logDir)
			if err != nil {
				returnErrMsg(session, err.Error())
				return
			}
			if exist {
				returnErrMsg(session, "duplicated task id")
				return
			}
			err = os.MkdirAll(logDir, os.ModePerm)
			if err != nil {
				returnErrMsg(session, err.Error())
				return
			}
			if !agent.graphMap.PutIfAbsent(taskId, graph) {
				// 不太可能会发生
				graph.Cancel(action.TaskCancelErr)
				returnErrMsg(session, "duplicated biz id")
				return
			}
			taskStore := newFileStore(logDir)
			// 首先置为排队状态
			taskStore.StoreStatus(QueueStatus, 0)
			if rErr := agent.workflowExecutor.Execute(func() {
				defer agent.graphMap.Remove(taskId)
				// 写入开始时间
				taskStore.StoreBeginTime(now)
				// 写入原始内容
				taskStore.StoreOrigin(input)
				// 初始状态 执行状态
				taskStore.StoreStatus(RunningStatus, 0)
				// 通知回调
				notifyCallback(callbackUrl, token, taskId, TaskStatusCallbackReq{
					Status: RunningStatus,
				})
				err := graph.Run(action.RunOpts{
					Workdir: filepath.Join(agent.workflowDir, taskId),
					StepOutputFunc: func(stat action.StepOutputStat) {
						defer stat.Output.Close()
						stepDir := filepath.Join(logDir, stat.JobName, strconv.Itoa(stat.Index))
						if mkdir(stepDir) {
							newFileStore(stepDir).StoreLog(stat.Output)
						}
					},
					JobBeforeFunc: func(stat action.JobBeforeStat) error {
						jobDir := filepath.Join(logDir, stat.JobName)
						err := os.MkdirAll(jobDir, os.ModePerm)
						if err == nil {
							jobStore := newFileStore(jobDir)
							// 记录job开始时间
							jobStore.StoreBeginTime(stat.BeginTime)
							// 设置初始状态
							jobStore.StoreStatus(RunningStatus, 0)
						}
						return err
					},
					JobAfterFunc: func(err error, stat action.JobRunStat) {
						jobDir := filepath.Join(logDir, stat.JobName)
						jobStore := newFileStore(jobDir)
						if err == nil {
							jobStore.StoreStatus(SuccessStatus, stat.Duration)
						} else {
							if err == context.DeadlineExceeded {
								jobStore.StoreStatus(TimeoutStatus, stat.Duration)
							} else {
								jobStore.StoreStatus(FailStatus, stat.Duration)
							}
							jobStore.StoreErrLog(err)
						}
					},
					StepAfterFunc: func(err error, stat action.StepRunStat) {
						stepDir := filepath.Join(logDir, stat.JobName, strconv.Itoa(stat.Index))
						if mkdir(stepDir) {
							stepStore := newFileStore(stepDir)
							// 记录step开始时间
							stepStore.StoreBeginTime(stat.BeginTime)
							if err == nil {
								stepStore.StoreStatus(SuccessStatus, stat.Duration)
							} else {
								stepStore.StoreStatus(FailStatus, stat.Duration)
								stepStore.StoreErrLog(err)
							}
						}
					},
					Args: envs,
				})
				if err != nil {
					graph.Cancel(action.TaskCancelErr)
				}
				var status Status
				if err == nil {
					status = SuccessStatus
				} else {
					switch err {
					case context.DeadlineExceeded:
						status = TimeoutStatus
					case action.TaskCancelErr:
						status = CancelStatus
					default:
						status = FailStatus
					}
					taskStore.StoreErrLog(err)
				}
				duration := graph.SinceBeginTime()
				taskStore.StoreStatus(status, duration)
				taskStatus := getTaskStatus(logDir)
				// 通知回调
				notifyCallback(callbackUrl, token, taskId, TaskStatusCallbackReq{
					Status:   status,
					Duration: duration.Milliseconds(),
					Task:     &taskStatus,
				})
			}); rErr != nil {
				agent.graphMap.Remove(taskId)
				returnErrMsg(session, "out of capacity")
				return
			}
			session.Exit(0)
		},
		"killWorkflow": func(session ssh.Session, args map[string]string) {
			taskId := args["i"]
			graph := agent.graphMap.GetById(taskId)
			if graph == nil {
				returnErrMsg(session, "unknown taskId: "+args["i"])
				return
			}
			taskStore := newFileStore(agent.GetWorkflowBaseDir(taskId))
			taskStore.StoreStatus(CancelStatus, graph.SinceBeginTime())
			graph.Cancel(action.TaskCancelErr)
			session.Exit(0)
		},
		"execute": func(session ssh.Session, args map[string]string) {
			service := args["s"]
			if service == "" {
				returnErrMsg(session, "invalid service")
				return
			}
			workdir := filepath.Join(agent.servicesDir, service)
			err := os.MkdirAll(workdir, os.ModePerm)
			if err != nil {
				returnErrMsg(session, err.Error())
				return
			}
			taskId := args["i"]
			if !validStageTaskIdRegexp.MatchString(taskId) {
				returnErrMsg(session, "invalid taskId")
				return
			}
			cmd := agent.cmdMap.GetById(taskId)
			if cmd != nil {
				returnErrMsg(session, "duplicated taskId")
				return
			}
			input := new(bytes.Buffer)
			_, err = io.Copy(input, session)
			if cmd != nil {
				returnErrMsg(session, err.Error())
				return
			}
			script := input.String()
			if strings.Count(script, "\n") > 0 {
				cmd = exec.Command("bash", "-c", script)
			} else {
				fields := strings.Fields(script)
				if len(fields) > 1 {
					cmd = exec.Command(fields[0], fields[1:]...)
				} else {
					cmd = exec.Command(script)
				}
			}
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Setpgid: true,
			}
			cmd.Env = append(os.Environ(), session.Environ()...)
			cmd.Dir = workdir
			cmd.Stdout = session
			cmd.Stderr = session.Stderr()
			if err != nil {
				returnErrMsg(session, err.Error())
				return
			}
			if !agent.cmdMap.PutIfAbsent(taskId, cmd) {
				returnErrMsg(session, "duplicated taskId")
				return
			}
			defer agent.cmdMap.Remove(taskId)
			err = cmd.Start()
			if err != nil {
				returnErrMsg(session, err.Error())
				return
			}
			err = cmd.Wait()
			if err != nil {
				returnErrMsg(session, err.Error())
				return
			}
			session.Exit(0)
		},
		"kill": func(session ssh.Session, args map[string]string) {
			taskId := args["i"]
			if !validStageTaskIdRegexp.MatchString(taskId) {
				returnErrMsg(session, "invalid taskId")
				return
			}
			cmd := agent.cmdMap.GetById(taskId)
			if cmd == nil {
				returnErrMsg(session, "unknown taskId")
				return
			}
			if cmd.Process != nil {
				err := util.KillNegativePid(cmd.Process.Pid)
				if err != nil {
					log.Printf("kill taskId: %s with err: %v", taskId, err)
				}
			}
			session.Exit(0)
		},
	}
	agentHost := global.Viper.GetString("ssh.agent.host")
	if agentHost == "" {
		agentHost = "127.0.0.1:6666"
	}
	if !regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}:\d+$`).MatchString(agentHost) {
		log.Fatalf("invalid agent host: %v", agentHost)
	}
	serv, err := zssh.NewServer(zssh.ServerOpts{
		Host:    agentHost,
		HostKey: filepath.Join(global.BaseDir, "ssh", "sshAgent.rsa"),
		PublicKeyHandler: func(ctx ssh.Context, key ssh.PublicKey) bool {
			if ctx.User() != "zall" {
				return false
			}
			return true
		},
		SessionHandler: func(session ssh.Session) {
			cmd, err := splitCommand(session.RawCommand())
			if err != nil {
				returnErrMsg(session, err.Error())
				return
			}
			fn, b := agent.handlerMap[cmd.Operation]
			if !b {
				returnErrMsg(session, "unrecognized command")
				return
			}
			// token校验
			if cmd.Args["t"] != agent.Token {
				returnErrMsg(session, "invalid Token")
				return
			}
			fn(session, cmd.Args)
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	agent.srv = serv
	return agent
}

func returnErrMsg(session ssh.Session, msg string) {
	fmt.Fprintln(session.Stderr(), msg)
	session.Exit(1)
}
