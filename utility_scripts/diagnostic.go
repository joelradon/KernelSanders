// utility_scripts/diagnostic.go

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// DiagnosticConfig holds configuration for diagnostics
type DiagnosticConfig struct {
	Port             string
	S3EndpointURL    string
	S3Region         string
	S3BucketName     string
	OpenAIEndpoint   string
	OpenAIAPIKey     string
	TelegramBotToken string
}

// CheckEnvironmentVariables verifies that all required environment variables are set
func CheckEnvironmentVariables(config *DiagnosticConfig) error {
	missingVars := []string{}

	if config.Port == "" {
		missingVars = append(missingVars, "PORT")
	}
	if config.S3EndpointURL == "" {
		missingVars = append(missingVars, "AWS_ENDPOINT_URL_S3")
	}
	if config.S3Region == "" {
		missingVars = append(missingVars, "AWS_REGION")
	}
	if config.S3BucketName == "" {
		missingVars = append(missingVars, "BUCKET_NAME")
	}
	if config.OpenAIAPIKey == "" {
		missingVars = append(missingVars, "OPENAI_KEY")
	}
	if config.TelegramBotToken == "" {
		missingVars = append(missingVars, "TELEGRAM_TOKEN")
	}

	if len(missingVars) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missingVars)
	}
	return nil
}

// CheckS3Connectivity verifies connectivity to the S3 bucket by listing objects
func CheckS3Connectivity(config *DiagnosticConfig) error {
	sess, err := session.NewSession(&aws.Config{
		Endpoint: aws.String(config.S3EndpointURL),
		Region:   aws.String(config.S3Region),
	})
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %v", err)
	}

	s3Svc := s3.New(sess)
	_, err = s3Svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:  aws.String(config.S3BucketName),
		MaxKeys: aws.Int64(1), // Minimal request
	})

	if err != nil {
		return fmt.Errorf("failed to connect to S3 bucket '%s': %v", config.S3BucketName, err)
	}

	return nil
}

// CheckOpenAIConnectivity verifies that the OpenAI API is reachable with the provided API key
func CheckOpenAIConnectivity(config *DiagnosticConfig) error {
	testQuery := map[string]interface{}{
		"model":    "gpt-4", // Corrected model name
		"messages": []map[string]string{{"role": "system", "content": "You are a test."}},
	}

	queryBytes, err := json.Marshal(testQuery)
	if err != nil {
		return fmt.Errorf("failed to marshal OpenAI test query: %v", err)
	}

	req, err := http.NewRequest("POST", config.OpenAIEndpoint, bytes.NewBuffer(queryBytes))
	if err != nil {
		return fmt.Errorf("failed to create OpenAI test request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.OpenAIAPIKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to OpenAI API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenAI API error: %s - %s", resp.Status, string(bodyBytes))
	}

	return nil
}

// CheckTelegramConnectivity verifies that the Telegram Bot API is reachable with the provided token
func CheckTelegramConnectivity(config *DiagnosticConfig) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", config.TelegramBotToken)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to Telegram API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram api responded with status %s: %s", resp.Status, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode Telegram API response: %v", err)
	}

	if ok, exists := result["ok"].(bool); !exists || !ok {
		return fmt.Errorf("telegram api returned not ok: %v", result)
	}

	return nil
}

// CheckPortAvailability verifies that the specified port is available for binding
func CheckPortAvailability(port string) error {
	address := fmt.Sprintf("0.0.0.0:%s", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("port %s is not available: %v", port, err)
	}
	listener.Close()
	return nil
}

func main() {
	// Initialize DiagnosticConfig from environment variables
	config := &DiagnosticConfig{
		Port:             os.Getenv("PORT"),
		S3EndpointURL:    os.Getenv("AWS_ENDPOINT_URL_S3"),
		S3Region:         os.Getenv("AWS_REGION"),
		S3BucketName:     os.Getenv("BUCKET_NAME"),
		OpenAIEndpoint:   os.Getenv("OPENAI_ENDPOINT"),
		OpenAIAPIKey:     os.Getenv("OPENAI_KEY"),
		TelegramBotToken: os.Getenv("TELEGRAM_TOKEN"),
	}

	// Define the interval for diagnostics
	diagnosticInterval := 5 * time.Minute

	for {
		log.Println("Starting Diagnostic Checks...")

		// 1. Check Environment Variables
		log.Println("1. Verifying Environment Variables...")
		if err := CheckEnvironmentVariables(config); err != nil {
			log.Printf("Environment Variables Check Failed: %v", err)
		} else {
			log.Println("Environment Variables Check Passed.")
		}

		// 2. Check S3 Connectivity
		log.Println("2. Checking S3 Connectivity...")
		if err := CheckS3Connectivity(config); err != nil {
			log.Printf("S3 Connectivity Check Failed: %v", err)
		} else {
			log.Println("S3 Connectivity Check Passed.")
		}

		// 3. Check OpenAI API Connectivity
		log.Println("3. Checking OpenAI API Connectivity...")
		if err := CheckOpenAIConnectivity(config); err != nil {
			log.Printf("OpenAI API Connectivity Check Failed: %v", err)
		} else {
			log.Println("OpenAI API Connectivity Check Passed.")
		}

		// 4. Check Telegram API Connectivity
		log.Println("4. Checking Telegram API Connectivity...")
		if err := CheckTelegramConnectivity(config); err != nil {
			log.Printf("Telegram API Connectivity Check Failed: %v", err)
		} else {
			log.Println("Telegram API Connectivity Check Passed.")
		}

		// 5. Check Port Availability
		log.Println("5. Checking Port Availability...")
		if err := CheckPortAvailability(config.Port); err != nil {
			log.Printf("Port Availability Check Failed: %v", err)
		} else {
			log.Println("Port Availability Check Passed.")
		}

		log.Println("Diagnostic Checks Completed.")

		// Wait for the next diagnostic run
		time.Sleep(diagnosticInterval)
	}
}
