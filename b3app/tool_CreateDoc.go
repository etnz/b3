package b3app

import (
	"context"
	"fmt"
	"strings"

	"github.com/etnz/b3/expert"
	"google.golang.org/api/drive/v3"
	"google.golang.org/genai"
)

// CreateDocTool is a tool for creating a Google Doc from Markdown content.
type CreateDocTool struct {
	app    *App
	logger expert.ConversationLogger
}

// NewCreateDocTool creates a new CreateDocTool.
func NewCreateDocTool(app *App) *CreateDocTool {
	return &CreateDocTool{app: app}
}

// Start initializes the tool.
func (t *CreateDocTool) Start(ctx context.Context, client *genai.Client, logger expert.ConversationLogger) error {
	t.logger = logger
	return nil
}

// Declare defines the function for the AI.
func (t *CreateDocTool) Declare() genai.FunctionDeclaration {
	return genai.FunctionDeclaration{
		Name: "CreateDoc",
		Description: `Creates a new Google Doc in the B4 folder from Markdown text.
		This is useful for drafting letters or other documents that require further editing or formatting.`,
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"output_name": {
					Type:        genai.TypeString,
					Description: "The file name for the new Google Doc.",
				},
				"markdown_content": {
					Type:        genai.TypeString,
					Description: "The Markdown content to be converted into the Google Doc.",
				},
			},
			Required: []string{"output_name", "markdown_content"},
		},
	}
}

// Call executes the doc creation.
func (t *CreateDocTool) Call(ctx context.Context, args map[string]any) (resp genai.FunctionResponse) {
	resp.Response = make(map[string]any)
	defer func() {
		if err, ok := resp.Response["error"]; ok {
			t.logger.LogResponse("CreateDoc", fmt.Sprintf("Error: %v", err))
		}
	}()

	outputName, ok := args["output_name"].(string)
	if !ok || outputName == "" {
		resp.Response["error"] = "missing required 'output_name' argument"
		return
	}

	markdownContent, ok := args["markdown_content"].(string)
	if !ok || markdownContent == "" {
		resp.Response["error"] = "missing required 'markdown_content' argument"
		return
	}

	t.logger.LogQuestion("CreateDoc", fmt.Sprintf("Creating new Google Doc named '%s'", outputName))

	b4FolderID, err := t.app.findB4FolderID(ctx)
	if err != nil {
		resp.Response["error"] = err.Error()
		return
	}

	// 1. Upload Markdown as a temporary file
	tempMdName := outputName
	tempMdFile, err := t.app.CreateFile(ctx, tempMdName, "Temporary markdown file for conversion", "text/markdown", b4FolderID, strings.NewReader(markdownContent))
	if err != nil {
		resp.Response["error"] = fmt.Sprintf("failed to upload temporary markdown file: %v", err)
		return
	}

	// 2. Copy and Convert to Google Doc
	newDoc, err := t.app.DriveService.Files.Copy(tempMdFile.ID, &drive.File{
		Name:     outputName,
		MimeType: "application/vnd.google-apps.document",
	}).Fields("id", "name").Do()
	if err != nil {
		// Attempt to clean up temp file even on copy failure
		_ = t.app.DeleteFile(ctx, tempMdFile.ID)
		resp.Response["error"] = fmt.Sprintf("failed to convert markdown to Google Doc: %v", err)
		return
	}

	// 3. Clean Up temporary markdown file
	if err := t.app.DeleteFile(ctx, tempMdFile.ID); err != nil {
		// Log a warning if cleanup fails, but don't fail the whole operation
		t.logger.LogResponse("CreateDoc", fmt.Sprintf("Warning: could not delete temporary file %s: %v", tempMdFile.ID, err))
	}

	out := fmt.Sprintf("Successfully created new Google Doc '%s' (ID: %s) in B4 folder.", newDoc.Name, newDoc.Id)
	resp.Response["output"] = out
	t.logger.LogResponse("CreateDoc", out)
	return
}
