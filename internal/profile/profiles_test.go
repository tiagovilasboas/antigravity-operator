package profile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
)

func setupMockUserDataDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()

	localStateJSON := `{
  "profile": {
    "last_used": "Default",
    "last_active_profiles": ["Default", "Profile 2"],
    "info_cache": {
      "Default": {
        "name": "Dev Personal",
        "user_name": "dev@gmail.com",
        "gaia_name": "Dev User",
        "hosted_domain": "NO_HOSTED_DOMAIN",
        "active_time": 1700000100.0
      },
      "Profile 1": {
        "name": "corp.example.com",
        "user_name": "",
        "gaia_name": "",
        "hosted_domain": "",
        "active_time": 1700000200.0
      },
      "Profile 2": {
        "name": "corp.example.com",
        "user_name": "dev@corp.example.com",
        "gaia_name": "Dev Corp",
        "hosted_domain": "corp.example.com",
        "active_time": 1700000300.0
      }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(tmp, "Local State"), []byte(localStateJSON), 0600); err != nil {
		t.Fatalf("failed to write Local State: %v", err)
	}

	for _, dir := range []string{"Default", "Profile 1", "Profile 2"} {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0700); err != nil {
			t.Fatalf("failed to mkdir %s: %v", dir, err)
		}
	}

	defaultPrefs := `{"profile":{"name":"Dev Personal"},"account_info":[{"email":"dev@gmail.com","full_name":"Dev User","hosted_domain":"NO_HOSTED_DOMAIN"}]}`
	ghostPrefs := `{"profile":{"name":"corp.example.com"},"account_info":[]}`
	corpPrefs := `{"profile":{"name":"corp.example.com"},"account_info":[{"email":"dev@corp.example.com","full_name":"Dev Corp","hosted_domain":"corp.example.com"}]}`

	_ = os.WriteFile(filepath.Join(tmp, "Default", "Preferences"), []byte(defaultPrefs), 0600)
	_ = os.WriteFile(filepath.Join(tmp, "Profile 1", "Preferences"), []byte(ghostPrefs), 0600)
	_ = os.WriteFile(filepath.Join(tmp, "Profile 2", "Preferences"), []byte(corpPrefs), 0600)

	return tmp
}

func TestDiscoverProfilesAndGhostClassification(t *testing.T) {
	dir := setupMockUserDataDir(t)

	profiles, err := profile.DiscoverProfiles(dir, "isolated")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(profiles))
	}

	// Ghost profile must be sorted last
	if profiles[2].Directory != "Profile 1" || profiles[2].Kind != profile.ProfileKindGhost {
		t.Errorf("expected Profile 1 to be classified as ghost and sorted last, got %+v", profiles[2])
	}

	// ResolveProfile with ambiguous name "corp.example.com" MUST pick Profile 2 (authenticated) over Profile 1 (ghost)
	resolved, err := profile.ResolveProfile(dir, "corp.example.com")
	if err != nil {
		t.Fatalf("unexpected error resolving corp.example.com: %v", err)
	}
	if resolved.Directory != "Profile 2" {
		t.Errorf("expected ResolveProfile to pick authenticated Profile 2 over ghost Profile 1, got %s", resolved.Directory)
	}

	// ResolveProfile by kind "workspace"
	wsProf, err := profile.ResolveProfile(dir, "workspace")
	if err != nil || wsProf.Directory != "Profile 2" {
		t.Errorf("expected workspace selector to resolve Profile 2, got %+v (err=%v)", wsProf, err)
	}

	// ResolveProfile by kind "personal"
	persProf, err := profile.ResolveProfile(dir, "personal")
	if err != nil || persProf.Directory != "Default" {
		t.Errorf("expected personal selector to resolve Default, got %+v (err=%v)", persProf, err)
	}
}

func TestResolveURLForProfilesAgnostic(t *testing.T) {
	dir := setupMockUserDataDir(t)
	profiles, err := profile.DiscoverProfiles(dir, "isolated")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Direct subdomain match against HostedDomain ("portal.corp.example.com")
	res1, err := profile.ResolveURLForProfiles(profiles, "https://portal.corp.example.com/dashboard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res1.Recommended == nil || res1.Recommended.Directory != "Profile 2" || res1.MatchedBy != "domain" {
		t.Errorf("expected domain match to Profile 2, got %+v", res1)
	}

	// 2. Shared Google Forms URL when user has both Personal and Workspace profiles
	res2, err := profile.ResolveURLForProfiles(profiles, "https://docs.google.com/forms/d/e/1FAIpQLSxyz/viewform")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res2.Recommended == nil || res2.Recommended.Directory != "Profile 2" {
		t.Errorf("expected shared Google Forms to recommend Workspace Profile 2, got %+v", res2)
	}
	if !strings.Contains(res2.SuggestedURL, "authuser=dev%40corp.example.com") {
		t.Errorf("expected SuggestedURL to include authuser=dev%%40corp.example.com, got %s", res2.SuggestedURL)
	}

	// 3. Google Classroom URL with /u/3/ path when Workspace account lives in its own Chrome Profile
	res3, err := profile.ResolveURLForProfiles(profiles, "https://classroom.google.com/u/3/c/12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res3.Recommended == nil || res3.Recommended.Directory != "Profile 2" {
		t.Errorf("expected /u/3/ classroom URL to route to Workspace Profile 2, got %+v", res3)
	}

	// 4. Generic external URL falls back to Default active profile
	res4, err := profile.ResolveURLForProfiles(profiles, "https://github.com/org/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res4.Recommended == nil || res4.Recommended.Directory != "Default" {
		t.Errorf("expected generic URL to recommend Default profile, got %+v", res4)
	}
}

func TestDetectAuthWallAndPruneGhosts(t *testing.T) {
	if wall, _ := profile.DetectAuthWall("Acesso negado", "https://docs.google.com/forms/d/e/123/viewform"); !wall {
		t.Errorf("expected Acesso negado to be detected as auth wall")
	}
	if wall, _ := profile.DetectAuthWall("Privacy error", "https://vpn.corp.com/remote/saml/start?redirect=1"); !wall {
		t.Errorf("expected SAML redirect to be detected as auth wall")
	}
	if wall, _ := profile.DetectAuthWall("Dashboard", "https://portal.corp.example.com/home"); wall {
		t.Errorf("did not expect normal dashboard page to trigger auth wall")
	}

	dir := setupMockUserDataDir(t)
	pruned, err := profile.PruneIsolatedGhostProfiles(dir)
	if err != nil {
		t.Fatalf("unexpected error pruning ghosts: %v", err)
	}
	if len(pruned) != 1 || pruned[0] != "Profile 1" {
		t.Fatalf("expected Profile 1 ghost to be pruned, got %v", pruned)
	}

	remaining, err := profile.DiscoverProfiles(dir, "isolated")
	if err != nil {
		t.Fatalf("unexpected error after prune: %v", err)
	}
	if len(remaining) != 2 {
		t.Errorf("expected 2 authenticated profiles remaining after pruning ghost, got %d", len(remaining))
	}
}
