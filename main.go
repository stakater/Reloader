package main

import (
	"os"

	"net/http"
	_ "net/http/pprof"

	"github.com/stakater/Reloader/internal/pkg/app"
)

func main() {

	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()

	if err := app.Run(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
