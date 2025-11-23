# **YUPS: A Shell Helper**

YUPS is a proof-of-concept shell utility designed to intercept common shell errors and provide helpful suggestions.

This MVP (Minimum Viable Product) focuses on capturing:

1. **Command Not Found** errors for common package managers.  
2. **Command Errors** from package manager commands.  
3. **User Queries** via the yups command for logging.

## **Requirements**

* python3 (3.7 or higher)  
* bash (with a \~/.bashrc file)  
* sudo (to install the script to /usr/local/bin)  
* sed (for safely modifying .bashrc)

## **1\. Installation**

1. Clone this repository or download the yups, install.sh, and uninstall.sh files.  
2. Make the scripts executable:  
   chmod \+x install.sh uninstall.sh yups

3. Run the installer **as your normal user** (not as root):  
   ./install.sh

   The script will ask for your sudo password when it needs to copy the yups executable to /usr/local/bin.  
4. Reload your shell configuration to activate the hooks:  
   source \~/.bashrc

   (Or simply open a new terminal).

## **2\. Testing the Features**

You can test the three core features:

### **Test 1: Command Not Found (CNF) Hook**

If you are on a system that uses apt (like Ubuntu), try to use a different package manager:

dnf install nano

YUPS will intercept this, and you should see:

YUPS: Command 'dnf' not found.  
YUPS: Maybe you meant 'apt'?  
YUPS: Try 'yups dnf install nano'

### **Test 2: Command Error (CE) Hook**

Type a command for your system's package manager with a typo:

sudo apt instal nano

The command will fail as usual. Then, YUPS will intercept the error and print:

\---  
YUPS: The command 'sudo apt instal nano' failed (Code: 100).  
YUPS: Did you mean 'install'?  
YUPS: Try 'yups sudo apt instal nano'  
\---

### **Test 3: YUPS Log Command**

Run the yups command with any query:

yups how do i install nano

YUPS will log your query and system information. You can check the created files:

\# See your system's cached configuration  
cat \~/.yups/config.json

\# See the log of your query  
cat \~/.yups/yups.log

## **3\. Uninstallation**

1. Run the uninstaller **as your normal user**:  
   ./uninstall.sh

2. The script will remove the executable and cleanly remove the hooks from your .bashrc file.  
3. It will ask you if you also want to remove the \~/.yups directory, which contains your logs and configuration.  
4. Reload your shell to complete the uninstallation:  
   source \~/.bashrc  
