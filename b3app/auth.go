package b3app

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pkg/browser"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

// TODO: Replace with your actual client ID and secret from Google Cloud Console.
// These credentials should be for a "Desktop app" OAuth 2.0 client ID.
var googleOauthConfig = &oauth2.Config{
	ClientID:     "999716078375-50cl3182oudsaom3sfhogg0k57m714c5.apps.googleusercontent.com",
	ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
	RedirectURL:  "http://localhost:8080",
	Scopes:       []string{drive.DriveReadonlyScope},
	Endpoint:     google.Endpoint,
}

// Login initiates the OAuth 2.0 flow to get and store a user token.
func Login() error {
	// Create a random state string for CSRF protection.
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return fmt.Errorf("failed to generate random state: %w", err)
	}
	state := fmt.Sprintf("%x", stateBytes)

	// Use a channel to receive the authorization code from the HTTP handler.
	codeChan := make(chan string)
	errChan := make(chan error)

	// Start a local server to handle the OAuth callback.
	server := &http.Server{Addr: googleOauthConfig.RedirectURL[len("http://"):], Handler: http.NewServeMux()}
	server.Handler.(*http.ServeMux).HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check for errors from Google.
		if errMsg := r.FormValue("error"); errMsg != "" {
			errChan <- fmt.Errorf("authentication failed: %s", errMsg)
			fmt.Fprintf(w, "Authentication failed. You can close this window.")
			return
		}

		// Verify the state parameter.
		if r.FormValue("state") != state {
			errChan <- fmt.Errorf("invalid state parameter received")
			http.Error(w, "Invalid state parameter.", http.StatusBadRequest)
			return
		}

		// Send the authorization code to the main function.
		codeChan <- r.FormValue("code")
		fmt.Fprintf(w, "✅ Authentication successful! You can now close this browser window and return to the terminal.")
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- fmt.Errorf("callback server error: %w", err)
		}
	}()

	// Get the authorization URL and open it in the user's browser.
	authURL := googleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	fmt.Println("Your browser should open for you to grant B3 access to your Google Drive...")
	if err := browser.OpenURL(authURL); err != nil {
		fmt.Printf("\nIf your browser didn't open, please open this URL manually:\n\n%s\n\n", authURL)
	}

	// Wait for the authorization code or an error.
	var authCode string
	select {
	case code := <-codeChan:
		authCode = code
	case err := <-errChan:
		return err
	}
	// we can now safely shutdown the server
	err := server.Shutdown(context.Background())
	if err != nil {
		//ignore but log it
		fmt.Printf("warning: failed to shutdown callback server: %s", err)
	}

	// Exchange the code for a token.
	tok, err := googleOauthConfig.Exchange(context.Background(), authCode)
	if err != nil {
		return fmt.Errorf("failed to exchange authorization code for token: %w", err)
	}

	return saveToken(tok)
}

// getTokenPath returns the path to the token file.
func getTokenPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	return filepath.Join(configDir, "b3", "token.json"), nil
}

// saveToken saves a token to a file.
func saveToken(token *oauth2.Token) error {
	tokenPath, err := getTokenPath()
	if err != nil {
		return fmt.Errorf("failed to determine token path: %w", err)
	}

	// Ensure the directory exists.
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Open the file with secure permissions (read/write for user only).
	f, err := os.OpenFile(tokenPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %w", err)
	}
	defer f.Close()

	// Encode the token as JSON and write to the file.
	if err := json.NewEncoder(f).Encode(token); err != nil {
		return fmt.Errorf("failed to encode token to file: %w", err)
	}

	return nil
}

// getClient uses a stored token to configure an HTTP client.
func getClient() (*http.Client, error) {
	tokenPath, err := getTokenPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine token path: %w", err)
	}

	f, err := os.Open(tokenPath)
	if err != nil {
		// If the file doesn't exist, the user needs to log in.
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not logged in. Please run 'b3 login' to authorize the application")
		}
		return nil, fmt.Errorf("failed to open token file: %w", err)
	}
	defer f.Close()

	tok := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(tok); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("token file is empty. Please run 'b3 login' again")
		}
		return nil, fmt.Errorf("failed to decode token from file: %w", err)
	}

	return googleOauthConfig.Client(context.Background(), tok), nil
}
