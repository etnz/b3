package b3app

import (
	"context"
	"fmt"

	"github.com/etnz/b3/expert"
	"google.golang.org/genai"
)

type B4FilesTool struct {
	app    *App
	logger expert.ConversationLogger
}

func NewB4FilesTool(app *App) *B4FilesTool {
	return &B4FilesTool{app: app}
}

func (t *B4FilesTool) Start(ctx context.Context, client *genai.Client, logger expert.ConversationLogger) error {
	t.logger = logger
	return nil
}

func (t *B4FilesTool) Declare() genai.FunctionDeclaration {
	return genai.FunctionDeclaration{
		Name: "B4Files",
		Description: `Fetches the most up-to-date index of all files in the user's B4 folder.
		You should call this at the beginning of a new conversation 
		to figure out what procedures the user is currently working on, and what is their status. 
		
		In the result you will get a list of all files and for each:
		  - their unique ID: to communicate with other tools
		  - the file name: to communicate with the user
		  - a rather long description that describes the document nature, purpose and content, and status
		`,
	}
}

func (t *B4FilesTool) Call(ctx context.Context, args map[string]any) (resp genai.FunctionResponse) {
	t.logger.LogQuestion("B4Files", "Fetch file list from B4 folder.")
	resp.Response = make(map[string]any)
	defer func() {
		if err, ok := resp.Response["error"]; ok {
			t.logger.LogResponse("B4Files", fmt.Sprintf("Error: %v", err))
		}
	}()

	files, err := t.app.B4Files(ctx)
	if err != nil {
		resp.Response["error"] = err.Error()
		return
	}
	resp.Response["output"] = files
	t.logger.LogResponse("B4Files", fmt.Sprintf("Found %d files.", len(files)))
	return
}
