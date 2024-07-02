package main

import (
	"github.com/LeeZXin/zallet/cmd"
	"log"
	"os"
)

func main() {
	log.SetOutput(os.Stdout)
	app := cmd.NewCliApp()
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err.Error())
	}
}
