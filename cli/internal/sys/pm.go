package sys

type PMAction struct {
	Help          string
	TakesPackages bool
	RequiresSudo  bool
	Commands      map[string]string
}

var PMTypes = []string{"dnf", "apt-get", "apt", "pacman", "zypper", "yum"}

const PackagesString = "{packages}"

var PMCommands = map[string]PMAction{
	"install": {
		Help:          "Install one or more packages.",
		TakesPackages: true,
		RequiresSudo:  true,
		Commands: map[string]string{
			"apt":    "apt install {packages}",
			"dnf":    "dnf install {packages}",
			"pacman": "pacman -S {packages}",
			"zypper": "zypper install {packages}",
		},
	},
	"remove": {
		Help:          "Remove one or more packages.",
		TakesPackages: true,
		RequiresSudo:  true,
		Commands: map[string]string{
			"apt":    "apt remove {packages}",
			"dnf":    "dnf remove {packages}",
			"pacman": "pacman -R {packages}",
			"zypper": "zypper remove {packages}",
		},
	},
	"search": {
		Help:          "Search for available packages.",
		TakesPackages: true,
		RequiresSudo:  false,
		Commands: map[string]string{
			"apt":    "apt search {packages}",
			"dnf":    "dnf search -C {packages}",
			"pacman": "pacman -Ss {packages}",
			"zypper": "zypper --no-refresh search {packages}",
		},
	},
	"autoremove": {
		Help:          "Remove unused packages (cleanup).",
		TakesPackages: false,
		RequiresSudo:  true,
		Commands: map[string]string{
			"apt":    "apt autoremove",
			"dnf":    "dnf autoremove",
			"pacman": "pacman -Rns $(pacman -Qdtq)",
			"zypper": "zypper remove --clean-deps",
		},
	},
	"upgrade": {
		Help:          "Upgrade all installed packages.",
		TakesPackages: false,
		RequiresSudo:  true,
		Commands: map[string]string{
			"apt":    "apt upgrade",
			"dnf":    "dnf upgrade",
			"pacman": "pacman -Syu",
			"zypper": "zypper dup",
		},
	},
	"update": {
		Help:          "Refresh package repository information.",
		TakesPackages: false,
		RequiresSudo:  true,
		Commands: map[string]string{
			"apt":    "apt update",
			"dnf":    "dnf check-update",
			"pacman": "pacman -Sy",
			"zypper": "zypper refresh",
		},
	},
	"provides": {
		Help:          "Find which package provides a file or command.",
		TakesPackages: true,
		RequiresSudo:  false,
		Commands: map[string]string{
			"apt":    "apt-file search {packages}",
			"dnf":    "dnf provides -C {packages}",
			"pacman": "pacman -F {packages}",
			"zypper": "zypper --no-refresh what-provides {packages}",
		},
	},
}
