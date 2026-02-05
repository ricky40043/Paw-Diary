package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	f, _ := os.Create("debug_result.txt")
	defer f.Close()

	// Simple .env parser since we might not have godotenv in standalone
	envMap := make(map[string]string)
	data, err := ioutil.ReadFile(".env")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				envMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	apiKey := envMap["AI_API_KEY"]
	model := envMap["AI_MODEL_NAME"]
	endpoint := envMap["AI_API_ENDPOINT"]

	msg := fmt.Sprintf("Testing API...\nEndpoint: %s\nModel: %s\nKey: %s...%s\n", endpoint, model, apiKey[:5], apiKey[len(apiKey)-5:])
	fmt.Print(msg)
	f.WriteString(msg)

	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Hi, are you working?",
			},
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://pawdiary.app")
	req.Header.Set("X-Title", "Paw Diary Debug")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("Error: %v\n", err)
		fmt.Print(errMsg)
		f.WriteString(errMsg)
		return
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	respMsg := fmt.Sprintf("Status: %s\nBody: %s\n", resp.Status, string(body))
	fmt.Print(respMsg)
	f.WriteString(respMsg)
}
