package install

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/btcwave/btcwave-cli/internal/detect"
)

const (
	KnotsVersion = "29.3.knots20260508"
	BaseURL      = "https://bitcoinknots.org/files/29.x/29.3.knots20260508"
	InstallDir   = "/usr/local/bin"
)

type InstallResult struct {
	Version    string `json:"version"`
	Binary     string `json:"binary"`
	Arch       string `json:"arch"`
	Verified   bool   `json:"verified"`
	Migrated   bool   `json:"migrated"`
	BackupPath string `json:"backup_path,omitempty"`
}

func archSuffix(hw *detect.Hardware) string {
	arch := hw.Arch
	if arch == "" {
		arch = runtime.GOARCH
	}
	switch arch {
	case "arm64", "aarch64":
		return "aarch64-linux-gnu"
	case "amd64", "x86_64":
		return "x86_64-linux-gnu"
	default:
		return arch + "-linux-gnu"
	}
}

func tarballName(hw *detect.Hardware) string {
	return fmt.Sprintf("bitcoin-%s-%s.tar.gz", KnotsVersion, archSuffix(hw))
}

func Download(hw *detect.Hardware, target string, jsonMode bool) (*InstallResult, error) {
	name := tarballName(hw)
	url := BaseURL + "/" + name

	tmpDir := os.TempDir()
	tarPath := filepath.Join(tmpDir, name)
	shaPath := filepath.Join(tmpDir, "SHA256SUMS")
	shaURL := BaseURL + "/SHA256SUMS"

	if !jsonMode {
		fmt.Printf("  Downloading %s...\n", name)
	}
	if err := downloadFile(tarPath, url); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	if !jsonMode {
		fmt.Println("  Downloading SHA256SUMS...")
	}
	if err := downloadFile(shaPath, shaURL); err != nil {
		return nil, fmt.Errorf("checksum download failed: %w", err)
	}

	if !jsonMode {
		fmt.Println("  Verifying checksum...")
	}
	ok, err := verifyChecksum(tarPath, shaPath, name)
	if err != nil {
		return nil, fmt.Errorf("checksum verification error: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("CHECKSUM MISMATCH — download is corrupt or tampered")
	}
	if !jsonMode {
		fmt.Println("  Checksum verified OK")
	}

	result := &InstallResult{
		Version:  KnotsVersion,
		Arch:     archSuffix(hw),
		Verified: true,
	}

	if target == "localhost" || target == "" {
		if err := installLocal(tarPath, tmpDir, result); err != nil {
			return nil, err
		}
	} else {
		if err := installRemote(tarPath, target, hw, result, jsonMode); err != nil {
			return nil, err
		}
	}

	os.Remove(tarPath)
	os.Remove(shaPath)

	return result, nil
}

func BackupExisting(target string, existing *detect.ExistingNode, jsonMode bool) (string, error) {
	if !existing.Found {
		return "", nil
	}

	if !jsonMode {
		fmt.Printf("  Backing up existing %s...\n", existing.Version)
	}

	if target == "localhost" || target == "" {
		backupDir := filepath.Join(filepath.Dir(existing.Binary), "btcwave-backup")
		os.MkdirAll(backupDir, 0755)
		src := existing.Binary
		dst := filepath.Join(backupDir, filepath.Base(src)+".pre-btcwave")
		data, err := os.ReadFile(src)
		if err != nil {
			return "", fmt.Errorf("backup read failed: %w", err)
		}
		if err := os.WriteFile(dst, data, 0755); err != nil {
			return "", fmt.Errorf("backup write failed: %w", err)
		}

		if existing.ConfigPath != "" {
			confBackup := existing.ConfigPath + ".pre-btcwave"
			confData, err := os.ReadFile(existing.ConfigPath)
			if err == nil {
				os.WriteFile(confBackup, confData, 0600)
			}
		}
		return backupDir, nil
	}

	backupDir := "/home/bitcoin/bitcoin-knots/btcwave-backup"
	cmds := fmt.Sprintf("sudo mkdir -p %s && sudo cp %s %s/%s.pre-btcwave",
		backupDir, existing.Binary, backupDir, filepath.Base(existing.Binary))
	if existing.ConfigPath != "" {
		cmds += fmt.Sprintf(" && sudo cp %s %s.pre-btcwave", existing.ConfigPath, existing.ConfigPath)
	}
	_, err := sshCmd(target, cmds)
	if err != nil {
		return "", fmt.Errorf("remote backup failed: %w", err)
	}
	return backupDir, nil
}

func installLocal(tarPath, tmpDir string, result *InstallResult) error {
	extractDir := filepath.Join(tmpDir, "btcwave-extract")
	os.MkdirAll(extractDir, 0755)
	defer os.RemoveAll(extractDir)

	cmd := exec.Command("tar", "xzf", tarPath, "-C", extractDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	binDir := filepath.Join(extractDir, fmt.Sprintf("bitcoin-%s", KnotsVersion), "bin")
	for _, bin := range []string{"bitcoind", "bitcoin-cli"} {
		src := filepath.Join(binDir, bin)
		dst := filepath.Join(InstallDir, bin)
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", bin, err)
		}
		if err := os.WriteFile(dst, data, 0755); err != nil {
			return fmt.Errorf("install %s: %w", bin, err)
		}
	}

	result.Binary = filepath.Join(InstallDir, "bitcoind")
	return nil
}

func installRemote(tarPath, target string, hw *detect.Hardware, result *InstallResult, jsonMode bool) error {
	if !jsonMode {
		fmt.Println("  Copying to target...")
	}

	remoteTmp := "/tmp/" + filepath.Base(tarPath)
	cmd := exec.Command("scp", tarPath, target+":"+remoteTmp)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("scp failed: %w", err)
	}

	installCmds := fmt.Sprintf(
		"cd /tmp && tar xzf %s && "+
			"sudo cp bitcoin-%s/bin/bitcoind /home/bitcoin/bitcoin-knots/current/bin/bitcoind && "+
			"sudo cp bitcoin-%s/bin/bitcoin-cli /home/bitcoin/bitcoin-knots/current/bin/bitcoin-cli && "+
			"rm -rf /tmp/bitcoin-%s*",
		filepath.Base(tarPath), KnotsVersion, KnotsVersion, KnotsVersion)

	if !jsonMode {
		fmt.Println("  Installing on target...")
	}
	_, err := sshCmd(target, installCmds)
	if err != nil {
		return fmt.Errorf("remote install failed: %w", err)
	}

	result.Binary = "/home/bitcoin/bitcoin-knots/current/bin/bitcoind"
	return nil
}

func downloadFile(path, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func verifyChecksum(filePath, sumsPath, fileName string) (bool, error) {
	sumsData, err := os.ReadFile(sumsPath)
	if err != nil {
		return false, err
	}

	var expectedHash string
	for _, line := range strings.Split(string(sumsData), "\n") {
		if strings.Contains(line, fileName) {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				expectedHash = parts[0]
				break
			}
		}
	}
	if expectedHash == "" {
		return false, fmt.Errorf("no checksum found for %s in SHA256SUMS", fileName)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	return actualHash == expectedHash, nil
}

func sshCmd(target, cmd string) (string, error) {
	out, err := exec.Command("ssh", target, cmd).Output()
	return string(out), err
}
