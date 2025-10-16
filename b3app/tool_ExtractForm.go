package b3app

import (
	"bytes"
	"context"
	"fmt"

	"github.com/etnz/b3/expert"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"google.golang.org/genai"
)

// ExtractFormTool is a tool for extracting form data from a PDF into a JSON file.
type ExtractFormTool struct {
	app    *App
	logger expert.ConversationLogger
}

// NewExtractFormTool creates a new ExtractFormTool.
func NewExtractFormTool(app *App) *ExtractFormTool {
	return &ExtractFormTool{app: app}
}

// Start initializes the tool.
func (t *ExtractFormTool) Start(ctx context.Context, client *genai.Client, logger expert.ConversationLogger) error {
	t.logger = logger
	return nil
}

// Declare defines the function for the AI.
func (t *ExtractFormTool) Declare() genai.FunctionDeclaration {
	return genai.FunctionDeclaration{
		Name: "ExtractForm",
		Description: `Extracts form field data from a PDF file and returns it as a JSON string.
		This is useful for analyzing or pre-processing data from PDF forms before deciding on the next steps.`,
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"file_id": {
					Type:        genai.TypeString,
					Description: "The unique FileID of the source PDF file containing the form.",
				},
			},
			Required: []string{"file_id"},
		},
	}
}

// Call executes the form extraction.
func (t *ExtractFormTool) Call(ctx context.Context, args map[string]any) (resp genai.FunctionResponse) {
	resp.Response = make(map[string]any)
	defer func() {
		if err, ok := resp.Response["error"]; ok {
			t.logger.LogResponse("ExtractForm", fmt.Sprintf("Error: %v", err))
		}
	}()

	fileID, _ := args["file_id"].(string)

	if fileID == "" {
		resp.Response["error"] = "missing required argument: 'file_id'"
		return
	}

	t.logger.LogQuestion("ExtractForm", fmt.Sprintf("Extracting form from file %s", fileID))

	pdfContent, _, err := t.app.GetFileContent(ctx, fileID)
	if err != nil {
		resp.Response["error"] = fmt.Sprintf("failed to get content for file %s: %v", fileID, err)
		return
	}

	// Use pdfcpu to extract form data as JSON from the in-memory content.
	var jsonBuffer bytes.Buffer
	err = api.ExportFormJSON(bytes.NewReader(pdfContent), &jsonBuffer, fileID, nil)
	if err != nil {
		resp.Response["error"] = fmt.Sprintf("failed to export form data from PDF %s: %v", fileID, err)
		return
	}

	outputJSON := jsonBuffer.String()
	resp.Response["output"] = outputJSON
	t.logger.LogResponse("ExtractForm", "Successfully extracted form data as JSON.")
	return
}
