package tfe

import "time"

// Organization is a Terraform Enterprise organization
type Organization struct {
	ID            string        `json:"id"`
	Links         Links         `json:"links"`
	Relationships Relationships `json:"relationships"`
}

// Workspace is a Terraform Enterprise workspace
type Workspace struct {
	ID            string              `json:"id"`
	Type          string              `json:"type"`
	Attributes    WorkspaceAttributes `json:"attributes"`
	Relationships Relationships       `json:"relationships"`
	Links         Links               `json:"links"`
}

type WorkspaceAttributes struct {
	Name             string          `json:"name"`
	Environment      string          `json:"environment"`
	AutoApply        bool            `json:"auto-apply"`
	Locked           bool            `json:"locked"`
	CreatedAt        time.Time       `json:"created-at"`
	WorkingDirectory string          `json:"working-directory"`
	TerraformVersion string          `json:"terraform-version"`
	VCSRepo          VCSRepo         `json:"vcs-repo"`
	Permissions      map[string]bool `json:"permissions"`
	Actions          map[string]bool `json:"actions"`
}

type Variable struct {
	ID            string             `json:"id"`
	Type          string             `json:"type"`
	Attributes    VariableAttributes `json:"attributes"`
	Relationships Relationships      `json:"relationships"`
	Links         Links              `json:"links"`
}

type VariableAttributes struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Category  string `json:"category"`
	HCL       bool   `json:"hcl"`
	Sensitive bool   `json:"sensitive"`
}

type VCSRepo struct {
	Branch            string `json:"branch"`
	IngressSubmodules bool   `json:"ingress-submodules"`
	Identifier        string `json:"identifier"`
	OauthTokenID      string `json:"oauth-token-id"`
}

type Relationships map[string]Relationship

type Relationship struct {
	Data  RelationshipData `json:"data"`
	Links Links            `json:"links"`
}

type RelationshipData struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type Links map[string]Link

type Link string

// StateVersion represents a single state version from Terraform Enterprise
type StateVersion struct {
	ID            string                 `json:"id"`
	Attributes    StateVersionAttributes `json:"attributes"`
	Links         Links                  `json:"links"`
	Relationships Relationships          `json:"relationships"`
}

type StateVersionAttributes struct {
	CreatedAt              time.Time `json:"created-at"`
	HostedStateDownloadURL string    `json:"hosted-state-download-url"`
	Serial                 int       `json:"serial"`
}

type CreateWorkspaceOptions struct {
	Name             string `validate:"required"`
	TerraformVersion string
	VCSIdentifier    string
	VCSOauthKeyID    string
}

type CreateVariableOptions struct {
	Key       string `validate:"required"`
	Value     string `validate:"required"`
	Category  string `validate:"required"`
	Sensitive bool
	HCL       bool
}

type AssignSSHKeyPayload struct {
	Type string           `json:"type"`
	Data SSHKeyAttributes `json:"data"`
}

type SSHKeyAttributes struct {
	ID string `json:"id"`
}
