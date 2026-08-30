package license

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestStateMachine 状态机：激活→Pro→过期→解绑（持久化重启）。
func TestStateMachine(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	state := NewState(store)
	if state.IsPro() {
		t.Error("初始应为非 Pro")
	}

	future := &Payload{ID: "s1", Version: 1, Type: "pro", ExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli()}
	if err := state.Activate("WPS-TEST", future); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if !state.IsPro() {
		t.Error("激活后应为 Pro")
	}
	// 持久化重启（新 State 实例）
	state2 := NewState(store)
	if !state2.IsPro() {
		t.Error("重启后应保持 Pro（持久化）")
	}

	// 过期
	expired := &Payload{ID: "s2", Version: 1, Type: "pro", ExpiresAt: time.Now().Add(-1 * time.Hour).UnixMilli()}
	_ = state.Activate("WPS-TEST2", expired)
	if state.IsPro() {
		t.Error("过期后不应为 Pro")
	}

	// 解绑
	if err := state.Deactivate(); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if state.IsPro() {
		t.Error("解绑后不应为 Pro")
	}
	if store.Load() != nil {
		t.Error("解绑后持久化应清除")
	}
}

// TestFingerprint 设备指纹（本机实测：非空且 64 位 hex）。
func TestFingerprint(t *testing.T) {
	fp, err := Fingerprint()
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if len(fp) != 64 {
		t.Errorf("指纹应为 64 位 hex，实际 %d 位", len(fp))
	}
	fp2, _ := Fingerprint()
	if fp != fp2 {
		t.Error("同机两次指纹应一致")
	}
}

// mockPlatform 模拟 LicenseHub 平台（契约 §4 响应结构）。
func mockPlatform(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)
	return srv
}

// TestActivateFlowOK 激活流程 200：持久化 + Pro（契约 §10 激活正例）。
func TestActivateFlowOK(t *testing.T) {
	priv := testKeyPair(t)
	payload := &Payload{ID: "f1", Version: 1, Type: "pro",
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour).UnixMilli()}
	key := buildTestKey(t, priv, payload, "WPS-")

	srv := mockPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/activate") {
			t.Errorf("路径应为 /api/activate: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok": true, "expiresAt": 1755000000000}`)
	})

	state := NewState(NewStore(t.TempDir()))
	client := &Client{http: srv.Client(), apiURL: srv.URL}
	result, err := ActivateFlow(key, "fp-test", client, state)
	if err != nil {
		t.Fatalf("ActivateFlow: %v", err)
	}
	if !result.OK || result.PendingOnline {
		t.Errorf("激活应成功: %+v", result)
	}
	if !state.IsPro() {
		t.Error("激活后应为 Pro")
	}
}

// TestActivateFlowConflict 激活流程 409 ALREADY_ACTIVATED（契约 §10 他机冲突）。
func TestActivateFlowConflict(t *testing.T) {
	priv := testKeyPair(t)
	payload := &Payload{ID: "f2", Version: 1, Type: "pro",
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour).UnixMilli()}
	key := buildTestKey(t, priv, payload, "WPS-")

	srv := mockPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"ok": false, "code": "ALREADY_ACTIVATED"}`)
	})

	state := NewState(NewStore(t.TempDir()))
	client := &Client{http: srv.Client(), apiURL: srv.URL}
	result, err := ActivateFlow(key, "fp-other", client, state)
	if err != nil {
		t.Fatalf("ActivateFlow: %v", err)
	}
	if result.OK || result.Code != "ALREADY_ACTIVATED" {
		t.Errorf("应返回 ALREADY_ACTIVATED: %+v", result)
	}
	if state.IsPro() {
		t.Error("409 不应进入 Pro")
	}
}

// TestActivateFlowHybridOffline 网络失败 hybrid 容错：本地验签通过 → 暂放行（契约 §7）。
func TestActivateFlowHybridOffline(t *testing.T) {
	priv := testKeyPair(t)
	payload := &Payload{ID: "f3", Version: 1, Type: "pro",
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour).UnixMilli()}
	key := buildTestKey(t, priv, payload, "WPS-")

	// 平台不可达（关闭的 server）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	state := NewState(NewStore(t.TempDir()))
	client := &Client{http: &http.Client{Timeout: time.Second}, apiURL: url}
	result, err := ActivateFlow(key, "fp-x", client, state)
	if err != nil {
		t.Fatalf("ActivateFlow: %v", err)
	}
	if !result.OK || !result.PendingOnline {
		t.Errorf("网络失败应 hybrid 放行: %+v", result)
	}
	if !state.IsPro() {
		t.Error("hybrid 放行后应进入激活态（离线可用）")
	}
}

// TestActivateFlowExpired 激活流程：本地过期检查拦截（契约 §7 ②，不发网络请求）。
func TestActivateFlowExpired(t *testing.T) {
	priv := testKeyPair(t)
	payload := &Payload{ID: "f5", Version: 1, Type: "pro",
		ExpiresAt: time.Now().Add(-1 * time.Hour).UnixMilli()}
	key := buildTestKey(t, priv, payload, "WPS-")

	srv := mockPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("过期码不应发网络请求（本地拦截）")
	})
	state := NewState(NewStore(t.TempDir()))
	client := &Client{http: srv.Client(), apiURL: srv.URL}
	result, err := ActivateFlow(key, "fp", client, state)
	if err != nil {
		t.Fatalf("ActivateFlow: %v", err)
	}
	if result.OK || result.Code != "EXPIRED" {
		t.Errorf("应返回 EXPIRED: %+v", result)
	}
}

// TestActivateFlowRevoked 激活流程：平台返回 403 REVOKED → 透传 code（契约 §4）。
func TestActivateFlowRevoked(t *testing.T) {
	priv := testKeyPair(t)
	payload := &Payload{ID: "f6", Version: 1, Type: "pro",
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour).UnixMilli()}
	key := buildTestKey(t, priv, payload, "WPS-")

	srv := mockPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"ok": false, "code": "REVOKED"}`)
	})
	state := NewState(NewStore(t.TempDir()))
	client := &Client{http: srv.Client(), apiURL: srv.URL}
	result, err := ActivateFlow(key, "fp", client, state)
	if err != nil {
		t.Fatalf("ActivateFlow: %v", err)
	}
	if result.OK || result.Code != "REVOKED" {
		t.Errorf("应返回 REVOKED: %+v", result)
	}
	if state.IsPro() {
		t.Error("REVOKED 不应进入 Pro")
	}
}

// TestDeactivateFingerprintMismatch 解绑：平台返回 403 FINGERPRINT_MISMATCH（契约 §5）。
func TestDeactivateFingerprintMismatch(t *testing.T) {
	srv := mockPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"ok": false, "code": "FINGERPRINT_MISMATCH"}`)
	})
	client := &Client{http: srv.Client(), apiURL: srv.URL}
	err := client.Deactivate("WPS-fake-key", "fp-other-device")
	if err == nil {
		t.Fatal("解绑应返回错误")
	}
	if !strings.Contains(err.Error(), "FINGERPRINT_MISMATCH") {
		t.Errorf("错误应包含 FINGERPRINT_MISMATCH: %v", err)
	}
}

// TestDeactivateRateLimited 解绑：平台返回 429 RATE_LIMITED（解绑防滥用，契约 §5）。
func TestDeactivateRateLimited(t *testing.T) {
	srv := mockPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"ok": false, "code": "RATE_LIMITED"}`)
	})
	client := &Client{http: srv.Client(), apiURL: srv.URL}
	err := client.Deactivate("WPS-fake-key", "fp")
	if err == nil || !strings.Contains(err.Error(), "RATE_LIMITED") {
		t.Errorf("应返回 RATE_LIMITED: %v", err)
	}
}

// TestActivateFlowInvalid 激活流程：非法码本地即拒（不发网络请求，契约 §7 ①）。
func TestActivateFlowInvalid(t *testing.T) {
	srv := mockPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("非法码不应发网络请求")
	})
	state := NewState(NewStore(t.TempDir()))
	client := &Client{http: srv.Client(), apiURL: srv.URL}
	result, err := ActivateFlow("WPS-not-a-valid-key", "fp", client, state)
	if err != nil {
		t.Fatalf("ActivateFlow: %v", err)
	}
	if result.OK || result.Code != "INVALID_KEY" {
		t.Errorf("应返回 INVALID_KEY: %+v", result)
	}
	if state.IsPro() {
		t.Error("非法码不应进入 Pro")
	}
}

// TestActivateFlowTypeRejected 激活流程：buyout 拒绝（产品策略）。
func TestActivateFlowTypeRejected(t *testing.T) {
	priv := testKeyPair(t)
	payload := &Payload{ID: "f4", Version: 1, Type: "buyout",
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour).UnixMilli()}
	key := buildTestKey(t, priv, payload, "WPS-")

	srv := mockPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("buyout 码不应发网络请求")
	})
	state := NewState(NewStore(t.TempDir()))
	client := &Client{http: srv.Client(), apiURL: srv.URL}
	result, err := ActivateFlow(key, "fp", client, state)
	if err != nil {
		t.Fatalf("ActivateFlow: %v", err)
	}
	if result.OK || result.Code != "TYPE_NOT_SUPPORTED" {
		t.Errorf("应返回 TYPE_NOT_SUPPORTED: %+v", result)
	}
}
