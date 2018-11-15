package tfe

// Includes live tests that really call TFE. These are disabled by default.
// They can be enabled with, e.g.
// go test -v ./... -args -enable-live -token <token> -org <org> -workspace <workspace>

import (
	"flag"
	"testing"
)

var testEnableLive = flag.Bool("enable-live", false, "enable tests that really call TFE (GETs only)")
var allowWrites = flag.Bool("allow-writes", false, "enable tests to write")
var testOrg = flag.String("org", "", "organization name")
var testWorkspace = flag.String("workspace", "", "workspace name")
var testAuthToken = flag.String("token", "", "auth token")
var sshKeyID = flag.String("ssh-key-id", "", "ssh key id")
var oauthKeyID = flag.String("oauth-key-id", "", "oauth key id")

func liveEnabled() bool {
	return !testing.Short() && *testEnableLive && *testAuthToken != ""
}

func writesEnabled() bool {
	return !testing.Short() && *allowWrites
}

func TestGetLatestStateVersion(t *testing.T) {
	if !liveEnabled() || *testOrg == "" || *testWorkspace == "" {
		t.Skip("missing -enable-live or -org or -workspace")
	}

	c := New(*testAuthToken, DefaultBaseURL)

	sv, err := c.GetLatestStateVersion(*testOrg, *testWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("got StateVersion: %#v", sv)
}

func TestCreateWorkspace(t *testing.T) {
	if !liveEnabled() || *testOrg == "" || !writesEnabled() {
		t.Skip("missing -enable-live or -org or -allow-writes or -oauth-key-id")
	}

	c := New(*testAuthToken, DefaultBaseURL)

	w, err := c.CreateWorkspace(*testOrg, CreateWorkspaceOptions{
		Name:             "test-workspace",
		TerraformVersion: "0.11.7",
		VCSIdentifier:    "segmentio/terracode-template",
		VCSOauthKeyID:    *oauthKeyID,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("got Workspace: %#v", w)
}

func TestCreateRun(t *testing.T) {
	if !liveEnabled() || *testWorkspace == "" || !writesEnabled() {
		t.Skip("missing -enable-live or -workspace or -allow-writes")
	}

	c := New(*testAuthToken, DefaultBaseURL)

	r, err := c.CreateRun(*testWorkspace)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("got Run: %#v", r)
}

func TestCreateVariable(t *testing.T) {
	if !liveEnabled() || *testWorkspace == "" || !writesEnabled() {
		t.Skip("missing -enable-live or -workspace -allow-writes")
	}

	c := New(*testAuthToken, DefaultBaseURL)

	v, err := c.CreateVariable(*testWorkspace, CreateVariableOptions{
		Key:       "foo",
		Value:     "bar",
		Category:  "env",
		Sensitive: true,
		HCL:       false,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("got Variable: %#v", v)
}

func TestAssignWorkspaceSSHKey(t *testing.T) {
	if !liveEnabled() || *testWorkspace == "" || !writesEnabled() {
		t.Skip("missing -enable-live or -workspace or -ssh-key-id or -allow-writes")
	}

	c := New(*testAuthToken, DefaultBaseURL)

	if err := c.AssignWorkspaceSSHKey(*testWorkspace, *sshKeyID); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadStateVersionLatest(t *testing.T) {
	if !liveEnabled() || *testOrg == "" || *testWorkspace == "" {
		t.Skip("missing -enable-live or -org or -workspace")
	}

	c := New(*testAuthToken, DefaultBaseURL)

	latest, err := c.GetLatestStateVersion(*testOrg, *testWorkspace)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.DownloadState(*testOrg, *testWorkspace, latest.ID)
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkDownloadStateVersionLatest(b *testing.B) {
	if !liveEnabled() || *testOrg == "" || *testWorkspace == "" {
		b.Skip("missing -enable-live or -org or -workspace")
	}

	if b.N > 10 {
		b.N = 10
	}

	c := New(*testAuthToken, DefaultBaseURL)

	for n := 0; n < b.N; n++ {
		latest, err := c.GetLatestStateVersion(*testOrg, *testWorkspace)
		if err != nil {
			b.Fatal(err)
		}

		_, err = c.DownloadState(*testOrg, *testWorkspace, latest.ID)
		if err != nil {
			b.Fatal(err)
		}
	}
}
