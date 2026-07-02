package gcp

import (
	"context"
	"strings"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"

	"github.com/ptyhard/env-sync/internal/provider"
)

// keepFunc はテスト用に Options.PruneKeep 相当の保持判定を作るヘルパー。
func keepFunc(definedKeys ...string) func(string) bool {
	opts := provider.Options{DefinedKeys: definedKeys}
	return opts.PruneKeep()
}

// --- computeGCPPrune のテスト ---

func TestComputeGCPPrune(t *testing.T) {
	secrets := []*secretmanagerpb.Secret{
		{Name: "projects/12345/secrets/DEFINED_KEY"},
		{Name: "projects/12345/secrets/STALE_KEY"},
	}
	got := computeGCPPrune(secrets, keepFunc("DEFINED_KEY"))
	if len(got) != 1 || got[0] != "STALE_KEY" {
		t.Errorf("prune 対象 = %v, want [STALE_KEY]", got)
	}
}

func TestComputeGCPPrune_AllDefined_Empty(t *testing.T) {
	secrets := []*secretmanagerpb.Secret{
		{Name: "projects/12345/secrets/FOO"},
	}
	if got := computeGCPPrune(secrets, keepFunc("FOO")); len(got) != 0 {
		t.Errorf("prune 対象 = %v, want 空", got)
	}
}

// --- Sync の prune フローのテスト ---

func TestSync_Prune_DeletesUndefinedManagedSecrets(t *testing.T) {
	t.Setenv("GCP_PROJECT_ID", "test-project")

	var listFilter string
	var deletedNames []string

	mock := &mockClient{
		getSecret: func(_ context.Context, req *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error) {
			return nil, notFoundErr()
		},
		createSecret: func(_ context.Context, req *secretmanagerpb.CreateSecretRequest) (*secretmanagerpb.Secret, error) {
			return &secretmanagerpb.Secret{}, nil
		},
		addSecretVersion: func(_ context.Context, _ *secretmanagerpb.AddSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
			return &secretmanagerpb.SecretVersion{}, nil
		},
		listSecrets: func(_ context.Context, req *secretmanagerpb.ListSecretsRequest) ([]*secretmanagerpb.Secret, error) {
			listFilter = req.Filter
			return []*secretmanagerpb.Secret{
				{Name: "projects/12345/secrets/MY_KEY"},    // 定義済み → 保持
				{Name: "projects/12345/secrets/STALE_KEY"}, // 未定義 → 削除
			}, nil
		},
		deleteSecret: func(_ context.Context, req *secretmanagerpb.DeleteSecretRequest) error {
			deletedNames = append(deletedNames, req.Name)
			return nil
		},
	}
	restore := SwapClientFunc(func(_ context.Context) (secretManagerClient, error) { return mock, nil })
	defer restore()

	p := &gcpProvider{}
	opts := provider.Options{Yes: true, Prune: true, DefinedKeys: []string{"MY_KEY"}}
	if err := p.Sync(opts, []provider.Entry{
		{Key: "MY_KEY", Value: "v", Secret: true},
	}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if !strings.Contains(listFilter, "managed-by") || !strings.Contains(listFilter, "env-sync") {
		t.Errorf("ListSecrets の filter = %q, want labels.managed-by=env-sync（管理外 Secret の誤削除防止）", listFilter)
	}
	if len(deletedNames) != 1 || deletedNames[0] != "projects/test-project/secrets/STALE_KEY" {
		t.Errorf("deletedNames = %v, want [projects/test-project/secrets/STALE_KEY]", deletedNames)
	}
}

func TestSync_PruneDisabled_NoListNoDelete(t *testing.T) {
	t.Setenv("GCP_PROJECT_ID", "test-project")

	listCalled, deleteCalled := false, false
	mock := &mockClient{
		getSecret: func(_ context.Context, req *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error) {
			return nil, notFoundErr()
		},
		createSecret: func(_ context.Context, req *secretmanagerpb.CreateSecretRequest) (*secretmanagerpb.Secret, error) {
			return &secretmanagerpb.Secret{}, nil
		},
		addSecretVersion: func(_ context.Context, _ *secretmanagerpb.AddSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
			return &secretmanagerpb.SecretVersion{}, nil
		},
		listSecrets: func(_ context.Context, _ *secretmanagerpb.ListSecretsRequest) ([]*secretmanagerpb.Secret, error) {
			listCalled = true
			return nil, nil
		},
		deleteSecret: func(_ context.Context, _ *secretmanagerpb.DeleteSecretRequest) error {
			deleteCalled = true
			return nil
		},
	}
	restore := SwapClientFunc(func(_ context.Context) (secretManagerClient, error) { return mock, nil })
	defer restore()

	p := &gcpProvider{}
	if err := p.Sync(provider.Options{Yes: true}, []provider.Entry{
		{Key: "MY_KEY", Value: "v", Secret: true},
	}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if listCalled || deleteCalled {
		t.Error("prune 無効時に ListSecrets / DeleteSecret が呼ばれるべきではない")
	}
}
