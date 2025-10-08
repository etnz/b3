package b3app

import (
	"context"
	"fmt"
	"strings"

	"github.com/etnz/b3/expert"
	"google.golang.org/genai"
)

// B4DeleteTool is a tool for deleting files from the B4 folder.
type B4DeleteTool struct {
	app    *App
	logger expert.ConversationLogger
}

// NewB4DeleteTool creates a new B4DeleteTool.
func NewB4DeleteTool(app *App) *B4DeleteTool {
	return &B4DeleteTool{app: app}
}

// Start initializes the tool.
func (t *B4DeleteTool) Start(ctx context.Context, client *genai.Client, logger expert.ConversationLogger) error {
	t.logger = logger
	return nil
}

// Declare defines the function for the AI.
func (t *B4DeleteTool) Declare() genai.FunctionDeclaration {
	return genai.FunctionDeclaration{
		Name: "B4Delete",
		Description: `Permanently deletes one or more files from the B4 folder.
		This action is irreversible. It will only delete files located inside the B4 folder as a safety measure.`,
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"file_ids": {
					Type:        genai.TypeArray,
					Description: "An array of unique FileIDs for the files to be deleted.",
					Items:       &genai.Schema{Type: genai.TypeString},
				},
			},
			Required: []string{"file_ids"},
		},
	}
}

// Call executes the file deletion.
func (t *B4DeleteTool) Call(ctx context.Context, args map[string]any) (resp genai.FunctionResponse) {
	resp.Response = make(map[string]any)
	defer func() {
		if err, ok := resp.Response["error"]; ok {
			t.logger.LogResponse("B4Delete", fmt.Sprintf("Error: %v", err))
		}
	}()

	fileIDsAny, ok := args["file_ids"].([]any)
	if !ok || len(fileIDsAny) == 0 {
		resp.Response["error"] = "missing or invalid 'file_ids' argument"
		return
	}

	var fileIDs []string
	for i, idAny := range fileIDsAny {
		id, ok := idAny.(string)
		if !ok {
			resp.Response["error"] = fmt.Sprintf("invalid file_id at index %d: not a string", i)
			return
		}
		fileIDs = append(fileIDs, id)
	}

	t.logger.LogQuestion("B4Delete", fmt.Sprintf("Attempting to delete %d file(s): %s", len(fileIDs), strings.Join(fileIDs, ", ")))

	var deletedFiles []string
	var warnings []string

	for _, id := range fileIDs {
		if err := t.app.DeleteFile(ctx, id); err != nil {
			warnings = append(warnings, fmt.Sprintf("could not delete file %s: %v", id, err))
			continue
		}
		deletedFiles = append(deletedFiles, id)
	}

	resp.Response["output"] = fmt.Sprintf("Successfully deleted %d file(s).", len(deletedFiles))
	if len(warnings) > 0 {
		resp.Response["output"] = fmt.Sprintf("%s Warnings: %s", resp.Response["output"], strings.Join(warnings, "; "))
	}
	t.logger.LogResponse("B4Delete", resp.Response["output"].(string))
	return
}
