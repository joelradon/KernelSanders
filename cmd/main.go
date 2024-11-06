// cmd/main.go

package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"KernelSandersBot/internal/app"
	"KernelSandersBot/internal/types"
)

func main() {
	botApp := app.NewApp()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Handle web page requests
			botApp.HandleWebRequest(w, r)
			return
		}

		// Handle Telegram updates
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var update types.TelegramUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			log.Printf("Failed to decode update: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		go botApp.HandleUpdate(&update) // Added HandleUpdate method to process updates

		w.WriteHeader(http.StatusOK)
	})

	// Updated port handling to ensure correct binding
	port := os.Getenv("PORT")
	if port == "" {
		port = "0.0.0.0:8080" // Explicitly bind to all IPv4 interfaces
	} else if !strings.Contains(port, ":") {
		port = "0.0.0.0:" + port // Ensure port is prefixed with "0.0.0.0:"
	} else if strings.HasPrefix(port, ":") {
		port = "0.0.0.0" + port // Convert ":8080" to "0.0.0.0:8080"
	}
	log.Printf("Starting server on %s...", port)

	// Log the actual network address being listened on
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to bind to address %s: %v", port, err)
	}
	defer listener.Close()

	log.Printf("Server successfully bound to %s", listener.Addr().String())

	if err := http.Serve(listener, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
