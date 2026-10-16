package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/tiagovilasboas/antigravity-operator/internal/platform"
	"github.com/tiagovilasboas/antigravity-operator/internal/profile"
)

func browserUsageString() string {
	return `Usage: agyo browser <subcommand> [options]

Subcommands:
  start [--profile <p>] [--headless] [--port <n>] Launch isolated Chrome with DevTools debugging flags
  status [--json]                                 Inspect DevTools port (9222) readiness and Chrome process PID
  profiles [--isolated|--system|--all] [--json]   Discover and classify Chrome profiles (personal, workspace, ghost)
  resolve <url-or-domain> [--json]                Resolve best authenticated profile and authuser URL for a target
  stop                                            Gracefully terminate isolated Chrome process
  tabs [--json]                                   List all open page tabs and target IDs in isolated Chrome
  open [--new-tab] <url>                          Open or reuse tab at the given URL in isolated Chrome
  close <tab-id>                                  Close a specific tab by ID
  eval [tab-id] "<js>"                            Evaluate JavaScript expression in active or target tab via CDP
  shot [filename]                                 Capture PNG screenshot of active tab (default: screenshot.png)

Examples:
  agyo browser profiles
  agyo browser resolve https://docs.google.com/forms/d/e/1FAIpQLS/viewform
  agyo browser start --profile workspace
  agyo browser status --json
  agyo browser tabs --json
  agyo browser shot auth-screen.png

Run 'agyo browser <subcommand> --help' for details on each subcommand.`
}

func printBrowserUsage() {
	fmt.Println(browserUsageString())
}

func collectRequestedProfiles(info *platform.Info, scope string) []profile.ChromeProfile {
	var all []profile.ChromeProfile
	if scope == "isolated" || scope == "all" {
		if iso, err := profile.DiscoverProfiles(info.BrowserProfile, "isolated"); err == nil {
			all = append(all, iso...)
		}
	}
	if scope == "system" || scope == "all" {
		if sys, err := profile.DiscoverProfiles(info.SystemChromeDir, "system"); err == nil {
			all = append(all, sys...)
		}
	}
	return all
}

func runBrowser(info *platform.Info, args []string) {
	if len(args) == 0 {
		printBrowserUsage()
		exitFunc(ExitUsageError)
		return
	}

	sub := args[0]
	if isHelp(sub) {
		printBrowserUsage()
		return
	}

	switch sub {
	case "status":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo browser status [--json]")
			fmt.Println("Inspect DevTools port (9222) readiness and Chrome process PID.")
			return
		}
		st := profile.CheckStatus(info.BrowserProfile)
		if hasJSON(args[1:]) {
			data, _ := json.MarshalIndent(st, "", "  ")
			fmt.Println(string(data))
			return
		}
		if st.IsRunning {
			pidInfo := ""
			if st.PID > 0 {
				pidInfo = fmt.Sprintf(" [PID: %d]", st.PID)
			}
			fmt.Printf("✅ Chrome DevTools ACTIVE on port %d (%s)%s\n", st.Port, st.Version, pidInfo)
			fmt.Printf("   Isolated profile: %s\n", st.ProfileDir)
			if st.ActiveProfileDir != "" {
				fmt.Printf("   Active directory: %s\n", st.ActiveProfileDir)
			}
		} else {
			fmt.Printf("⚠️  Chrome DevTools INACTIVE on port %d\n", st.Port)
			fmt.Printf("   To launch, run: agyo browser start\n")
		}

	case "profiles":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo browser profiles [--isolated|--system|--all] [--json]")
			fmt.Println("Discover and classify Chrome profiles (personal, workspace, ghost) without hardcoding.")
			return
		}
		profCmd := flag.NewFlagSet("browser profiles", flag.ExitOnError)
		onlyIso := profCmd.Bool("isolated", false, "Show only isolated agyo Chrome profiles")
		onlySys := profCmd.Bool("system", false, "Show only system Chrome profiles")
		showAll := profCmd.Bool("all", false, "Show both isolated and system Chrome profiles")
		jsonOut := profCmd.Bool("json", false, "Output in JSON format")
		_ = profCmd.Parse(args[1:])

		scope := "all"
		if *onlyIso && !*onlySys && !*showAll {
			scope = "isolated"
		} else if *onlySys && !*onlyIso && !*showAll {
			scope = "system"
		}

		profiles := collectRequestedProfiles(info, scope)
		if *jsonOut || hasJSON(args[1:]) {
			if profiles == nil {
				profiles = []profile.ChromeProfile{}
			}
			data, _ := json.MarshalIndent(profiles, "", "  ")
			fmt.Println(string(data))
			return
		}

		if len(profiles) == 0 {
			fmt.Println("ℹ️  No Chrome profiles found.")
			return
		}

		fmt.Printf("👥 Discovered Chrome Profiles (%d total):\n", len(profiles))
		fmt.Println("--------------------------------------------------------------------------------")
		ghostCount := 0
		for _, p := range profiles {
			var badge string
			switch p.Kind {
			case profile.ProfileKindWorkspace:
				badge = fmt.Sprintf("🏢 WORKSPACE [%s]", p.HostedDomain)
			case profile.ProfileKindPersonal:
				badge = "👤 PERSONAL"
			default:
				badge = "👻 GHOST (unauthenticated)"
				ghostCount++
			}
			activeTag := ""
			if p.IsLastUsed {
				activeTag = " ★ last-used"
			}
			emailInfo := p.PrimaryEmail
			if emailInfo == "" {
				emailInfo = "(no signed-in account)"
			}
			fmt.Printf("[%s] %-11s | %-26s | %s%s\n", p.Source, p.Directory, badge, emailInfo, activeTag)
		}
		fmt.Println("--------------------------------------------------------------------------------")
		if ghostCount > 0 {
			fmt.Printf("💡 Note: %d ghost/unauthenticated profile(s) detected. 'agyo' automatically ignores them when resolving profiles.\n", ghostCount)
		}

	case "resolve":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo browser resolve <url-or-domain> [--json]")
			fmt.Println("Resolve the best authenticated profile and authuser URL for a given target URL or domain.")
			return
		}
		var target string
		for _, a := range args[1:] {
			if !strings.HasPrefix(a, "-") {
				target = a
				break
			}
		}
		if target == "" {
			usageError("missing URL or domain\nUsage: agyo browser resolve <url-or-domain> [--json]")
			return
		}

		// Prefer isolated profiles if they have workspace/personal accounts, fallback to all
		profiles := collectRequestedProfiles(info, "isolated")
		if len(profiles) == 0 {
			profiles = collectRequestedProfiles(info, "system")
		} else {
			// Also include system profiles if isolated lacks workspace accounts present in system
			sysProfiles := collectRequestedProfiles(info, "system")
			profiles = append(profiles, sysProfiles...)
		}

		res, err := profile.ResolveURLForProfiles(profiles, target)
		if err != nil {
			runtimeError(err, "")
			return
		}

		if hasJSON(args[1:]) {
			data, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(data))
			return
		}

		fmt.Printf("🧭 Profile Resolution for: %s\n", res.InputURL)
		fmt.Println("--------------------------------------------------------------------------------")
		if res.Recommended != nil {
			fmt.Printf("   Recommended Profile : [%s] %s (%s)\n", res.Recommended.Source, res.Recommended.Directory, res.Recommended.Kind)
			if res.Recommended.PrimaryEmail != "" {
				fmt.Printf("   Account Email       : %s\n", res.Recommended.PrimaryEmail)
			}
			if res.Recommended.HostedDomain != "" {
				fmt.Printf("   Organization Domain : %s\n", res.Recommended.HostedDomain)
			}
			fmt.Printf("   Matched By          : %s\n", res.MatchedBy)
			fmt.Printf("   Suggested URL       : %s\n", res.SuggestedURL)
			fmt.Printf("   Launch Command      : %s\n", res.StartCommand)
		}
		fmt.Printf("   Guidance            : %s\n", res.Hint)
		fmt.Println("--------------------------------------------------------------------------------")

	case "start":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo browser start [--profile <id|email|domain|kind>] [--headless] [--port <p>]")
			fmt.Println("Launch isolated Chrome with remote debugging flags.")
			return
		}
		browserCmd := flag.NewFlagSet("browser start", flag.ExitOnError)
		headless := browserCmd.Bool("headless", false, "Force headless mode even with display available")
		port := browserCmd.Int("port", profile.DefaultDebugPort, "Chrome remote debugging port")
		profSel := browserCmd.String("profile", "", "Profile selector (directory, email, domain, 'workspace' or 'personal')")
		_ = browserCmd.Parse(args[1:])

		fmt.Printf("🚀 Launching isolated Chrome for Google Antigravity on port %d...\n", *port)
		err := profile.Start(info, profile.StartOptions{
			Port:            *port,
			ForceHeadless:   *headless,
			ProfileSelector: *profSel,
		})
		if err != nil {
			runtimeError(err, "Check if Chrome is installed or if the port is already in use")
			return
		}
		fmt.Printf("✅ Isolated Chrome active on port %d!\n", *port)

	case "stop":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo browser stop")
			fmt.Println("Gracefully terminate isolated Chrome process.")
			return
		}
		fmt.Println("🛑 Terminating isolated Chrome instance...")
		err := profile.Stop(info)
		if err != nil {
			runtimeError(err, "")
			return
		}
		fmt.Println("✅ Isolated Chrome terminated successfully.")

	case "tabs":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo browser tabs [--json]")
			fmt.Println("List all open page tabs and target IDs in isolated Chrome.")
			return
		}
		tabs, err := profile.ListTabs(profile.DefaultDebugPort)
		if err != nil {
			runtimeError(err, "")
			return
		}
		if hasJSON(args[1:]) {
			if tabs == nil {
				tabs = []profile.Tab{}
			}
			data, _ := json.MarshalIndent(tabs, "", "  ")
			fmt.Println(string(data))
			return
		}
		if len(tabs) == 0 {
			fmt.Println("ℹ️  No open page tabs found in Chrome.")
			return
		}
		fmt.Printf("🌐 Open Chrome Tabs (%d active):\n", len(tabs))
		fmt.Println("-----------------------------------------------------------------")
		for _, t := range tabs {
			idPrefix := t.ID
			if len(idPrefix) > 8 {
				idPrefix = idPrefix[:8]
			}
			fmt.Printf("[%s] %-35s : %s\n", idPrefix, t.Title, t.URL)
		}
		fmt.Println("-----------------------------------------------------------------")

	case "open":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo browser open [--new-tab] <url>")
			fmt.Println("Open or reuse an existing/blank tab at the given URL in isolated Chrome with automatic profile & authuser routing.")
			return
		}
		forceNew := false
		var targetURL string
		for _, a := range args[1:] {
			if a == "--new-tab" || a == "-new-tab" {
				forceNew = true
				continue
			}
			if !strings.HasPrefix(a, "-") && targetURL == "" {
				targetURL = a
			}
		}
		if targetURL == "" {
			usageError("missing URL\nUsage: agyo browser open [--new-tab] <url>")
			return
		}

		// 1. Observe & Resolve URL against discovered profiles
		profiles := collectRequestedProfiles(info, "isolated")
		if len(profiles) == 0 {
			profiles = collectRequestedProfiles(info, "system")
		} else {
			profiles = append(profiles, collectRequestedProfiles(info, "system")...)
		}

		openURL := targetURL
		recommendedDir := ""
		if res, rErr := profile.ResolveURLForProfiles(profiles, targetURL); rErr == nil && res != nil {
			if res.SuggestedURL != "" && res.SuggestedURL != targetURL {
				openURL = res.SuggestedURL
				if res.Recommended != nil {
					fmt.Printf("🧭 Auto-routing URL for %s (%s): %s\n", res.Recommended.Directory, res.Recommended.PrimaryEmail, openURL)
				}
			}
			if res.Recommended != nil && res.Recommended.Source == "isolated" {
				recommendedDir = res.Recommended.Directory
			}
		}

		// 2. Auto-start isolated Chrome with the matched profile if not running
		if !profile.CheckStatus(info.BrowserProfile).IsRunning {
			fmt.Printf("🚀 Auto-starting isolated Chrome on port %d...\n", profile.DefaultDebugPort)
			if sErr := profile.Start(info, profile.StartOptions{
				Port:            profile.DefaultDebugPort,
				ProfileSelector: recommendedDir,
			}); sErr != nil {
				runtimeError(sErr, "")
				return
			}
		}

		// 3. Open or reuse tab
		tab, reused, err := profile.OpenOrReuseTab(profile.DefaultDebugPort, openURL, forceNew)
		if err != nil {
			runtimeError(err, "")
			return
		}
		idPrefix := tab.ID
		if len(idPrefix) > 8 {
			idPrefix = idPrefix[:8]
		}
		if reused {
			fmt.Printf("♻️  Tab reused [%s]: %s\n", idPrefix, openURL)
		} else {
			fmt.Printf("✅ Tab opened [%s]: %s\n", idPrefix, openURL)
		}

		// 4. Observe post-navigation state for Auth/VPN walls
		if wall, reason := profile.DetectAuthWall(tab.Title, tab.URL); wall {
			fmt.Printf("⚠️  [Auth Sensor] %s\n", reason)
		}

	case "close":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo browser close <tab-id>")
			fmt.Println("Close a specific tab by ID.")
			return
		}
		if len(args) < 2 {
			usageError("missing tab ID\nUsage: agyo browser close <tab-id>")
			return
		}
		tabID := args[1]
		if err := profile.CloseTab(profile.DefaultDebugPort, tabID); err != nil {
			runtimeError(err, "")
			return
		}
		fmt.Printf("✅ Tab [%s] closed successfully.\n", tabID)

	case "eval":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo browser eval [tab-id] \"<javascript>\"")
			fmt.Println("Evaluate JavaScript expression in active or target tab via CDP.")
			return
		}
		if len(args) < 2 {
			usageError("missing JavaScript expression\nUsage: agyo browser eval [tab-id] \"<javascript>\"")
			return
		}
		var val string
		var err error
		if len(args) >= 3 {
			tabID := args[1]
			expr := strings.Join(args[2:], " ")
			val, err = profile.EvalTab(profile.DefaultDebugPort, tabID, expr)
		} else {
			expr := args[1]
			val, err = profile.Eval(profile.DefaultDebugPort, expr)
		}
		if err != nil {
			runtimeError(err, "")
			return
		}
		fmt.Println(val)

	case "shot", "screenshot":
		if hasHelp(args[1:]) {
			fmt.Println("Usage: agyo browser shot [filename]")
			fmt.Println("Capture PNG screenshot of active tab (default: screenshot.png).")
			return
		}
		dest := "screenshot.png"
		if len(args) > 1 {
			dest = args[1]
		}
		if err := profile.Screenshot(profile.DefaultDebugPort, dest); err != nil {
			runtimeError(err, "")
			return
		}
		fmt.Printf("📸 Screenshot saved successfully to: %s\n", dest)

	default:
		usageError("unknown browser subcommand %q\nRun 'agyo browser --help' for usage.", sub)
	}
}
