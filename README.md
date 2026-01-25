# Fuck Sophos - The Ultimate Sophos Captive Portal Auto-Login for Linux

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Platform](https://img.shields.io/badge/platform-Linux-linux)
![Language](https://img.shields.io/badge/language-Go-00ADD8)

**The Sophos Client Authentication Agent (CAA) for Linux is effectively dead.**

It is long overdue for an update, unmaintained, and worst of all—**it does not work on Debian 13 (Trixie), Ubuntu 24.04+, or other modern Linux distributions** due to outdated dependencies and legacy SSL issues. Linux users have been left behind with unstable browser-based logins that time out during critical tasks.

**Fuck Sophos** is the modern, robust, high-performance solution. It is a "fire and forget" daemon written in Go that ensures you *never* lose internet connectivity again.

```
`
   ▄████████ ███    █▄   ▄████████    ▄█   ▄█▄         ▄████████  ▄██████▄     ▄███████▄    ▄█    █▄     ▄██████▄     ▄████████ 
  ███    ███ ███    ███ ███    ███   ███ ▄███▀        ███    ███ ███    ███   ███    ███   ███    ███   ███    ███   ███    ███ 
  ███    █▀  ███    ███ ███    █▀    ███▐██▀          ███    █▀  ███    ███   ███    ███   ███    ███   ███    ███   ███    █▀  
 ▄███▄▄▄     ███    ███ ███         ▄█████▀           ███        ███    ███   ███    ███  ▄███▄▄▄▄███▄▄ ███    ███   ███        
▀▀███▀▀▀     ███    ███ ███        ▀▀█████▄         ▀███████████ ███    ███ ▀█████████▀  ▀▀███▀▀▀▀███▀  ███    ███ ▀███████████ 
  ███        ███    ███ ███    █▄    ███▐██▄                 ███ ███    ███   ███          ███    ███   ███    ███          ███ 
  ███        ███    ███ ███    ███   ███ ▀███▄         ▄█    ███ ███    ███   ███          ███    ███   ███    ███    ▄█    ███ 
  ███        ████████▀  ████████▀    ███   ▀█▀       ▄████████▀   ▀██████▀   ▄████▀        ███    █▀     ▀██████▀   ▄████████▀  
                                     ▀                                                                                          
`
```

## 🚀 Why Use This?

If you are behind a Sophos XG Firewall (Cyberroam/Sophos) in a university, corporate office, or dorm, you know the struggle:
*   **The "Timeout" disconnect:** Your downloads fail and SSH sessions die because the portal session expired.
*   **Sophos CAA is broken:** The official Linux client is ancient and fails on modern kernels/glibc.
*   **Browser tabs are annoying:** Keeping a tab open to `10.10.10.100` just to have internet is a waste of RAM.

**This tool fixes it all.** It doesn't just "keep alive"—it aggressively manages your session to ensure 100% uptime.

## ✨ Key Features

*   **♻️ Aggressive Session Management:** Instead of weak keep-alive packets, it performs a full **Logout + Login cycle** every 30 minutes. This forces the firewall to refresh your session timer, preventing the dreaded "hard timeout".
*   **🌐 Real Connectivity Checks:** It doesn't trust the firewall's "Login Successful" message. It verifies actual internet access (pinging Cloudflare/Google) before waiting.
*   **⚡ Instant Recovery:** If the internet drops, it detects it immediately and enters an aggressive retry mode (every 30 seconds) until connection is restored.
*   **🐧 Native Systemd Integration:** Installs as a proper Linux service. Starts on boot, restarts automatically if it crashes.
*   **🛡️ Zero Dependencies:** Written in Go. Compiles to a single, static binary. No Python venvs, no pip requirements, no missing libraries.

## 📦 Installation

### Prerequisites
You need Go installed to build the binary (or download a release if available).
```bash
sudo apt update && sudo apt install golang
```

### Build from Source
```bash
git clone https://github.com/hxri-nxrxyxn/fuck-sophos.git
cd fuck-sophos
go build -o sophos-autologin main.go
```

## 🛠️ Usage

### 1. Install as a Background Service (Recommended)
This is the "set it and forget it" method. It installs a systemd service that runs automatically on boot.

```bash
sudo ./sophos-autologin --install --username "your_username" --password "your_password"
```

**That's it.** You now have permanent internet.

### 2. Manage the Service
Start the service:
```bash
sudo systemctl start fuck-sophos
```
Enable on boot:
```bash
sudo systemctl enable fuck-sophos
```
Check status:
```bash
sudo systemctl status fuck-sophos
```
View live logs:
```bash
journalctl -u fuck-sophos -f
```

### 3. Uninstall
If you ever leave the network:
```bash
sudo ./sophos-autologin --uninstall
```

### 4. One-Time Run (CLI Mode)
If you just want to login once without installing the service:
```bash
./sophos-autologin --username "user" --password "pass" --once
```

## 🔧 Technical Details

The tool targets the standard Sophos/Cyberroam captive portal endpoints:
- **Login:** `http://10.10.10.100:8090/login.xml`
- **Logout:** `http://10.10.10.100:8090/logout.xml`

It mimics a standard browser user-agent to avoid detection or filtering by the firewall policies.

---

### SEO Keywords & Related Issues
*Sophos Client Authentication Agent Linux download, Sophos CAA Debian 13 fix, Sophos XG Firewall auto login script, Cyberroam client for Linux, Sophos captive portal keepalive, sophos-auth-daemon, Sophos authentication bypass script, Linux network authentication automation.*
