package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/es-3581100/llm-builder-hub/local_app/internal/workstation"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "local listen address")
	statePath := flag.String("state", ".llm-hub/workstation-state.json", "authoritative local state file")
	flag.Parse()

	store := workstation.NewStore(*statePath)
	if err := store.Ensure(); err != nil {
		log.Fatal(err)
	}

	logger := log.New(os.Stderr, "[llm-hub-local] ", log.LstdFlags)
	server := &workstation.Server{Store: store, Log: logger}
	fmt.Printf("LLM-Hub local workstation\nAUTHORITY: LOCAL\nstate: %s\nurl: http://%s/\n", store.Path(), *addr)
	if err := http.ListenAndServe(*addr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}
