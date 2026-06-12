package detect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ExistingNode struct {
	Found       bool   `json:"found"`
	Binary      string `json:"binary,omitempty"`
	Version     string `json:"version,omitempty"`
	IsKnots     bool   `json:"is_knots"`
	DataDir     string `json:"datadir,omitempty"`
	ConfigPath  string `json:"config_path,omitempty"`
	HasTxIndex  bool   `json:"has_txindex"`
	HasTor      bool   `json:"has_tor"`
	HasZMQ      bool   `json:"has_zmq"`
	ChainSizeGB int    `json:"chain_size_gb,omitempty"`
}

func DetectExistingNode(target string) (*ExistingNode, error) {
	if target == "localhost" || target == "" {
		return detectLocalNode()
	}
	return detectRemoteNode(target)
}

func detectLocalNode() (*ExistingNode, error) {
	node := &ExistingNode{}

	for _, bin := range []string{"bitcoind", "/usr/local/bin/bitcoind", "/usr/bin/bitcoind"} {
		out, err := exec.Command(bin, "--version").Output()
		if err == nil {
			node.Found = true
			node.Binary = bin
			node.Version = parseVersion(string(out))
			node.IsKnots = strings.Contains(string(out), "Knots")
			break
		}
	}

	if !node.Found {
		return node, nil
	}

	home, _ := os.UserHomeDir()
	dataDirs := []string{
		filepath.Join(home, ".bitcoin"),
		"/home/bitcoin/.bitcoin",
	}
	for _, d := range dataDirs {
		confPath := filepath.Join(d, "bitcoin.conf")
		if _, err := os.Stat(confPath); err == nil {
			node.DataDir = d
			node.ConfigPath = confPath
			parseConfig(node, confPath)
			break
		}
	}

	if node.DataDir != "" {
		blocksDir := filepath.Join(node.DataDir, "blocks")
		node.ChainSizeGB = dirSizeGB(blocksDir)
	}

	return node, nil
}

func detectRemoteNode(target string) (*ExistingNode, error) {
	node := &ExistingNode{}

	out, err := sshCmd(target, "bitcoind --version 2>/dev/null || /usr/local/bin/bitcoind --version 2>/dev/null")
	if err != nil || strings.TrimSpace(out) == "" {
		return node, nil
	}

	node.Found = true
	node.Version = parseVersion(out)
	node.IsKnots = strings.Contains(out, "Knots")

	out, err = sshCmd(target, "which bitcoind 2>/dev/null")
	if err == nil {
		node.Binary = strings.TrimSpace(out)
	}

	for _, d := range []string{"/home/bitcoin/.bitcoin", "~/.bitcoin"} {
		confCmd := fmt.Sprintf("cat %s/bitcoin.conf 2>/dev/null", d)
		out, err = sshCmd(target, confCmd)
		if err == nil && strings.TrimSpace(out) != "" {
			node.DataDir = d
			node.ConfigPath = d + "/bitcoin.conf"
			node.HasTxIndex = strings.Contains(out, "txindex=1")
			node.HasTor = strings.Contains(out, "proxy=127.0.0.1:9050")
			node.HasZMQ = strings.Contains(out, "zmqpub")
			break
		}
	}

	out, err = sshCmd(target, "du -sg /home/bitcoin/.bitcoin/blocks 2>/dev/null | awk '{print $1}'")
	if err == nil {
		s := strings.TrimSpace(out)
		if s != "" {
			fmt.Sscanf(s, "%d", &node.ChainSizeGB)
		}
	}

	return node, nil
}

func parseVersion(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "version") {
			parts := strings.Fields(line)
			for _, p := range parts {
				if strings.HasPrefix(p, "v") {
					return p
				}
			}
		}
	}
	return strings.TrimSpace(strings.Split(out, "\n")[0])
}

func parseConfig(node *ExistingNode, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	content := string(data)
	node.HasTxIndex = strings.Contains(content, "txindex=1")
	node.HasTor = strings.Contains(content, "proxy=127.0.0.1:9050")
	node.HasZMQ = strings.Contains(content, "zmqpub")
}

func dirSizeGB(path string) int {
	var total int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return int(total / 1024 / 1024 / 1024)
}
