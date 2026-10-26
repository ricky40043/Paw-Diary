// Shared password recovery v1, 2026-10-26. Copied into each repository.
package recovery

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

//go:embed password-recovery.html
var page string

type Recovery struct {
	DB     *sql.DB
	Prefix string
	Title  string
	Paw    bool
}

type member struct {
	ID, Email, Hash string
	Verified        int
}
type queryer interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

var rateMutex sync.Mutex

func Fingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (r *Recovery) find(q queryer, account string, byID bool) (member, error) {
	var u member
	query := "SELECT CAST(id AS TEXT),COALESCE(email,''),password_hash,1 FROM users WHERE lower(email)=? AND deleted_at IS NULL"
	args := []interface{}{strings.ToLower(strings.TrimSpace(account))}
	if r.Paw {
		query = "SELECT id,COALESCE(email,''),password_hash,email_verified FROM users WHERE username=? OR (lower(email)=? AND email_verified=1)"
		args = append(args, args[0])
	}
	if byID {
		query = "SELECT CAST(id AS TEXT),COALESCE(email,''),password_hash,1 FROM users WHERE id=? AND deleted_at IS NULL"
		if r.Paw {
			query = "SELECT id,COALESCE(email,''),password_hash,email_verified FROM users WHERE id=?"
		}
		args = []interface{}{account}
	}
	err := q.QueryRow(query, args...).Scan(&u.ID, &u.Email, &u.Hash, &u.Verified)
	return u, err
}

func (r *Recovery) Register(router *gin.Engine) error {
	_, err := r.DB.Exec(`CREATE TABLE IF NOT EXISTS recovery_tokens(token_hash TEXT PRIMARY KEY,user_id TEXT NOT NULL,password_snapshot TEXT NOT NULL,purpose TEXT NOT NULL,pending_email TEXT NOT NULL DEFAULT '',expires_at INTEGER NOT NULL,used INTEGER NOT NULL DEFAULT 0);
 CREATE TABLE IF NOT EXISTS recovery_password_changes(user_id TEXT PRIMARY KEY,changed_at INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS recovery_attempts(scope TEXT NOT NULL,created_at INTEGER NOT NULL);
 CREATE INDEX IF NOT EXISTS recovery_attempts_scope ON recovery_attempts(scope,created_at);`)
	if err != nil {
		return err
	}
	group := router.Group(r.Prefix)
	group.GET("/password-help", func(c *gin.Context) {
		content := strings.ReplaceAll(page, "__APP_NAME__", r.Title)
		encoded, _ := json.Marshal(r.Prefix)
		content = strings.ReplaceAll(content, "__API_PREFIX__", string(encoded))
		if r.Paw {
			content = strings.ReplaceAll(content, "type=\"email\" autocomplete=\"username\"", "type=\"text\" autocomplete=\"username\"")
			content = strings.ReplaceAll(content, "<label for=\"changeEmail\">電子郵件</label>", "<label for=\"changeEmail\">帳號或已驗證的信箱</label>")
			content = strings.ReplaceAll(content, "<!-- BIND_EMAIL -->", pawBindingForm)
		}
		c.Header("Cache-Control", "no-store")
		c.Header("Referrer-Policy", "no-referrer")
		c.Data(200, "text/html; charset=utf-8", []byte(content))
	})
	group.POST("/forgot-password", r.forgot)
	group.POST("/reset-password", r.reset)
	group.POST("/change-password", r.change)
	if r.Paw {
		group.POST("/recovery-email/request", r.bind)
		group.POST("/recovery-email/verify", r.verifyEmail)
	}
	return nil
}

func (r *Recovery) limit(c *gin.Context, account, purpose string) bool {
	rateMutex.Lock()
	defer rateMutex.Unlock()
	tx, err := r.DB.Begin()
	if err != nil {
		c.JSON(503, gin.H{"detail": "服務暫時忙碌"})
		return false
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	if _, err = tx.Exec("DELETE FROM recovery_attempts WHERE created_at<?", now-900); err != nil {
		c.JSON(503, gin.H{"detail": "服務暫時忙碌"})
		return false
	}
	scopes := []struct {
		key   string
		limit int
	}{{purpose + ":account:" + account, 5}, {purpose + ":ip:" + c.RemoteIP(), 30}}
	for _, s := range scopes {
		key := Fingerprint(s.key)
		var count int
		err = tx.QueryRow("SELECT COUNT(*) FROM recovery_attempts WHERE scope=? AND created_at>?", key, now-900).Scan(&count)
		if err != nil {
			c.JSON(503, gin.H{"detail": "服務暫時忙碌"})
			return false
		}
		if count >= s.limit {
			c.Header("Retry-After", "900")
			c.JSON(429, gin.H{"detail": "要求過於頻繁，請15分鐘後再試"})
			return false
		}
		if _, err = tx.Exec("INSERT INTO recovery_attempts VALUES (?,?)", key, now); err != nil {
			c.JSON(503, gin.H{"detail": "服務暫時忙碌"})
			return false
		}
	}
	if err = tx.Commit(); err != nil {
		c.JSON(503, gin.H{"detail": "服務暫時忙碌"})
		return false
	}
	return true
}

type input struct {
	Email           string `json:"email"`
	Username        string `json:"username"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	Token           string `json:"token"`
}

func read(c *gin.Context) (input, bool) {
	var b input
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	if c.ShouldBindJSON(&b) != nil {
		c.JSON(422, gin.H{"detail": "輸入格式不正確"})
		return b, false
	}
	b.Email = strings.ToLower(strings.TrimSpace(b.Email))
	return b, true
}
func validEmail(s string) bool {
	address, err := mail.ParseAddress(s)
	return err == nil && address.Address == s && len(s) <= 254
}
func configured() bool {
	return os.Getenv("MAIL_SERVICE_URL") != "" && os.Getenv("MAIL_SERVICE_KEY") != ""
}
func passwordValid(p string) bool { return utf8.RuneCountInString(p) >= 8 && len(p) <= 72 }

func (r *Recovery) newToken(u member, purpose, email string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	_, err := r.DB.Exec("INSERT INTO recovery_tokens(token_hash,user_id,password_snapshot,purpose,pending_email,expires_at) VALUES (?,?,?,?,?,?)", Fingerprint(token), u.ID, Fingerprint(u.Hash), purpose, email, time.Now().Add(30*time.Minute).Unix())
	return token, err
}

func sendShared(endpoint string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", strings.TrimRight(os.Getenv("MAIL_SERVICE_URL"), "/")+endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mail-Key", os.Getenv("MAIL_SERVICE_KEY"))
	client := &http.Client{Timeout: 25 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return errors.New("mail service connection failed")
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 2048))
	if response.StatusCode != 200 {
		return fmt.Errorf("mail service returned %d", response.StatusCode)
	}
	return nil
}

func (r *Recovery) forgot(c *gin.Context) {
	b, ok := read(c)
	if !ok {
		return
	}
	if !validEmail(b.Email) {
		c.JSON(422, gin.H{"detail": "請輸入有效的信箱"})
		return
	}
	if !configured() {
		c.JSON(503, gin.H{"detail": "寄信服務尚未設定"})
		return
	}
	if !r.limit(c, b.Email, "forgot") {
		return
	}
	u, err := r.find(r.DB, b.Email, false)
	if err == nil && u.Hash != "" && u.Email != "" && u.Verified == 1 {
		token, err := r.newToken(u, "reset", "")
		if err != nil {
			c.JSON(503, gin.H{"detail": "服務暫時忙碌"})
			return
		}
		go func() {
			if sendShared("/api/mail/password-reset", gin.H{"recipient": u.Email, "token": token, "expires_minutes": 30}) != nil {
				log.Print("password reset email failed; inspect shared mail service")
			}
		}()
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		c.JSON(503, gin.H{"detail": "服務暫時忙碌"})
		return
	}
	c.JSON(202, gin.H{"message": "若此信箱已綁定有效帳號，我們會寄出30分鐘內有效的重設連結。"})
}

func (r *Recovery) save(tx *sql.Tx, u member, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	result, err := tx.Exec("UPDATE users SET password_hash=? WHERE id=? AND password_hash=?", string(hash), u.ID, u.Hash)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return errors.New("account changed")
	}
	if _, err = tx.Exec("INSERT OR REPLACE INTO recovery_password_changes VALUES (?,?)", u.ID, time.Now().Unix()); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE recovery_tokens SET used=1 WHERE user_id=?", u.ID); err != nil {
		return err
	}
	if r.Paw {
		_, err = tx.Exec("DELETE FROM sessions WHERE user_id=?", u.ID)
	}
	return err
}

func (r *Recovery) reset(c *gin.Context) {
	b, ok := read(c)
	if !ok {
		return
	}
	if !passwordValid(b.NewPassword) || len(b.Token) < 20 || len(b.Token) > 2048 {
		c.JSON(422, gin.H{"detail": "新密碼至少8碼且最多72位元組，並需有效連結"})
		return
	}
	tx, err := r.DB.Begin()
	if err != nil {
		c.JSON(503, gin.H{"detail": "服務暫時忙碌"})
		return
	}
	defer tx.Rollback()
	var id, snapshot string
	err = tx.QueryRow("SELECT user_id,password_snapshot FROM recovery_tokens WHERE token_hash=? AND purpose='reset' AND used=0 AND expires_at>?", Fingerprint(b.Token), time.Now().Unix()).Scan(&id, &snapshot)
	if err != nil {
		c.JSON(400, gin.H{"detail": "連結無效、已使用或已過期"})
		return
	}
	u, err := r.find(tx, id, true)
	if err != nil || u.Hash == "" || snapshot != Fingerprint(u.Hash) {
		c.JSON(400, gin.H{"detail": "連結無效、已使用或已過期"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(b.NewPassword)) == nil {
		c.JSON(400, gin.H{"detail": "新密碼不可與目前密碼相同"})
		return
	}
	spent, err := tx.Exec("UPDATE recovery_tokens SET used=1 WHERE token_hash=? AND used=0", Fingerprint(b.Token))
	if err != nil {
		c.JSON(400, gin.H{"detail": "請重新申請連結"})
		return
	}
	n, _ := spent.RowsAffected()
	if n != 1 {
		c.JSON(400, gin.H{"detail": "連結已使用"})
		return
	}
	if r.save(tx, u, b.NewPassword) != nil || tx.Commit() != nil {
		c.JSON(503, gin.H{"detail": "服務暫時忙碌，請重新操作"})
		return
	}
	c.JSON(200, gin.H{"message": "密碼已重設，舊登入已失效，請重新登入。"})
}

func (r *Recovery) change(c *gin.Context) {
	b, ok := read(c)
	if !ok {
		return
	}
	if !passwordValid(b.NewPassword) || len(b.CurrentPassword) > 1024 || b.Email == "" {
		c.JSON(422, gin.H{"detail": "請輸入帳號、目前密碼與至少8碼的新密碼（最多72位元組）"})
		return
	}
	if !r.limit(c, b.Email, "change") {
		return
	}
	tx, err := r.DB.Begin()
	if err != nil {
		c.JSON(503, gin.H{"detail": "服務暫時忙碌"})
		return
	}
	defer tx.Rollback()
	u, err := r.find(tx, b.Email, false)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(b.CurrentPassword)) != nil {
		c.JSON(400, gin.H{"detail": "帳號或目前密碼不正確"})
		return
	}
	if b.CurrentPassword == b.NewPassword {
		c.JSON(400, gin.H{"detail": "新密碼不可與目前密碼相同"})
		return
	}
	if r.save(tx, u, b.NewPassword) != nil || tx.Commit() != nil {
		c.JSON(503, gin.H{"detail": "服務暫時忙碌，請重新操作"})
		return
	}
	c.JSON(200, gin.H{"message": "密碼已修改，舊登入已失效，請重新登入。"})
}

func (r *Recovery) bind(c *gin.Context) {
	b, ok := read(c)
	if !ok {
		return
	}
	if !configured() {
		c.JSON(503, gin.H{"detail": "寄信服務尚未設定"})
		return
	}
	if !validEmail(b.Email) || b.Username == "" || len(b.CurrentPassword) > 1024 {
		c.JSON(422, gin.H{"detail": "請輸入帳號、目前密碼及有效信箱"})
		return
	}
	if !r.limit(c, b.Username, "bind") {
		return
	}
	u, err := r.find(r.DB, strings.ToLower(strings.TrimSpace(b.Username)), false)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(b.CurrentPassword)) != nil {
		c.JSON(400, gin.H{"detail": "帳號或目前密碼不正確"})
		return
	}
	token, err := r.newToken(u, "bind", b.Email)
	if err != nil {
		c.JSON(503, gin.H{"detail": "服務暫時忙碌"})
		return
	}
	target := os.Getenv("PASSWORD_RECOVERY_URL")
	if target == "" {
		c.JSON(503, gin.H{"detail": "尚未設定驗證網址"})
		return
	}
	err = sendShared("/api/mail/send", gin.H{"recipient": b.Email, "subject": r.Title + "：驗證恢復信箱", "text": "請在30分鐘內開啟此連結驗證恢復信箱：\n" + target + "#verify_token=" + token + "\n\n若非本人操作，請忽略。"})
	if err != nil {
		c.JSON(502, gin.H{"detail": "驗證信寄送失敗，請稍後再試"})
		return
	}
	c.JSON(200, gin.H{"message": "驗證信已寄出，完成信箱驗證後才能用來重設密碼。"})
}

func (r *Recovery) verifyEmail(c *gin.Context) {
	b, ok := read(c)
	if !ok {
		return
	}
	if len(b.Token) < 20 || len(b.Token) > 2048 {
		c.JSON(400, gin.H{"detail": "驗證連結無效"})
		return
	}
	tx, err := r.DB.Begin()
	if err != nil {
		c.JSON(503, gin.H{"detail": "服務暫時忙碌"})
		return
	}
	defer tx.Rollback()
	var id, snapshot, email string
	err = tx.QueryRow("SELECT user_id,password_snapshot,pending_email FROM recovery_tokens WHERE token_hash=? AND purpose='bind' AND used=0 AND expires_at>?", Fingerprint(b.Token), time.Now().Unix()).Scan(&id, &snapshot, &email)
	u, findErr := r.find(tx, id, true)
	if err != nil || findErr != nil || snapshot != Fingerprint(u.Hash) {
		c.JSON(400, gin.H{"detail": "驗證連結無效、已使用或已過期"})
		return
	}
	if _, err = tx.Exec("UPDATE users SET email=?,email_verified=1 WHERE id=?", email, id); err != nil {
		c.JSON(400, gin.H{"detail": "此信箱已被綁定或暫時無法使用"})
		return
	}
	if _, err = tx.Exec("UPDATE recovery_tokens SET used=1 WHERE user_id=?", id); err != nil {
		c.JSON(503, gin.H{"detail": "服務暫時忙碌"})
		return
	}
	if tx.Commit() != nil {
		c.JSON(503, gin.H{"detail": "服務暫時忙碌"})
		return
	}
	c.JSON(200, gin.H{"message": "恢復信箱已驗證，可用此信箱申請密碼重設。"})
}

const pawBindingForm = `<section><h2>綁定恢復信箱</h2><form id="bindForm"><label for="bindUsername">帳號名稱</label><input id="bindUsername" autocomplete="username" required><label for="bindPassword">目前密碼</label><input id="bindPassword" type="password" autocomplete="current-password" required><label for="bindEmail">恢復信箱</label><input id="bindEmail" type="email" required><button>寄出信箱驗證信</button></form></section><script>document.getElementById('bindForm').onsubmit=e=>submit(e,'/recovery-email/request',{username:document.getElementById('bindUsername').value.trim(),current_password:document.getElementById('bindPassword').value,email:document.getElementById('bindEmail').value.trim()});const verifyToken=new URLSearchParams(location.hash.slice(1)).get('verify_token');if(verifyToken){history.replaceState(null,'',location.pathname);fetch('/api/account/recovery-email/verify',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({token:verifyToken})}).then(r=>r.json()).then(d=>document.getElementById('message').textContent=d.message||d.detail).catch(()=>document.getElementById('message').textContent='驗證失敗，請重新申請');}</script>`
