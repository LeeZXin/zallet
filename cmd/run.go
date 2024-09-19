package cmd

import (
	"github.com/LeeZXin/zallet/internal/zallet"
	"github.com/urfave/cli/v2"
)

var Run = &cli.Command{
	Name:   "run",
	Usage:  "This command starts zallet server",
	Action: run,
	Hidden: true,
}

func run(_ *cli.Context) error {
	zallet.Run()
	return nil
}
