package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

var consumerEmailDomains = map[string]bool{
	"gmail.com": true, "googlemail.com": true, "hotmail.com": true, "outlook.com": true,
	"live.com": true, "msn.com": true, "yahoo.com": true, "yahoo.com.br": true,
	"icloud.com": true, "me.com": true, "proton.me": true, "protonmail.com": true,
	"uol.com.br": true, "bol.com.br": true, "terra.com.br": true,
}

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
	if data, err := os.ReadFile(filepath.Join(userDataDir, "Local State")); err == nil {
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
			name := e.Name()
			if !e.IsDir() || seenDirs[name] || (name != "Default" && !strings.HasPrefix(name, "Profile ")) {
				continue
			}
			if _, err := os.Stat(filepath.Join(userDataDir, name, "Preferences")); err == nil {
				seenDirs[name] = true
				dirs = append(dirs, name)
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

		profiles = append(profiles, ChromeProfile{
			Source:       sourceLabel,
			UserDataDir:  userDataDir,
			Directory:    dir,
			Name:         name,
			PrimaryEmail: primaryEmail,
			FullName:     fullName,
			HostedDomain: hostedDomain,
			Kind:         classifyProfile(primaryEmail, hostedDomain, accounts),
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
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &prefs)
	}
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
	if len(parts) != 2 || parts[1] == "" || consumerEmailDomains[parts[1]] {
		return ""
	}
	return parts[1]
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

var profileKindRank = map[ProfileKind]int{ProfileKindPersonal: -2, ProfileKindWorkspace: -1}

func sortProfiles(profiles []ChromeProfile) {
	sort.Slice(profiles, func(i, j int) bool {
		// 1. Perfis autenticados vêm sempre antes de perfis fantasmas
		ri, rj := profileKindRank[profiles[i].Kind], profileKindRank[profiles[j].Kind]
		if (ri == 0) != (rj == 0) {
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

	var exactAuth, partialAuth, ghostMatches []ChromeProfile
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

	for _, list := range [][]ChromeProfile{exactAuth, partialAuth, ghostMatches} {
		if len(list) > 0 {
			return &list[0], nil
		}
	}

	return nil, fmt.Errorf("nenhum perfil corresponde ao seletor %q", selector)
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

	if len(pruned) == 0 {
		return pruned, nil
	}
	lsPath := filepath.Join(isolatedUserDataDir, "Local State")
	var raw map[string]interface{}
	if data, err := os.ReadFile(lsPath); err != nil || json.Unmarshal(data, &raw) != nil {
		return pruned, nil
	}
	profMap, ok := raw["profile"].(map[string]interface{})
	if !ok {
		return pruned, nil
	}
	cacheMap, ok := profMap["info_cache"].(map[string]interface{})
	if !ok {
		return pruned, nil
	}
	for _, dir := range pruned {
		delete(cacheMap, dir)
	}
	if out, mErr := json.MarshalIndent(raw, "", "  "); mErr == nil {
		_ = os.WriteFile(lsPath, out, 0600)
	}
	return pruned, nil
}

func hasUserBookmarks(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(data), `"url"`)
}
