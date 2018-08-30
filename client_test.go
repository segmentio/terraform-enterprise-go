package tfe

// Includes live tests that really call TFE. These are disabled by default.
// They can be enabled with, e.g.
// go test -v ./... -args -enable-live -token <token> -org <org> -workspace <workspace>

import (
	"flag"
	"testing"
)

var testEnableLive = flag.Bool("enable-live", false, "enable tests that really call TFE (GETs only)")
var testOrg = flag.String("org", "", "organization name")
var testWorkspace = flag.String("workspace", "", "workspace name")
var testAuthToken = flag.String("token", "", "auth token")

func liveEnabled() bool {
	return !testing.Short() && *testEnableLive && *testAuthToken != ""
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
