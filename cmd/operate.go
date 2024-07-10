package cmd

import (
	"errors"
	"fmt"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/urfave/cli/v2"
	"io"
	"net/http"
)

var (
	Kill    = newOperateCommand("kill")
	Delete  = newOperateCommand("delete")
	Restart = newOperateCommand("restart")
)

func newOperateCommand(op string) *cli.Command {
	return &cli.Command{
		Name:   op,
		Usage:  "This command " + op + " process service",
		Action: operate(op),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name: "sock",
			},
			&cli.StringFlag{
				Name: "service",
			},
		},
	}
}

func operate(op string) func(ctx *cli.Context) error {
	return func(ctx *cli.Context) error {
		return putService(ctx, op)
	}
}

func putService(ctx *cli.Context, operation string) error {
	serviceId := ctx.String("service")
	if serviceId == "" {
		return errors.New("invalid -service")
	}
	sockFile := getSockFile(ctx)
	httpClient := util.NewUnixHttpClient(sockFile)
	defer httpClient.CloseIdleConnections()
	request, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://fake/api/v1/%s/%s", operation, serviceId), nil)
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
	fmt.Println(fmt.Sprintf("%s ok", serviceId))
	return nil
}
