package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SanitizeICSText sanitizes text fields for iCalendar format, neutralizing CRLF injection
// and stripping dangerous control characters (PLUG-03).
func SanitizeICSText(s string) string {
	var b strings.Builder
	for _, r := range s {
		// Strip ASCII control characters except tab (0x09), line feed (\n), and carriage return (\r)
		if (r >= 0 && r < 0x20 && r != '\t' && r != '\n' && r != '\r') || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	cleaned := b.String()
	// Normalize all newline variations to \n
	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "\n")

	// RFC 5545 escaping order: backslash first, then semicolon, comma, newline
	cleaned = strings.ReplaceAll(cleaned, `\`, `\\`)
	cleaned = strings.ReplaceAll(cleaned, `;`, `\;`)
	cleaned = strings.ReplaceAll(cleaned, `,`, `\,`)
	cleaned = strings.ReplaceAll(cleaned, "\n", `\n`)
	return cleaned
}

// ToICS generates an RFC 5545 compliant iCalendar string representation of the event
// with full sanitization against CRLF property injection (PLUG-03).
func (e CalendarEvent) ToICS() string {
	start := e.Start
	if start.IsZero() {
		start = time.Now()
	}
	end := e.End
	if end.IsZero() || !end.After(start) {
		end = start.Add(1 * time.Hour)
	}

	uid := strings.TrimSpace(e.ID)
	if uid == "" {
		uid = fmt.Sprintf("%d-%s@antigravity", time.Now().UnixNano(), GenerateState()[:8])
	} else {
		uid = SanitizeICSText(uid)
	}

	nowUTC := time.Now().UTC().Format("20060102T150405Z")
	startUTC := start.UTC().Format("20060102T150405Z")
	endUTC := end.UTC().Format("20060102T150405Z")

	var sb strings.Builder
	sb.WriteString("BEGIN:VCALENDAR\r\n")
	sb.WriteString("VERSION:2.0\r\n")
	sb.WriteString("PRODID:-//Antigravity Operator//agyo//PT-BR\r\n")
	sb.WriteString("CALSCALE:GREGORIAN\r\n")
	sb.WriteString("METHOD:PUBLISH\r\n")
	sb.WriteString("BEGIN:VEVENT\r\n")
	sb.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
	sb.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", nowUTC))
	sb.WriteString(fmt.Sprintf("DTSTART:%s\r\n", startUTC))
	sb.WriteString(fmt.Sprintf("DTEND:%s\r\n", endUTC))
	sb.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", SanitizeICSText(e.Summary)))
	if e.Description != "" {
		sb.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", SanitizeICSText(e.Description)))
	}
	if e.Location != "" {
		sb.WriteString(fmt.Sprintf("LOCATION:%s\r\n", SanitizeICSText(e.Location)))
	}
	sb.WriteString("STATUS:CONFIRMED\r\n")
	sb.WriteString("END:VEVENT\r\n")
	sb.WriteString("END:VCALENDAR\r\n")

	return sb.String()
}

// CalendarEventsToICS serializes multiple calendar events into a single RFC 5545 iCalendar stream (PLUG-03).
func CalendarEventsToICS(events []CalendarEvent) string {
	var sb strings.Builder
	sb.WriteString("BEGIN:VCALENDAR\r\n")
	sb.WriteString("VERSION:2.0\r\n")
	sb.WriteString("PRODID:-//Antigravity Operator//agyo//PT-BR\r\n")
	sb.WriteString("CALSCALE:GREGORIAN\r\n")
	sb.WriteString("METHOD:PUBLISH\r\n")

	for _, e := range events {
		st := e.Start
		if st.IsZero() {
			st = time.Now()
		}
		et := e.End
		if et.IsZero() || !et.After(st) {
			et = st.Add(1 * time.Hour)
		}

		uid := strings.TrimSpace(e.ID)
		if uid == "" {
			uid = fmt.Sprintf("%d-%s@antigravity", time.Now().UnixNano(), GenerateState()[:8])
		} else {
			uid = SanitizeICSText(uid)
		}

		nowUTC := time.Now().UTC().Format("20060102T150405Z")
		startUTC := st.UTC().Format("20060102T150405Z")
		endUTC := et.UTC().Format("20060102T150405Z")

		sb.WriteString("BEGIN:VEVENT\r\n")
		sb.WriteString(fmt.Sprintf("UID:%s\r\n", uid))
		sb.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", nowUTC))
		sb.WriteString(fmt.Sprintf("DTSTART:%s\r\n", startUTC))
		sb.WriteString(fmt.Sprintf("DTEND:%s\r\n", endUTC))
		sb.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", SanitizeICSText(e.Summary)))
		if e.Description != "" {
			sb.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", SanitizeICSText(e.Description)))
		}
		if e.Location != "" {
			sb.WriteString(fmt.Sprintf("LOCATION:%s\r\n", SanitizeICSText(e.Location)))
		}
		sb.WriteString("STATUS:CONFIRMED\r\n")
		sb.WriteString("END:VEVENT\r\n")
	}

	sb.WriteString("END:VCALENDAR\r\n")
	return sb.String()
}

// evalPathSymlinks canonicalizes a path by resolving symlinks on its deepest existing ancestor (e.g. /var -> /private/var on macOS).
func evalPathSymlinks(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	cleaned := filepath.Clean(abs)
	curr := cleaned
	var suffix []string
	for {
		if _, err := os.Lstat(curr); err == nil {
			if eval, err := filepath.EvalSymlinks(curr); err == nil {
				curr = eval
			}
			break
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		suffix = append([]string{filepath.Base(curr)}, suffix...)
		curr = parent
	}
	for _, part := range suffix {
		curr = filepath.Join(curr, part)
	}
	return filepath.Clean(curr)
}

// ValidateSecureICSPath verifies that the provided targetPath is a valid .ics file path
// and strictly resides within the authorized base directory, preventing Path Traversal (PLUG-03).
func ValidateSecureICSPath(targetPath, allowedBaseDir string) (string, error) {
	if strings.TrimSpace(targetPath) == "" {
		return "", errors.New("caminho do arquivo ICS não pode ser vazio")
	}
	if strings.ContainsRune(targetPath, 0) {
		return "", errors.New("caminho do arquivo ICS contém caractere nulo inválido")
	}

	cleaned := filepath.Clean(targetPath)
	ext := strings.ToLower(filepath.Ext(cleaned))
	if ext != ".ics" {
		return "", fmt.Errorf("extensão inválida para arquivo de calendário: esperado .ics (recebido: %q)", ext)
	}

	if allowedBaseDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("obter diretório de trabalho: %w", err)
		}
		allowedBaseDir = cwd
	}

	absBase := evalPathSymlinks(allowedBaseDir)
	absTarget := evalPathSymlinks(cleaned)

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path traversal detectado: caminho fora do diretório autorizado (%s)", absBase)
	}

	// Defense in depth: block sensitive system root paths
	sensitiveRoots := []string{"/etc", "/sys", "/proc", "/dev", "/boot", "/root", "/bin", "/sbin", "/usr"}
	for _, sr := range sensitiveRoots {
		if absTarget == sr || strings.HasPrefix(absTarget, sr+string(filepath.Separator)) ||
			strings.HasPrefix(absTarget, "/private"+sr+string(filepath.Separator)) {
			return "", fmt.Errorf("escrita bloqueada em diretório sensível do sistema: %s", sr)
		}
	}

	return absTarget, nil
}

// WriteSecureICSFile writes an iCalendar file with strict path validation and restricted permissions (PLUG-03).
func WriteSecureICSFile(targetPath string, content []byte, allowedBaseDir string) error {
	safePath, err := ValidateSecureICSPath(targetPath, allowedBaseDir)
	if err != nil {
		return err
	}

	dir := filepath.Dir(safePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("criar diretório do arquivo ICS: %w", err)
	}
	_ = os.Chmod(dir, 0700)

	// Verify symlink destination if the file already exists
	if lstat, err := os.Lstat(safePath); err == nil {
		if lstat.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(safePath)
			if err != nil {
				return fmt.Errorf("avaliar link simbólico: %w", err)
			}
			if _, err := ValidateSecureICSPath(resolved, allowedBaseDir); err != nil {
				return fmt.Errorf("symlink traversal detectado: link aponta para fora do diretório autorizado: %w", err)
			}
		}
	}

	if err := os.WriteFile(safePath, content, 0600); err != nil {
		return fmt.Errorf("escrever arquivo ICS: %w", err)
	}
	_ = os.Chmod(safePath, 0600)
	return nil
}
