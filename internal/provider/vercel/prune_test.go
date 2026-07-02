package vercel

import (
	"net/http"
	"strings"
	"testing"

	"github.com/ptyhard/env-sync/internal/provider"
)

// keepFunc はテスト用に Options.PruneKeep 相当の保持判定を作るヘルパー。
func keepFunc(definedKeys ...string) func(string) bool {
	opts := provider.Options{DefinedKeys: definedKeys}
	return opts.PruneKeep()
}

// --- computeVercelPrune のテスト ---

func TestComputeVercelPrune_UndefinedKeyIsPruned(t *testing.T) {
	envs := []vercelEnv{
		{ID: "id1", Key: "DEFINED_KEY"},
		{ID: "id2", Key: "STALE_KEY"},
	}
	got := computeVercelPrune(envs, keepFunc("DEFINED_KEY"))
	if len(got) != 1 || got[0].Key != "STALE_KEY" {
		t.Errorf("prune 対象 = %v, want [STALE_KEY]", got)
	}
}

func TestComputeVercelPrune_SkipsSystemAndIntegration(t *testing.T) {
	envs := []vercelEnv{
		{ID: "id1", Key: "VERCEL_URL", System: true},                             // システム変数
		{ID: "id2", Key: "NX_DAEMON", Type: "system"},                            // type=system
		{ID: "id3", Key: "BLOB_READ_WRITE_TOKEN", ConfigurationID: "icfg_12345"}, // インテグレーション（Blob Store 等）
		{ID: "id4", Key: "STALE_KEY"},
	}
	got := computeVercelPrune(envs, keepFunc())
	if len(got) != 1 || got[0].Key != "STALE_KEY" {
		t.Errorf("prune 対象 = %v, want [STALE_KEY]（system / インテグレーション由来は除外）", got)
	}
}

func TestComputeVercelPrune_MultipleRecordsSameKey(t *testing.T) {
	// 同一 key の複数レコード（environments 別）はすべて削除対象になる
	envs := []vercelEnv{
		{ID: "id1", Key: "STALE_KEY", Target: []string{"production"}},
		{ID: "id2", Key: "STALE_KEY", Target: []string{"preview"}},
	}
	got := computeVercelPrune(envs, keepFunc())
	if len(got) != 2 {
		t.Errorf("prune 対象件数 = %d, want 2（レコード単位で削除）", len(got))
	}
}

func TestComputeVercelPrune_SkipsEmptyID(t *testing.T) {
	// ID が空のレコードは削除 URL を組み立てられないため安全側に倒して除外する
	envs := []vercelEnv{
		{ID: "", Key: "NO_ID_KEY"},
		{ID: "id1", Key: "STALE_KEY"},
	}
	got := computeVercelPrune(envs, keepFunc())
	if len(got) != 1 || got[0].Key != "STALE_KEY" {
		t.Errorf("prune 対象 = %v, want [STALE_KEY]（ID 空は除外）", got)
	}
}

func TestComputeVercelPrune_AllDefined_Empty(t *testing.T) {
	envs := []vercelEnv{{ID: "id1", Key: "FOO"}}
	if got := computeVercelPrune(envs, keepFunc("FOO")); len(got) != 0 {
		t.Errorf("prune 対象 = %v, want 空", got)
	}
}

// --- deleteVercelEnvs のテスト ---

func TestDeleteVercelEnvs_SendsDeleteToCorrectURL(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotAuth string
	srv := newVercelTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})
	withVercelAPIBase(t, srv.URL)

	envs := []vercelEnv{{ID: "env-id-123", Key: "STALE_KEY"}}
	ok, ng := deleteVercelEnvs(&http.Client{}, "test-token", "pid", "team-1", envs)
	if ok != 1 || ng != 0 {
		t.Errorf("ok=%d ng=%d, want ok=1 ng=0", ok, ng)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("メソッド = %q, want DELETE", gotMethod)
	}
	if want := "/v9/projects/pid/env/env-id-123"; gotPath != want {
		t.Errorf("パス = %q, want %q", gotPath, want)
	}
	if !strings.Contains(gotQuery, "teamId=team-1") {
		t.Errorf("クエリ = %q, want teamId=team-1 を含む", gotQuery)
	}
	if want := "Bearer test-token"; gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}
}

func TestDeleteVercelEnvs_PartialFailure_ContinuesAndCounts(t *testing.T) {
	var count int
	srv := newVercelTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	withVercelAPIBase(t, srv.URL)

	envs := []vercelEnv{
		{ID: "id1", Key: "FAIL_KEY"},
		{ID: "id2", Key: "OK_KEY"},
	}
	ok, ng := deleteVercelEnvs(&http.Client{}, "token", "pid", "", envs)
	if ok != 1 || ng != 1 {
		t.Errorf("ok=%d ng=%d, want ok=1 ng=1（1 件失敗しても残りを削除し集計する）", ok, ng)
	}
	if count != 2 {
		t.Errorf("DELETE 回数 = %d, want 2", count)
	}
}
