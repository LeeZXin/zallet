package zallet

import (
	"context"
	"github.com/LeeZXin/zallet/internal/app"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func registerHandler(e *gin.Engine) {
	group := e.Group("/api/v1")
	{
		// 查询服务
		group.GET("/ls", lsService)
		// 删除服务
		group.PUT("/delete/:serviceId", deleteService)
		// 杀死服务
		group.PUT("/kill/:serviceId", killService)
		// 杀死服务
		group.PUT("/restart/:serviceId", restartService)
		// 探针
		group.GET("/health", health)
		// 启动服务
		group.POST("/apply", applyAppYaml)
		// 来自服务的上报
		group.POST("/reportDaemon", reportDaemon)
		// 上报状态
		group.POST("/reportStatus", reportStatus)
		// 上报探针
		group.POST("/reportProbe", reportProbe)
		// 上报cpu和内存
		group.POST("/reportStat", reportStat)
	}
}

func (s *Server) shutdown() {
	if s.httpServer != nil {
		s.httpServer.Shutdown(context.Background())
	}
	if s.checkServiceTaskCancelFunc != nil {
		s.checkServiceTaskCancelFunc()
	}
	s.xengine.Close()
}

func (s *Server) checkServiceExist() {
	ctx, cancelFunc := context.WithCancel(context.Background())
	s.checkServiceTaskCancelFunc = cancelFunc
	for ctx.Err() == nil {
		time.Sleep(10 * time.Second)
		err := s.xengine.
			Where("instance_id = ?", s.instanceId).
			Iterate(new(ServiceModel), func(_ int, bean interface{}) error {
				sm := bean.(*ServiceModel)
				err := exec.Command("kill", "-0", strconv.Itoa(sm.Pid)).Run()
				// 理应被kill 但进程还存在的
				if sm.ServiceStatus == app.KilledServiceStatus {
					if err == nil {
						log.Printf("checkServiceExist kill service: %s pid: %v", sm.ServiceId, sm.Pid)
					}
					return nil
				}
				// 其他状态的进程都应该存在
				if err != nil {
					session := s.xengine.NewSession()
					defer session.Close()
					_, err = deleteServiceByServiceId(session, sm.ServiceId)
					log.Printf("checkServiceExist delete service: %s pid: %v", sm.ServiceId, sm.Pid)
					return err
				}
				return nil
			})
		if err != nil {
			log.Printf("checkServiceExist failed with err: %v", err)
		}
	}
}

func (s *Server) startHttpServer() {
	s.sockFile = filepath.Join(s.baseDir, "zallet.sock")
	err := os.Remove(s.sockFile)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("try to remove %s failed with err:%v", s.sockFile, err)
	}
	listener, err := net.Listen("unix", s.sockFile)
	if err != nil {
		log.Fatalf("listen unix http server failed with err:%v", err)
	}
	//gin mode
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.UseH2C = true
	engine.ContextWithFallback = true
	registerHandler(engine)
	log.Printf("http server listen on sock file: %s", s.sockFile)
	go func() {
		s.httpServer = &http.Server{
			Handler: engine.Handler(),
		}
		err = s.httpServer.Serve(listener)
		if err != http.ErrServerClosed {
			log.Fatalf("start http server failed with err:%v", err)
		}
	}()
}

func reportStat(c *gin.Context) {
	var req app.ReportStatReq
	if util.ShouldBindJSON(&req, c) {
		server.ReportStat(req)
		c.String(http.StatusOK, "")
	}
}

func applyAppYaml(c *gin.Context) {
	var req app.Yaml
	if util.ShouldBindYAML(&req, c) {
		if req.IsValid() != nil {
			c.String(http.StatusBadRequest, "bad request")
			return
		}
		err := server.ApplyAppYaml(req)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	}
}

func lsService(c *gin.Context) {
	srvs, err := server.LsService(c.Query("app"), cast.ToBool(c.Query("global")), c.Query("status"))
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, srvs)
}

func health(c *gin.Context) {
	c.String(http.StatusOK, "")
}

func reportStatus(c *gin.Context) {
	var req app.ReportStatusReq
	if util.ShouldBindJSON(&req, c) {
		server.ReportStatus(req)
		c.String(http.StatusOK, "")
	}
}

func reportDaemon(c *gin.Context) {
	var req app.ReportDaemonReq
	if util.ShouldBindJSON(&req, c) {
		err := server.ReportDaemon(req)
		var resp app.ReportDaemonResp
		if err != nil {
			resp.Exist = false
			resp.Message = err.Error()
		} else {
			resp.Exist = true
		}
		c.JSON(http.StatusOK, resp)
	}
}

func reportProbe(c *gin.Context) {
	var req app.ReportProbeReq
	if util.ShouldBindJSON(&req, c) {
		server.ReportProbe(req)
		c.String(http.StatusOK, "")
	}
}

func killService(c *gin.Context) {
	err := server.KillService(c.Param("serviceId"))
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, "ok")
}

func deleteService(c *gin.Context) {
	_, err := server.DeleteService(c.Param("serviceId"))
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, "ok")
}

func restartService(c *gin.Context) {
	err := server.RestartService(c.Param("serviceId"))
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, "ok")
}
