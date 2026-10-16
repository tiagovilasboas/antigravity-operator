package profile

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ProfileKind classifica agnosticamente um perfil do Chrome.
type ProfileKind string

const (
	ProfileKindPersonal  ProfileKind = "personal"
	ProfileKindWorkspace ProfileKind = "workspace"
	ProfileKindGhost     ProfileKind = "ghost"
)

// ProfileAccount representa uma conta conectada dentro do cookie jar de um perfil.
type ProfileAccount struct {
	Index        int    `json:"index"`
	Email        string `json:"email"`
	FullName     string `json:"full_name,omitempty"`
	HostedDomain string `json:"hosted_domain,omitempty"`
}

// ChromeProfile descreve um perfil descoberto no diretório User Data do Chrome.
type ChromeProfile struct {
	Source       string           `json:"source"` // "isolated" ou "system"
	UserDataDir  string           `json:"user_data_dir"`
	Directory    string           `json:"directory"` // ex: "Default", "Profile 2"
	Name         string           `json:"name"`
	PrimaryEmail string           `json:"primary_email,omitempty"`
	FullName     string           `json:"full_name,omitempty"`
	HostedDomain string           `json:"hosted_domain,omitempty"`
	Kind         ProfileKind      `json:"kind"` // "personal", "workspace", "ghost"
	Accounts     []ProfileAccount `json:"accounts,omitempty"`
	IsLastUsed   bool             `json:"is_last_used"`
	IsActive     bool             `json:"is_active"`
	ActiveTime   float64          `json:"active_time,omitempty"`
}

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

var consumerEmailDomains = map[string]bool{
	"gmail.com":      true,
	"googlemail.com": true,
	"hotmail.com":    true,
	"outlook.com":    true,
	"live.com":       true,
	"msn.com":        true,
	"yahoo.com":      true,
	"yahoo.com.br":   true,
	"icloud.com":     true,
	"me.com":         true,
	"proton.me":      true,
	"protonmail.com": true,
	"uol.com.br":     true,
	"bol.com.br":     true,
	"terra.com.br":   true,
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

type localStateFile struct {
	Profile struct {
		LastUsed           string                         `json:"last_used"`
		LastActiveProfiles []string                       `json:"last_active_profiles"`
		InfoCache          map[string]localStateInfoEntry `json:"info_cache"`
	} `json:"profile"`
}

type localStateInfoEntry struct {
	Name         string  `json:"name"`
	UserName     string  `json:"user_name"`
	GaiaName     string  `json:"gaia_name"`
	HostedDomain string  `json:"hosted_domain"`
	ActiveTime   float64 `json:"active_time"`
}

type preferencesFile struct {
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
	AccountInfo []struct {
		Email        string `json:"email"`
		FullName     string `json:"full_name"`
		HostedDomain string `json:"hosted_domain"`
	} `json:"account_info"`
}

// DiscoverProfiles inspeciona um diretório User Data do Chrome e retorna todos os perfis classificados.
func DiscoverProfiles(userDataDir string, sourceLabel string) ([]ChromeProfile, error) {
	if userDataDir == "" {
		return nil, fmt.Errorf("diretório User Data vazio")
	}
	if _, err := os.Stat(userDataDir); err != nil {
		return nil, err
	}

	var ls localStateFile
	lsPath := filepath.Join(userDataDir, "Local State")
	if data, err := os.ReadFile(lsPath); err == nil {
		_ = json.Unmarshal(data, &ls)
	}

	activeSet := make(map[string]bool)
	for _, a := range ls.Profile.LastActiveProfiles {
		activeSet[a] = true
	}

	seenDirs := make(map[string]bool)
	var dirs []string
	for dir := range ls.Profile.InfoCache {
		seenDirs[dir] = true
		dirs = append(dirs, dir)
	}

	// Também descobre diretórios Default / Profile * no disco caso Local State esteja incompleto
	if entries, err := os.ReadDir(userDataDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if name == "Default" || strings.HasPrefix(name, "Profile ") {
				if !seenDirs[name] {
					if _, err := os.Stat(filepath.Join(userDataDir, name, "Preferences")); err == nil {
						seenDirs[name] = true
						dirs = append(dirs, name)
					}
				}
			}
		}
	}

	var profiles []ChromeProfile
	for _, dir := range dirs {
		entry := ls.Profile.InfoCache[dir]
		prefs := readPreferences(filepath.Join(userDataDir, dir, "Preferences"))

		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = strings.TrimSpace(prefs.Profile.Name)
		}
		if name == "" {
			name = dir
		}

		primaryEmail := strings.TrimSpace(entry.UserName)
		fullName := strings.TrimSpace(entry.GaiaName)
		hostedDomain := normalizeHostedDomain(entry.HostedDomain)

		var accounts []ProfileAccount
		for idx, acc := range prefs.AccountInfo {
			email := strings.TrimSpace(acc.Email)
			if email == "" {
				continue
			}
			accDomain := normalizeHostedDomain(acc.HostedDomain)
			if accDomain == "" {
				accDomain = inferOrganizationDomain(email)
			}
			accounts = append(accounts, ProfileAccount{
				Index:        idx,
				Email:        email,
				FullName:     strings.TrimSpace(acc.FullName),
				HostedDomain: accDomain,
			})
			if primaryEmail == "" && idx == 0 {
				primaryEmail = email
			}
			if fullName == "" && acc.FullName != "" {
				fullName = strings.TrimSpace(acc.FullName)
			}
			if hostedDomain == "" && accDomain != "" {
				hostedDomain = accDomain
			}
		}

		if hostedDomain == "" && primaryEmail != "" {
			hostedDomain = inferOrganizationDomain(primaryEmail)
		}

		kind := classifyProfile(primaryEmail, hostedDomain, accounts)

		profiles = append(profiles, ChromeProfile{
			Source:       sourceLabel,
			UserDataDir:  userDataDir,
			Directory:    dir,
			Name:         name,
			PrimaryEmail: primaryEmail,
			FullName:     fullName,
			HostedDomain: hostedDomain,
			Kind:         kind,
			Accounts:     accounts,
			IsLastUsed:   ls.Profile.LastUsed == dir,
			IsActive:     activeSet[dir],
			ActiveTime:   entry.ActiveTime,
		})
	}

	sortProfiles(profiles)
	return profiles, nil
}

func readPreferences(path string) preferencesFile {
	var prefs preferencesFile
	data, err := os.ReadFile(path)
	if err != nil {
		return prefs
	}
	_ = json.Unmarshal(data, &prefs)
	return prefs
}

func normalizeHostedDomain(d string) string {
	d = strings.TrimSpace(strings.ToLower(d))
	if d == "" || d == "no_hosted_domain" {
		return ""
	}
	return d
}

func inferOrganizationDomain(email string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(email)), "@")
	if len(parts) != 2 || parts[1] == "" {
		return ""
	}
	domain := parts[1]
	if consumerEmailDomains[domain] {
		return ""
	}
	return domain
}

func classifyProfile(primaryEmail, hostedDomain string, accounts []ProfileAccount) ProfileKind {
	if primaryEmail == "" && len(accounts) == 0 {
		return ProfileKindGhost
	}
	if hostedDomain != "" {
		return ProfileKindWorkspace
	}
	for _, a := range accounts {
		if a.HostedDomain != "" {
			return ProfileKindWorkspace
		}
	}
	return ProfileKindPersonal
}

func sortProfiles(profiles []ChromeProfile) {
	kindRank := func(k ProfileKind) int {
		switch k {
		case ProfileKindPersonal:
			return 0
		case ProfileKindWorkspace:
			return 1
		default:
			return 2
		}
	}

	sort.Slice(profiles, func(i, j int) bool {
		// 1. Perfis autenticados vêm sempre antes de perfis fantasmas
		ri, rj := kindRank(profiles[i].Kind), kindRank(profiles[j].Kind)
		if (ri == 2) != (rj == 2) {
			return ri < rj
		}
		// 2. Default vem primeiro entre autenticados se IsLastUsed
		if profiles[i].IsLastUsed != profiles[j].IsLastUsed {
			return profiles[i].IsLastUsed
		}
		// 3. Mais recentemente ativo
		if profiles[i].ActiveTime != profiles[j].ActiveTime {
			return profiles[i].ActiveTime > profiles[j].ActiveTime
		}
		return profiles[i].Directory < profiles[j].Directory
	})
}

// ResolveProfile resolve um seletor humano (diretório, email, domínio, nome ou tipo "workspace"/"personal")
// para o melhor perfil autenticado disponível, ignorando perfis fantasmas sempre que houver um autenticado.
func ResolveProfile(userDataDir string, selector string) (*ChromeProfile, error) {
	profiles, err := DiscoverProfiles(userDataDir, "isolated")
	if err != nil {
		return nil, err
	}
	return MatchProfileSelector(profiles, selector)
}

// MatchProfileSelector filtra uma lista de perfis a partir de um seletor livre.
func MatchProfileSelector(profiles []ChromeProfile, selector string) (*ChromeProfile, error) {
	if len(profiles) == 0 {
		return nil, fmt.Errorf("nenhum perfil encontrado")
	}

	sel := strings.TrimSpace(strings.ToLower(selector))
	if sel == "" {
		// Retorna o primeiro autenticado (ou o primeiro da lista)
		return &profiles[0], nil
	}

	// Normaliza aliases de tipo
	switch sel {
	case "work", "org", "corp", "edu", "managed":
		sel = string(ProfileKindWorkspace)
	case "default_personal", "home":
		sel = string(ProfileKindPersonal)
	}

	var exactAuth []ChromeProfile
	var partialAuth []ChromeProfile
	var ghostMatches []ChromeProfile

	for _, p := range profiles {
		isExact := strings.EqualFold(p.Directory, selector) ||
			strings.EqualFold(p.PrimaryEmail, selector) ||
			strings.EqualFold(p.HostedDomain, selector) ||
			strings.EqualFold(string(p.Kind), sel)

		isPartial := strings.Contains(strings.ToLower(p.Directory), sel) ||
			strings.Contains(strings.ToLower(p.Name), sel) ||
			strings.Contains(strings.ToLower(p.PrimaryEmail), sel) ||
			strings.Contains(strings.ToLower(p.HostedDomain), sel)

		if !isExact && !isPartial {
			for _, acc := range p.Accounts {
				if strings.EqualFold(acc.Email, selector) || strings.EqualFold(acc.HostedDomain, selector) {
					isExact = true
					break
				}
				if strings.Contains(strings.ToLower(acc.Email), sel) || strings.Contains(strings.ToLower(acc.HostedDomain), sel) {
					isPartial = true
				}
			}
		}

		if !isExact && !isPartial {
			continue
		}

		if p.Kind == ProfileKindGhost {
			ghostMatches = append(ghostMatches, p)
			continue
		}

		if isExact {
			exactAuth = append(exactAuth, p)
		} else {
			partialAuth = append(partialAuth, p)
		}
	}

	if len(exactAuth) > 0 {
		return &exactAuth[0], nil
	}
	if len(partialAuth) > 0 {
		return &partialAuth[0], nil
	}
	if len(ghostMatches) > 0 {
		return &ghostMatches[0], nil
	}

	return nil, fmt.Errorf("nenhum perfil corresponde ao seletor %q", selector)
}

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

// PruneIsolatedGhostProfiles remove diretórios de perfis fantasmas (sem conta logada e sem Bookmarks)
// exclusivamente dentro do diretório de perfil isolado do agyo.
func PruneIsolatedGhostProfiles(isolatedUserDataDir string) ([]string, error) {
	profiles, err := DiscoverProfiles(isolatedUserDataDir, "isolated")
	if err != nil {
		return nil, nil // Se não existir ainda, nada a limpar
	}

	var pruned []string
	for _, p := range profiles {
		if p.Kind != ProfileKindGhost || p.Directory == "Default" || !strings.HasPrefix(p.Directory, "Profile ") {
			continue
		}
		targetDir := filepath.Join(isolatedUserDataDir, p.Directory)
		// Segurança extra: preserva caso exista arquivo Bookmarks com favoritos reais do usuário
		if hasUserBookmarks(filepath.Join(targetDir, "Bookmarks")) {
			continue
		}
		if err := os.RemoveAll(targetDir); err == nil {
			pruned = append(pruned, p.Directory)
		}
	}

	if len(pruned) > 0 {
		lsPath := filepath.Join(isolatedUserDataDir, "Local State")
		if data, err := os.ReadFile(lsPath); err == nil {
			var raw map[string]interface{}
			if json.Unmarshal(data, &raw) == nil {
				if profMap, ok := raw["profile"].(map[string]interface{}); ok {
					if cacheMap, ok := profMap["info_cache"].(map[string]interface{}); ok {
						for _, dir := range pruned {
							delete(cacheMap, dir)
						}
						if out, mErr := json.MarshalIndent(raw, "", "  "); mErr == nil {
							_ = os.WriteFile(lsPath, out, 0600)
						}
					}
				}
			}
		}
	}

	return pruned, nil
}

func hasUserBookmarks(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), `"url"`)
}
