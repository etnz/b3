package b3app

import (
	"context"
	"fmt"
	"io"
	"time"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// File represents a file or folder within Google Drive.
type File struct {
	ID          string    `json:"id"`                    // The unique identifier for the file.
	Name        string    `json:"name"`                  // The name of the file.
	Modified    time.Time `json:"modified"`              // The last time the file was modified.
	Description string    `json:"description,omitempty"` // The user-provided description of the file.
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

// ListFiles finds the "B3" folder and recursively lists all files within it and its subfolders.
func (a *App) ListFiles(ctx context.Context) ([]File, error) {
	b3FolderID, err := a.findB3FolderID(ctx)
	if err != nil {
		return nil, err // Propagate the clear error message from findB3FolderID
	}

	var files []File
	foldersToScan := []string{b3FolderID}

	for len(foldersToScan) > 0 {
		currentFolderID := foldersToScan[0]
		foldersToScan = foldersToScan[1:] // Dequeue

		query := fmt.Sprintf("'%s' in parents and trashed = false", currentFolderID)

		err := a.DriveService.Files.List().
			Q(query).
			Fields("nextPageToken, files(id, name, mimeType, modifiedTime, description)").
			Pages(ctx, func(page *drive.FileList) error {
				for _, f := range page.Files {
					isFolder := f.MimeType == "application/vnd.google-apps.folder"
					if isFolder {
						foldersToScan = append(foldersToScan, f.Id) // Enqueue subfolder for scanning
						continue
					}

					modifiedTime, err := time.Parse(time.RFC3339, f.ModifiedTime)
					if err != nil {
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
		if err != nil {
			return nil, fmt.Errorf("failed to list files in folder %s: %w", currentFolderID, err)
		}
	}

	return files, nil
}

// GetFileContent downloads and returns the content of a specific file.
func (a *App) GetFileContent(ctx context.Context, fileID string) ([]byte, string, error) {
	// First, get file metadata to retrieve the MIME type.
	file, err := a.DriveService.Files.Get(fileID).Fields("mimeType").Do()
	if err != nil {
		return nil, "", fmt.Errorf("unable to get file metadata for %s: %w", fileID, err)
	}

	resp, err := a.DriveService.Files.Get(fileID).Download()
	if err != nil {
		return nil, "", fmt.Errorf("unable to download file %s: %w", fileID, err)
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("unable to read content of file %s: %w", fileID, err)
	}
	return content, file.MimeType, nil
}

// UpdateFile updates the metadata (name and/or description) of a specific file.
// Pass an empty string for a field if you don't want to update it.
func (a *App) UpdateFile(ctx context.Context, fileID, newName, newDescription string) error {
	fileToUpdate := &drive.File{}
	var fieldsToUpdate []googleapi.Field

	if newName != "" {
		fileToUpdate.Name = newName
		fieldsToUpdate = append(fieldsToUpdate, "name")
	}
	if newDescription != "" {
		fileToUpdate.Description = newDescription
		fieldsToUpdate = append(fieldsToUpdate, "description")
	}

	if len(fieldsToUpdate) == 0 {
		return nil // Nothing to update
	}

	_, err := a.DriveService.Files.Update(fileID, fileToUpdate).Fields(fieldsToUpdate...).Do()
	return err
}
