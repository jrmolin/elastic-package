// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package llmagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/docs"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/tui"
)

const (
	initialPrompt = `You are an expert technical writer specializing in documentation for Elastic Integrations. Your mission is to create a comprehensive, user-friendly README.md file by synthesizing information from the integration's source code, external research, and a provided template.

Core Task:

Generate or update the _dev/build/docs/README.md file for the integration specified below.

* Package Name: %s
* Title: %s
* Type: %s
* Version: %s
* Description: %s


Critical Directives (Follow These Strictly):

1.  File Restriction: You MUST ONLY write to the _dev/build/docs/README.md file. Do not modify any other files.
2.  Preserve Human Content: You MUST preserve any content between <!-- HUMAN-EDITED START --> and <!-- HUMAN-EDITED END --> comment blocks. This content is non-negotiable and must be kept verbatim in its original position.
3.  No Hallucination: If you cannot find a piece of information in the package files or through web search, DO NOT invent it. Instead, insert a clear placeholder in the document: << INFORMATION NOT AVAILABLE - PLEASE UPDATE >>.

Available Tools (Use These for All Operations):

* list_directory: List files and directories in the package. Use path="" for package root.
* read_file: Read contents of files within the package. Provide relative path from package root.
* write_file: Write content to files. Can only write to _dev/build/docs/ directory.
* get_readme_template: Get the README.md template structure you must follow.
* get_example_readme: Get a high-quality example README for reference on style and quality.

Tool Usage Guidelines:
- Always use get_readme_template first to understand the required structure
- Use get_example_readme to understand the target quality and style
- Use list_directory and read_file extensively to analyze the package structure and content
- All file paths for read_file must be relative to package root (e.g., "manifest.yml", "data_stream/logs/manifest.yml")
- Only use write_file for the final README.md in _dev/build/docs/README.md

Your Step-by-Step Process:

1.  Get Template and Example:
    * First, call get_readme_template to get the structure you must follow
    * Call get_example_readme to understand the target quality and style

2.  Initial Analysis:
    * Begin by listing the contents of the package to understand its structure.
    * Read the existing _dev/build/docs/README.md (if it exists) to identify its current state and locate any human-edited sections that must be preserved.

3.  Internal Information Gathering:
    * Analyze the package files to extract key details. Pay close attention to:
        * manifest.yml: For top-level metadata, owner, license, and supported Elasticsearch versions.
        * data_stream/*/manifest.yml: To compile a list of all data streams, their types (logs, metrics), and a brief description of the data each collects.
        * data_stream/*/fields/fields.yml: To understand the data schema and important fields. Mentioning a few key fields can be helpful for users.

4.  External Information Gathering:
    * Use your web search tool to find the official documentation for the service or technology this integration supports (e.g., "NGINX logs setup," "AWS S3 access logs format").
    * Your goal is to find **actionable, step-by-step instructions** for users on how to configure the *source system* to generate the data this integration is designed to collect.

5.  Drafting the Documentation:
    * Using the template from get_readme_template, begin writing the README.md.
    * Follow the style and quality demonstrated in the example from get_example_readme.
    * Integrate the information gathered from the package files and your web research into the appropriate sections.
    * Re-insert any preserved human-edited sections into their original locations.

6.  Review and Finalize:
    * Read through your generated README to ensure it is clear, accurate, and easy to follow.
    * Verify that all critical directives (file restrictions, content preservation) have been followed.
    * Confirm that the tone and style align with the high-quality example.

7. Write the results:
    * Write the generated README to _dev/build/docs/README.md using the write_file tool.
    * Do not return the results as a response in this conversation.

Style and Content Guidance:

* Audience & Tone: Write for a technical audience (e.g., DevOps Engineers, SREs, Security Analysts). The tone should be professional, clear, and direct. Use active voice.
* Template is a Blueprint: The template from get_readme_template is your required structure. Follow it closely.
* The Example is Your "Gold Standard": The example from get_example_readme demonstrates the target quality, level of detail, and formatting. Emulate its style, especially in the "Configuration" and "Setup" sections. Explain *why* a step is needed, not just *what* the step is.
* Be Specific: Instead of saying "configure the service," provide a concrete configuration snippet or a numbered list of steps. Link to official external documentation where appropriate to provide users with more depth.

Please begin. Start by getting the template and example, then proceed with the "Initial Analysis" step.`
	revisionPrompt = `You are continuing to work on documentation for an Elastic Integration. You have access to tools to analyze the package and make changes.

CURRENT TASK: Make specific revisions to the existing documentation based on user feedback.

Package Information:
* Package Name: %s
* Title: %s
* Type: %s
* Version: %s
* Description: %s

Critical Directives (Follow These Strictly):
1. File Restriction: You MUST ONLY write to the _dev/build/docs/README.md file. Do not modify any other files.
2. Preserve Human Content: You MUST preserve any content between <!-- HUMAN-EDITED START --> and <!-- HUMAN-EDITED END --> comment blocks.
3. Read Current Content: First read the existing _dev/build/docs/README.md to understand the current state.
4. No Hallucination: If you need information not available in package files, insert placeholders: << INFORMATION NOT AVAILABLE - PLEASE UPDATE >>.

Available Tools (Use These for All Operations):

* list_directory: List files and directories in the package. Use path="" for package root.
* read_file: Read contents of files within the package. Provide relative path from package root.
* write_file: Write content to files. Can only write to _dev/build/docs/ directory.
* get_readme_template: Get the README.md template structure you must follow.
* get_example_readme: Get a high-quality example README for reference on style and quality.

Tool Usage Guidelines:
- Use get_readme_template to understand the required structure if needed
- Use get_example_readme to understand the target quality and style if needed
- Use list_directory and read_file extensively to analyze the package structure and content
- All file paths for read_file must be relative to package root (e.g., "manifest.yml", "data_stream/logs/manifest.yml")
- Only use write_file for the final README.md in _dev/build/docs/README.md

Your Step-by-Step Process:
1. Read the current _dev/build/docs/README.md file to understand what exists
2. If needed, get template and example references using get_readme_template and get_example_readme
3. Analyze the requested changes carefully
4. Use available tools to gather any additional information needed
5. Make the specific changes requested while preserving existing good content
6. Ensure the result is comprehensive and follows Elastic documentation standards
7. Write the generated README to _dev/build/docs/README.md using write_file

User-Requested Changes:
%s

Begin by reading the current README.md file, then implement the requested changes thoughtfully.`
)

// DocumentationAgent handles documentation updates for packages
type DocumentationAgent struct {
	agent                 *Agent
	packageRoot           string
	originalReadmeContent *string // Stores original README content for restoration on cancel
}

// NewDocumentationAgent creates a new documentation agent
func NewDocumentationAgent(provider LLMProvider, packageRoot string) (*DocumentationAgent, error) {
	var tools []Tool
	// Load the mcp file
	servers := MCPTools()
	if servers != nil {
		for _, srv := range servers.Inner {
			if len(srv.Tools) > 0 {
				tools = append(tools, srv.Tools...)
			}
		}

	}

	// Create tools for package operations
	tools = append(tools, PackageTools(packageRoot)...)

	// Create the agent
	agent := NewAgent(provider, tools)

	return &DocumentationAgent{
		agent:       agent,
		packageRoot: packageRoot,
	}, nil
}

// UpdateDocumentation runs the documentation update process
func (d *DocumentationAgent) UpdateDocumentation(ctx context.Context, nonInteractive bool) error {
	// Read package manifest for context
	manifest, err := packages.ReadPackageManifestFromPackageRoot(d.packageRoot)
	if err != nil {
		return fmt.Errorf("failed to read package manifest: %w", err)
	}

	// Backup original README content before making any changes
	d.backupOriginalReadme()

	// Create the initial prompt
	prompt := d.buildInitialPrompt(manifest)

	if nonInteractive {
		return d.runNonInteractiveMode(ctx, prompt)
	}

	return d.runInteractiveMode(ctx, prompt)
}

// runNonInteractiveMode handles the non-interactive documentation update flow
func (d *DocumentationAgent) runNonInteractiveMode(ctx context.Context, prompt string) error {
	fmt.Println("Starting non-interactive documentation update process...")
	fmt.Println("The LLM agent will analyze your package and generate documentation automatically.")
	fmt.Println()

	// First attempt
	result, err := d.executeTaskWithLogging(ctx, prompt)
	if err != nil {
		return err
	}

	// Show the result
	fmt.Println("\n📝 Agent Response:")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Println(result.FinalContent)
	fmt.Println(strings.Repeat("-", 50))

	// Check for token limit messages first - these need special handling
	if isTokenLimitMessage(result.FinalContent) {
		fmt.Println("\n⚠️  LLM hit token limits. Switching to section-based generation...")
		newPrompt, err := d.handleTokenLimitResponse(result.FinalContent)
		if err != nil {
			return fmt.Errorf("failed to handle token limit: %w", err)
		}

		// Retry with section-based approach
		if _, err := d.executeTaskWithLogging(ctx, newPrompt); err != nil {
			return fmt.Errorf("section-based retry failed: %w", err)
		}

		// Check if README was successfully updated after retry
		if updated, err := d.handleReadmeUpdate(); updated {
			fmt.Println("\n📄 README.md was updated successfully with section-based approach!")
			return err
		}
	}

	// Check for errors in response using enhanced detection with conversation context
	if isTaskResultError(result.FinalContent, result.Conversation) {
		fmt.Println("\n❌ Error detected in LLM response.")
		fmt.Println("In non-interactive mode, exiting due to error.")
		return fmt.Errorf("LLM agent encountered an error: %s", result.FinalContent)
	}

	// Check if README was successfully updated
	if updated, err := d.handleReadmeUpdate(); updated {
		fmt.Println("\n📄 README.md was updated successfully!")
		return err
	}

	// Second attempt with specific instructions
	fmt.Println("⚠️  No README.md was updated. Trying again with specific instructions...")
	specificPrompt := "You haven't updated a README.md file yet. Please create the README.md file in the _dev/build/docs/ directory based on your analysis. This is required to complete the task."

	if _, err := d.executeTaskWithLogging(ctx, specificPrompt); err != nil {
		return fmt.Errorf("second attempt failed: %w", err)
	}

	// Final check
	if updated, err := d.handleReadmeUpdate(); updated {
		fmt.Println("\n📄 README.md was updated on second attempt!")
		return err
	}

	return fmt.Errorf("failed to create README.md after two attempts")
}

// runInteractiveMode handles the interactive documentation update flow
func (d *DocumentationAgent) runInteractiveMode(ctx context.Context, prompt string) error {
	fmt.Println("Starting documentation update process...")
	fmt.Println("The LLM agent will analyze your package and update the documentation.")
	fmt.Println()

	for {
		// Execute the task
		result, err := d.executeTaskWithLogging(ctx, prompt)
		if err != nil {
			return err
		}

		// Check for token limit messages first - these need special handling
		if isTokenLimitMessage(result.FinalContent) {
			fmt.Println("\n⚠️  LLM hit token limits. Switching to section-based generation...")
			newPrompt, err := d.handleTokenLimitResponse(result.FinalContent)
			if err != nil {
				return err
			}
			prompt = newPrompt
			continue
		}

		// Handle error responses using enhanced detection with conversation context
		if isTaskResultError(result.FinalContent, result.Conversation) {
			newPrompt, shouldContinue, err := d.handleInteractiveError()
			if err != nil {
				return err
			}
			if !shouldContinue {
				d.restoreOriginalReadme()
				return fmt.Errorf("user chose to exit due to LLM error")
			}
			prompt = newPrompt
			continue
		}

		// Display README content if updated
		readmeUpdated := d.displayReadmeIfUpdated()

		// Get user action
		action, err := d.getUserAction()
		if err != nil {
			return err
		}

		// Handle user action
		newPrompt, shouldContinue, shouldExit, err := d.handleUserAction(action, readmeUpdated)
		if err != nil {
			return err
		}
		if shouldExit {
			return nil
		}
		if shouldContinue {
			prompt = newPrompt
			continue
		}
	}
}

// logAgentResponse logs debug information about the agent response
func (d *DocumentationAgent) logAgentResponse(result *TaskResult) {
	logger.Debugf("DEBUG: Full agent task response follows (may contain sensitive content)")
	logger.Debugf("Agent task response - Success: %t", result.Success)
	logger.Debugf("Agent task response - FinalContent: %s", result.FinalContent)
	logger.Debugf("Agent task response - Conversation entries: %d", len(result.Conversation))
	for i, entry := range result.Conversation {
		logger.Debugf("Agent task response - Conversation[%d]: type=%s, content_length=%d",
			i, entry.Type, len(entry.Content))
		logger.Tracef("Agent task response - Conversation[%d]: content=%s", i, entry.Content)
	}
}

// executeTaskWithLogging executes a task and logs the result
func (d *DocumentationAgent) executeTaskWithLogging(ctx context.Context, prompt string) (*TaskResult, error) {
	fmt.Println("🤖 LLM Agent is working...")

	result, err := d.agent.ExecuteTask(ctx, prompt)
	if err != nil {
		fmt.Println("❌ Agent task failed")
		fmt.Printf("❌ result is %v\n", result)
		return nil, fmt.Errorf("agent task failed: %w", err)
	}

	fmt.Println("✅ Task completed")
	d.logAgentResponse(result)
	return result, nil
}

// handleReadmeUpdate checks if README was updated and reports the result
func (d *DocumentationAgent) handleReadmeUpdate() (bool, error) {
	readmeUpdated := d.checkReadmeUpdated()
	if !readmeUpdated {
		return false, nil
	}

	content, err := d.readCurrentReadme()
	if err != nil || content == "" {
		return false, err
	}

	fmt.Printf("✅ Documentation update completed! (%d characters written)\n", len(content))
	return true, nil
}

// handleInteractiveError handles error responses in interactive mode
func (d *DocumentationAgent) handleInteractiveError() (string, bool, error) {
	fmt.Println("\n❌ Error detected in LLM response.")

	errorPrompt := tui.NewSelect("What would you like to do?", []string{
		"Try again",
		"Exit",
	}, "Try again")

	var errorAction string
	err := tui.AskOne(errorPrompt, &errorAction)
	if err != nil {
		return "", false, fmt.Errorf("prompt failed: %w", err)
	}

	if errorAction == "Exit" {
		fmt.Println("⚠️  Exiting due to LLM error.")
		return "", false, nil
	}

	// Continue with retry prompt
	newPrompt := d.buildRevisionPrompt("The previous attempt encountered an error. Please try a different approach to analyze the package and create/update the documentation.")
	return newPrompt, true, nil
}

// handleUserAction processes the user's chosen action
func (d *DocumentationAgent) handleUserAction(action string, readmeUpdated bool) (string, bool, bool, error) {
	switch action {
	case "Accept and finalize":
		return d.handleAcceptAction(readmeUpdated)
	case "Request changes":
		return d.handleRequestChanges()
	case "Cancel":
		fmt.Println("❌ Documentation update cancelled.")
		d.restoreOriginalReadme()
		return "", false, true, nil
	default:
		return "", false, false, fmt.Errorf("unknown action: %s", action)
	}
}

// handleAcceptAction handles the "Accept and finalize" action
func (d *DocumentationAgent) handleAcceptAction(readmeUpdated bool) (string, bool, bool, error) {
	if readmeUpdated {
		// Validate preserved sections if we had original content
		if d.originalReadmeContent != nil {
			if newContent, err := d.readCurrentReadme(); err == nil {
				warnings := d.validatePreservedSections(*d.originalReadmeContent, newContent)
				if len(warnings) > 0 {
					fmt.Println("⚠️  Warning: Some human-edited sections may not have been preserved:")
					for _, warning := range warnings {
						fmt.Printf("   - %s\n", warning)
					}
					fmt.Println("   Please review the documentation to ensure important content wasn't lost.")
				}
			}
		}

		fmt.Println("✅ Documentation update completed!")
		return "", false, true, nil
	}

	// README wasn't updated - ask user what to do
	continuePrompt := tui.NewSelect("README.md file wasn't updated. What would you like to do?", []string{
		"Try again",
		"Exit anyway",
	}, "Try again")

	var continueChoice string
	err := tui.AskOne(continuePrompt, &continueChoice)
	if err != nil {
		return "", false, false, fmt.Errorf("prompt failed: %w", err)
	}

	if continueChoice == "Exit anyway" {
		fmt.Println("⚠️  Exiting without creating README.md file.")
		d.restoreOriginalReadme()
		return "", false, true, nil
	}

	fmt.Println("🔄 Trying again to create README.md...")
	newPrompt := d.buildRevisionPrompt("You haven't written a README.md file yet. Please write the README.md file in the _dev/build/docs/ directory based on your analysis.")
	return newPrompt, true, false, nil
}

// handleRequestChanges handles the "Request changes" action
func (d *DocumentationAgent) handleRequestChanges() (string, bool, bool, error) {
	changes, err := tui.AskTextArea("What changes would you like to make to the documentation?")
	if err != nil {
		// Check if user cancelled
		if errors.Is(err, tui.ErrCancelled) {
			fmt.Println("⚠️  Changes request cancelled.")
			return "", true, false, nil // Continue the loop
		}
		return "", false, false, fmt.Errorf("prompt failed: %w", err)
	}

	// Check if no changes were provided
	if strings.TrimSpace(changes) == "" {
		fmt.Println("⚠️  No changes specified. Please try again.")
		return "", true, false, nil // Continue the loop
	}

	newPrompt := d.buildRevisionPrompt(changes)
	return newPrompt, true, false, nil
}

// buildInitialPrompt creates the initial prompt for the LLM
func (d *DocumentationAgent) buildInitialPrompt(manifest *packages.PackageManifest) string {
	return fmt.Sprintf(initialPrompt,
		manifest.Name,
		manifest.Title,
		manifest.Type,
		manifest.Version,
		manifest.Description)
}

// buildRevisionPrompt creates a comprehensive prompt for document revisions that includes all necessary context
func (d *DocumentationAgent) buildRevisionPrompt(changes string) string {
	// Read package manifest for context
	manifest, err := packages.ReadPackageManifestFromPackageRoot(d.packageRoot)
	if err != nil {
		// Fallback to a simpler prompt if we can't read the manifest
		return fmt.Sprintf("Please make the following changes to the documentation:\n\n%s", changes)
	}

	return fmt.Sprintf(revisionPrompt,
		manifest.Name,
		manifest.Title,
		manifest.Type,
		manifest.Version,
		manifest.Description,
		changes)
}

// handleTokenLimitResponse creates a section-based prompt when LLM hits token limits
func (d *DocumentationAgent) handleTokenLimitResponse(originalResponse string) (string, error) {
	// Read package manifest for context
	manifest, err := packages.ReadPackageManifestFromPackageRoot(d.packageRoot)
	if err != nil {
		return "", fmt.Errorf("failed to read package manifest: %w", err)
	}

	// Create a section-based generation prompt
	sectionBasedPrompt := d.buildSectionBasedPrompt(manifest)
	return sectionBasedPrompt, nil
}

// buildSectionBasedPrompt creates a prompt for generating README in sections
func (d *DocumentationAgent) buildSectionBasedPrompt(manifest *packages.PackageManifest) string {
	return fmt.Sprintf(`You previously hit token limits when generating documentation. Let's break this into manageable sections.

CURRENT TASK: Generate README.md documentation section by section for the integration below.

Package Information:
* Package Name: %s
* Title: %s
* Type: %s
* Version: %s
* Description: %s

IMPORTANT INSTRUCTIONS:

1. **Section-Based Approach**: Instead of generating the entire README at once, we'll build it section by section.

2. **Current Strategy**: 
   - First, use get_readme_template to understand the required structure
   - Then generate ONLY the first major section (Overview/Introduction)
   - Write that section to the file
   - In subsequent iterations, we'll add more sections

3. **First Section Focus**: 
   - Start with the Overview/Introduction section only
   - Include: Brief description, compatibility info, and how it works
   - Keep this section under 1000 words to avoid token limits

4. **Available Tools**: 
   - get_readme_template: Get the template structure
   - get_example_readme: Get style reference
   - list_directory, read_file: Analyze package
   - write_file: Write the section to _dev/build/docs/README.md

5. **File Strategy**:
   - Read existing README (if any) to preserve human-edited sections
   - Write the first section, preserving any existing content
   - Later iterations will append additional sections

STEP-BY-STEP PROCESS:
1. Get the template structure using get_readme_template
2. Read current README.md (if exists) to understand what's already there
3. Analyze package structure briefly using list_directory
4. Generate ONLY the Overview/Introduction section
5. Write this section to the README.md file

Begin by getting the template, then focus on creating just the first section.`,
		manifest.Name,
		manifest.Title,
		manifest.Type,
		manifest.Version,
		manifest.Description)
}

// displayReadmeIfUpdated shows README content if it was updated
func (d *DocumentationAgent) displayReadmeIfUpdated() bool {
	readmeUpdated := d.checkReadmeUpdated()
	if !readmeUpdated {
		fmt.Println("\n⚠️  README.md file not updated")
		return false
	}

	sourceContent, err := d.readCurrentReadme()
	if err != nil || sourceContent == "" {
		fmt.Println("\n⚠️  README.md file exists but could not be read or is empty")
		return false
	}

	// Try to render the content
	renderedContent, shouldBeRendered, err := docs.GenerateReadme("README.md", d.packageRoot)
	if err != nil || !shouldBeRendered {
		fmt.Println("\n⚠️  The generated README.md could not be rendered.")
		fmt.Println("It's recommended that you do not accept this version (ask for revisions or cancel).")
		return true
	} else {
		// Show the processed/rendered content
		processedContentStr := string(renderedContent)
		fmt.Printf("📊 Processed README stats: %d characters, %d lines\n", len(processedContentStr), strings.Count(processedContentStr, "\n")+1)

		title := "📄 Processed README.md (as generated by elastic-package build)"
		if err := tui.ShowContent(title, processedContentStr); err != nil {
			// Fallback to simple print if viewer fails
			fmt.Printf("\n%s:\n", title)
			fmt.Println(strings.Repeat("=", 70))
			fmt.Println(processedContentStr)
			fmt.Println(strings.Repeat("=", 70))
		}
	}

	return true
}

// getUserAction prompts the user for their next action
func (d *DocumentationAgent) getUserAction() (string, error) {
	selectPrompt := tui.NewSelect("What would you like to do?", []string{
		"Accept and finalize",
		"Request changes",
		"Cancel",
	}, "Accept and finalize")

	var action string
	err := tui.AskOne(selectPrompt, &action)
	if err != nil {
		return "", fmt.Errorf("prompt failed: %w", err)
	}

	return action, nil
}

// checkReadmeUpdated checks if README.md has been updated by comparing current content to originalReadmeContent
func (d *DocumentationAgent) checkReadmeUpdated() bool {
	readmePath := filepath.Join(d.packageRoot, "_dev", "build", "docs", "README.md")

	// Check if file exists
	if _, err := os.Stat(readmePath); err != nil {
		return false
	}

	// Read current content
	currentContent, err := os.ReadFile(readmePath)
	if err != nil {
		return false
	}

	currentContentStr := string(currentContent)

	// If there was no original content, any new content means it's updated
	if d.originalReadmeContent == nil {
		return currentContentStr != ""
	}

	// Compare current content with original content
	return currentContentStr != *d.originalReadmeContent
}

// readCurrentReadme reads the current README.md content
func (d *DocumentationAgent) readCurrentReadme() (string, error) {
	readmePath := filepath.Join(d.packageRoot, "_dev", "build", "docs", "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// validatePreservedSections checks if human-edited sections are preserved in the new content
func (d *DocumentationAgent) validatePreservedSections(originalContent, newContent string) []string {
	var warnings []string

	// Extract preserved sections from original content
	preservedSections := d.extractPreservedSections(originalContent)

	// Check if each preserved section exists in the new content
	for marker, content := range preservedSections {
		if !strings.Contains(newContent, content) {
			warnings = append(warnings, fmt.Sprintf("Human-edited section '%s' was not preserved", marker))
		}
	}

	return warnings
}

// isErrorResponse detects if the LLM response indicates an error occurred
// This is now a wrapper that calls the more sophisticated analysis function
func isErrorResponse(content string) bool {
	// Use the enhanced error detection that considers conversation context
	return isTaskResultError(content, nil)
}

// isTaskResultError provides sophisticated error detection considering conversation context
func isTaskResultError(content string, conversation []ConversationEntry) bool {
	// Empty content is not necessarily an error - it might be after successful tool execution
	if strings.TrimSpace(content) == "" {
		// If we have conversation context, check if recent tools succeeded
		if conversation != nil && hasRecentSuccessfulTools(conversation) {
			return false
		}
		// Empty content without context might indicate a problem, but let's be lenient
		return false
	}

	// Check for token limit messages - these are NOT errors, they're recoverable conditions
	if isTokenLimitMessage(content) {
		return false
	}

	errorIndicators := []string{
		"I encountered an error",
		"I'm experiencing an error",
		"I cannot complete",
		"I'm unable to complete",
		"Something went wrong",
		"There was an error",
		"I'm having trouble",
		"I failed to",
		"Error occurred",
		"Task did not complete within maximum iterations",
	}

	contentLower := strings.ToLower(content)

	// Check for explicit error indicators
	hasErrorIndicator := false
	for _, indicator := range errorIndicators {
		if strings.Contains(contentLower, strings.ToLower(indicator)) {
			hasErrorIndicator = true
			break
		}
	}

	if !hasErrorIndicator {
		return false
	}

	// If we have conversation context and recent tools succeeded, this might be a false error
	if conversation != nil && hasRecentSuccessfulTools(conversation) {
		return false
	}

	return true
}

// isTokenLimitMessage detects if the LLM response indicates it hit token limits
func isTokenLimitMessage(content string) bool {
	tokenLimitIndicators := []string{
		"I reached the maximum response length",
		"maximum response length",
		"reached the token limit",
		"response is too long",
		"breaking this into smaller tasks",
		"due to length constraints",
		"response length limit",
		"token limit reached",
		"output limit exceeded",
		"maximum length exceeded",
	}

	contentLower := strings.ToLower(content)
	for _, indicator := range tokenLimitIndicators {
		if strings.Contains(contentLower, strings.ToLower(indicator)) {
			return true
		}
	}
	return false
}

// hasRecentSuccessfulTools checks if recent tool executions in the conversation were successful
func hasRecentSuccessfulTools(conversation []ConversationEntry) bool {
	// Look at the last few conversation entries for successful tool results
	for i := len(conversation) - 1; i >= 0 && i >= len(conversation)-5; i-- {
		entry := conversation[i]
		if entry.Type == "tool_result" {
			content := strings.ToLower(entry.Content)
			// Check for success indicators
			if strings.Contains(content, "✅ success") ||
				strings.Contains(content, "successfully wrote") ||
				strings.Contains(content, "completed successfully") {
				return true
			}
			// If we hit an actual error, stop looking
			if strings.Contains(content, "❌ error") ||
				strings.Contains(content, "failed:") ||
				strings.Contains(content, "access denied") {
				return false
			}
		}
	}
	return false
}

// extractPreservedSections extracts all human-edited sections from content
func (d *DocumentationAgent) extractPreservedSections(content string) map[string]string {
	sections := make(map[string]string)

	// Define marker pairs
	markers := []struct {
		start, end string
		name       string
	}{
		{"<!-- HUMAN-EDITED START -->", "<!-- HUMAN-EDITED END -->", "HUMAN-EDITED"},
		{"<!-- PRESERVE START -->", "<!-- PRESERVE END -->", "PRESERVE"},
	}

	for _, marker := range markers {
		startIdx := 0
		sectionNum := 1

		for {
			start := strings.Index(content[startIdx:], marker.start)
			if start == -1 {
				break
			}
			start += startIdx

			end := strings.Index(content[start:], marker.end)
			if end == -1 {
				break
			}
			end += start

			// Extract the full section including markers
			sectionContent := content[start : end+len(marker.end)]
			sectionKey := fmt.Sprintf("%s-%d", marker.name, sectionNum)
			sections[sectionKey] = sectionContent

			startIdx = end + len(marker.end)
			sectionNum++
		}
	}

	return sections
}

// backupOriginalReadme stores the current README content for potential restoration and comparison to the generated version
func (d *DocumentationAgent) backupOriginalReadme() {
	readmePath := filepath.Join(d.packageRoot, "_dev", "build", "docs", "README.md")

	// Check if README exists
	if _, err := os.Stat(readmePath); err == nil {
		// Read and store the original content
		if content, err := os.ReadFile(readmePath); err == nil {
			contentStr := string(content)
			d.originalReadmeContent = &contentStr
			fmt.Printf("📋 Backed up original README.md (%d characters)\n", len(contentStr))
		} else {
			fmt.Printf("⚠️  Could not read original README.md for backup: %v\n", err)
		}
	} else {
		d.originalReadmeContent = nil
		fmt.Println("📋 No existing README.md found - will create new one")
	}
}

// restoreOriginalReadme restores the README to its original state
func (d *DocumentationAgent) restoreOriginalReadme() {
	readmePath := filepath.Join(d.packageRoot, "_dev", "build", "docs", "README.md")

	if d.originalReadmeContent != nil {
		// Restore original content
		if err := os.WriteFile(readmePath, []byte(*d.originalReadmeContent), 0o644); err != nil {
			fmt.Printf("⚠️  Failed to restore original README.md: %v\n", err)
		} else {
			fmt.Printf("🔄 Restored original README.md (%d characters)\n", len(*d.originalReadmeContent))
		}
	} else {
		// No original file existed, so remove any file that was created
		if err := os.Remove(readmePath); err != nil {
			if !os.IsNotExist(err) {
				fmt.Printf("⚠️  Failed to remove created README.md: %v\n", err)
			}
		} else {
			fmt.Println("🗑️  Removed created README.md file - restored to original state (no file)")
		}
	}
}
