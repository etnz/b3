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
		The file name should be descriptive of the document nature, the description should 
		describe the file content in detail, contains all the relevant personal information contained
		in the file, as well as the relationship with the primary identity, and any extra relevant information
		that might have been captured in the discussion.
		Optionally, for files in the B4 folder, an 'archive' option will move them to the B3 folder.
		Returns true on success.
		`,
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"file_id":     {Type: genai.TypeString, Description: "The unique FileID of the file to modify."},
				"name":        {Type: genai.TypeString, Description: "The new name for the file."},
				"description": {Type: genai.TypeString, Description: "The new text for the file's description."},
				"archive":     {Type: genai.TypeBoolean, Description: "when true, and the file will be moved to the B4 folder."},
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
	archive, _ := args["archive"].(bool)
	// arch will be true iif the parameter is there and its value is true

	var updates []string
	if name != "" {
		updates = append(updates, fmt.Sprintf("name to '%s'", name))
	}
	if description != "" {
		updates = append(updates, "description")
	}

	t.logger.LogQuestion("UpdateFile", fmt.Sprintf("Update file %s: set %s.", fileID, strings.Join(updates, " and ")))

	err := t.app.UpdateFile(ctx, fileID, name, description, archive)
	if err != nil {
		resp.Response["error"] = err.Error()
		return
	}
	resp.Response["output"] = true
	return
}
