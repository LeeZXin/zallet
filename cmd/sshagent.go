package cmd

import (
	"github.com/LeeZXin/zallet/internal/sshagent"
	"github.com/LeeZXin/zallet/internal/static"
	"github.com/urfave/cli/v2"
	"os"
	"os/signal"
	"syscall"
)

var SshAgent = &cli.Command{
	Name:   "agent",
	Usage:  "This command starts ssh agent",
	Action: sshAgent,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "baseDir",
			Usage: "zallet baseDir flag",
		},
	},
}

func sshAgent(ctx *cli.Context) error {
	baseDir := ctx.String("baseDir")
	if baseDir == "" {
		baseDir = "/usr/local/zallet"
	}
	static.Init(baseDir)
	server := sshagent.NewAgentServer(baseDir)
	server.Start()
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
		server.Shutdown()
	}
	return nil
}
