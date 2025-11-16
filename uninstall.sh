#!/bin/bash

# YUPS Uninstallation Script

# --- Configuration ---
INSTALL_PATH="/usr/local/bin/yups"
BASHRC_FILE=~/.bashrc
CONFIG_DIR=~/.yups

# --- 1. Root Check ---
if [ "$EUID" -eq 0 ]; then
  echo "ERROR: Do not run this script as root."
  echo "Run it as your normal user. It will ask for 'sudo' when needed."
  exit 1
fi

echo "Uninstalling YUPS..."

# --- 2. Remove Executable ---
echo "Removing executable ($INSTALL_PATH)..."
if [ -f "$INSTALL_PATH" ]; then
    if ! sudo rm -f "$INSTALL_PATH"; then
        echo "ERROR: Could not remove executable. Did 'sudo' fail?"
        # Don't stop, still try to clean up bashrc
    else
        echo "✓ Executable removed."
    fi
else
    echo "✓ Executable not found (skipped)."
fi

# --- 3. Remove Hooks from .bashrc ---
echo "Cleaning $BASHRC_FILE..."
if grep -q "# --- YUPS_HOOK_START ---" "$BASHRC_FILE"; then
    # Use sed to delete the block between our markers
    sed -i '/# --- YUPS_HOOK_START ---/,/# --- YUPS_HOOK_END ---/d' "$BASHRC_FILE"
    echo "✓ Bash hooks removed."
else
    echo "✓ No hooks found in $BASHRC_FILE (skipped)."
fi

# --- 4. Remove Config Directory (Optional) ---
if [ -d "$CONFIG_DIR" ]; then
    echo "The configuration directory and logs are still at $CONFIG_DIR"
    read -p "Do you want to remove them? (y/N): " choice
    if [[ "$choice" == "y" || "$choice" == "Y" ]]; then
        rm -rf "$CONFIG_DIR"
        echo "✓ Configuration directory removed."
    fi
fi

# --- 5. Finish ---
echo -e "\nYUPS uninstallation complete!"
echo "Please restart your terminal or run:"
echo "  source $BASHRC_FILE"
