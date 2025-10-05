package expert

import (
	"context"

	"google.golang.org/genai"
)

// Tool is any function that can be exposed to Gemini for its own usage.
type Tool interface {
	// Declare returns the FunctionDeclaration that this tools exposes to an AI.
	Declare() genai.FunctionDeclaration
	// Start is called to start the Tool resources.
	Start(context.Context, *genai.Client, ConversationLogger) error
	// Call is called when the AI actually requested this function to be called.
	Call(ctx context.Context, args map[string]any) genai.FunctionResponse
}
