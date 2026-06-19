package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// db 是全域 SQLite 連線（純 Go driver，配合 CGO_ENABLED=0）。
// 資料庫檔放在 storage/ 之下，走現有 docker volume，重新部署不會消失。
var db *sql.DB

const dbSchema = `
CREATE TABLE IF NOT EXISTS tasks (
	id                 TEXT PRIMARY KEY,
	name               TEXT,
	dog_name           TEXT,
	dog_breed          TEXT,
	owner_relationship TEXT,
	story_mode         TEXT,
	owner_message      TEXT,
	status             TEXT,
	error              TEXT,
	video_count        INTEGER,
	story_title        TEXT,
	dog_response       TEXT,
	vision_model       TEXT,
	text_model         TEXT,
	final_video        TEXT,
	ending_image       TEXT,
	analysis_ms        INTEGER,
	story_ms           INTEGER,
	composite_ms       INTEGER,
	total_ms           INTEGER,
	created_at         TEXT,
	saved_at           TEXT
);
CREATE TABLE IF NOT EXISTS task_videos (
	id                INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id           TEXT,
	video_id          TEXT,
	video_index       INTEGER,
	playback_position INTEGER,
	original_name     TEXT,
	size_bytes        INTEGER,
	duration          REAL,
	has_dog           INTEGER,
	has_human         INTEGER,
	interaction_type  TEXT,
	emotion           TEXT,
	short_caption     TEXT,
	clip_start        REAL,
	clip_end          REAL,
	clip_duration     REAL,
	narration         TEXT
);
CREATE INDEX IF NOT EXISTS idx_task_videos_task ON task_videos(task_id);
CREATE INDEX IF NOT EXISTS idx_tasks_created ON tasks(created_at);

CREATE TABLE IF NOT EXISTS users (
	id            TEXT PRIMARY KEY,
	username      TEXT UNIQUE,
	password_hash TEXT,
	created_at    TEXT
);
CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	user_id    TEXT,
	created_at TEXT
);
`

// initDB 開啟（或建立）SQLite 並建表。失敗只記錄不中斷服務（DB 是輔助功能）。
func initDB(storageDir string) {
	dbPath := filepath.Join(storageDir, "paw_diary.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Printf("⚠️ DB 開啟失敗（後台記錄將停用）: %v", err)
		return
	}
	// SQLite 單寫者：限制 1 條連線避免 "database is locked"
	conn.SetMaxOpenConns(1)
	if _, err := conn.Exec(dbSchema); err != nil {
		log.Printf("⚠️ DB 建表失敗: %v", err)
		return
	}
	// 為既有 tasks 表補上 user_id 欄位（可空 = 匿名）；已存在會報 duplicate column，忽略即可。
	if _, err := conn.Exec(`ALTER TABLE tasks ADD COLUMN user_id TEXT`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			log.Printf("note: add tasks.user_id: %v", err)
		}
	}
	conn.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_user ON tasks(user_id)`)
	db = conn
	log.Printf("✅ DB ready: %s", dbPath)
}

type taskTiming struct {
	AnalysisMs  int64
	StoryMs     int64
	CompositeMs int64
	TotalMs     int64
}

// saveTaskRecord 把一次任務的完整輸入到輸出寫入 DB（成功或失敗都呼叫）。
// 必須在清理原始影片檔之前呼叫，才能讀到檔案大小。
func saveTaskRecord(p *Project, t taskTiming) {
	if db == nil || p == nil {
		return
	}

	storyTitle, dogResponse := "", ""
	chapterByVideoID := map[string]*StoryChapter{}
	if p.Story != nil {
		storyTitle = p.Story.Title
		dogResponse = p.Story.DogResponse
		for i := range p.Story.Chapters {
			ch := &p.Story.Chapters[i]
			chapterByVideoID[ch.VideoID] = ch
		}
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("⚠️ saveTaskRecord begin: %v", err)
		return
	}
	defer tx.Rollback()

	now := time.Now().Format(time.RFC3339)
	_, err = tx.Exec(`
		INSERT INTO tasks (id,name,dog_name,dog_breed,owner_relationship,story_mode,owner_message,
			status,error,video_count,story_title,dog_response,vision_model,text_model,
			final_video,ending_image,analysis_ms,story_ms,composite_ms,total_ms,created_at,saved_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status, error=excluded.error, video_count=excluded.video_count,
			story_title=excluded.story_title, dog_response=excluded.dog_response,
			final_video=excluded.final_video, ending_image=excluded.ending_image,
			analysis_ms=excluded.analysis_ms, story_ms=excluded.story_ms,
			composite_ms=excluded.composite_ms, total_ms=excluded.total_ms, saved_at=excluded.saved_at`,
		p.ID, p.Name, p.DogName, p.DogBreed, p.OwnerRelationship, p.StoryMode, p.OwnerMessage,
		p.Status, p.Error, len(p.Videos), storyTitle, dogResponse, aiVisionModel, aiTextModel,
		p.FinalVideo, p.EndingImage, t.AnalysisMs, t.StoryMs, t.CompositeMs, t.TotalMs,
		p.CreatedAt.Format(time.RFC3339), now,
	)
	if err != nil {
		log.Printf("⚠️ saveTaskRecord insert task: %v", err)
		return
	}

	// 重寫該任務的影片明細
	if _, err := tx.Exec(`DELETE FROM task_videos WHERE task_id=?`, p.ID); err != nil {
		log.Printf("⚠️ saveTaskRecord clear videos: %v", err)
		return
	}
	for idx, v := range p.Videos {
		var hasDog, hasHuman bool
		var interaction, emotion, caption string
		if v.Analysis != nil {
			hasDog = v.Analysis.HasDog
			hasHuman = v.Analysis.HasHuman
			interaction = v.Analysis.InteractionType
			emotion = v.Analysis.Emotion
			caption = v.Analysis.ShortCaption
		}
		var size int64
		if st, err := os.Stat(v.Path); err == nil {
			size = st.Size()
		}
		playbackPos, clipStart, clipEnd, clipDur, narration := -1, 0.0, 0.0, 0.0, ""
		if ch, ok := chapterByVideoID[v.ID]; ok {
			playbackPos = ch.Index
			clipStart, clipEnd, clipDur, narration = ch.StartTime, ch.EndTime, ch.Duration, ch.Narration
		}
		_, err := tx.Exec(`
			INSERT INTO task_videos (task_id,video_id,video_index,playback_position,original_name,
				size_bytes,duration,has_dog,has_human,interaction_type,emotion,short_caption,
				clip_start,clip_end,clip_duration,narration)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			p.ID, v.ID, idx, playbackPos, v.OriginalName, size, v.Duration,
			boolToInt(hasDog), boolToInt(hasHuman), interaction, emotion, caption,
			clipStart, clipEnd, clipDur, narration,
		)
		if err != nil {
			log.Printf("⚠️ saveTaskRecord insert video: %v", err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("⚠️ saveTaskRecord commit: %v", err)
		return
	}
	log.Printf("📝 Task %s saved to DB (status=%s, videos=%d)", p.ID, p.Status, len(p.Videos))
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
