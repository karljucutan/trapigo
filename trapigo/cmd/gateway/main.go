package main

import (
	"log"

	"github.com/karljucutan/trapigo/trapigo/internal/app/gateway/bootstrap"
)

func main() {
	app, err := bootstrap.CreateApp()
	if err != nil {
		log.Fatal(err)
	}

	app.Run()
}
