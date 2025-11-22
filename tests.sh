#!/bin/bash

_test_ce_handle() {
    local exit_code=$?
    local command=$1
    # The performance guard
    if [[ $exit_code -eq 0 ]] || [[ $exit_code -eq 127 ]]; then
        return
    fi
    # Call the Python brain (suppressing its errors)
    "/usr/local/bin/yups" --ce-handle "$exit_code" "$command"
}

echo "Let's uninstall yups. Press intro key"
read
./uninstall.sh
echo "Let's install yups. Press intro key"
read
./install.sh
source ~/.bashrc
echo "Let's install nano. Press intro key"
read
yups install nano
echo "Let's uninstall nano. Press intro key"
read
yups - nano
echo "Let's search nano-. Press intro key"
read
yups search nano-
echo "Let's search package that provides nano. Press intro key"
read
yups provides nano
echo "Let's test command not found handler. Press intro key"
read
nano
echo "Let's test specific distro commands."
if command -v apt >/dev/null 2>&1; then
    echo "Let's test command not found handler another native pm. Press intro key"
    read
    dnf install nano
    echo "Let's test command error. Press intro key"
    read
	apt instal nano;_test_ce_handle "apt instal nano"
fi
if command -v dnf >/dev/null 2>&1; then
    echo "Let's test command not found handler another native pm. Press intro key"
    read
    apt install nano
    echo "Let's test command error. Press intro key"
    read
	dnf instal nano;_test_ce_handle "dnf instal nano"
fi
if command -v pacman >/dev/null 2>&1; then
	echo "Let's test command not found handler another native pm. Press intro key"
    read
    dnf install nano
    echo "Let's test command error. Press intro key"
    read
    pacman instal nano;_test_ce_handle "pacman instal nano"
fi
if command -v zypper >/dev/null 2>&1; then
	echo "Let's test command not found handler another native pm. Press intro key"
    read
    dnf install nano
    echo "Let's test command error. Press intro key"
    read
    zypper instal nano;_test_ce_handle "zypper instal nano"
fi
