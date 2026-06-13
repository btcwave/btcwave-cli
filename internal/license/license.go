package license

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultAPI = "https://btcwave.app"

type RedeemResult struct {
	Status string                 `json:"status"`
	Tier   string                 `json:"tier"`
	Config map[string]interface{} `json:"config"`
}

type ValidateResult struct {
	Valid    bool   `json:"valid"`
	Tier     string `json:"tier"`
	Redeemed bool   `json:"redeemed"`
	Revoked  bool   `json:"revoked"`
}

func Validate(key string) (*ValidateResult, error) {
	if !isValidFormat(key) {
		return nil, fmt.Errorf("invalid key format (expected WAVE-TIER-XXXX-XXXX)")
	}

	url := fmt.Sprintf("%s/api/keys/validate?key=%s", defaultAPI, key)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("cannot reach license server: %w", err)
	}
	defer resp.Body.Close()

	var result ValidateResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid response from license server")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("key validation failed")
	}

	return &result, nil
}

func Redeem(key string) (*RedeemResult, error) {
	if !isValidFormat(key) {
		return nil, fmt.Errorf("invalid key format (expected WAVE-TIER-XXXX-XXXX)")
	}

	body, _ := json.Marshal(map[string]string{"key": key})
	url := fmt.Sprintf("%s/api/keys/redeem", defaultAPI)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cannot reach license server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("%s", errResp["error"])
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("redemption failed (HTTP %d)", resp.StatusCode)
	}

	var result RedeemResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid response from license server")
	}

	return &result, nil
}

func isValidFormat(key string) bool {
	parts := strings.Split(key, "-")
	if len(parts) != 4 {
		return false
	}
	if parts[0] != "WAVE" {
		return false
	}
	return true
}
