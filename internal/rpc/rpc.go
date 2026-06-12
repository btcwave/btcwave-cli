package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Client struct {
	URL      string
	User     string
	Password string
}

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type BlockchainInfo struct {
	Chain                string  `json:"chain"`
	Blocks               int     `json:"blocks"`
	Headers              int     `json:"headers"`
	BestBlockHash        string  `json:"bestblockhash"`
	Difficulty           float64 `json:"difficulty"`
	VerificationProgress float64 `json:"verificationprogress"`
	InitialBlockDownload bool    `json:"initialblockdownload"`
	SizeOnDisk           int64   `json:"size_on_disk"`
	Pruned               bool    `json:"pruned"`
}

type NetworkInfo struct {
	Version         int    `json:"version"`
	Subversion      string `json:"subversion"`
	Connections     int    `json:"connections"`
	ConnectionsIn   int    `json:"connections_in"`
	ConnectionsOut  int    `json:"connections_out"`
	NetworkActive   bool   `json:"networkactive"`
}

type MempoolInfo struct {
	Loaded bool    `json:"loaded"`
	Size   int     `json:"size"`
	Bytes  int64   `json:"bytes"`
	MinFee float64 `json:"mempoolminfee"`
}

type MiningInfo struct {
	Blocks         int     `json:"blocks"`
	Difficulty     float64 `json:"difficulty"`
	NetworkHashPS  float64 `json:"networkhashps"`
}

type NodeStatus struct {
	Blockchain *BlockchainInfo `json:"blockchain"`
	Network    *NetworkInfo    `json:"network"`
	Mempool    *MempoolInfo    `json:"mempool"`
	Mining     *MiningInfo     `json:"mining"`
	Synced     bool            `json:"synced"`
	SyncPct    float64         `json:"sync_pct"`
}

func NewFromCookie(host string, cookiePath string) (*Client, error) {
	data, err := os.ReadFile(cookiePath)
	if err != nil {
		return nil, fmt.Errorf("read cookie: %w", err)
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cookie format")
	}
	return &Client{
		URL:      fmt.Sprintf("http://%s:8332", host),
		User:     parts[0],
		Password: parts[1],
	}, nil
}

func NewFromAuth(host, user, password string) *Client {
	return &Client{
		URL:      fmt.Sprintf("http://%s:8332", host),
		User:     user,
		Password: password,
	}
}

func FindCookie(dataDir string) string {
	candidates := []string{
		filepath.Join(dataDir, ".cookie"),
		"/home/bitcoin/.bitcoin/.cookie",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".bitcoin", ".cookie"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func (c *Client) call(method string, params ...interface{}) (json.RawMessage, error) {
	req := rpcRequest{
		JSONRPC: "1.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", c.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.SetBasicAuth(c.User, c.Password)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("RPC connection failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("invalid RPC response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

func (c *Client) GetStatus() (*NodeStatus, error) {
	status := &NodeStatus{}

	if result, err := c.call("getblockchaininfo"); err == nil {
		var info BlockchainInfo
		json.Unmarshal(result, &info)
		status.Blockchain = &info
		status.SyncPct = info.VerificationProgress * 100
		status.Synced = !info.InitialBlockDownload && info.VerificationProgress > 0.9999
	} else {
		return nil, fmt.Errorf("getblockchaininfo: %w", err)
	}

	if result, err := c.call("getnetworkinfo"); err == nil {
		var info NetworkInfo
		json.Unmarshal(result, &info)
		status.Network = &info
	}

	if result, err := c.call("getmempoolinfo"); err == nil {
		var info MempoolInfo
		json.Unmarshal(result, &info)
		status.Mempool = &info
	}

	if result, err := c.call("getmininginfo"); err == nil {
		var info MiningInfo
		json.Unmarshal(result, &info)
		status.Mining = &info
	}

	return status, nil
}
