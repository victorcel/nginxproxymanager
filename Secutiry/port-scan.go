package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

// Config holds the application configuration
type Config struct {
	Port            int
	FailedThreshold int
}

// Monitor handles the port monitoring logic
type Monitor struct {
	config    Config
	failedIPs map[string]int
	mutex     sync.RWMutex
	ipRegexp  *regexp.Regexp
}

// NewMonitor creates a new Monitor instance
func NewMonitor(config Config) (*Monitor, error) {
	ipRegex, err := regexp.Compile(`IP\s(\d+\.\d+\.\d+\.\d+)\.\d+\s>\s(\d+\.\d+\.\d+\.\d+)\.\d+`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile IP regex: %w", err)
	}

	return &Monitor{
		config:    config,
		failedIPs: make(map[string]int),
		ipRegexp:  ipRegex,
	}, nil
}

// blockIP blocks an IP using UFW
func (m *Monitor) blockIP(ip string) error {
	cmd := exec.Command("sudo", "ufw", "deny", "from", ip)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to block IP %s: %w", ip, err)
	}
	log.Printf("Blocked IP: %s", ip)
	return nil
}

// processLine processes a single line from tcpdump
func (m *Monitor) processLine(buffer *strings.Builder) {
	if !strings.Contains(buffer.String(), "HTTP/1.1 401 Unauthorized") {
		return
	}

	matches := m.ipRegexp.FindStringSubmatch(buffer.String())
	if len(matches) < 3 {
		return
	}

	sourceIP := matches[2]
	m.mutex.Lock()
	m.failedIPs[sourceIP]++
	attempts := m.failedIPs[sourceIP]
	m.mutex.Unlock()

	log.Printf("Failed attempt from IP: %s (Total: %d)", sourceIP, attempts)

	if attempts >= m.config.FailedThreshold {
		if err := m.blockIP(sourceIP); err != nil {
			log.Printf("Error blocking IP: %v", err)
		}
		m.mutex.Lock()
		m.failedIPs[sourceIP] = 0
		m.mutex.Unlock()
	}
}

// MonitorPort starts monitoring the specified port
func (m *Monitor) MonitorPort() error {
	cmd := exec.Command("sudo", "tcpdump", "-nn", "-A", "-s", "0", "-i", "any",
		fmt.Sprintf("tcp port %d or tcp port 443", m.config.Port))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tcpdump: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	var buffer strings.Builder

	for scanner.Scan() {
		buffer.WriteString(scanner.Text())
		buffer.WriteString("\n")

		if strings.Contains(scanner.Text(), "HTTP/1.1") {
			m.processLine(&buffer)
			buffer.Reset()
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading tcpdump output: %w", err)
	}

	return nil
}

func main() {
	config := Config{
		Port:            81,
		FailedThreshold: 3,
	}

	monitor, err := NewMonitor(config)
	if err != nil {
		log.Fatalf("Failed to create monitor: %v", err)
	}

	log.Printf("Starting monitoring on port %d", config.Port)

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		<-sigChan
		log.Println("Monitoring stopped.")
		os.Exit(0)
	}()

	if err := monitor.MonitorPort(); err != nil {
		log.Fatalf("Error monitoring port: %v", err)
	}
}
