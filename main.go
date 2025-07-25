package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/stakater/Reloader/internal/pkg/app"
)

func main() {
	// Start pprof server in a goroutine
	go func() {
		err := http.ListenAndServe("localhost:6060", nil)
		if err != nil {
			fmt.Println("Failed to start pprof server: " + err.Error() + "\n")
		} else {
			fmt.Println("pprof server started on localhost:6060")
		}
	}()

	if err := app.Run(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
