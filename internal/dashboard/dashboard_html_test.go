package dashboard

import (
	"regexp"
	"strings"
	"testing"
)

// TestDashboardHTML_EscapesUntrustedInterpolations garante que todo ${...} do script embutido
// passa por escapeHtml/Number ou é uma constante interna conhecida. Títulos/URLs de abas do
// Chrome, eventos do transcript, sessão e doctor são dados não confiáveis e chegam via innerHTML.
func TestDashboardHTML_EscapesUntrustedInterpolations(t *testing.T) {
	for _, helper := range []string{"function escapeHtml(", "function safeHttpUrl("} {
		if !strings.Contains(dashboardHTML, helper) {
			t.Errorf("helper ausente no dashboard.html: %s", helper)
		}
	}

	// Constantes internas (ícone/cor do doctor) e fragmentos já escapados montados no próprio script.
	allowed := map[string]bool{"icon": true, "color": true, "pendingHtml": true, "urlHtml": true}

	interp := regexp.MustCompile(`\$\{([^}]*)\}`)
	for _, m := range interp.FindAllStringSubmatch(dashboardHTML, -1) {
		expr := strings.TrimSpace(m[1])
		if allowed[expr] || strings.HasPrefix(expr, "escapeHtml(") || strings.HasPrefix(expr, "Number(") {
			continue
		}
		t.Errorf("interpolação sem escape no dashboard.html: ${%s}", expr)
	}

	// Concatenações cruas conhecidas (forma antiga dos próximos passos da sessão).
	if strings.Contains(dashboardHTML, "' + p + '") {
		t.Errorf("pendingList ainda é concatenado sem escape")
	}

	// href de aba só pode vir de safeHttpUrl (bloqueia javascript:, data:, etc.).
	href := regexp.MustCompile(`href="\$\{([^}]*)\}"`)
	for _, m := range href.FindAllStringSubmatch(dashboardHTML, -1) {
		if m[1] != "escapeHtml(href)" {
			t.Errorf("href não passa por safeHttpUrl+escapeHtml: ${%s}", m[1])
		}
	}
	if !strings.Contains(dashboardHTML, "const href = safeHttpUrl(t.url)") {
		t.Errorf("URL da aba não é validada por safeHttpUrl")
	}
}
