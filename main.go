package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/etnz/b3/b3app"
	"github.com/google/subcommands"
)

type loginCmd struct{}

func (*loginCmd) Name() string     { return "login" }
func (*loginCmd) Synopsis() string { return "Authorize the B3 CLI to access your Google Drive." }
func (*loginCmd) Usage() string {
	return `login:
  Initiate the Google OAuth 2.0 flow to grant B3 read-only access to your Google Drive.
`
}

func (c *loginCmd) SetFlags(f *flag.FlagSet) {}

func (c *loginCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	// The core logic is delegated to the b3app package.
	err := b3app.Login()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
		return subcommands.ExitFailure
	}

	fmt.Println("✅ Successfully logged in. B3 is now authorized to access your Google Drive.")
	return subcommands.ExitSuccess
}

type listCmd struct {
	long bool
	// recursive bool // To be implemented later
}

func (*listCmd) Name() string     { return "list" }
func (*listCmd) Synopsis() string { return "List files in your B3 Google Drive folder." }
func (*listCmd) Usage() string {
	return `list [-l]:
  List files and directories within the user's Google Drive "B3" folder.
  -l: Use a long listing format.
`
}

func (c *listCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&c.long, "l", false, "Use a long listing format.")
}

func (c *listCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	app, err := b3app.New(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return subcommands.ExitFailure
	}

	files, err := app.ListFiles(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return subcommands.ExitFailure
	}

	for _, file := range files {
		if c.long {
			fmt.Printf("%-25s %-12s %-35s %s\n", file.ID, file.Modified.Format("2006-01-02"), file.Name, file.Description)
		} else {
			fmt.Println(file.Name)
		}
	}
	return subcommands.ExitSuccess
}

func main() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(&loginCmd{}, "")
	subcommands.Register(&listCmd{}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}
