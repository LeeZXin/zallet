package cmd

import (
	"github.com/LeeZXin/zallet/internal/zallet"
	"github.com/urfave/cli/v2"
)

var Run = &cli.Command{
	Name:   "run",
	Usage:  "This command starts zallet server",
	Action: run,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "baseDir",
		},
	},
	Hidden: true,
}

func run(ctx *cli.Context) error {
	baseDir := ctx.String("baseDir")
	zallet.Init(baseDir)
	return nil
}
