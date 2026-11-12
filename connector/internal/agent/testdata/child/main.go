// A test-only connector process with a real private loopback health listener.
package main

import (
	"net/http"
	"os"
)

func main() {
	for i, arg := range os.Args {
		if arg == "--metrics" && i+1 < len(os.Args) {
			http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ready")) })
			http.ListenAndServe(os.Args[i+1], nil)
			return
		}
	}
	os.Exit(2)
}
