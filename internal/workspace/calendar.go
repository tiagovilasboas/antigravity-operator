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
		if t, err := time.Parse(time.RFC3339Nano, dateTimeStr); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339, dateTimeStr); err == nil {
			return t
		}
	}
	if dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			return t
		}
	}
	return time.Time{}
}
