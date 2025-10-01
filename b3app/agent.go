package b3app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"google.golang.org/genai"
)

// Agent is the AI assistant that handles the chat session.
type Agent struct {
	w      io.Writer
	r      *bufio.Reader
	expert *Expert
}

// NewAgent creates a new Agent.
func NewAgent(expert *Expert, w io.Writer, r io.Reader) *Agent {
	return &Agent{
		w:      w,
		r:      bufio.NewReader(r),
		expert: expert,
	}
}

// Start initializes the Gemini client and the chat session for the agent's expert.
func (a *Agent) Start(ctx context.Context) error {
	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create genai client: %w", err)
	}

	return a.expert.Start(ctx, client)
}

// Run starts the interactive REPL session for the agent.
func (a *Agent) Run(ctx context.Context, inputs ...string) error {
	if a.expert.chat == nil {
		if err := a.Start(ctx); err != nil {
			return err
		}
	}

	fmt.Fprintln(a.w, "Welcome to B3. I am ready to assist you with your documents.")
	fmt.Fprintln(a.w, "Type 'bye' or press Ctrl+D to exit.")

	// REPL loop
	for {
		fmt.Fprint(a.w, "> ")

		// Read input from args, or from user input
		var input string
		if len(inputs) > 0 {
			input = strings.TrimSpace(inputs[0])
			inputs = inputs[1:]
			if input == "" {
				continue
			}
			fmt.Fprintln(a.w, input)
		} else {
			var err error
			input, err = a.r.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					fmt.Fprintln(a.w) // Newline on exit
					return nil        // Clean exit on Ctrl+D
				}
				return err
			}
		}

		if strings.TrimSpace(input) == "bye" {
			return nil
		}

		content, err := a.expert.Ask(ctx, a.w, &genai.Part{Text: strings.TrimSpace(input)})
		if err != nil {
			return fmt.Errorf("failed to send message to Gemini: %w", err)
		}

		// Print the final text response from the model.
		for _, part := range content.Parts {
			switch {
			case part.Text != "":
				fmt.Fprintln(a.w, part.Text)
			default:
				fmt.Fprintf(a.w, "unhandled part type: %#v\n", part)
			}
		}
	}
}
