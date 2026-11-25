package profile

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// URLProfileResolution descreve qual perfil deve atender uma URL e como evitar conflitos de conta.
type URLProfileResolution struct {
	InputURL           string          `json:"input_url"`
	Host               string          `json:"host"`
	IsSharedMultiLogin bool            `json:"is_shared_multi_login"`
	MatchedBy          string          `json:"matched_by"` // "domain", "authuser_param", "shared_workspace", "default_active"
	Recommended        *ChromeProfile  `json:"recommended,omitempty"`
	Alternatives       []ChromeProfile `json:"alternatives,omitempty"`
	SuggestedURL       string          `json:"suggested_url,omitempty"`
	StartCommand       string          `json:"start_command,omitempty"`
	Hint               string          `json:"hint"`
}

var sharedGoogleHosts = map[string]bool{
	"docs.google.com":       true,
	"drive.google.com":      true,
	"classroom.google.com":  true,
	"sites.google.com":      true,
	"mail.google.com":       true,
	"calendar.google.com":   true,
	"meet.google.com":       true,
	"notebooklm.google.com": true,
	"accounts.google.com":   true,
}

var googleUserPathRegex = regexp.MustCompile(`/u/(\d+)(/|$)`)

// ResolveURLForProfiles analisa uma URL contra os perfis descobertos e recomenda o perfil e a URL ideal.
func ResolveURLForProfiles(profiles []ChromeProfile, rawURL string) (*URLProfileResolution, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, fmt.Errorf("URL vazia")
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("URL inválida: %q", rawURL)
	}

	host := strings.ToLower(parsed.Hostname())
	res := &URLProfileResolution{
		InputURL:           rawURL,
		Host:               host,
		IsSharedMultiLogin: sharedGoogleHosts[host],
	}

	// Filtra apenas perfis autenticados (ignora ghosts)
	var authProfiles []ChromeProfile
	for _, p := range profiles {
		if p.Kind != ProfileKindGhost {
			authProfiles = append(authProfiles, p)
		}
	}

	if len(authProfiles) == 0 {
		res.MatchedBy = "none"
		res.Hint = "Nenhum perfil autenticado encontrado. Inicie e faça login com 'agyo browser start'."
		return res, nil
	}

	// 1. Verifica se a própria URL já especifica ?authuser=<email|idx> ou /u/<idx>/
	if authUser := parsed.Query().Get("authuser"); authUser != "" {
		if matched := matchByAuthUser(authProfiles, authUser); matched != nil {
			res.MatchedBy = "authuser_param"
			res.Recommended = matched
			res.SuggestedURL = ApplyAuthUserURL(trimmed, matched.PrimaryEmail)
			res.StartCommand = fmt.Sprintf("agyo browser start --profile %q", matched.Directory)
			res.Hint = fmt.Sprintf("A URL especifica authuser=%s, que pertence ao perfil %s (%s).", authUser, matched.Directory, matched.PrimaryEmail)
			return res, nil
		}
	}

	if m := googleUserPathRegex.FindStringSubmatch(parsed.Path); len(m) == 3 {
		if idx, err := strconv.Atoi(m[1]); err == nil {
			// Se pede /u/N/ (ex: /u/3/) e existem perfis Workspace separados, prioriza Workspace quando idx > 0
			for i := range authProfiles {
				for _, acc := range authProfiles[i].Accounts {
					if acc.Index == idx {
						res.MatchedBy = "url_user_index"
						res.Recommended = &authProfiles[i]
						res.SuggestedURL = ApplyAuthUserURL(trimmed, acc.Email)
						res.StartCommand = fmt.Sprintf("agyo browser start --profile %q", authProfiles[i].Directory)
						res.Hint = fmt.Sprintf("O caminho /u/%d/ aponta para a conta %s no perfil %s.", idx, acc.Email, authProfiles[i].Directory)
						return res, nil
					}
				}
			}
			if idx > 0 {
				for i := range authProfiles {
					if authProfiles[i].Kind == ProfileKindWorkspace {
						res.MatchedBy = "workspace_for_multiuser_path"
						res.Recommended = &authProfiles[i]
						res.SuggestedURL = ApplyAuthUserURL(trimmed, authProfiles[i].PrimaryEmail)
						res.StartCommand = fmt.Sprintf("agyo browser start --profile %q", authProfiles[i].Directory)
						res.Hint = fmt.Sprintf("O caminho /u/%d/ indica conta secundária/institucional, mas ela está isolada no perfil separado %s (%s).", idx, authProfiles[i].Directory, authProfiles[i].PrimaryEmail)
						return res, nil
					}
				}
			}
		}
	}

	// 2. Verifica casamento agnóstico de domínio entre o host/path da URL e o HostedDomain de algum perfil
	for i := range authProfiles {
		p := &authProfiles[i]
		if domainMatchesURL(p.HostedDomain, host, parsed.Path) {
			res.MatchedBy = "domain"
			res.Recommended = p
			res.SuggestedURL = ApplyAuthUserURL(trimmed, p.PrimaryEmail)
			res.StartCommand = fmt.Sprintf("agyo browser start --profile %q", p.Directory)
			res.Hint = fmt.Sprintf("Domínio organizacional %q corresponde ao perfil %s (%s).", p.HostedDomain, p.Directory, p.PrimaryEmail)
			return res, nil
		}
		for _, acc := range p.Accounts {
			if domainMatchesURL(acc.HostedDomain, host, parsed.Path) {
				res.MatchedBy = "domain"
				res.Recommended = p
				res.SuggestedURL = ApplyAuthUserURL(trimmed, acc.Email)
				res.StartCommand = fmt.Sprintf("agyo browser start --profile %q", p.Directory)
				res.Hint = fmt.Sprintf("Domínio organizacional %q corresponde à conta %s no perfil %s.", acc.HostedDomain, acc.Email, p.Directory)
				return res, nil
			}
		}
	}

	// 3. Se for um host Google compartilhado (Forms, Drive, Classroom, Docs) e o usuário tiver múltiplos perfis (Personal + Workspace)
	if res.IsSharedMultiLogin && len(authProfiles) > 1 {
		var workspaces []ChromeProfile
		var personals []ChromeProfile
		for _, p := range authProfiles {
			if p.Kind == ProfileKindWorkspace {
				workspaces = append(workspaces, p)
			} else {
				personals = append(personals, p)
			}
		}

		// Classroom ou Forms em ambiente com conta Workspace: alerta sobre restrição de organização
		if (host == "classroom.google.com" || strings.HasPrefix(parsed.Path, "/forms/")) && len(workspaces) > 0 {
			rec := workspaces[0]
			res.MatchedBy = "shared_workspace"
			res.Recommended = &rec
			res.Alternatives = append(res.Alternatives, personals...)
			res.SuggestedURL = ApplyAuthUserURL(trimmed, rec.PrimaryEmail)
			res.StartCommand = fmt.Sprintf("agyo browser start --profile %q", rec.Directory)
			res.Hint = fmt.Sprintf("Host compartilhado (%s) com perfil Workspace disponível (%s - %s). Use authuser=%s ou o perfil %s para evitar 'Não foi possível abrir o arquivo' no Google Drive.", host, rec.Directory, rec.PrimaryEmail, rec.PrimaryEmail, rec.Directory)
			return res, nil
		}
	}

	// 4. Fallback: perfil autenticado padrão/mais recente
	rec := authProfiles[0]
	res.MatchedBy = "default_active"
	res.Recommended = &rec
	if len(authProfiles) > 1 {
		res.Alternatives = authProfiles[1:]
	}
	res.SuggestedURL = trimmed
	res.StartCommand = fmt.Sprintf("agyo browser start --profile %q", rec.Directory)
	res.Hint = fmt.Sprintf("Perfil padrão selecionado: %s (%s).", rec.Directory, rec.PrimaryEmail)
	return res, nil
}

func matchByAuthUser(profiles []ChromeProfile, authUser string) *ChromeProfile {
	for i := range profiles {
		if strings.EqualFold(profiles[i].PrimaryEmail, authUser) {
			return &profiles[i]
		}
		for _, acc := range profiles[i].Accounts {
			if strings.EqualFold(acc.Email, authUser) {
				return &profiles[i]
			}
		}
	}
	return nil
}

// domainMatchesURL verifica de forma agnóstica se o HostedDomain (ex: "aluno.impacta.edu.br" ou "acme.com.br")
// casa com o host da URL (ex: "portal.impacta.edu.br") ou com o slug organizacional em Google Sites/Drive.
func domainMatchesURL(hostedDomain, urlHost, urlPath string) bool {
	if hostedDomain == "" {
		return false
	}
	if urlHost == hostedDomain || strings.HasSuffix(urlHost, "."+hostedDomain) || strings.HasSuffix(hostedDomain, "."+urlHost) {
		return true
	}

	// Extrai a raiz organizacional (ex: "aluno.impacta.edu.br" -> "impacta.edu.br" / "impacta")
	rootDomain, orgSlug := extractOrgRootAndSlug(hostedDomain)
	if rootDomain != "" && (urlHost == rootDomain || strings.HasSuffix(urlHost, "."+rootDomain)) {
		return true
	}
	if orgSlug != "" && len(orgSlug) >= 4 {
		if strings.Contains(urlHost, orgSlug) || strings.Contains(strings.ToLower(urlPath), orgSlug) {
			return true
		}
	}
	return false
}

func extractOrgRootAndSlug(domain string) (string, string) {
	parts := strings.Split(strings.ToLower(domain), ".")
	if len(parts) < 2 {
		return domain, domain
	}
	// Trata TLDs compostos (.com.br, .edu.br, .org.br, .gov.br, .co.uk)
	compoundTLD := false
	if len(parts) >= 3 {
		lastTwo := parts[len(parts)-2] + "." + parts[len(parts)-1]
		switch lastTwo {
		case "com.br", "edu.br", "org.br", "gov.br", "net.br", "co.uk", "ac.uk", "edu.au", "com.au":
			compoundTLD = true
		}
	}

	if compoundTLD && len(parts) >= 3 {
		org := parts[len(parts)-3]
		root := strings.Join(parts[len(parts)-3:], ".")
		return root, org
	}

	org := parts[len(parts)-2]
	root := strings.Join(parts[len(parts)-2:], ".")
	return root, org
}

// ApplyAuthUserURL injeta ?authuser=<email> em URLs de serviços Google para forçar a conta correta sem AccountChooser loop.
func ApplyAuthUserURL(rawURL string, email string) string {
	if email == "" {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := strings.ToLower(parsed.Hostname())
	if !sharedGoogleHosts[host] && !strings.HasSuffix(host, ".google.com") {
		return rawURL
	}
	q := parsed.Query()
	q.Set("authuser", email)
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

// DetectAuthWall observa o título e a URL pós-navegação de uma aba e identifica bloqueios de permissão,
// AccountChooser ou interceptação SAML/VPN.
func DetectAuthWall(title, currentURL string) (bool, string) {
	tLower := strings.ToLower(strings.TrimSpace(title))
	uLower := strings.ToLower(strings.TrimSpace(currentURL))

	if strings.Contains(uLower, "remote/saml/start") || strings.Contains(uLower, "127.0.0.1:8020") {
		return true, "Interceptação de VPN/SAML detectada na rede. Conclua a autenticação da VPN antes de navegar."
	}
	if strings.Contains(uLower, "accounts.google.com/v3/signin") ||
		strings.Contains(uLower, "accounts.google.com/servicelogin") ||
		strings.Contains(uLower, "accountchooser") {
		return true, "Redirecionado para Google AccountChooser/Login. Verifique se o perfil correto está ativo ('agyo browser profiles')."
	}
	if strings.Contains(tLower, "acesso negado") ||
		strings.Contains(tLower, "access denied") ||
		strings.Contains(tLower, "não foi possível abrir o arquivo") ||
		strings.Contains(tLower, "você precisa de permissão") ||
		strings.Contains(tLower, "you need permission") {
		return true, "Bloqueio de permissão organizacional detectado ('Acesso negado' / 'Não foi possível abrir o arquivo'). A URL exige o perfil Workspace da organização."
	}
	return false, ""
}
