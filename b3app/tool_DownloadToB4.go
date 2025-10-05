package b3app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/etnz/b3/expert"
	"google.golang.org/genai"
)

type DownloadToB4Tool struct {
	app    *App
	logger expert.ConversationLogger
}

func NewDownloadToB4Tool(app *App) *DownloadToB4Tool {
	return &DownloadToB4Tool{app: app}
}

func (t *DownloadToB4Tool) Start(ctx context.Context, client *genai.Client, logger expert.ConversationLogger) error {
	t.logger = logger
	return nil
}

func (t *DownloadToB4Tool) Declare() genai.FunctionDeclaration {
	return genai.FunctionDeclaration{
		Name: "DownloadToB4",
		Description: `Downloads a PDF file from a given URI and saves it into the B4 folder.
		Use this to fetch external documents like official forms needed for an administrative procedure.`,
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"uri": {
					Type:        genai.TypeString,
					Description: "The public URI of the PDF file to download.",
				},
				"name": {
					Type:        genai.TypeString,
					Description: "The file name for the new document in the B4 folder.",
				},
				"description": {
					Type:        genai.TypeString,
					Description: "A detailed description for the new file, explaining its purpose.",
				},
			},
			Required: []string{"uri", "name", "description"},
		},
	}
}

func (t *DownloadToB4Tool) Call(ctx context.Context, args map[string]any) (resp genai.FunctionResponse) {
	resp.Response = make(map[string]any)
	defer func() {
		if err, ok := resp.Response["error"]; ok {
			t.logger.LogResponse("DownloadToB4", fmt.Sprintf("Error: %v", err))
		}
	}()

	uri, ok := args["uri"].(string)
	if !ok || uri == "" {
		resp.Response["error"] = "missing or invalid 'uri' argument"
		return
	}
	name, ok := args["name"].(string)
	if !ok || name == "" {
		resp.Response["error"] = "missing required 'name' argument"
		return
	}
	description, ok := args["description"].(string)
	if !ok || description == "" {
		resp.Response["error"] = "missing required 'description' argument"
		return
	}

	t.logger.LogQuestion("DownloadToB4", fmt.Sprintf("Download from %s to create file '%s'.", uri, name))

	httpResp, err := http.Get(uri)
	if err != nil {
		resp.Response["error"] = fmt.Sprintf("failed to download file from %s: %v", uri, err)
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Response["error"] = fmt.Sprintf("failed to download file: received status code %d", httpResp.StatusCode)
		return
	}

	// Never trust the source. Get the MIME type from the response header.
	mimeType := httpResp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	b4FolderID, err := t.app.findB4FolderID(ctx)
	if err != nil {
		resp.Response["error"] = err.Error()
		return
	}

	newFile, err := t.app.CreateFile(ctx, name, description, mimeType, b4FolderID, httpResp.Body)
	if err != nil {
		resp.Response["error"] = fmt.Sprintf("failed to create file in Drive: %v", err)
		return
	}

	out := fmt.Sprintf("Successfully downloaded and saved file '%s' (ID: %s) to B4 folder.", newFile.Name, newFile.ID)
	resp.Response["output"] = out
	t.logger.LogResponse("DownloadToB4", out)
	return
}
