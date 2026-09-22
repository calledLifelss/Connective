package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// shortcutTargets are the .lnk files the installer may have created.
// Only existing links are rewritten — a user who declined the desktop
// icon must not gain one from an update.
func shortcutTargets() []string {
	var dirs []string
	if pd := os.Getenv("ProgramData"); pd != "" {
		dirs = append(dirs, filepath.Join(pd, `Microsoft\Windows\Start Menu\Programs\Connective`))
	}
	if pub := os.Getenv("PUBLIC"); pub != "" {
		dirs = append(dirs, filepath.Join(pub, "Desktop"))
	}
	if app := os.Getenv("APPDATA"); app != "" {
		dirs = append(dirs, filepath.Join(app, `Microsoft\Windows\Start Menu\Programs\Connective`))
	}
	if up := os.Getenv("USERPROFILE"); up != "" {
		dirs = append(dirs, filepath.Join(up, "Desktop"))
	}
	var out []string
	for _, d := range dirs {
		out = append(out, filepath.Join(d, "Connective.lnk"))
	}
	return out
}

// rewriteShortcuts repoints existing Connective shortcuts at the new
// versioned exe via WScript.Shell (no new links are created).
func rewriteShortcuts(root, version string) error {
	exe := filepath.Join(root, "versions", version, "connective.exe")
	var missing []string
	for _, lnk := range shortcutTargets() {
		if _, err := os.Stat(lnk); err != nil {
			continue
		}
		if err := rewriteOneShortcut(lnk, exe); err != nil {
			missing = append(missing, lnk+": "+err.Error())
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("update: shortcut rewrite: %v", missing)
	}
	return nil
}

func rewriteOneShortcut(lnk, exe string) error {
	script := fmt.Sprintf(
		`$s=(New-Object -COM WScript.Shell).CreateShortcut(%s);$s.TargetPath=%s;$s.WorkingDirectory=%s;$s.IconLocation=%s;$s.Save()`,
		psStr(lnk), psStr(exe), psStr(filepath.Dir(exe)), psStr(exe))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "powershell",
		"-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", oneLine(string(out)), err)
	}
	return nil
}

func psStr(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func oneLine(s string) string {
	for i, r := range s {
		if r == '\n' || r == '\r' {
			return s[:i]
		}
	}
	return s
}
