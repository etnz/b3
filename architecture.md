# B3 CLI Architecture

## 1. Overview

This document outlines the software architecture for the B3 command-line interface. The design prioritizes simplicity, a clear separation of concerns, and testability.

The architecture is split into two primary components:

1.  **The Command Layer (`main.go`):** Responsible for parsing user input and managing all command-line interactions.
2.  **The Application Core (`b3app` package):** Contains all the business logic, including authentication and interaction with Google services.

This separation ensures that the core logic is independent of the user interface, making it easier to maintain, test, and potentially reuse in the future.

## 2. Project Structure

The project will follow a streamlined layout, with the `main.go` entry point in the root for a mnemonic `go install` experience.

```
b3/
├── main.go               # Main entry point and subcommand registration
├── b3app/
│   ├── auth.go           # Handles Google OAuth2 flow and token management
│   └── drive.go          # Functions to interact with Google Drive API
└── go.mod
```

## 3. Component Responsibilities

### 3.1. `main.go` (The Command Layer)

The `main.go` file is the user-facing entry point of the application. Its sole responsibilities are to define the CLI structure and delegate tasks to the application core.

* **Role:** Controller / User Interface.
* **Responsibilities:**
    * Initializes and registers all user-facing commands (e.g., `list`, `auth login`, `auth status`) using the `github.com/google/subcommands` library.
    * Parses all command-line flags (e.g., `--recursive`, `--long`).
    * Instantiates the core application by calling the constructor from the `b3app` package.
    * Calls the appropriate methods within the `b3app` package based on the user's input.
    * Handles all output to the console (e.g., printing file lists, status messages, or errors).

### 3.2. `b3app` Package (The Application Core)

The `b3app` package is the heart of the application. It is completely decoupled from the command-line interface and contains all the logic for configuration, authentication, and communication with Google's APIs.

* **Role:** Model / Business Logic.
* **Responsibilities:**
    * **Instantiation:** Provides a constructor function (e.g., `b3app.New()`) that reads configuration, handles the authentication flow to get a valid Google API client, and returns a fully initialized `App` object.
    * **State Management:** Defines an `App` struct that holds the application's state and dependencies, such as the authenticated `http.Client` and the `drive.Service` instance.
    * **Core Logic:** Contains the methods that perform the actual work, such as finding the B3 folder or listing its contents.

#### `b3app/auth.go`
* Manages the entire OAuth 2.0 flow.
* Handles the secure storage and retrieval of the user's refresh token from `~/.config/b3/token.json`.
* Provides the function to create an authenticated `http.Client` for use with Google's API libraries.

#### `b3app/drive.go`
* Contains all functions for interacting with the Google Drive API.
* These functions will operate on the `App` struct or accept the `drive.Service` instance to perform their tasks (e.g., `app.ListFiles(...)`).

## 4. Execution Flow Example

To illustrate the separation of concerns, here is the flow for a user running `b3 list`:

1.  The `main()` function in `main.go` starts.
2.  It registers the `list` subcommand and passes execution to the `subcommands` library.
3.  The `subcommands` library identifies the `list` command and invokes its `Execute` method.
4.  Inside `Execute`, the first step is to instantiate the application core: `app, err := b3app.New()`.
    * This call triggers the logic in `b3app/auth.go` to find a stored token or initiate the full browser-based OAuth 2.0 flow.
    * Upon success, a fully configured `app` object, containing an authenticated `drive.Service`, is returned.
5.  The `Execute` method then calls the core logic method: `files, err := app.ListAllFiles()`.
6.  The `ListAllFiles` method in `b3app/drive.go` performs the necessary API calls to Google Drive to find the B3 folder and list its contents.
7.  The `b3app` method returns the list of files (as a data structure) back to the `Execute` method in `main.go`.
8.  Finally, the `Execute` method formats the data received from `b3app` and prints it to the console for the user to see.
````