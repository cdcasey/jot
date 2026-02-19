package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/chris/jot/config"
	"github.com/joho/godotenv"
)

const (
	label     = "com.jot.agent"
	binDest   = "/usr/local/bin/jot"
	plistName = label + ".plist"
)

func plistDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents")
}

func plistPath() string {
	return filepath.Join(plistDir(), plistName)
}

func logDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Logs")
}

func stdoutLogPath() string { return filepath.Join(logDir(), "jot-stdout.log") }
func stderrLogPath() string { return filepath.Join(logDir(), "jot-stderr.log") }

// Install copies the binary to /usr/local/bin, seeds ~/.jot/config from .env
// if needed, generates a launchd plist, and loads it.
func Install() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	// Copy binary to /usr/local/bin
	input, err := os.ReadFile(exe)
	if err != nil {
		return fmt.Errorf("reading binary: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(binDest), 0755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(binDest), err)
	}
	if err := os.WriteFile(binDest, input, 0755); err != nil {
		return fmt.Errorf("copying binary to %s: %w", binDest, err)
	}
	fmt.Printf("installed binary to %s\n", binDest)

	// Seed ~/.jot/config from .env if config doesn't exist yet
	configFile := config.ConfigFile()
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if envData, err := os.ReadFile(".env"); err == nil {
			if err := os.MkdirAll(config.ConfigDir(), 0700); err != nil {
				return fmt.Errorf("creating config dir: %w", err)
			}
			if err := os.WriteFile(configFile, envData, 0600); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}
			fmt.Printf("seeded config from .env -> %s\n", configFile)
		}
	} else {
		fmt.Printf("config already exists at %s\n", configFile)
	}

	// Resolve working directory for the plist.
	// Use the directory containing the database, or fall back to ~/.jot.
	workDir := resolveWorkDir()

	// Generate plist
	plist, err := renderPlist(workDir)
	if err != nil {
		return fmt.Errorf("generating plist: %w", err)
	}

	// Unload existing plist if present (ignore errors)
	if _, err := os.Stat(plistPath()); err == nil {
		_ = launchctl("unload", plistPath())
	}

	// Write plist
	if err := os.MkdirAll(plistDir(), 0755); err != nil {
		return fmt.Errorf("creating LaunchAgents dir: %w", err)
	}
	if err := os.WriteFile(plistPath(), []byte(plist), 0644); err != nil {
		return fmt.Errorf("writing plist: %w", err)
	}
	fmt.Printf("wrote plist to %s\n", plistPath())

	// Load
	if err := launchctl("load", plistPath()); err != nil {
		return fmt.Errorf("loading plist: %w", err)
	}
	fmt.Println("service loaded and will start on login")
	return nil
}

// resolveWorkDir determines the working directory for the launchd service.
// It reads the config to find DATABASE_PATH; if it's relative, we use the
// current directory. Otherwise we use ~/.jot.
func resolveWorkDir() string {
	// Read the config that will actually be used at runtime
	envVars, _ := godotenv.Read(config.ConfigFile())
	if dbPath, ok := envVars["DATABASE_PATH"]; ok && !filepath.IsAbs(dbPath) {
		// Relative database path — working directory matters.
		// Use current directory (where the user ran install from).
		if wd, err := os.Getwd(); err == nil {
			return wd
		}
	}
	return config.ConfigDir()
}

// Uninstall unloads the plist, removes it, and removes the binary.
func Uninstall() error {
	if _, err := os.Stat(plistPath()); err == nil {
		if err := launchctl("unload", plistPath()); err != nil {
			fmt.Fprintf(os.Stderr, "warning: unload failed: %v\n", err)
		}
		if err := os.Remove(plistPath()); err != nil {
			return fmt.Errorf("removing plist: %w", err)
		}
		fmt.Printf("removed %s\n", plistPath())
	} else {
		fmt.Println("plist not found, skipping")
	}

	if _, err := os.Stat(binDest); err == nil {
		if err := os.Remove(binDest); err != nil {
			return fmt.Errorf("removing binary: %w", err)
		}
		fmt.Printf("removed %s\n", binDest)
	} else {
		fmt.Println("binary not found in /usr/local/bin, skipping")
	}

	fmt.Println("uninstalled")
	return nil
}

func Start() error {
	return launchctl("start", label)
}

func Stop() error {
	return launchctl("stop", label)
}

func Restart() error {
	_ = Stop()
	return Start()
}

func Status() error {
	cmd := exec.Command("launchctl", "list", label)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("service is not loaded")
		return nil
	}
	return nil
}

// Logs tails both stdout and stderr log files.
func Logs() error {
	cmd := exec.Command("tail", "-f", stdoutLogPath(), stderrLogPath())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func launchctl(args ...string) error {
	cmd := exec.Command("launchctl", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launchctl %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return nil
}

var plistTemplate = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.BinPath}}</string>
		<string>run</string>
	</array>
	<key>WorkingDirectory</key>
	<string>{{.WorkDir}}</string>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>{{.StdoutLog}}</string>
	<key>StandardErrorPath</key>
	<string>{{.StderrLog}}</string>
</dict>
</plist>
`))

type plistData struct {
	Label     string
	BinPath   string
	WorkDir   string
	StdoutLog string
	StderrLog string
}

func renderPlist(workDir string) (string, error) {
	var buf bytes.Buffer
	err := plistTemplate.Execute(&buf, plistData{
		Label:     label,
		BinPath:   binDest,
		WorkDir:   workDir,
		StdoutLog: stdoutLogPath(),
		StderrLog: stderrLogPath(),
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
