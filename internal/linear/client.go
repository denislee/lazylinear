package linear

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const apiURL = "https://api.linear.app/graphql"

// Client is a Linear API client.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Linear API client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// graphQLRequest is the JSON body sent to the GraphQL endpoint.
type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// graphQLResponse is the JSON body returned from the GraphQL endpoint.
type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// execute sends a GraphQL request and decodes the response data into result.
func (c *Client) execute(query string, variables map[string]any, result any) error {
	body, err := json.Marshal(graphQLRequest{
		Query:     query,
		Variables: variables,
	})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var gqlResp graphQLResponse
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("graphql error: %s", gqlResp.Errors[0].Message)
	}

	if result != nil {
		if err := json.Unmarshal(gqlResp.Data, result); err != nil {
			return fmt.Errorf("decode data: %w", err)
		}
	}

	return nil
}

// GetViewer returns the authenticated user.
func (c *Client) GetViewer() (*User, error) {
	var resp struct {
		Viewer User `json:"viewer"`
	}
	if err := c.execute(queryViewer, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Viewer, nil
}

// GetTeams returns all teams the authenticated user belongs to.
func (c *Client) GetTeams() ([]Team, error) {
	var resp struct {
		Teams struct {
			Nodes []Team `json:"nodes"`
		} `json:"teams"`
	}
	if err := c.execute(queryTeams, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Teams.Nodes, nil
}

// GetIssues returns a paginated list of issues for a team.
// The filter parameter is optional and can be nil.
func (c *Client) GetIssues(teamID string, first int, after string, filter map[string]any) (*IssueConnection, error) {
	vars := map[string]any{
		"teamId": teamID,
		"first":  first,
	}
	if after != "" {
		vars["after"] = after
	}
	if filter != nil {
		vars["filter"] = filter
	}

	var resp struct {
		Team struct {
			Issues IssueConnection `json:"issues"`
		} `json:"team"`
	}
	if err := c.execute(queryIssues, vars, &resp); err != nil {
		return nil, err
	}
	return &resp.Team.Issues, nil
}

// GetIssue returns a single issue by ID.
func (c *Client) GetIssue(id string) (*Issue, error) {
	vars := map[string]any{
		"id": id,
	}
	var resp struct {
		Issue Issue `json:"issue"`
	}
	if err := c.execute(queryIssue, vars, &resp); err != nil {
		return nil, err
	}
	return &resp.Issue, nil
}

// GetWorkflowStates returns all workflow states for a team.
func (c *Client) GetWorkflowStates(teamID string) ([]WorkflowState, error) {
	vars := map[string]any{
		"teamId": teamID,
	}
	var resp struct {
		Team struct {
			States struct {
				Nodes []WorkflowState `json:"nodes"`
			} `json:"states"`
		} `json:"team"`
	}
	if err := c.execute(queryWorkflowStates, vars, &resp); err != nil {
		return nil, err
	}
	return resp.Team.States.Nodes, nil
}

// CreateIssue creates a new issue and returns it.
func (c *Client) CreateIssue(input IssueCreateInput) (*Issue, error) {
	vars := map[string]any{
		"input": input,
	}
	var resp struct {
		IssueCreate struct {
			Success bool  `json:"success"`
			Issue   Issue `json:"issue"`
		} `json:"issueCreate"`
	}
	if err := c.execute(mutationCreateIssue, vars, &resp); err != nil {
		return nil, err
	}
	if !resp.IssueCreate.Success {
		return nil, fmt.Errorf("issue creation failed")
	}
	return &resp.IssueCreate.Issue, nil
}

// UpdateIssue updates an existing issue and returns it.
func (c *Client) UpdateIssue(id string, input IssueUpdateInput) (*Issue, error) {
	vars := map[string]any{
		"id":    id,
		"input": input,
	}
	var resp struct {
		IssueUpdate struct {
			Success bool  `json:"success"`
			Issue   Issue `json:"issue"`
		} `json:"issueUpdate"`
	}
	if err := c.execute(mutationUpdateIssue, vars, &resp); err != nil {
		return nil, err
	}
	if !resp.IssueUpdate.Success {
		return nil, fmt.Errorf("issue update failed")
	}
	return &resp.IssueUpdate.Issue, nil
}
