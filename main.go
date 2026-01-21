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
	portalLoginURL    = "http://10.10.10.100:8090/login.xml"
	portalLogoutURL   = "http://10.10.10.100:8090/logout.xml"
	refererURL        = "http://10.10.10.100:8090/"
	reloginInterval   = 30 * time.Minute // Logout + Login every 30 minutes
	retryDelay        = 30 * time.Second // Quick retry on failure
	serviceName       = "fuck-sophos"
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
	"Respectfully requesting internet access...",
	"Politely asking the firewall to step aside...",
	"Establishing connection to the series of tubes...",
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
Description=Fuck Sophos Captive Portal Auto-Login
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

	fmt.Printf("%sFuck Sophos - Never Lose Internet%s\n", colorGreen, colorReset)
	fmt.Printf("%sUser: %s%s\n", colorGray, config.Username, colorReset)
	fmt.Printf("%sPortal: %s%s\n\n", colorGray, portalLoginURL, colorReset)

	if config.OnceMode {
		fmt.Printf("%sRunning in once mode - will login once and exit%s\n", colorYellow, colorReset)
		if err := performLogoutLogin(config.Username, config.Password); err != nil {
			log.Fatalf("Login failed: %v", err)
		}
		fmt.Printf("%sLogin successful!%s\n", colorGreen, colorReset)
		return
	}

	// Initial login
	if err := performLogoutLogin(config.Username, config.Password); err != nil {
		fmt.Printf("%sInitial login failed: %v%s\n", colorRed, err, colorReset)
		fmt.Printf("%sRetrying in %v...%s\n", colorYellow, retryDelay, colorReset)
		time.Sleep(retryDelay)
		if err := performLogoutLogin(config.Username, config.Password); err != nil {
			log.Fatalf("Retry failed: %v. Cannot continue.", err)
		}
	}

	// Start re-login loop
	ticker := time.NewTicker(reloginInterval)
	defer ticker.Stop()

	fmt.Printf("%sAuto re-login enabled:%s\n", colorGreen, colorReset)
	fmt.Printf("%s  - Logout + Login cycle every %v%s\n", colorGray, reloginInterval, colorReset)
	fmt.Printf("%s  - Aggressive retry on any failure%s\n\n", colorGray, colorReset)

	for range ticker.C {
		if err := performLogoutLogin(config.Username, config.Password); err != nil {
			fmt.Printf("%sRe-login failed: %v%s\n", colorRed, err, colorReset)
			
			// Immediate retry on failure
			for i := 1; i <= 3; i++ {
				fmt.Printf("%sRetry attempt %d/%d in %v...%s\n", colorYellow, i, 3, retryDelay, colorReset)
				time.Sleep(retryDelay)
				
				if err := performLogoutLogin(config.Username, config.Password); err == nil {
					fmt.Printf("%sRetry successful!%s\n", colorGreen, colorReset)
					break
				}
				
				if i == 3 {
					fmt.Printf("%sAll retries failed. Will try again at next scheduled interval.%s\n", colorRed, colorReset)
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

func performLogoutLogin(username, password string) error {
	// First, try to logout (ignore errors as we might not be logged in)
	performLogout(username)
	
	// Small delay between logout and login
	time.Sleep(500 * time.Millisecond)
	
	// Now perform fresh login
	if err := performLogin(username, password); err != nil {
		return err
	}
	
	// Verify internet connectivity
	time.Sleep(1 * time.Second) // Give the portal a moment to activate
	if err := verifyInternetConnectivity(); err != nil {
		return fmt.Errorf("login succeeded but internet not working: %w", err)
	}
	
	return nil
}

func performLogout(username string) {
	fmt.Printf("%s[%s]%s Logging out...\n", colorGray, timestamp(), colorReset)

	ts := time.Now().UnixMilli()

	formData := url.Values{
		"mode":        {"193"},
		"username":    {username},
		"a":           {fmt.Sprintf("%d", ts)},
		"producttype": {"0"},
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("POST", portalLogoutURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return // Ignore logout errors
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", refererURL)

	resp, err := client.Do(req)
	if err != nil {
		return // Ignore logout errors
	}
	defer resp.Body.Close()

	fmt.Printf("%s[%s]%s Logged out\n", colorGray, timestamp(), colorReset)
}

func performLogin(username, password string) error {
	greeting := randomGreeting()
	fmt.Printf("%s[%s]%s %s\n", colorGray, timestamp(), colorReset, greeting)

	ts := time.Now().UnixMilli()

	formData := url.Values{
		"mode":        {"191"},
		"username":    {username},
		"password":    {password},
		"a":           {fmt.Sprintf("%d", ts)},
		"producttype": {"0"},
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("POST", portalLoginURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", refererURL)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	bodyStr := string(body)
	if strings.Contains(strings.ToLower(bodyStr), "failed") ||
		strings.Contains(strings.ToLower(bodyStr), "error") ||
		strings.Contains(strings.ToLower(bodyStr), "invalid") {
		return fmt.Errorf("login rejected: %s", bodyStr)
	}

	fmt.Printf("%s[%s]%s %sLogin successful%s\n", colorGray, timestamp(), colorReset, colorGreen, colorReset)
	return nil
}

func verifyInternetConnectivity() error {
	fmt.Printf("%s[%s]%s Verifying internet connectivity...\n", colorGray, timestamp(), colorReset)
	
	// Try to reach common reliable endpoints
	testURLs := []string{
		"http://1.1.1.1",
		"http://8.8.8.8",
		"http://www.google.com",
	}
	
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	
	for _, testURL := range testURLs {
		req, err := http.NewRequest("GET", testURL, nil)
		if err != nil {
			continue
		}
		
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			fmt.Printf("%s[%s]%s %sInternet is working%s\n", colorGray, timestamp(), colorReset, colorGreen, colorReset)
			return nil
		}
	}
	
	return fmt.Errorf("cannot reach external internet")
}
