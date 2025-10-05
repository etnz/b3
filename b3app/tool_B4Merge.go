package b3app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/etnz/b3/expert"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"google.golang.org/genai"
)

type B4MergeTool struct {
	app    *App
	logger expert.ConversationLogger
}

func NewB4MergeTool(app *App) *B4MergeTool {
	return &B4MergeTool{app: app}
}

func (t *B4MergeTool) Start(ctx context.Context, client *genai.Client, logger expert.ConversationLogger) error {
	t.logger = logger
	return nil
}

func (t *B4MergeTool) Declare() genai.FunctionDeclaration {
	return genai.FunctionDeclaration{
		Name: "B4Merge",
		Description: `Merges several PDF files into a new single PDF file inside the B4 folder.
		Use this to assemble a final document from multiple sources for a specific administrative procedure.`,
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"file_ids": {
					Type:        genai.TypeArray,
					Description: "An array of unique FileIDs for the PDF files to be merged.",
					Items:       &genai.Schema{Type: genai.TypeString},
				},
				"output_name": {
					Type:        genai.TypeString,
					Description: "The file name for the new merged PDF document.",
				},
				"output_description": {
					Type:        genai.TypeString,
					Description: "A detailed description for the new merged PDF, explaining its purpose and content.",
				},
			},
			Required: []string{"file_ids", "output_name", "output_description"},
		},
	}
}

func (t *B4MergeTool) Call(ctx context.Context, args map[string]any) (resp genai.FunctionResponse) {
	resp.Response = make(map[string]any)
	defer func() {
		if err, ok := resp.Response["error"]; ok {
			t.logger.LogResponse("B4Merge", fmt.Sprintf("Error: %v", err))
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
	outputName, ok := args["output_name"].(string)
	if !ok || outputName == "" {
		resp.Response["error"] = "missing required 'output_name' argument"
		return
	}
	outputDescription, ok := args["output_description"].(string)
	if !ok || outputDescription == "" {
		resp.Response["error"] = "missing required 'output_description' argument"
		return
	}
	// delete_sources is optional, defaults to false

	t.logger.LogQuestion("B4Merge", fmt.Sprintf("Merge %d files (%s) into new file '%s'.", len(fileIDs), strings.Join(fileIDs, ", "), outputName))

	b4FolderID, err := t.app.findB4FolderID(ctx)
	if err != nil {
		resp.Response["error"] = err.Error()
		return
	}

	var tempFiles []string
	for _, id := range fileIDs {
		content, mimeType, err := t.app.GetFileContent(ctx, id)
		if err != nil {
			resp.Response["error"] = fmt.Sprintf("getting content for file %s: %s", id, err)
			return
		}
		if mimeType != "application/pdf" {
			resp.Response["error"] = fmt.Sprintf("file %s is not a PDF (%s), cannot merge", id, mimeType)
			return
		}

		// pdfcpu's Merge API works with file paths, so we must write content to a temporary file.
		tmpFile, err := os.CreateTemp("", "b3-merge-*.pdf")
		if err != nil {
			resp.Response["error"] = fmt.Sprintf("creating temporary file for merging: %s", err)
			return
		}
		// Make sure that every file created is always cleaned up.
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.Write(content); err != nil {
			tmpFile.Close() // Attempt to close before returning
			resp.Response["error"] = fmt.Sprintf("writing to temporary file: %s", err)
			return
		}
		tmpFile.Close() // Close the file so pdfcpu can open it.
		tempFiles = append(tempFiles, tmpFile.Name())
	}

	mergedFile, err := os.CreateTemp("", "b3-merged-*.pdf")
	if err != nil {
		resp.Response["error"] = fmt.Sprintf("creating temporary file for merged output: %s", err)
		return
	}
	mergedPDFPath := mergedFile.Name()
	mergedFile.Close() // Close the file so pdfcpu can create and write to it.
	defer os.Remove(mergedPDFPath)

	conf := model.NewDefaultConfiguration()
	if err := api.MergeCreateFile(tempFiles, mergedPDFPath, false, conf); err != nil {
		resp.Response["error"] = fmt.Sprintf("merging PDFs: %s", err)
		return
	}

	newFile, err := t.app.uploadLocalFile(ctx, mergedPDFPath, outputName, outputDescription, "application/pdf", b4FolderID)
	if err != nil {
		resp.Response["error"] = err.Error()
		return
	}

	out := fmt.Sprintf("Successfully merged %d files into new file '%s' (ID: %s) in B4 folder.", len(fileIDs), newFile.Name, newFile.ID)
	resp.Response["output"] = out
	t.logger.LogResponse("B4Merge", out)
	return
}
