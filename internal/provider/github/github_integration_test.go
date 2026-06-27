package github

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/nacl/box"

	"github.com/ptyhard/env-sync/internal/provider"
)

// withGitHubAPIBase はテスト中だけ githubAPIBase をテストサーバに差し替える。
func withGitHubAPIBase(t *testing.T, base string) {
	t.Helper()
	orig := githubAPIBase
	githubAPIBase = base
	t.Cleanup(func() { githubAPIBase = orig })
}

// genTestPublicKey は base64 エンコードした 32 バイトの curve25519 公開鍵を返す。
func genTestPublicKey(t *testing.T) (b64 string, pub *[32]byte) {
	t.Helper()
	pubKey, _, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("鍵生成失敗: %v", err)
	}
	return base64.StdEncoding.EncodeToString(pubKey[:]), pubKey
}

func TestGitHubPublicKey_Success(t *testing.T) {
	b64Key, _ := genTestPublicKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/actions/secrets/public-key" {
			t.Errorf("path = %s, want repo-level public-key", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok123" {
			t.Errorf("Authorization = %q, want Bearer tok123", got)
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept = %q", got)
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != "2022-11-28" {
			t.Errorf("X-GitHub-Api-Version = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"key_id": "kid-1", "key": b64Key})
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	keyID, key, err := githubPublicKey(srv.Client(), "tok123", "owner", "repo", "")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if keyID != "kid-1" {
		t.Errorf("keyID = %q, want kid-1", keyID)
	}
	if key == nil || len(base64.StdEncoding.EncodeToString(key[:])) == 0 {
		t.Error("公開鍵が取得できていない")
	}
	if base64.StdEncoding.EncodeToString(key[:]) != b64Key {
		t.Error("公開鍵がレスポンスと一致しない")
	}
}

func TestGitHubPublicKey_EnvironmentScope(t *testing.T) {
	b64Key, _ := genTestPublicKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/environments/staging/secrets/public-key" {
			t.Errorf("path = %s, want environment-scoped public-key", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"key_id": "kid-2", "key": b64Key})
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	_, _, err := githubPublicKey(srv.Client(), "tok", "owner", "repo", "staging")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
}

func TestGitHubPublicKey_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	_, _, err := githubPublicKey(srv.Client(), "tok", "owner", "repo", "")
	if err == nil {
		t.Fatal("エラーを期待したが nil")
	}
	if !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("エラーメッセージに API メッセージが含まれていない: %v", err)
	}
}

func TestGitHubPublicKey_InvalidKeyLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 32 バイトでない不正な鍵を返す
		_ = json.NewEncoder(w).Encode(map[string]string{
			"key_id": "kid",
			"key":    base64.StdEncoding.EncodeToString([]byte("too-short")),
		})
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	_, _, err := githubPublicKey(srv.Client(), "tok", "owner", "repo", "")
	if err == nil {
		t.Fatal("不正な鍵長でエラーを期待したが nil")
	}
}

func TestGitHubPutSecret_Success(t *testing.T) {
	for _, status := range []int{http.StatusCreated, http.StatusNoContent} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %s, want PUT", r.Method)
			}
			if r.URL.Path != "/repos/owner/repo/actions/secrets/MY_SECRET" {
				t.Errorf("path = %s", r.URL.Path)
			}
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["encrypted_value"] != "enc==" {
				t.Errorf("encrypted_value = %q", body["encrypted_value"])
			}
			if body["key_id"] != "kid" {
				t.Errorf("key_id = %q", body["key_id"])
			}
			w.WriteHeader(status)
		}))
		withGitHubAPIBase(t, srv.URL)
		err := githubPutSecret(srv.Client(), "tok", "owner", "repo", "", "MY_SECRET", "enc==", "kid")
		if err != nil {
			t.Errorf("status %d で予期しないエラー: %v", status, err)
		}
		srv.Close()
	}
}

func TestGitHubPutSecret_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Invalid request"})
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	err := githubPutSecret(srv.Client(), "tok", "owner", "repo", "", "S", "enc", "kid")
	if err == nil {
		t.Fatal("エラーを期待したが nil")
	}
	if !strings.Contains(err.Error(), "Invalid request") {
		t.Errorf("API メッセージが含まれていない: %v", err)
	}
}

func TestGitHubVariableExists(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   bool
		errOK  bool
	}{
		{"存在する", http.StatusOK, true, false},
		{"存在しない", http.StatusNotFound, false, false},
		{"サーバエラー", http.StatusInternalServerError, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/owner/repo/actions/variables/MY_VAR" {
					t.Errorf("path = %s", r.URL.Path)
				}
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()
			withGitHubAPIBase(t, srv.URL)

			got, err := githubVariableExists(srv.Client(), "tok", "owner", "repo", "", "MY_VAR")
			if tc.errOK && err == nil {
				t.Fatal("エラーを期待したが nil")
			}
			if !tc.errOK && err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if got != tc.want {
				t.Errorf("exists = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGitHubCreateVariable_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/actions/variables" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "MY_VAR" || body["value"] != "hello" {
			t.Errorf("body = %v", body)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	if err := githubCreateVariable(srv.Client(), "tok", "owner", "repo", "", "MY_VAR", "hello"); err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
}

func TestGitHubUpdateVariable_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/actions/variables/MY_VAR" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	if err := githubUpdateVariable(srv.Client(), "tok", "owner", "repo", "", "MY_VAR", "hello"); err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
}

func TestGitHubCreateVariable_EnvironmentScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/environments/prod/variables" {
			t.Errorf("path = %s, want environment-scoped variables", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	if err := githubCreateVariable(srv.Client(), "tok", "owner", "repo", "prod", "V", "x"); err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
}

func TestGitHubPutSecret_EnvironmentScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/environments/staging/secrets/MY_SECRET" {
			t.Errorf("path = %s, want environment-scoped secret PUT", r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["encrypted_value"] != "enc==" {
			t.Errorf("encrypted_value = %q", body["encrypted_value"])
		}
		if body["key_id"] != "kid" {
			t.Errorf("key_id = %q", body["key_id"])
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	if err := githubPutSecret(srv.Client(), "tok", "owner", "repo", "staging", "MY_SECRET", "enc==", "kid"); err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
}

func TestGitHubVariableExists_EnvironmentScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/environments/staging/variables/MY_VAR" {
			t.Errorf("path = %s, want environment-scoped variable GET", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	got, err := githubVariableExists(srv.Client(), "tok", "owner", "repo", "staging", "MY_VAR")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !got {
		t.Error("exists = false, want true")
	}
}

func TestGitHubUpdateVariable_EnvironmentScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/environments/staging/variables/MY_VAR" {
			t.Errorf("path = %s, want environment-scoped variable PATCH", r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "MY_VAR" || body["value"] != "hello" {
			t.Errorf("body = %v", body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	if err := githubUpdateVariable(srv.Client(), "tok", "owner", "repo", "staging", "MY_VAR", "hello"); err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
}

func TestGitHubSecretExists(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   bool
		errOK  bool
	}{
		{"存在する", http.StatusOK, true, false},
		{"存在しない", http.StatusNotFound, false, false},
		{"サーバエラー", http.StatusInternalServerError, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				if r.URL.Path != "/repos/owner/repo/actions/secrets/MY_SECRET" {
					t.Errorf("path = %s, want repo-level secret GET", r.URL.Path)
				}
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()
			withGitHubAPIBase(t, srv.URL)

			got, err := githubSecretExists(srv.Client(), "tok", "owner", "repo", "", "MY_SECRET")
			if tc.errOK && err == nil {
				t.Fatal("エラーを期待したが nil")
			}
			if !tc.errOK && err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if got != tc.want {
				t.Errorf("exists = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGitHubSecretExists_EnvironmentScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/environments/staging/secrets/MY_SECRET" {
			t.Errorf("path = %s, want environment-scoped secret GET", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	got, err := githubSecretExists(srv.Client(), "tok", "owner", "repo", "staging", "MY_SECRET")
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if !got {
		t.Error("exists = false, want true")
	}
}

func TestParseGitHubErrorBody(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"メッセージあり", `{"message":"Bad credentials"}`, "Bad credentials"},
		{"メッセージなし", `{"foo":"bar"}`, ""},
		{"不正な JSON", `not json`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseGitHubErrorBody(io.NopCloser(strings.NewReader(tc.body)))
			if got != tc.want {
				t.Errorf("parseGitHubErrorBody = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- syncOneGitHubTarget のテスト ---

// makeTestTasks は テスト用に Entry スライスを githubTask スライスへ変換するヘルパー。
func makeTestTasks(entries []provider.Entry) []githubTask {
	return expandGitHubTasks(entries)
}

func TestSyncOneGitHubTarget_UsesPerTargetToken(t *testing.T) {
	// per-target token が Authorization ヘッダに反映されること
	b64Key, _ := genTestPublicKey(t)
	var receivedToken string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		receivedToken = auth
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "public-key"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"key_id": "kid-1", "key": b64Key})
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	tasks := makeTestTasks([]provider.Entry{
		{Key: "SECRET_FOO", Value: "bar", Secret: true},
	})
	// classified=nil（分類スキップ）で呼ぶ
	ok, ng := syncOneGitHubTarget(srv.Client(), "per-target-token", "owner", "repo", tasks, nil)
	if ok != 1 || ng != 0 {
		t.Errorf("ok=%d ng=%d, want ok=1 ng=0", ok, ng)
	}
	if receivedToken != "Bearer per-target-token" {
		t.Errorf("Authorization = %q, want Bearer per-target-token", receivedToken)
	}
}

func TestSyncOneGitHubTarget_MultipleRepos(t *testing.T) {
	// 2 つのリポジトリへの送信が異なるトークンで行われること
	b64Key1, _ := genTestPublicKey(t)
	b64Key2, _ := genTestPublicKey(t)

	var repo1Tokens, repo2Tokens []string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		mu.Lock()
		if strings.Contains(r.URL.Path, "/repo1/") {
			repo1Tokens = append(repo1Tokens, auth)
		} else if strings.Contains(r.URL.Path, "/repo2/") {
			repo2Tokens = append(repo2Tokens, auth)
		}
		mu.Unlock()

		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "public-key"):
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(r.URL.Path, "/repo1/") {
				_ = json.NewEncoder(w).Encode(map[string]string{"key_id": "kid-1", "key": b64Key1})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]string{"key_id": "kid-2", "key": b64Key2})
			}
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	tasks := makeTestTasks([]provider.Entry{
		{Key: "MY_SECRET", Value: "value1", Secret: true},
	})

	// リポジトリ 1: token-for-repo1（classified=nil で呼ぶ）
	ok1, ng1 := syncOneGitHubTarget(srv.Client(), "token-for-repo1", "owner", "repo1", tasks, nil)
	// リポジトリ 2: token-for-repo2（classified=nil で呼ぶ）
	ok2, ng2 := syncOneGitHubTarget(srv.Client(), "token-for-repo2", "owner", "repo2", tasks, nil)

	if ok1 != 1 || ng1 != 0 {
		t.Errorf("repo1: ok=%d ng=%d, want ok=1 ng=0", ok1, ng1)
	}
	if ok2 != 1 || ng2 != 0 {
		t.Errorf("repo2: ok=%d ng=%d, want ok=1 ng=0", ok2, ng2)
	}

	mu.Lock()
	defer mu.Unlock()
	// repo1 の全リクエストには token-for-repo1 が使われる
	for _, tok := range repo1Tokens {
		if tok != "Bearer token-for-repo1" {
			t.Errorf("repo1 token = %q, want Bearer token-for-repo1", tok)
		}
	}
	// repo2 の全リクエストには token-for-repo2 が使われる
	for _, tok := range repo2Tokens {
		if tok != "Bearer token-for-repo2" {
			t.Errorf("repo2 token = %q, want Bearer token-for-repo2", tok)
		}
	}
}

func TestSyncOneGitHubTarget_PartialFailure_ContinuesAndCounts(t *testing.T) {
	// 1 件目の Secret PUT が失敗（422）でも 2 件目は送信され、失敗が集計されること。
	// 個別変数の送信失敗はループ内で継続する設計（os.Exit は呼ばない）の検証。
	b64Key, _ := genTestPublicKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "public-key"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"key_id": "kid-1", "key": b64Key})
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "secrets/FAIL_KEY"):
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Invalid request"})
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	tasks := makeTestTasks([]provider.Entry{
		{Key: "FAIL_KEY", Value: "v1", Secret: true},
		{Key: "OK_KEY", Value: "v2", Secret: true},
	})
	ok, ng := syncOneGitHubTarget(srv.Client(), "token", "owner", "repo", tasks, nil)
	if ok != 1 || ng != 1 {
		t.Errorf("ok=%d ng=%d, want ok=1 ng=1（1 件失敗しても残りを送信し集計する）", ok, ng)
	}
}

func TestSyncOneGitHubTarget_FailingTargetDoesNotBlockNext(t *testing.T) {
	// 1 つ目のリポジトリへの送信が全失敗でも、2 つ目のリポジトリへ PUT が飛ぶこと。
	// Sync が複数リポジトリを順に処理する設計の最小検証（os.Exit を含む Sync 自体は範囲外）。
	b64Key1, _ := genTestPublicKey(t)
	b64Key2, _ := genTestPublicKey(t)
	var repo2Puts int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "public-key"):
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(r.URL.Path, "/repo1/") {
				_ = json.NewEncoder(w).Encode(map[string]string{"key_id": "kid-1", "key": b64Key1})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]string{"key_id": "kid-2", "key": b64Key2})
			}
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/repo1/"):
			// repo1 は全失敗
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "boom"})
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/repo2/"):
			mu.Lock()
			repo2Puts++
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	tasks := makeTestTasks([]provider.Entry{
		{Key: "MY_SECRET", Value: "v", Secret: true},
	})

	ok1, ng1 := syncOneGitHubTarget(srv.Client(), "token1", "owner", "repo1", tasks, nil)
	ok2, ng2 := syncOneGitHubTarget(srv.Client(), "token2", "owner", "repo2", tasks, nil)

	if ok1 != 0 || ng1 != 1 {
		t.Errorf("repo1: ok=%d ng=%d, want ok=0 ng=1（全失敗）", ok1, ng1)
	}
	if ok2 != 1 || ng2 != 0 {
		t.Errorf("repo2: ok=%d ng=%d, want ok=1 ng=0（前リポジトリ失敗に影響されず成功）", ok2, ng2)
	}
	mu.Lock()
	defer mu.Unlock()
	if repo2Puts != 1 {
		t.Errorf("repo2 への PUT 回数 = %d, want 1（前リポジトリの失敗で中断されない）", repo2Puts)
	}
}

func TestSyncOneGitHubTarget_Variable(t *testing.T) {
	// variable の送信確認（classified=nil の場合は存在確認 API にフォールバック）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "variables/MY_VAR"):
			// 存在しない（新規）
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "variables"):
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	tasks := makeTestTasks([]provider.Entry{
		{Key: "MY_VAR", Value: "value", Secret: false},
	})
	ok, ng := syncOneGitHubTarget(srv.Client(), "token", "owner", "repo", tasks, nil)
	if ok != 1 || ng != 0 {
		t.Errorf("ok=%d ng=%d, want ok=1 ng=0", ok, ng)
	}
}

func TestSyncOneGitHubTarget_Variable_WithClassified(t *testing.T) {
	// classified が渡された場合、変数の存在確認 API を再呼び出しせずに classified を使う
	var existCheckCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "variables"):
			// 存在確認 API が呼ばれたら記録
			existCheckCalled = true
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "variables"):
			// 更新（classified で isNew=false）
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	withGitHubAPIBase(t, srv.URL)

	entries := []provider.Entry{
		{Key: "MY_VAR", Value: "value", Secret: false},
	}
	tasks := makeTestTasks(entries)
	// classified を用意: isNew=false（更新扱い）
	classified := []githubClassifiedTask{
		{task: tasks[0], isNew: false},
	}
	ok, ng := syncOneGitHubTarget(srv.Client(), "token", "owner", "repo", tasks, classified)
	if ok != 1 || ng != 0 {
		t.Errorf("ok=%d ng=%d, want ok=1 ng=0", ok, ng)
	}
	if existCheckCalled {
		t.Error("classified が渡されているのに存在確認 API が呼ばれた（二重呼び出し退行）")
	}
}
