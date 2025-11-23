#!/bin/bash

# YUPS Installation Script (Architecture v5: Semantic Versioning Fix)
# Philosophy: We don't fix the OS. We expect a valid environment.

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
    if [[ $exit_code -eq 0 ]] || [[ $exit_code -eq 127 ]]; then
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
    # Just for display logging
    $1 -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")' 2>/dev/null
}

check_python_meets_requirements() {
    # $1: The python executable
    # We ask Python directly using tuple comparison which handles 3.13 > 3.7 correctly
    # (Unlike bash/awk which thinks 3.13 < 3.7 because math)
    $1 -c 'import sys; sys.exit(0 if sys.version_info >= (3, 7) else 1)' 2>/dev/null
}

# --- 1. Pre-flight Checks ---
if [ ! -f "$SOURCE_FILE" ]; then
    echo "ERROR: '$SOURCE_FILE' script not found in this directory."
    exit 1
fi

echo "🚀 Starting YUPS Installation..."

# --- 2. Environment Validation ---
echo "🐍 Validating Python Environment (Requirement: >= 3.7)..."

CHOSEN_PYTHON=""
# We check standard python3 first, then try to find explicit newer versions if system default is old
CANDIDATES=("python3" "python3.13" "python3.12" "python3.11" "python3.10" "python3.9" "python3.8")

for candidate in "${CANDIDATES[@]}"; do
    if command -v $candidate &> /dev/null; then
        ver=$(get_python_version_str $candidate)
        
        # New robust check using python itself
        if check_python_meets_requirements "$candidate"; then
            echo "   -> Found '$candidate' (v$ver) - OK."
            CHOSEN_PYTHON=$(command -v $candidate)
            break
        else
            echo "   -> Found '$candidate' (v$ver) - Too old."
        fi
    fi
done

if [[ -z "$CHOSEN_PYTHON" ]]; then
    echo ""
    echo "❌ ERROR: YUPS requires Python 3.7 or newer."
    echo "   Your system default is too old and no alternative was found."
    echo "   ACTION REQUIRED: Please install 'python3.9' or newer using your package manager."
    echo "   (e.g., 'dnf install python39' on RHEL/Rocky 8)"
    exit 1
fi

echo "✅ Selected Interpreter: $CHOSEN_PYTHON"

# --- 3. Create Isolated Environment ---
echo "📦 Setting up isolated Python environment in $VENV_PATH..."

# Security check: If existing venv has a different python version, nuke it
if [ -d "$VENV_PATH" ]; then
    # We use the full path to avoid ambiguity
    if [ -f "$VENV_PATH/bin/python3" ]; then
        VENV_VER=$("$VENV_PATH/bin/python3" -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")' 2>/dev/null)
        CHOSEN_VER=$($CHOSEN_PYTHON -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")')
        
        if [[ "$VENV_VER" != "$CHOSEN_VER" ]]; then
            echo "   -> Venv version mismatch ($VENV_VER vs $CHOSEN_VER). Recreating..."
            $SUDO rm -rf "$VENV_PATH"
        fi
    else
        # Broken venv dir
        $SUDO rm -rf "$VENV_PATH"
    fi
fi

$SUDO mkdir -p "$(dirname "$VENV_PATH")"

# Ubuntu/Debian Helper (Only specifically for the venv module)
if grep -qi "ubuntu\|debian" /etc/os-release; then
    if ! dpkg -s python3-venv &> /dev/null; then
        echo "   -> Installing python3-venv package..."
        $SUDO apt-get update && $SUDO apt-get install -y python3-venv
    fi
fi

if [ ! -d "$VENV_PATH" ]; then
    $SUDO "$CHOSEN_PYTHON" -m venv "$VENV_PATH"
    if [ $? -ne 0 ]; then
        echo "❌ ERROR: Failed to create virtual environment."
        exit 1
    fi
    echo "   -> Virtual environment created."
else
    echo "   -> Virtual environment already exists."
fi

# --- 4. Install Dependencies ---
echo "📚 Installing dependencies..."
if ! $SUDO "$VENV_PATH/bin/pip" install --upgrade pip huggingface_hub > /dev/null; then
    echo "❌ ERROR: Failed to install Python dependencies via pip."
    exit 1
fi
echo "   -> Dependencies installed successfully."

# --- 5. Install Executable & Rewrite Shebang ---
echo "🔧 Installing executable to $INSTALL_PATH..."

$SUDO cp "$SOURCE_FILE" "$INSTALL_PATH"

echo "   -> Linking executable to isolated environment..."
TMP_FILE=$(mktemp)
sed "1s|.*|#!$VENV_PATH/bin/python3|" "$SOURCE_FILE" > "$TMP_FILE"
$SUDO cp "$TMP_FILE" "$INSTALL_PATH"
rm "$TMP_FILE"

$SUDO chmod +x "$INSTALL_PATH"
echo "✓ Executable installed and linked."

# --- 6. User Configuration (Bashrc) ---
echo "🎣 Injecting hooks into $BASHRC_FILE..."
if grep -q "# --- YUPS_HOOK_START ---" "$BASHRC_FILE"; then
    echo "   -> Hooks block already detected. Skipping."
else
    echo -e "\n$BASH_HOOKS\n" >> "$BASHRC_FILE"
    echo "✓ Bash hooks installed."
fi

# --- 7. HF Token Check ---
if [[ -z "$HF_TOKEN" ]]; then
    if grep -q "HF_TOKEN" "$BASHRC_FILE"; then
        echo "✓ HF_TOKEN found in .bashrc"
    else
        echo "🔑 HF_TOKEN environment variable is not set."
        read -p "Please enter your Hugging Face Token: " token_input
        if [[ -n "$token_input" ]]; then
            echo -e "\nexport HF_TOKEN=\"$token_input\"" >> "$BASHRC_FILE"
            export HF_TOKEN="$token_input"
        else
            echo "⚠️ WARNING: No token provided. Smart features will fail."
        fi
    fi
fi

# --- 8. Initialize ---
echo "⚙️  Initializing YUPS config..."
"$INSTALL_PATH" --auto-config

echo -e "\n✅ YUPS installation complete!"
echo "Please run: source $BASHRC_FILE"