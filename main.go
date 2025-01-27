package main

import (
	"bufio"
	"embed"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/andygrunwald/go-jira"
	"github.com/go-git/go-git/v5"
	"github.com/joho/godotenv"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

//go:embed .env
var envFile embed.FS

func extractTicketID(branchName string) string {
	parts := strings.Split(branchName, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

func main() {
	// Load embedded .env file
	envAsFile, err := envFile.Open(".env")
	if err != nil {
		log.Fatal("Error reading embedded .env file:", err)
	}

	env, err := godotenv.Parse(envAsFile)
	if err != nil {
		log.Fatal("Error parsing .env file:", err)
	}

	// Get GitLab API key
	gitlabToken := env["GITLAB_TOKEN"]
	if gitlabToken == "" {
		log.Fatal("GITLAB_TOKEN not found in .env file")
	}

	// Get YouTrack API key
	youtrackToken := env["YOUTRACK_TOKEN"]
	if youtrackToken == "" {
		log.Fatal("YOUTRACK_TOKEN not found in .env file")
	}

	youtrackURL := env["YOUTRACK_URL"]
	if youtrackURL == "" {
		log.Fatal("YOUTRACK_URL not found in .env file")
	}

	// Open the current directory as a Git repository
	repo, err := git.PlainOpen(".")
	if err != nil {
		log.Fatal("Error opening git repository:", err)
	}

	// Get the remote URL
	remote, err := repo.Remote("origin")
	if err != nil {
		log.Fatal("Error getting remote:", err)
	}

	remoteURL := remote.Config().URLs[0]

	// Check if it's a GitLab repository
	if !strings.Contains(remoteURL, "gitlab") {
		log.Fatal("Not a GitLab repository")
	}

	// Extract project path from remote URL
	// Example: git@gitlab.com:namespace/project.git or https://gitlab.com/namespace/project.git
	var projectPath string
	if strings.Contains(remoteURL, "@gitlab.com:") {
		// SSH format
		parts := strings.Split(remoteURL, ":")
		projectPath = strings.TrimSuffix(parts[1], ".git")
	} else {
		// HTTPS format
		parts := strings.Split(remoteURL, "gitlab.com/")
		projectPath = strings.TrimSuffix(parts[1], ".git")
	}

	fmt.Printf("Using GitLab project path: %s\n", projectPath)

	// Create GitLab client
	git, err := gitlab.NewClient(gitlabToken)
	if err != nil {
		log.Fatal("Failed to create GitLab client:", err)
	}

	// List merge requests
	opts := &gitlab.ListProjectMergeRequestsOptions{
		State: gitlab.String("opened"),
	}

	mrs, resp, err := git.MergeRequests.ListProjectMergeRequests(projectPath, opts)
	if err != nil {
		if resp != nil {
			log.Fatalf("GitLab API error (Status %d): %v\nFull response: %+v", resp.StatusCode, err, resp)
		} else {
			log.Fatalf("GitLab API error: %v", err)
		}
	}

	// Print merge requests with numbers
	fmt.Println("Open Merge Requests:")
	fmt.Println("-------------------")
	for i, mr := range mrs {
		fmt.Printf("[%d] Title: %s\n", i+1, mr.Title)
		fmt.Printf("    Author: %s\n", mr.Author.Username)
		fmt.Printf("    Source Branch: %s\n", mr.SourceBranch)
		fmt.Printf("    Target Branch: %s\n", mr.TargetBranch)
		fmt.Printf("    URL: %s\n", mr.WebURL)
		fmt.Printf("    Created: %s\n", mr.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Println("-------------------")
	}

	// Get current branch
	head, err := repo.Head()
	if err != nil {
		log.Fatal("Error getting current branch:", err)
	}
	currentBranch := head.Name().Short()

	// Find matching MR
	var selectedMR *gitlab.MergeRequest
	for _, mr := range mrs {
		if mr.SourceBranch == currentBranch {
			selectedMR = mr
			break
		}
	}

	if selectedMR == nil {
		fmt.Println("No open merge request found for current branch:", currentBranch)
		fmt.Println("\nAvailable merge requests:")
		fmt.Println("-------------------")
		for i, mr := range mrs {
			fmt.Printf("[%d] Title: %s\n", i+1, mr.Title)
			fmt.Printf("    Source Branch: %s\n", mr.SourceBranch)
			fmt.Printf("    URL: %s\n", mr.WebURL)
			fmt.Println("-------------------")
		}

		// Fall back to manual selection
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Select merge request number (1-" + strconv.Itoa(len(mrs)) + "): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		selection, err := strconv.Atoi(input)
		if err != nil || selection < 1 || selection > len(mrs) {
			log.Fatal("Invalid selection")
		}
		selectedMR = mrs[selection-1]
	} else {
		fmt.Printf("Found merge request for current branch '%s':\n", currentBranch)
		fmt.Printf("Title: %s\n", selectedMR.Title)
		fmt.Printf("URL: %s\n", selectedMR.WebURL)
	}

	ticketID := extractTicketID(selectedMR.SourceBranch)
	if ticketID == "" {
		log.Fatal("Could not extract ticket ID from branch name:", selectedMR.SourceBranch)
	}

	// Initialize YouTrack client
	tp := jira.BasicAuthTransport{
		Username: "Bearer",
		Password: youtrackToken,
	}
	youtrackClient, err := jira.NewClient(tp.Client(), strings.TrimSuffix(youtrackURL, "/"))
	if err != nil {
		log.Fatal("Error creating YouTrack client:", err)
	}

	// Extract repo name from project path
	pathParts := strings.Split(projectPath, "/")
	repoName := pathParts[len(pathParts)-1]

	// Add comment to YouTrack
	commentInput := struct {
		Text string `json:"text"`
	}{
		Text: fmt.Sprintf("%s MR: %s", repoName, selectedMR.WebURL),
	}

	req, err := youtrackClient.NewRequest("POST", fmt.Sprintf("/api/issues/%s/comments", ticketID), commentInput)
	if err != nil {
		log.Fatal("Error creating request:", err)
	}

	_, err = youtrackClient.Do(req, nil)
	if err != nil {
		log.Fatal("Error adding comment to YouTrack:", err)
	}

	fmt.Printf("Successfully added MR link to YouTrack ticket %s\n", ticketID)
}
