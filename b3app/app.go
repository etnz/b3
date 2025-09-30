package b3app

import (
	"context"
	"fmt"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// App holds the application's state and dependencies, like the authenticated
// Google Drive service client.
type App struct {
	DriveService *drive.Service
}

// New creates and returns a new, fully initialized App instance.
// It handles the authentication flow to get a valid Google API client.
func New(ctx context.Context) (*App, error) {
	httpClient, err := getClient()
	if err != nil {
		return nil, fmt.Errorf("could not get authenticated client: %w", err)
	}

	driveService, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("could not create drive service: %w", err)
	}

	return &App{DriveService: driveService}, nil
}
