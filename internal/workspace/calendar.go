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
