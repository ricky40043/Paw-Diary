package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
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
	StoryMode         string      `json:"story_mode,omitempty"`         // 故事模式: warm(溫馨感人), cute(可愛活潑), funny(幽默風趣)
	EndingImage       string      `json:"ending_image,omitempty"`       // 結尾圖片路徑
	OwnerMessage      string      `json:"owner_message,omitempty"`      // 主人想對狗狗說的話
	Status            string      `json:"status"`                       // pending, analyzing, generating_story, generating_video, completed, failed
	Progress          int         `json:"progress"`                     // 0-100
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

type chunkUploadState struct {
	TotalChunks    int
	ReceivedChunks map[int]bool
	Filename       string
	ChunkDir       string
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

	// Chunked upload tracking
	chunkUploads      = make(map[string]*chunkUploadState)
	chunkUploadsMutex sync.Mutex

	storagePath   string
	aiAPIKey      string
	aiAPIEndpoint string
	aiModelName   string
	aiVisionModel string // 視覺分析模型
	aiTextModel   string // 文字生成模型
)

// ============================================================================
// Main Entry Point
// ============================================================================

func main() {
	// Load environment variables
	godotenv.Load()

	// Load configuration from env
	LoadConfig()

	port := getEnv("PORT", "8080")
	storagePath = getEnv("STORAGE_PATH", "./storage")
	aiAPIKey = getEnv("AI_API_KEY", "")
	aiAPIEndpoint = getEnv("AI_API_ENDPOINT", "https://openrouter.ai/api/v1/chat/completions")
	aiModelName = getEnv("AI_MODEL_NAME", "google/gemini-2.0-flash-001")
	aiVisionModel = getEnv("AI_VISION_MODEL", aiModelName) // 預設使用主模型
	aiTextModel = getEnv("AI_TEXT_MODEL", aiModelName)     // 預設使用主模型

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
			StoryMode         string `json:"story_mode"`         // warm, cute, funny
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
			return
		}

		// 預設關係為「主人」
		if req.OwnerRelationship == "" {
			req.OwnerRelationship = "主人"
		}

		// 驗證並設定故事模式，預設為 warm
		validModes := map[string]bool{"warm": true, "cute": true}
		if req.StoryMode == "" || !validModes[req.StoryMode] {
			req.StoryMode = "warm"
		}

		projectID := uuid.New().String()
		project := &Project{
			ID:                projectID,
			Name:              req.Name,
			DogName:           req.DogName,
			DogBreed:          req.DogBreed,
			OwnerRelationship: req.OwnerRelationship,
			StoryMode:         req.StoryMode,
			Status:            "pending",
			Progress:          0,
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

	// POST /api/v2/story/projects/:projectId/video-chunk - 分塊上傳（支援大檔）
	router.POST("/api/v2/story/projects/:projectId/video-chunk", func(c *gin.Context) {
		projectID := c.Param("projectId")
		projectsMutex.RLock()
		project, exists := projects[projectID]
		projectsMutex.RUnlock()
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		fileID := c.PostForm("file_id")
		chunkIndex, _ := strconv.Atoi(c.PostForm("chunk_index"))
		totalChunks, _ := strconv.Atoi(c.PostForm("total_chunks"))
		filename := c.PostForm("filename")
		if fileID == "" || totalChunks == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing chunk metadata"})
			return
		}

		chunk, err := c.FormFile("chunk")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No chunk data"})
			return
		}

		// 儲存 chunk
		chunkDir := filepath.Join(storagePath, "chunks", fileID)
		os.MkdirAll(chunkDir, 0755)
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%05d", chunkIndex))
		if err := c.SaveUploadedFile(chunk, chunkPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save chunk"})
			return
		}

		// 更新 chunk 狀態
		chunkUploadsMutex.Lock()
		if _, ok := chunkUploads[fileID]; !ok {
			chunkUploads[fileID] = &chunkUploadState{
				TotalChunks:    totalChunks,
				ReceivedChunks: map[int]bool{},
				Filename:       filename,
				ChunkDir:       chunkDir,
			}
		}
		state := chunkUploads[fileID]
		state.ReceivedChunks[chunkIndex] = true
		allReceived := len(state.ReceivedChunks) == totalChunks
		chunkUploadsMutex.Unlock()

		if !allReceived {
			c.JSON(http.StatusOK, gin.H{"received": len(state.ReceivedChunks), "total": totalChunks})
			return
		}

		// 所有 chunk 到齊，組合檔案
		ext := strings.ToLower(filepath.Ext(filename))
		if ext != ".mp4" && ext != ".mov" && ext != ".avi" {
			os.RemoveAll(chunkDir)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported video format"})
			return
		}

		projectDir := filepath.Join(storagePath, "projects", projectID)
		os.MkdirAll(projectDir, 0755)
		videoID := uuid.New().String()
		videoPath := filepath.Join(projectDir, videoID+ext)

		outFile, err := os.Create(videoPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create video file"})
			return
		}
		for i := 0; i < totalChunks; i++ {
			cp := filepath.Join(chunkDir, fmt.Sprintf("chunk_%05d", i))
			cf, err := os.Open(cp)
			if err != nil {
				outFile.Close()
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Missing chunk %d", i)})
				return
			}
			io.Copy(outFile, cf)
			cf.Close()
		}
		outFile.Close()

		// 清理 chunk 暫存
		os.RemoveAll(chunkDir)
		chunkUploadsMutex.Lock()
		delete(chunkUploads, fileID)
		chunkUploadsMutex.Unlock()

		duration := getVideoDuration(videoPath)
		videoInfo := VideoInfo{
			ID:           videoID,
			OriginalName: filename,
			Path:         videoPath,
			Duration:     duration,
			FramesDir:    filepath.Join(projectDir, videoID+"_frames"),
			Analyzed:     false,
		}

		projectsMutex.Lock()
		project.Videos = append(project.Videos, videoInfo)
		project.UpdatedAt = time.Now()
		projectsMutex.Unlock()

		log.Printf("✅ Chunked upload complete: %s (%d chunks)", filename, totalChunks)
		c.JSON(http.StatusOK, gin.H{"assembled": true, "video": videoInfo})
	})

	// DELETE /api/v2/story/projects/:projectId/videos/:videoId - 移除單支已上傳影片
	router.DELETE("/api/v2/story/projects/:projectId/videos/:videoId", func(c *gin.Context) {
		projectID := c.Param("projectId")
		videoID := c.Param("videoId")

		projectsMutex.Lock()
		defer projectsMutex.Unlock()

		project, exists := projects[projectID]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		idx := -1
		for i := range project.Videos {
			if project.Videos[i].ID == videoID {
				idx = i
				break
			}
		}
		if idx == -1 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
			return
		}

		// 刪除實體檔案與 frames
		if p := project.Videos[idx].Path; p != "" {
			os.Remove(p)
		}
		if d := project.Videos[idx].FramesDir; d != "" {
			os.RemoveAll(d)
		}
		project.Videos = append(project.Videos[:idx], project.Videos[idx+1:]...)
		project.UpdatedAt = time.Now()

		log.Printf("🗑️ Removed video %s from project %s", videoID, projectID)
		c.JSON(http.StatusOK, gin.H{"deleted": videoID, "remaining": len(project.Videos)})
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
			"story_mode":         project.StoryMode,
			"ending_image":       project.EndingImage,
			"status":             project.Status,
			"progress":           project.Progress,
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

// ============================================================================
// FFmpeg Hardware Acceleration (VAAPI)
// ============================================================================

// useVAAPI enables Intel VAAPI hardware encoding via ENABLE_VAAPI=true env var.
// Requires /dev/dri to be mounted in the container.
var useVAAPI = os.Getenv("ENABLE_VAAPI") == "true"

// vaapiVF appends hwupload to a software filter chain for VAAPI encoding.
// format=nv12 converts to VAAPI's preferred input format before upload.
func vaapiVF(vf string) string {
	if !useVAAPI {
		return vf
	}
	return vf + ",format=nv12,hwupload=extra_hw_frames=64,format=vaapi"
}

// 統一的輸出規格。所有片段（片頭、主片、結尾）都用相同解析度 / fps / 編碼參數
// 產生，之後才能用 concat demuxer 的 -c copy 無損串接（不需重新編碼）。
const (
	outputWidth  = 1280 // 720p：像素量約為 1080p 的 44%，編碼快很多，手機觀看幾乎無差
	outputHeight = 720
	outputFPS    = "30"
)

// h264Args returns video encoder args: h264_vaapi (GPU) or libx264 (CPU).
// 軟體路徑一律用 veryfast preset；preset 參數保留相容性但已不再放慢編碼。
// 一律輸出固定 fps，確保各片段參數一致以便後續 -c copy 串接。
func h264Args(preset string) []string {
	if useVAAPI {
		return []string{"-c:v", "h264_vaapi", "-r", outputFPS}
	}
	return []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-r", outputFPS}
}

// h264ArgsWithCRF returns video encoder args with CRF quality (CPU only).
func h264ArgsWithCRF(preset, crf string) []string {
	if useVAAPI {
		return []string{"-c:v", "h264_vaapi", "-r", outputFPS}
	}
	return []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", crf, "-r", outputFPS}
}

// concatCopy 用 concat demuxer 以 -c copy 串接多個已用相同參數編碼的片段，
// 完全不重新編碼，速度接近瞬間。失敗時回傳 error 讓呼叫端 fallback。
func concatCopy(parts []string, outputPath string) error {
	if len(parts) == 0 {
		return fmt.Errorf("concatCopy: no parts")
	}
	listPath := filepath.Join(filepath.Dir(outputPath), fmt.Sprintf("concat_copy_%d.txt", time.Now().UnixNano()))
	var sb strings.Builder
	for _, p := range parts {
		// concat demuxer 的路徑相對於 list 檔所在目錄
		sb.WriteString(fmt.Sprintf("file '%s'\n", filepath.Base(p)))
	}
	if err := os.WriteFile(listPath, []byte(sb.String()), 0644); err != nil {
		return err
	}
	defer os.Remove(listPath)

	cmd := exec.Command("ffmpeg",
		"-f", "concat", "-safe", "0",
		"-i", listPath,
		"-c", "copy",
		"-movflags", "+faststart",
		"-y", outputPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("concat copy error: %v, output: %s", err, string(output))
	}
	return nil
}

// pixFmtArgs returns pix_fmt yuv420p for software encode, empty for VAAPI.
func pixFmtArgs() []string {
	if useVAAPI {
		return nil
	}
	return []string{"-pix_fmt", "yuv420p"}
}

// vaapiGlobalArgs returns -init_hw_device and -filter_hw_device flags required
// for hwupload to work in filter chains. Must be prepended before -i flags.
func vaapiGlobalArgs() []string {
	if !useVAAPI {
		return nil
	}
	device := os.Getenv("VAAPI_DEVICE")
	if device == "" {
		device = "/dev/dri/renderD128"
	}
	return []string{
		"-init_hw_device", "vaapi=va:" + device,
		"-filter_hw_device", "va",
	}
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

	// 智能選擇最多 N 張代表性圖片（均勻分佈）
	maxImages := MaxAIImages
	selectedFrames := []string{}

	if len(framePaths) <= maxImages {
		// 圖片數量不多，全部使用
		selectedFrames = framePaths
	} else {
		// 均勻選擇 N 張圖片
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
		// 送 AI 的圖片放大到 1024（原本 320x240 太小，導致場景/抱姿誤判）
		// 視覺模型對每張圖計費大致固定，放大解析度幾乎不增加成本卻大幅提升準確度
		compressedData, err := compressImage(framePath, 1024, 1024)
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

	// 構建 OpenRouter (OpenAI-compatible) API 請求
	// OpenAI Chat Completion with Vision format
	log.Printf("🤖 Using Vision Model: %s", aiVisionModel)
	requestBody := map[string]interface{}{
		"model": aiVisionModel,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
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
				},
			},
		},
		"temperature": 0.4,
		"max_tokens":  2000,
	}

	// 添加所有圖片 (OpenAI Vision Format)
	contentList := requestBody["messages"].([]map[string]interface{})[0]["content"].([]map[string]interface{})
	for _, imgData := range base64Images {
		contentList = append(contentList, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]string{
				"url": fmt.Sprintf("data:image/jpeg;base64,%s", imgData),
			},
		})
	}
	requestBody["messages"].([]map[string]interface{})[0]["content"] = contentList

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// 發送請求 (OpenRouter)
	req, err := http.NewRequest("POST", aiAPIEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", aiAPIKey))
	req.Header.Set("HTTP-Referer", "https://pawdiary.app") // Required by OpenRouter
	req.Header.Set("X-Title", "Paw Diary")

	client := &http.Client{Timeout: 90 * time.Second} // OpenRouter might be slower for Vision
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析 OpenAI 格式回應
	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v, body: %s", err, string(bodyBytes))
	}

	if apiResponse.Error != nil {
		return nil, fmt.Errorf("AI API Error: %s", apiResponse.Error.Message)
	}

	if len(apiResponse.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := apiResponse.Choices[0].Message.Content
	log.Printf("AI response content: %s", content)

	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var analysis Analysis
	if err := json.Unmarshal([]byte(content), &analysis); err != nil {
		// Fallback: search for json block
		start := strings.Index(content, "{")
		end := strings.LastIndex(content, "}")
		if start != -1 && end != -1 && end > start {
			jsonContent := content[start : end+1]
			if err2 := json.Unmarshal([]byte(jsonContent), &analysis); err2 == nil {
				goto AnalysisSuccess
			}
		}
		return nil, fmt.Errorf("failed to parse AI response: %v, content: %s", err, content)
	}

AnalysisSuccess:

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
	start := time.Now()

	if aiAPIKey == "" {
		markProjectFailed(projectID, "AI_API_KEY 未設定，請在 Render Dashboard 的 Environment 頁面填入 API Key")
		return
	}

	projectsMutex.Lock()
	project := projects[projectID]
	project.Status = "analyzing"
	project.Progress = 5
	project.UpdatedAt = time.Now()
	projectsMutex.Unlock()

	log.Printf("Processing project %s with %d videos", projectID, len(project.Videos))

	// Step 1: Analyze all videos (concurrently)
	log.Printf("Starting parallel analysis for %d videos", len(project.Videos))

	var wg sync.WaitGroup
	successChan := make(chan bool, len(project.Videos))
	totalVideos := len(project.Videos)
	completedVideos := 0

	for i := range project.Videos {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			if err := analyzeVideo(project, index); err != nil {
				log.Printf("⚠️ Warning: Failed to analyze video %s: %v (continuing)", project.Videos[index].ID, err)
				return // Don't send to successChan
			}
			successChan <- true

			// Update progress incrementally for analysis (5% to 45%)
			projectsMutex.Lock()
			completedVideos++
			// Progress from 5 to 45 mainly based on video analysis
			project.Progress = 5 + int(40.0*float64(completedVideos)/float64(totalVideos))
			projectsMutex.Unlock()
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(successChan)

	successCount := 0
	for range successChan {
		successCount++
	}

	analysisDuration := time.Since(start)
	log.Printf("Step 1: Video Analysis took %v", analysisDuration)

	// 至少要有一半的影片分析成功才能繼續
	if successCount == 0 {
		markProjectFailed(projectID, "All videos failed to analyze")
		return
	}

	log.Printf("✅ Successfully analyzed %d/%d videos", successCount, len(project.Videos))

	// Step 2: Generate story with AI
	projectsMutex.Lock()
	project.Status = "generating_story"
	project.Progress = 50
	project.UpdatedAt = time.Now()
	projectsMutex.Unlock()

	storyStart := time.Now()
	story, err := generateStoryWithAI(project)
	if err != nil {
		markProjectFailed(projectID, "Failed to generate story: "+err.Error())
		return
	}
	storyDuration := time.Since(storyStart)
	log.Printf("Step 2: Story Generation took %v", storyDuration)

	projectsMutex.Lock()
	project.Story = story
	project.Status = "generating_video"
	project.Progress = 70
	project.UpdatedAt = time.Now()
	projectsMutex.Unlock()

	// Step 3: Generate TTS audio for each chapter
	// Consider TTS as part of video generation phase (70-80%)
	// ttsStart := time.Now()
	// for i := range project.Story.Chapters {
	// 	if err := generateTTS(project, i); err != nil {
	// 		log.Printf("Warning: TTS generation failed for chapter %d: %v", i, err)
	// 		// Continue without audio
	// 	}
	// 	// Increment progress slightly
	// 	projectsMutex.Lock()
	// 	project.Progress = 70 + int(10.0*float64(i+1)/float64(len(project.Story.Chapters)))
	// 	projectsMutex.Unlock()
	// }
	// ttsDuration := time.Since(ttsStart)
	// log.Printf("Step 3: TTS Generation took %v", ttsDuration)
	log.Printf("Step 3: TTS Generation Skipped (User Request)")

	// Step 4: Composite final video (with subtitles and background music)
	compositeStart := time.Now()
	if err := compositeVideo(project); err != nil {
		markProjectFailed(projectID, "Failed to composite video: "+err.Error())
		return
	}
	compositeDuration := time.Since(compositeStart)
	log.Printf("Step 4: Video Composition took %v", compositeDuration)

	totalDuration := time.Since(start)
	log.Printf("🔥 Total Processing Time: %v", totalDuration)

	projectsMutex.Lock()
	project.Status = "completed"
	project.Progress = 100
	project.UpdatedAt = time.Now()
	projectsMutex.Unlock()

	// 清理原始影片與 frames（保留最終影片和結尾圖片）
	for _, video := range project.Videos {
		if video.Path != "" {
			os.Remove(video.Path)
		}
		if video.FramesDir != "" {
			os.RemoveAll(video.FramesDir)
		}
	}
	log.Printf("🗑️ Cleaned up original videos and frames for project %s", projectID)

	log.Printf("Project %s completed successfully", projectID)
}

func analyzeVideo(project *Project, videoIndex int) error {
	video := &project.Videos[videoIndex]

	log.Printf("Analyzing video %s (%s)", video.ID, video.OriginalName)

	// Extract frames - 使用 config.go 中的設定
	os.MkdirAll(video.FramesDir, 0755)
	outputPattern := filepath.Join(video.FramesDir, "frame_%04d.jpg")
	// -t: 只讀取前 N 秒
	// fps: 1/AnalysisInterval fps (e.g., 2.0s -> 0.5 fps)
	fps := 1.0 / AnalysisInterval
	// 720p 影格：保留足夠細節讓 AI 正確判斷場景（室內外、抱姿等）
	cmd := exec.Command("ffmpeg",
		"-t", fmt.Sprintf("%d", AnalysisDurationSeconds),
		"-i", video.Path,
		"-vf", fmt.Sprintf("fps=%.4f,scale=1280:720:force_original_aspect_ratio=decrease", fps),
		outputPattern)
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
			InteractionType: "playing",
			Emotion:         "neutral",
			ShortCaption:    "影片分析",
		}
	}

	// 根據 AI 分析結果創建 segments
	// frames per segment = SegmentDurationSeconds / AnalysisInterval
	framesPerSegment := int(float64(SegmentDurationSeconds) / AnalysisInterval)
	if framesPerSegment < 1 {
		framesPerSegment = 1
	}
	segments := []Segment{}

	frameDuration := AnalysisInterval

	for i := 0; i < len(files); i += framesPerSegment {
		end := i + framesPerSegment
		if end > len(files) {
			end = len(files)
		}

		segment := Segment{
			Index:      len(segments) + 1,
			Start:      float64(i) * frameDuration,
			End:        float64(end) * frameDuration,
			FramePaths: files[i:end],
			Analysis:   analysis, // 所有 segment 使用相同的分析結果
		}
		segments = append(segments, segment)
	}

	// Find highlights based on analysis
	highlights := []Highlight{}

	// 如果有互動，將整個影片（或前幾個 segment）標記為 highlight
	if analysis.HasDog && analysis.HasHuman && analysis.InteractionType != "none" {
		// 取前 AnalysisDurationSeconds 秒作為 highlight
		maxHighlightDuration := float64(AnalysisDurationSeconds)
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
	log.Printf("Generating story for project %s with AI (mode: %s)", project.ID, project.StoryMode)

	// 品種預設值
	dogBreed := project.DogBreed
	if dogBreed == "" {
		dogBreed = "狗狗"
	}

	// 為「每一支影片」建立一行描述（用 AI 視覺分析的 caption/emotion）。
	// 對白會「一支影片一段、依序對應」，且必須緊扣該片描述，避免文字與畫面對不上。
	videoDescriptions := []string{}
	for i, video := range project.Videos {
		caption := "與狗狗相處的溫馨畫面"
		emotion := ""
		if len(video.Highlights) > 0 {
			if video.Highlights[0].Caption != "" {
				caption = video.Highlights[0].Caption
			}
			emotion = video.Highlights[0].Emotion
		}
		if emotion != "" {
			videoDescriptions = append(videoDescriptions, fmt.Sprintf("影片 %d：%s（狗狗情緒：%s）", i, caption, emotion))
		} else {
			videoDescriptions = append(videoDescriptions, fmt.Sprintf("影片 %d：%s", i, caption))
		}
	}

	if len(videoDescriptions) == 0 {
		return nil, fmt.Errorf("no videos to generate story")
	}
	numChapters := len(videoDescriptions)

	// 根據關係設定稱呼
	ownerTitle := project.OwnerRelationship
	if ownerTitle == "" {
		ownerTitle = "主人"
	}

	// 根據模式設定不同的提示詞風格
	var modeStyle, modeExamples, modeEmotion, modeChapter5Instruction string
	switch project.StoryMode {
	case "cute": // 可愛活潑
		modeStyle = "活潑、親人、愛撒嬌、對什麼事都充滿好奇心的開心小狗"
		modeEmotion = "充滿活力、直率開心、黏人，說話像個精力充沛的小朋友，不裝深沉，就是單純地喜歡你。偶爾用語氣詞（嘿嘿、好啦、欸）自然帶出個性。"
		modeExamples = fmt.Sprintf(`「%s你回來啦！我在門口等好久了，快點快點，我要撲過去！」
「每次你叫我名字，我的耳朵就自己豎起來，我根本沒辦法不理你。」
「%s，我今天有超乖，所以可以多給我一個抱抱嗎？就一個就好啦嘿嘿。」`,
			ownerTitle, ownerTitle)
		modeChapter5Instruction = "最後一段要比前幾段更撒嬌，帶著依依不捨但又俏皮的感覺，像是「就算你出門不帶我，我也要偷偷跟去啦」這類句子，讓人忍不住又笑又鼻酸。"

	default: // warm - 溫馨感人
		modeStyle = "溫柔、感性、很在意細節的小天使狗狗"
		modeEmotion = "溫馨、感動、深情，用具體回憶來表達對主人的依戀與感謝，而不是一直重複同一句話。"
		modeExamples = fmt.Sprintf(`「%s你看，我跑得有點慢了，可是我還是想要走到門口等你。」
「只要你在，我就覺得家裡好安靜、好安全，我可以放心地睡在你腳邊。」
「謝謝你一直陪著我，累的時候還是會摸摸我、叫我的名字，我真的好喜歡那個聲音。」`,
			ownerTitle)
		modeChapter5Instruction = "最後一段要特別有感情，帶一點不捨與感謝，可以提到「就算看不到我，我還是在你身邊」這類句子，讓人覺得被好好抱住。"
	}

	// 構建 prompt - 一支影片一段對白，且必須緊扣該片實際畫面（避免文字對不上影片）
	prompt := fmt.Sprintf(`你是一隻名叫「%s」的%s，是一個有靈魂的小毛孩。
請用「第一人稱」的口吻，像一個 3～5 歲的小朋友，看著這些回憶影片，對你的「%s」說悄悄話。

🎭 本次風格設定：
- 角色性格：%s
- 情感基調：%s
- 你非常愛你的%s，也非常依賴他/她。

下面是這次要做成影片的片段，每一行是「一支影片」的實際畫面描述（依序編號）：
%s

請替「狗狗本人」寫出正好 %d 段對白，一支影片一段、順序與上面完全一致：
第 0 段對應「影片 0」、第 1 段對應「影片 1」，以此類推。

🚨 最重要的規則（務必遵守，否則文字會跟畫面對不上）：
- 每一段「只能描述該編號影片裡實際出現的畫面與動作」。
- 嚴禁虛構描述裡沒提到的場景或動作。例如描述沒提到「門口、散步、睡覺、奔跑」，就絕對不要寫。
- 若描述是「在車內被抱著」，就寫被抱著、靠在身上、吐舌的感覺，不要寫成在外面跑跳。

其他要求：
1. 用「我」稱呼自己，用「%s」稱呼對方；口吻單純直接、像小孩講話、有情緒層次。
2. 每段 2～3 句短句，約 30～60 個中文字。
3. 情緒大多溫暖／可愛（依風格）；%s
4. 少用空泛形容詞與制式句（如「你是我最好的朋友」「謝謝你的陪伴」），多寫來自該畫面的具體細節。

風格示意（只參考語氣與情緒，不要照抄）：
%s

請用「嚴格 JSON」回應，chapters 必須正好 %d 段，video_index 依序為 0,1,2…：

{
  "title": "給%s的悄悄話",
  "chapters": [
    {"narration": "對應影片0的對白", "video_index": 0, "highlight_index": 0},
    {"narration": "對應影片1的對白", "video_index": 1, "highlight_index": 0}
  ]
}

注意：
- chapters 數量必須「正好等於 %d」，video_index 從 0 連續編到 %d，不可跳號或超出。
- 只回傳 JSON，不要任何註解、markdown 或多餘文字。`,
		project.DogName,
		dogBreed,
		ownerTitle,
		modeStyle,
		modeEmotion,
		ownerTitle,
		strings.Join(videoDescriptions, "\n"),
		numChapters,
		ownerTitle,
		modeChapter5Instruction,
		modeExamples,
		numChapters,
		ownerTitle,
		numChapters,
		numChapters-1)

	// 調用 OpenRouter (OpenAI-compatible) API
	log.Printf("🤖 Using Text Model for story: %s", aiTextModel)
	requestBody := map[string]interface{}{
		"model": aiTextModel,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": prompt,
					},
				},
			},
		},
		"temperature": 0.8,
		"max_tokens":  8000,
	}

	jsonData, _ := json.Marshal(requestBody)
	// OpenRouter API
	req, _ := http.NewRequest("POST", aiAPIEndpoint, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", aiAPIKey))
	req.Header.Set("HTTP-Referer", "https://pawdiary.app")
	req.Header.Set("X-Title", "Paw Diary")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// OpenAI Response Structure
	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v, body: %s", err, string(bodyBytes))
	}

	if apiResponse.Error != nil {
		return nil, fmt.Errorf("AI API Error: %s", apiResponse.Error.Message)
	}

	if len(apiResponse.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := apiResponse.Choices[0].Message.Content
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

	// 收集影片描述（給模型一點上下文，不用太長）
	videoDescriptions := []string{}
	for i, chapter := range story.Chapters {
		videoDescriptions = append(videoDescriptions, fmt.Sprintf("影片 %d: %s", i+1, chapter.Narration))
	}

	// 品種預設值
	dogBreed := project.DogBreed
	if dogBreed == "" {
		dogBreed = "狗狗"
	}

	// 根據關係設定稱呼
	ownerTitle := project.OwnerRelationship
	if ownerTitle == "" {
		ownerTitle = "主人"
	}

	// 根據模式設定不同的回應風格
	var modeStyle, modeEmotion, modeExamples, modeNote string
	switch project.StoryMode {
	case "cute": // 可愛活潑
		modeStyle = "活潑、愛撒嬌、對什麼都很好奇、精力充沛的開心小狗"
		modeEmotion = "充滿活力、直率開心、超黏人，說話像精力充沛的小孩，不裝深沉，就是直白地說「我好愛你」。偶爾用語氣詞（嘿嘿、欸、好啦）帶出個性，但不要每句都裝可愛。"
		modeExamples = fmt.Sprintf(`「%s你說那麼多話，我的耳朵都豎起來聽了，你的聲音我一秒就認得出來！」
「欸%s，你哭的時候我就想爬到你腿上，因為那樣感覺你就不難過了，嘿嘿。」`,
			ownerTitle, ownerTitle)
		modeNote = "語氣要活潑直率，像一隻開心黏人的小狗直接說出心裡話，可以用語氣詞帶出個性，但不要整段都是疊字或太做作。結尾要讓人覺得被這隻活潑的小狗抱住了。"

	default: // warm - 溫馨感人
		// 感人模式：像小孩很認真在安慰最重要的大人
		modeStyle = "溫柔、感性、特別在意%s心情、很怕你難過的小天使狗狗"
		modeStyle = fmt.Sprintf(modeStyle, ownerTitle)
		modeEmotion = "溫馨、感動、帶著深深的思念，像小朋友抱著大人的手，一邊說話一邊偷偷安慰對方。"
		modeExamples = fmt.Sprintf(`「%s，我真的有一個一個記住你說的每一句話，你難過的時候，我也好想抱抱你。」
「%s，你不要一直覺得自己一個人走，我會像以前一樣，在你看不到的地方跟著你走路、陪你回家、在門口等你。」`,
			ownerTitle, ownerTitle)
		modeNote = "請用具體畫面（等你回家、一起睡覺、聽你說話、跟著你走路…）來表達思念和感謝，而不是只重複『謝謝你』『我愛你』這些字。整體情緒要溫暖、讓人有被好好抱住的感覺。"
	}

	// 根據模式設定基調與語氣指令
	var toneInstruction, modeFinalInstruction string
	switch project.StoryMode {
	case "cute":
		toneInstruction = "用天真、直率、充滿活力的語氣說話，就像你平常最開心的樣子。不用裝成熟，直接說出你單純的愛和黏人的感覺。"
		modeFinalInstruction = fmt.Sprintf("活潑、真誠、像一隻開心撒嬌的小狗對 %s 說的結尾告白", ownerTitle)
	default: // warm
		toneInstruction = "用成熟、溫柔的大人語氣說話，好像一個長大後的孩子在安慰自己最重要的家人。"
		modeFinalInstruction = fmt.Sprintf("溫暖、真誠、像一位長大後的孩子對 %s 說的結尾告白", ownerTitle)
	}

	// 建立 prompt：讓狗狗在結尾說一段「成熟、真心安慰媽媽」的告白
	prompt := fmt.Sprintf(`你是一隻名叫「%s」的%s。你的「%s」剛剛對你說了一段很重要的話，裡面充滿了想念和感謝。
	請你以一隻%s的狗狗身份，對 %s 說一段真心的結尾告白。這段話會出現在故事的最後，但內容本身不要提到「影片」「畫面」這些字，就當作你真的站在她面前，安安靜靜地把心裡話說完。

	【你的角色設定】
	- 你是：%s
	- 語氣特徵：%s
	- 特別注意：%s

	【%s 對你說的話】（請真正參考裡面的情緒與重點）：
	「%s」

	【你們一起經歷過的一些回憶畫面】（只作為靈感參考，不用逐條回應）：
	%s

	請以「狗狗自己的第一人稱（我）」回應，創作一段給 %s 的結尾告白，遵守以下要求：

	1. 語氣：
	   - %s
	   - 可以帶一點撒嬌或俏皮，但整體要穩定、真誠、讓人覺得被好好抱住。
	   - 根據當前模式維持風格：%s。

	2. 內容：
	   - 不要解釋或重複「你剛剛說了什麼」，直接表達對她的愛和感謝
	   - 可以簡單提到 1 個具體回憶或感受
	   - 表達你對她的愛、感激和陪伴

	3. 字數與句子：
	   - **重要！！！嚴格控制在 80-100 個中文字之間**
	   - **絕對不能超過 100 字，也不能少於 80 字**
	   - 不要寫太短，多說一點心裡話，讓表達更完整
	   - 簡潔有力，每個字都要有意義
	   - 如果超過 100 字，請刪減內容直到符合字數

	4. 稱呼與限制：
	   - 回應中要直接叫「%s」至少一次
	   - 不要使用「汪汪」「嗚嗚」這類擬聲詞
	   - 不要提到「影片」「畫面」等詞
	   - **再次強調：總字數必須在 80-100 字之間，請務必計算字數**
	   - 只回傳狗狗說的話，不要任何其他內容

	【風格示意（只參考語氣，不要照抄）】：
	%s

	請根據以上資訊，寫出一段%s。只回傳那一段對白文字，不要其他內容。`,
		project.DogName,
		dogBreed,
		ownerTitle,
		modeStyle,
		ownerTitle,
		modeStyle,
		modeEmotion,
		modeNote,
		ownerTitle,
		project.OwnerMessage,
		strings.Join(videoDescriptions, "\n"),
		ownerTitle,
		toneInstruction,
		modeEmotion,
		ownerTitle,
		modeExamples,
		modeFinalInstruction)

	log.Printf("Dog response prompt (mode=%s): %s", project.StoryMode, prompt)
	log.Printf("ＡＬＬ prompt：%s", prompt)

	// 調用 OpenRouter (OpenAI-compatible) API
	log.Printf("🤖 Using Text Model for dog response: %s", aiTextModel)
	requestBody := map[string]interface{}{
		"model": aiTextModel,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": prompt,
					},
				},
			},
		},
		"temperature": 0.9,
		"max_tokens":  2000,
	}

	jsonData, _ := json.Marshal(requestBody)
	// OpenRouter API
	req, _ := http.NewRequest("POST", aiAPIEndpoint, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", aiAPIKey))
	req.Header.Set("HTTP-Referer", "https://pawdiary.app")
	req.Header.Set("X-Title", "Paw Diary")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// OpenAI Response Structure
	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(bodyBytes, &apiResponse); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	if apiResponse.Error != nil {
		return "", fmt.Errorf("AI API Error: %s", apiResponse.Error.Message)
	}

	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	response := apiResponse.Choices[0].Message.Content
	log.Printf("Dog AI response: %s", response)

	response = strings.TrimSpace(response)
	// 去掉可能包起來的引號或書名號
	response = strings.Trim(response, "「」\"")

	// 檢查長度與結尾，避免看起來像「講到一半就被切斷」
	runeCount := len([]rune(response))
	log.Printf("Generated dog response (cleaned, runes=%d): %s", runeCount, response)

	// 強制限制字數在 80-100 字之間
	if runeCount > 100 {
		log.Printf("⚠️ Dog response too long (%d chars), truncating to 100 chars", runeCount)
		runes := []rune(response)
		// 截取前 100 字
		response = string(runes[:100])
		// 找最後一個句號、逗號或感嘆號的位置，在那裡截斷比較自然
		lastPunc := -1
		responseRunes := []rune(response)
		for i := len(responseRunes) - 1; i >= 80; i-- {
			if responseRunes[i] == '。' || responseRunes[i] == '，' || responseRunes[i] == '！' {
				lastPunc = i + 1
				break
			}
		}
		if lastPunc > 80 {
			response = string(responseRunes[:lastPunc])
		}
		runeCount = len([]rune(response))
		log.Printf("✂️ Truncated to %d chars: %s", runeCount, response)
	} else if runeCount < 60 { // 放寬下限一點點，避免稍微短一點的被換掉
		log.Printf("⚠️ Dog response too short (%d chars), using fallback", runeCount)
		response = fmt.Sprintf("%s，謝謝你這麼愛我。雖然我現在不在你身邊，但我會永遠住在你心裡。記得要開心喔，因為我最喜歡看你笑的樣子了！我會一直在天上守護著你的，永遠愛你。", ownerTitle)
		runeCount = len([]rune(response))
	}

	// 1) 如果回應太短（少於 120 個中文字），直接使用預設的感人結尾
	// if runeCount < 120 {
	// 	log.Printf("⚠️ Dog response too short (%d runes), using fallback", runeCount)
	// 	response = fmt.Sprintf("%s，謝謝你給我這麼多的愛，每一個和你在一起的日子，都是我最珍貴的回憶。你抱著我的時候，我覺得全世界都是溫暖的。就算現在你看不到我，我也會一直守在你身邊，陪你走過每一個早晨和夜晚。不要難過，因為我從來沒有離開過你。%s，我永遠愛你。", ownerTitle, ownerTitle)
	// 	return response, nil
	// }

	// 2) 確保結尾有標點符號（但不要再加長文字，避免超過字數限制）
	if !strings.HasSuffix(response, "。") && !strings.HasSuffix(response, "！") && !strings.HasSuffix(response, "？") {
		response = response + "。"
	}

	// 最終檢查字數
	finalRuneCount := len([]rune(response))
	log.Printf("Final dog response (runes=%d): %s", finalRuneCount, response)

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
	log.Printf("Compositing final video for project %s (transitions, subtitles, title, ending, music)", project.ID)

	if len(project.Story.Chapters) == 0 {
		return fmt.Errorf("no chapters to composite")
	}

	outputDir := filepath.Join(storagePath, "projects", project.ID)

	// ---------------------------------------------------------------------
	// 新版 pipeline：只在「燒字幕」做一次完整重新編碼，其餘片段（片頭、結尾）
	// 都用相同參數產生後，以 concat demuxer -c copy 無損串接，速度大幅提升。
	// 整支影片無 TTS，故全程影片無音軌，最後一步才加入背景音樂作為唯一音軌。
	// ---------------------------------------------------------------------

	// Step 1：把各章節剪成片段並串成主片（720p、無音軌）
	log.Printf("Step 1: Creating main video from chapter segments")
	mainPath := filepath.Join(outputDir, "main.mp4")
	if err := createVideoWithTransitionsAndTTS(project, mainPath); err != nil {
		return fmt.Errorf("failed to create main video: %v", err)
	}
	setProjectProgress(project, 80)

	// Step 2：燒字幕（唯一一次完整重新編碼）
	log.Printf("Step 2: Burning subtitles")
	subPath := filepath.Join(outputDir, "main_sub.mp4")
	if err := addSubtitles(project, mainPath, subPath); err != nil {
		log.Printf("Warning: Failed to add subtitles: %v, continuing without subtitles", err)
		subPath = mainPath
	}
	setProjectProgress(project, 86)

	// 串接順序：片頭 + 主片 + 結尾，全部用 -c copy
	parts := []string{}

	// Step 3：片頭字卡（短、720p、無音軌）
	titlePath := filepath.Join(outputDir, "title_card.mp4")
	if err := createTitleCard(project, titlePath); err != nil {
		log.Printf("Warning: Failed to create title card: %v, skipping", err)
	} else {
		parts = append(parts, titlePath)
	}

	parts = append(parts, subPath)

	// Step 4：結尾圖片片段（短、720p、無音軌）
	if project.EndingImage != "" {
		ensureDogResponse(project)
		endingPath := filepath.Join(outputDir, "ending_segment.mp4")
		if err := createEndingSegment(project, endingPath); err != nil {
			log.Printf("Warning: Failed to create ending segment: %v, skipping", err)
		} else {
			parts = append(parts, endingPath)
		}
	}
	setProjectProgress(project, 90)

	// Step 5：用 -c copy 串接所有片段（幾乎瞬間，不重新編碼）
	log.Printf("Step 3: Concatenating %d parts with stream copy", len(parts))
	assembledPath := filepath.Join(outputDir, "assembled.mp4")
	if len(parts) == 1 {
		// 只有主片，直接用
		os.Rename(parts[0], assembledPath)
	} else if err := concatCopy(parts, assembledPath); err != nil {
		log.Printf("Warning: concat copy failed (%v), falling back to subtitled main only", err)
		// fallback：至少輸出有字幕的主片
		if err := copyFile(subPath, assembledPath); err != nil {
			return fmt.Errorf("failed to assemble video: %v", err)
		}
	}
	setProjectProgress(project, 94)

	// Step 6：加入背景音樂（影片 -c copy，只編碼音訊）
	log.Printf("Step 4: Adding background music")
	finalVideoPath := filepath.Join(outputDir, "final.mp4")
	if err := addBackgroundMusic(project, assembledPath, finalVideoPath); err != nil {
		log.Printf("Warning: Failed to add background music: %v, using version without music", err)
		os.Rename(assembledPath, finalVideoPath)
	} else {
		os.Remove(assembledPath)
	}
	setProjectProgress(project, 98)

	// 清理中間檔案
	os.Remove(mainPath)
	os.Remove(titlePath)
	os.Remove(filepath.Join(outputDir, "ending_segment.mp4"))
	if subPath != mainPath {
		os.Remove(subPath)
	}

	projectsMutex.Lock()
	project.FinalVideo = finalVideoPath
	projectsMutex.Unlock()

	log.Printf("✅ Created final video with all effects for project %s", project.ID)
	return nil
}

// ensureDogResponse 確保結尾要顯示的狗狗回應文字存在（必要時用 AI 生成或退回預設）。
func ensureDogResponse(project *Project) {
	if project.OwnerMessage != "" && (project.Story.DogResponse == "" || project.Story.DogResponse == "主人，我愛你！") {
		log.Printf("🤖 Regenerating dog response based on owner message")
		if dogResponse, err := generateDogResponse(project, project.Story); err != nil {
			log.Printf("⚠️ Failed to generate dog response: %v, using default", err)
			project.Story.DogResponse = fmt.Sprintf("%s，我也永遠愛你！每天和你在一起，是我最幸福的時光。", ownerTitleOf(project))
		} else {
			project.Story.DogResponse = dogResponse
			log.Printf("✅ Generated dog response: %s", dogResponse)
		}
		project.Story.OwnerMessage = project.OwnerMessage
	} else if project.Story.DogResponse == "" {
		project.Story.DogResponse = fmt.Sprintf("%s，我愛你！每天和你在一起，是我最幸福的時光～", ownerTitleOf(project))
	}
}

func ownerTitleOf(project *Project) string {
	if project.OwnerRelationship == "" {
		return "主人"
	}
	return project.OwnerRelationship
}

// createVideoWithTransitionsAndTTS - 創建帶轉場效果和 TTS 的影片（移除原始音訊）
func createVideoWithTransitionsAndTTS(project *Project, outputPath string) error {
	outputDir := filepath.Dir(outputPath)

	log.Printf("🎬 Creating video segments with fade transitions and TTS audio")

	// 統一目標尺寸為 16:9 720p（加速編碼，手機觀看幾乎無差）
	targetWidth := outputWidth
	targetHeight := outputHeight
	log.Printf("📐 Target resolution: %dx%d (16:9)", targetWidth, targetHeight)

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
			log.Printf("⚠️ Warning: video not found for chapter %d", i+1)
			continue
		}

		// 獲取原始影片尺寸
		origWidth, origHeight := getVideoResolution(videoPath)
		log.Printf("📹 Chapter %d: original size=%dx%d, duration=%.2f-%.2f",
			chapter.Index, origWidth, origHeight, chapter.StartTime, chapter.EndTime)

		// 剪切影片片段（移除音訊）
		segmentPath := filepath.Join(outputDir, fmt.Sprintf("segment_%d.mp4", chapter.Index))

		// 計算淡入淡出
		fadeDuration := 0.5
		videoDuration := chapter.EndTime - chapter.StartTime
		// 防止影片太短導致 fade 時間為負
		if videoDuration < fadeDuration*3 {
			fadeDuration = videoDuration / 4
		}

		// 組合濾鏡：先 format=yuv420p 確保 iPhone HEVC 10-bit 相容，再縮放 + 淡入淡出
		videoFilter := fmt.Sprintf(
			"format=yuv420p,"+
				"scale=%d:%d:force_original_aspect_ratio=decrease,"+
				"pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,"+
				"fade=t=in:st=0:d=%.2f,fade=t=out:st=%.2f:d=%.2f",
			targetWidth, targetHeight,
			targetWidth, targetHeight,
			fadeDuration, videoDuration-fadeDuration, fadeDuration)

		log.Printf("🎨 Chapter %d filter: %s", chapter.Index, videoFilter)

		segArgs := vaapiGlobalArgs()
		segArgs = append(segArgs,
			"-i", videoPath,
			"-ss", fmt.Sprintf("%.2f", chapter.StartTime),
			"-to", fmt.Sprintf("%.2f", chapter.EndTime),
			"-vf", vaapiVF(videoFilter),
			"-an",
		)
		segArgs = append(segArgs, h264Args("fast")...)
		segArgs = append(segArgs, pixFmtArgs()...)
		segArgs = append(segArgs, "-y", segmentPath)
		cmd := exec.Command("ffmpeg", segArgs...)

		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("❌ Failed to create segment %d: %v, output: %s", chapter.Index, err, string(output))
			continue
		}

		log.Printf("✅ Chapter %d segment created: %s", chapter.Index, segmentPath)
		processedSegments = append(processedSegments, segmentPath)

		// 細分進度：主片片段製作佔 70→78%，避免最長步驟卡住不動
		if total := len(project.Story.Chapters); total > 0 {
			setProjectProgress(project, 70+int(8.0*float64(i+1)/float64(total)))
		}

		// 如果有 TTS 音訊，記錄下來
		if chapter.AudioPath != "" {
			audioInputs = append(audioInputs, chapter.AudioPath)
		}
	}

	if len(processedSegments) == 0 {
		return fmt.Errorf("❌ no segments created")
	}

	log.Printf("📦 Total %d segments created, preparing to concatenate", len(processedSegments))

	// 合併所有影片片段
	concatListPath := filepath.Join(outputDir, "concat_segments.txt")
	f, err := os.Create(concatListPath)
	if err != nil {
		return err
	}
	defer f.Close()

	for i, seg := range processedSegments {
		fmt.Fprintf(f, "file '%s'\n", filepath.Base(seg))
		log.Printf("  %d. %s", i+1, filepath.Base(seg))
	}
	f.Close()

	log.Printf("🔗 Concatenating %d video segments...", len(processedSegments))

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
		return fmt.Errorf("❌ ffmpeg concat error: %v, output: %s", err, string(output))
	}

	log.Printf("✅ Video segments concatenated: %s", videoOnlyPath)

	// 合併所有 TTS 音訊
	if len(audioInputs) > 0 {
		log.Printf("🎤 Merging %d TTS audio files", len(audioInputs))

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
// createEndingSegment 產生獨立的結尾片段（720p、無音軌），內容為結尾圖片 +
// 狗狗回應文字。用相同編碼參數產生，之後可用 concat -c copy 串到主片後面。
func createEndingSegment(project *Project, outputPath string) error {
	if project.EndingImage == "" {
		return fmt.Errorf("no ending image")
	}
	endingDuration := 8.0 // 結尾 8 秒

	dogText := strings.TrimSpace(strings.ReplaceAll(project.Story.DogResponse, "\n\n", "\n"))

	width := outputWidth
	height := outputHeight
	fontFile := getFontPath()
	fontSize := 36
	lineHeight := fontSize + 10

	// 將文字分行（每行最多 20 字），每行用獨立 drawtext，避免換行亂碼
	rawLines := strings.Split(dogText, "\n")
	var textLines []string
	for _, part := range rawLines {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		runes := []rune(part)
		const maxCharsPerLine = 20
		for start := 0; start < len(runes); start += maxCharsPerLine {
			end := start + maxCharsPerLine
			if end > len(runes) {
				end = len(runes)
			}
			textLines = append(textLines, string(runes[start:end]))
		}
	}
	if len(textLines) > 6 {
		textLines = textLines[:6]
	}

	startY := height/2 + 20
	if len(textLines) > 0 {
		startY = height - len(textLines)*lineHeight - 60
		if startY < height/2 {
			startY = height / 2
		}
	}

	baseVF := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,"+
			"pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black",
		width, height, width, height,
	)

	var dtParts []string
	for i, line := range textLines {
		y := startY + i*lineHeight
		dtParts = append(dtParts, fmt.Sprintf(
			"drawtext=fontfile='%s':text='%s':fontsize=%d:fontcolor=white:"+
				"x=(w-text_w)/2:y=%d:box=1:boxcolor=black@0.6:boxborderw=8",
			fontFile, escapeFFmpegText(line), fontSize, y,
		))
	}

	fadePart := fmt.Sprintf("fade=t=in:st=0:d=0.5,fade=t=out:st=%.1f:d=0.5,format=yuv420p", endingDuration-0.5)
	endingVF := baseVF
	if len(dtParts) > 0 {
		endingVF += "," + strings.Join(dtParts, ",")
	}
	endingVF += "," + fadePart

	args := vaapiGlobalArgs()
	args = append(args,
		"-loop", "1",
		"-i", project.EndingImage,
		"-vf", vaapiVF(endingVF),
		"-t", fmt.Sprintf("%.2f", endingDuration),
		"-an",
	)
	args = append(args, h264Args("")...)
	args = append(args, pixFmtArgs()...)
	args = append(args, "-y", outputPath)

	if output, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("create ending segment error: %v, output: %s", err, string(output))
	}
	if stat, err := os.Stat(outputPath); err != nil || stat.Size() == 0 {
		return fmt.Errorf("ending segment not created properly")
	}
	log.Printf("✅ Created ending segment (%.0fs, %dx%d)", endingDuration, width, height)
	return nil
}

func addEndingImage(project *Project, inputVideo, outputVideo string) error {
	log.Printf("Adding ending image with dog response (concat approach)")

	outputDir := filepath.Dir(inputVideo)
	endingDuration := 15.0 // 結尾 15 秒

	// 準備狗狗回應文字
	dogText := project.Story.DogResponse
	dogText = strings.ReplaceAll(dogText, "\n\n", "\n")
	dogText = strings.TrimSpace(dogText)

	// 獲取輸入影片時長和原始解析度
	inputDuration := getVideoDuration(inputVideo)
	originalWidth, originalHeight := getVideoResolution(inputVideo)
	if inputDuration == 0 || originalWidth == 0 || originalHeight == 0 {
		log.Printf("⚠️ Warning: Could not get input video info (duration: %.2f, size: %dx%d), copying input as-is", inputDuration, originalWidth, originalHeight)
		return exec.Command("cp", inputVideo, outputVideo).Run()
	}
	log.Printf("📹 Input video info: duration=%.2fs, original size=%dx%d", inputDuration, originalWidth, originalHeight)

	// 統一使用 16:9 比例 (1920x1080)
	width := 1920
	height := 1080
	log.Printf("🎬 Target video size: %dx%d (16:9)", width, height)

	// 創建結尾圖片影片（10秒）
	endingVideoPath := filepath.Join(outputDir, "ending_segment.mp4")

	// 選擇字體 (macOS 使用 STHeiti 或 PingFang，Linux 使用 Noto Sans CJK)
	fontFile := getFontPath()
	log.Printf("🔤 Using font: %s", fontFile)

	fontSize := 52
	lineHeight := fontSize + 12
	log.Printf("📏 Font size: %d, line height: %d", fontSize, lineHeight)

	// 將文字分行（每行最多 20 字），每行用獨立的 drawtext filter 避免亂碼問題
	rawLines := strings.Split(dogText, "\n")
	var textLines []string
	for _, part := range rawLines {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		runes := []rune(part)
		const maxCharsPerLine = 20
		for start := 0; start < len(runes); start += maxCharsPerLine {
			end := start + maxCharsPerLine
			if end > len(runes) {
				end = len(runes)
			}
			textLines = append(textLines, string(runes[start:end]))
		}
	}
	if len(textLines) == 0 {
		textLines = []string{""}
	}
	// 限制最多 6 行，避免超出畫面
	if len(textLines) > 6 {
		textLines = textLines[:6]
	}

	totalTextHeight := len(textLines) * lineHeight
	// 文字區塊起始 Y：置於畫面下方 1/3 處，並加 margin
	startY := height - totalTextHeight - 80
	if startY < height/2 {
		startY = height / 2
	}

	// 基礎濾鏡：縮放 + padding
	baseVF := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,"+
			"pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black",
		width, height, width, height,
	)

	// 為每一行建立獨立的 drawtext 濾鏡，避免 \n 亂碼問題
	var dtParts []string
	for i, line := range textLines {
		y := startY + i*lineHeight
		dt := fmt.Sprintf(
			"drawtext=fontfile='%s':text='%s':fontsize=%d:fontcolor=white:"+
				"x=(w-text_w)/2:y=%d:"+
				"box=1:boxcolor=black@0.6:boxborderw=8",
			fontFile,
			escapeFFmpegText(line),
			fontSize,
			y,
		)
		dtParts = append(dtParts, dt)
	}

	fadePart := fmt.Sprintf("fade=t=in:st=0:d=0.5,fade=t=out:st=%.1f:d=0.5,format=yuv420p", endingDuration-0.5)
	endingVF := baseVF + "," + strings.Join(dtParts, ",") + "," + fadePart
	endingArgs := vaapiGlobalArgs()
	endingArgs = append(endingArgs,
		"-loop", "1",
		"-i", project.EndingImage,
		"-f", "lavfi",
		"-i", "anullsrc=r=44100:cl=stereo",
		"-vf", vaapiVF(endingVF),
		"-t", fmt.Sprintf("%.2f", endingDuration),
		"-c:a", "aac",
		"-shortest",
	)
	endingArgs = append(endingArgs, h264Args("")...)
	endingArgs = append(endingArgs, pixFmtArgs()...)
	endingArgs = append(endingArgs, "-y", endingVideoPath)
	endingCmd := exec.Command("ffmpeg", endingArgs...)

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
	concatFC := "[0:v][0:a][1:v][1:a]concat=n=2:v=1:a=1[outv][outa]"
	concatVMap := "[outv]"
	if useVAAPI {
		concatFC += ";[outv]format=nv12,hwupload=extra_hw_frames=64,format=vaapi[vout]"
		concatVMap = "[vout]"
	}
	concatArgs := vaapiGlobalArgs()
	concatArgs = append(concatArgs,
		"-i", inputVideo,
		"-i", endingVideoPath,
		"-filter_complex", concatFC,
		"-map", concatVMap,
		"-map", "[outa]",
		"-c:a", "aac",
	)
	concatArgs = append(concatArgs, h264Args("fast")...)
	concatArgs = append(concatArgs, "-y", outputVideo)
	concatCmd := exec.Command("ffmpeg", concatArgs...)

	concatOutput, err := concatCmd.CombinedOutput()
	if err != nil {
		log.Printf("Concat failed: %v, output: %s", err, string(concatOutput))

		// 嘗試不帶音訊的 concat (如果輸入影片沒有音訊)
		log.Printf("Trying concat without audio...")
		naFC := "[0:v][1:v]concat=n=2:v=1:a=0[outv]"
		naVMap := "[outv]"
		if useVAAPI {
			naFC += ";[outv]format=nv12,hwupload=extra_hw_frames=64,format=vaapi[vout]"
			naVMap = "[vout]"
		}
		naArgs := vaapiGlobalArgs()
		naArgs = append(naArgs, "-i", inputVideo, "-i", endingVideoPath, "-filter_complex", naFC, "-map", naVMap)
		naArgs = append(naArgs, h264Args("fast")...)
		naArgs = append(naArgs, "-y", outputVideo)
		concatCmdNoAudio := exec.Command("ffmpeg", naArgs...)
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
		cutArgs := vaapiGlobalArgs()
		cutArgs = append(cutArgs,
			"-i", videoPath,
			"-ss", fmt.Sprintf("%.2f", chapter.StartTime),
			"-to", fmt.Sprintf("%.2f", chapter.EndTime),
			"-c:a", "aac",
		)
		if useVAAPI {
			cutArgs = append(cutArgs, "-vf", "format=nv12,hwupload=extra_hw_frames=64,format=vaapi")
		}
		cutArgs = append(cutArgs, h264Args("")...)
		cutArgs = append(cutArgs, "-y", segmentPath)
		cmd := exec.Command("ffmpeg", cutArgs...)

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

			speedArgs := vaapiGlobalArgs()
			speedArgs = append(speedArgs,
				"-i", videoPath,
				"-ss", fmt.Sprintf("%.2f", chapter.StartTime),
				"-t", fmt.Sprintf("%.2f", segmentDuration),
				"-filter:v", vaapiVF(filterComplex),
				"-an",
			)
			speedArgs = append(speedArgs, h264Args("fast")...)
			speedArgs = append(speedArgs, "-y", segmentPath+"_video.mp4")
			cmd := exec.Command("ffmpeg", speedArgs...)

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

			fadeArgs := vaapiGlobalArgs()
			fadeArgs = append(fadeArgs,
				"-i", videoPath,
				"-ss", fmt.Sprintf("%.2f", chapter.StartTime),
				"-t", fmt.Sprintf("%.2f", segmentDuration),
				"-vf", vaapiVF(fadeFilter),
				"-an",
			)
			fadeArgs = append(fadeArgs, h264Args("fast")...)
			fadeArgs = append(fadeArgs, "-y", segmentPath)
			cmd := exec.Command("ffmpeg", fadeArgs...)

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

// setProjectProgress 安全地更新專案進度（單調遞增，不倒退）。
func setProjectProgress(project *Project, p int) {
	projectsMutex.Lock()
	if p > project.Progress {
		project.Progress = p
	}
	project.UpdatedAt = time.Now()
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

// wrapTextForASS 將長文字按最大字數換行，使用 ASS 的 \N 硬換行符
func wrapTextForASS(text string, maxChars int) string {
	if maxChars <= 0 {
		return text
	}
	runes := []rune(text)
	var result strings.Builder
	lineLen := 0
	for i, r := range runes {
		if lineLen >= maxChars && i > 0 {
			result.WriteString(`\N`)
			lineLen = 0
		}
		result.WriteRune(r)
		lineLen++
	}
	return result.String()
}

// wrapTextForFFmpeg 將長文字按最大字數換行，避免在 drawtext 中被左右切掉
// maxChars 是「每行最多的字數」（以 rune 計算，適合中英文摻雜的情況）
func wrapTextForFFmpeg(text string, maxChars int) string {
	if maxChars <= 0 {
		return text
	}

	// 先以原本的換行切開，每一段再做一次包裝
	parts := strings.Split(text, "\n")
	wrappedParts := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			wrappedParts = append(wrappedParts, "")
			continue
		}

		runes := []rune(p)
		lineStart := 0
		for lineStart < len(runes) {
			end := lineStart + maxChars
			if end > len(runes) {
				end = len(runes)
			}
			wrappedParts = append(wrappedParts, string(runes[lineStart:end]))
			lineStart = end
		}
	}

	// 使用真正的換行字元，讓 FFmpeg drawtext 正確換行
	return strings.Join(wrappedParts, "\n")
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
		"-q:v", "3", // 品質 3（1-31，數字越小品質越高）
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

	outputDir := filepath.Dir(inputVideo)
	assPath := filepath.Join(outputDir, "subtitles.ass")

	// 生成 ASS 格式字幕（支援淡入淡出效果）
	var sb strings.Builder
	sb.WriteString("[Script Info]\nScriptType: v4.00+\nPlayResX: 1920\nPlayResY: 1080\n\n")
	sb.WriteString("[V4+ Styles]\n")
	sb.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	sb.WriteString("Style: Default,Arial,80,&H00FFFFFF,&H000000FF,&H00000000,&H80000000,0,0,0,0,100,100,0,0,1,3,1,2,10,10,60,1\n\n")
	sb.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")

	currentTime := 0.0
	for _, chapter := range project.Story.Chapters {
		startTime := currentTime
		endTime := currentTime + chapter.Duration
		// 每行最多 18 個字，避免在螢幕邊緣被截掉
		wrapped := wrapTextForASS(chapter.Narration, 18)
		sb.WriteString(fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0,0,0,,{\\fad(500,500)}%s\n",
			formatASSTime(startTime), formatASSTime(endTime), wrapped))
		currentTime = endTime
	}

	if err := os.WriteFile(assPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to create subtitle file: %v", err)
	}
	defer os.Remove(assPath)

	fontFamily, fontsDir := getFontInfo()
	subtitleStyle := "FontSize=80,PrimaryColour=&H00FFFFFF,OutlineColour=&H00000000,BorderStyle=1,Outline=3,Shadow=1,MarginV=60"

	subArgs := vaapiGlobalArgs()
	subArgs = append(subArgs,
		"-i", inputVideo,
		// FontName 必須是「家族名稱」+ fontsdir，否則 libass 每幀重掃字型 → 慢數分鐘
		"-vf", vaapiVF(fmt.Sprintf("subtitles='%s':fontsdir='%s':force_style='%s,FontName=%s'", assPath, fontsDir, subtitleStyle, fontFamily)),
		"-an", // 主片無音軌；最後一步才加背景音樂
	)
	subArgs = append(subArgs, h264Args("fast")...)
	subArgs = append(subArgs, "-y", outputVideo)
	cmd := exec.Command("ffmpeg", subArgs...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg subtitle error: %v, output: %s", err, string(output))
	}

	log.Printf("✅ Added subtitles for project %s", project.ID)
	return nil
}

func formatASSTime(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	cs := int((seconds - float64(int(seconds))) * 100)
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}

// createTitleCard 建立片頭字卡（用 subtitles filter，與字幕同一套字型機制）
func createTitleCard(project *Project, outputPath string) error {
	const duration = 3.0
	outputDir := filepath.Dir(outputPath)

	// 用 ASS + alignment=5（置中）產生片頭標題，\fad 淡入淡出
	assPath := filepath.Join(outputDir, "title_card.ass")
	titleText := project.DogName + " 的回憶錄"
	assContent := fmt.Sprintf(
		"[Script Info]\nScriptType: v4.00+\nPlayResX: 1920\nPlayResY: 1080\n\n"+
			"[V4+ Styles]\n"+
			"Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n"+
			"Style: Default,Arial,72,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,3,0,1,2,1,5,10,10,0,1\n\n"+
			"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"+
			"Dialogue: 0,0:00:00.00,0:00:03.00,Default,,0,0,0,,{\\fad(800,800)}%s\n",
		titleText,
	)
	if err := os.WriteFile(assPath, []byte(assContent), 0644); err != nil {
		return fmt.Errorf("failed to write title ass: %v", err)
	}
	defer os.Remove(assPath)

	fontFamily, fontsDir := getFontInfo()
	titleArgs := vaapiGlobalArgs()
	titleArgs = append(titleArgs,
		"-f", "lavfi", "-i", fmt.Sprintf("color=c=black:size=%dx%d:rate=%s:duration=%.1f", outputWidth, outputHeight, outputFPS, duration),
		// FontName 用家族名稱 + fontsdir（避免 libass 逐幀掃字型而變慢）
		"-vf", vaapiVF(fmt.Sprintf("subtitles='%s':fontsdir='%s':force_style='FontName=%s'", assPath, fontsDir, fontFamily)),
		"-an", // 無音軌，與其他片段一致以便 -c copy 串接
		"-t", fmt.Sprintf("%.1f", duration),
	)
	titleArgs = append(titleArgs, h264ArgsWithCRF("fast", "23")...)
	titleArgs = append(titleArgs, "-y", outputPath)
	cmd := exec.Command("ffmpeg", titleArgs...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create title card: %v, output: %s", err, string(output))
	}
	log.Printf("✅ Created title card: %s", titleText)
	return nil
}

// prependTitleCard 將片頭字卡接在主影片前面
func prependTitleCard(titlePath, mainPath, outputPath string) error {
	prependFC := "[0:v][0:a][1:v][1:a]concat=n=2:v=1:a=1[v][a]"
	prependVMap := "[v]"
	if useVAAPI {
		prependFC += ";[v]format=nv12,hwupload=extra_hw_frames=64,format=vaapi[vout]"
		prependVMap = "[vout]"
	}
	prependArgs := vaapiGlobalArgs()
	prependArgs = append(prependArgs,
		"-i", titlePath,
		"-i", mainPath,
		"-filter_complex", prependFC,
		"-map", prependVMap, "-map", "[a]",
		"-c:a", "aac", "-b:a", "128k",
	)
	prependArgs = append(prependArgs, h264ArgsWithCRF("fast", "23")...)
	prependArgs = append(prependArgs, "-y", outputPath)
	cmd := exec.Command("ffmpeg", prependArgs...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to prepend title card: %v, output: %s", err, string(output))
	}
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
	bgmDir := "./assets_bgm"

	// 根據故事模式選擇背景音樂
	bgmFilename := "bibi-pianopachelbels-canon-终于弹了这首-世界上最治愈的钢琴曲卡农.mp3" // Default (warm)
	if project.StoryMode == "cute" {
		bgmFilename = "lively.mp3"
	}

	specificBGM := filepath.Join(bgmDir, bgmFilename)
	musicCopied := false

	log.Printf("🎵 Checking for specific BGM file: %s", specificBGM)
	if _, err := os.Stat(specificBGM); err == nil {
		log.Printf("✅ Found specific BGM file: %s", specificBGM)
		if err := copyFile(specificBGM, musicPath); err == nil {
			musicCopied = true
		}
	} else {
		log.Printf("❌ Specific BGM not found, searching in %s...", bgmDir)
		// 如果精確匹配失敗，尋找目錄下的第一個 mp3
		files, _ := filepath.Glob(filepath.Join(bgmDir, "*.mp3"))
		if len(files) > 0 {
			log.Printf("✅ Found alternative BGM: %s", files[0])
			if err := copyFile(files[0], musicPath); err == nil {
				musicCopied = true
			}
		}
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

	// 主片已無音軌，背景音樂為唯一音軌，並在最後 3 秒淡出。
	// 影片直接 -c:v copy（不重新編碼），只編碼音訊，速度極快。
	videoDuration := getVideoDuration(inputVideo)
	fadeStartTime := videoDuration - 3.0
	if fadeStartTime < 0 {
		fadeStartTime = 0
	}

	filterComplex := fmt.Sprintf("[1:a]volume=1.0,afade=t=out:st=%.2f:d=3[aout]", fadeStartTime)
	log.Printf("Audio filter: %s (video duration: %.2fs, fade start: %.2fs)", filterComplex, videoDuration, fadeStartTime)

	cmd := exec.Command("ffmpeg",
		"-i", inputVideo,
		"-i", musicPath,
		"-filter_complex", filterComplex,
		"-map", "0:v",
		"-map", "[aout]",
		"-c:v", "copy",
		"-c:a", "aac",
		"-shortest",
		"-movflags", "+faststart",
		"-y",
		outputVideo,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg music add error: %v, output: %s", err, string(output))
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

func getFontPath() string {
	// macOS
	macFonts := []string{
		"/System/Library/Fonts/STHeiti Medium.ttc",
		"/System/Library/Fonts/PingFang.ttc",
		"/Library/Fonts/Arial Unicode.ttf",
	}

	// Linux (Alpine Docker: font-noto-cjk 安裝路徑)
	linuxFonts := []string{
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/TTF/NotoSansCJK-Regular.ttc",
	}

	for _, f := range macFonts {
		if _, err := os.Stat(f); err == nil {
			return f
		}
	}

	for _, f := range linuxFonts {
		if _, err := os.Stat(f); err == nil {
			return f
		}
	}

	return "Arial" // 最後手段
}

// getFontInfo 回傳給 libass subtitles filter 用的「字型家族名稱」與「字型目錄」。
// 重要：subtitles 的 force_style=FontName 必須是「家族名稱」而非「檔案路徑」，
// 否則 libass 會對每一幀重新掃描整個系統字型，導致燒字幕慢到數分鐘（吐血主因）。
// 搭配 fontsdir 指定目錄可讓 fontconfig 快速命中。
func getFontInfo() (family string, fontsDir string) {
	path := getFontPath()
	dir := filepath.Dir(path)
	switch {
	case strings.Contains(path, "STHeiti"):
		return "Heiti TC", dir
	case strings.Contains(path, "PingFang"):
		return "PingFang TC", dir
	case strings.Contains(path, "Arial Unicode"):
		return "Arial Unicode MS", dir
	case strings.Contains(path, "NotoSansCJK"):
		return "Noto Sans CJK TC", dir
	default:
		// 退而求其次：用檔名（去副檔名）當家族名，仍提供 fontsdir
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if base == "" || base == "Arial" {
			return "sans-serif", dir
		}
		return base, dir
	}
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
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
