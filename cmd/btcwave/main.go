package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/btcwave/btcwave-cli/internal/config"
	"github.com/btcwave/btcwave-cli/internal/detect"
	"github.com/btcwave/btcwave-cli/internal/state"
)

var version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	jsonMode := hasFlag("--json")

	switch os.Args[1] {
	case "setup":
		runSetup(jsonMode)
	case "status":
		runStatus(jsonMode)
	case "doctor":
		runDoctor(jsonMode)
	case "version":
		fmt.Printf("btcwave %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func hasFlag(flag string) bool {
	for _, a := range os.Args {
		if a == flag {
			return true
		}
	}
	return false
}

func flagValue(flag string) string {
	for i, a := range os.Args {
		if a == flag && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

func printUsage() {
	fmt.Println(`btcwave — Bitcoin Wave node installer and manager

Usage:
  btcwave setup [--key KEY] [--target HOST] [--json]
  btcwave status [--json]
  btcwave doctor [--json]
  btcwave version

Commands:
  setup    Install and configure a Bitcoin Wave node
  status   Show node sync progress and health
  doctor   Run diagnostics on the current installation
  version  Print version

Flags:
  --key KEY       License key (e.g. WAVE-FULL-7K3M-XXXX)
  --target HOST   Target machine (default: localhost)
  --json          Machine-readable JSON output`)
}

func runSetup(jsonMode bool) {
	key := flagValue("--key")
	target := flagValue("--target")
	if target == "" {
		target = "localhost"
	}

	s, err := state.Load()
	if err != nil {
		s = state.New()
	}

	if s.Phase != state.PhaseComplete {
		if !jsonMode {
			fmt.Println("Bitcoin Wave — Node Setup")
			fmt.Println("========================")
			fmt.Println()
		}
	}

	if s.Phase == state.PhaseNew {
		if key == "" {
			fmt.Fprintln(os.Stderr, "error: --key is required for initial setup")
			os.Exit(1)
		}
		s.LicenseKey = key
		s.Target = target
		s.Phase = state.PhaseDetect
		s.Save()
	}

	if s.Phase == state.PhaseDetect {
		if !jsonMode {
			fmt.Println("Detecting hardware...")
		}
		hw, err := detect.Detect(target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error detecting hardware: %v\n", err)
			os.Exit(1)
		}
		s.Hardware = hw
		s.Phase = state.PhaseConfig
		s.Save()

		if jsonMode {
			b, _ := json.Marshal(hw)
			fmt.Println(string(b))
		} else {
			fmt.Printf("  CPU:    %s (%d cores)\n", hw.Arch, hw.Cores)
			fmt.Printf("  RAM:    %d MB\n", hw.MemoryMB)
			fmt.Printf("  Disk:   %d GB free\n", hw.DiskFreeGB)
			fmt.Println()

			if hw.DiskFreeGB < 700 {
				fmt.Println("WARNING: Less than 700GB free. Full node requires ~850GB.")
				fmt.Println("         Consider a larger drive or pruned mode (post-v1).")
				fmt.Println()
			}
		}
	}

	if s.Phase == state.PhaseConfig {
		if !jsonMode {
			fmt.Println("Generating node configuration...")
		}
		conf, err := config.Generate(s.Hardware, s.Target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error generating config: %v\n", err)
			os.Exit(1)
		}
		s.ConfigPath = conf
		s.Phase = state.PhaseInstall
		s.Save()

		if !jsonMode {
			fmt.Printf("  Config written to: %s\n\n", conf)
		}
	}

	if s.Phase == state.PhaseInstall {
		if !jsonMode {
			fmt.Println("Ready to install Bitcoin Knots.")
			fmt.Println("This will download, verify, and install the node binary.")
			fmt.Println()
			fmt.Println("Next: btcwave setup (re-run to continue)")
			fmt.Println("      The CLI is resumable — safe to re-run at any point.")
		}
		if jsonMode {
			b, _ := json.Marshal(s)
			fmt.Println(string(b))
		}
	}
}

func runStatus(jsonMode bool) {
	s, err := state.Load()
	if err != nil {
		if jsonMode {
			fmt.Println(`{"installed":false}`)
		} else {
			fmt.Println("No Bitcoin Wave installation found.")
			fmt.Println("Run: btcwave setup --key YOUR-KEY")
		}
		return
	}

	if jsonMode {
		b, _ := json.Marshal(s)
		fmt.Println(string(b))
	} else {
		fmt.Printf("Bitcoin Wave Node Status\n")
		fmt.Printf("=======================\n")
		fmt.Printf("  Phase:   %s\n", s.Phase)
		fmt.Printf("  Target:  %s\n", s.Target)
		if s.Hardware != nil {
			fmt.Printf("  Arch:    %s\n", s.Hardware.Arch)
			fmt.Printf("  Cores:   %d\n", s.Hardware.Cores)
			fmt.Printf("  Memory:  %d MB\n", s.Hardware.MemoryMB)
			fmt.Printf("  Disk:    %d GB free\n", s.Hardware.DiskFreeGB)
		}
	}
}

func runDoctor(jsonMode bool) {
	checks := []struct {
		Name string
		Fn   func() (string, bool)
	}{
		{"state_file", checkStateFile},
		{"config_file", checkConfigFile},
	}

	type result struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	}
	var results []result

	if !jsonMode {
		fmt.Println("Bitcoin Wave Doctor")
		fmt.Println("===================")
	}

	for _, c := range checks {
		detail, ok := c.Fn()
		status := "pass"
		if !ok {
			status = "fail"
		}
		results = append(results, result{c.Name, status, detail})
		if !jsonMode {
			icon := "OK"
			if !ok {
				icon = "FAIL"
			}
			fmt.Printf("  [%s] %s: %s\n", icon, c.Name, detail)
		}
	}

	if jsonMode {
		b, _ := json.Marshal(results)
		fmt.Println(string(b))
	}
}

func checkStateFile() (string, bool) {
	_, err := state.Load()
	if err != nil {
		return "no state file found", false
	}
	return "state file present", true
}

func checkConfigFile() (string, bool) {
	s, err := state.Load()
	if err != nil || s.ConfigPath == "" {
		return "no config generated yet", false
	}
	if _, err := os.Stat(s.ConfigPath); err != nil {
		return fmt.Sprintf("config missing: %s", s.ConfigPath), false
	}
	return fmt.Sprintf("config at %s", s.ConfigPath), true
}
