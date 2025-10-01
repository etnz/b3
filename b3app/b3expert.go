package b3app

import (
	"context"
	"fmt"
	"io"
	"log"

	"google.golang.org/genai"
)

const b3SystemPrompt = `You are B3, a personal data assistant that lives in the user's terminal. 
Your mission is to help the user by deeply understanding the documents they have stored in their 'B3' Google Drive folder. 
You have three tools to help you: refresh() to see the current list of files, content() to read a specific file, and update() to curate the list of documents.
Your primary workflow is to Refresh, Read, and then Update to continuously enrich your knowledge base about the primary user personal data.

When a file has no description and appears to have been scanned recently, start reading it and create a meaningful description:
 - accurately describes the nature of the document (passport, french identity card)
 - contains directly readable personal data: e.g. full name, identification numbers (like tax ID, security Number, etc.), addresses, important dates.
 - relation of those data with the primary user. Could be his fullname or his daughter's full name, or his mother's fullname, etc.
Look for apparent conflicts (like multiple identities, or adresses) try to figure out the possible resolution, but ask the user to confirm, and record the solution
in the B3 metadata (file name and/or description).
`

// NewB3Expert creates and configures an Expert specifically for the B3 application.
// This expert knows how to interact with the Google Drive files via the App dependency.
func NewB3Expert(app *App, contentExpert, adminExpert *Expert) *Expert { // Added contentExpert parameter
	// Define the tool-handling logic specific to the B3 expert.
	handleToolCall := func(ctx context.Context, fc *genai.FunctionCall) *genai.FunctionResponse {
		log.Printf("🤖 Call %s(%v)\n", fc.Name, fc.Args)
		resp := &genai.FunctionResponse{
			Name: fc.Name,
			ID:   fc.ID,
		}

		switch fc.Name {
		case "refresh":
			files, err := app.ListFiles(ctx)
			// if err == nil {
			// var jsonData []byte
			// jsonData, err = json.Marshal(files)
			if err != nil {
				resp.Response = map[string]any{"error": err.Error()}
			} else {
				resp.Response = map[string]any{"output": files}
			}
			// }
		case "content":
			if fileID, ok := fc.Args["file_id"].(string); ok {
				var (
					content  []byte
					mimeType string
				)
				content, mimeType, err := app.GetFileContent(ctx, fileID)
				if err != nil {
					resp.Response = map[string]any{"error": err.Error()}
					break
				}
				xr, err := contentExpert.Ask(ctx, io.Discard, // Use the contentExpert to process the file content
					&genai.Part{Text: "Provide a detailed description of that file content. Start with the document nature and abstract description"},
					&genai.Part{InlineData: &genai.Blob{MIMEType: mimeType, Data: content}},
				)

				if err != nil {
					resp.Response = map[string]any{"error": err.Error()}
				} else if len(xr.Parts) > 0 && xr.Parts[0].Text != "" { // Extract the text response from the content expert
					resp.Response = map[string]any{"output": xr.Parts[0].Text}
				} else {
					resp.Response = map[string]any{"error": "content expert returned no text output"}
				}
			} else {
				resp.Response = map[string]any{"error": fmt.Sprintf("invalid 'file_id' argument: %v", fc.Args["file_id"])}
			}

		case "update":
			fileID, ok1 := fc.Args["file_id"].(string)
			// Name and description are optional.
			name, _ := fc.Args["name"].(string)
			description, _ := fc.Args["description"].(string)

			if !ok1 {
				resp.Response = map[string]any{"error": fmt.Sprintf("missing required 'file_id' argument: %v", fc.Args)}
				break
			}

			if name == "" && description == "" {
				resp.Response = map[string]any{"error": "update tool called without 'name' or 'description' to update."}
			} else {
				err := app.UpdateFile(ctx, fileID, name, description)
				if err != nil {
					resp.Response = map[string]any{"error": err.Error()}
				} else {
					resp.Response = map[string]any{"output": true}
				}
			}
		case adminExpert.Name:
			return adminExpert.Call(ctx, fc.ID, fc.Args)

		default:
			resp.Response = map[string]any{"error": fmt.Sprintf("unknown tool call: %s", fc.Name)}
		}

		if e := resp.Response["error"]; e != nil {
			// Log the error
			log.Printf("🤖 Err: %v\n", e)

		}
		return resp
	}

	// Define the functions (tools) the B3 expert can use.
	tools := []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "refresh",
					Description: "Fetches the most up-to-date index of all files in the user's B3 folder. You should call this at the beginning of a new conversation or if you suspect the user may have added or changed files, as it provides you with your primary context.",
				},
				{
					Name:        "content",
					Description: "Reads and extract the full detailed content of a single, specific file. Use this when you need to perform a deep analysis of a document, especially one that has a missing or incomplete description.",
					Parameters:  &genai.Schema{Type: genai.TypeObject, Properties: map[string]*genai.Schema{"file_id": {Type: genai.TypeString, Description: "The unique ID of the file to read."}}, Required: []string{"file_id"}},
				},
				{
					Name:        "update",
					Description: "Updates the metadata (name and/or description) for a specific file. After you have analyzed a file's content, use this tool to save your findings. This permanently enriches your knowledge base for all future conversations.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"file_id":     {Type: genai.TypeString, Description: "The ID of the file to modify."},
							"name":        {Type: genai.TypeString, Description: "The new name for the file."},
							"description": {Type: genai.TypeString, Description: "The new text for the file's description."},
						},
						Required: []string{"file_id"},
					},
				},
				adminExpert.Declaration(),
				// contentExpert.Declaration(),
			},
		},
	}

	expert := NewExpert("B3", "A personal data assistant for Google Drive.")
	expert.ModelName = "gemini-2.5-pro"
	expert.Experts = []*Expert{contentExpert, adminExpert}

	expert.Library = handleToolCall
	expert.Config = &genai.GenerateContentConfig{

		Tools: tools,
		SystemInstruction: &genai.Content{Parts: []*genai.Part{
			{Text: b3SystemPrompt},
		}},
	}
	return expert
}

// NewContentExpert creates and configures an Expert specifically for extracting content from documents.
// This expert is designed to analyze various document types (text, images, PDFs) and provide a detailed description.
func NewContentExpert() *Expert {
	expert := NewExpert("ContentExtractor", "An expert at extracting detailed content from various document types.")
	expert.ModelName = "gemini-2.5-pro" // Use a multimodal model for robust content extraction
	// This expert does not use external tools; its role is purely generative based on input.
	expert.Config = &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		},
		SystemInstruction: &genai.Content{Parts: []*genai.Part{
			{Text: `You are an expert document content extractor. 
			Your task is to analyze the provided document content:
			 - identify its type and official nature of the document, you can use Google search for confirmation.
			 - extract all relevant information in a structured and detailed manner. 
			
			Focus on key entities: 
			  - personal data
			  - dates: creation, expiration etc.
			  - numbers (like identification number, document number, 
			  - and any other data relative to a person identity. 
			If the document is an image, describe its visual content and any text present. Try to see if it 
			looks like a well know type of document.
			If it's a text document, summarize its main points and extract critical details.
			 Present the extracted information clearly and concisely.`},
		}},
	}
	return expert
}

// NewAdminExpert creates an expert knowledgeable in administrative procedures.
// This expert uses Google Search to devise plans for tasks like registering with
// government agencies and can outline the necessary steps and documents.
func NewAdminExpert() *Expert {
	expert := NewExpert("AdminExpert", "An expert on administrative procedures and paperwork.")
	expert.ModelName = "gemini-2.5-pro" // A powerful model for reasoning and planning
	expert.Config = &genai.GenerateContentConfig{
		Tools: []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		},
		SystemInstruction: &genai.Content{Parts: []*genai.Part{
			{Text: `You are a world-class administrative assistant, an expert in navigating bureaucracy.
			Your primary role is to help users achieve their administrative goals by providing clear, actionable plans.

			When a user asks for help with a specific task (e.g., 'how do I register for unemployment?', 'what do I need to get a new passport?'), 
			your process is to help him with a plan:
				1.  Use Google Search to find the most current, official procedures for the user's specific request and location if provided.
				2.  Synthesize this information into a step-by-step plan.
				3.  For each step, clearly list the required documents.
				4.  Present the final plan to the user in a clear, easy-to-follow format.
			`},
		}},
	}
	return expert
}
