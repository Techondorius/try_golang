package httpapi_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"try_golang/internal/application"
	"try_golang/internal/infrastructure/memory"
	"try_golang/internal/infrastructure/security"
	"try_golang/internal/transport/httpapi"
)

// setup は、本物のGin・ユースケース・メモリ保存・bcryptを接続したHTTPテスト環境を作ります。
// 時計とbcryptのcostを制御しつつ、HTTP入口から各層が正しくつながることを確認します。
func setup(t *testing.T) (*gin.Engine, *time.Time) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	s := memory.New(clock)
	auth, err := application.NewAuthUseCase(s, s, security.Bcrypt{Cost: bcrypt.MinCost}, security.RandomID, clock, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	router, err := httpapi.NewRouter(auth, application.NewTasksUseCase(s, security.RandomID, clock), httpapi.Options{Now: clock, AuthLimit: 100, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	return router, &now
}

// request は、ネットワークの待受を行わず、http.Handlerへ直接リクエストを渡します。
// httptestの記録先から、ステータス・ヘッダー・JSON本文を実際のHTTP処理の結果として検証できます。
func request(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.1:1234"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	out := httptest.NewRecorder()
	router.ServeHTTP(out, req)
	return out
}

// wantStatus は、期待するHTTPステータスを確認し、不一致なら応答本文とともにテストを止めます。
func wantStatus(t *testing.T, r *httptest.ResponseRecorder, status int) {
	t.Helper()
	if r.Code != status {
		t.Fatalf("status %d, want %d: %s", r.Code, status, r.Body.String())
	}
}

// object は、応答をJSONオブジェクトとして読み、公開された項目を検証できる形にします。
func object(t *testing.T, r *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// registerLogin は、HTTP経由でユーザーを登録し、ログインしたトークンを返します。
// 保存層へ直接データを入れず、認証が必要なテストの準備でも公開APIの流れを使います。
func registerLogin(t *testing.T, router http.Handler, email string) string {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":"long-password"}`, email)
	r := request(router, "POST", "/v1/auth/register", body, "")
	wantStatus(t, r, 201)
	if strings.Contains(r.Body.String(), "password") || strings.Contains(r.Body.String(), "$2") {
		t.Fatal("registration exposed credentials")
	}
	r = request(router, "POST", "/v1/auth/login", body, "")
	wantStatus(t, r, 200)
	obj := object(t, r)
	if obj["token_type"] != "Bearer" || r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("unsafe token response")
	}
	token, ok := obj["access_token"].(string)
	if !ok || token == "" {
		t.Fatal("missing access token")
	}
	return token
}

func TestAuthenticatedTaskAPI(t *testing.T) {
	router, _ := setup(t)
	wantStatus(t, request(router, "GET", "/healthz", "", ""), 200)
	wantStatus(t, request(router, "GET", "/v1/tasks", "", ""), 401)
	alice := registerLogin(t, router, "alice@example.com")
	bob := registerLogin(t, router, "bob@example.com")
	me := request(router, "GET", "/v1/me", "", alice)
	wantStatus(t, me, 200)
	if object(t, me)["email"] != "alice@example.com" || strings.Contains(me.Body.String(), "password") {
		t.Fatal("incorrect profile")
	}
	r := request(router, "POST", "/v1/tasks", `{"title":" Write tests "}`, alice)
	wantStatus(t, r, 201)
	v := object(t, r)
	id := v["id"].(string)
	path := "/v1/tasks/" + id
	if v["title"] != "Write tests" || v["status"] != "pending" || r.Header().Get("Location") != path {
		t.Fatalf("unexpected task: %v", v)
	}
	wantStatus(t, request(router, "GET", path, "", alice), 200)
	for _, operation := range []struct{ method, path, body string }{{"GET", path, ""}, {"PATCH", path, `{"title":"stolen"}`}, {"POST", path + "/complete", ""}, {"DELETE", path, ""}} {
		wantStatus(t, request(router, operation.method, operation.path, operation.body, bob), 404)
	}
	r = request(router, "GET", "/v1/tasks", "", bob)
	wantStatus(t, r, 200)
	if items := object(t, r)["tasks"].([]any); len(items) != 0 {
		t.Fatal("foreign tasks leaked")
	}
	r = request(router, "GET", "/v1/tasks", "", alice)
	wantStatus(t, r, 200)
	if len(object(t, r)["tasks"].([]any)) != 1 {
		t.Fatal("created task missing")
	}
	r = request(router, "PATCH", path, `{"title":"Refactor"}`, alice)
	wantStatus(t, r, 200)
	if object(t, r)["title"] != "Refactor" {
		t.Fatal("rename failed")
	}
	r = request(router, "POST", path+"/complete", "", alice)
	wantStatus(t, r, 200)
	if object(t, r)["status"] != "completed" {
		t.Fatal("completion failed")
	}
	wantStatus(t, request(router, "POST", path+"/complete", "", alice), 409)
	wantStatus(t, request(router, "PATCH", path, `{"title":"late edit"}`, alice), 409)
	wantStatus(t, request(router, "DELETE", path, "", alice), 204)
	wantStatus(t, request(router, "GET", path, "", alice), 404)
	wantStatus(t, request(router, "POST", "/v1/auth/logout", "", alice), 204)
	wantStatus(t, request(router, "GET", "/v1/me", "", alice), 401)
	wantStatus(t, request(router, "GET", "/v1/me", "", bob), 200)
}

func TestAuthValidationExpiryAndErrorContract(t *testing.T) {
	router, now := setup(t)
	token := registerLogin(t, router, "alice@example.com")
	wantStatus(t, request(router, "POST", "/v1/auth/register", `{"email":"ALICE@example.com","password":"long-password"}`, ""), 409)
	wrong := request(router, "POST", "/v1/auth/login", `{"email":"alice@example.com","password":"wrong-password"}`, "")
	missing := request(router, "POST", "/v1/auth/login", `{"email":"unknown@example.com","password":"wrong-password"}`, "")
	wantStatus(t, wrong, 401)
	wantStatus(t, missing, 401)
	if wrong.Body.String() != missing.Body.String() {
		t.Fatal("credential error enumerates users")
	}
	if wrong.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("missing authentication challenge")
	}
	wantStatus(t, request(router, "POST", "/v1/auth/register", `{"email":"bad","password":"long-password"}`, ""), 400)
	wantStatus(t, request(router, "POST", "/v1/auth/register", `{"email":"x@example.com","password":"short"}`, ""), 400)
	for _, body := range []string{`{`, `null`, `{}`, `{"title":""}`, `{"title":"ok","owner_id":"bob"}`, `{"title":"ok"} {}`} {
		wantStatus(t, request(router, "POST", "/v1/tasks", body, token), 400)
	}
	wantStatus(t, request(router, "POST", "/v1/tasks", `{"title":"`+strings.Repeat("x", 9000)+`"}`, token), 413)
	req := httptest.NewRequest("POST", "/v1/tasks", strings.NewReader(`{"title":"x"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	out := httptest.NewRecorder()
	router.ServeHTTP(out, req)
	wantStatus(t, out, 415)
	for _, header := range []string{"Basic xyz", "Bearer", "Bearer token extra", "Bearer invalid"} {
		req = httptest.NewRequest("GET", "/v1/me", nil)
		req.Header.Set("Authorization", header)
		out = httptest.NewRecorder()
		router.ServeHTTP(out, req)
		wantStatus(t, out, 401)
	}
	*now = now.Add(time.Hour)
	wantStatus(t, request(router, "GET", "/v1/me", "", token), 401)
	r := request(router, "GET", "/missing", "", "")
	wantStatus(t, r, 404)
	if object(t, r)["error"] == nil {
		t.Fatal("non-JSON 404")
	}
	r = request(router, "PUT", "/healthz", "", "")
	wantStatus(t, r, 405)
	if object(t, r)["error"] == nil {
		t.Fatal("non-JSON 405")
	}
}

func TestTrailingSlashErrorContract(t *testing.T) {
	router, _ := setup(t)
	out := request(router, "GET", "/healthz/", "", "")
	wantStatus(t, out, 404)
	if object(t, out)["error"].(map[string]any)["code"] != "not_found" || out.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("inconsistent unknown URL response")
	}
}
