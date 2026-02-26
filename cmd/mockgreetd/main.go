package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/nickheyer/greetdeez/internal/mockgreetd"
)

func main() {
	sock := flag.String("sock", "/tmp/greetd.sock", "Path to the Unix socket")
	usersFlag := flag.String("users", "test:test,demo:demo", "Comma-separated user:password pairs")
	flag.Parse()

	users := make(map[string]string)
	for _, pair := range strings.Split(*usersFlag, ",") {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			users[parts[0]] = parts[1]
		}
	}

	srv := mockgreetd.New(*sock, users)
	if err := srv.Start(); err != nil {
		log.Fatalf("failed to start: %v", err)
	}

	fmt.Fprintf(os.Stderr, "mockgreetd listening on %s (users: %s)\n", *sock, *usersFlag)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	fmt.Fprintln(os.Stderr, "shutting down...")
	srv.Stop()
}
