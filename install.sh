#!/bin/bash

# YUPS Installation Script

# --- Configuration ---
YUPS_EXECUTABLE="yups"
INSTALL_PATH="/usr/local/bin/yups"
BASHRC_FILE=~/.bashrc

# --- Bash Hooks Text Block (minimal) ---
# We use 'heredoc' to define the text block
read -r -d '' BASH_HOOKS <<'EOF'

# --- YUPS_HOOK_START ---
# Hooks for the YUPS project
# (Installed on $(date))

# 1: Command Not Found Hook
command_not_found_handle() {
    # Call the Python brain
    if "$INSTALL_PATH" --cnf-handle "$@"; then
        return $?
    else
        # Fallback if the yups script fails
        echo "bash: $1: command not found" >&2
        return 127
    fi
}

# 2: Command Error Hook
_yups_ce_handle() {
    local exit_code=$?
    # The performance guard
    if [[ $exit_code -eq 0 ]] || [[ $exit_code -eq 127 ]]; then
        return
    fi
    local last_command_text=$(history 1 | sed 's/^[ ]*[0-9]\+[ ]\+//')
    # Call the Python brain (suppressing its errors)
    "$INSTALL_PATH" --ce-handle "$exit_code" "$last_command_text"
}

# Activation for the CE handler
if [[ -z "$PROMPT_COMMAND" ]]; then
    export PROMPT_COMMAND="_yups_ce_handle"
elif ! [[ "$PROMPT_COMMAND" == *"_yups_ce_handle"* ]]; then
    export PROMPT_COMMAND="_yups_ce_handle;${PROMPT_COMMAND}"
fi
# --- YUPS_HOOK_END ---
EOF
# --- End of heredoc ---


# --- 1. Root Check ---
if [ "$EUID" -eq 0 ]; then
  echo "ERROR: Do not run this script as root."
  echo "Run it as your normal user. It will ask for 'sudo' when needed."
  exit 1
fi

# --- 2. Check for Executable ---
if [ ! -f "$YUPS_EXECUTABLE" ]; then
    echo "ERROR: '$YUPS_EXECUTABLE' script not found in this directory."
    echo "Please ensure you are in the root of the repository."
    exit 1
fi

echo "Installing YUPS..."

# --- 3. Copy Executable ---
echo "Copying '$YUPS_EXECUTABLE' to '$INSTALL_PATH'..."
if ! sudo cp "$YUPS_EXECUTABLE" "$INSTALL_PATH"; then
    echo "ERROR: Could not copy file. Did 'sudo' fail?"
    exit 1
fi

echo "Setting execute permissions..."
if ! sudo chmod +x "$INSTALL_PATH"; then
    echo "ERROR: Could not set execute permissions."
    exit 1
fi
echo "✓ Executable installed."

# --- 4. Inject Hooks into .bashrc ---
echo "Injecting hooks into $BASHRC_FILE..."
if ! grep -q "# --- YUPS_HOOK_START ---" "$BASHRC_FILE"; then
    # Inject the hooks
    # We replace the $INSTALL_PATH placeholder with the real path
    echo -e "\n${BASH_HOOKS//\$INSTALL_PATH/$INSTALL_PATH}\n" >> "$BASHRC_FILE"
    echo "✓ Bash hooks installed."
else
    echo "✓ Bash hooks already installed (skipped)."
fi

# --- 5. Run initial auto-config ---
echo "Creating initial configuration cache..."
"$INSTALL_PATH" --auto-config

# --- 6. Finish ---
echo -e "\nYUPS installation complete!"
echo "Please restart your terminal or run:"
echo "  source $BASHRC_FILE"
