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

const (
	baseURL = "http://localhost:8080/api/v2/story/projects"
)

func main() {
	fmt.Println("🚀 Starting Phase 2 Integration Test")

	// 1. Create Project
	projectID, err := createProject("Test Dog", "Golden Retriever")
	if err != nil {
		fmt.Printf("❌ Failed to create project: %v\n", err)
		return
	}
	fmt.Printf("✅ Project created: %s\n", projectID)

	// 2. Upload Videos (using dummy files)
	// Create dummy video files if they don't exist
	dummyVideo := "test_video.mp4"
	if _, err := os.Stat(dummyVideo); os.IsNotExist(err) {
		// Try to find any mp4 file in current directory or create a fake one
		// For this test, we really need a valid video file for ffmpeg to work
		// Let's assume user has one, or we fail
		fmt.Println("⚠️  Please ensure 'test_video.mp4' exists in current directory for testing")
		// Check if there is any mp4 file in the directory
		files, _ := filepath.Glob("*.mp4")
		if len(files) > 0 {
			dummyVideo = files[0]
			fmt.Printf("ℹ️  Using existing video: %s\n", dummyVideo)
		} else {
			fmt.Println("❌ No video file found. Please provide a test video.")
			return
		}
	}

	if err := uploadVideos(projectID, dummyVideo); err != nil {
		fmt.Printf("❌ Failed to upload videos: %v\n", err)
		return
	}
	fmt.Println("✅ Videos uploaded")

	// 3. Upload Ending Image
	dummyImage := "test_image.jpg"
	if _, err := os.Stat(dummyImage); os.IsNotExist(err) {
		// Check for any jpg/png
		files, _ := filepath.Glob("*.jpg")
		if len(files) > 0 {
			dummyImage = files[0]
			fmt.Printf("ℹ️  Using existing image: %s\n", dummyImage)
		} else {
			files, _ = filepath.Glob("*.png")
			if len(files) > 0 {
				dummyImage = files[0]
				fmt.Printf("ℹ️  Using existing image: %s\n", dummyImage)
			} else {
				fmt.Println("❌ No image file found. Please provide a test image.")
				return
			}
		}
	}

	if err := uploadEndingImage(projectID, dummyImage); err != nil {
		fmt.Printf("❌ Failed to upload ending image: %v\n", err)
		return
	}
	fmt.Println("✅ Ending image uploaded")

	// 4. Generate Story
	if err := generateStory(projectID); err != nil {
		fmt.Printf("❌ Failed to start story generation: %v\n", err)
		return
	}
	fmt.Println("✅ Story generation started")

	// 5. Wait for 'generating_story' status (or completed if fast)
	// Actually we need to wait until it's done analyzing to set owner message?
	// The API allows setting owner message anytime. Let's set it now.

	// 6. Set Owner Message
	if err := setOwnerMessage(projectID, "寶貝，謝謝你來到我的生命中，我會永遠愛你！"); err != nil {
		fmt.Printf("❌ Failed to set owner message: %v\n", err)
		return
	}
	fmt.Println("✅ Owner message set")

	// 7. Poll for completion
	fmt.Println("⏳ Waiting for processing to complete...")
	for {
		status, err := getProjectStatus(projectID)
		if err != nil {
			fmt.Printf("❌ Failed to get status: %v\n", err)
			return
		}

		fmt.Printf("Status: %s\n", status)

		if status == "completed" {
			fmt.Println("🎉 Project completed successfully!")
			break
		} else if status == "failed" {
			fmt.Println("❌ Project failed processing")
			return
		}

		time.Sleep(5 * time.Second)
	}

	// 8. Verify Output
	// Check if final video URL is accessible
	// (In a real test we would download it)
	fmt.Printf("✅ Test finished. Please check http://localhost:8080/api/v2/story/projects/%s for results.\n", projectID)
}

func createProject(dogName, dogBreed string) (string, error) {
	data := map[string]string{
		"name":      "Test Project",
		"dog_name":  dogName,
		"dog_breed": dogBreed,
	}
	jsonData, _ := json.Marshal(data)

	resp, err := http.Post(baseURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result["project_id"].(string), nil
}

func uploadVideos(projectID, videoPath string) error {
	// Upload 5 videos to simulate full story
	url := fmt.Sprintf("%s/%s/videos", baseURL, projectID)

	// Try to find videos from 狗狗影片 folder first, then from root
	videoFiles := []string{}
	
	// First, check 狗狗影片 folder
	dogVideos, _ := filepath.Glob("./狗狗影片/*.mp4")
	for _, f := range dogVideos {
		videoFiles = append(videoFiles, f)
	}
	
	// If not enough, add from root folder
	if len(videoFiles) < 5 {
		rootVideos, _ := filepath.Glob("*.mp4")
		for _, f := range rootVideos {
			// Skip the final output files
			if f != "final.mp4" && f != "video_with_ending.mp4" && f != "tmp_test_gemini.mp4" {
				videoFiles = append(videoFiles, f)
			}
		}
	}

	// Select up to 5 different videos
	finalList := []string{}
	for i := 0; i < 5 && i < len(videoFiles); i++ {
		finalList = append(finalList, videoFiles[i])
	}
	
	// If we still don't have 5 videos, reuse from the beginning
	for len(finalList) < 5 {
		if len(videoFiles) > 0 {
			finalList = append(finalList, videoFiles[len(finalList)%len(videoFiles)])
		} else {
			finalList = append(finalList, videoPath)
		}
	}

	fmt.Printf("ℹ️  Uploading %d videos: %v\n", len(finalList), finalList)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for i, vPath := range finalList {
		file, err := os.Open(vPath)
		if err != nil {
			return err
		}
		defer file.Close()

		part, err := writer.CreateFormFile("videos", filepath.Base(vPath))
		if err != nil {
			return err
		}
		io.Copy(part, file)
		fmt.Printf("   - Uploaded video %d: %s\n", i+1, vPath)
	}
	writer.Close()

	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func uploadEndingImage(projectID, imagePath string) error {
	url := fmt.Sprintf("%s/%s/ending-image", baseURL, projectID)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer file.Close()

	part, err := writer.CreateFormFile("image", filepath.Base(imagePath))
	if err != nil {
		return err
	}
	io.Copy(part, file)
	writer.Close()

	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func generateStory(projectID string) error {
	url := fmt.Sprintf("%s/%s/generate", baseURL, projectID)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func setOwnerMessage(projectID, message string) error {
	url := fmt.Sprintf("%s/%s/owner-message", baseURL, projectID)
	data := map[string]string{"message": message}
	jsonData, _ := json.Marshal(data)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func getProjectStatus(projectID string) (string, error) {
	url := fmt.Sprintf("%s/%s", baseURL, projectID)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Status, nil
}
