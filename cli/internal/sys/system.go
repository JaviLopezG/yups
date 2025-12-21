package sys

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type Info struct {
	OS            string
	PM            string
	DistroID      string
	DistroVersion string
	DistroPretty  string
	IsRoot        bool
}

func GetSystemInfo() Info {
	info := Info{
		OS:     runtime.GOOS,
		IsRoot: os.Geteuid() == 0,
		PM:     "unknown",
	}

	if info.OS == "linux" {
		parseOsRelease(&info)
		info.PM = detectPM()
	}

	return info
}

func parseOsRelease(info *Info) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := parts[0]
		val := strings.Trim(parts[1], `"'`)

		switch key {
		case "ID":
			info.DistroID = val
		case "VERSION_ID":
			info.DistroVersion = val
		case "PRETTY_NAME":
			info.DistroPretty = val
		}
	}
}

func detectPM() string {
	for _, pm := range PMTypes {
		if _, err := exec.LookPath(pm); err == nil {
			if pm == "apt-get" {
				return "apt"
			}
			return pm
		}
	}
	return "unknown"
}
