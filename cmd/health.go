package cmd

import (
	"fmt"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/urfave/cli/v2"
	"net/http"
)

var Health = &cli.Command{
	Name:   "health",
	Usage:  "This command send health request to daemon server",
	Action: health,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "sock",
			Usage: "zallet server sock file path",
		},
	},
}

func health(ctx *cli.Context) error {
	sockFile := getSockFile(ctx)
	httpClient := util.NewUnixHttpClient(sockFile)
	defer httpClient.CloseIdleConnections()
	resp, err := httpClient.Get("http://fake/api/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server return http status code: %v", resp.StatusCode)
	}
	println("ok")
	return nil
}
