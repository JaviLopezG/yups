package sys

type PMAction struct {
	Help          string
	TakesArgument bool
	RequiresSudo  bool
	Commands      map[string]string
}

var PMTypes = []string{"dnf", "apt-get", "apt", "pacman", "zypper", "yum"}

const ArgumentString = "{value}"

var PMCommands = map[string]PMAction{
	"install": {
		Help:          "Install one or more packages.",
		TakesArgument: true,
		RequiresSudo:  true,
		Commands: map[string]string{
			"apt":    "apt install {value}",
			"dnf":    "dnf install {value}",
			"pacman": "pacman -S {value}",
			"zypper": "zypper install {value}",
		},
	},
	"remove": {
		Help:          "Remove one or more packages.",
		TakesArgument: true,
		RequiresSudo:  true,
		Commands: map[string]string{
			"apt":    "apt remove {value}",
			"dnf":    "dnf remove {value}",
			"pacman": "pacman -R {value}",
			"zypper": "zypper remove {value}",
		},
	},
	"search": {
		Help:          "Search for available packages.",
		TakesArgument: true,
		RequiresSudo:  false,
		Commands: map[string]string{
			"apt":    "apt search {value}",
			"dnf":    "dnf search -C {value}",
			"pacman": "pacman -Ss {value}",
			"zypper": "zypper --no-refresh search {value}",
		},
	},
	"autoremove": {
		Help:          "Remove unused packages (cleanup).",
		TakesArgument: false,
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
		TakesArgument: false,
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
		TakesArgument: false,
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
		TakesArgument: true,
		RequiresSudo:  false,
		Commands: map[string]string{
			"apt":    "apt-file  search --regexp \"^/.*bin/{value}$\"",
			"dnf":    "dnf provides -C {value}",
			"pacman": "pacman -F {value}",
			"zypper": "zypper --no-refresh what-provides {value}",
		},
	},
}
