package detect

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type Hardware struct {
	Arch       string `json:"arch"`
	OS         string `json:"os"`
	Cores      int    `json:"cores"`
	MemoryMB   int    `json:"memory_mb"`
	DiskFreeGB int    `json:"disk_free_gb"`
}

func Detect(target string) (*Hardware, error) {
	if target == "localhost" || target == "" {
		return detectLocal()
	}
	return detectRemote(target)
}

func detectLocal() (*Hardware, error) {
	hw := &Hardware{
		Arch:  runtime.GOARCH,
		OS:    runtime.GOOS,
		Cores: runtime.NumCPU(),
	}

	hw.MemoryMB = getLocalMemoryMB()
	hw.DiskFreeGB = getLocalDiskFreeGB()

	return hw, nil
}

func detectRemote(target string) (*Hardware, error) {
	hw := &Hardware{}

	out, err := sshCmd(target, "uname -m")
	if err != nil {
		return nil, err
	}
	hw.Arch = normalizeArch(strings.TrimSpace(out))
	hw.OS = "linux"

	out, err = sshCmd(target, "nproc")
	if err == nil {
		hw.Cores, _ = strconv.Atoi(strings.TrimSpace(out))
	}

	out, err = sshCmd(target, "grep MemTotal /proc/meminfo | awk '{print $2}'")
	if err == nil {
		kb, _ := strconv.Atoi(strings.TrimSpace(out))
		hw.MemoryMB = kb / 1024
	}

	out, err = sshCmd(target, "df -BG --output=avail / | tail -1")
	if err == nil {
		s := strings.TrimSpace(out)
		s = strings.TrimSuffix(s, "G")
		hw.DiskFreeGB, _ = strconv.Atoi(strings.TrimSpace(s))
	}

	return hw, nil
}

func sshCmd(target, cmd string) (string, error) {
	out, err := exec.Command("ssh", target, cmd).Output()
	return string(out), err
}

func normalizeArch(arch string) string {
	switch arch {
	case "aarch64":
		return "arm64"
	case "x86_64":
		return "amd64"
	default:
		return arch
	}
}

func getLocalMemoryMB() int {
	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					kb, _ := strconv.Atoi(fields[1])
					return kb / 1024
				}
			}
		}
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err == nil {
			bytes, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
			return int(bytes / 1024 / 1024)
		}
	}
	return 0
}

func getLocalDiskFreeGB() int {
	switch runtime.GOOS {
	case "linux":
		out, err := exec.Command("df", "-BG", "--output=avail", "/").Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) >= 2 {
				s := strings.TrimSpace(lines[len(lines)-1])
				s = strings.TrimSuffix(s, "G")
				gb, _ := strconv.Atoi(s)
				return gb
			}
		}
	case "darwin":
		out, err := exec.Command("df", "-g", "/").Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 4 {
					gb, _ := strconv.Atoi(fields[3])
					return gb
				}
			}
		}
	}
	return 0
}
