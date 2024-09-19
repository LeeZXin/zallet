package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/LeeZXin/zallet/internal/global"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/urfave/cli/v2"
	"io"
	"net/http"
	"strconv"
	"strings"
)

var Ls = &cli.Command{
	Name:   "ls",
	Usage:  "This command ls services",
	Action: ls,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "sock",
		},
		&cli.StringFlag{
			Name: "app",
		},
		&cli.BoolFlag{
			Name: "global",
		},
		&cli.BoolFlag{
			Name: "onlyServiceId",
		},
		&cli.StringFlag{
			Name: "status",
		},
	},
}

func ls(ctx *cli.Context) error {
	sockFile := getSockFile(ctx)
	httpClient := util.NewUnixHttpClient(sockFile)
	defer httpClient.CloseIdleConnections()
	resp, err := httpClient.Get(fmt.Sprintf("http://fake/api/v1/ls?app=%s&global=%v&status=%s", ctx.String("app"), ctx.Bool("global"), ctx.String("status")))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("server return http status code: %v resp: %v", resp.StatusCode, message)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	ret := make([]global.ServiceVO, 0)
	err = json.Unmarshal(body, &ret)
	if err != nil {
		return err
	}
	onlyServiceId := ctx.Bool("onlyServiceId")
	if onlyServiceId {
		for _, vo := range ret {
			fmt.Println(vo.ServiceId)
		}
		return nil
	}
	rows := []string{"serviceId", "app", "env", "serviceStatus", "pid", "agentHost"}
	maxVarLength := make([]int, 6)
	for i := 0; i < 6; i++ {
		maxVarLength[i] = len(rows[i])
	}
	for _, vo := range ret {
		p := len(vo.ServiceId)
		if maxVarLength[0] < p {
			maxVarLength[0] = p
		}
		p = len(vo.App)
		if maxVarLength[1] < p {
			maxVarLength[1] = p
		}
		p = len(vo.Env)
		if maxVarLength[2] < p {
			maxVarLength[2] = p
		}
		p = len(vo.ServiceStatus)
		if maxVarLength[3] < p {
			maxVarLength[3] = p
		}
		p = len(strconv.Itoa(vo.Pid))
		if maxVarLength[4] < p {
			maxVarLength[4] = p
		}
		p = len(vo.AgentHost)
		if maxVarLength[5] < p {
			maxVarLength[5] = p
		}
	}
	padding := func(str string, l int) string {
		return str + strings.Repeat(" ", l-len(str))
	}
	fmt.Println(fmt.Sprintf("%s  %s  %s  %s  %s  %s",
		padding(rows[0], maxVarLength[0]),
		padding(rows[1], maxVarLength[1]),
		padding(rows[2], maxVarLength[2]),
		padding(rows[3], maxVarLength[3]),
		padding(rows[4], maxVarLength[4]),
		padding(rows[5], maxVarLength[5]),
	))
	for _, vo := range ret {
		fmt.Println(fmt.Sprintf("%s  %s  %s  %s  %s  %s",
			padding(vo.ServiceId, maxVarLength[0]),
			padding(vo.App, maxVarLength[1]),
			padding(vo.Env, maxVarLength[2]),
			padding(vo.ServiceStatus, maxVarLength[3]),
			padding(strconv.Itoa(vo.Pid), maxVarLength[4]),
			padding(vo.AgentHost, maxVarLength[5]),
		))
	}
	return nil
}
