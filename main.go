package main

import (
	"log"

	"magpie-pulse/internal/app"
)

func main() {
	if err := app.New().Run(); err != nil {
		log.Fatal(err)
	}
}
