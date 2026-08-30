package license

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"
	"time"
)

// testKeyPair 生成测试密钥对并注入公钥（模拟平台导出公钥内置客户端）；defer 恢复原公钥。
func testKeyPair(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	priv, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("generateTestKeyPair: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal pub: %v", err)
	}
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
	old := PublicKeysPEM
	PublicKeysPEM = []string{pemStr}
	t.Cleanup(func() { PublicKeysPEM = old })
	return priv
}

// buildTestKey 构造测试激活码（契约格式）：
// {prefix}-分组5(base64(payload)).分组5(base64(签名))；签名对象 = payloadB64 字符串字节。
func buildTestKey(t *testing.T, priv *rsa.PrivateKey, payload *Payload, prefix string) string {
	t.Helper()
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	payloadB64 := base64.StdEncoding.EncodeToString(payloadBytes)
	sig, err := signPayload(priv, payloadB64)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	enc := payloadB64 + "." + base64.StdEncoding.EncodeToString(sig)
	// 每 5 字符一组加 '-'（可读性分组）
	var grouped []byte
	for i := 0; i < len(enc); i += 5 {
		end := i + 5
		if end > len(enc) {
			end = len(enc)
		}
		grouped = append(grouped, enc[i:end]...)
		if end < len(enc) {
			grouped = append(grouped, '-')
		}
	}
	return prefix + string(grouped)
}

// TestParseKey 激活码解析：前缀匹配（最长优先）/分组符/载荷字段。
func TestParseKey(t *testing.T) {
	priv := testKeyPair(t)
	payload := &Payload{
		ID: "test-001", Version: 1, Type: "pro",
		ExpiresAt:  time.Now().Add(30 * 24 * time.Hour).UnixMilli(),
		MaxDevices: 1, BatchID: "batch-001",
		IssuedAt: time.Now().UnixMilli(),
	}
	key := buildTestKey(t, priv, payload, "WPS-")
	got, payloadB64, sig, err := ParseKey(key)
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	if got.ID != "test-001" || got.Type != "pro" || got.Version != 1 {
		t.Errorf("载荷解析异常: %+v", got)
	}
	if payloadB64 == "" || len(sig) == 0 {
		t.Error("payloadB64 与签名应为非空")
	}
	if got.IsExpired() {
		t.Error("30 天后过期不应判定为过期")
	}
	// TF- 前缀（多前缀集合）
	keyTF := buildTestKey(t, priv, payload, "TF-")
	if p, _, _, err := ParseKey(keyTF); err != nil || p.ID != "test-001" {
		t.Errorf("TF- 前缀解析失败: %v", err)
	}
	// 未知前缀
	if _, _, _, err := ParseKey("XX-abc"); err == nil {
		t.Error("未知前缀应报错")
	}
	// 格式错误
	if _, _, _, err := ParseKey("WPS-abc"); err == nil {
		t.Error("缺分隔符的码应报错")
	}
	// 过期载荷
	expired := &Payload{ID: "x", Version: 1, Type: "pro", ExpiresAt: time.Now().Add(-1 * time.Hour).UnixMilli()}
	expKey := buildTestKey(t, priv, expired, "WPS-")
	expGot, _, _, err := ParseKey(expKey)
	if err != nil {
		t.Fatalf("ParseKey expired: %v", err)
	}
	if !expGot.IsExpired() {
		t.Error("过期载荷应判定为过期")
	}
}

// TestParseAndVerify 解析+验签组合：正确通过、篡改失败（契约 §10 验签正/反例）。
func TestParseAndVerify(t *testing.T) {
	priv := testKeyPair(t)
	payload := &Payload{ID: "t1", Version: 1, Type: "pro", ExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli()}
	key := buildTestKey(t, priv, payload, "WPS-")

	got, _, err := ParseAndVerify(key)
	if err != nil {
		t.Fatalf("ParseAndVerify 正例失败: %v", err)
	}
	if got.ID != "t1" {
		t.Errorf("payload 异常: %+v", got)
	}
	// 反例：篡改激活码任意一个字符 → 验签失败（跳过前缀与分组符/分隔符，替换为与原字符不同的字符）
	_, _, origSig, _ := ParseKey(key)
	for i := len("WPS-"); i < len(key); i++ {
		if key[i] == '-' || key[i] == '.' {
			continue
		}
		replacement := byte('A')
		if key[i] == 'A' {
			replacement = 'B'
		}
		tampered := key[:i] + string(replacement) + key[i+1:]
		// 跳过篡改无效的位置：base64 填充区相邻字符的低位被丢弃，
		// 篡改后解码字节不变（如 "cA==" 的 A/B），验签自然通过，不算反例
		_, _, newSig, err := ParseKey(tampered)
		if err != nil || bytes.Equal(newSig, origSig) {
			continue
		}
		if _, _, err := ParseAndVerify(tampered); err == nil {
			t.Errorf("篡改位置 %d 应验签失败", i)
			break
		}
	}
	// 类型校验：buyout/trial 拒绝（产品策略）
	for _, typ := range []string{"buyout", "trial"} {
		p := &Payload{ID: "t2", Version: 1, Type: typ, ExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli()}
		if err := ValidateType(p); err == nil {
			t.Errorf("type=%s 应被拒绝", typ)
		}
	}
	if err := ValidateType(&Payload{ID: "t3", Version: 1, Type: "pro"}); err != nil {
		t.Errorf("type=pro 应通过: %v", err)
	}
}

// TestVerifyLocal 本地验签：正确签名通过、篡改载荷失败（签名对象 = payloadB64 字符串）。
func TestVerifyLocal(t *testing.T) {
	priv := testKeyPair(t)
	payloadB64 := base64.StdEncoding.EncodeToString([]byte(`{"id":"v1","type":"pro"}`))
	sig, err := signPayload(priv, payloadB64)
	if err != nil {
		t.Fatalf("signPayload: %v", err)
	}
	if !VerifyLocal(payloadB64, sig) {
		t.Error("正确签名应通过")
	}
	// 篡改 payloadB64（等价篡改激活码）→ 验签失败
	if VerifyLocal(payloadB64+"A", sig) {
		t.Error("篡改 payloadB64 应验签失败")
	}
}
