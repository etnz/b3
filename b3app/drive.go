package b3app

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/drive/v3"
)

// File represents a file or folder within Google Drive.
type File struct {
	ID          string
	Name        string
	Modified    time.Time
	Description string
}

// findB3FolderID searches for the "B3" folder in the root of the user's Drive.
func (a *App) findB3FolderID(ctx context.Context) (string, error) {
	query := "name = 'B3' and mimeType = 'application/vnd.google-apps.folder' and 'root' in parents and trashed = false"
	fileList, err := a.DriveService.Files.List().Q(query).PageSize(1).Fields("files(id)").Do()
	if err != nil {
		return "", fmt.Errorf("failed to search for 'B3' folder: %w", err)
	}

	if len(fileList.Files) == 0 {
		return "", fmt.Errorf("'B3' folder not found in the root of your Google Drive. Please create it and try again")
	}

	return fileList.Files[0].Id, nil
}

// ListFiles finds the "B3" folder and lists its contents.
func (a *App) ListFiles(ctx context.Context) ([]File, error) {
	b3FolderID, err := a.findB3FolderID(ctx)
	if err != nil {
		return nil, err // Propagate the clear error message from findB3FolderID
	}

	query := fmt.Sprintf("'%s' in parents and trashed = false", b3FolderID)
	var files []File

	err = a.DriveService.Files.List().
		Q(query).
		Fields("nextPageToken, files(id, name, modifiedTime, description)").
		OrderBy("folder, name").
		Pages(ctx, func(page *drive.FileList) error {
			for _, f := range page.Files {
				modifiedTime, err := time.Parse(time.RFC3339, f.ModifiedTime)
				if err != nil {
					// This should ideally not happen with API data
					return fmt.Errorf("could not parse modified time for file %s: %w", f.Name, err)
				}
				files = append(files, File{
					ID:          f.Id,
					Name:        f.Name,
					Modified:    modifiedTime,
					Description: f.Description,
				})
			}
			return nil
		})

	return files, err
}
