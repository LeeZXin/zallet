package action

import (
	"context"
	"errors"
	"fmt"
	"github.com/LeeZXin/zallet/internal/executor/completable"
	"github.com/LeeZXin/zallet/internal/hashset"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/kballard/go-shellquote"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	ThereHasBug        = errors.New("there has bug")
	ValidJobNameRegexp = regexp.MustCompile(`^\S+$`)
	TaskCancelErr      = errors.New("task cancelled")
)

type GraphCfg struct {
	Jobs map[string]JobCfg `json:"jobs" yaml:"jobs"`
}

func (c *GraphCfg) String() string {
	return fmt.Sprintf("jobs: %v", c.Jobs)
}

func (c *GraphCfg) IsValid() error {
	if len(c.Jobs) == 0 {
		return errors.New("empty jobs")
	}
	allJobNames := hashset.NewHashSet[string]()
	// 检查是否有重复的jobName
	for jobName, cfg := range c.Jobs {
		if !ValidJobNameRegexp.MatchString(jobName) {
			return fmt.Errorf("invalid jobName: %s", jobName)
		}
		if err := cfg.IsValid(); err != nil {
			return err
		}
		// 有重复的名字
		if allJobNames.Contains(jobName) {
			return fmt.Errorf("job has duplicated Alias: %v", jobName)
		}
		allJobNames.Add(jobName)
	}
	// 检查job needs
	for k, cfg := range c.Jobs {
		for _, n := range cfg.Needs {
			b := allJobNames.Contains(n)
			// 检查jobNeeds 是否存在
			if !b {
				return fmt.Errorf("job node does not exist: %v", n)
			}
			// 检查jobNeeds是否指向自己
			if n == k {
				return fmt.Errorf("job needs point to itself: %v", n)
			}
		}
	}
	// 检查job是否有环
	return c.checkRoundJob()
}

type jobTemp struct {
	Name  string
	Needs *hashset.HashSet[string]
	Next  *hashset.HashSet[string]
}

func newJobTemp(name string) *jobTemp {
	return &jobTemp{
		Name:  name,
		Needs: hashset.NewHashSet[string](),
		Next:  hashset.NewHashSet[string](),
	}
}

func (c *GraphCfg) checkRoundJob() error {
	tmap := make(map[string]*jobTemp, len(c.Jobs))
	for k, cfg := range c.Jobs {
		t := newJobTemp(k)
		if len(cfg.Needs) > 0 {
			t.Needs.Add(cfg.Needs...)
		}
		tmap[k] = t
	}
	for k, cfg := range c.Jobs {
		for _, need := range cfg.Needs {
			tmap[need].Next.Add(k)
		}
	}
	// 寻找深度优先遍历开始节点
	starts := make([]string, 0)
	for k, temp := range tmap {
		if temp.Next.Size() == 0 {
			starts = append(starts, k)
		}
	}
	// 深度优先遍历
	for _, start := range starts {
		if err := c.dfs([]string{}, tmap[start], tmap); err != nil {
			return err
		}
	}
	return nil
}

func (c *GraphCfg) dfs(path []string, t *jobTemp, all map[string]*jobTemp) error {
	if util.FindInSlice(path, t.Name) {
		return fmt.Errorf("round job: %v %v", path, t.Name)
	}
	p := append(path[:], t.Name)
	for _, key := range t.Needs.AllKeys() {
		if err := c.dfs(p, all[key], all); err != nil {
			return err
		}
	}
	return nil
}

func (c *GraphCfg) ConvertToGraph() (*Graph, error) {
	if c.IsValid() != nil {
		return nil, errors.New("invalid action yaml content")
	}
	// 转换jobs
	jobs := make([]*job, 0, len(c.Jobs))
	for k, j := range c.Jobs {
		jobs = append(jobs, j.convertToJob(k))
	}
	graphJobs(jobs, c.Jobs)
	return &Graph{
		allJobs: jobs,
	}, nil
}

type StepCfg struct {
	Name    string            `json:"name" yaml:"name"`
	Secrets []string          `json:"secrets"`
	With    map[string]string `json:"with" yaml:"with"`
	Script  string            `json:"script" yaml:"script"`
}

func (c *StepCfg) convertToStep() *step {
	cpyMap := make(map[string]string, len(c.With))
	for k, v := range c.With {
		cpyMap[k] = v
	}
	return &step{
		Name:   c.Name,
		With:   cpyMap,
		Script: c.Script,
	}
}

func (c *StepCfg) IsValid() error {
	if c.Script == "" {
		return errors.New("empty Action")
	}
	return nil
}

type JobCfg struct {
	Needs   []string  `json:"needs" yaml:"needs"`
	Steps   []StepCfg `json:"steps" yaml:"steps"`
	Timeout string    `json:"timeout" yaml:"timeout"`
}

func (c *JobCfg) String() string {
	return fmt.Sprintf("needs: %v, steps: %v", c.Needs, c.Steps)
}

func (c *JobCfg) IsValid() error {
	if len(c.Steps) == 0 {
		return errors.New("empty steps")
	}
	for _, cfg := range c.Steps {
		if err := cfg.IsValid(); err != nil {
			return err
		}
	}
	return nil
}

func (c *JobCfg) convertToJob(jobName string) *job {
	steps := make([]*step, 0, len(c.Steps))
	for _, s := range c.Steps {
		steps = append(steps, s.convertToStep())
	}
	ctx := context.Background()
	timeout, _ := time.ParseDuration(c.Timeout)
	var (
		cancelFn context.CancelFunc
	)
	if timeout > 0 {
		ctx, cancelFn = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancelFn = context.WithCancel(ctx)
	}
	return &job{
		name:     jobName,
		steps:    steps,
		needs:    hashset.NewHashSet[*job](),
		next:     hashset.NewHashSet[*job](),
		ctx:      ctx,
		cancelFn: cancelFn,
	}
}

type RunOpts struct {
	Workdir        string
	JobBeforeFunc  func(JobBeforeStat) error
	JobAfterFunc   func(error, JobRunStat)
	StepOutputFunc func(StepOutputStat)
	StepAfterFunc  func(error, StepRunStat)
	Args           map[string]string
}

type StepRunStat struct {
	JobName   string
	Index     int
	Duration  time.Duration
	BeginTime time.Time
}

type StepOutputStat struct {
	JobName   string
	Index     int
	BeginTime time.Time
	Output    io.ReadCloser
}

type JobRunStat struct {
	JobName   string
	Duration  time.Duration
	BeginTime time.Time
}

type JobBeforeStat struct {
	JobName   string
	BeginTime time.Time
}

type Graph struct {
	allJobs   []*job
	lock      sync.Mutex
	beginTime atomic.Value
}

func (g *Graph) ListJobInfo() []JobInfo {
	ret := util.Map(g.allJobs, func(t *job) JobInfo {
		steps := make([]StepInfo, 0, len(t.steps))
		for i, s := range t.steps {
			steps = append(steps, StepInfo{
				Index: i,
				Name:  s.Name,
			})
		}
		return JobInfo{
			Name:  t.name,
			Steps: steps,
		}
	})
	return ret
}

func (g *Graph) Run(opts RunOpts) error {
	// 记录开始时间
	g.beginTime.Store(time.Now())
	if opts.Workdir != "" {
		err := os.Mkdir(opts.Workdir, os.ModePerm)
		if err != nil {
			return err
		}
		defer util.RemoveAll(opts.Workdir)
	}
	futures := make(map[string]completable.Future[any])
	// 找到最后一层节点
	layers := util.Filter(g.allJobs, func(j *job) bool {
		return j.next.Size() == 0
	})
	finalFutures := util.Map(layers, func(t *job) completable.IBase {
		return loadJob(futures, t, &opts)
	})
	if len(finalFutures) > 0 {
		// 最后一层的节点就可以不用异步
		future := completable.ThenAllOf(finalFutures...)
		_, err := future.Get()
		return err
	}
	// finalLayers必须大于0 不应该会走到这 否则就是bug
	return ThereHasBug
}

func (g *Graph) SinceBeginTime() time.Duration {
	val := g.beginTime.Load()
	if val == nil {
		return 0
	}
	return time.Since(val.(time.Time))
}

func (g *Graph) Cancel(err error) {
	for _, j := range g.allJobs {
		j.Cancel(err)
	}
}

// loadJob 递归调用 从后置节点往前置节点递归整个graph
func loadJob(all map[string]completable.Future[any], j *job, opts *RunOpts) completable.Future[any] {
	// 防止重复执行
	f, b := all[j.name]
	if b {
		return f
	}
	if j.needs.Size() == 0 {
		all[j.name] = completable.CallAsync(func() (any, error) {
			return nil, j.Run(opts)
		})
	} else {
		needs := make([]completable.IBase, 0, j.needs.Size())
		j.needs.Range(func(j *job) {
			needs = append(needs, loadJob(all, j, opts))
		})
		all[j.name] = completable.CallAsync(func() (any, error) {
			// 等待前置节点执行完，还得执行自己
			allOfAsync := completable.ThenAllOf(needs...)
			_, err := allOfAsync.Get()
			if err != nil {
				return nil, err
			}
			return nil, j.Run(opts)
		})
	}
	return all[j.name]
}

type step struct {
	Name   string
	With   map[string]string
	Script string
	sync.Mutex
	curr   *exec.Cmd
	killed bool
}

func (s *step) SetCurr(cmd *exec.Cmd) bool {
	s.Lock()
	defer s.Unlock()
	if !s.killed {
		s.curr = cmd
	}
	return !s.killed
}

func (s *step) Kill() {
	s.Lock()
	defer s.Unlock()
	s.killed = true
	if s.curr != nil {
		if s.curr.Process != nil {
			log.Printf("kill pid: %v with err: %v", s.curr.Process.Pid, syscall.Kill(-s.curr.Process.Pid, syscall.SIGKILL))
		}
	}
}

func (s *step) Run(opts *RunOpts, j *job, index int) error {
	err := j.getErr()
	beginTime := time.Now()
	reader, writer := io.Pipe()
	go opts.StepOutputFunc(StepOutputStat{
		JobName:   j.name,
		Index:     index,
		BeginTime: beginTime,
		Output:    reader,
	})
	if err == nil {
		cmdPath := filepath.Join(opts.Workdir, util.RandomUuid())
		err = os.WriteFile(cmdPath, []byte(s.Script), os.ModePerm)
		if err == nil {
			defer util.RemoveAll(cmdPath)
			err = executeCommand(j.ctx, "chmod +x "+cmdPath, nil, nil, opts.Workdir, nil)
			if err == nil {
				var cmd *exec.Cmd
				cmd, err = newCommand(j.ctx, "bash -c "+cmdPath, writer, writer, opts.Workdir, mergeEnvs(s.With, opts.Args))
				if err == nil {
					if s.SetCurr(cmd) {
						err = cmd.Run()
					} else {
						err = TaskCancelErr
					}
				}
			}
		}
	}
	if err != nil {
		writer.Write([]byte(err.Error()))
	}
	writer.Close()
	s.SetCurr(nil)
	endTime := time.Now()
	opts.StepAfterFunc(err, StepRunStat{
		JobName:   j.name,
		Index:     index,
		Duration:  endTime.Sub(beginTime),
		BeginTime: beginTime,
	})
	return err
}

func mergeEnvs(args, with map[string]string) []string {
	ret := make([]string, 0, len(args)+len(with))
	for k, v := range args {
		ret = append(ret, k+"="+v)
	}
	for k, v := range with {
		ret = append(ret, k+"="+v)
	}
	return ret
}

func executeCommand(ctx context.Context, line string, stdout, stderr io.Writer, workdir string, envs []string) error {
	cmd, err := newCommand(ctx, line, stdout, stderr, workdir, envs)
	if err != nil {
		return err
	}
	return cmd.Run()
}

func newCommand(ctx context.Context, line string, stdout, stderr io.Writer, workdir string, envs []string) (*exec.Cmd, error) {
	fields, err := shellquote.Split(line)
	if err != nil {
		return nil, err
	}
	var cmd *exec.Cmd
	if len(fields) > 1 {
		cmd = exec.CommandContext(ctx, fields[0], fields[1:]...)
	} else if len(fields) == 1 {
		cmd = exec.CommandContext(ctx, fields[0])
	} else {
		return nil, fmt.Errorf("empty command")
	}
	if len(envs) > 0 {
		cmd.Env = envs
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Dir = workdir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd, nil
}

type job struct {
	name       string
	steps      []*step
	needs      *hashset.HashSet[*job]
	next       *hashset.HashSet[*job]
	ctx        context.Context
	customErr  atomic.Value
	cancelFn   context.CancelFunc
	cancelOnce sync.Once
}

func (j *job) getErr() error {
	val := j.customErr.Load()
	if val != nil {
		return val.(error)
	}
	return j.ctx.Err()
}

func (j *job) Run(opts *RunOpts) error {
	var err error
	beginTime := time.Now()
	err = opts.JobBeforeFunc(JobBeforeStat{
		JobName:   j.name,
		BeginTime: beginTime,
	})
	if err == nil {
		// 是否配置了超时
		_, b := j.ctx.Deadline()
		if b {
			go func() {
				select {
				case <-j.ctx.Done():
					if j.ctx.Err() == context.DeadlineExceeded {
						j.Cancel(context.DeadlineExceeded)
					}
				}
			}()
		}
		for i, s := range j.steps {
			err = j.getErr()
			if err == nil {
				err = s.Run(opts, j, i)
				if err != nil {
					break
				}
			} else {
				break
			}
		}
		val := j.customErr.Load()
		if val != nil {
			err = val.(error)
		}
		opts.JobAfterFunc(err, JobRunStat{
			JobName:   j.name,
			Duration:  time.Since(beginTime),
			BeginTime: beginTime,
		})
	}
	j.Cancel(err)
	return err
}

func (j *job) Cancel(err error) {
	j.cancelOnce.Do(func() {
		if err != nil {
			j.customErr.Store(err)
		}
		for _, s := range j.steps {
			s.Kill()
		}
	})
}

func graphJobs(jobs []*job, c map[string]JobCfg) {
	m := make(map[string]*job)
	for _, j := range jobs {
		m[j.name] = j
	}
	for k, cfg := range c {
		for _, need := range cfg.Needs {
			m[k].needs.Add(m[need])
			m[need].next.Add(m[k])
		}
	}
}

type JobInfo struct {
	Name  string
	Steps []StepInfo
}

type StepInfo struct {
	Index int
	Name  string
}
