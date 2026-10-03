package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/antigravity-operator/internal/installer"
	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
)

func TestRun(t *testing.T) {
	info := &platform.Info{
		OS:             "darwin",
		Arch:           "arm64",
		HomeDir:        t.TempDir(),
		HasDisplay:     true,
		ChromeBin:      "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		GeminiDir:      filepath.Join(t.TempDir(), ".gemini", "antigravity"),
		BrowserProfile: filepath.Join(t.TempDir(), ".gemini", "antigravity-browser-profile"),
		HarnessCore:    "",
	}

	report := Run(info)
	if report == nil {
		t.Fatal("expected report to not be nil")
	}

	if report.Platform != info {
		t.Errorf("expected platform %v, got %v", info, report.Platform)
	}

	if len(report.Checks) < 5 {
		t.Fatalf("expected at least 5 checks, got %d", len(report.Checks))
	}

	for _, check := range report.Checks {
		if check.Name == "" {
			t.Error("check name should not be empty")
		}
		if check.Status != "OK" && check.Status != "WARN" && check.Status != "FAIL" && check.Status != "INFO" {
			t.Errorf("unexpected check status: %s for check %s", check.Status, check.Name)
		}
	}
}

func TestCheckChrome(t *testing.T) {
	// 1. Missing Chrome
	noChromeInfo := &platform.Info{ChromeBin: ""}
	res := checkChrome(noChromeInfo)
	if res.Status != "FAIL" {
		t.Errorf("expected FAIL for missing chrome, got %s", res.Status)
	}

	// 2. Found Chrome
	chromeInfo := &platform.Info{ChromeBin: "/usr/bin/google-chrome"}
	resOK := checkChrome(chromeInfo)
	if resOK.Status != "OK" {
		t.Errorf("expected OK for existing chrome path, got %s", resOK.Status)
	}
}

func TestCheckHarnessCore(t *testing.T) {
	// 1. Empty HarnessCore (Standalone)
	emptyInfo := &platform.Info{HarnessCore: ""}
	res := checkHarnessCore(emptyInfo)
	if res.Status != "INFO" {
		t.Errorf("expected INFO for empty harness core, got %s", res.Status)
	}

	// 2. Non-existent path
	fakeInfo := &platform.Info{HarnessCore: filepath.Join(t.TempDir(), "nonexistent")}
	resFake := checkHarnessCore(fakeInfo)
	if resFake.Status != "INFO" {
		t.Errorf("expected INFO for non-repo harness core, got %s", resFake.Status)
	}

	// 3. Valid git repo
	gitDir := t.TempDir()
	dotGit := filepath.Join(gitDir, ".git")
	_ = os.MkdirAll(dotGit, 0755)
	validInfo := &platform.Info{HarnessCore: gitDir}
	_ = checkHarnessCore(validInfo) // won't fail
}

func TestCheckGit(t *testing.T) {
	check := checkGit()
	if check.Name != "Git" && check.Name != "Git Identity" {
		t.Errorf("unexpected check name: %s", check.Name)
	}
}

func TestCheckNode(t *testing.T) {
	check := checkNode()
	if check.Name != "NPX (MCP Runtime)" {
		t.Errorf("unexpected check name: %s", check.Name)
	}
	if check.Status != "OK" && check.Status != "WARN" {
		t.Errorf("unexpected check status: %s", check.Status)
	}
}

func TestCheckDevTools(t *testing.T) {
	tmpDir := t.TempDir()
	info := &platform.Info{BrowserProfile: tmpDir}
	check := checkDevTools(info)
	if check.Name != "Chrome DevTools (Port 9222)" {
		t.Errorf("unexpected check name: %s", check.Name)
	}
	// On a random temp dir, DevTools won't be running on port 9222 unless active
	if check.Status != "OK" && check.Status != "WARN" {
		t.Errorf("unexpected check status: %s", check.Status)
	}
}

func TestCheckAntigravity(t *testing.T) {
	info := &platform.Info{OS: "darwin"}
	check := checkAntigravity(info)
	if check.Name != "Google Antigravity" {
		t.Errorf("unexpected check name: %s", check.Name)
	}
	if check.Status != "OK" && check.Status != "INFO" {
		t.Errorf("unexpected status: %s", check.Status)
	}

	infoLinux := &platform.Info{OS: "linux"}
	checkLinux := checkAntigravity(infoLinux)
	if checkLinux.Name != "Google Antigravity" {
		t.Errorf("unexpected check name: %s", checkLinux.Name)
	}
}

func TestCheckAPIKeys(t *testing.T) {
	// Test without keys
	_ = os.Unsetenv("GEMINI_API_KEY")
	_ = os.Unsetenv("GOOGLE_API_KEY")
	resEmpty := checkAPIKeys()
	if resEmpty.Status != "INFO" {
		t.Errorf("expected INFO for empty keys, got %s", resEmpty.Status)
	}

	// Test with GEMINI_API_KEY
	_ = os.Setenv("GEMINI_API_KEY", "AIzaSyTestKey123456789")
	defer os.Unsetenv("GEMINI_API_KEY")
	resSet := checkAPIKeys()
	if resSet.Status != "OK" {
		t.Errorf("expected OK for set key, got %s", resSet.Status)
	}
}

func TestCheckSessionMemory(t *testing.T) {
	// 1. Diretório sem sessão
	emptyDir := t.TempDir()
	resEmpty := checkSessionMemory(emptyDir)
	if resEmpty.Status != "INFO" {
		t.Errorf("esperava INFO para diretório sem sessão, obteve %s", resEmpty.Status)
	}

	// 2. Diretório com sessão saudável
	sessionDir := t.TempDir()
	dotAgents := filepath.Join(sessionDir, ".agents", "session")
	_ = os.MkdirAll(dotAgents, 0755)
	_ = os.WriteFile(filepath.Join(dotAgents, "state.md"), []byte("# Estado\n## Objetivo Atual\n- Teste\n"), 0644)
	_ = os.WriteFile(filepath.Join(dotAgents, "todo.md"), []byte("# Tarefas\n- [x] Tarefa 1\n"), 0644)

	resOK := checkSessionMemory(sessionDir)
	if resOK.Status != "OK" {
		t.Errorf("esperava OK para sessão saudável, obteve %s", resOK.Status)
	}

	// 3. Diretório com inchaço de tarefas (> 15 tarefas concluídas)
	bloatedDir := t.TempDir()
	bloatedAgents := filepath.Join(bloatedDir, ".agents", "session")
	_ = os.MkdirAll(bloatedAgents, 0755)
	_ = os.WriteFile(filepath.Join(bloatedAgents, "state.md"), []byte("# Estado\n## Objetivo Atual\n- Bloat Test\n"), 0644)
	var bloatedTasks []string
	bloatedTasks = append(bloatedTasks, "# Tarefas")
	for i := 1; i <= 20; i++ {
		bloatedTasks = append(bloatedTasks, "- [x] Tarefa acumulada sem compactação")
	}
	_ = os.WriteFile(filepath.Join(bloatedAgents, "todo.md"), []byte(strings.Join(bloatedTasks, "\n")), 0644)

	resWarn := checkSessionMemory(bloatedDir)
	if resWarn.Status != "WARN" {
		t.Errorf("esperava WARN para sessão com inchaço de tarefas, obteve %s", resWarn.Status)
	}
}

func TestFix(t *testing.T) {
	tempDir := t.TempDir()
	info := &platform.Info{}

	// 1. Executa Fix em diretório vazio (deve criar templates)
	res, err := Fix(tempDir, info)
	if err != nil {
		t.Fatalf("Fix falhou: %v", err)
	}

	if len(res.Repaired) == 0 {
		t.Errorf("esperava arquivos reparados/criados")
	}

	// 2. Executa Fix novamente (idempotente: deve apenas dar skip)
	res2, err := Fix(tempDir, info)
	if err != nil {
		t.Fatalf("segundo Fix falhou: %v", err)
	}

	if len(res2.Skipped) == 0 {
		t.Errorf("esperava arquivos identificados como intactos")
	}
}

func TestRun_WarnsOnOutdatedMCPManifest(t *testing.T) {
	info := &platform.Info{OS: "linux", GeminiDir: t.TempDir()}
	mcp := filepath.Join(info.GeminiDir, "mcp")
	_ = os.MkdirAll(mcp, 0o755)
	legacy := `{"mcpServers":{"chrome-devtools":{"command":"npx","args":["-y","@modelcontextprotocol/server-chrome-devtools"]}}}`
	_ = os.WriteFile(filepath.Join(mcp, "default-servers.json"), []byte(legacy), 0o644)

	var got *CheckItem
	for _, c := range Run(info).Checks {
		if c.Name == "MCP Manifest" {
			c := c
			got = &c
		}
	}
	if got == nil {
		t.Fatal("doctor has no MCP Manifest check")
	}
	if got.Status != "WARN" || !strings.Contains(got.Details, "@modelcontextprotocol/server-chrome-devtools") || !strings.Contains(got.Details, "agyo sync --update-mcp") {
		t.Fatalf("want WARN naming the unpinned package and the fix, got %+v", got)
	}
}

func TestCheckMCPManifest_States(t *testing.T) {
	info := &platform.Info{GeminiDir: t.TempDir()}
	if c := checkMCPManifest(info); c.Status != "INFO" {
		t.Fatalf("missing manifest: %+v", c)
	}
	if _, _, err := installer.UpdateMCP(info, time.Now()); err != nil {
		t.Fatal(err)
	}
	if c := checkMCPManifest(info); c.Status != "OK" {
		t.Fatalf("built-in manifest: %+v", c)
	}
	_ = os.WriteFile(installer.MCPPath(info), []byte(`{"mcpServers":{}}`), 0o644)
	if c := checkMCPManifest(info); c.Status != "WARN" || !strings.Contains(c.Details, "Differs") {
		t.Fatalf("custom manifest: %+v", c)
	}
}
