package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/etnz/b3/b3app"
)

func main() {
	verboseFlag := flag.Bool("v", false, "Print logs")
	loginFlag := flag.Bool("login", false, "Authorize the B3 CLI to access your Google Drive.")
	listFlag := flag.Bool("list", false, "List files in your B3 Google Drive folder as JSON.")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "B3: The Bureaucratic Barriers Buster\n\n")
		fmt.Fprintf(os.Stderr, "B3 is a chat-first intelligent agent for your documents.\n")
		fmt.Fprintf(os.Stderr, "Run without flags to start a conversational session.\n\n")
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()
	ctx := context.Background()

	if !*verboseFlag {
		log.SetOutput(io.Discard)
	}

	// Handle -login flag
	if *loginFlag {
		err := b3app.Login()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Successfully logged in. B3 is now authorized to access your Google Drive.")
		return
	}

	// Handle -list flag
	if *listFlag {
		app, err := b3app.New(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		files, err := app.B3Files(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := json.NewEncoder(os.Stdout).Encode(files); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding files to JSON: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Default action: Start the conversational agent
	app, err := b3app.New(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing B3: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "B3 is getting ready, scanning B3 and B4 folders...")
	b3Files, err := app.B3Files(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing B3 files: %v\n", err)
		os.Exit(1)
	}
	b4Files, err := app.B4Files(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing B4 files: %v\n", err)
		os.Exit(1)
	}

	args := flag.Args()

	// Create the B3 expert, passing the application context and the content expert.
	b3Expert := b3app.NewB3Expert(app, b3Files, b4Files)
	agent := b3app.NewAgent(b3Expert, os.Stdout, os.Stdin)
	if err := agent.Run(ctx, args...); err != nil {
		fmt.Fprintf(os.Stderr, "\nAn error occurred: %v\n", err)
		os.Exit(1)
	}
}
