package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/btcwave/btcwave-cli/internal/detect"
)

func Generate(hw *detect.Hardware, target string) (string, error) {
	rpcUser := "btcwave"
	salt, hash := generateRPCAuth(rpcUser)
	rpcAuth := fmt.Sprintf("%s:%s$%s", rpcUser, salt, hash)

	maxConn := 40
	if hw.MemoryMB >= 8192 {
		maxConn = 64
	}
	if hw.MemoryMB >= 16384 {
		maxConn = 96
	}

	var b strings.Builder
	b.WriteString("# Bitcoin Wave — Generated Node Configuration\n")
	b.WriteString(fmt.Sprintf("# Target: %s | Arch: %s | RAM: %dMB | Cores: %d\n", target, hw.Arch, hw.MemoryMB, hw.Cores))
	b.WriteString("# Profile version: 1.0.0\n\n")

	b.WriteString("# --- Core ---\n")
	b.WriteString("server=1\n")
	b.WriteString("txindex=1\n\n")

	b.WriteString("# --- Network (Tor by default) ---\n")
	b.WriteString("listen=1\n")
	b.WriteString("listenonion=1\n")
	b.WriteString("proxy=127.0.0.1:9050\n")
	b.WriteString("torcontrol=127.0.0.1:9051\n\n")

	b.WriteString("# --- RPC ---\n")
	b.WriteString(fmt.Sprintf("rpcauth=%s\n", rpcAuth))
	b.WriteString("zmqpubrawblock=tcp://127.0.0.1:28332\n")
	b.WriteString("zmqpubrawtx=tcp://127.0.0.1:28333\n")
	b.WriteString("rpcbind=127.0.0.1\n")
	b.WriteString("rpcallowip=127.0.0.1\n\n")

	b.WriteString("# --- Resource limits ---\n")
	b.WriteString(fmt.Sprintf("maxconnections=%d\n", maxConn))
	b.WriteString("maxuploadtarget=5000\n\n")

	b.WriteString("# --- Spam filtering (Knots policy) ---\n")
	b.WriteString("datacarriersize=42\n")
	b.WriteString("permitbaremultisig=0\n")
	b.WriteString("minrelaytxfee=0.00005\n")
	b.WriteString("blockmaxsize=1000000\n")
	b.WriteString("limitancestorcount=10\n")
	b.WriteString("limitdescendantcount=20\n")
	b.WriteString("dustrelayfee=0.00003\n\n")

	b.WriteString("# --- BIP-110 / RDTS ---\n")
	b.WriteString("consensusrules=rdts\n")

	home, _ := os.UserHomeDir()
	outDir := filepath.Join(home, ".btcwave", "generated")
	if err := os.MkdirAll(outDir, 0700); err != nil {
		return "", err
	}
	outPath := filepath.Join(outDir, "bitcoin.conf")
	if err := os.WriteFile(outPath, []byte(b.String()), 0600); err != nil {
		return "", err
	}

	return outPath, nil
}

func generateRPCAuth(user string) (string, string) {
	saltBytes := make([]byte, 16)
	rand.Read(saltBytes)
	salt := hex.EncodeToString(saltBytes)

	passBytes := make([]byte, 32)
	rand.Read(passBytes)
	password := hex.EncodeToString(passBytes)

	hmacInput := fmt.Sprintf("%s%s", salt, password)
	h := sha256.Sum256([]byte(hmacInput))
	hash := hex.EncodeToString(h[:])

	return salt, hash
}
