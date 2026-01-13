package sys

// PMAction: Define las propiedades de una acción del gestor de paquetes.
type PMAction struct {
	Help          string
	TakesPackages bool
	Commands      map[string]string
}

const PackagesString = "{packages}"

// PMCommands: Mapa global que traduce intenciones de usuario a comandos específicos por distro.
var PMCommands = map[string]PMAction{
	"install": {
		Help:          "Instalar uno o más paquetes.",
		TakesPackages: true,
		Commands: map[string]string{
			"apt":    "sudo apt install {packages}",
			"dnf":    "sudo dnf install {packages}",
			"pacman": "sudo pacman -S {packages}",
			"zypper": "sudo zypper install {packages}",
		},
	},
	"remove": {
		Help:          "Eliminar uno o más paquetes.",
		TakesPackages: true,
		Commands: map[string]string{
			"apt":    "sudo apt remove {packages}",
			"dnf":    "sudo dnf remove {packages}",
			"pacman": "sudo pacman -R {packages}",
			"zypper": "sudo zypper remove {packages}",
		},
	},
	"search": {
		Help:          "Buscar paquetes disponibles.",
		TakesPackages: true,
		Commands: map[string]string{
			"apt":    "apt search {packages}",
			"dnf":    "dnf search -C {packages}",
			"pacman": "pacman -Ss {packages}",
			"zypper": "zypper --no-refresh search {packages}",
		},
	},
	"autoremove": {
		Help:          "Limpiar paquetes no utilizados.",
		TakesPackages: false,
		Commands: map[string]string{
			"apt":    "sudo apt autoremove",
			"dnf":    "sudo dnf autoremove",
			"pacman": "sudo pacman -Rns $(pacman -Qdtq)",
			"zypper": "sudo zypper remove --clean-deps",
		},
	},
	"provides": {
		Help:          "Encontrar qué paquete provee un archivo o comando.",
		TakesPackages: true,
		Commands: map[string]string{
			"apt":    "apt-file search {packages}",
			"dnf":    "dnf provides -C {packages}",
			"pacman": "pacman -F {packages}",
			"zypper": "zypper --no-refresh what-provides {packages}",
		},
	},
}
