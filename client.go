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

// HTTPClient looks like net/http.Client, without the convenience methods
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type retryHTTPClient struct {
	// http.Client used for requests
	Client *http.Client

	// Retries happen every Interval * 2 ** attempt_num
	Interval time.Duration

	// Give up after this many attempts
	MaxAttempts int

	// ShouldRetry() -> true means the request will be retried
	// if nil, uses defaultShouldRetry
	ShouldRetry func(*http.Response, error) bool
}

func (c *retryHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		// otherwise we'd have to buffer Body ourselves
		// no requests in Client currently use Body
		panic("Retries for requests with Body unsupported")
	}
	var shouldRetry func(*http.Response, error) bool
	if c.ShouldRetry == nil {
		shouldRetry = defaultShouldRetry
	} else {
		shouldRetry = c.ShouldRetry
	}
	var err error
	var resp *http.Response
	for i := 0; i < c.MaxAttempts; i++ {
		resp, err = c.Client.Do(req)
		if !shouldRetry(resp, err) {
			return resp, err
		}
		time.Sleep(c.Interval * time.Duration(math.Pow(2, float64(i))))
	}
	return resp, err
}

func defaultShouldRetry(resp *http.Response, e error) bool {
	if resp.StatusCode != 200 {
		return true
	}
	if e, ok := e.(net.Error); ok && e.Timeout() {
		// Retry timeouts
		return true
	}
	return false
}

// Client exposes an API for communicating with Terraform Enterprise
type Client struct {
	// AtlasToken is the token used to authenticate with Terraform Enterprise,
	// you can generate one from the Terraform Enterprise UI
	AtlasToken string

	// BaseURL is the base used for all api calls.  If you are using
	// Terraform Enterprise SaaS, you can set this to DefaultBaseURL
	BaseURL string

	client HTTPClient
}

// New creates and returns a new Terraform Enterprise client
func New(atlasToken string, baseURL string) *Client {
	return NewWithHTTPClient(
		atlasToken,
		baseURL,
		&retryHTTPClient{
			Client: &http.Client{
				Timeout: time.Second * 10,
			},
			Interval:    500 * time.Millisecond,
			MaxAttempts: 10,
		},
	)
}

// NewWithClient creates and returns a new Terraform Enterprise client, like New,
// but with a custom http.Client
func NewWithClient(atlasToken string, baseURL string, client *http.Client) *Client {
	return &Client{
		AtlasToken: atlasToken,
		BaseURL:    baseURL,
		client:     client,
	}
}

// NewWithClient creates and returns a new Terraform Enterprise client, like New,
// but with a custom HTTPClient
func NewWithHTTPClient(atlasToken string, baseURL string, client HTTPClient) *Client {
	return &Client{
		AtlasToken: atlasToken,
		BaseURL:    baseURL,
		client:     client,
	}
}

// ListOrganizations lists all organizations your token can access
func (c *Client) ListOrganizations() ([]Organization, error) {
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
		q := url.Values{}
		q.Add("page[number]", strconv.Itoa(resp.Meta.Pagination.CurrentPage+1))
		if err := c.do("GET", path, nil, nil, &resp); err != nil {
			return []Organization{}, err
		}
		orgs = append(orgs, resp.Data...)
	}
	return orgs, nil
}

// ListWorkspaces lists all workspaces for a given organization
func (c *Client) ListWorkspaces(organization string) ([]Workspace, error) {
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
		q := url.Values{}
		q.Add("page[number]", strconv.Itoa(resp.Meta.Pagination.CurrentPage+1))
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
		q = url.Values{}
		q.Add("filter[organization][name]", organization)
		q.Add("filter[workspace][name]", workspace)
		q.Add("page[number]", strconv.Itoa(resp.Meta.Pagination.CurrentPage+1))
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

	// the HostedStateDownloadURL is an unauthenticated "secret" URL
	req, err := http.NewRequest("GET", sv.Attributes.HostedStateDownloadURL, nil)
	if err != nil {
		return nil, err
	}
	// we avoid c.do because we don't want to unmarshal the body
	resp, err := c.client.Do(req)
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
}
