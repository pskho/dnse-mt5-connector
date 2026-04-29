package setup

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dnse-mt5-connector/internal/logger"
)

// EnsureDirectories creates required data and log directories if they don't exist.
func EnsureDirectories() error {
	dirs := []string{"data", "logs", "config"}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

type MT5Installation struct {
	ID   string
	Path string
}

// DetectMT5Folders scans the standard Windows AppData folder for MT5 installations.
func DetectMT5Folders() ([]MT5Installation, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return nil, fmt.Errorf("APPDATA environment variable not set")
	}

	terminalPath := filepath.Join(appData, "MetaQuotes", "Terminal")
	entries, err := os.ReadDir(terminalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No MT5 installed at standard location
		}
		return nil, err
	}

	var installations []MT5Installation
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// MT5 uses 32-character hex strings for installation folders
		if len(entry.Name()) == 32 {
			mql5Path := filepath.Join(terminalPath, entry.Name(), "MQL5")
			if stat, err := os.Stat(mql5Path); err == nil && stat.IsDir() {
				installations = append(installations, MT5Installation{
					ID:   entry.Name(),
					Path: filepath.Join(terminalPath, entry.Name()),
				})
			}
		}
	}
	return installations, nil
}

// InstallFiles copies the DLL and EA to the target MT5 installation folder.
func InstallFiles(mt5Path string, appLog *logger.FileLogger) ([]string, error) {
	appLog.Info("setup_install_started", map[string]any{"path": mt5Path})
	var logs []string

	dllSrc := filepath.Join("cpp", "build", "Release", "DNSEBridge.dll")
	eaSrc := filepath.Join("mql5", "DNSE_MarketData_Bridge.mq5")

	dllDest := filepath.Join(mt5Path, "MQL5", "Libraries", "DNSEBridge.dll")
	eaFolder := filepath.Join(mt5Path, "MQL5", "Experts", "DNSE")
	eaDest := filepath.Join(eaFolder, "DNSE_MarketData_Bridge.mq5")
	legacyEAFiles := []string{
		filepath.Join(mt5Path, "MQL5", "Experts", "DNSE_MarketData_Bridge.mq5"),
		filepath.Join(mt5Path, "MQL5", "Experts", "DNSE_MarketData_Bridge.ex5"),
	}

	logs = append(logs, "Found MT5 Installation at: " + mt5Path)

	backedUpDll, err := backupAndCopy(dllSrc, dllDest)
	if err != nil {
		appLog.Error("dll_installed_failed", map[string]any{"error": err.Error()})
		return logs, fmt.Errorf("failed to install DLL: %w", err)
	}
	if backedUpDll {
		logs = append(logs, "Backed up existing DNSEBridge.dll to DNSEBridge.dll.bak")
	}
	logs = append(logs, "Successfully copied DNSEBridge.dll to: " + dllDest)
	appLog.Info("dll_installed", map[string]any{"dest": dllDest})

	backedUpEa, err := backupAndCopy(eaSrc, eaDest)
	if err != nil {
		appLog.Error("ea_installed_failed", map[string]any{"error": err.Error()})
		return logs, fmt.Errorf("failed to install EA: %w", err)
	}
	if backedUpEa {
		logs = append(logs, "Backed up existing DNSE_MarketData_Bridge.mq5 to DNSE_MarketData_Bridge.mq5.bak")
	}
	logs = append(logs, "Successfully copied DNSE_MarketData_Bridge.mq5 to: " + eaDest)
	appLog.Info("ea_installed", map[string]any{"dest": eaDest})

	for _, legacyPath := range legacyEAFiles {
		if _, err := os.Stat(legacyPath); err == nil {
			if err := os.Remove(legacyPath); err == nil {
				logs = append(logs, "Removed legacy root EA copy: " + legacyPath)
				appLog.Info("ea_legacy_removed", map[string]any{"dest": legacyPath})
			} else {
				logs = append(logs, "WARNING: Could not remove legacy root EA copy: " + legacyPath)
				appLog.Error("ea_legacy_remove_failed", map[string]any{"dest": legacyPath, "error": err.Error()})
			}
		}
	}

	if metaEditor := findMetaEditor(); metaEditor != "" {
		logs = append(logs, "Found MetaEditor: " + metaEditor)
		if compileLog, err := compileEA(metaEditor, eaDest); err != nil {
			logs = append(logs, "WARNING: Auto-compile failed: " + err.Error())
			appLog.Error("ea_compile_failed", map[string]any{"ea": eaDest, "error": err.Error()})
		} else {
			logs = append(logs, "Auto-compiled EA successfully.")
			if compileLog != "" {
				logs = append(logs, "Compile log: " + compileLog)
			}
			appLog.Info("ea_compiled", map[string]any{"ea": eaDest, "metaEditor": metaEditor})
		}
	} else {
		logs = append(logs, "WARNING: MetaEditor not found. EA copied but not auto-compiled.")
	}

	return logs, nil
}

func backupAndCopy(src, dest string) (bool, error) {
	backedUp := false
	// Check if source exists
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return backedUp, fmt.Errorf("source file missing: %s", src)
	}

	// Backup existing destination file
	if _, err := os.Stat(dest); err == nil {
		backupPath := dest + ".bak"
		_ = os.Rename(dest, backupPath) // Ignore error if backup fails (e.g., file in use)
		backedUp = true
	}

	return backedUp, copyFile(src, dest)
}

func copyFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func findMetaEditor() string {
	for _, name := range []string{"metaeditor64.exe", "metaeditor.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	roots := []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")}
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := strings.ToLower(entry.Name())
			if !strings.Contains(name, "metatrader") && !strings.Contains(name, "metaeditor") && !strings.Contains(name, "dnse") {
				continue
			}
			for _, exe := range []string{"metaeditor64.exe", "metaeditor.exe"} {
				candidate := filepath.Join(root, entry.Name(), exe)
				if _, err := os.Stat(candidate); err == nil {
					return candidate
				}
			}
		}
	}
	return ""
}

func compileEA(metaEditorPath, eaPath string) (string, error) {
	logPath := strings.TrimSuffix(eaPath, filepath.Ext(eaPath)) + ".compile.log"
	args := []string{
		fmt.Sprintf("/compile:%s", eaPath),
		fmt.Sprintf("/log:%s", logPath),
	}
	cmd := exec.Command(metaEditorPath, args...)
	if err := cmd.Run(); err != nil {
		return logPath, err
	}
	return logPath, nil
}

// ExportSupportPackage creates a zip archive containing masked logs and config.
func ExportSupportPackage(configPath, logPath string) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Add masked config
	if configData, err := os.ReadFile(configPath); err == nil {
		maskedConfig := maskConfigSecrets(string(configData))
		if f, err := zipWriter.Create("config.yaml"); err == nil {
			f.Write([]byte(maskedConfig))
		}
	}

	// Add log file (we just take the last 1MB to avoid huge zips)
	if logFile, err := os.Open(logPath); err == nil {
		defer logFile.Close()
		stat, _ := logFile.Stat()
		size := stat.Size()
		var offset int64 = 0
		if size > 1024*1024 {
			offset = size - 1024*1024
		}
		logFile.Seek(offset, 0)
		
		if f, err := zipWriter.Create("app.jsonl"); err == nil {
			io.Copy(f, logFile)
		}
	}

	// Add a system status summary
	if f, err := zipWriter.Create("system_status.json"); err == nil {
		status := map[string]any{
			"os": "windows",
			"timestamp": os.Getenv("DATE"),
		}
		b, _ := json.MarshalIndent(status, "", "  ")
		f.Write(b)
	}

	zipWriter.Close()
	return buf.Bytes(), nil
}

func maskConfigSecrets(config string) string {
	lines := strings.Split(config, "\n")
	for i, line := range lines {
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "api_secret:") || strings.Contains(lowerLine, "trading_token:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				lines[i] = parts[0] + ": \"*** MASKED ***\""
			}
		}
	}
	return strings.Join(lines, "\n")
}
