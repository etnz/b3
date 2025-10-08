package b3app

import (
	"context"
	"fmt"
	"os"

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
		It can either create a new file or append the content to an existing file.
		Source files can be optionally deleted after the merge, but only from the B4 folder.`,
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
					Description: "The file name for the new merged PDF document. Required if 'target_file_id' is not provided.",
				},
				"output_description": {
					Type:        genai.TypeString,
					Description: "A detailed description for the new merged PDF. Required if 'target_file_id' is not provided.",
				},
				"target_file_id": {
					Type:        genai.TypeString,
					Description: "Optional. The FileID of an existing PDF in the B4 folder to which the new files will be appended. If provided, 'output_name' and 'output_description' are ignored.",
				},
				"delete_sources": {
					Type:        genai.TypeBoolean,
					Description: "Optional. If set to true, the source files from 'file_ids' will be deleted after a successful merge. This is only allowed for files in the B4 folder.",
				},
			},
			Required: []string{"file_ids"},
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

	targetFileID, _ := args["target_file_id"].(string)
	deleteSources, _ := args["delete_sources"].(bool)

	var outputName, outputDescription string
	if targetFileID == "" {
		outputName, ok = args["output_name"].(string)
		if !ok || outputName == "" {
			resp.Response["error"] = "missing required 'output_name' argument when 'target_file_id' is not provided"
			return
		}
		outputDescription, ok = args["output_description"].(string)
		if !ok || outputDescription == "" {
			resp.Response["error"] = "missing required 'output_description' argument when 'target_file_id' is not provided"
			return
		}
		t.logger.LogQuestion("B4Merge", fmt.Sprintf("Merge %d files into new file '%s'.", len(fileIDs), outputName))
	} else {
		t.logger.LogQuestion("B4Merge", fmt.Sprintf("Appending %d files to target file %s.", len(fileIDs), targetFileID))
	}

	b4FolderID, err := t.app.findB4FolderID(ctx)
	if err != nil {
		resp.Response["error"] = err.Error()
		return
	}

	var tempFiles []string
	// If appending, the target file must be the first in the list for merging.
	if targetFileID != "" {
		content, mimeType, err := t.app.GetFileContent(ctx, targetFileID)
		if err != nil {
			resp.Response["error"] = fmt.Sprintf("getting content for target file %s: %s", targetFileID, err)
			return
		}
		if mimeType != "application/pdf" {
			resp.Response["error"] = fmt.Sprintf("target file %s is not a PDF (%s), cannot merge", targetFileID, mimeType)
			return
		}
		tmpFile, err := os.CreateTemp("", "b3-merge-target-*.pdf")
		if err != nil {
			resp.Response["error"] = fmt.Sprintf("creating temporary file for merging: %s", err)
			return
		}
		defer os.Remove(tmpFile.Name())
		if _, err := tmpFile.Write(content); err != nil {
			tmpFile.Close()
			resp.Response["error"] = fmt.Sprintf("writing to temporary file: %s", err)
			return
		}
		tmpFile.Close()
		tempFiles = append(tempFiles, tmpFile.Name())
	}

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

		tmpFile, err := os.CreateTemp("", "b3-merge-*.pdf")
		if err != nil {
			resp.Response["error"] = fmt.Sprintf("creating temporary file for merging: %s", err)
			return
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.Write(content); err != nil {
			tmpFile.Close()
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
	mergedFile.Close()
	defer os.Remove(mergedPDFPath)

	conf := model.NewDefaultConfiguration()
	if err := api.MergeCreateFile(tempFiles, mergedPDFPath, false, conf); err != nil {
		resp.Response["error"] = fmt.Sprintf("merging PDFs: %s", err)
		return
	}

	var newFile *File
	if targetFileID != "" {
		// Update the existing file
		file, err := os.Open(mergedPDFPath)
		if err != nil {
			resp.Response["error"] = fmt.Sprintf("failed to open merged file for upload: %v", err)
			return
		}
		defer file.Close()
		updatedFile, err := t.app.UpdateFileContent(ctx, targetFileID, "application/pdf", file)
		if err != nil {
			resp.Response["error"] = fmt.Sprintf("failed to update file in Drive: %v", err)
			return
		}
		newFile = updatedFile

	} else {
		// Create a new file
		uploadedFile, err := t.app.uploadLocalFile(ctx, mergedPDFPath, outputName, outputDescription, "application/pdf", b4FolderID)
		if err != nil {
			resp.Response["error"] = err.Error()
			return
		}
		newFile = uploadedFile
	}

	if deleteSources {
		for _, id := range fileIDs {
			if err := t.app.DeleteFile(ctx, id); err != nil {
				t.logger.LogResponse("B4Merge", fmt.Sprintf("Warning: could not delete source file %s: %v", id, err))
			}
		}
	}

	out := fmt.Sprintf("Successfully merged %d files into file '%s' (ID: %s) in B4 folder.", len(fileIDs), newFile.Name, newFile.ID)
	resp.Response["output"] = out
	t.logger.LogResponse("B4Merge", out)
	return
}
