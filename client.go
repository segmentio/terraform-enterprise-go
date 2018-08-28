package tfe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	// DefaultBaseURL is the default base url to reach Terraform Enterprise
	DefaultBaseURL  = "https://app.terraform.io"
	DefaultPageSize = 150
)

// Error Types
var (
	ErrUnauthorized         = errors.New("User is not authorized to perform this action")
	ErrNotFound             = errors.New("Not found")
	ErrWorkspaceNotFound    = errors.New("Workspace not found")
	ErrStateVersionNotFound = errors.New("State version not found")
	ErrBadStatus            = errors.New("Unrecognized status code")
)

type PaginatedResponse struct {
	Meta MetaInfo `json:"meta"`
}

type MetaInfo struct {
	Pagination PaginationInfo `json:"pagination"`
}

type PaginationInfo struct {
	CurrentPage int `json:"current-page"`
	NextPage    int `json:"next-page"`
	TotalPages  int `json:"total-pages"`
}

// Client exposes an API for communicating with Terraform Enterprise
type Client struct {
	// AtlasToken is the token used to authenticate with Terraform Enterprise,
	// you can generate one from the Terraform Enterprise UI
	AtlasToken string

	// BaseURL is the base used for all api calls.  If you are using
	// Terraform Enterprise SaaS, you can set this to DefaultBaseURL
	BaseURL string

	client *http.Client
}

// New creates and returns a new Terraform Enterprise client
func New(atlasToken string, baseURL string) *Client {
	return &Client{
		AtlasToken: atlasToken,
		BaseURL:    baseURL,
		client: &http.Client{
			Timeout: time.Second * 10,
		},
	}
}

// ListOrganizations lists all organizations your token can access
func (c *Client) ListOrganizations() ([]Organization, error) {
	q := url.Values{}
	q.Add("page[size]", DefaultPageSize)

	path := "/api/v2/organizations"
	orgs := []Organization{}

	type wrapper struct {
		PaginatedResponse
		Data []Organization `json:"data"`
	}

	var resp wrapper
	if err := c.do("GET", path, nil, nil, &resp); err != nil {
		return []Organization{}, err
	}
	orgs = append(orgs, resp.Data...)

	for resp.Meta.Pagination.CurrentPage < resp.Meta.Pagination.TotalPages {
		q.Set("page[number]", strconv.Itoa(resp.Meta.Pagination.CurrentPage+1))
		if err := c.do("GET", path, nil, nil, &resp); err != nil {
			return []Organization{}, err
		}
		orgs = append(orgs, resp.Data...)
	}
	return orgs, nil
}

// ListWorkspaces lists all workspaces for a given organization
func (c *Client) ListWorkspaces(organization string) ([]Workspace, error) {
	q := url.Values{}
	q.Add("page[size]", DefaultPageSize)

	path := fmt.Sprintf("/api/v2/organizations/%s/workspaces", organization)
	workspaces := []Workspace{}

	type wrapper struct {
		PaginatedResponse
		Data []Workspace `json:"data"`
	}

	var resp wrapper
	if err := c.do("GET", path, nil, nil, &resp); err != nil {
		if err == ErrNotFound {
			return []Workspace{}, ErrWorkspaceNotFound
		}
		return []Workspace{}, err
	}
	workspaces = append(workspaces, resp.Data...)

	for resp.Meta.Pagination.CurrentPage < resp.Meta.Pagination.TotalPages {
		q.Set("page[number]", strconv.Itoa(resp.Meta.Pagination.CurrentPage+1))

		if err := c.do("GET", path, nil, q, &resp); err != nil {
			if err == ErrNotFound {
				return []Workspace{}, ErrWorkspaceNotFound
			}
			return []Workspace{}, err
		}
		workspaces = append(workspaces, resp.Data...)
	}
	return workspaces, nil
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
	q.Add("page[size]", DefaultPageSize)

	svs := []StateVersion{}

	path := "/api/v2/state-versions"

	type wrapper struct {
		PaginatedResponse
		Data []StateVersion `json:"data"`
	}

	var resp wrapper
	if err := c.do("GET", path, nil, q, &resp); err != nil {
		if err == ErrNotFound {
			return []StateVersion{}, ErrStateVersionNotFound
		}
		return []StateVersion{}, err
	}
	svs = append(svs, resp.Data...)

	for resp.Meta.Pagination.CurrentPage < resp.Meta.Pagination.TotalPages {
		q.Set("page[number]", strconv.Itoa(resp.Meta.Pagination.CurrentPage+1))

		if err := c.do("GET", path, nil, q, &resp); err != nil {
			if err == ErrNotFound {
				return []StateVersion{}, ErrStateVersionNotFound
			}
			return []StateVersion{}, err
		}
		svs = append(svs, resp.Data...)
	}
	return svs, nil
}

// GetLatestStateVersion gets the latest state version for a given
// workspace
func (c *Client) GetLatestStateVersion(organization, workspace string) (StateVersion, error) {
	q := url.Values{}
	q.Add("filter[organization][name]", organization)
	q.Add("filter[workspace][name]", workspace)
	q.Add("page[size]", 1)

	path := "/api/v2/state-versions"

	type wrapper struct {
		Data []StateVersion `json:"data"`
	}

	var resp wrapper
	if err := c.do("GET", path, nil, q, &resp); err != nil {
		if err == ErrNotFound {
			return StateVersion{}, ErrStateVersionNotFound
		}
		return StateVersion{}, err
	}

	if len(resp.Data) < 1 {
		return StateVersion{}, ErrStateVersionNotFound
	}

	return resp.Data[0], nil
}

// GetStateVersion gets a specific state version
func (c *Client) GetStateVersion(organization, workspace, stateVersion string) (StateVersion, error) {
	q := url.Values()
	q.Add("page[size]", DefaultPageSize)

	path := fmt.Sprintf("/api/v2/state-versions/%s", stateVersion)

	type wrapper struct {
		Data StateVersion `json:"data"`
	}

	var resp wrapper
	if err := c.do("GET", path, nil, q, &resp); err != nil {
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

	var resp *http.Response
	err = withRetries(
		func() error {
			var err error
			resp, err = c.client.Get(sv.Attributes.HostedStateDownloadURL)
			if err != nil {
				return err
			}

			if resp.StatusCode != 200 {
				return ErrBadStatus
			}
			return nil
		},
		func(e error) bool {
			if e == ErrBadStatus {
				return true
			}
			if e, ok := e.(net.Error); ok && e.Timeout() {
				// Retry timeouts
				return true
			}
			return false
		},
		10,
	)
	if err != nil {
		return nil, err
	}

	raw, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
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

	return withRetries(
		func() error {
			req, err := http.NewRequest(method, parsed.String(), body)
			if err != nil {
				return err
			}

			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.AtlasToken))
			req.Header.Add("Content-Type", "application/vnd.api+json")

			resp, err := c.client.Do(req)
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
		},
		func(e error) bool {
			if e == ErrBadStatus {
				return true
			}
			if e, ok := e.(net.Error); ok && e.Timeout() {
				// Retry timeouts
				return true
			}
			return false
		},
		10,
	)
}

func withRetries(f func() error, shouldRetry func(e error) bool, attempts int) error {
	interval := 500 * time.Millisecond
	var err error
	for i := 0; i < attempts; i++ {
		err = f()
		if !shouldRetry(err) {
			return err
		}
		time.Sleep(interval * time.Duration(math.Pow(2, float64(i))))
	}
	return err
}
