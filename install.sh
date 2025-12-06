#!/bin/bash

# YUPS Installation Script (v6 - Lean & Mean)
# Changes:
# - Removed huggingface_hub dependency (we use the API now).
# - Added auto-installation of 'apt-file' and 'pkgfile' for native 'provides' support.
# - Forces update of file databases (apt-file update / pkgfile -u).


# --- Configuration ---
INSTALL_PATH="/usr/local/bin/yups"
VENV_PATH="/opt/yups/venv"
SOURCE_FILE="yups"
BASHRC_FILE=~/.bashrc

# Detect sudo requirement
SUDO="sudo"
if [ "$EUID" -eq 0 ]; then
  echo "WARNING: Do not run this script as root."
  echo "Run it as your normal user. It will ask for 'sudo' when needed."
  read -p "Do you want to continue? (Y/n): " choice
  if [[ "$choice" == "n" || "$choice" == "N" ]]; then
      exit 1
  fi
  SUDO=""
fi

# --- Bash Hooks Text Block ---
read -r -d '' BASH_HOOKS <<'EOF'

# --- YUPS_HOOK_START ---
# Hooks for the YUPS project
# 1: Command Not Found Hook
command_not_found_handle() {
    if "/usr/local/bin/yups" --cnf-handle "$@"; then
        return $?
    else
        return 127
    fi
}

# 2: Command Error Hook
_yups_ce_handle() {
    local exit_code=$?
    if [[ $exit_code -eq 0 ]] || [[ $exit_code -eq 127 ]] || [[ $exit_code -eq 130]]; then
        return
    fi
    local last_command_text=$(history 1 | sed 's/^[ ]*[0-9]\+[ ]\+//')
    "/usr/local/bin/yups" --ce-handle "$exit_code" "$last_command_text"
}

if [[ -z "$PROMPT_COMMAND" ]]; then
    export PROMPT_COMMAND="_yups_ce_handle"
elif ! [[ "$PROMPT_COMMAND" == *"_yups_ce_handle"* ]]; then
    export PROMPT_COMMAND="_yups_ce_handle;${PROMPT_COMMAND}"
fi
# --- YUPS_HOOK_END ---
EOF

# --- Helper Functions ---

get_python_version_str() {
    $1 -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")' 2>/dev/null
}

check_python_meets_requirements() {
    # YUPS v10 only needs requests, so 3.7+ is fine, even 3.6 might work but let's stick to 3.7 standard
    $1 -c 'import sys; sys.exit(0 if sys.version_info >= (3, 7) else 1)' 2>/dev/null
}

# --- 1. Pre-flight Checks ---
if [ ! -f "$SOURCE_FILE" ]; then
    echo "ERROR: '$SOURCE_FILE' script not found in this directory."
    exit 1
fi

echo "🚀 Starting YUPS Installation..."

# --- 2. Environment Validation ---
echo "🐍 Validating Python Environment..."

CHOSEN_PYTHON=""
CANDIDATES=("python3" "python3.13" "python3.12" "python3.11" "python3.10" "python3.9" "python3.8")

for candidate in "${CANDIDATES[@]}"; do
    if command -v $candidate &> /dev/null; then
        ver=$(get_python_version_str $candidate)
        if check_python_meets_requirements "$candidate"; then
            echo "   -> Found '$candidate' (v$ver) - OK."
            CHOSEN_PYTHON=$(command -v $candidate)
            break
        fi
    fi
done

if [[ -z "$CHOSEN_PYTHON" ]]; then
    echo "❌ ERROR: YUPS requires Python 3.7 or newer."
    # Try to help Rocky Linux users
    if grep -qi "rocky\|rhel\|centos" /etc/os-release; then
         echo "   -> Detected RHEL-based system. Trying to install python39..."
         if $SUDO dnf install -y python39; then
             CHOSEN_PYTHON=$(command -v python3.9)
         fi
    fi
    
    if [[ -z "$CHOSEN_PYTHON" ]]; then
        echo "   Please install a newer python manually."
        exit 1
    fi
fi

echo "✅ Selected Interpreter: $CHOSEN_PYTHON"

# --- 3. Install Helper Tools (apt-file / pkgfile) ---
echo "🛠️  Checking for 'provides' helper tools..."

if command -v apt-get &> /dev/null; then
    if ! command -v apt-file &> /dev/null; then
        echo "   -> installing 'apt-file' for advanced search..."
        $SUDO apt-get update && $SUDO apt-get install -y apt-file
        echo "   -> updating apt-file cache (this may take a moment)..."
        $SUDO apt-file update
    else
        echo "   -> 'apt-file' is already installed."
    fi
elif command -v pacman &> /dev/null; then
    if ! command -v pkgfile &> /dev/null; then
        echo "   -> installing 'pkgfile' for advanced search..."
        $SUDO pacman -S --noconfirm pkgfile
        echo "   -> updating pkgfile database..."
        $SUDO pkgfile --update
    else
        echo "   -> 'pkgfile' is already installed."
    fi
fi

# --- 4. Create Isolated Environment ---
echo "📦 Setting up isolated Python environment in $VENV_PATH..."

# Security check for venv python version match
if [ -d "$VENV_PATH" ] && [ -f "$VENV_PATH/bin/python3" ]; then
    VENV_VER=$("$VENV_PATH/bin/python3" -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")' 2>/dev/null)
    CHOSEN_VER=$($CHOSEN_PYTHON -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")')
    if [[ "$VENV_VER" != "$CHOSEN_VER" ]]; then
        echo "   -> Version mismatch. Recreating venv..."
        $SUDO rm -rf "$VENV_PATH"
    fi
fi

$SUDO mkdir -p "$(dirname "$VENV_PATH")"

# Ubuntu/Debian specific fix
if grep -qi "ubuntu\|debian" /etc/os-release; then
    if ! dpkg -s python3-venv &> /dev/null; then
        echo "   -> Installing python3-venv package..."
        $SUDO apt-get update && $SUDO apt-get install -y python3-venv
    fi
fi

if [ ! -d "$VENV_PATH" ]; then
    $SUDO "$CHOSEN_PYTHON" -m venv "$VENV_PATH"
fi

# --- 5. Install Dependencies ---
echo "📚 Installing dependencies (requests)..."
# We ONLY need requests now, huggingface_hub is gone!
if ! $SUDO "$VENV_PATH/bin/pip" install --upgrade pip requests > /dev/null; then
    echo "❌ ERROR: Failed to install Python dependencies."
    exit 1
fi

# --- 6. Install Executable & Rewrite Shebang ---
echo "🔧 Installing executable to $INSTALL_PATH..."
$SUDO cp "$SOURCE_FILE" "$INSTALL_PATH"

echo "   -> Linking executable to isolated environment..."
TMP_FILE=$(mktemp)
sed "1s|.*|#!$VENV_PATH/bin/python3|" "$SOURCE_FILE" > "$TMP_FILE"
$SUDO cp "$TMP_FILE" "$INSTALL_PATH"
rm "$TMP_FILE"

$SUDO chmod +x "$INSTALL_PATH"

# --- 7. User Configuration (Bashrc) ---
echo "🎣 Injecting hooks into $BASHRC_FILE..."
if grep -q "# --- YUPS_HOOK_START ---" "$BASHRC_FILE"; then
    echo "   -> Hooks block already detected. Skipping."
else
    echo -e "\n$BASH_HOOKS\n" >> "$BASHRC_FILE"
    echo "✓ Bash hooks installed."
fi

# --- 8. HF Token Check (No longer needed strictly, but kept for legacy or custom use) ---
# We skip the mandatory check since the server handles auth now.

# --- 9. Initialize ---
echo "⚙️  Initializing YUPS config..."
"$INSTALL_PATH" --auto-config

echo -e "\n✅ YUPS installation complete!"
echo "Please run: source $BASHRC_FILE"
