package state

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/btcwave/btcwave-cli/internal/detect"
)

type Phase string

const (
	PhaseNew      Phase = "new"
	PhaseDetect   Phase = "detect"
	PhaseConfig   Phase = "config"
	PhaseInstall  Phase = "install"
	PhaseSync     Phase = "syncing"
	PhaseSeed     Phase = "seed_ceremony"
	PhaseStack    Phase = "stack"
	PhaseComplete Phase = "complete"
)

type State struct {
	Phase        Phase                `json:"phase"`
	LicenseKey   string               `json:"license_key,omitempty"`
	Target       string               `json:"target"`
	Hardware     *detect.Hardware     `json:"hardware,omitempty"`
	ExistingNode *detect.ExistingNode `json:"existing_node,omitempty"`
	Migration    bool                 `json:"migration"`
	ConfigPath   string               `json:"config_path,omitempty"`
}

func statePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".btcwave", "state.json")
}

func New() *State {
	return &State{Phase: PhaseNew}
}

func Load() (*State, error) {
	data, err := os.ReadFile(statePath())
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *State) Save() error {
	dir := filepath.Dir(statePath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(), data, 0600)
}
