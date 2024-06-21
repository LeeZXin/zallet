package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/LeeZXin/zallet/internal/app"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"os"
)

var Apply = &cli.Command{
	Name:   "apply",
	Usage:  "This command starts process service",
	Action: apply,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "sock",
			Usage: "zallet server sock file path",
		},
		&cli.StringFlag{
			Name:  "file",
			Usage: "zallet service yaml file",
		},
	},
}

func apply(ctx *cli.Context) error {
	filePath := ctx.String("file")
	if filePath == "" {
		return errors.New("invalid -file")
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	var y app.Yaml
	err = yaml.Unmarshal(content, &y)
	if err != nil {
		return err
	}
	err = y.IsValid()
	if err != nil {
		return err
	}
	sockFile := getSockFile(ctx)
	httpClient := util.NewUnixHttpClient(sockFile)
	defer httpClient.CloseIdleConnections()
	req, _ := json.Marshal(y)
	resp, err := httpClient.Post(
		"http://fake/api/apply",
		"application/yaml;charset=utf-8",
		bytes.NewReader(req),
	)
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
