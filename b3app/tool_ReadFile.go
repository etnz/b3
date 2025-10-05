package b3app

import (
	"context"
	"fmt"

	"github.com/etnz/b3/expert"
	"google.golang.org/genai"
)

type ReadFileTool struct {
	app    *App
	client *genai.Client
	logger expert.ConversationLogger
}

func NewReadFileTool(app *App) *ReadFileTool {
	return &ReadFileTool{app: app}
}

func (t *ReadFileTool) Start(ctx context.Context, client *genai.Client, logger expert.ConversationLogger) error {
	t.client = client
	t.logger = logger
	return nil
}

func (t *ReadFileTool) Declare() genai.FunctionDeclaration {
	return genai.FunctionDeclaration{
		Name: "ReadFile",
		Description: `Reads and extract the full detailed content of a single, specific file. 
		Use this when you need to perform a deep analysis of a document, 
		especially one that has a missing or incomplete description.`,
		Parameters: &genai.Schema{Type: genai.TypeObject, Properties: map[string]*genai.Schema{"file_id": {Type: genai.TypeString, Description: "The unique ID of the file to read."}}, Required: []string{"file_id"}},
	}
}

func (t *ReadFileTool) Call(ctx context.Context, args map[string]any) (resp genai.FunctionResponse) {
	resp.Response = make(map[string]any)
	defer func() {
		if err, ok := resp.Response["error"]; ok {
			t.logger.LogResponse("ReadFile", fmt.Sprintf("Error: %v", err))
		}
	}()

	fileID, ok := args["file_id"].(string)
	if !ok {
		resp.Response["error"] = fmt.Sprintf("invalid 'file_id' argument: %v", args["file_id"])
		return
	}

	t.logger.LogQuestion("ReadFile", fmt.Sprintf("Read file with ID: %s", fileID))

	content, mimeType, err := t.app.GetFileContent(ctx, fileID)
	if err != nil {
		resp.Response["error"] = err.Error()
		return
	}

	// Send the blob to a dedicated Gemini instance for doc reading
	genContent := []*genai.Content{&genai.Content{
		// TODO(etnz): this is a bug, the role should be user
		Role: genai.RoleUser,
		Parts: []*genai.Part{
			&genai.Part{
				InlineData: &genai.Blob{MIMEType: mimeType, Data: content},
			},
		},
	}}

	gen, err := t.client.Models.GenerateContent(ctx, "gemini-2.5-pro", genContent, &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: []*genai.Part{
			{Text: `Read the file provided to you, and extract a good name and description.
			A good name reflects the administrative nature of the document.
			A good description describes:
			  - the administrative nature of the document.
			  - the adminstrative purpose of such a document.
			  - the content of the file. If the file contains personal data (ID number, name, personal dates, expiration dates) they
			    must be extracted and listed in the description.
`},
		}},
	})

	if err != nil {
		resp.Response["error"] = fmt.Sprintf("analyzing content: %v", err)
		return
	}
	if len(gen.Candidates) == 0 {
		resp.Response["error"] = "received 0 Candidates from analysis"
		return
	}
	parts := gen.Candidates[0].Content.Parts
	if len(parts) == 0 || parts[0].Text == "" {
		resp.Response["error"] = "received empty response from analysis"
		return
	}

	resp.Response["output"] = parts[0].Text
	t.logger.LogResponse("ReadFile", "Successfully extracted and analyzed file content.")
	return
}
