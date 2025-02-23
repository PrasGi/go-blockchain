package utils

import (
	"fmt"
	"log"
	"net"
	"regexp"
	"time"
)

func IsFoundHost(host string, port uint16) bool {
	target := fmt.Sprintf("%s:%d", host, port)
	log.Printf("Attempting to connect to %s", target)

	_, err := net.DialTimeout("tcp", target, 1*time.Second)
	if err != nil {
		log.Printf("Failed to connect to %s: %v", target, err)
		return false
	}
	log.Printf("Successfully connected to %s", target)
	return true
}

var PATTERN = regexp.MustCompile(`((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?\.){3})(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)`)

func FindNeighbors(myHost string, myPort uint16, startIp uint8, endIp uint8, startPort uint16, endPort uint16) []string {
	if myHost != "127.0.0.1" {
		myHost = "127.0.0.1" // Pastikan hanya mencari di localhost
	}
	address := fmt.Sprintf("%s:%d", myHost, myPort)
	log.Printf("Starting neighbor search with myHost: %s, myPort: %d, Port range: %d-%d",
		myHost, myPort, startPort, endPort)

	neighbors := make([]string, 0)
	for port := startPort; port <= endPort; port += 1 {
		log.Printf("Checking port: %d", port)
		if port == myPort { // Skip port saat ini
			log.Printf("Skipping current port: %d", port)
			continue
		}
		guessTarget := fmt.Sprintf("127.0.0.1:%d", port)
		log.Printf("Trying neighbor: %s", guessTarget)
		if guessTarget != address && IsFoundHost("127.0.0.1", port) {
			log.Printf("Found active neighbor: %s", guessTarget)
			neighbors = append(neighbors, guessTarget)
		}
	}
	log.Printf("Finished neighbor search, found neighbors: %v", neighbors)
	return neighbors
}

func GetHost() string {
	// hostname, err := os.Hostname()
	// if err != nil {
	// 	log.Printf("Failed to get hostname, using default 127.0.0.1: %v", err)
	// 	return "127.0.0.1"
	// }
	// log.Printf("Hostname: %s", hostname)
	// address, err := net.LookupHost(hostname)
	// if err != nil {
	// 	log.Printf("Failed to lookup host %s, using default 127.0.0.1: %v", hostname, err)
	// 	return "127.0.0.1"
	// }
	// log.Printf("Resolved IP addresses for %s: %v", hostname, address)
	// return address[0]
	return "127.0.0.1"
}
