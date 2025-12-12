package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const baseURL = "http://localhost:8080"

func main() {
	fmt.Println("🚀 Starting Phase 2 Integration Test")

	// Step 1: Create project
	projectID, err := createProject()
	if err != nil {
		fmt.Printf("❌ Failed to create project: %v\n", err)
		return
	}
	fmt.Printf("✅ Project created: %s\n", projectID)

	// Step 2: Upload videos
	if err := uploadVideos(projectID); err != nil {
		fmt.Printf("❌ Failed to upload videos: %v\n", err)
		return
	}
	fmt.Println("✅ Videos uploaded")

	// Step 3: Upload ending image
	if err := uploadEndingImage(projectID); err != nil {
		fmt.Printf("❌ Failed to upload ending image: %v\n", err)
		return
	}
	fmt.Println("✅ Ending image uploaded")

	// Step 4: Start generation
	if err := startGeneration(projectID); err != nil {
		fmt.Printf("❌ Failed to start generation: %v\n", err)
		return
	}
	fmt.Println("✅ Story generation started")

	// Step 5: Set owner message
	if err := setOwnerMessage(projectID); err != nil {
		fmt.Printf("❌ Failed to set owner message: %v\n", err)
		return
	}
	fmt.Println("✅ Owner message set")

	// Step 6: Wait for completion
	if err := waitForCompletion(projectID); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		return
	}

	fmt.Printf("🎉 Project completed successfully!\n")
	fmt.Printf("✅ Test finished. Please check http://localhost:8080/api/v2/story/projects/%s for results.\n", projectID)
}

func createProject() (string, error) {
	data := map[string]interface{}{
		"name":               "阿給辣的回憶",
		"dog_name":           "阿給辣",
		"dog_breed":          "吉娃娃",
		"owner_relationship": "媽媽",
		"story_mode":         "warm", // 明確指定感人模式
	}

	jsonData, _ := json.Marshal(data)
	resp, err := http.Post(baseURL+"/api/v2/story/projects", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	projectID, ok := result["project_id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid response: %+v", result)
	}

	return projectID, nil
}

func uploadVideos(projectID string) error {
	// 從狗狗影片資料夾上傳5個影片
	videoFiles, err := filepath.Glob("./狗狗影片/*.mp4")
	if err != nil {
		return err
	}

	// 限制為5個影片
	if len(videoFiles) > 5 {
		videoFiles = videoFiles[:5]
	}

	fmt.Printf("ℹ️  Uploading %d videos: %v\n", len(videoFiles), videoFiles)

	for i, videoPath := range videoFiles {
		if err := uploadSingleVideo(projectID, videoPath); err != nil {
			return fmt.Errorf("failed to upload video %d (%s): %v", i+1, videoPath, err)
		}
		fmt.Printf("   - Uploaded video %d: %s\n", i+1, videoPath)
	}

	return nil
}

func uploadSingleVideo(projectID, videoPath string) error {
	file, err := os.Open(videoPath)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("videos", filepath.Base(videoPath))
	if err != nil {
		return err
	}

	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	writer.Close()

	url := fmt.Sprintf("%s/api/v2/story/projects/%s/videos", baseURL, projectID)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func uploadEndingImage(projectID string) error {
	imagePath := "./狗狗影片/S__19439640.jpg"

	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", filepath.Base(imagePath))
	if err != nil {
		return err
	}

	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	writer.Close()

	url := fmt.Sprintf("%s/api/v2/story/projects/%s/ending-image", baseURL, projectID)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func startGeneration(projectID string) error {
	url := fmt.Sprintf("%s/api/v2/story/projects/%s/generate", baseURL, projectID)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("generation failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func setOwnerMessage(projectID string) error {
	data := map[string]string{
		"message": "阿給辣，從你來到我身邊的那一天起，我的生命就充滿了溫暖和快樂。每一個清晨，看到你搖著尾巴迎接我，就是我一整天最大的幸福。謝謝你無條件的愛，謝謝你陪我走過人生中最艱難的時光。你不只是我的寵物，你是我最親愛的家人，是我心中永遠的寶貝。媽媽會永遠愛你，永遠保護你。",
	}

	jsonData, _ := json.Marshal(data)
	url := fmt.Sprintf("%s/api/v2/story/projects/%s/owner-message", baseURL, projectID)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("set owner message failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func waitForCompletion(projectID string) error {
	fmt.Println("⏳ Waiting for processing to complete...")

	url := fmt.Sprintf("%s/api/v2/story/projects/%s", baseURL, projectID)

	for i := 0; i < 120; i++ { // 最多等待10分鐘
		time.Sleep(5 * time.Second)

		resp, err := http.Get(url)
		if err != nil {
			continue
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		status, ok := result["status"].(string)
		if !ok {
			continue
		}

		fmt.Printf("Status: %s\n", status)

		if status == "completed" {
			return nil
		}

		if status == "failed" {
			return fmt.Errorf("project processing failed")
		}
	}

	return fmt.Errorf("timeout waiting for completion")
}
