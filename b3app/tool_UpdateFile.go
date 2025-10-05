package b3app

import (
	"context"
	"fmt"
	"strings"

	"github.com/etnz/b3/expert"
	"google.golang.org/genai"
)

type UpdateFileTool struct {
	app    *App
	logger expert.ConversationLogger
}

func NewUpdateFileTool(app *App) *UpdateFileTool {
	return &UpdateFileTool{app: app}
}

func (t *UpdateFileTool) Start(ctx context.Context, client *genai.Client, logger expert.ConversationLogger) error {
	t.logger = logger
	return nil
}

func (t *UpdateFileTool) Declare() genai.FunctionDeclaration {
	return genai.FunctionDeclaration{
		Name: "UpdateFile",
		Description: `Updates the metadata (name and/or description) for a specific file. 
		After you have analyzed a file's content, use this tool to save your findings. 
		This permanently improves the B3 knowledge base for all future conversations.
		Returns true on success.
		`,
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"file_id":     {Type: genai.TypeString, Description: "The unique FileID of the file to modify."},
				"name":        {Type: genai.TypeString, Description: "The new name for the file."},
				"description": {Type: genai.TypeString, Description: "The new text for the file's description."},
			},
			Required: []string{"file_id"},
		},
	}
}

func (t *UpdateFileTool) Call(ctx context.Context, args map[string]any) (resp genai.FunctionResponse) {
	resp.Response = make(map[string]any)
	defer func() {
		if err, ok := resp.Response["error"]; ok {
			t.logger.LogResponse("UpdateFile", fmt.Sprintf("Error: %v", err))
		} else {
			t.logger.LogResponse("UpdateFile", "Successfully updated file metadata.")
		}
	}()

	fileID, ok1 := args["file_id"].(string)
	// Name and description are optional.
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)

	if !ok1 {
		resp.Response["error"] = fmt.Sprintf("missing required 'file_id' argument: %v", args)
		return
	}

	if name == "" && description == "" {
		resp.Response["error"] = "update tool called without 'name' or 'description' to update."
		return
	}

	var updates []string
	if name != "" {
		updates = append(updates, fmt.Sprintf("name to '%s'", name))
	}
	if description != "" {
		updates = append(updates, "description")
	}
	t.logger.LogQuestion("UpdateFile", fmt.Sprintf("Update file %s: set %s.", fileID, strings.Join(updates, " and ")))

	err := t.app.UpdateFile(ctx, fileID, name, description)
	if err != nil {
		resp.Response["error"] = err.Error()
		return
	}
	resp.Response["output"] = true
	return
}
