package b3app

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/etnz/b3/expert"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"google.golang.org/genai"
)

// FillFormTool is a tool for filling a PDF form in-place.
type FillFormTool struct {
	app    *App
	logger expert.ConversationLogger
}

// NewFillFormTool creates a new FillFormTool.
func NewFillFormTool(app *App) *FillFormTool {
	return &FillFormTool{app: app}
}

// Start initializes the tool.
func (t *FillFormTool) Start(ctx context.Context, client *genai.Client, logger expert.ConversationLogger) error {
	t.logger = logger
	return nil
}

// Declare defines the function for the AI.
func (t *FillFormTool) Declare() genai.FunctionDeclaration {
	return genai.FunctionDeclaration{
		Name: "FillForm",
		Description: `Fills a PDF form in-place using a JSON string of form data.
		The original PDF file will be updated with the filled data.`,
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"file_id": {
					Type:        genai.TypeString,
					Description: "The unique FileID of the source PDF file to be filled.",
				},
				"form_data": {
					Type:        genai.TypeString,
					Description: "A JSON string containing form extracted from the pdf and filled for the user.",
				},
			},
			Required: []string{"file_id", "form_data"},
		},
	}
}

// Call executes the form filling.
func (t *FillFormTool) Call(ctx context.Context, args map[string]any) (resp genai.FunctionResponse) {
	resp.Response = make(map[string]any)
	defer func() {
		if err, ok := resp.Response["error"]; ok {
			t.logger.LogResponse("FillForm", fmt.Sprintf("Error: %v", err))
		}
	}()

	fileID, _ := args["file_id"].(string)
	formData, _ := args["form_data"].(string)

	if fileID == "" || formData == "" {
		resp.Response["error"] = "missing one or more required arguments: 'file_id', 'form_data'"
		return
	}

	t.logger.LogQuestion("FillForm", fmt.Sprintf("Filling form for file %s", fileID))

	pdfContent, mimeType, err := t.app.GetFileContent(ctx, fileID)
	if err != nil {
		resp.Response["error"] = fmt.Sprintf("failed to get content for file %s: %v", fileID, err)
		return
	}

	var filledPDF bytes.Buffer
	err = api.FillForm(bytes.NewReader(pdfContent), strings.NewReader(formData), &filledPDF, model.NewDefaultConfiguration())
	if err != nil {
		resp.Response["error"] = fmt.Sprintf("failed to fill form for PDF %s: %v", fileID, err)
		return
	}

	if _, err := t.app.UpdateFileContent(ctx, fileID, mimeType, &filledPDF); err != nil {
		resp.Response["error"] = fmt.Sprintf("failed to update file content for %s: %v", fileID, err)
		return
	}

	resp.Response["output"] = fmt.Sprintf("Successfully filled and updated form for file %s.", fileID)
	t.logger.LogResponse("FillForm", resp.Response["output"].(string))
	return
}
