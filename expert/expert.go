package expert

import (
	"context"
	"fmt"
	"io" // still needed for Ask

	"google.golang.org/genai"
)

// Expert represent a special kind of Tool that respond to request by asking an AI.
//
// The Expert holds its own Gemini configuration so that it can be configured for the type of
// expertise it has.
// The Expert can expose Tools to the AI, and it is itself a Tool that expose a single function "Ask+Name()".
type Expert struct {
	// Name is the expert name
	Name string `json:"name"`
	// Description is the expert description as read by the calling AI. So all the skills
	// and capacities must be described to it.
	Description string `json:"description"`
	// Gemini configuration

	// ModelName to use.
	ModelName string `json:"model_name"`
	// Config holds the model config. The Tools section will be replaced by the one generated with Tools and Expert fields.
	Config *genai.GenerateContentConfig `json:"config"`

	// Tools made available to the Gemini model.
	Tools   []Tool
	chat    *genai.Chat
	logger  ConversationLogger
	toolmap map[string]Tool
}

func NewExpert(name, description string, tools ...Tool) *Expert {
	return &Expert{
		Name:        name,
		Description: description,
		Tools:       tools,
	}
}

// Start initializes the expert, its tools, and its connection to the Gemini API.
// It requires a ConversationLogger to handle outputting the flow of questions,
// responses, and tool calls.
func (e *Expert) Start(ctx context.Context, client *genai.Client, logger ConversationLogger) error {
	e.logger = logger

	if len(e.Tools) > 0 {
		e.toolmap = make(map[string]Tool, len(e.Tools))
		// Start and record all tools
		// Create an empty Tool
		e.Config.Tools = []*genai.Tool{{FunctionDeclarations: []*genai.FunctionDeclaration{}}}
		for _, t := range e.Tools {
			if err := t.Start(ctx, client, logger); err != nil {
				return err
			}
			d := t.Declare()
			e.Config.Tools[0].FunctionDeclarations = append(e.Config.Tools[0].FunctionDeclarations, &d)
			e.toolmap[t.Declare().Name] = t
		}
	}
	var err error
	e.chat, err = client.Chats.Create(ctx, e.ModelName, e.Config, nil)
	if err != nil {
		return err
	}
	return nil
}

// Ask is a simple wrapper on top of Chat.Send to make it simpler for callbacks.
func (e *Expert) Ask(ctx context.Context, w io.Writer, parts ...*genai.Part) (*genai.Content, error) {
	resp, err := e.chat.Send(ctx, parts...)
	if err != nil {
		return nil, err
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		return &genai.Content{}, nil //fmt.Errorf("no response from expert %s", e.Name)
	}
	rparts := resp.Candidates[0].Content.Parts
	// TWO cases either there is a function call, then we shall proceed it
	// OR we simply return the content
	hasFunctionCall := false
	for _, part0 := range rparts {
		if part0.FunctionCall != nil {
			hasFunctionCall = true
			break
		}
	}

	if !hasFunctionCall {
		return resp.Candidates[0].Content, nil
	}
	// process parts locally and return the first functioncall
	for _, p := range rparts {
		if p.Text != "" {
			fmt.Fprintln(w, p.Text)
		}
		if p.FunctionCall != nil {
			tool, exists := e.toolmap[p.FunctionCall.Name]
			if !exists {
				return nil, fmt.Errorf("unknown function %q", p.FunctionCall.Name)
			}

			e.logger.LogResponse(e.Name, fmt.Sprintf("Calling %s", p.FunctionCall.Name))

			// Make the callback. No possible error, this error should be sent via 'resp'
			resp := tool.Call(ctx, p.FunctionCall.Args)
			resp.ID = p.FunctionCall.ID
			resp.Name = p.FunctionCall.Name

			e.logger.LogQuestion(e.Name, fmt.Sprintf("Processing %s's response", resp.Name))
			// Ask again the expert with the response he asked for
			// until we have a real response.
			return e.Ask(ctx, w, &genai.Part{FunctionResponse: &resp})
		}
	}
	// unexpected because we tested that there was a functioncall.
	return resp.Candidates[0].Content, nil
}

// Declaration returns the function declaration to ask a question to this expert.
func (e *Expert) Declare() genai.FunctionDeclaration {
	return genai.FunctionDeclaration{
		Name:        e.Name,
		Description: e.Description,
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"question": {
					Type:        genai.TypeString,
					Description: "The question to ask the expert.",
				},
			},
			Required: []string{"question"},
		},
		Response: &genai.Schema{
			Type:        genai.TypeString,
			Description: "Expert's reponse.",
		},
	}
}

// Call perform the call of asking this expert.
func (e *Expert) Call(ctx context.Context, args map[string]any) genai.FunctionResponse {
	resp := genai.FunctionResponse{Response: make(map[string]any)}
	arg0 := args["question"]
	question, ok := arg0.(string)
	if !ok {
		resp.Response["error"] = fmt.Errorf("invalid type got %T, expected string", arg0)
		return resp
	}

	e.logger.LogQuestion(e.Name, question)

	response, err := e.Ask(ctx, io.Discard, &genai.Part{Text: question})
	if err != nil {
		resp.Response["error"] = fmt.Errorf("something went wrong while calling the expert: %w", err)
		return resp
	}

	// Log the multi-line response, line by line
	r := response.Parts[0].Text
	e.logger.LogResponse(e.Name, r)

	resp.Response = map[string]any{
		"output": r,
	}
	return resp
}
