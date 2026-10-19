package workspace

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseFlexTime_StandardFormats(t *testing.T) {
	ref := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		input    string
		expected time.Time
	}{
		{
			input:    "2026-10-25T14:30:00Z",
			expected: time.Date(2026, 10, 25, 14, 30, 0, 0, time.UTC),
		},
		{
			input:    "2026-10-25T14:30:00.000Z",
			expected: time.Date(2026, 10, 25, 14, 30, 0, 0, time.UTC),
		},
		{
			input:    "2026-10-25 14:30:00",
			expected: time.Date(2026, 10, 25, 14, 30, 0, 0, time.UTC),
		},
		{
			input:    "2026-10-25 14:30",
			expected: time.Date(2026, 10, 25, 14, 30, 0, 0, time.UTC),
		},
		{
			input:    "2026-10-25T14:30:00",
			expected: time.Date(2026, 10, 25, 14, 30, 0, 0, time.UTC),
		},
		{
			input:    "2026-10-25T14:30",
			expected: time.Date(2026, 10, 25, 14, 30, 0, 0, time.UTC),
		},
		{
			input:    "2026-10-25",
			expected: time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC),
		},
		{
			input:    "25/10/2026",
			expected: time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC),
		},
		{
			input:    "25/10/2026 14:30",
			expected: time.Date(2026, 10, 25, 14, 30, 0, 0, time.UTC),
		},
		{
			input:    "25/10/2026 14:30:00",
			expected: time.Date(2026, 10, 25, 14, 30, 0, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseFlexTime(tc.input, ref)
			if err != nil {
				t.Fatalf("ParseFlexTime(%q) inesperadamente falhou: %v", tc.input, err)
			}
			if !got.Equal(tc.expected) {
				t.Errorf("ParseFlexTime(%q) = %v; want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestParseFlexTime_RelativeExpressions(t *testing.T) {
	loc := time.FixedZone("BRT", -3*3600)
	ref := time.Date(2026, 10, 19, 14, 15, 0, 0, loc)

	cases := []struct {
		name     string
		input    string
		expected time.Time
	}{
		{
			name:     "hoje com horario",
			input:    "hoje 19:30",
			expected: time.Date(2026, 10, 19, 19, 30, 0, 0, loc),
		},
		{
			name:     "today with time",
			input:    "today 19:30",
			expected: time.Date(2026, 10, 19, 19, 30, 0, 0, loc),
		},
		{
			name:     "tomorrow with time",
			input:    "tomorrow 10:00",
			expected: time.Date(2026, 10, 20, 10, 0, 0, 0, loc),
		},
		{
			name:     "amanha com horario",
			input:    "amanha 10:00",
			expected: time.Date(2026, 10, 20, 10, 0, 0, 0, loc),
		},
		{
			name:     "amanhã acentuado",
			input:    "amanhã 10:00",
			expected: time.Date(2026, 10, 20, 10, 0, 0, 0, loc),
		},
		{
			name:     "hoje isolado",
			input:    "hoje",
			expected: time.Date(2026, 10, 19, 9, 0, 0, 0, loc),
		},
		{
			name:     "today isolated",
			input:    "today",
			expected: time.Date(2026, 10, 19, 9, 0, 0, 0, loc),
		},
		{
			name:     "tomorrow isolated",
			input:    "tomorrow",
			expected: time.Date(2026, 10, 20, 9, 0, 0, 0, loc),
		},
		{
			name:     "amanha isolado",
			input:    "amanha",
			expected: time.Date(2026, 10, 20, 9, 0, 0, 0, loc),
		},
		{
			name:     "offset +2h",
			input:    "+2h",
			expected: ref.Add(2 * time.Hour),
		},
		{
			name:     "offset +30m",
			input:    "+30m",
			expected: ref.Add(30 * time.Minute),
		},
		{
			name:     "offset +1d",
			input:    "+1d",
			expected: ref.AddDate(0, 0, 1),
		},
		{
			name:     "formato 15h04",
			input:    "hoje 15h30",
			expected: time.Date(2026, 10, 19, 15, 30, 0, 0, loc),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseFlexTime(tc.input, ref)
			if err != nil {
				t.Fatalf("ParseFlexTime(%q) erro inesperado: %v", tc.input, err)
			}
			if !got.Equal(tc.expected) {
				t.Errorf("ParseFlexTime(%q) = %v; want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestParseFlexTime_InvalidInputs(t *testing.T) {
	ref := time.Now()
	invalidInputs := []string{
		"",
		"   ",
		"invalid-date",
		"hoje horario_invalido",
		"tomorrow 25:99",
		"+xyz",
		"99/99/9999",
		"2026-13-45",
	}

	for _, in := range invalidInputs {
		in := in
		t.Run(fmt.Sprintf("input_%s", in), func(t *testing.T) {
			_, err := ParseFlexTime(in, ref)
			if err == nil {
				t.Errorf("ParseFlexTime(%q) esperava erro, retornou nil", in)
			}
		})
	}
}

func TestBuildCalendarWebURL_Validation(t *testing.T) {
	start := time.Date(2026, 10, 20, 14, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 20, 15, 30, 0, 0, time.UTC)

	event := CalendarEvent{
		Summary:     "Reunião de Alinhamento & Sprint",
		Description: "Pauta: arquitetura, métricas e deploy.\nLink: https://meet.google.com/xyz",
		Location:    "Google Meet (Online) #sala-4",
		Start:       start,
		End:         end,
	}

	webURL := BuildCalendarWebURL(event)

	if !strings.HasPrefix(webURL, "https://calendar.google.com/calendar/render?action=TEMPLATE") {
		t.Fatalf("URL não começa com base esperada: %s", webURL)
	}

	parsed, err := url.Parse(webURL)
	if err != nil {
		t.Fatalf("URL gerada é inválida: %v", err)
	}

	q := parsed.Query()
	if q.Get("action") != "TEMPLATE" {
		t.Errorf("action = %q; want TEMPLATE", q.Get("action"))
	}
	if q.Get("text") != event.Summary {
		t.Errorf("text = %q; want %q", q.Get("text"), event.Summary)
	}
	if q.Get("details") != event.Description {
		t.Errorf("details = %q; want %q", q.Get("details"), event.Description)
	}
	if q.Get("location") != event.Location {
		t.Errorf("location = %q; want %q", q.Get("location"), event.Location)
	}
	expectedDates := "20261020T140000Z/20261020T153000Z"
	if q.Get("dates") != expectedDates {
		t.Errorf("dates = %q; want %q", q.Get("dates"), expectedDates)
	}

	// Verifica caso com Start e End zerados
	zeroEvent := CalendarEvent{
		Summary: "Evento Imediato",
	}
	zeroURL := BuildCalendarWebURL(zeroEvent)
	if !strings.Contains(zeroURL, "text=Evento+Imediato") {
		t.Errorf("zeroURL não contém resumo esperado: %s", zeroURL)
	}
}

func TestToICS_RFC5545Compliance(t *testing.T) {
	start := time.Date(2026, 10, 20, 14, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 20, 15, 0, 0, 0, time.UTC)

	event := CalendarEvent{
		ID:          "custom-uid-12345",
		Summary:     "Prova de Segurança e Gestão de Riscos",
		Description: "Avaliação AC02 Impacta; peso 40%, estudar capítulos 1 a 4.",
		Location:    "Sala Virtual",
		Start:       start,
		End:         end,
	}

	ics := event.ToICS()

	// Validação de conformidade RFC 5545
	requiredElements := []string{
		"BEGIN:VCALENDAR\r\n",
		"VERSION:2.0\r\n",
		"PRODID:-//Antigravity Operator//agyo//PT-BR\r\n",
		"CALSCALE:GREGORIAN\r\n",
		"METHOD:PUBLISH\r\n",
		"BEGIN:VEVENT\r\n",
		"UID:custom-uid-12345\r\n",
		"DTSTART:20261020T140000Z\r\n",
		"DTEND:20261020T150000Z\r\n",
		"SUMMARY:Prova de Segurança e Gestão de Riscos\r\n",
		"DESCRIPTION:Avaliação AC02 Impacta\\; peso 40%\\, estudar capítulos 1 a 4.\r\n",
		"LOCATION:Sala Virtual\r\n",
		"STATUS:CONFIRMED\r\n",
		"END:VEVENT\r\n",
		"END:VCALENDAR\r\n",
	}

	for _, req := range requiredElements {
		if !strings.Contains(ics, req) {
			t.Errorf("ICS output está ausente de elemento obrigatório RFC 5545: %q\nConteúdo completo:\n%s", req, ics)
		}
	}

	// Garante que todas as quebras de linha são CRLF \r\n estritas
	lines := strings.Split(ics, "\n")
	for i, line := range lines {
		if len(line) > 0 && !strings.HasSuffix(line, "\r") {
			t.Errorf("Linha %d não termina com CRLF (\\r\\n): %q", i+1, line)
		}
	}
}

func TestParseGoogleCredentials_ValidAndInvalid(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Installed client credentials
	installedJSON := `{
  "installed": {
    "client_id": "123456789-abcdef.apps.googleusercontent.com",
    "project_id": "impacta-estudos-proj",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "client_secret": "GOCSPX-supersecretkey1234567"
  }
}`
	installedPath := filepath.Join(tempDir, "installed_creds.json")
	if err := os.WriteFile(installedPath, []byte(installedJSON), 0600); err != nil {
		t.Fatal(err)
	}

	creds, err := ParseGoogleCredentials(installedPath)
	if err != nil {
		t.Fatalf("ParseGoogleCredentials(installed) erro: %v", err)
	}
	if creds.ClientID != "123456789-abcdef.apps.googleusercontent.com" {
		t.Errorf("creds.ClientID = %q; want 123456789-abcdef.apps.googleusercontent.com", creds.ClientID)
	}
	if creds.ClientSecret != "GOCSPX-supersecretkey1234567" {
		t.Errorf("creds.ClientSecret não corresponde")
	}
	if creds.ProjectID != "impacta-estudos-proj" {
		t.Errorf("creds.ProjectID = %q; want impacta-estudos-proj", creds.ProjectID)
	}

	// Testa String() e MaskSecret() para garantia de não vazamento
	masked := creds.String()
	if strings.Contains(masked, "GOCSPX-supersecretkey1234567") {
		t.Fatalf("creds.String() vazou client_secret não mascarado: %s", masked)
	}
	if !strings.Contains(masked, "[REDACTED]") {
		t.Errorf("creds.String() não contém [REDACTED]: %s", masked)
	}

	// 2. Web client credentials
	webJSON := `{
  "web": {
    "client_id": "987654321-webclient.apps.googleusercontent.com",
    "client_secret": "GOCSPX-websecret987654321",
    "project_id": "web-project"
  }
}`
	webPath := filepath.Join(tempDir, "web_creds.json")
	if err := os.WriteFile(webPath, []byte(webJSON), 0600); err != nil {
		t.Fatal(err)
	}

	webCreds, err := ParseGoogleCredentials(webPath)
	if err != nil {
		t.Fatalf("ParseGoogleCredentials(web) erro: %v", err)
	}
	if webCreds.ClientID != "987654321-webclient.apps.googleusercontent.com" {
		t.Errorf("webCreds.ClientID mismatch")
	}

	// 3. Casos Inválidos
	invalidCases := []struct {
		name string
		path string
	}{
		{
			name: "caminho vazio",
			path: "",
		},
		{
			name: "arquivo inexistente",
			path: filepath.Join(tempDir, "nao_existe.json"),
		},
		{
			name: "extensao nao json",
			path: filepath.Join(tempDir, "creds.txt"),
		},
	}

	// Cria arquivo txt para o caso
	_ = os.WriteFile(filepath.Join(tempDir, "creds.txt"), []byte("data"), 0600)

	for _, ic := range invalidCases {
		ic := ic
		t.Run(ic.name, func(t *testing.T) {
			_, err := ParseGoogleCredentials(ic.path)
			if err == nil {
				t.Errorf("ParseGoogleCredentials(%q) esperava erro, retornou nil", ic.path)
			}
		})
	}

	// Arquivo vazio (0 bytes)
	emptyPath := filepath.Join(tempDir, "empty.json")
	_ = os.WriteFile(emptyPath, []byte(""), 0600)
	if _, err := ParseGoogleCredentials(emptyPath); err == nil {
		t.Error("esperava erro ao processar arquivo vazio de 0 bytes")
	}

	// JSON malformado
	badJSONPath := filepath.Join(tempDir, "bad.json")
	_ = os.WriteFile(badJSONPath, []byte("{corrompido: true"), 0600)
	if _, err := ParseGoogleCredentials(badJSONPath); err == nil {
		t.Error("esperava erro ao processar JSON inválido")
	}

	// JSON sem client_id ou client_secret
	missingFieldPath := filepath.Join(tempDir, "missing.json")
	_ = os.WriteFile(missingFieldPath, []byte(`{"installed": {"project_id": "sem-id"}}`), 0600)
	if _, err := ParseGoogleCredentials(missingFieldPath); err == nil {
		t.Error("esperava erro ao processar credenciais sem client_id/secret")
	}
}
