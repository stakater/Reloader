package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/stakater/Reloader/internal/pkg/app"
)

func main() {
	// Start pprof server in a goroutine
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()

	if err := app.Run(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
