package recovery

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func fixture(t *testing.T, paw bool) (*Recovery, *gin.Engine) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE users(id TEXT PRIMARY KEY,username TEXT,email TEXT UNIQUE,password_hash TEXT,email_verified INTEGER,deleted_at TEXT);CREATE TABLE sessions(token TEXT,user_id TEXT);`)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("OriginalPassword123"), bcrypt.MinCost)
	_, err = db.Exec("INSERT INTO users VALUES ('1','member','member@example.com',?,1,NULL)", string(hash))
	if err != nil {
		t.Fatal(err)
	}
	r := &Recovery{DB: db, Prefix: "/api/auth", Title: "Test", Paw: paw}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if err = r.Register(router); err != nil {
		t.Fatal(err)
	}
	return r, router
}

func post(router *gin.Engine, path string, body interface{}) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	request := httptest.NewRequest("POST", path, bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:1234"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestOneTimeResetAndSnapshot(t *testing.T) {
	r, router := fixture(t, false)
	u, _ := r.find(r.DB, "member@example.com", false)
	token, err := r.newToken(u, "reset", "")
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]string{"token": token, "new_password": "ReplacementPassword123"}
	if got := post(router, "/api/auth/reset-password", body); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := post(router, "/api/auth/reset-password", body); got.Code != 400 {
		t.Fatal(got.Code, got.Body.String())
	}
	after, _ := r.find(r.DB, u.ID, true)
	if bcrypt.CompareHashAndPassword([]byte(after.Hash), []byte(body["new_password"])) != nil {
		t.Fatal("password not changed")
	}
	var count int
	r.DB.QueryRow("SELECT COUNT(*) FROM recovery_password_changes WHERE user_id=?", u.ID).Scan(&count)
	if count != 1 {
		t.Fatal("missing revocation")
	}
}

func TestExpiryAndPasswordChangeInvalidatesReset(t *testing.T) {
	r, router := fixture(t, true)
	u, _ := r.find(r.DB, "member", false)
	token, _ := r.newToken(u, "reset", "")
	r.DB.Exec("UPDATE recovery_tokens SET expires_at=?", time.Now().Unix()-1)
	if got := post(router, "/api/auth/reset-password", map[string]string{"token": token, "new_password": "ReplacementPassword123"}); got.Code != 400 {
		t.Fatal(got.Code)
	}
	token, _ = r.newToken(u, "reset", "")
	r.DB.Exec("INSERT INTO sessions VALUES ('old-session','1')")
	body := map[string]string{"email": "member", "current_password": "wrong", "new_password": "ReplacementPassword123"}
	if got := post(router, "/api/auth/change-password", body); got.Code != 400 {
		t.Fatal(got.Code)
	}
	body["current_password"] = "OriginalPassword123"
	if got := post(router, "/api/auth/change-password", body); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := post(router, "/api/auth/reset-password", map[string]string{"token": token, "new_password": "AnotherPassword123"}); got.Code != 400 {
		t.Fatal(got.Code)
	}
	var count int
	r.DB.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	if count != 0 {
		t.Fatal("old sessions survived")
	}
}

func TestWeakPasswordDoesNotSpendToken(t *testing.T) {
	r, router := fixture(t, false)
	u, _ := r.find(r.DB, "member@example.com", false)
	token, _ := r.newToken(u, "reset", "")
	if got := post(router, "/api/auth/reset-password", map[string]string{"token": token, "new_password": "short"}); got.Code != 422 {
		t.Fatal(got.Code)
	}
	if got := post(router, "/api/auth/reset-password", map[string]string{"token": token, "new_password": "ReplacementPassword123"}); got.Code != 200 {
		t.Fatal(got.Code)
	}
}

func TestVerifiedEmailBinding(t *testing.T) {
	r, router := fixture(t, true)
	u, _ := r.find(r.DB, "member", false)
	token, _ := r.newToken(u, "bind", "new@example.com")
	if got := post(router, "/api/auth/recovery-email/verify", map[string]string{"token": token}); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	after, _ := r.find(r.DB, "new@example.com", false)
	if after.ID != u.ID || after.Verified != 1 {
		t.Fatal("email not verified")
	}
	if got := post(router, "/api/auth/recovery-email/verify", map[string]string{"token": token}); got.Code != 400 {
		t.Fatal(got.Code)
	}
}
