package sys

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Info almacena los datos básicos del entorno del usuario.
type Info struct {
	OS            string
	PM            string
	DistroID      string
	DistroVersion string
	DistroPretty  string
}

// GetSystemInfo detecta el sistema operativo y el gestor de paquetes.
// Usa exec.LookPath, que es el equivalente al comando 'which'.
func GetSystemInfo() Info {
	info := Info{
		OS: runtime.GOOS,
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
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			key := parts[0]
			val := strings.Trim(parts[1], `"'`)
			switch key {
			case "ID": info.DistroID = val
			case "VERSION_ID": info.DistroVersion = val
			case "PRETTY_NAME": info.DistroPretty = val
			}
		}
	}
}

func detectPM() string {
	pms := []string{"dnf", "apt-get", "pacman", "zypper"}
	for _, pm := range pms {
		if _, err := exec.LookPath(pm); err == nil {
			if pm == "apt-get" { return "apt" }
			return pm
		}
	}
	return "unknown"
}
