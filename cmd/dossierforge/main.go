package main

import (
	"bilge-lib/internal/app"
	"context"
	"log"
)

func main() {
	ctx, _ := context.WithCancel(context.Background())
	err := app.Execute(ctx)
	if err != nil {
		log.Fatal(err)
	}
}
