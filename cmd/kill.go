package cmd

import (
	"errors"
	"fmt"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/urfave/cli/v2"
	"io"
	"net/http"
)

var Kill = &cli.Command{
	Name:   "kill",
	Usage:  "This command kill process service",
	Action: kill,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "sock",
			Usage: "zallet server sock file path",
		},
		&cli.StringFlag{
			Name:  "service",
			Usage: "service id",
		},
	},
}

func kill(ctx *cli.Context) error {
	serviceId := ctx.String("service")
	if serviceId == "" {
		return errors.New("invalid -service")
	}
	sockFile := getSockFile(ctx)
	httpClient := util.NewUnixHttpClient(sockFile)
	defer httpClient.CloseIdleConnections()
	request, err := http.NewRequest(http.MethodDelete, "http://fake/api/kill/"+serviceId, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("zallet return http request statusCode: %v resp: %v", resp.StatusCode, string(message))
	}
	return nil
}
