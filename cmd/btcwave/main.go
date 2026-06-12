package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/btcwave/btcwave-cli/internal/config"
	"github.com/btcwave/btcwave-cli/internal/detect"
	"github.com/btcwave/btcwave-cli/internal/install"
	"github.com/btcwave/btcwave-cli/internal/rpc"
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

		if !jsonMode {
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

		if !jsonMode {
			fmt.Println("Checking for existing Bitcoin node...")
		}
		existing, err := detect.DetectExistingNode(target)
		if err == nil && existing.Found {
			s.ExistingNode = existing
			s.Migration = true
			if jsonMode {
				b, _ := json.Marshal(existing)
				fmt.Println(string(b))
			} else {
				fmt.Println("  EXISTING NODE DETECTED")
				fmt.Printf("  Binary:   %s\n", existing.Binary)
				fmt.Printf("  Version:  %s\n", existing.Version)
				if existing.IsKnots {
					fmt.Println("  Type:     Bitcoin Knots (no binary swap needed)")
				} else {
					fmt.Println("  Type:     Bitcoin Core (will migrate to Knots)")
				}
				if existing.DataDir != "" {
					fmt.Printf("  Data dir: %s\n", existing.DataDir)
				}
				if existing.ChainSizeGB > 0 {
					fmt.Printf("  Chain:    %d GB (will be preserved)\n", existing.ChainSizeGB)
				}
				fmt.Printf("  txindex:  %v | tor: %v | zmq: %v\n", existing.HasTxIndex, existing.HasTor, existing.HasZMQ)
				fmt.Println()
				fmt.Println("  Migration mode: existing config will be backed up,")
				fmt.Println("  btcwave profile merged on top. No resync required.")
				fmt.Println()
			}
		} else {
			if !jsonMode {
				fmt.Println("  No existing node found — fresh install.")
				fmt.Println()
			}
		}

		s.Phase = state.PhaseConfig
		s.Save()
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
			fmt.Println("Installing Bitcoin Knots...")
			fmt.Println()
		}

		if s.Migration && s.ExistingNode != nil && s.ExistingNode.Found {
			backupPath, err := install.BackupExisting(s.Target, s.ExistingNode, jsonMode)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error backing up existing node: %v\n", err)
				os.Exit(1)
			}
			if !jsonMode && backupPath != "" {
				fmt.Printf("  Backup saved to: %s\n\n", backupPath)
			}
		}

		result, err := install.Download(s.Hardware, s.Target, jsonMode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error installing: %v\n", err)
			os.Exit(1)
		}

		s.Phase = state.PhaseSync
		s.Save()

		if jsonMode {
			b, _ := json.Marshal(result)
			fmt.Println(string(b))
		} else {
			fmt.Println()
			fmt.Printf("  Installed: Bitcoin Knots %s\n", result.Version)
			fmt.Printf("  Binary:    %s\n", result.Binary)
			fmt.Printf("  Verified:  %v\n", result.Verified)
			if result.Migrated {
				fmt.Printf("  Migration: existing node backed up at %s\n", result.BackupPath)
			}
			fmt.Println()
			fmt.Println("Node installed. Next phase: initial block download.")
			fmt.Println("Re-run: btcwave setup")
		}
	}
}

func runStatus(jsonMode bool) {
	host := flagValue("--host")
	if host == "" {
		host = "127.0.0.1"
	}
	dataDir := flagValue("--datadir")
	if dataDir == "" {
		dataDir = "/home/bitcoin/.bitcoin"
	}
	user := flagValue("--rpcuser")
	pass := flagValue("--rpcpassword")

	var client *rpc.Client
	var err error

	if user != "" && pass != "" {
		client = rpc.NewFromAuth(host, user, pass)
	} else {
		cookiePath := rpc.FindCookie(dataDir)
		if cookiePath == "" {
			if jsonMode {
				fmt.Println(`{"error":"no RPC credentials found"}`)
			} else {
				fmt.Println("No RPC credentials found.")
				fmt.Println("Use --rpcuser/--rpcpassword or ensure .cookie exists at --datadir")
			}
			os.Exit(1)
		}
		client, err = rpc.NewFromCookie(host, cookiePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading cookie: %v\n", err)
			os.Exit(1)
		}
	}

	status, err := client.GetStatus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error querying node: %v\n", err)
		os.Exit(1)
	}

	if jsonMode {
		b, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Println("Bitcoin Wave Node Status")
	fmt.Println("========================")

	if status.Blockchain != nil {
		bc := status.Blockchain
		if status.Synced {
			fmt.Println("  Sync:     COMPLETE")
		} else {
			fmt.Printf("  Sync:     %.2f%%\n", status.SyncPct)
		}
		fmt.Printf("  Height:   %d / %d\n", bc.Blocks, bc.Headers)
		fmt.Printf("  Chain:    %s\n", bc.Chain)
		fmt.Printf("  Disk:     %.1f GB\n", float64(bc.SizeOnDisk)/1e9)
		fmt.Printf("  Pruned:   %v\n", bc.Pruned)
	}

	if status.Network != nil {
		n := status.Network
		fmt.Printf("  Peers:    %d (in: %d, out: %d)\n", n.Connections, n.ConnectionsIn, n.ConnectionsOut)
		fmt.Printf("  Version:  %s\n", n.Subversion)
	}

	if status.Mempool != nil {
		m := status.Mempool
		fmt.Printf("  Mempool:  %d tx (%.1f MB)\n", m.Size, float64(m.Bytes)/1e6)
	}

	if status.Mining != nil {
		hashrate := status.Mining.NetworkHashPS / 1e18
		fmt.Printf("  Hashrate: %.1f EH/s\n", hashrate)
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
