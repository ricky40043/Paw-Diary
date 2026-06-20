package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// adminAuthRequired 用 ADMIN_TOKEN 保護後台 API。
// 未設定 ADMIN_TOKEN → 直接停用後台（回 503），避免不小心公開資料。
// Token 可放 Header(X-Admin-Token / Authorization: Bearer) 或 ?token= 查詢字串。
func adminAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		want := os.Getenv("ADMIN_TOKEN")
		if want == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "admin disabled: ADMIN_TOKEN not set"})
			return
		}
		got := c.GetHeader("X-Admin-Token")
		if got == "" {
			if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				got = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		if got == "" {
			got = c.Query("token")
		}
		if got != want {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

// registerAdminRoutes 掛上後台 API（全部受 token 保護）。
func registerAdminRoutes(router *gin.Engine) {
	g := router.Group("/api/admin", adminAuthRequired())
	g.GET("/verify", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	g.GET("/tasks", adminListTasks)
	g.GET("/tasks/:id", adminTaskDetail)
	g.DELETE("/tasks/:id", adminDeleteTask)
	g.GET("/stats", adminStats)
}

func adminDBReady(c *gin.Context) bool {
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return false
	}
	return true
}

// GET /api/admin/tasks?limit=&offset=
func adminListTasks(c *gin.Context) {
	if !adminDBReady(c) {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var total int
	db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&total)

	rows, err := db.Query(`
		SELECT t.id, t.dog_name, t.owner_relationship, t.story_mode, t.status, t.video_count,
		       t.total_ms, t.story_title, t.created_at, t.saved_at, COALESCE(u.username,'')
		FROM tasks t LEFT JOIN users u ON u.id=t.user_id
		ORDER BY t.created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := []gin.H{}
	for rows.Next() {
		var id, dogName, rel, mode, status, title, createdAt, savedAt, username string
		var videoCount int
		var totalMs int64
		rows.Scan(&id, &dogName, &rel, &mode, &status, &videoCount, &totalMs, &title, &createdAt, &savedAt, &username)
		list = append(list, gin.H{
			"id": id, "dog_name": dogName, "owner_relationship": rel, "story_mode": mode,
			"status": status, "video_count": videoCount, "total_ms": totalMs,
			"story_title": title, "created_at": createdAt, "saved_at": savedAt, "username": username,
		})
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "limit": limit, "offset": offset, "tasks": list})
}

// GET /api/admin/tasks/:id
func adminTaskDetail(c *gin.Context) {
	if !adminDBReady(c) {
		return
	}
	id := c.Param("id")

	var t gin.H
	{
		var dogName, breed, rel, mode, msg, status, errMsg, title, dogResp string
		var visionModel, textModel, finalVideo, endingImage, createdAt, savedAt, name string
		var userID, username, ip string
		var videoCount int
		var analysisMs, storyMs, compositeMs, totalMs int64
		err := db.QueryRow(`
			SELECT t.name,t.dog_name,t.dog_breed,t.owner_relationship,t.story_mode,t.owner_message,t.status,t.error,
			       t.video_count,t.story_title,t.dog_response,t.vision_model,t.text_model,t.final_video,t.ending_image,
			       t.analysis_ms,t.story_ms,t.composite_ms,t.total_ms,t.created_at,t.saved_at,
			       COALESCE(t.user_id,''), COALESCE(u.username,''), COALESCE(t.ip,'')
			FROM tasks t LEFT JOIN users u ON u.id=t.user_id WHERE t.id=?`, id).Scan(
			&name, &dogName, &breed, &rel, &mode, &msg, &status, &errMsg,
			&videoCount, &title, &dogResp, &visionModel, &textModel, &finalVideo, &endingImage,
			&analysisMs, &storyMs, &compositeMs, &totalMs, &createdAt, &savedAt,
			&userID, &username, &ip)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		t = gin.H{
			"id": id, "name": name, "dog_name": dogName, "dog_breed": breed,
			"owner_relationship": rel, "story_mode": mode, "owner_message": msg,
			"status": status, "error": errMsg, "video_count": videoCount,
			"story_title": title, "dog_response": dogResp,
			"vision_model": visionModel, "text_model": textModel,
			"final_video": finalVideo, "ending_image": endingImage,
			"analysis_ms": analysisMs, "story_ms": storyMs, "composite_ms": compositeMs, "total_ms": totalMs,
			"created_at": createdAt, "saved_at": savedAt,
			"username": username, "is_anonymous": userID == "", "ip": ip,
		}
	}

	rows, err := db.Query(`
		SELECT video_index,playback_position,original_name,size_bytes,duration,
		       has_dog,has_human,interaction_type,emotion,short_caption,
		       clip_start,clip_end,clip_duration,narration
		FROM task_videos WHERE task_id=? ORDER BY video_index`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	videos := []gin.H{}
	for rows.Next() {
		var vIdx, playbackPos, hasDog, hasHuman int
		var sizeBytes int64
		var duration, clipStart, clipEnd, clipDur float64
		var name, interaction, emotion, caption, narration string
		rows.Scan(&vIdx, &playbackPos, &name, &sizeBytes, &duration,
			&hasDog, &hasHuman, &interaction, &emotion, &caption,
			&clipStart, &clipEnd, &clipDur, &narration)
		videos = append(videos, gin.H{
			"video_index": vIdx, "playback_position": playbackPos, "original_name": name,
			"size_bytes": sizeBytes, "duration": duration,
			"has_dog": hasDog == 1, "has_human": hasHuman == 1,
			"interaction_type": interaction, "emotion": emotion, "short_caption": caption,
			"clip_start": clipStart, "clip_end": clipEnd, "clip_duration": clipDur, "narration": narration,
		})
	}

	c.JSON(http.StatusOK, gin.H{"task": t, "videos": videos})
}

// DELETE /api/admin/tasks/:id —— 刪除任務紀錄、影片明細，以及最終成品檔案
func adminDeleteTask(c *gin.Context) {
	if !adminDBReady(c) {
		return
	}
	id := c.Param("id")
	db.Exec(`DELETE FROM task_videos WHERE task_id=?`, id)
	res, err := db.Exec(`DELETE FROM tasks WHERE id=?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 連同最終影片資料夾一起刪（id 是 uuid，安全）
	if id != "" {
		os.RemoveAll(filepath.Join(storagePath, "projects", id))
	}
	n, _ := res.RowsAffected()
	c.JSON(http.StatusOK, gin.H{"deleted": n})
}

// GET /api/admin/stats —— 基本統計：總數、成功率、風格/互動分布、平均耗時與影片數
func adminStats(c *gin.Context) {
	if !adminDBReady(c) {
		return
	}
	stats := gin.H{}

	var total, completed, failed int
	db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&total)
	db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status='completed'`).Scan(&completed)
	db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status='failed'`).Scan(&failed)
	stats["total"] = total
	stats["completed"] = completed
	stats["failed"] = failed

	var avgTotalMs, avgVideos float64
	db.QueryRow(`SELECT COALESCE(AVG(total_ms),0) FROM tasks WHERE status='completed' AND total_ms>0`).Scan(&avgTotalMs)
	db.QueryRow(`SELECT COALESCE(AVG(video_count),0) FROM tasks`).Scan(&avgVideos)
	stats["avg_total_ms"] = avgTotalMs
	stats["avg_videos"] = avgVideos

	stats["by_mode"] = groupCount(`SELECT story_mode, COUNT(*) FROM tasks GROUP BY story_mode`)
	stats["by_interaction"] = groupCount(`SELECT interaction_type, COUNT(*) FROM task_videos GROUP BY interaction_type ORDER BY COUNT(*) DESC`)
	stats["by_emotion"] = groupCount(`SELECT emotion, COUNT(*) FROM task_videos GROUP BY emotion ORDER BY COUNT(*) DESC`)

	c.JSON(http.StatusOK, stats)
}

// groupCount 執行「label, count」型查詢，回成 [{label,count}]，方便前端畫長條圖。
func groupCount(query string) []gin.H {
	out := []gin.H{}
	rows, err := db.Query(query)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var label string
		var count int
		rows.Scan(&label, &count)
		if label == "" {
			label = "(空)"
		}
		out = append(out, gin.H{"label": label, "count": count})
	}
	return out
}
