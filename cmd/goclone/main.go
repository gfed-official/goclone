package main

import (
	"fmt"
	"log"
	"os"

	"goclone/internal/api"
	"goclone/internal/config"

	"github.com/pkg/errors"
)

var MainConfig config.Config

func main() {
	MainConfig, err := config.LoadConfig(".")
	if err != nil {
		log.Fatal(errors.Wrap(err, "Failed to get config"))
	}

	fmt.Fprintln(os.Stdout, []any{"Starting Goclone"}...)
	api.StartAPI(MainConfig)
}
