package sys

type PMAction struct {
	Help          string
	TakesPackages bool
	Commands      map[string]string
}

const PackagesString = "{packages}"

var PMCommands = map[string]PMAction{
	"install": {
		Help:          "Install one or more packages.",
		TakesPackages: true,
		Commands: map[string]string{
			"apt":    "sudo apt install {packages}",
			"dnf":    "sudo dnf install {packages}",
			"pacman": "sudo pacman -S {packages}",
			"zypper": "sudo zypper install {packages}",
		},
	},
	"remove": {
		Help:          "Remove one or more packages.",
		TakesPackages: true,
		Commands: map[string]string{
			"apt":    "sudo apt remove {packages}",
			"dnf":    "sudo dnf remove {packages}",
			"pacman": "sudo pacman -R {packages}",
			"zypper": "sudo zypper remove {packages}",
		},
	},
	"search": {
		Help:          "Search for available packages.",
		TakesPackages: true,
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
		Commands: map[string]string{
			"apt":    "sudo apt autoremove",
			"dnf":    "sudo dnf autoremove",
			"pacman": "sudo pacman -Rns $(pacman -Qdtq)",
			"zypper": "sudo zypper remove --clean-deps",
		},
	},
	"upgrade": {
		Help:          "Upgrade all installed packages.",
		TakesPackages: false,
		Commands: map[string]string{
			"apt":    "sudo apt upgrade",
			"dnf":    "sudo dnf upgrade",
			"pacman": "sudo pacman -Syu",
			"zypper": "sudo zypper dup",
		},
	},
	"update": {
		Help:          "Refresh package repository information.",
		TakesPackages: false,
		Commands: map[string]string{
			"apt":    "sudo apt update",
			"dnf":    "sudo dnf check-update",
			"pacman": "sudo pacman -Sy",
			"zypper": "sudo zypper refresh",
		},
	},
	"provides": {
		Help:          "Find which package provides a file or command.",
		TakesPackages: true,
		Commands: map[string]string{
			"apt":    "apt-file search {packages}",
			"dnf":    "dnf provides -C {packages}",
			"pacman": "pacman -F {packages}",
			"zypper": "zypper --no-refresh what-provides {packages}",
		},
	},
}
