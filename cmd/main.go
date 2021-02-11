package main

import (
	"github.com/kok-stack/plugin-center/pkg/server"
)

func main() {
	command, _, _ := server.NewCommand()
	err := command.Execute()
	if err != nil {
		panic(err.Error())
	}
}
