package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
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

type authBody struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	ClaimIDs []string `json:"claim_ids"` // 登入/註冊時順便認領的本機匿名影片 id
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
	claimed := claimTasks(userID, b.ClaimIDs)
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
	claimed := claimTasks(userID, b.ClaimIDs)
	c.JSON(http.StatusOK, gin.H{"token": token, "username": username, "claimed": claimed})
}

func newSession(userID string) string {
	token := genToken()
	db.Exec(`INSERT INTO sessions (token,user_id,created_at) VALUES (?,?,?)`,
		token, userID, time.Now().Format(time.RFC3339))
	return token
}

// claimTasks 把一批「還沒有主人」的影片任務綁到此使用者，回傳成功認領的數量。
func claimTasks(userID string, ids []string) int {
	if len(ids) == 0 {
		return 0
	}
	n := 0
	for _, id := range ids {
		res, err := db.Exec(`UPDATE tasks SET user_id=? WHERE id=? AND (user_id IS NULL OR user_id='')`, userID, id)
		if err == nil {
			if c, _ := res.RowsAffected(); c > 0 {
				n++
			}
		}
	}
	return n
}

// GET /api/account/me
func accountMe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"username": c.GetString("username")})
}

// POST /api/account/claim  { ids: [...] }
func accountClaim(c *gin.Context) {
	var body struct {
		IDs []string `json:"ids"`
	}
	c.ShouldBindJSON(&body)
	n := claimTasks(c.GetString("userID"), body.IDs)
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
