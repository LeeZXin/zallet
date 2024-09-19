package zallet

import (
	"github.com/LeeZXin/zallet/internal/global"
	"github.com/LeeZXin/zallet/internal/httpagent"
	"github.com/LeeZXin/zallet/internal/sshagent"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func Run() {
	global.Init()
	httpServer := httpagent.StartServer()
	sshServer := sshagent.StartServer()
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("closing")
	sshServer.Shutdown()
	httpServer.Shutdown()
}
