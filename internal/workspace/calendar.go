package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const DefaultCalendarBaseURL = "https://www.googleapis.com/calendar/v3"

// CalendarEvent represents a single event in Google Calendar.
type CalendarEvent struct {
	ID          string    `json:"id,omitempty"`
	Summary     string    `json:"summary"`
	Description string    `json:"description,omitempty"`
	Location    string    `json:"location,omitempty"`
	HtmlLink    string    `json:"htmlLink,omitempty"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Status      string    `json:"status,omitempty"`
}

// CalendarClient interacts with the Google Calendar API v3.
type CalendarClient struct {
	Client      *http.Client
	BaseURL     string
	TokenSource func(ctx context.Context) (string, error)
}

// NewCalendarClient instantiates a client with sensible defaults.
func NewCalendarClient(client *http.Client, baseURL string, tokenSource func(ctx context.Context) (string, error)) *CalendarClient {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = DefaultCalendarBaseURL
	}
	return &CalendarClient{
		Client:      client,
		BaseURL:     baseURL,
		TokenSource: tokenSource,
	}
}

// ListEvents retrieves upcoming events from the specified calendar (default "primary").
func (c *CalendarClient) ListEvents(ctx context.Context, calendarID string, timeMin, timeMax time.Time, maxResults int) ([]CalendarEvent, error) {
	if calendarID == "" {
		calendarID = "primary"
	}
	if maxResults <= 0 {
		maxResults = 20
	}
	if timeMin.IsZero() {
		timeMin = time.Now()
	}

	token, err := c.TokenSource(ctx)
	if err != nil {
		return nil, fmt.Errorf("get auth token: %w", err)
	}

	endpoint := fmt.Sprintf("%s/calendars/%s/events", c.BaseURL, url.PathEscape(calendarID))
	q := url.Values{}
	q.Set("singleEvents", "true")
	q.Set("orderBy", "startTime")
	q.Set("maxResults", strconv.Itoa(maxResults))
	q.Set("timeMin", timeMin.Format(time.RFC3339))
	if !timeMax.IsZero() {
		q.Set("timeMax", timeMax.Format(time.RFC3339))
	}

	reqURL := fmt.Sprintf("%s?%s", endpoint, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calendar list request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("calendar api error (%d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Items []struct {
			ID          string `json:"id"`
			Summary     string `json:"summary"`
			Description string `json:"description"`
			Location    string `json:"location"`
			HtmlLink    string `json:"htmlLink"`
			Status      string `json:"status"`
			Start       struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"start"`
			End struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"end"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse calendar list json: %w", err)
	}

	events := make([]CalendarEvent, 0, len(raw.Items))
	for _, it := range raw.Items {
		st := parseTime(it.Start.DateTime, it.Start.Date)
		et := parseTime(it.End.DateTime, it.End.Date)

		events = append(events, CalendarEvent{
			ID:          it.ID,
			Summary:     it.Summary,
			Description: it.Description,
			Location:    it.Location,
			HtmlLink:    it.HtmlLink,
			Status:      it.Status,
			Start:       st,
			End:         et,
		})
	}

	return events, nil
}

// CreateEvent inserts a new event into the target calendar (default "primary").
func (c *CalendarClient) CreateEvent(ctx context.Context, calendarID string, event CalendarEvent) (*CalendarEvent, error) {
	if calendarID == "" {
		calendarID = "primary"
	}
	if event.Summary == "" {
		return nil, errors.New("event summary cannot be empty")
	}
	if event.Start.IsZero() {
		event.Start = time.Now()
	}
	if event.End.IsZero() || !event.End.After(event.Start) {
		event.End = event.Start.Add(1 * time.Hour)
	}

	token, err := c.TokenSource(ctx)
	if err != nil {
		return nil, fmt.Errorf("get auth token: %w", err)
	}

	payload := map[string]interface{}{
		"summary":     event.Summary,
		"description": event.Description,
		"location":    event.Location,
		"start": map[string]string{
			"dateTime": event.Start.Format(time.RFC3339),
		},
		"end": map[string]string{
			"dateTime": event.End.Format(time.RFC3339),
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/calendars/%s/events", c.BaseURL, url.PathEscape(calendarID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calendar create request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("calendar api create error (%d): %s", resp.StatusCode, string(body))
	}

	var created struct {
		ID       string `json:"id"`
		HtmlLink string `json:"htmlLink"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("parse calendar create json: %w", err)
	}

	event.ID = created.ID
	event.HtmlLink = created.HtmlLink
	event.Status = created.Status
	return &event, nil
}

func parseTime(dateTimeStr, dateStr string) time.Time {
	if dateTimeStr != "" {
		if t, err := ParseFlexTime(dateTimeStr, time.Time{}); err == nil {
			return t
		}
	}
	if dateStr != "" {
		if t, err := ParseFlexTime(dateStr, time.Time{}); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ParseFlexTime parses datetime strings in flexible formats (PLUG-01):
// - time.RFC3339 and time.RFC3339Nano
// - ISO datetime: "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02T15:04:05"
// - Date only: "2006-01-02" (define início às 09:00 ou na hora de ref)
// - Formato brasileiro/europeu: "02/01/2006 15:04:05", "02/01/2006 15:04", "02/01/2006"
// - Expressões naturais relativas: "today HH:MM", "hoje HH:MM", "tomorrow HH:MM", "amanha HH:MM", "amanhã HH:MM", "+<N>h", "+<N>m", "+<N>d"
func ParseFlexTime(s string, ref time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("empty time string")
	}
	if ref.IsZero() {
		ref = time.Now()
	}
	loc := ref.Location()
	lower := strings.ToLower(s)

	// 1. Relative offset expressions (+<N>h, +<N>m, +<N>d)
	if strings.HasPrefix(lower, "+") {
		offsetStr := strings.TrimSpace(lower[1:])
		if strings.HasSuffix(offsetStr, "d") {
			daysStr := strings.TrimSuffix(offsetStr, "d")
			if days, err := strconv.Atoi(strings.TrimSpace(daysStr)); err == nil {
				return ref.AddDate(0, 0, days), nil
			}
		}
		d, err := time.ParseDuration(offsetStr)
		if err == nil {
			return ref.Add(d), nil
		}
	}

	// 2. Relative natural expressions (today, hoje, tomorrow, amanha, amanhã)
	var baseDate time.Time
	var timePart string
	isRelative := false

	switch {
	case lower == "today" || lower == "hoje":
		return time.Date(ref.Year(), ref.Month(), ref.Day(), 9, 0, 0, 0, loc), nil
	case lower == "tomorrow" || lower == "amanha" || lower == "amanhã":
		tom := ref.AddDate(0, 0, 1)
		return time.Date(tom.Year(), tom.Month(), tom.Day(), 9, 0, 0, 0, loc), nil
	case strings.HasPrefix(lower, "today "):
		baseDate = ref
		timePart = strings.TrimSpace(s[len("today "):])
		isRelative = true
	case strings.HasPrefix(lower, "hoje "):
		baseDate = ref
		timePart = strings.TrimSpace(s[len("hoje "):])
		isRelative = true
	case strings.HasPrefix(lower, "tomorrow "):
		baseDate = ref.AddDate(0, 0, 1)
		timePart = strings.TrimSpace(s[len("tomorrow "):])
		isRelative = true
	case strings.HasPrefix(lower, "amanha "):
		baseDate = ref.AddDate(0, 0, 1)
		timePart = strings.TrimSpace(s[len("amanha "):])
		isRelative = true
	case strings.HasPrefix(lower, "amanhã "):
		baseDate = ref.AddDate(0, 0, 1)
		fields := strings.Fields(s)
		if len(fields) > 1 {
			timePart = strings.Join(fields[1:], " ")
		}
		isRelative = true
	}

	if isRelative {
		timeFormats := []string{
			"15:04:05",
			"15:04",
			"15h04",
			"15h",
			"3:04pm",
			"3:04PM",
		}
		for _, tf := range timeFormats {
			if t, err := time.ParseInLocation(tf, timePart, loc); err == nil {
				return time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc), nil
			}
		}
		return time.Time{}, fmt.Errorf("formato de horário inválido em expressão relativa %q: use HH:MM ou HH:MM:SS", s)
	}

	// 3. RFC3339 / RFC3339Nano (with explicit timezone)
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// 4. ISO datetimes and Brazilian datetimes
	datetimeFormats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"02/01/2006 15:04:05",
		"02/01/2006 15:04",
	}
	for _, layout := range datetimeFormats {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}

	// 5. Date-only formats (default start time at 09:00:00 or ref hour)
	dateOnlyFormats := []string{
		"2006-01-02",
		"02/01/2006",
	}
	for _, layout := range dateOnlyFormats {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 9, 0, 0, 0, loc), nil
		}
	}

	return time.Time{}, fmt.Errorf("formato de data/hora não reconhecido %q: use RFC3339, ISO (YYYY-MM-DD HH:MM), formato brasileiro (DD/MM/YYYY HH:MM) ou relativo (ex: 'hoje 15:00', '+2h')", s)
}

// BuildCalendarWebURL constructs a Google Calendar web template URL with strict URL escaping
// on all query parameters (text, details, location) to prevent parameter injection attacks (PLUG-02).
func BuildCalendarWebURL(event CalendarEvent) string {
	start := event.Start
	if start.IsZero() {
		start = time.Now()
	}
	end := event.End
	if end.IsZero() || !end.After(start) {
		end = start.Add(1 * time.Hour)
	}

	dates := fmt.Sprintf("%s/%s", start.UTC().Format("20060102T150405Z"), end.UTC().Format("20060102T150405Z"))

	baseURL := "https://calendar.google.com/calendar/render?action=TEMPLATE"
	u := baseURL + "&text=" + url.QueryEscape(event.Summary) + "&dates=" + url.QueryEscape(dates)
	if event.Description != "" {
		u += "&details=" + url.QueryEscape(event.Description)
	}
	if event.Location != "" {
		u += "&location=" + url.QueryEscape(event.Location)
	}
	return u
}

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
