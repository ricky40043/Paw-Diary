package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// 使用者帳號：可選功能。匿名照舊用前端 localStorage；登入後影片綁帳號、可跨裝置看。

func genToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func registerAccountRoutes(router *gin.Engine) {
	g := router.Group("/api/account")
	g.POST("/register", accountRegister)
	g.POST("/login", accountLogin)

	auth := router.Group("/api/account", userAuthRequired())
	auth.GET("/me", accountMe)
	auth.POST("/claim", accountClaim)
	auth.GET("/videos", accountVideos)
	auth.POST("/logout", accountLogout)
}

// userAuthRequired 驗證使用者 token（Authorization: Bearer 或 X-Auth-Token），
// 解出 user_id 與 username 放進 context。
func userAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
			return
		}
		tok := c.GetHeader("X-Auth-Token")
		if tok == "" {
			if a := c.GetHeader("Authorization"); strings.HasPrefix(a, "Bearer ") {
				tok = strings.TrimPrefix(a, "Bearer ")
			}
		}
		if tok == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not logged in"})
			return
		}
		var userID, username string
		err := db.QueryRow(`
			SELECT u.id, u.username FROM sessions s JOIN users u ON u.id=s.user_id
			WHERE s.token=?`, tok).Scan(&userID, &username)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session invalid"})
			return
		}
		c.Set("userID", userID)
		c.Set("username", username)
		c.Next()
	}
}

func normalizeUsername(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// claimItem：前端本機紀錄的一支影片（含名稱與時間，讓 DB 沒有紀錄的舊影片也能補建）
type claimItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	TS   int64  `json:"ts"` // epoch 毫秒（本機紀錄的完成時間）
}

type authBody struct {
	Username   string      `json:"username"`
	Password   string      `json:"password"`
	ClaimItems []claimItem `json:"claim_items"` // 登入/註冊時順便認領的本機影片
}

// POST /api/account/register
func accountRegister(c *gin.Context) {
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}
	var b authBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	username := normalizeUsername(b.Username)
	if len(username) < 2 || len(username) > 30 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "帳號長度需 2–30 字"})
		return
	}
	if len(b.Password) < 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密碼至少 4 碼"})
		return
	}
	// 帳號重複檢查
	var exists int
	db.QueryRow(`SELECT COUNT(*) FROM users WHERE username=?`, username).Scan(&exists)
	if exists > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "這個帳號已經有人用了"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(b.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	userID := uuid.New().String()
	if _, err := db.Exec(`INSERT INTO users (id,username,password_hash,created_at) VALUES (?,?,?,?)`,
		userID, username, string(hash), time.Now().Format(time.RFC3339)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "建立帳號失敗"})
		return
	}
	token := newSession(userID)
	claimed := claimTasksFull(userID, b.ClaimItems)
	c.JSON(http.StatusOK, gin.H{"token": token, "username": username, "claimed": claimed})
}

// POST /api/account/login
func accountLogin(c *gin.Context) {
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}
	var b authBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	username := normalizeUsername(b.Username)
	var userID, hash string
	err := db.QueryRow(`SELECT id, password_hash FROM users WHERE username=?`, username).Scan(&userID, &hash)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "帳號或密碼錯誤"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(b.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "帳號或密碼錯誤"})
		return
	}
	token := newSession(userID)
	claimed := claimTasksFull(userID, b.ClaimItems)
	c.JSON(http.StatusOK, gin.H{"token": token, "username": username, "claimed": claimed})
}

func newSession(userID string) string {
	token := genToken()
	db.Exec(`INSERT INTO sessions (token,user_id,created_at) VALUES (?,?,?)`,
		token, userID, time.Now().Format(time.RFC3339))
	return token
}

// claimTasksFull 把一批本機影片綁到此使用者，回傳成功認領數量。
// - tasks 已有該筆且沒主人 → 綁定
// - tasks 沒有該筆（例如 DB 功能上線前做的舊影片）→ 只要最終影片檔還在，就補建一筆完成紀錄
// - 已被別人擁有 → 略過
func claimTasksFull(userID string, items []claimItem) int {
	if len(items) == 0 {
		return 0
	}
	n := 0
	now := time.Now().Format(time.RFC3339)
	for _, it := range items {
		if it.ID == "" {
			continue
		}
		var owner sql.NullString
		err := db.QueryRow(`SELECT user_id FROM tasks WHERE id=?`, it.ID).Scan(&owner)
		if err == nil {
			// 已存在
			if !owner.Valid || owner.String == "" {
				if _, e := db.Exec(`UPDATE tasks SET user_id=? WHERE id=?`, userID, it.ID); e == nil {
					n++
				}
			}
			continue
		}
		// 不存在 → 檔案還在才補建（避免建出無效紀錄）
		finalPath := filepath.Join(storagePath, "projects", it.ID, "final.mp4")
		if _, e := os.Stat(finalPath); e != nil {
			continue
		}
		created := now
		if it.TS > 0 {
			created = time.UnixMilli(it.TS).Format(time.RFC3339)
		}
		name := it.Name
		if name == "" {
			name = "毛孩"
		}
		if _, e := db.Exec(`INSERT INTO tasks (id,dog_name,status,user_id,final_video,video_count,created_at,saved_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			it.ID, name, "completed", userID, finalPath, 0, created, now); e == nil {
			n++
		}
	}
	return n
}

// GET /api/account/me
func accountMe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"username": c.GetString("username")})
}

// POST /api/account/claim  { items: [{id,name,ts}, ...] }
func accountClaim(c *gin.Context) {
	var body struct {
		Items []claimItem `json:"items"`
	}
	c.ShouldBindJSON(&body)
	n := claimTasksFull(c.GetString("userID"), body.Items)
	c.JSON(http.StatusOK, gin.H{"claimed": n})
}

// GET /api/account/videos —— 此帳號做過、已完成的影片（跨裝置）
func accountVideos(c *gin.Context) {
	userID := c.GetString("userID")
	rows, err := db.Query(`
		SELECT id, dog_name, created_at FROM tasks
		WHERE user_id=? AND status='completed' ORDER BY created_at DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []gin.H{}
	for rows.Next() {
		var id, dogName, createdAt string
		rows.Scan(&id, &dogName, &createdAt)
		list = append(list, gin.H{
			"id":         id,
			"dog_name":   dogName,
			"created_at": createdAt,
			"url":        fmt.Sprintf("/storage/projects/%s/final.mp4", id),
		})
	}
	c.JSON(http.StatusOK, gin.H{"videos": list})
}

// POST /api/account/logout
func accountLogout(c *gin.Context) {
	if tok := c.GetHeader("X-Auth-Token"); tok != "" {
		db.Exec(`DELETE FROM sessions WHERE token=?`, tok)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
