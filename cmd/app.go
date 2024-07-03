package cmd

import (
	"github.com/urfave/cli/v2"
	"runtime"
)

var (
	cmdList = []*cli.Command{
		Run,
		Apply,
		Service,
		Health,
		Kill,
		Delete,
		Restart,
		SshAgent,
		Ls,
	}
)

func NewCliApp() *cli.App {
	app := cli.NewApp()
	app.EnableBashCompletion = true
	app.HideHelp = true
	app.Commands = cmdList
	app.Name = "zallet"
	app.Usage = "A zallet server used for deploy service"
	app.Version = formatBuiltWith()
	return app
}

func formatBuiltWith() string {
	return " built with " + runtime.Version()
}

func getSockFile(ctx *cli.Context) string {
	sockFile := ctx.String("sock")
	if sockFile == "" {
		sockFile = "/usr/local/zallet/zallet.sock"
	}
	return sockFile
}
