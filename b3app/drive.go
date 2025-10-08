package b3app

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// File represents a file in B3 folder
type File struct {
	ID          string    `json:"id"`                    // The unique identifier for the file.
	Name        string    `json:"name"`                  // The name of the file.
	Modified    time.Time `json:"modified"`              // The last time the file was modified.
	Description string    `json:"description,omitempty"` // The user-provided description of the file.
}

// findB3FolderID searches for the "B3" folder in the root of the user's Drive.
func (a *App) findB3FolderID(ctx context.Context) (string, error) {
	query := "name = 'B3' and mimeType = 'application/vnd.google-apps.folder' and 'root' in parents and trashed = false"
	fileList, err := a.DriveService.Files.List().Context(ctx).Q(query).PageSize(1).Fields("files(id)").Do()
	if err != nil {
		return "", fmt.Errorf("failed to search for 'B3' folder: %w", err)
	}

	if len(fileList.Files) == 0 {
		return "", fmt.Errorf("'B3' folder not found in the root of your Google Drive. Please create it and try again")
	}

	return fileList.Files[0].Id, nil
}

// findB4FolderID searches for the "B3" folder in the root of the user's Drive.
func (a *App) findB4FolderID(ctx context.Context) (string, error) {
	query := "name = 'B4' and mimeType = 'application/vnd.google-apps.folder' and 'root' in parents and trashed = false"
	fileList, err := a.DriveService.Files.List().Context(ctx).Q(query).PageSize(1).Fields("files(id)").Do()
	if err != nil {
		return "", fmt.Errorf("failed to search for 'B4' folder: %w", err)
	}

	if len(fileList.Files) == 0 {
		return "", fmt.Errorf("'B4' folder not found in the root of your Google Drive. Please create it and try again")
	}

	return fileList.Files[0].Id, nil
}

// B3Files finds the "B3" folder and recursively lists all files within it and its subfolders.
func (a *App) B3Files(ctx context.Context) ([]File, error) {
	b3FolderID, err := a.findB3FolderID(ctx)
	if err != nil {
		return nil, err // Propagate the clear error message from findB3FolderID
	}
	return a.ListFiles(ctx, b3FolderID)
}

// B4Files finds the "B4" folder and recursively lists all files within it and its subfolders.
func (a *App) B4Files(ctx context.Context) ([]File, error) {
	b4FolderID, err := a.findB4FolderID(ctx)
	if err != nil {
		return nil, err // Propagate the clear error message from findB3FolderID
	}
	return a.ListFiles(ctx, b4FolderID)
}

func (a *App) ListFiles(ctx context.Context, folderID string) ([]File, error) {
	var files []File
	foldersToScan := []string{folderID}

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

// CreateFile uploads a new file to Google Drive.
func (a *App) CreateFile(ctx context.Context, name, description, mimeType, parentID string, content io.Reader) (*File, error) {
	driveFile := &drive.File{
		Name:        name,
		Description: description,
		MimeType:    mimeType,
		Parents:     []string{parentID},
	}

	createdFile, err := a.DriveService.Files.Create(driveFile).Context(ctx).Media(content).Do()
	if err != nil {
		return nil, fmt.Errorf("could not create file '%s': %w", name, err)
	}

	return &File{ID: createdFile.Id, Name: createdFile.Name}, nil
}

// uploadLocalFile is a helper to upload a file from a local path to Google Drive.
func (a *App) uploadLocalFile(ctx context.Context, localPath, name, description, mimeType, parentID string) (*File, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file for upload: %w", err)
	}
	defer file.Close()

	newFile, err := a.CreateFile(ctx, name, description, mimeType, parentID, file)
	if err != nil {
		return nil, fmt.Errorf("failed to create file in Drive: %w", err)
	}
	return newFile, nil
}

// ExportFile downloads a Google Workspace document (like a Google Doc) by exporting it to a specified MIME type.
func (a *App) ExportFile(ctx context.Context, fileID, mimeType string) ([]byte, error) {
	resp, err := a.DriveService.Files.Export(fileID, mimeType).Download()
	if err != nil {
		return nil, fmt.Errorf("unable to export file %s to %s: %w", fileID, mimeType, err)
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read content of exported file %s: %w", fileID, err)
	}

	return content, nil
}

// isFileInFolder checks if a file is a descendant of a specific folder.
func (a *App) isFileInFolder(ctx context.Context, fileID, folderID string) (bool, error) {
	file, err := a.DriveService.Files.Get(fileID).Fields("parents").Do()
	if err != nil {
		return false, fmt.Errorf("unable to get file metadata for %s: %w", fileID, err)
	}

	for _, parentID := range file.Parents {
		if parentID == folderID {
			return true, nil
		}
		// Recursively check parent folders
		isInFolder, err := a.isFileInFolder(ctx, parentID, folderID)
		if err == nil && isInFolder {
			return true, nil
		}
	}

	return false, nil
}

// DeleteFile permanently deletes a file from Google Drive, but only if it's in the B4 folder.
func (a *App) DeleteFile(ctx context.Context, fileID string) error {
	b4FolderID, err := a.findB4FolderID(ctx)
	if err != nil {
		return err
	}

	isSafeToDelete, err := a.isFileInFolder(ctx, fileID, b4FolderID)
	if err != nil {
		return fmt.Errorf("could not verify file location for deletion: %w", err)
	}

	if !isSafeToDelete {
		return fmt.Errorf("safety check failed: file %s is not in the B4 folder and will not be deleted", fileID)
	}

	err = a.DriveService.Files.Delete(fileID).Do()
	if err != nil {
		return fmt.Errorf("failed to delete file with ID %s: %w", fileID, err)
	}
	return nil
}

// UpdateFileContent updates the content of a specific file.
func (a *App) UpdateFileContent(ctx context.Context, fileID, mimeType string, content io.Reader) (*File, error) {
	updatedFile, err := a.DriveService.Files.Update(fileID, &drive.File{MimeType: mimeType}).Context(ctx).Media(content).Fields("id", "name").Do()
	if err != nil {
		return nil, fmt.Errorf("could not update file '%s': %w", fileID, err)
	}

	return &File{ID: updatedFile.Id, Name: updatedFile.Name}, nil
}
