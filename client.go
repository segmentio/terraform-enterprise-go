package tfe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	// DefaultBaseURL is the default base url to reach Terraform Enterprise
	DefaultBaseURL = "https://app.terraform.io"
)

// Error Types
var (
	ErrUnauthorized         = errors.New("User is not authorized to perform this action")
	ErrNotFound             = errors.New("Not found")
	ErrWorkspaceNotFound    = errors.New("Workspace not found")
	ErrStateVersionNotFound = errors.New("State version not found")
	ErrBadStatus            = errors.New("Unrecognized status code")
)

// Client exposes an API for communicating with Terraform Enterprise
type Client struct {
	// AtlasToken is the token used to authenticate with Terraform Enterprise,
	// you can generate one from the Terraform Enterprise UI
	AtlasToken string

	// BaseURL is the base used for all api calls.  If you are using
	// Terraform Enterprise SaaS, you can set this to DefaultBaseURL
	BaseURL string
}

// New creates and returns a new Terraform Enterprise client
func New(atlasToken string, baseURL string) *Client {
	return &Client{
		AtlasToken: atlasToken,
		BaseURL:    baseURL,
	}
}

// ListWorkspaces lists all workspaces for a given organization
func (c *Client) ListWorkspaces(organization string) ([]Workspace, error) {
	path := fmt.Sprintf("/api/v2/organizations/%s/workspaces", organization)

	type wrapper struct {
		Data []Workspace `json:"data"`
	}

	var resp wrapper
	if err := c.do("GET", path, nil, nil, &resp); err != nil {
		if err == ErrNotFound {
			return []Workspace{}, ErrWorkspaceNotFound
		}
		return []Workspace{}, err
	}

	return resp.Data, nil
}

// GetWorkspace gets a specific workspace
func (c *Client) GetWorkspace(organization, workspace string) (Workspace, error) {
	path := fmt.Sprintf("/api/v2/organizations/%s/workspaces/%s", organization, workspace)

	type wrapper struct {
		Data Workspace `json:"data"`
	}

	var resp wrapper
	if err := c.do("GET", path, nil, nil, &resp); err != nil {
		if err == ErrNotFound {
			return Workspace{}, ErrWorkspaceNotFound
		}
		return Workspace{}, err
	}

	return resp.Data, nil
}

// ListStateVersions lists all state versions for a given workspace
func (c *Client) ListStateVersions(organization, workspace string) ([]StateVersion, error) {
	q := url.Values{}
	q.Add("filter[organization][name]", organization)
	q.Add("filter[workspace][name]", workspace)

	path := "/api/v2/state-versions"

	type wrapper struct {
		Data []StateVersion `json:"data"`
	}

	var resp wrapper
	if err := c.do("GET", path, nil, q, &resp); err != nil {
		if err == ErrNotFound {
			return []StateVersion{}, ErrStateVersionNotFound
		}
		return []StateVersion{}, err
	}

	return resp.Data, nil
}

// GetStateVersion gets a specific state version
func (c *Client) GetStateVersion(organization, workspace, stateVersion string) (StateVersion, error) {
	path := fmt.Sprintf("/api/v2/state-versions/%s", stateVersion)

	type wrapper struct {
		Data StateVersion `json:"data"`
	}

	var resp wrapper
	if err := c.do("GET", path, nil, nil, &resp); err != nil {
		if err == ErrNotFound {
			return StateVersion{}, ErrStateVersionNotFound
		}
		return StateVersion{}, err
	}

	return resp.Data, nil
}

// DownloadState downloads the raw state file from Terraform Enterprise
func (c *Client) DownloadState(organization, workspace, stateVersion string) ([]byte, error) {
	sv, err := c.GetStateVersion(organization, workspace, stateVersion)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(sv.Attributes.HostedStateDownloadURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := ioutil.ReadAll(resp.Body)
	return raw, err
}

func (c *Client) do(method string, path string, body io.Reader, query url.Values, recv interface{}) error {
	parsed, err := url.Parse(c.BaseURL)
	if err != nil {
		return err
	}

	parsed.Path = path
	if query == nil {
		query = url.Values{}
	}
	parsed.RawQuery = query.Encode()

	fmt.Printf("Requesting %s\n", parsed.String())
	req, err := http.NewRequest(method, parsed.String(), body)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.AtlasToken))
	req.Header.Add("Content-Type", "application/vnd.api+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == 401:
		return ErrUnauthorized
	case resp.StatusCode == 404:
		return ErrNotFound
	case resp.StatusCode != 200:
		return ErrBadStatus
	}

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&recv)
	return err
}