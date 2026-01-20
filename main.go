package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	portalURL         = "http://10.10.10.100:8090/login.xml"
	keepaliveURL      = "http://10.10.10.100:8090/live"
	refererURL        = "http://10.10.10.100:8090/httpclient.html"
	loginInterval     = 55 * time.Minute
	keepaliveInterval = 5 * time.Minute
	retryDelay        = 2 * time.Minute
	serviceName       = "sophos-autologin"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGray   = "\033[90m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorRed    = "\033[31m"
)

var greetings = []string{
	"Beep boop, authenticating your digital existence...",
	"Knocking on Sophos's door... with a battering ram",
	"Convincing the firewall you're totally legit...",
	"Bribing the captive portal with ones and zeros...",
	"Performing the sacred ritual of network authentication...",
	"Sweet-talking the gateway into letting you through...",
	"Proving to the firewall you're not a robot... wait...",
	"Negotiating with the cyber bouncer...",
	"Hacking the mainframe... just kidding, logging in normally",
	"Sending carrier pigeons with your credentials...",
	"Asking the network gods for passage...",
	"Sprinkling some authentication fairy dust...",
	"Rolling the dice of network connectivity...",
	"Sliding into the portal's DMs...",
	"Offering a sacrifice to the ping gods...",
}

const banner = `
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

const systemdServiceTemplate = `[Unit]
Description=Sophos Captive Portal Auto-Login
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s --username %s --password %s
Restart=always
RestartSec=10
User=%s

[Install]
WantedBy=multi-user.target
`

type Config struct {
	Username    string
	Password    string
	OnceMode    bool
	Install     bool
	Uninstall   bool
}

func main() {
	// Seed random for greetings
	rand.Seed(time.Now().UnixNano())

	// Disable log timestamps (we'll format our own)
	log.SetFlags(0)

	config := parseFlags()

	// Handle install/uninstall
	if config.Install {
		if err := installSystemdService(config.Username, config.Password); err != nil {
			log.Fatalf("%sInstallation failed: %v%s\n", colorRed, err, colorReset)
		}
		fmt.Printf("%sSystemd service installed successfully!%s\n", colorGreen, colorReset)
		fmt.Printf("%sRun: sudo systemctl start %s%s\n", colorGray, serviceName, colorReset)
		fmt.Printf("%sRun: sudo systemctl enable %s  (to start on boot)%s\n", colorGray, serviceName, colorReset)
		return
	}

	if config.Uninstall {
		if err := uninstallSystemdService(); err != nil {
			log.Fatalf("%sUninstallation failed: %v%s\n", colorRed, err, colorReset)
		}
		fmt.Printf("%sSystemd service uninstalled successfully!%s\n", colorGreen, colorReset)
		return
	}

	// Validate credentials
	if config.Username == "" || config.Password == "" {
		log.Fatal("Username and password are required")
	}

	// Print banner
	fmt.Print(colorCyan + banner + colorReset)
	fmt.Println()

	fmt.Printf("%sStarting Sophos Auto-Login%s\n", colorGreen, colorReset)
	fmt.Printf("%sUser: %s%s\n", colorGray, config.Username, colorReset)
	fmt.Printf("%sPortal: %s%s\n\n", colorGray, portalURL, colorReset)

	if config.OnceMode {
		fmt.Printf("%sRunning in once mode - will login once and exit%s\n", colorYellow, colorReset)
		if err := performLogin(config.Username, config.Password); err != nil {
			log.Fatalf("Login failed: %v", err)
		}
		fmt.Printf("%sLogin successful!%s\n", colorGreen, colorReset)
		return
	}

	// Initial login
	if err := performLogin(config.Username, config.Password); err != nil {
		fmt.Printf("Initial login failed: %v. Will retry in %v\n", err, retryDelay)
	}

	// Start dual-ticker system
	keepaliveTicker := time.NewTicker(keepaliveInterval)
	loginTicker := time.NewTicker(loginInterval)
	defer keepaliveTicker.Stop()
	defer loginTicker.Stop()

	fmt.Printf("%sAuto-login enabled:%s\n", colorGreen, colorReset)
	fmt.Printf("%s  - Keepalive every %v%s\n", colorGray, keepaliveInterval, colorReset)
	fmt.Printf("%s  - Full re-login every %v%s\n\n", colorGray, loginInterval, colorReset)

	for {
		select {
		case <-keepaliveTicker.C:
			if err := performKeepalive(config.Username); err != nil {
				fmt.Printf("Keepalive failed: %v\n", err)
			}
		case <-loginTicker.C:
			if err := performLogin(config.Username, config.Password); err != nil {
				fmt.Printf("Re-login failed: %v. Will retry in %v\n", err, retryDelay)
				time.Sleep(retryDelay)
				if err := performLogin(config.Username, config.Password); err != nil {
					fmt.Printf("Retry failed: %v. Will wait until next scheduled login\n", err)
				}
			}
		}
	}
}

func parseFlags() *Config {
	config := &Config{
		Username: "event",
		Password: "daily@net",
	}

	username := flag.String("username", "", "Username for authentication")
	password := flag.String("password", "", "Password for authentication")
	once := flag.Bool("once", false, "Login once and exit (useful for testing)")
	install := flag.Bool("install", false, "Install systemd service")
	uninstall := flag.Bool("uninstall", false, "Uninstall systemd service")

	flag.Parse()

	config.OnceMode = *once
	config.Install = *install
	config.Uninstall = *uninstall

	if *username != "" {
		config.Username = *username
	}
	if *password != "" {
		config.Password = *password
	}

	return config
}

func installSystemdService(username, password string) error {
	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("installation requires root privileges. Run with sudo")
	}

	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Copy binary to /usr/local/bin if not already there
	targetPath := "/usr/local/bin/" + serviceName
	if exePath != targetPath {
		input, err := os.ReadFile(exePath)
		if err != nil {
			return fmt.Errorf("failed to read executable: %w", err)
		}

		if err := os.WriteFile(targetPath, input, 0755); err != nil {
			return fmt.Errorf("failed to copy executable to %s: %w", targetPath, err)
		}
		fmt.Printf("%sCopied binary to %s%s\n", colorGray, targetPath, colorReset)
	}

	// Get the current user (who ran sudo)
	currentUser := os.Getenv("SUDO_USER")
	if currentUser == "" {
		currentUser = "root"
	}

	// Create systemd service file
	serviceContent := fmt.Sprintf(systemdServiceTemplate, targetPath, username, password, currentUser)
	servicePath := "/etc/systemd/system/" + serviceName + ".service"

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to create service file: %w", err)
	}
	fmt.Printf("%sCreated service file: %s%s\n", colorGray, servicePath, colorReset)

	// Reload systemd
	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}
	fmt.Printf("%sReloaded systemd daemon%s\n", colorGray, colorReset)

	return nil
}

func uninstallSystemdService() error {
	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("uninstallation requires root privileges. Run with sudo")
	}

	servicePath := "/etc/systemd/system/" + serviceName + ".service"

	// Stop the service if running
	cmd := exec.Command("systemctl", "stop", serviceName)
	cmd.Run() // Ignore error if service not running

	// Disable the service
	cmd = exec.Command("systemctl", "disable", serviceName)
	cmd.Run() // Ignore error if service not enabled

	// Remove service file
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}
	fmt.Printf("%sRemoved service file: %s%s\n", colorGray, servicePath, colorReset)

	// Reload systemd
	cmd = exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}
	fmt.Printf("%sReloaded systemd daemon%s\n", colorGray, colorReset)

	// Optionally remove binary
	binaryPath := "/usr/local/bin/" + serviceName
	if _, err := os.Stat(binaryPath); err == nil {
		fmt.Printf("%sBinary still exists at %s%s\n", colorYellow, binaryPath, colorReset)
		fmt.Printf("%sRemove manually if desired: sudo rm %s%s\n", colorGray, binaryPath, colorReset)
	}

	return nil
}

func randomGreeting() string {
	return greetings[rand.Intn(len(greetings))]
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}

func performLogin(username, password string) error {
	greeting := randomGreeting()
	fmt.Printf("%s[%s]%s %s\n", colorGray, timestamp(), colorReset, greeting)

	// Generate timestamp (milliseconds since epoch)
	ts := time.Now().UnixMilli()

	// Prepare form data
	formData := url.Values{
		"mode":        {"191"},
		"username":    {username},
		"password":    {password},
		"a":           {fmt.Sprintf("%d", ts)},
		"producttype": {"0"},
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create the request
	req, err := http.NewRequest("POST", portalURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-IN,en-GB;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://10.10.10.100:8090")
	req.Header.Set("Referer", refererURL)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Check for failure indicators
	bodyStr := string(body)
	if strings.Contains(strings.ToLower(bodyStr), "failed") ||
		strings.Contains(strings.ToLower(bodyStr), "error") ||
		strings.Contains(strings.ToLower(bodyStr), "invalid") {
		return fmt.Errorf("login rejected: %s", bodyStr)
	}

	fmt.Printf("%s[%s]%s Login successful\n", colorGray, timestamp(), colorReset)
	return nil
}

func performKeepalive(username string) error {
	fmt.Printf("%s[%s]%s Sending keepalive ping...\n", colorGray, timestamp(), colorReset)

	// Generate timestamp
	ts := time.Now().UnixMilli()

	// Build keepalive URL
	keepaliveReq := fmt.Sprintf("%s?mode=192&username=%s&a=%d&producttype=0",
		keepaliveURL,
		url.QueryEscape(username),
		ts)

	// Create HTTP client
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create request
	req, err := http.NewRequest("GET", keepaliveReq, nil)
	if err != nil {
		return fmt.Errorf("failed to create keepalive request: %w", err)
	}

	// Add headers
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-IN,en-GB;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", refererURL)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("keepalive request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read keepalive response: %w", err)
	}

	// Check status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("keepalive HTTP %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("%s[%s]%s Keepalive sent\n", colorGray, timestamp(), colorReset)
	return nil
}
