package b3app

import (
	"context"
	"fmt"

	"github.com/etnz/b3/expert"
	"google.golang.org/genai"
)

type B3FilesTool struct {
	app    *App
	logger expert.ConversationLogger
}

func NewB3FilesTool(app *App) *B3FilesTool {
	return &B3FilesTool{app: app}
}

func (t *B3FilesTool) Start(ctx context.Context, client *genai.Client, logger expert.ConversationLogger) error {
	t.logger = logger
	return nil
}

func (t *B3FilesTool) Declare() genai.FunctionDeclaration {
	return genai.FunctionDeclaration{
		Name: "B3Files",
		Description: `Fetches the most up-to-date index of all files in the user's B3 folder. 
		You should call this at the beginning of a new conversation 
		or if you suspect the user may have added or changed files. 
		In the result you will get a list of all files and for each:
		  - their unique ID: to communicate with other tools)
		  - a human meaningful name: to communicate with the user
		  - a rather long description that describes the document nature, purpose and content. 
		`,
	}
}

func (t *B3FilesTool) Call(ctx context.Context, args map[string]any) (resp genai.FunctionResponse) {
	t.logger.LogQuestion("B3Files", "Fetch file list from B3 folder.")
	resp.Response = make(map[string]any)
	defer func() {
		if err, ok := resp.Response["error"]; ok {
			t.logger.LogResponse("B3Files", fmt.Sprintf("Error: %v", err))
		}
	}()

	files, err := t.app.B3Files(ctx)
	if err != nil {
		resp.Response["error"] = err.Error()
		return
	}
	resp.Response["output"] = files
	t.logger.LogResponse("B3Files", fmt.Sprintf("Found %d files.", len(files)))
	return
}
