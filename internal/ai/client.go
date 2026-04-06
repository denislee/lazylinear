package ai

import (
	"fmt"
	"os/exec"
	"strings"
)

// AIClient defines the interface for interacting with an AI service to categorize issues.
type AIClient interface {
	CategorizeIssue(identifier, title, description string, allowedCategories []string) (string, error)
}

// GeminiClient is an implementation of AIClient that uses the Gemini CLI.
type GeminiClient struct{}

// NewGeminiClient creates a new GeminiClient.
func NewGeminiClient() *GeminiClient {
	return &GeminiClient{}
}

// CategorizeIssue categorizes an issue into one of the allowed categories.
func (c *GeminiClient) CategorizeIssue(identifier, title, description string, allowedCategories []string) (string, error) {
	desc := description
	if len(desc) > 300 {
		desc = desc[:300]
	}

	prompt := fmt.Sprintf(
		"Categorize this Linear issue into EXACTLY ONE of these categories:\n%s\n\nIssue:\nID: %s\nTitle: %s\nDescription: %s\n\nRespond ONLY with the category name. Do not include any other text.",
		strings.Join(allowedCategories, ", "),
		identifier,
		title,
		desc,
	)

	cmd := exec.Command("gemini", "-p", prompt)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gemini command failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
