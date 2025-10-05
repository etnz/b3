package b3app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/etnz/b3/expert"
	"github.com/mitchellh/go-wordwrap"
	"google.golang.org/genai"
)

// Agent is the AI assistant that handles the chat session.
type Agent struct {
	w       io.Writer
	r       *bufio.Reader
	expert  *expert.Expert
	started bool
}

// NewAgent creates a new Agent.
func NewAgent(expert *expert.Expert, w io.Writer, r io.Reader) *Agent {
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

	return a.expert.Start(ctx, client, a)
}

// Run starts the interactive REPL session for the agent.
func (a *Agent) Run(ctx context.Context, inputs ...string) error {
	if !a.started {
		if err := a.Start(ctx); err != nil {
			return err
		}
		a.started = true
	}

	fmt.Fprintln(a.w, "Welcome! I am B3, ready to assist you with your documents.")
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

// logMultiline is a helper to log multi-line text with a consistent format.
// It prefixes the first line with the expert's name and a prompt character,
// and indents subsequent lines.
func (a *Agent) logMultiline(name, promptChar, text string) {
	const wrapWidth = 80 // Approximate width to wrap lines

	// The prefix for the first line, e.g., "           Admin: "
	firstLinePrefix := fmt.Sprintf("%20s%s ", name, promptChar)
	// The prefix for subsequent/wrapped lines, e.g., "                 "
	indentPrefix := fmt.Sprintf("%20s  ", "")

	textWidth := wrapWidth - len(firstLinePrefix)
	if textWidth < 20 { // Ensure we have a minimum width
		textWidth = 20
	}

	// Wrap the entire text block according to the calculated width.
	// This intelligently breaks lines at spaces.
	wrappedText := wordwrap.WrapString(text, uint(textWidth))
	wrappedLines := strings.Split(wrappedText, "\n")

	// Print the wrapped lines with the correct prefixes.
	for i, line := range wrappedLines {
		if i == 0 {
			fmt.Fprintf(a.w, "%s%s\n", firstLinePrefix, line)
		} else {
			fmt.Fprintf(a.w, "%s%s\n", indentPrefix, line)
		}
	}
}

// LogQuestion implements the expert.ConversationLogger interface.
func (a *Agent) LogQuestion(expertName, question string) {
	a.logMultiline(expertName, ">", question)
}

// LogResponse implements the expert.ConversationLogger interface.
func (a *Agent) LogResponse(expertName, response string) {
	a.logMultiline(expertName, ":", response)
}
