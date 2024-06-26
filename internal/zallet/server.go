package zallet

import (
	"context"
	"github.com/LeeZXin/zallet/internal/sshagent"
	"github.com/LeeZXin/zallet/internal/static"
	"github.com/LeeZXin/zallet/internal/util"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
	"xorm.io/xorm"
)

var (
	server *Server
)

type Server struct {
	baseDir                    string
	tempDir                    string
	instanceId                 string
	sockFile                   string
	xengine                    *xorm.Engine
	httpClient                 *http.Client
	httpServer                 *http.Server
	appPath                    string
	checkServiceTaskCancelFunc context.CancelFunc
}

func Init(baseDir string) {
	server = new(Server)
	if baseDir == "" {
		server.baseDir = "/usr/local/zallet"
	} else {
		var err error
		server.baseDir, err = filepath.Abs(baseDir)
		if err != nil {
			log.Fatalf("find absolute baseDir failed with err: %v", err)
		}
	}
	log.Printf("baseDir is %s", server.baseDir)
	// 加载配置
	static.Init(server.baseDir)
	server.tempDir = filepath.Join(server.baseDir, "temp")
	err := os.MkdirAll(server.tempDir, os.ModePerm)
	if err != nil {
		log.Fatalf("MkdirAll %s failed with err: %v", server.tempDir, err)
	}
	server.initXorm()
	server.readInstanceId()
	server.httpClient = &http.Client{
		Timeout: time.Second,
	}
	server.appPath = util.GetAppPath()
	server.startHttpServer()
	// 反向检查服务是否存在
	go server.checkServiceExist()
	// ssh server
	agent := sshagent.NewAgentServer(server.baseDir)
	agent.Start()
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	agent.Shutdown()
	server.shutdown()
}

func (s *Server) readInstanceId() {
	instanceFilePath := filepath.Join(s.baseDir, "instance")
	file, err := os.ReadFile(instanceFilePath)
	if err == nil {
		id := string(file)
		if regexp.MustCompile(`^\w{32}$`).MatchString(id) {
			s.instanceId = id
			log.Printf("read from instance file instaceId: %s", s.instanceId)
			return
		}
	} else if !os.IsNotExist(err) {
		log.Fatalf("Read instance file failed with err: %v", err)
	}
	s.instanceId = util.RandomUuid()
	err = os.WriteFile(instanceFilePath, []byte(s.instanceId), os.ModePerm)
	if err != nil {
		log.Fatalf("write instance file failed with err:%v", err)
	}
	log.Printf("instaceId: %s", s.instanceId)
}
