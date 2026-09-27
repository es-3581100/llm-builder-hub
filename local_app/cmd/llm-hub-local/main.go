package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/es-3581100/llm-builder-hub/local_app/internal/workstation"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "local listen address")
	statePath := flag.String("state", defaultStatePath(), "authoritative local workstation state file")
	repoPath := flag.String("repo", ".", "real local Git repository to inspect read-only")
	flag.Parse()

	store := workstation.NewStore(*statePath)
	if err := store.Ensure(); err != nil {
		log.Fatal(err)
	}

	repository := workstation.NewGitRepository(*repoPath)
	repositorySnapshot, err := repository.Inspect(context.Background())
	if err != nil {
		log.Fatalf("repository preflight failed: %v", err)
	}

	logger := log.New(os.Stderr, "[llm-hub-local] ", log.LstdFlags)
	server := &workstation.Server{Store: store, Repository: repository, Log: logger}
	fmt.Printf("LLM-Hub local workstation\nAUTHORITY: LOCAL\nstate: %s\nrepository: %s\nrepository_id: %s\nhead: %s\nbranch: %s\nurl: http://%s/\n",
		store.Path(),
		repositorySnapshot.Root,
		repositorySnapshot.RepositoryID,
		display(repositorySnapshot.HeadShort, "UNBORN"),
		display(repositorySnapshot.Branch, detachedLabel(repositorySnapshot)),
		*addr,
	)
	if err := http.ListenAndServe(*addr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}

func defaultStatePath() string {
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
		return filepath.Join(stateHome, "llm-hub", "workstation-state.json")
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".local", "state", "llm-hub", "workstation-state.json")
	}
	return filepath.Join(os.TempDir(), "llm-hub-workstation-state.json")
}

func display(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func detachedLabel(snapshot workstation.RepositorySnapshot) string {
	if snapshot.Detached {
		return "DETACHED"
	}
	if snapshot.Unborn {
		return "UNBORN"
	}
	return "UNKNOWN"
}
