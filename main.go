package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// ============================================================================
// Data Structures
// ============================================================================

// Phase 1: Single video POC
type Job struct {
	ID             string      `json:"id"`
	Status         string      `json:"status"` // pending, processing, completed, failed
	VideoPath      string      `json:"video_path"`
	FramesDir      string      `json:"frames_dir"`
	Segments       []Segment   `json:"segments"`
	Highlights     []Highlight `json:"highlights"`
	HighlightVideo string      `json:"highlight_video,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	Error          string      `json:"error,omitempty"`
}

// Phase 2: Multi-video story generation
type Project struct {
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	DogName           string      `json:"dog_name"`
	DogBreed          string      `json:"dog_breed,omitempty"`
	OwnerRelationship string      `json:"owner_relationship,omitempty"` // 主人與毛小孩的關係 (媽媽/爸爸/小主人等)
	EndingImage       string      `json:"ending_image,omitempty"`       // 結尾圖片路徑
	OwnerMessage      string      `json:"owner_message,omitempty"`      // 主人想對狗狗說的話
	Status            string      `json:"status"`                       // pending, analyzing, generating_story, generating_video, completed, failed
	Videos            []VideoInfo `json:"videos"`
	Story             *Story      `json:"story,omitempty"`
	FinalVideo        string      `json:"final_video,omitempty"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
	Error             string      `json:"error,omitempty"`
}

type VideoInfo struct {
	ID           string      `json:"id"`
	OriginalName string      `json:"original_name"`
	Path         string      `json:"path"`
	Duration     float64     `json:"duration"`
	FramesDir    string      `json:"frames_dir"`
	Analyzed     bool        `json:"analyzed"`
	Segments     []Segment   `json:"segments,omitempty"`
	Highlights   []Highlight `json:"highlights,omitempty"`
}

type Story struct {
	Title        string         `json:"title"`
	Chapters     []StoryChapter `json:"chapters"`
	OwnerMessage string         `json:"owner_message,omitempty"` // 主人想對狗狗說的話
	DogResponse  string         `json:"dog_response,omitempty"`  // 狗狗回應主人（AI 生成）
	FinalMessage string         `json:"final_message,omitempty"` // 兼容舊代碼，可能不再使用
}

type StoryChapter struct {
	Index     int     `json:"index"`
	Narration string  `json:"narration"`
	VideoID   string  `json:"video_id"`
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time"`
	AudioPath string  `json:"audio_path,omitempty"`
	Duration  float64 `json:"duration"`
}

type Segment struct {
	Index      int       `json:"segment_index"`
	Start      float64   `json:"start"`
	End        float64   `json:"end"`
	FramePaths []string  `json:"frame_paths"`
	Analysis   *Analysis `json:"analysis,omitempty"`
}

type Analysis struct {
	HasDog          bool   `json:"has_dog"`
	HasHuman        bool   `json:"has_human"`
	InteractionType string `json:"interaction_type"`
	Emotion         string `json:"emotion"`
	ShortCaption    string `json:"short_caption"`
}

type Highlight struct {
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Caption     string  `json:"caption"`
	Interaction string  `json:"interaction"`
	Emotion     string  `json:"emotion"`
}

// ============================================================================
// Global State
// ============================================================================

var (
	// Phase 1 storage
	jobs      = make(map[string]*Job)
	jobsMutex sync.RWMutex

	// Phase 2 storage
	projects      = make(map[string]*Project)
	projectsMutex sync.RWMutex

	storagePath   string
	aiAPIKey      string
	aiAPIEndpoint string
)

// ============================================================================
// Main Entry Point
// ============================================================================

func main() {
	// Load environment variables
	godotenv.Load()

	port := getEnv("PORT", "8080")
	storagePath = getEnv("STORAGE_PATH", "./storage")
	aiAPIKey = getEnv("AI_API_KEY", "")
	aiAPIEndpoint = getEnv("AI_API_ENDPOINT", "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent")

	// Create storage directories
	createStorageDirectories()

	// Setup Gin router
	router := gin.Default()

	// CORS middleware
	router.Use(corsMiddleware())

	// Serve static files for frontend
	router.Static("/assets", "./frontend/dist/assets")
	router.StaticFile("/", "./frontend/dist/index.html")

	// Serve storage files
	router.Static("/storage", storagePath)

	// ========================================================================
	// Phase 1 APIs - All in one place, not separated
	// ========================================================================

	// POST /api/v1/poc/jobs - Upload video and create job
	router.POST("/api/v1/poc/jobs", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
			return
		}

		// Validate file extension
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext != ".mp4" && ext != ".mov" && ext != ".avi" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Only video files are supported"})
			return
		}

		// Create job
		jobID := uuid.New().String()
		videoDir := filepath.Join(storagePath, "videos", jobID)
		os.MkdirAll(videoDir, 0755)

		videoPath := filepath.Join(videoDir, "original"+ext)
		if err := c.SaveUploadedFile(file, videoPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
			return
		}

		job := &Job{
			ID:        jobID,
			Status:    "pending",
			VideoPath: videoPath,
			FramesDir: filepath.Join(videoDir, "frames"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		jobsMutex.Lock()
		jobs[jobID] = job
		jobsMutex.Unlock()

		// Start processing in background
		go processJob(jobID)

		c.JSON(http.StatusOK, gin.H{
			"job_id": jobID,
			"status": "pending",
		})
	})

	// GET /api/v1/poc/jobs/:jobId - Get job status and results
	router.GET("/api/v1/poc/jobs/:jobId", func(c *gin.Context) {
		jobID := c.Param("jobId")

		jobsMutex.RLock()
		job, exists := jobs[jobID]
		jobsMutex.RUnlock()

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}

		response := gin.H{
			"id":         job.ID,
			"status":     job.Status,
			"created_at": job.CreatedAt,
			"updated_at": job.UpdatedAt,
		}

		if job.Error != "" {
			response["error"] = job.Error
		}

		if job.Status == "completed" {
			response["highlights"] = job.Highlights
			if job.HighlightVideo != "" {
				// Fix: Construct URL directly to avoid path prefix issues
				// The file is always at storage/videos/{jobID}/highlight.mp4
				response["highlight_video_url"] = fmt.Sprintf("/storage/videos/%s/highlight.mp4", job.ID)
			}
		}

		c.JSON(http.StatusOK, response)
	})

	// GET /api/v1/poc/jobs - List all jobs
	router.GET("/api/v1/poc/jobs", func(c *gin.Context) {
		jobsMutex.RLock()
		defer jobsMutex.RUnlock()

		jobList := make([]*Job, 0, len(jobs))
		for _, job := range jobs {
			jobList = append(jobList, job)
		}

		c.JSON(http.StatusOK, gin.H{
			"jobs":  jobList,
			"total": len(jobList),
		})
	})

	// Health check
	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now(),
		})
	})

	// ========================================================================
	// Phase 2 APIs - Multi-video Story Generation
	// ========================================================================

	// POST /api/v2/story/projects - Create a new project
	router.POST("/api/v2/story/projects", func(c *gin.Context) {
		var req struct {
			Name              string `json:"name" binding:"required"`
			DogName           string `json:"dog_name" binding:"required"`
			DogBreed          string `json:"dog_breed"`
			OwnerRelationship string `json:"owner_relationship"` // 媽媽/爸爸/小主人等
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
			return
		}

		// 預設關係為「主人」
		if req.OwnerRelationship == "" {
			req.OwnerRelationship = "主人"
		}

		projectID := uuid.New().String()
		project := &Project{
			ID:                projectID,
			Name:              req.Name,
			DogName:           req.DogName,
			DogBreed:          req.DogBreed,
			OwnerRelationship: req.OwnerRelationship,
			Status:            "pending",
			Videos:            []VideoInfo{},
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		projectsMutex.Lock()
		projects[projectID] = project
		projectsMutex.Unlock()

		c.JSON(http.StatusOK, gin.H{
			"project_id": projectID,
			"status":     "pending",
		})
	})

	// POST /api/v2/story/projects/:projectId/ending-image - Upload ending image
	router.POST("/api/v2/story/projects/:projectId/ending-image", func(c *gin.Context) {
		projectID := c.Param("projectId")

		projectsMutex.RLock()
		project, exists := projects[projectID]
		projectsMutex.RUnlock()

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		file, err := c.FormFile("image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No image uploaded"})
			return
		}

		// 驗證圖片格式
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Only JPG and PNG images are supported"})
			return
		}

		projectDir := filepath.Join(storagePath, "projects", projectID)
		imagePath := filepath.Join(projectDir, "ending_image"+ext)

		if err := c.SaveUploadedFile(file, imagePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
			return
		}

		projectsMutex.Lock()
		project.EndingImage = imagePath
		project.UpdatedAt = time.Now()
		log.Printf("Ending image saved for project %s: %s", projectID, imagePath)
		projectsMutex.Unlock()

		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"image_path": imagePath,
		})
	})

	// POST /api/v2/story/projects/:projectId/owner-message - Set owner message
	router.POST("/api/v2/story/projects/:projectId/owner-message", func(c *gin.Context) {
		projectID := c.Param("projectId")

		projectsMutex.RLock()
		project, exists := projects[projectID]
		projectsMutex.RUnlock()

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		var req struct {
			Message string `json:"message" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
			return
		}

		projectsMutex.Lock()
		project.OwnerMessage = req.Message
		project.UpdatedAt = time.Now()
		log.Printf("Owner message saved for project %s: %s", projectID, req.Message)
		projectsMutex.Unlock()

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": req.Message,
		})
	})

	// POST /api/v2/story/projects/:projectId/videos - Upload videos to project
	router.POST("/api/v2/story/projects/:projectId/videos", func(c *gin.Context) {
		projectID := c.Param("projectId")

		projectsMutex.RLock()
		project, exists := projects[projectID]
		projectsMutex.RUnlock()

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
			return
		}

		files := form.File["videos"]
		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No videos uploaded"})
			return
		}

		projectDir := filepath.Join(storagePath, "projects", projectID)
		os.MkdirAll(projectDir, 0755)

		uploadedVideos := []VideoInfo{}

		for _, file := range files {
			ext := strings.ToLower(filepath.Ext(file.Filename))
			if ext != ".mp4" && ext != ".mov" && ext != ".avi" {
				continue
			}

			videoID := uuid.New().String()
			videoPath := filepath.Join(projectDir, videoID+ext)

			if err := c.SaveUploadedFile(file, videoPath); err != nil {
				log.Printf("Failed to save video %s: %v", file.Filename, err)
				continue
			}

			// Get video duration using ffprobe
			duration := getVideoDuration(videoPath)

			videoInfo := VideoInfo{
				ID:           videoID,
				OriginalName: file.Filename,
				Path:         videoPath,
				Duration:     duration,
				FramesDir:    filepath.Join(projectDir, videoID+"_frames"),
				Analyzed:     false,
			}

			uploadedVideos = append(uploadedVideos, videoInfo)
		}

		projectsMutex.Lock()
		project.Videos = append(project.Videos, uploadedVideos...)
		project.UpdatedAt = time.Now()
		projectsMutex.Unlock()

		c.JSON(http.StatusOK, gin.H{
			"uploaded": len(uploadedVideos),
			"videos":   uploadedVideos,
		})
	})

	// POST /api/v2/story/projects/:projectId/generate - Generate story
	router.POST("/api/v2/story/projects/:projectId/generate", func(c *gin.Context) {
		projectID := c.Param("projectId")

		projectsMutex.RLock()
		project, exists := projects[projectID]
		projectsMutex.RUnlock()

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		if len(project.Videos) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No videos in project"})
			return
		}

		// Start processing in background
		go processProject(projectID)

		c.JSON(http.StatusOK, gin.H{
			"project_id": projectID,
			"status":     "processing",
		})
	})

	// GET /api/v2/story/projects/:projectId - Get project status
	router.GET("/api/v2/story/projects/:projectId", func(c *gin.Context) {
		projectID := c.Param("projectId")

		projectsMutex.RLock()
		project, exists := projects[projectID]
		projectsMutex.RUnlock()

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		response := gin.H{
			"id":                 project.ID,
			"name":               project.Name,
			"dog_name":           project.DogName,
			"dog_breed":          project.DogBreed,
			"owner_relationship": project.OwnerRelationship,
			"ending_image":       project.EndingImage,
			"status":             project.Status,
			"videos":             project.Videos,
			"created_at":         project.CreatedAt,
			"updated_at":         project.UpdatedAt,
		}

		if project.Error != "" {
			response["error"] = project.Error
		}

		if project.Story != nil {
			response["story"] = project.Story
		}

		if project.FinalVideo != "" {
			response["final_video_url"] = fmt.Sprintf("/storage/projects/%s/final.mp4", project.ID)
		}

		c.JSON(http.StatusOK, response)
	})

	// GET /api/v2/story/projects - List all projects
	router.GET("/api/v2/story/projects", func(c *gin.Context) {
		projectsMutex.RLock()
		defer projectsMutex.RUnlock()

		projectList := make([]*Project, 0, len(projects))
		for _, project := range projects {
			projectList = append(projectList, project)
		}

		c.JSON(http.StatusOK, gin.H{
			"projects": projectList,
			"total":    len(projectList),
		})
	})

	// Catch-all for SPA routing
	router.NoRoute(func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.File("./frontend/dist/index.html")
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
		}
	})

	// Start server
	log.Printf("Server starting on port %s", port)
	router.Run(":" + port)
}

// ============================================================================
// Processing Pipeline
// ============================================================================

func processJob(jobID string) {
	jobsMutex.Lock()
	job := jobs[jobID]
	job.Status = "processing"
	job.UpdatedAt = time.Now()
	jobsMutex.Unlock()

	log.Printf("Processing job %s", jobID)

	// Step 1: Extract frames
	if err := extractFrames(job); err != nil {
		markJobFailed(jobID, "Failed to extract frames: "+err.Error())
		return
	}

	// Step 2: Create segments
	if err := createSegments(job); err != nil {
		markJobFailed(jobID, "Failed to create segments: "+err.Error())
		return
	}

	// Step 3: Analyze segments with AI
	if err := analyzeSegments(job); err != nil {
		markJobFailed(jobID, "Failed to analyze segments: "+err.Error())
		return
	}

	// Step 4: Find highlights
	if err := findHighlights(job); err != nil {
		markJobFailed(jobID, "Failed to find highlights: "+err.Error())
		return
	}

	// Step 5: Create highlight video
	if len(job.Highlights) > 0 {
		if err := createHighlightVideo(job); err != nil {
			markJobFailed(jobID, "Failed to create highlight video: "+err.Error())
			return
		}
	}

	jobsMutex.Lock()
	job.Status = "completed"
	job.UpdatedAt = time.Now()
	jobsMutex.Unlock()

	log.Printf("Job %s completed successfully", jobID)
}

func extractFrames(job *Job) error {
	os.MkdirAll(job.FramesDir, 0755)

	outputPattern := filepath.Join(job.FramesDir, "frame_%04d.jpg")

	// Extract 2 frames per second (0.5s intervals) and downscale to 360p
	// scale=640:360 = 360p resolution to reduce file size and processing
	cmd := exec.Command("ffmpeg", "-i", job.VideoPath, "-vf", "fps=2,scale=640:360", outputPattern)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %v, output: %s", err, string(output))
	}

	log.Printf("Extracted frames for job %s (2 fps, 360p)", job.ID)
	return nil
}

func createSegments(job *Job) error {
	// List all frame files
	files, err := filepath.Glob(filepath.Join(job.FramesDir, "frame_*.jpg"))
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no frames extracted")
	}

	// Create segments of 2-4 seconds
	// At 2 fps: 6 frames = 3 seconds (0.5s per frame)
	segmentSize := 6 // 6 frames = 3 seconds at 2 fps
	segments := []Segment{}

	for i := 0; i < len(files); i += segmentSize {
		end := i + segmentSize
		if end > len(files) {
			end = len(files)
		}

		segment := Segment{
			Index:      len(segments) + 1,
			Start:      float64(i) * 0.5, // 0.5s per frame at 2 fps
			End:        float64(end) * 0.5,
			FramePaths: files[i:end],
		}
		segments = append(segments, segment)
	}

	jobsMutex.Lock()
	job.Segments = segments
	jobsMutex.Unlock()

	log.Printf("Created %d segments for job %s", len(segments), job.ID)
	return nil
}

func analyzeSegments(job *Job) error {
	if aiAPIKey == "" || aiAPIKey == "your_api_key_here" || aiAPIKey == "your_gemini_api_key_here" {
		return fmt.Errorf("AI API key not configured. Please set AI_API_KEY in .env file")
	}

	log.Printf("Using Gemini AI analysis for job %s", job.ID)

	successCount := 0
	// 使用真實 AI 分析每個 segment
	for i := range job.Segments {
		analysis, err := analyzeSegmentWithAI(&job.Segments[i])
		if err != nil {
			// 記錄錯誤但繼續處理其他 segments
			log.Printf("Warning: AI analysis failed for segment %d: %v (skipping)", i, err)
			// 設定一個預設的分析結果
			job.Segments[i].Analysis = &Analysis{
				HasDog:          false,
				HasHuman:        false,
				InteractionType: "none",
				Emotion:         "neutral",
				ShortCaption:    "分析失敗",
			}
			continue
		}
		job.Segments[i].Analysis = analysis
		successCount++

		// 避免 API 限流，稍微延遲
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("AI analyzed %d/%d segments successfully for job %s", successCount, len(job.Segments), job.ID)

	// 只要有至少一半的 segments 分析成功就繼續
	if successCount < len(job.Segments)/2 {
		return fmt.Errorf("too many segments failed analysis (%d/%d succeeded)", successCount, len(job.Segments))
	}

	return nil
}

// analyzeVideoWithAI - 整個影片只打一次 API，傳送最多 10 張代表性圖片
// 有ＡＩ
func analyzeVideoWithAI(framePaths []string, videoID string) (*Analysis, error) {
	if len(framePaths) == 0 {
		return nil, fmt.Errorf("no frames provided")
	}

	// 智能選擇最多 10 張代表性圖片（均勻分佈）
	maxImages := 10
	selectedFrames := []string{}

	if len(framePaths) <= maxImages {
		// 圖片數量不多，全部使用
		selectedFrames = framePaths
	} else {
		// 均勻選擇 10 張圖片
		step := float64(len(framePaths)) / float64(maxImages)
		for i := 0; i < maxImages; i++ {
			idx := int(float64(i) * step)
			if idx < len(framePaths) {
				selectedFrames = append(selectedFrames, framePaths[idx])
			}
		}
	}

	log.Printf("Video %s: Analyzing with %d images (total frames: %d)", videoID, len(selectedFrames), len(framePaths))

	// 壓縮並編碼所有選中的圖片
	base64Images := []string{}
	for _, framePath := range selectedFrames {
		compressedData, err := compressImage(framePath, 320, 240) // 壓縮到 320x240
		if err != nil {
			log.Printf("Warning: failed to compress image %s: %v", framePath, err)
			continue
		}

		base64Image := base64.StdEncoding.EncodeToString(compressedData)
		base64Images = append(base64Images, base64Image)
	}

	if len(base64Images) == 0 {
		return nil, fmt.Errorf("no frames could be processed")
	}

	log.Printf("Successfully compressed %d images for video %s", len(base64Images), videoID)

	// 構建 API 請求
	parts := []map[string]interface{}{
		{
			"text": fmt.Sprintf(`這些是來自同一個影片的 %d 張連續截圖（每隔 2 秒一張）。請綜合分析整個影片，判斷以下內容並以 JSON 格式回應：

{
  "has_dog": true/false,
  "has_human": true/false,
  "interaction_type": "running_towards_owner" | "playing" | "being_petted" | "fetching" | "cuddling" | "none",
  "emotion": "happy" | "excited" | "calm" | "neutral" | "sad",
  "short_caption": "用中文簡短描述這個影片的主要內容（15字以內）"
}

判斷標準：
- has_dog: 影片中是否有狗
- has_human: 影片中是否有人
- interaction_type: 狗和人之間的主要互動類型
- emotion: 狗的整體情緒
- short_caption: 簡短描述影片內容

**重要**：這些圖片來自同一個完整影片，請綜合所有圖片進行分析。

只回傳 JSON，不要其他文字。`, len(base64Images)),
		},
	}

	// 添加所有圖片
	for _, imgData := range base64Images {
		parts = append(parts, map[string]interface{}{
			"inline_data": map[string]string{
				"mime_type": "image/jpeg",
				"data":      imgData,
			},
		})
	}

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": parts,
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":      0.4,
			"maxOutputTokens":  2000, // 增加到 2000，避免 MAX_TOKENS 錯誤
			"responseMimeType": "application/json",
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// 發送請求
	url := fmt.Sprintf("%s?key=%s", aiAPIEndpoint, aiAPIKey)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second} // 增加超時時間
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// 讀取回應
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// 解析回應
	var apiResponse struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	// 檢查錯誤
	if apiResponse.Error != nil {
		return nil, fmt.Errorf("Gemini API error: %d - %s", apiResponse.Error.Code, apiResponse.Error.Message)
	}

	if len(apiResponse.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	candidate := apiResponse.Candidates[0]

	// 檢查內容
	if candidate.Content.Parts == nil || len(candidate.Content.Parts) == 0 {
		log.Printf("Gemini returned empty content for video %s. FinishReason: %s, Response: %s",
			videoID, candidate.FinishReason, string(bodyBytes))
		return nil, fmt.Errorf("no content (finishReason: %s)", candidate.FinishReason)
	}

	// 解析 JSON
	content := candidate.Content.Parts[0].Text
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var analysis Analysis
	if err := json.Unmarshal([]byte(content), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %v, content: %s", err, content)
	}

	log.Printf("✅ Video %s analyzed: has_dog=%v, has_human=%v, interaction=%s, emotion=%s, caption=%s",
		videoID, analysis.HasDog, analysis.HasHuman, analysis.InteractionType, analysis.Emotion, analysis.ShortCaption)

	return &analysis, nil
}

// analyzeSegmentWithAI - 保留此函數供 Phase 1 使用
func analyzeSegmentWithAI(segment *Segment) (*Analysis, error) {
	if len(segment.FramePaths) == 0 {
		return nil, fmt.Errorf("no frames in segment")
	}

	// 使用新的函數分析
	return analyzeVideoWithAI(segment.FramePaths, fmt.Sprintf("segment_%d", segment.Index))
}

func findHighlights(job *Job) error {
	highlights := []Highlight{}

	var currentHighlight *Highlight

	for _, segment := range job.Segments {
		if segment.Analysis == nil {
			continue
		}

		// Check if this segment qualifies as highlight
		if segment.Analysis.HasDog && segment.Analysis.HasHuman &&
			segment.Analysis.InteractionType != "none" {

			if currentHighlight == nil {
				// Start new highlight
				currentHighlight = &Highlight{
					Start:       segment.Start,
					End:         segment.End,
					Caption:     segment.Analysis.ShortCaption,
					Interaction: segment.Analysis.InteractionType,
					Emotion:     segment.Analysis.Emotion,
				}
			} else {
				// Extend current highlight
				currentHighlight.End = segment.End
				if currentHighlight.Caption != "" {
					currentHighlight.Caption += " → " + segment.Analysis.ShortCaption
				}
			}
		} else {
			// No interaction, save current highlight if exists
			if currentHighlight != nil {
				highlights = append(highlights, *currentHighlight)
				currentHighlight = nil
			}
		}
	}

	// Save last highlight if exists
	if currentHighlight != nil {
		highlights = append(highlights, *currentHighlight)
	}

	jobsMutex.Lock()
	job.Highlights = highlights
	jobsMutex.Unlock()

	log.Printf("Found %d highlights for job %s", len(highlights), job.ID)
	return nil
}

func createHighlightVideo(job *Job) error {
	if len(job.Highlights) == 0 {
		return nil
	}

	outputDir := filepath.Join(storagePath, "videos", job.ID)
	outputPath := filepath.Join(outputDir, "highlight.mp4")

	// For simplicity, create video from first highlight
	// In production, you'd concatenate all highlights
	highlight := job.Highlights[0]

	cmd := exec.Command("ffmpeg",
		"-i", job.VideoPath,
		"-ss", fmt.Sprintf("%.2f", highlight.Start),
		"-to", fmt.Sprintf("%.2f", highlight.End),
		"-c", "copy",
		"-y",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %v, output: %s", err, string(output))
	}

	jobsMutex.Lock()
	job.HighlightVideo = outputPath
	jobsMutex.Unlock()

	log.Printf("Created highlight video for job %s", job.ID)
	return nil
}

func markJobFailed(jobID, errorMsg string) {
	log.Printf("Job %s failed: %s", jobID, errorMsg)

	jobsMutex.Lock()
	if job, exists := jobs[jobID]; exists {
		job.Status = "failed"
		job.Error = errorMsg
		job.UpdatedAt = time.Now()
	}
	jobsMutex.Unlock()
}

// ============================================================================
// Phase 2 Processing Pipeline
// ============================================================================

func processProject(projectID string) {
	projectsMutex.Lock()
	project := projects[projectID]
	project.Status = "analyzing"
	project.UpdatedAt = time.Now()
	projectsMutex.Unlock()

	log.Printf("Processing project %s with %d videos", projectID, len(project.Videos))

	// Step 1: Analyze all videos (繼續處理即使有錯誤)
	successCount := 0
	for i := range project.Videos {
		if err := analyzeVideo(project, i); err != nil {
			log.Printf("⚠️ Warning: Failed to analyze video %s: %v (continuing)", project.Videos[i].ID, err)
			// 不要立即返回，繼續處理其他影片
			continue
		}
		successCount++
	}

	// 至少要有一半的影片分析成功才能繼續
	if successCount == 0 {
		markProjectFailed(projectID, "All videos failed to analyze")
		return
	}

	log.Printf("✅ Successfully analyzed %d/%d videos", successCount, len(project.Videos))

	// Step 2: Generate story with AI
	projectsMutex.Lock()
	project.Status = "generating_story"
	project.UpdatedAt = time.Now()
	projectsMutex.Unlock()

	story, err := generateStoryWithAI(project)
	if err != nil {
		markProjectFailed(projectID, "Failed to generate story: "+err.Error())
		return
	}

	projectsMutex.Lock()
	project.Story = story
	project.Status = "generating_video"
	project.UpdatedAt = time.Now()
	projectsMutex.Unlock()

	// Step 3: Generate TTS audio for each chapter
	for i := range project.Story.Chapters {
		if err := generateTTS(project, i); err != nil {
			log.Printf("Warning: TTS generation failed for chapter %d: %v", i, err)
			// Continue without audio
		}
	}

	// Step 4: Composite final video (with subtitles and background music)
	if err := compositeVideo(project); err != nil {
		markProjectFailed(projectID, "Failed to composite video: "+err.Error())
		return
	}

	projectsMutex.Lock()
	project.Status = "completed"
	project.UpdatedAt = time.Now()
	projectsMutex.Unlock()

	log.Printf("Project %s completed successfully", projectID)
}

func analyzeVideo(project *Project, videoIndex int) error {
	video := &project.Videos[videoIndex]

	log.Printf("Analyzing video %s (%s)", video.ID, video.OriginalName)

	// Extract frames - 每2秒一張 (fps=0.5)
	os.MkdirAll(video.FramesDir, 0755)
	outputPattern := filepath.Join(video.FramesDir, "frame_%04d.jpg")
	cmd := exec.Command("ffmpeg", "-i", video.Path, "-vf", "fps=0.5,scale=640:360", outputPattern)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg error: %v, output: %s", err, string(output))
	}

	// Get all frame files
	files, err := filepath.Glob(filepath.Join(video.FramesDir, "frame_*.jpg"))
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no frames extracted")
	}

	log.Printf("Extracted %d frames from video %s", len(files), video.ID)

	// **新邏輯：整個影片只打一次 API，一次傳送所有圖片（最多10張）**
	analysis, err := analyzeVideoWithAI(files, video.ID)
	if err != nil {
		log.Printf("Warning: AI analysis failed for video %s: %v (using default analysis)", video.ID, err)
		// 使用預設分析，讓流程繼續
		analysis = &Analysis{
			HasDog:          true,
			HasHuman:        true,
			InteractionType: "none",
			Emotion:         "neutral",
			ShortCaption:    "影片分析",
		}
	}

	// 根據 AI 分析結果創建 segments（每個 segment = 6 秒）
	segmentSize := 3 // 3 frames = 6 seconds at fps=0.5 (2s per frame)
	segments := []Segment{}

	for i := 0; i < len(files); i += segmentSize {
		end := i + segmentSize
		if end > len(files) {
			end = len(files)
		}

		segment := Segment{
			Index:      len(segments) + 1,
			Start:      float64(i) * 2.0, // 2.0s per frame at fps=0.5
			End:        float64(end) * 2.0,
			FramePaths: files[i:end],
			Analysis:   analysis, // 所有 segment 使用相同的分析結果
		}
		segments = append(segments, segment)
	}

	// Find highlights based on analysis
	highlights := []Highlight{}

	// 如果有互動，將整個影片（或前幾個 segment）標記為 highlight
	if analysis.HasDog && analysis.HasHuman && analysis.InteractionType != "none" {
		// 取前 15 秒作為 highlight
		maxHighlightDuration := 15.0
		for _, segment := range segments {
			if segment.End <= maxHighlightDuration {
				if len(highlights) == 0 {
					highlights = append(highlights, Highlight{
						Start:       segment.Start,
						End:         segment.End,
						Caption:     analysis.ShortCaption,
						Interaction: analysis.InteractionType,
						Emotion:     analysis.Emotion,
					})
				} else {
					highlights[0].End = segment.End
				}
			}
		}
	}

	// Update video info
	projectsMutex.Lock()
	project.Videos[videoIndex].Segments = segments
	project.Videos[videoIndex].Highlights = highlights
	project.Videos[videoIndex].Analyzed = true
	projectsMutex.Unlock()

	log.Printf("Analyzed video %s: %d segments, %d highlights", video.ID, len(segments), len(highlights))
	return nil
}

// 有ＡＩ
func generateStoryWithAI(project *Project) (*Story, error) {
	log.Printf("Generating story for project %s with AI", project.ID)

	// 收集所有高光片段的描述
	allHighlights := []string{}
	for _, video := range project.Videos {
		for _, highlight := range video.Highlights {
			allHighlights = append(allHighlights, fmt.Sprintf("影片《%s》: %s (情緒：%s)",
				video.OriginalName, highlight.Caption, highlight.Emotion))
		}
	}

	if len(allHighlights) == 0 {
		return nil, fmt.Errorf("no highlights found in any video")
	}

	// 根據關係設定稱呼
	ownerTitle := project.OwnerRelationship
	if ownerTitle == "" {
		ownerTitle = "主人"
	}

	// 構建 prompt - 生成 5 段狗狗對白
	prompt := fmt.Sprintf(`你是一隻名叫「%s」的%s。請想像你是一個 3 歲的小孩，用天真、單純、開心的語氣，看著這些影片對你的「%s」說話。

影片片段：
%s

請創作 5 段對白，每段約 20-30 字：
- **角色設定**：你是 3 歲的小朋友，說話不用太複雜，要可愛、直接。
- **情感基調**：充滿愛、開心、感謝。覺得%s是世界上最棒的人。
- **稱呼**：請在對白中適當使用「%s」這個稱呼。
- **必須創作正好 5 段對白**，對應 5 個影片。
- 第 5 段要特別感人，表達會永遠愛%s。

範例風格：
「%s你看！我跑得好快喔！因為我想快點撲到你懷裡～」
「最喜歡跟%s在一起了，只要有你在，我就好安心好開心！」

以 JSON 格式回應（必須是正好 5 個 chapters）：
{
  "title": "給%s的悄悄話",
  "chapters": [
    {"narration": "第一段對白", "video_index": 0, "highlight_index": 0},
    {"narration": "第二段對白", "video_index": 1, "highlight_index": 0},
    {"narration": "第三段對白", "video_index": 2, "highlight_index": 0},
    {"narration": "第四段對白", "video_index": 3, "highlight_index": 0},
    {"narration": "第五段對白", "video_index": 4, "highlight_index": 0}
  ]
}

只回傳 JSON，不要其他文字。`,
		project.DogName,
		project.DogBreed,
		ownerTitle,
		strings.Join(allHighlights, "\n"),
		ownerTitle,
		ownerTitle,
		ownerTitle,
		ownerTitle,
		ownerTitle,
		ownerTitle)

	// 調用 Gemini AI
	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":      0.8, // 稍微提高溫度，讓語氣更活潑
			"maxOutputTokens":  8000,
			"responseMimeType": "application/json",
		},
	}

	jsonData, _ := json.Marshal(requestBody)
	url := fmt.Sprintf("%s?key=%s", aiAPIEndpoint, aiAPIKey)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	var apiResponse struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return nil, err
	}

	if len(apiResponse.Candidates) == 0 {
		log.Printf("Story generation failed: no candidates. Response: %s", string(bodyBytes))
		return nil, fmt.Errorf("no content in AI response")
	}

	candidate := apiResponse.Candidates[0]

	if len(candidate.Content.Parts) == 0 {
		log.Printf("Story generation failed: no parts. FinishReason: %v, Response: %s",
			candidate, string(bodyBytes))
		return nil, fmt.Errorf("no content in AI response (finishReason may be MAX_TOKENS or SAFETY)")
	}

	content := apiResponse.Candidates[0].Content.Parts[0].Text
	log.Printf("Story AI response content: %s", content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var storyResponse struct {
		Title    string `json:"title"`
		Chapters []struct {
			Narration      string `json:"narration"`
			VideoIndex     int    `json:"video_index"`
			HighlightIndex int    `json:"highlight_index"`
		} `json:"chapters"`
	}

	if err := json.Unmarshal([]byte(content), &storyResponse); err != nil {
		return nil, fmt.Errorf("failed to parse story: %v", err)
	}

	// 轉換為 Story 結構
	story := &Story{
		Title:    storyResponse.Title,
		Chapters: []StoryChapter{},
	}

	for i, ch := range storyResponse.Chapters {
		if ch.VideoIndex >= len(project.Videos) {
			log.Printf("Warning: chapter %d video_index %d >= videos length %d, skipping",
				i, ch.VideoIndex, len(project.Videos))
			continue
		}
		video := project.Videos[ch.VideoIndex]

		// 如果沒有 highlights 或 highlight_index 超出範圍，使用整個影片
		var startTime, endTime float64
		if len(video.Highlights) > 0 && ch.HighlightIndex < len(video.Highlights) {
			highlight := video.Highlights[ch.HighlightIndex]
			startTime = highlight.Start
			endTime = highlight.End
		} else {
			// 沒有 highlights，使用影片前 15 秒
			startTime = 0
			endTime = 15.0
			if video.Duration > 0 && video.Duration < 15.0 {
				endTime = video.Duration
			}
			log.Printf("Using full video duration for chapter %d: 0 to %.2f", i+1, endTime)
		}

		chapter := StoryChapter{
			Index:     i + 1,
			Narration: ch.Narration,
			VideoID:   video.ID,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  endTime - startTime,
		}
		story.Chapters = append(story.Chapters, chapter)
	}

	log.Printf("Generated story with %d chapters", len(story.Chapters))

	// 如果主人有留言，生成狗狗的回應
	if project.OwnerMessage != "" {
		dogResponse, err := generateDogResponse(project, story)
		if err != nil {
			log.Printf("Warning: Failed to generate dog response: %v", err)
			story.DogResponse = fmt.Sprintf("%s，我愛你！", ownerTitle) // 預設回應
		} else {
			story.DogResponse = dogResponse
		}
		story.OwnerMessage = project.OwnerMessage
	}

	return story, nil
}

// 有ＡＩ
func generateDogResponse(project *Project, story *Story) (string, error) {
	log.Printf("Generating dog response for project %s", project.ID)

	// 收集影片描述
	videoDescriptions := []string{}
	for i, chapter := range story.Chapters {
		videoDescriptions = append(videoDescriptions, fmt.Sprintf("影片 %d: %s", i+1, chapter.Narration))
	}

	// 根據關係設定稱呼
	ownerTitle := project.OwnerRelationship
	if ownerTitle == "" {
		ownerTitle = "主人"
	}

	prompt := fmt.Sprintf(`你是一隻名叫「%s」的%s。你的「%s」剛剛對你說了一段很感人的話。
請你用 **3 歲小孩** 的語氣和心智，回應你的%s。

%s對你說：
「%s」

你們的回憶：
%s

請以狗狗的第一人稱（我）回應，要求：
1. **語氣像 3 歲小孩**：天真、單純、直接、可愛。不要用太成熟或文謅謅的詞。
2. **字數**：30-50 字左右，不要太長。
3. **內容**：表達感謝和開心，告訴%s你也很愛他/她，會永遠陪著他/她。
4. **稱呼**：回應中要叫「%s」。
5. **不要**用「汪汪」或「嗚嗚」等擬聲詞，用人類的語言（小朋友的口吻）表達。

範例風格：
「%s，我聽到了！我也最愛你了！雖然我只會跑跑跳跳，但我會一直黏著你，做你最乖的寶貝！」
「%s不要哭，我會永遠保護你的！我們打勾勾，要一直在一起喔！」

請根據%s的話，創作一段溫暖、可愛、像小朋友一樣的回應。只回傳回應文字，不要其他內容。`,
		project.DogName,
		project.DogBreed,
		ownerTitle,
		ownerTitle,
		ownerTitle,
		project.OwnerMessage,
		strings.Join(videoDescriptions, "\n"),
		ownerTitle,
		ownerTitle,
		ownerTitle,
		ownerTitle)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.8, // 提高溫度，增加情感豐富度
			"maxOutputTokens": 500,
		},
	}

	jsonData, _ := json.Marshal(requestBody)
	url := fmt.Sprintf("%s?key=%s", aiAPIEndpoint, aiAPIKey)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	var apiResponse struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return "", err
	}

	if len(apiResponse.Candidates) == 0 || len(apiResponse.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from AI")
	}

	response := apiResponse.Candidates[0].Content.Parts[0].Text
	response = strings.TrimSpace(response)
	response = strings.Trim(response, "「」\"")

	log.Printf("Generated dog response: %s", response)
	return response, nil
}

func generateTTS(project *Project, chapterIndex int) error {
	chapter := &project.Story.Chapters[chapterIndex]

	log.Printf("Generating TTS for chapter %d: %s", chapterIndex+1, chapter.Narration)

	// 使用 Google Cloud Text-to-Speech API
	// API endpoint: https://texttospeech.googleapis.com/v1/text:synthesize

	requestBody := map[string]interface{}{
		"input": map[string]string{
			"text": chapter.Narration,
		},
		"voice": map[string]interface{}{
			"languageCode": "zh-TW",
			"name":         "cmn-TW-Wavenet-A", // 台灣中文女聲
			"ssmlGender":   "FEMALE",
		},
		"audioConfig": map[string]interface{}{
			"audioEncoding": "MP3",
			"speakingRate":  0.95, // 稍微慢一點，更溫暖
			"pitch":         0.0,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal TTS request: %v", err)
	}

	// 使用與 Gemini 相同的 API Key
	url := fmt.Sprintf("https://texttospeech.googleapis.com/v1/text:synthesize?key=%s", aiAPIKey)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create TTS request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send TTS request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TTS API error %d: %s", resp.StatusCode, string(body))
	}

	// 解析回應
	var ttsResponse struct {
		AudioContent string `json:"audioContent"` // Base64 encoded MP3
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read TTS response: %v", err)
	}

	if err := json.Unmarshal(bodyBytes, &ttsResponse); err != nil {
		return fmt.Errorf("failed to parse TTS response: %v", err)
	}

	if ttsResponse.AudioContent == "" {
		return fmt.Errorf("no audio content in TTS response")
	}

	// 解碼 Base64 音訊
	audioData, err := base64.StdEncoding.DecodeString(ttsResponse.AudioContent)
	if err != nil {
		return fmt.Errorf("failed to decode audio: %v", err)
	}

	// 儲存音訊檔案
	outputDir := filepath.Join(storagePath, "projects", project.ID, "audio")
	os.MkdirAll(outputDir, 0755)

	audioPath := filepath.Join(outputDir, fmt.Sprintf("chapter_%d.mp3", chapterIndex+1))
	if err := os.WriteFile(audioPath, audioData, 0644); err != nil {
		return fmt.Errorf("failed to save audio: %v", err)
	}

	// 取得音訊時長
	duration := getAudioDuration(audioPath)

	// 更新章節資訊
	projectsMutex.Lock()
	project.Story.Chapters[chapterIndex].AudioPath = audioPath
	project.Story.Chapters[chapterIndex].Duration = duration
	projectsMutex.Unlock()

	log.Printf("Generated TTS audio for chapter %d (duration: %.2fs)", chapterIndex+1, duration)
	return nil
}

func getAudioDuration(audioPath string) float64 {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		audioPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	duration := 0.0
	fmt.Sscanf(string(output), "%f", &duration)
	return duration
}

func compositeVideo(project *Project) error {
	log.Printf("Compositing final video for project %s (with transitions, subtitles and music)", project.ID)

	if len(project.Story.Chapters) == 0 {
		return fmt.Errorf("no chapters to composite")
	}

	outputDir := filepath.Join(storagePath, "projects", project.ID)

	// Step 1: 生成帶轉場效果的影片片段（移除原始音訊，只保留 TTS）
	log.Printf("Step 1: Creating video segments with transitions and TTS audio")
	videoWithTTSPath := filepath.Join(outputDir, "video_with_tts.mp4")
	if err := createVideoWithTransitionsAndTTS(project, videoWithTTSPath); err != nil {
		return fmt.Errorf("failed to create video with transitions: %v", err)
	}

	// Step 2: 如果有結尾圖片和狗狗回應，添加結尾片段
	videoWithEndingPath := videoWithTTSPath
	log.Printf("📸 EndingImage check: EndingImage='%s', DogResponse='%s', OwnerMessage='%s'",
		project.EndingImage, project.Story.DogResponse, project.OwnerMessage)

	if project.EndingImage != "" {
		// 如果有 OwnerMessage 但 DogResponse 還是預設的簡短回應，重新生成
		if project.OwnerMessage != "" && (project.Story.DogResponse == "" || project.Story.DogResponse == "主人，我愛你！") {
			log.Printf("🤖 Regenerating dog response based on owner message")
			dogResponse, err := generateDogResponse(project, project.Story)
			if err != nil {
				log.Printf("⚠️ Failed to generate dog response: %v, using default", err)
				ownerTitle := project.OwnerRelationship
				if ownerTitle == "" {
					ownerTitle = "主人"
				}
				project.Story.DogResponse = fmt.Sprintf("%s，我也永遠愛你！每天和你在一起，是我最幸福的時光。", ownerTitle)
			} else {
				project.Story.DogResponse = dogResponse
				log.Printf("✅ Generated dog response: %s", dogResponse)
			}
			project.Story.OwnerMessage = project.OwnerMessage
		} else if project.Story.DogResponse == "" {
			log.Printf("⚠️ No DogResponse, using default response for ending")
			ownerTitle := project.OwnerRelationship
			if ownerTitle == "" {
				ownerTitle = "主人"
			}
			project.Story.DogResponse = fmt.Sprintf("%s，我愛你！每天和你在一起，是我最幸福的時光～", ownerTitle)
		}

		log.Printf("Step 2: Adding ending image with dog response")
		videoWithEndingPath = filepath.Join(outputDir, "video_with_ending.mp4")
		if err := addEndingImage(project, videoWithTTSPath, videoWithEndingPath); err != nil {
			log.Printf("❌ Failed to add ending image: %v, continuing without it", err)
			videoWithEndingPath = videoWithTTSPath
		} else {
			log.Printf("✅ Ending image added successfully")
		}
	} else {
		log.Printf("Step 2: Skipping ending image (EndingImage path is empty)")
	}

	// Step 3: 加入字幕
	log.Printf("Step 3: Adding subtitles")
	subtitledVideoPath := filepath.Join(outputDir, "subtitled_video.mp4")
	if err := addSubtitles(project, videoWithEndingPath, subtitledVideoPath); err != nil {
		log.Printf("Warning: Failed to add subtitles: %v, continuing without subtitles", err)
		subtitledVideoPath = videoWithEndingPath
	}

	// Step 4: 加入背景音樂（100% 音量）
	log.Printf("Step 4: Adding background music")
	finalVideoPath := filepath.Join(outputDir, "final.mp4")
	if err := addBackgroundMusic(project, subtitledVideoPath, finalVideoPath); err != nil {
		log.Printf("Warning: Failed to add background music: %v, using version without music", err)
		os.Rename(subtitledVideoPath, finalVideoPath)
	} else {
		os.Remove(subtitledVideoPath)
	}

	// 清理中間檔案
	os.Remove(videoWithTTSPath)
	if videoWithEndingPath != videoWithTTSPath {
		os.Remove(videoWithEndingPath)
	}

	projectsMutex.Lock()
	project.FinalVideo = finalVideoPath
	projectsMutex.Unlock()

	log.Printf("✅ Created final video with all effects for project %s", project.ID)
	return nil
}

// createVideoWithTransitionsAndTTS - 創建帶轉場效果和 TTS 的影片（移除原始音訊）
func createVideoWithTransitionsAndTTS(project *Project, outputPath string) error {
	outputDir := filepath.Dir(outputPath)

	log.Printf("Creating video segments with fade transitions and TTS audio")

	// 處理每個章節
	processedSegments := []string{}
	// filterComplex := []string{} // Unused
	audioInputs := []string{}

	for i, chapter := range project.Story.Chapters {
		// 找到對應的影片
		var videoPath string
		for _, video := range project.Videos {
			if video.ID == chapter.VideoID {
				videoPath = video.Path
				break
			}
		}

		if videoPath == "" {
			log.Printf("Warning: video not found for chapter %d", i+1)
			continue
		}

		// 剪切影片片段（移除音訊）
		segmentPath := filepath.Join(outputDir, fmt.Sprintf("segment_%d.mp4", chapter.Index))

		// 計算淡入淡出
		fadeDuration := 0.5
		videoDuration := chapter.EndTime - chapter.StartTime

		// 使用 ffmpeg 剪切並添加淡入淡出效果（移除音訊）
		fadeFilter := fmt.Sprintf("fade=t=in:st=0:d=%.2f,fade=t=out:st=%.2f:d=%.2f",
			fadeDuration, videoDuration-fadeDuration, fadeDuration)

		cmd := exec.Command("ffmpeg",
			"-i", videoPath,
			"-ss", fmt.Sprintf("%.2f", chapter.StartTime),
			"-to", fmt.Sprintf("%.2f", chapter.EndTime),
			"-vf", fadeFilter,
			"-an", // 移除音訊
			"-c:v", "libx264",
			"-y",
			segmentPath,
		)

		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Failed to create segment %d: %v, output: %s", chapter.Index, err, string(output))
			continue
		}

		processedSegments = append(processedSegments, segmentPath)

		// 如果有 TTS 音訊，記錄下來
		if chapter.AudioPath != "" {
			audioInputs = append(audioInputs, chapter.AudioPath)
		}
	}

	if len(processedSegments) == 0 {
		return fmt.Errorf("no segments created")
	}

	// 合併所有影片片段
	concatListPath := filepath.Join(outputDir, "concat_segments.txt")
	f, err := os.Create(concatListPath)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, seg := range processedSegments {
		fmt.Fprintf(f, "file '%s'\n", filepath.Base(seg))
	}
	f.Close()

	// 拼接影片
	videoOnlyPath := filepath.Join(outputDir, "video_only.mp4")
	cmd := exec.Command("ffmpeg",
		"-f", "concat",
		"-safe", "0",
		"-i", concatListPath,
		"-c", "copy",
		"-y",
		videoOnlyPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg concat error: %v, output: %s", err, string(output))
	}

	// 合併所有 TTS 音訊
	if len(audioInputs) > 0 {
		log.Printf("Merging %d TTS audio files", len(audioInputs))

		audioListPath := filepath.Join(outputDir, "concat_audio.txt")
		af, err := os.Create(audioListPath)
		if err != nil {
			return err
		}

		for _, audioPath := range audioInputs {
			fmt.Fprintf(af, "file '%s'\n", audioPath)
		}
		af.Close()

		mergedAudioPath := filepath.Join(outputDir, "merged_audio.mp3")
		cmd = exec.Command("ffmpeg",
			"-f", "concat",
			"-safe", "0",
			"-i", audioListPath,
			"-c", "copy",
			"-y",
			mergedAudioPath,
		)

		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Warning: Failed to merge audio: %v, output: %s", err, string(output))
			// 沒有音訊，直接使用影片
			os.Rename(videoOnlyPath, outputPath)
		} else {
			// 合併影片和音訊
			cmd = exec.Command("ffmpeg",
				"-i", videoOnlyPath,
				"-i", mergedAudioPath,
				"-c:v", "copy",
				"-c:a", "aac",
				"-shortest",
				"-y",
				outputPath,
			)

			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to merge video and audio: %v, output: %s", err, string(output))
			}

			os.Remove(mergedAudioPath)
		}

		os.Remove(audioListPath)
	} else {
		// 沒有 TTS，直接使用影片
		os.Rename(videoOnlyPath, outputPath)
	}

	// 清理
	os.Remove(concatListPath)
	for _, seg := range processedSegments {
		os.Remove(seg)
	}

	log.Printf("✅ Created video with transitions and TTS audio")
	return nil
}

// addEndingImage - 添加結尾圖片並顯示狗狗的回應
// 使用 concat 協議合併影片，確保結尾圖片正確顯示
func addEndingImage(project *Project, inputVideo, outputVideo string) error {
	log.Printf("Adding ending image with dog response (concat approach)")

	outputDir := filepath.Dir(inputVideo)
	endingDuration := 10.0 // 結尾 10 秒

	// 準備狗狗回應文字
	dogText := fmt.Sprintf("🐾 %s：%s", project.DogName, project.Story.DogResponse)

	// 獲取輸入影片時長和解析度
	inputDuration := getVideoDuration(inputVideo)
	width, height := getVideoResolution(inputVideo)
	if inputDuration == 0 || width == 0 || height == 0 {
		log.Printf("Warning: Could not get input video info (duration: %.2f, size: %dx%d), copying input as-is", inputDuration, width, height)
		return exec.Command("cp", inputVideo, outputVideo).Run()
	}
	log.Printf("Input video info: duration=%.2fs, size=%dx%d", inputDuration, width, height)

	// 創建結尾圖片影片（10秒）
	endingVideoPath := filepath.Join(outputDir, "ending_segment.mp4")

	// 選擇字體 (macOS 使用 STHeiti 或 PingFang，其他使用默認或 Arial)
	// STHeiti (华文黑体) 通常比 PingFang 更容易被 FFmpeg 識別
	fontFile := "/System/Library/Fonts/STHeiti Medium.ttc"
	if _, err := os.Stat(fontFile); err != nil {
		fontFile = "/System/Library/Fonts/PingFang.ttc"
		if _, err := os.Stat(fontFile); err != nil {
			fontFile = "Arial" // Fallback
		}
	}
	log.Printf("Using font: %s", fontFile)

	// 計算字體大小 (根據高度調整)
	fontSize := height / 25
	if fontSize < 24 {
		fontSize = 24
	}

	// 使用 FFmpeg 創建結尾圖片影片
	// 1. 循環圖片 10 秒
	// 2. 添加靜音音軌 (anullsrc)
	// 3. 縮放並添加文字
	// 注意：使用 input 的寬高，並確保顏色空間與主影片一致
	endingCmd := exec.Command("ffmpeg",
		"-loop", "1",
		"-i", project.EndingImage,
		"-f", "lavfi",
		"-i", "anullsrc=r=44100:cl=stereo",
		"-vf", fmt.Sprintf(
			"scale=%d:%d:force_original_aspect_ratio=decrease,"+
				"pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,"+
				"drawtext=fontfile='%s':text='%s':fontsize=%d:fontcolor=white:"+
				"x=(w-text_w)/2:y=h-%d:"+
				"box=1:boxcolor=black@0.6:boxborderw=10,"+
				"fade=t=in:st=0:d=0.5,fade=t=out:st=%.1f:d=0.5,"+
				"format=yuv420p,colorspace=bt709:iall=bt601-6-625:fast=1",
			width, height,
			width, height,
			fontFile,
			escapeFFmpegText(dogText),
			fontSize,
			height/5, // y position relative to height
			endingDuration-0.5,
		),
		"-t", fmt.Sprintf("%.2f", endingDuration),
		"-c:v", "libx264",
		"-c:a", "aac",
		"-pix_fmt", "yuv420p",
		"-color_range", "tv",
		"-colorspace", "bt709",
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-shortest",
		"-y",
		endingVideoPath,
	)

	endingOutput, err := endingCmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to create ending segment: %v, output: %s", err, string(endingOutput))
		return fmt.Errorf("failed to create ending: %v", err)
	}

	// 驗證結尾影片
	if stat, err := os.Stat(endingVideoPath); err != nil || stat.Size() == 0 {
		return fmt.Errorf("ending segment not created properly")
	}

	log.Printf("Created ending segment: %s", endingVideoPath)

	// 使用 concat filter 合併影片
	// [0:v][0:a][1:v][1:a]concat=n=2:v=1:a=1[outv][outa]
	concatCmd := exec.Command("ffmpeg",
		"-i", inputVideo,
		"-i", endingVideoPath,
		"-filter_complex", "[0:v][0:a][1:v][1:a]concat=n=2:v=1:a=1[outv][outa]",
		"-map", "[outv]",
		"-map", "[outa]",
		"-c:v", "libx264",
		"-c:a", "aac",
		"-preset", "fast",
		"-y",
		outputVideo,
	)

	concatOutput, err := concatCmd.CombinedOutput()
	if err != nil {
		log.Printf("Concat failed: %v, output: %s", err, string(concatOutput))

		// 嘗試不帶音訊的 concat (如果輸入影片沒有音訊)
		log.Printf("Trying concat without audio...")
		concatCmdNoAudio := exec.Command("ffmpeg",
			"-i", inputVideo,
			"-i", endingVideoPath,
			"-filter_complex", "[0:v][1:v]concat=n=2:v=1:a=0[outv]",
			"-map", "[outv]",
			"-c:v", "libx264",
			"-preset", "fast",
			"-y",
			outputVideo,
		)
		if out, err := concatCmdNoAudio.CombinedOutput(); err != nil {
			log.Printf("Concat no-audio failed: %v, output: %s", err, string(out))
			return fmt.Errorf("failed to concat: %v", err)
		}
	}

	// 驗證輸出
	finalDuration := getVideoDuration(outputVideo)
	log.Printf("Created video with ending: duration=%.2fs (expected: %.2fs)", finalDuration, inputDuration+endingDuration)

	// 清理
	os.Remove(endingVideoPath)

	log.Printf("✅ Added ending image with duration %.2fs", endingDuration)
	return nil
}

func compositeVideoOnly(project *Project, outputPath string) error {
	outputDir := filepath.Dir(outputPath)

	// 建立影片片段列表檔案
	listFile := filepath.Join(outputDir, "concat_list.txt")
	f, err := os.Create(listFile)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, chapter := range project.Story.Chapters {
		// 找到對應的影片
		var videoPath string
		for _, video := range project.Videos {
			if video.ID == chapter.VideoID {
				videoPath = video.Path
				break
			}
		}

		if videoPath == "" {
			continue
		}

		// 剪出這個片段
		segmentPath := filepath.Join(outputDir, fmt.Sprintf("segment_%d.mp4", chapter.Index))
		cmd := exec.Command("ffmpeg",
			"-i", videoPath,
			"-ss", fmt.Sprintf("%.2f", chapter.StartTime),
			"-to", fmt.Sprintf("%.2f", chapter.EndTime),
			"-c:v", "libx264",
			"-c:a", "aac",
			"-y",
			segmentPath,
		)

		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Failed to create segment %d: %v, output: %s", chapter.Index, err, string(output))
			continue
		}

		fmt.Fprintf(f, "file '%s'\n", filepath.Base(segmentPath))
	}

	// 拼接所有片段
	cmd := exec.Command("ffmpeg",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
		"-c", "copy",
		"-y",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg concat error: %v, output: %s", err, string(output))
	}

	projectsMutex.Lock()
	project.FinalVideo = outputPath
	projectsMutex.Unlock()

	log.Printf("Created final video (no audio) for project %s", project.ID)
	return nil
}

func compositeVideoWithAudio(project *Project, outputPath string) error {
	outputDir := filepath.Dir(outputPath)

	log.Printf("Compositing video with TTS audio for project %s", project.ID)

	// 處理每個章節：調整影片時長以匹配音訊時長
	processedSegments := []string{}

	for i, chapter := range project.Story.Chapters {
		// 找到對應的影片
		var videoPath string
		for _, video := range project.Videos {
			if video.ID == chapter.VideoID {
				videoPath = video.Path
				break
			}
		}

		if videoPath == "" {
			log.Printf("Warning: video not found for chapter %d", i+1)
			continue
		}

		segmentPath := filepath.Join(outputDir, fmt.Sprintf("segment_%d.mp4", i+1))

		// 剪輯影片為 15 秒左右
		targetDuration := 15.0 // 目標 15 秒
		actualEndTime := chapter.StartTime + targetDuration
		if actualEndTime > chapter.EndTime {
			actualEndTime = chapter.EndTime
		}

		if chapter.AudioPath != "" && chapter.Duration > 0 {
			// 有音訊：調整影片速度以匹配音訊時長
			videoDuration := actualEndTime - chapter.StartTime

			// 如果音訊比影片長，減慢影片播放速度
			// 如果音訊比影片短，加快影片播放速度
			speedFactor := videoDuration / chapter.Duration

			// 限制速度範圍（0.5x - 2.0x）
			if speedFactor < 0.5 {
				speedFactor = 0.5
			} else if speedFactor > 2.0 {
				speedFactor = 2.0
			}

			log.Printf("Chapter %d: video=%.2fs, audio=%.2fs, speed=%.2fx",
				i+1, videoDuration, chapter.Duration, speedFactor)

			// 剪出影片片段並調整速度（移除原音）+ 淡入淡出
			log.Printf("Creating segment %d with speed adjustment (%.2fx) and fade: %s to %s", i+1,
				speedFactor, fmt.Sprintf("%.2f", chapter.StartTime), fmt.Sprintf("%.2f", actualEndTime))

			segmentDuration := actualEndTime - chapter.StartTime
			fadeDuration := 0.5

			// 組合濾鏡：速度調整 + 淡入淡出
			filterComplex := fmt.Sprintf("setpts=%.4f*PTS,fade=t=in:st=0:d=%.2f,fade=t=out:st=%.2f:d=%.2f",
				1.0/speedFactor, fadeDuration, segmentDuration-fadeDuration, fadeDuration)

			cmd := exec.Command("ffmpeg",
				"-i", videoPath,
				"-ss", fmt.Sprintf("%.2f", chapter.StartTime),
				"-t", fmt.Sprintf("%.2f", segmentDuration),
				"-filter:v", filterComplex,
				"-an", // 移除原音訊
				"-c:v", "libx264",
				"-preset", "fast",
				"-y",
				segmentPath+"_video.mp4",
			)

			if output, err := cmd.CombinedOutput(); err != nil {
				log.Printf("Failed to process video for chapter %d: %v, output: %s", i+1, err, string(output))
				continue
			}

			// 合併音訊與影片
			cmd = exec.Command("ffmpeg",
				"-i", segmentPath+"_video.mp4",
				"-i", chapter.AudioPath,
				"-c:v", "copy",
				"-c:a", "aac",
				"-shortest",
				"-y",
				segmentPath,
			)

			if output, err := cmd.CombinedOutput(); err != nil {
				log.Printf("Failed to merge audio for chapter %d: %v, output: %s", i+1, err, string(output))
				continue
			}

			// 清理臨時檔案
			os.Remove(segmentPath + "_video.mp4")

		} else {
			// 沒有音訊，只剪出影片（移除原音）+ 淡入淡出
			log.Printf("Creating segment %d without audio with fade effects: %s to %s", i+1,
				fmt.Sprintf("%.2f", chapter.StartTime), fmt.Sprintf("%.2f", actualEndTime))

			segmentDuration := actualEndTime - chapter.StartTime
			fadeDuration := 0.5 // 淡入淡出 0.5 秒

			// 淡入淡出濾鏡
			fadeFilter := fmt.Sprintf("fade=t=in:st=0:d=%.2f,fade=t=out:st=%.2f:d=%.2f",
				fadeDuration, segmentDuration-fadeDuration, fadeDuration)

			cmd := exec.Command("ffmpeg",
				"-i", videoPath,
				"-ss", fmt.Sprintf("%.2f", chapter.StartTime),
				"-t", fmt.Sprintf("%.2f", segmentDuration),
				"-vf", fadeFilter, // 加入淡入淡出
				"-an", // 移除原音訊
				"-c:v", "libx264",
				"-preset", "fast",
				"-y",
				segmentPath,
			)

			if output, err := cmd.CombinedOutput(); err != nil {
				log.Printf("Failed to create segment %d: %v, output: %s", i+1, err, string(output))
				continue
			}

			log.Printf("Segment %d created with fade effects", i+1)
		}

		processedSegments = append(processedSegments, segmentPath)
	}

	if len(processedSegments) == 0 {
		return fmt.Errorf("no segments were successfully processed")
	}

	// 建立拼接列表
	listFile := filepath.Join(outputDir, "concat_list.txt")
	f, err := os.Create(listFile)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, segmentPath := range processedSegments {
		fmt.Fprintf(f, "file '%s'\n", filepath.Base(segmentPath))
	}

	// 拼接所有片段
	tempConcatPath := filepath.Join(outputDir, "temp_concat.mp4")
	cmd := exec.Command("ffmpeg",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
		"-c", "copy",
		"-y",
		tempConcatPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg concat error: %v, output: %s", err, string(output))
	}

	// 如果有結尾圖片，加入結尾圖片和文字
	log.Printf("Checking ending image: path=%s, message=%s", project.EndingImage, project.Story.FinalMessage)

	if project.EndingImage != "" && project.Story != nil && project.Story.FinalMessage != "" {
		log.Printf("Adding ending image to video...")
		// 使用一個新的臨時檔案來存儲帶有結尾的影片
		videoWithEndingPath := filepath.Join(outputDir, "video_with_ending.mp4")

		if err := addEndingImage(project, tempConcatPath, videoWithEndingPath); err != nil {
			log.Printf("❌ Failed to add ending image: %v", err)
			// 如果失敗，使用沒有結尾的版本
			os.Rename(tempConcatPath, outputPath)
		} else {
			log.Printf("✅ Ending image added successfully")
			// 成功加入結尾，將結果移動到最終輸出路徑
			os.Rename(videoWithEndingPath, outputPath)
			os.Remove(tempConcatPath)
		}
	} else {
		log.Printf("No ending image or message, skipping. EndingImage=%s, FinalMessage=%v",
			project.EndingImage, project.Story != nil && project.Story.FinalMessage != "")
		os.Rename(tempConcatPath, outputPath)
	}

	projectsMutex.Lock()
	project.FinalVideo = outputPath
	projectsMutex.Unlock()

	log.Printf("Created final video with TTS audio for project %s", project.ID)
	return nil
}

func markProjectFailed(projectID, errorMsg string) {
	log.Printf("Project %s failed: %s", projectID, errorMsg)

	projectsMutex.Lock()
	if project, exists := projects[projectID]; exists {
		project.Status = "failed"
		project.Error = errorMsg
		project.UpdatedAt = time.Now()
	}
	projectsMutex.Unlock()
}

func getVideoDuration(videoPath string) float64 {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	duration := 0.0
	fmt.Sscanf(string(output), "%f", &duration)
	return duration
}

func getVideoResolution(videoPath string) (int, int) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=s=x:p=0",
		videoPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error getting video resolution: %v", err)
		return 0, 0
	}

	var width, height int
	fmt.Sscanf(strings.TrimSpace(string(output)), "%dx%d", &width, &height)
	return width, height
}

func escapeFFmpegText(text string) string {
	// FFmpeg drawtext 需要轉義特殊字符
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, "'", "\\'")
	text = strings.ReplaceAll(text, ":", "\\:")
	text = strings.ReplaceAll(text, "[", "\\[")
	text = strings.ReplaceAll(text, "]", "\\]")
	return text
}

func generateOwnerMessageTTS(message, outputPath string) error {
	log.Printf("Generating TTS for owner message: %s", message)

	requestBody := map[string]interface{}{
		"input": map[string]string{
			"text": message,
		},
		"voice": map[string]interface{}{
			"languageCode": "zh-TW",
			"name":         "cmn-TW-Wavenet-C", // 台灣中文男聲
			"ssmlGender":   "MALE",
		},
		"audioConfig": map[string]interface{}{
			"audioEncoding": "MP3",
			"speakingRate":  0.9,
			"pitch":         -2.0, // 稍微低沉一點
		},
	}

	return executeTTSRequest(requestBody, outputPath)
}

func generateDogResponseTTS(message, outputPath string) error {
	log.Printf("Generating TTS for dog response: %s", message)

	requestBody := map[string]interface{}{
		"input": map[string]string{
			"text": message,
		},
		"voice": map[string]interface{}{
			"languageCode": "zh-TW",
			"name":         "cmn-TW-Wavenet-A", // 台灣中文女聲（狗狗的聲音）
			"ssmlGender":   "FEMALE",
		},
		"audioConfig": map[string]interface{}{
			"audioEncoding": "MP3",
			"speakingRate":  0.95,
			"pitch":         2.0, // 稍微高一點，更可愛
		},
	}

	return executeTTSRequest(requestBody, outputPath)
}

func executeTTSRequest(requestBody map[string]interface{}, outputPath string) error {
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal TTS request: %v", err)
	}

	url := fmt.Sprintf("https://texttospeech.googleapis.com/v1/text:synthesize?key=%s", aiAPIKey)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create TTS request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send TTS request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TTS API error %d: %s", resp.StatusCode, string(body))
	}

	var ttsResponse struct {
		AudioContent string `json:"audioContent"`
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read TTS response: %v", err)
	}

	if err := json.Unmarshal(bodyBytes, &ttsResponse); err != nil {
		return fmt.Errorf("failed to parse TTS response: %v", err)
	}

	audioData, err := base64.StdEncoding.DecodeString(ttsResponse.AudioContent)
	if err != nil {
		return fmt.Errorf("failed to decode audio: %v", err)
	}

	if err := os.WriteFile(outputPath, audioData, 0644); err != nil {
		return fmt.Errorf("failed to write audio file: %v", err)
	}

	return nil
}

// 壓縮圖片到指定大小
func compressImage(inputPath string, maxWidth, maxHeight int) ([]byte, error) {
	// 使用 FFmpeg 壓縮圖片
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-vf", fmt.Sprintf("scale='min(%d,iw)':min'(%d,ih)':force_original_aspect_ratio=decrease", maxWidth, maxHeight),
		"-q:v", "5", // 品質 5（1-31，數字越小品質越高）
		"-f", "image2",
		"-",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg compress error: %v", err)
	}

	return output, nil
}

// ============================================================================
// Subtitles and Background Music
// ============================================================================

func addSubtitles(project *Project, inputVideo, outputVideo string) error {
	log.Printf("Adding subtitles to video for project %s", project.ID)

	// 建立 SRT 字幕檔案
	outputDir := filepath.Dir(inputVideo)
	srtPath := filepath.Join(outputDir, "subtitles.srt")

	f, err := os.Create(srtPath)
	if err != nil {
		return fmt.Errorf("failed to create subtitle file: %v", err)
	}
	defer f.Close()

	// 生成 SRT 格式字幕
	currentTime := 0.0
	subtitleIndex := 1

	// 添加前 5 個影片的字幕（狗狗的對白）
	for _, chapter := range project.Story.Chapters {
		startTime := currentTime
		endTime := currentTime + chapter.Duration

		// SRT 格式
		fmt.Fprintf(f, "%d\n", subtitleIndex)
		fmt.Fprintf(f, "%s --> %s\n", formatSRTTime(startTime), formatSRTTime(endTime))
		fmt.Fprintf(f, "%s\n\n", chapter.Narration)

		currentTime = endTime
		subtitleIndex++
	}

	// 結尾部分的字幕已由 addEndingImage 直接燒錄到影片中，此處不再添加 SRT 字幕
	// 這樣可以避免字幕重複或樣式衝突，並符合用戶需求

	// 使用 FFmpeg 將字幕燒錄到影片中
	// 字幕樣式：白色文字、黑色邊框、底部居中
	subtitleStyle := "FontSize=24,PrimaryColour=&H00FFFFFF,OutlineColour=&H00000000,BorderStyle=1,Outline=2,Shadow=1,MarginV=30"

	cmd := exec.Command("ffmpeg",
		"-i", inputVideo,
		"-vf", fmt.Sprintf("subtitles=%s:force_style='%s'", srtPath, subtitleStyle),
		"-c:a", "copy",
		"-y",
		outputVideo,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg subtitle error: %v, output: %s", err, string(output))
	}

	log.Printf("✅ Added subtitles for project %s (including ending)", project.ID)
	return nil
}

func formatSRTTime(seconds float64) string {
	hours := int(seconds / 3600)
	minutes := int((seconds - float64(hours*3600)) / 60)
	secs := int(seconds) % 60
	millis := int((seconds - float64(int(seconds))) * 1000)

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, millis)
}

func addBackgroundMusic(project *Project, inputVideo, outputVideo string) error {
	log.Printf("Adding background music to video for project %s", project.ID)

	// 生成背景音樂
	outputDir := filepath.Dir(inputVideo)
	musicPath := filepath.Join(outputDir, "background_music.mp3")

	// 先刪除舊的音樂文件（如果存在）
	if _, err := os.Stat(musicPath); err == nil {
		log.Printf("🗑️ Removing old background music file: %s", musicPath)
		os.Remove(musicPath)
	}

	// 檢查是否有指定的背景音樂檔案
	specificBGM := "./狗狗影片/bibi-pianopachelbels-canon-终于弹了这首-世界上最治愈的钢琴曲卡农.mp3"
	musicCopied := false

	log.Printf("🎵 Checking for specific BGM file: %s", specificBGM)
	if stat, err := os.Stat(specificBGM); err == nil {
		log.Printf("✅ Found specific BGM file: %s (size: %d bytes)", specificBGM, stat.Size())

		// 複製到輸出目錄以避免檔名問題
		inputMusic, err := os.ReadFile(specificBGM)
		if err == nil {
			log.Printf("📖 Successfully read BGM file, size: %d bytes", len(inputMusic))
			if err := os.WriteFile(musicPath, inputMusic, 0644); err != nil {
				log.Printf("❌ Failed to copy BGM, falling back to generation: %v", err)
			} else {
				// 驗證寫入
				if verifyStats, err := os.Stat(musicPath); err == nil {
					log.Printf("✅ Successfully copied BGM to: %s (size: %d bytes)", musicPath, verifyStats.Size())
					musicCopied = true
				} else {
					log.Printf("❌ Failed to verify copied BGM file: %v", err)
				}
			}
		} else {
			log.Printf("❌ Failed to read BGM, falling back to generation: %v", err)
		}
	} else {
		log.Printf("❌ Specific BGM not found: %s, error: %v", specificBGM, err)
	}

	// 如果沒有複製成功，則生成
	if !musicCopied {
		// 取得影片時長
		videoDuration := getVideoDuration(inputVideo)
		if videoDuration == 0 {
			return fmt.Errorf("failed to get video duration")
		}

		log.Printf("Generating background music with duration %.2fs", videoDuration)
		// 生成柔和的背景音樂
		if err := generateBackgroundMusic(musicPath, videoDuration); err != nil {
			return fmt.Errorf("failed to generate music: %v", err)
		}
	}

	// 將背景音樂與影片合併
	// 用戶要求背景音樂音量 100% (volume=1.0)
	// 原始影片音訊 (TTS) 音量保持 1.0
	// 在最後 3 秒淡出音訊
	videoDuration := getVideoDuration(inputVideo)
	fadeStartTime := videoDuration - 3.0
	if fadeStartTime < 0 {
		fadeStartTime = 0
	}

	// filter_complex: 混合音訊後，在最後 3 秒淡出
	filterComplex := fmt.Sprintf("[0:a]volume=1.0[a1];[1:a]volume=1.0[a2];[a1][a2]amix=inputs=2:duration=shortest,afade=t=out:st=%.2f:d=3[aout]", fadeStartTime)
	log.Printf("Audio filter: %s (video duration: %.2fs, fade start: %.2fs)", filterComplex, videoDuration, fadeStartTime)

	cmd := exec.Command("ffmpeg",
		"-i", inputVideo,
		"-i", musicPath,
		"-filter_complex", filterComplex,
		"-map", "0:v",
		"-map", "[aout]",
		"-c:v", "copy",
		"-c:a", "aac",
		"-y",
		outputVideo,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// 如果混合失敗（可能沒有原始音訊），嘗試直接加入音樂並淡出
		log.Printf("Audio mix failed, trying direct add with fade: %v", err)
		fadeFilter := fmt.Sprintf("afade=t=out:st=%.2f:d=3", fadeStartTime)
		cmd = exec.Command("ffmpeg",
			"-i", inputVideo,
			"-i", musicPath,
			"-filter_complex", fmt.Sprintf("[1:a]%s[aout]", fadeFilter),
			"-map", "0:v",
			"-map", "[aout]",
			"-c:v", "copy",
			"-c:a", "aac",
			"-shortest",
			"-y",
			outputVideo,
		)

		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("ffmpeg music add error: %v, output: %s", err, string(output))
		}
	}

	log.Printf("Added background music for project %s", project.ID)
	return nil
}

func generateBackgroundMusic(outputPath string, duration float64) error {
	// 生成溫柔的背景音樂
	// 使用 C 大調和弦 (C-E-G)

	cmd := exec.Command("ffmpeg",
		"-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=261.63:duration=%.2f", duration),
		"-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=329.63:duration=%.2f", duration),
		"-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=392.00:duration=%.2f", duration),
		"-filter_complex",
		"[0:a]volume=0.3[a0];[1:a]volume=0.2[a1];[2:a]volume=0.15[a2];[a0][a1][a2]amix=inputs=3:duration=first[aout]",
		"-map", "[aout]",
		"-c:a", "libmp3lame",
		"-b:a", "128k",
		"-y",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate music: %v, output: %s", err, string(output))
	}

	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func createStorageDirectories() {
	dirs := []string{
		filepath.Join(storagePath, "videos"),
		filepath.Join(storagePath, "frames"),
		filepath.Join(storagePath, "highlights"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
