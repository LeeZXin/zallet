package cmd

import (
	"encoding/json"
	"github.com/LeeZXin/zallet/internal/app"
	"github.com/urfave/cli/v2"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var Service = &cli.Command{
	Name:     "service",
	Usage:    "This command fork service, should only called by zallet server",
	Action:   service,
	HideHelp: true,
}

func service(ctx *cli.Context) error {
	input, err := io.ReadAll(ctx.App.Reader)
	if err != nil {
		return err
	}
	var opts app.ServiceOpts
	err = json.Unmarshal(input, &opts)
	if err != nil {
		return err
	}
	srv, err := app.RunService(opts)
	if err != nil {
		return err
	}
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-quit:
		log.Printf("receive signal: %v", sig)
		srv.Shutdown(nil, false)
		// 给时间收尾
		time.Sleep(5 * time.Second)
	case <-srv.ShutdownChan:
		log.Println("receive shutdown chan")
	}
	return nil
}
