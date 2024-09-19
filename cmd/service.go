package cmd

import (
	"encoding/json"
	"github.com/LeeZXin/zallet/internal/process"
	"github.com/urfave/cli/v2"
	"io"
	"os"
	"os/signal"
	"syscall"
)

var Service = &cli.Command{
	Name:     "service",
	Usage:    "This command fork target service, should only called by zallet server",
	Action:   service,
	HideHelp: true,
}

func service(ctx *cli.Context) error {
	input, err := io.ReadAll(ctx.App.Reader)
	if err != nil {
		return err
	}
	var opts process.ServiceOpts
	err = json.Unmarshal(input, &opts)
	if err != nil {
		return err
	}
	err = opts.IsValid()
	if err != nil {
		return err
	}
	supv := process.NewSupervisor(opts)
	err = supv.Run()
	if err != nil {
		return err
	}
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
	case <-supv.ShutdownChan:
	}
	supv.Shutdown()
	return nil
}
