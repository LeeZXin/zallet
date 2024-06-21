package main

import (
	"github.com/LeeZXin/zallet/cmd"
	"log"
	"os"
)

func main() {
	app := cmd.NewCliApp()
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err.Error())
	}
}
