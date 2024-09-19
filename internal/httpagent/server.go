package httpagent

import (
	"github.com/LeeZXin/zallet/internal/global"
	"github.com/LeeZXin/zallet/internal/process"
	"github.com/LeeZXin/zallet/internal/servicemd"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

type Server struct {
	srv *http.Server
}

func (s *Server) Shutdown() {
	s.srv.Shutdown(nil)
}

func StartServer() *Server {
	err := os.Remove(global.SockFile)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("try to remove %s failed with err: %v", global.SockFile, err)
	}
	listener, err := net.Listen("unix", global.SockFile)
	if err != nil {
		log.Fatalf("listen unix http server failed with err:%v", err)
	}
	//gin mode
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.UseH2C = true
	engine.ContextWithFallback = true
	group := engine.Group("/api/v1")
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
		// 上报状态
		group.POST("/reportStatus", reportStatus)
	}
	log.Printf("http server listen on sock file: %s", global.SockFile)
	srv := &http.Server{
		Handler: engine.Handler(),
	}
	go func() {
		for {
			// 删除过期服务
			deleteExpiredService()
			time.Sleep(30 * time.Second)
		}
	}()
	go func() {
		err2 := srv.Serve(listener)
		if err2 != nil && err2 != http.ErrServerClosed {
			log.Fatalf("start http server failed with err:%v", err2)
		}
	}()
	return &Server{
		srv: srv,
	}
}

func deleteExpiredService() {
	session := global.Xengine.NewSession()
	defer session.Close()
	err := servicemd.DeleteServiceByInstanceIdAndExpiredTime(session, global.InstanceId, time.Now().Add(-10*time.Minute))
	if err != nil {
		log.Printf("delete expired service with err: %v", err)
	}
}

func lsService(c *gin.Context) {
	srvs, err := doLsService(
		c.Query("app"),
		cast.ToBool(c.Query("global")),
		c.Query("status"),
	)
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
	var req global.ReportStatusReq
	if util.ShouldBindJSON(&req, c) {
		doReportStatus(req)
		c.String(http.StatusOK, "")
	}
}

func killService(c *gin.Context) {
	err := doKillService(c.Param("serviceId"))
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, "ok")
}

func deleteService(c *gin.Context) {
	_, err := doDeleteService(c.Param("serviceId"))
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, "ok")
}

func restartService(c *gin.Context) {
	err := doRestartService(c.Param("serviceId"))
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.String(http.StatusOK, "ok")
}

func applyAppYaml(c *gin.Context) {
	var req process.Yaml
	if util.ShouldBindYAML(&req, c) {
		if req.IsValid() != nil {
			c.String(http.StatusBadRequest, "bad request")
			return
		}
		err := doApplyAppYaml(req)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	}
}
