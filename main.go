package main

import (
	"fmt"
	"os"

	"net/http"
	_ "net/http/pprof"

	"github.com/stakater/Reloader/internal/pkg/app"
)

func main() {

	go func() {
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			fmt.Println("Failed to start pprof server:", err)
		}
	}()

	if err := app.Run(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
