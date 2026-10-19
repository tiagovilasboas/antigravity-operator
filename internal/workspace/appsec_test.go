package workspace

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestAppSec_BuildCalendarWebURL_StrictEscaping validates strict URL query escaping
// preventing query parameter injection, parameter pollution, and control char attacks (PLUG-02).
func TestAppSec_BuildCalendarWebURL_StrictEscaping(t *testing.T) {
	startTime := time.Date(2026, 10, 20, 14, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 10, 20, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		name        string
		event       CalendarEvent
		mustContain []string
		mustNotHave []string
	}{
		{
			name: "Parameter injection attempt in summary",
			event: CalendarEvent{
				Summary:     "Sprint Planning &action=DELETE&evil=1#fragment",
				Description: "Discuss backlog items",
				Location:    "Room 101",
				Start:       startTime,
				End:         endTime,
			},
			mustContain: []string{
				"action=TEMPLATE",
				"text=Sprint+Planning+%26action%3DDELETE%26evil%3D1%23fragment",
			},
			mustNotHave: []string{
				"&evil=1",
			},
		},
		{
			name: "CRLF injection attempt in description",
			event: CalendarEvent{
				Summary:     "Normal Meeting",
				Description: "Notes:\r\nInjected-Header: evil\r\nBcc: hacker@evil.com",
				Location:    "Online",
				Start:       startTime,
				End:         endTime,
			},
			mustContain: []string{
				"details=Notes%3A%0D%0AInjected-Header%3A+evil%0D%0ABcc%3A+hacker%40evil.com",
			},
			mustNotHave: []string{
				"\r",
				"\n",
			},
		},
		{
			name: "Special characters, unicode, and emojis",
			event: CalendarEvent{
				Summary:     "Reunião de Alinhamento 🚀 & Q&A",
				Description: "Pauta: 100% segurança / conformidade",
				Location:    "São Paulo - SP",
				Start:       startTime,
				End:         endTime,
			},
			mustContain: []string{
				"text=Reuni%C3%A3o+de+Alinhamento+%F0%9F%9A%80+%26+Q%26A",
			},
		},
		{
			name: "Zero times default to now and one hour later",
			event: CalendarEvent{
				Summary: "Ad-hoc Call",
			},
			mustContain: []string{
				"action=TEMPLATE",
				"text=Ad-hoc+Call",
				"dates=",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rawURL := BuildCalendarWebURL(tc.event)

			// 1. Verify URL is well-formed
			parsed, err := url.Parse(rawURL)
			if err != nil {
				t.Fatalf("BuildCalendarWebURL returned invalid URL %q: %v", rawURL, err)
			}

			// 2. Verify query parameter values match original input without injection
			q := parsed.Query()
			if gotAction := q.Get("action"); gotAction != "TEMPLATE" {
				t.Errorf("expected action=TEMPLATE, got %q", gotAction)
			}

			if gotText := q.Get("text"); gotText != tc.event.Summary {
				t.Errorf("text mismatch: expected %q, got %q", tc.event.Summary, gotText)
			}

			if tc.event.Description != "" {
				if gotDesc := q.Get("details"); gotDesc != tc.event.Description {
					t.Errorf("details mismatch: expected %q, got %q", tc.event.Description, gotDesc)
				}
			}

			if tc.event.Location != "" {
				if gotLoc := q.Get("location"); gotLoc != tc.event.Location {
					t.Errorf("location mismatch: expected %q, got %q", tc.event.Location, gotLoc)
				}
			}

			// 3. Verify no injected parameters exist in query map
			for _, mustNot := range tc.mustNotHave {
				if strings.Contains(rawURL, mustNot) {
					t.Errorf("URL contains prohibited substring %q: %s", mustNot, rawURL)
				}
			}

			for _, mustHave := range tc.mustContain {
				if !strings.Contains(rawURL, mustHave) {
					t.Errorf("URL missing expected substring %q: %s", mustHave, rawURL)
				}
			}
		})
	}
}

// TestAppSec_ICS_SanitizationAndCRLFPrevention tests sanitization of iCalendar fields (PLUG-03).
func TestAppSec_ICS_SanitizationAndCRLFPrevention(t *testing.T) {
	t.Run("SanitizeICSText strips control characters and escapes RFC 5545 reserved chars", func(t *testing.T) {
		input := "Title with \x00 null, \x1b escape, \x7f del, \r\n newline, ; semicolon, , comma, \\ backslash"
		sanitized := SanitizeICSText(input)

		// Must not contain raw \r or \n
		if strings.Contains(sanitized, "\r") {
			t.Errorf("sanitized text contains raw carriage return: %q", sanitized)
		}
		if strings.Contains(sanitized, "\n") {
			t.Errorf("sanitized text contains raw newline: %q", sanitized)
		}
		// Must not contain null byte or escape
		if strings.ContainsRune(sanitized, 0) || strings.ContainsRune(sanitized, 0x1b) || strings.ContainsRune(sanitized, 0x7f) {
			t.Errorf("sanitized text contains control characters: %q", sanitized)
		}
		// Must contain escaped versions
		if !strings.Contains(sanitized, `\n`) {
			t.Errorf("sanitized text should escape newline to \\n: %q", sanitized)
		}
		if !strings.Contains(sanitized, `\;`) {
			t.Errorf("sanitized text should escape semicolon to \\;: %q", sanitized)
		}
		if !strings.Contains(sanitized, `\,`) {
			t.Errorf("sanitized text should escape comma to \\,: %q", sanitized)
		}
		if !strings.Contains(sanitized, `\\`) {
			t.Errorf("sanitized text should escape backslash to \\\\: %q", sanitized)
		}
	})

	t.Run("ToICS prevents CRLF property injection attacks", func(t *testing.T) {
		event := CalendarEvent{
			Summary:     "Legit Event\r\nSUMMARY:Faked Injected Event\r\nATTENDEE:hacker@evil.com",
			Description: "Desc line 1\r\nTRIGGER:-PT15M\r\nATTACH:http://evil.com/malware.exe",
			Location:    "Room 404\r\nSTATUS:TENTATIVE",
			Start:       time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC),
			End:         time.Date(2026, 10, 25, 11, 0, 0, 0, time.UTC),
		}

		icsContent := event.ToICS()

		// Verify that all lines end with \r\n as per RFC 5545
		lines := strings.Split(icsContent, "\r\n")

		injectedSummaryCount := 0
		injectedAttendeeCount := 0
		injectedTriggerCount := 0
		injectedAttachCount := 0

		for _, line := range lines {
			if strings.HasPrefix(line, "SUMMARY:") {
				injectedSummaryCount++
			}
			if strings.HasPrefix(line, "ATTENDEE:") {
				injectedAttendeeCount++
			}
			if strings.HasPrefix(line, "TRIGGER:") {
				injectedTriggerCount++
			}
			if strings.HasPrefix(line, "ATTACH:") {
				injectedAttachCount++
			}
		}

		// Only ONE SUMMARY line must exist! The injected SUMMARY must NOT become a line
		if injectedSummaryCount != 1 {
			t.Fatalf("CRLF injection succeeded! Expected exactly 1 SUMMARY line, found %d in:\n%s", injectedSummaryCount, icsContent)
		}
		if injectedAttendeeCount != 0 {
			t.Fatalf("CRLF injection succeeded! Found unauthorized ATTENDEE line in:\n%s", icsContent)
		}
		if injectedTriggerCount != 0 {
			t.Fatalf("CRLF injection succeeded! Found unauthorized TRIGGER line in:\n%s", icsContent)
		}
		if injectedAttachCount != 0 {
			t.Fatalf("CRLF injection succeeded! Found unauthorized ATTACH line in:\n%s", icsContent)
		}

		// Verify essential standard tags are present
		if !strings.Contains(icsContent, "BEGIN:VCALENDAR") || !strings.Contains(icsContent, "END:VCALENDAR") {
			t.Fatalf("missing VCALENDAR envelope: %s", icsContent)
		}
		if !strings.Contains(icsContent, "BEGIN:VEVENT") || !strings.Contains(icsContent, "END:VEVENT") {
			t.Fatalf("missing VEVENT envelope: %s", icsContent)
		}
	})
}

// TestAppSec_ICS_PathTraversalProtection verifies protection against filesystem traversal in --ics (PLUG-03).
func TestAppSec_ICS_PathTraversalProtection(t *testing.T) {
	tempBase := t.TempDir()
	allowedDir := filepath.Join(tempBase, "authorized_workspace")
	if err := os.MkdirAll(allowedDir, 0700); err != nil {
		t.Fatalf("failed to create authorized dir: %v", err)
	}

	traversalPayloads := []struct {
		name        string
		path        string
		expectError bool
		errorSubstr string
	}{
		{
			name:        "Parent directory traversal",
			path:        filepath.Join(allowedDir, "../escaped.ics"),
			expectError: true,
			errorSubstr: "path traversal detectado",
		},
		{
			name:        "Deep parent directory traversal",
			path:        filepath.Join(allowedDir, "sub/../../../../etc/cron.d/job.ics"),
			expectError: true,
			errorSubstr: "path traversal detectado",
		},
		{
			name:        "Sensitive system path /etc",
			path:        "/etc/malicious.ics",
			expectError: true,
			errorSubstr: "path traversal detectado",
		},
		{
			name:        "Invalid extension .txt",
			path:        filepath.Join(allowedDir, "event.txt"),
			expectError: true,
			errorSubstr: "extensão inválida",
		},
		{
			name:        "Invalid extension .sh script overwrite attempt",
			path:        filepath.Join(allowedDir, "exploit.sh"),
			expectError: true,
			errorSubstr: "extensão inválida",
		},
		{
			name:        "Null byte in filename",
			path:        filepath.Join(allowedDir, "event\x00.ics"),
			expectError: true,
			errorSubstr: "caractere nulo",
		},
		{
			name:        "Empty path",
			path:        "   ",
			expectError: true,
			errorSubstr: "não pode ser vazio",
		},
		{
			name:        "Valid relative path in allowed directory",
			path:        filepath.Join(allowedDir, "valid_event.ics"),
			expectError: false,
		},
		{
			name:        "Valid nested path in allowed directory",
			path:        filepath.Join(allowedDir, "calendars/2026/event.ics"),
			expectError: false,
		},
	}

	for _, tc := range traversalPayloads {
		t.Run(tc.name, func(t *testing.T) {
			safePath, err := ValidateSecureICSPath(tc.path, allowedDir)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error for path %q, but got nil (resolved path: %s)", tc.path, safePath)
				}
				if !strings.Contains(err.Error(), tc.errorSubstr) {
					t.Fatalf("expected error containing %q, got: %v", tc.errorSubstr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for path %q: %v", tc.path, err)
				}
				rel, err := filepath.Rel(evalPathSymlinks(allowedDir), safePath)
				if err != nil || strings.HasPrefix(rel, "..") {
					t.Fatalf("safe path %q is not inside allowedDir %q (rel: %s)", safePath, allowedDir, rel)
				}
			}
		})
	}

	t.Run("WriteSecureICSFile enforces permissions and writes safely", func(t *testing.T) {
		targetFile := filepath.Join(allowedDir, "exports/meeting.ics")
		content := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n")

		if err := WriteSecureICSFile(targetFile, content, allowedDir); err != nil {
			t.Fatalf("failed to write secure ICS file: %v", err)
		}

		info, err := os.Stat(targetFile)
		if err != nil {
			t.Fatalf("failed to stat written file: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("expected file perm 0600, got %o", info.Mode().Perm())
		}

		dirInfo, err := os.Stat(filepath.Dir(targetFile))
		if err != nil {
			t.Fatalf("failed to stat target directory: %v", err)
		}
		if dirInfo.Mode().Perm() != 0700 {
			t.Errorf("expected dir perm 0700, got %o", dirInfo.Mode().Perm())
		}
	})
}

// TestAppSec_GoogleCredentials_DefensiveParsing validates defensive parser controls for credentials.json (PLUG-04).
func TestAppSec_GoogleCredentials_DefensiveParsing(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("Rejects file larger than 1 MB", func(t *testing.T) {
		largeFilePath := filepath.Join(tempDir, "large_credentials.json")
		// Write 1 MB + 1024 bytes
		largeData := make([]byte, 1024*1024+1024)
		for i := range largeData {
			largeData[i] = ' '
		}
		if err := os.WriteFile(largeFilePath, largeData, 0600); err != nil {
			t.Fatalf("failed to create large file: %v", err)
		}

		_, err := ParseGoogleCredentials(largeFilePath)
		if err == nil {
			t.Fatal("expected error parsing file > 1 MB, got nil")
		}
		if !strings.Contains(err.Error(), "excede o limite máximo permitido de 1 MB") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("Rejects empty file (0 bytes)", func(t *testing.T) {
		emptyPath := filepath.Join(tempDir, "empty_credentials.json")
		if err := os.WriteFile(emptyPath, []byte{}, 0600); err != nil {
			t.Fatalf("failed to create empty file: %v", err)
		}

		_, err := ParseGoogleCredentials(emptyPath)
		if err == nil {
			t.Fatal("expected error parsing empty file, got nil")
		}
		if !strings.Contains(err.Error(), "está vazio") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("Rejects non-json extensions", func(t *testing.T) {
		txtPath := filepath.Join(tempDir, "credentials.txt")
		if err := os.WriteFile(txtPath, []byte(`{"client_id":"abc"}`), 0600); err != nil {
			t.Fatalf("failed to create txt file: %v", err)
		}

		_, err := ParseGoogleCredentials(txtPath)
		if err == nil {
			t.Fatal("expected error for non-json extension, got nil")
		}
		if !strings.Contains(err.Error(), "deve possuir extensão .json") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("Rejects directory path passed as credentials file", func(t *testing.T) {
		dirPath := filepath.Join(tempDir, "fake_dir.json")
		if err := os.Mkdir(dirPath, 0700); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}

		_, err := ParseGoogleCredentials(dirPath)
		if err == nil {
			t.Fatal("expected error for directory path, got nil")
		}
		if !strings.Contains(err.Error(), "deve ser um arquivo regular") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("Rejects malformed JSON", func(t *testing.T) {
		corruptedPath := filepath.Join(tempDir, "corrupted.json")
		if err := os.WriteFile(corruptedPath, []byte(`{"installed": { broken json`), 0600); err != nil {
			t.Fatalf("failed to write corrupted json: %v", err)
		}

		_, err := ParseGoogleCredentials(corruptedPath)
		if err == nil {
			t.Fatal("expected error for malformed json, got nil")
		}
		if !strings.Contains(err.Error(), "estrutura JSON de credenciais inválida") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("Rejects missing client_id or client_secret", func(t *testing.T) {
		missingSecret := filepath.Join(tempDir, "no_secret.json")
		if err := os.WriteFile(missingSecret, []byte(`{"installed":{"client_id":"my-id"}}`), 0600); err != nil {
			t.Fatalf("failed to write no secret json: %v", err)
		}
		_, err := ParseGoogleCredentials(missingSecret)
		if err == nil {
			t.Fatal("expected error for missing client_secret, got nil")
		}

		missingID := filepath.Join(tempDir, "no_id.json")
		if err := os.WriteFile(missingID, []byte(`{"installed":{"client_secret":"super-secret"}}`), 0600); err != nil {
			t.Fatalf("failed to write no id json: %v", err)
		}
		_, err = ParseGoogleCredentials(missingID)
		if err == nil {
			t.Fatal("expected error for missing client_id, got nil")
		}
	})

	t.Run("Parses valid installed application format successfully", func(t *testing.T) {
		installedPath := filepath.Join(tempDir, "installed_creds.json")
		payload := `{
			"installed": {
				"client_id": "installed-client-123.apps.googleusercontent.com",
				"project_id": "test-project",
				"auth_uri": "https://accounts.google.com/o/oauth2/auth",
				"token_uri": "https://oauth2.googleapis.com/token",
				"client_secret": "GOCSPX-supersecretkey123"
			}
		}`
		if err := os.WriteFile(installedPath, []byte(payload), 0600); err != nil {
			t.Fatalf("failed to write installed json: %v", err)
		}

		creds, err := ParseGoogleCredentials(installedPath)
		if err != nil {
			t.Fatalf("unexpected error parsing installed credentials: %v", err)
		}
		if creds.ClientID != "installed-client-123.apps.googleusercontent.com" {
			t.Errorf("unexpected client_id: %s", creds.ClientID)
		}
		if creds.ClientSecret != "GOCSPX-supersecretkey123" {
			t.Errorf("unexpected client_secret: %s", creds.ClientSecret)
		}
		if creds.ProjectID != "test-project" {
			t.Errorf("unexpected project_id: %s", creds.ProjectID)
		}
	})

	t.Run("Parses valid web application format successfully", func(t *testing.T) {
		webPath := filepath.Join(tempDir, "web_creds.json")
		payload := `{
			"web": {
				"client_id": "web-client-456.apps.googleusercontent.com",
				"project_id": "web-project",
				"client_secret": "GOCSPX-websecret987"
			}
		}`
		if err := os.WriteFile(webPath, []byte(payload), 0600); err != nil {
			t.Fatalf("failed to write web json: %v", err)
		}

		creds, err := ParseGoogleCredentials(webPath)
		if err != nil {
			t.Fatalf("unexpected error parsing web credentials: %v", err)
		}
		if creds.ClientID != "web-client-456.apps.googleusercontent.com" {
			t.Errorf("unexpected client_id: %s", creds.ClientID)
		}
		if creds.ClientSecret != "GOCSPX-websecret987" {
			t.Errorf("unexpected client_secret: %s", creds.ClientSecret)
		}
	})

	t.Run("Parses valid flat format successfully", func(t *testing.T) {
		flatPath := filepath.Join(tempDir, "flat_creds.json")
		payload := `{
			"client_id": "flat-client-789.apps.googleusercontent.com",
			"client_secret": "GOCSPX-flatsecret456"
		}`
		if err := os.WriteFile(flatPath, []byte(payload), 0600); err != nil {
			t.Fatalf("failed to write flat json: %v", err)
		}

		creds, err := ParseGoogleCredentials(flatPath)
		if err != nil {
			t.Fatalf("unexpected error parsing flat credentials: %v", err)
		}
		if creds.ClientID != "flat-client-789.apps.googleusercontent.com" {
			t.Errorf("unexpected client_id: %s", creds.ClientID)
		}
		if creds.ClientSecret != "GOCSPX-flatsecret456" {
			t.Errorf("unexpected client_secret: %s", creds.ClientSecret)
		}
	})
}

// TestAppSec_SecretLeakagePrevention ensures client secrets are never leaked into logs, JSON, or terminal (PLUG-04).
func TestAppSec_SecretLeakagePrevention(t *testing.T) {
	secret := "GOCSPX-supersecretkey999"
	creds := &OAuthClientCredentials{
		ClientID:     "123456789.apps.googleusercontent.com",
		ClientSecret: secret,
		ProjectID:    "cyber-defense-project",
	}

	t.Run("String method redacts client_secret", func(t *testing.T) {
		strRepr := creds.String()
		if strings.Contains(strRepr, secret) {
			t.Fatalf("CRITICAL SECRET LEAKAGE: String() exposes raw secret: %s", strRepr)
		}
		if !strings.Contains(strRepr, "[REDACTED]") {
			t.Fatalf("String() should contain [REDACTED]: %s", strRepr)
		}
	})

	t.Run("JSON serialization omits client_secret completely", func(t *testing.T) {
		marshaled, err := json.Marshal(creds)
		if err != nil {
			t.Fatalf("failed to marshal creds: %v", err)
		}
		jsonStr := string(marshaled)
		if strings.Contains(jsonStr, secret) {
			t.Fatalf("CRITICAL SECRET LEAKAGE: json.Marshal exposes raw secret: %s", jsonStr)
		}
		if strings.Contains(jsonStr, "client_secret") {
			t.Fatalf("json.Marshal should omit client_secret field entirely: %s", jsonStr)
		}
	})

	t.Run("SafeSummary does not expose secret", func(t *testing.T) {
		summary := creds.SafeSummary()
		summaryJSON, _ := json.Marshal(summary)
		if strings.Contains(string(summaryJSON), secret) {
			t.Fatalf("CRITICAL SECRET LEAKAGE: SafeSummary exposed secret: %s", string(summaryJSON))
		}
		if summary["has_secret"] != true {
			t.Errorf("expected has_secret to be true")
		}
	})

	t.Run("MaskSecret reveals at most 3 chars at ends", func(t *testing.T) {
		shortMasked := MaskSecret("secret1")
		if shortMasked != "********" {
			t.Errorf("expected 8 asterisks for short secret, got %q", shortMasked)
		}

		longMasked := MaskSecret("GOCSPX-supersecretkey999")
		if !strings.HasPrefix(longMasked, "GOC...") || !strings.HasSuffix(longMasked, "...999") {
			t.Errorf("unexpected long secret mask: %q", longMasked)
		}
		if strings.Contains(longMasked, "supersecretkey") {
			t.Errorf("masked secret leaks internal secret body: %q", longMasked)
		}
	})
}

// TestAppSec_FileAndDirectoryPermissions verifies strict 0600 on secrets and 0700 on config dirs.
func TestAppSec_FileAndDirectoryPermissions(t *testing.T) {
	tempDir := t.TempDir()
	nestedConfigDir := filepath.Join(tempDir, "secure_config_test", "subdir")
	authPath := filepath.Join(nestedConfigDir, "auth.json")

	// Set permissive umask temporarily to verify that SaveToken explicitly overrides umask
	oldUmask := syscall.Umask(0000)
	defer syscall.Umask(oldUmask)

	token := &TokenInfo{
		AccessToken:  "secure-access-token",
		RefreshToken: "secure-refresh-token",
		ClientID:     "safe-client-id",
		ClientSecret: "safe-secret",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	if err := SaveToken(authPath, token); err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	// 1. Check directory permissions
	dirInfo, err := os.Stat(nestedConfigDir)
	if err != nil {
		t.Fatalf("stat config dir failed: %v", err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("expected directory perm 0700, got %o", dirInfo.Mode().Perm())
	}

	// 2. Check token file permissions
	fileInfo, err := os.Stat(authPath)
	if err != nil {
		t.Fatalf("stat auth file failed: %v", err)
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Errorf("expected file perm 0600, got %o", fileInfo.Mode().Perm())
	}

	// 3. Overwrite an existing file that previously had 0644
	_ = os.Chmod(authPath, 0644)
	token.AccessToken = "updated-access-token"
	if err := SaveToken(authPath, token); err != nil {
		t.Fatalf("SaveToken overwrite failed: %v", err)
	}

	overwrittenInfo, err := os.Stat(authPath)
	if err != nil {
		t.Fatalf("stat overwritten auth file failed: %v", err)
	}
	if overwrittenInfo.Mode().Perm() != 0600 {
		t.Errorf("SaveToken failed to reset permissions to 0600 on existing file, got %o", overwrittenInfo.Mode().Perm())
	}
}
