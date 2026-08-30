package updater

import "testing"

func TestParseScutilProxy(t *testing.T) {
	sample := `<dictionary> {
  HTTPSEnable : 1
  HTTPSPort : 7890
  HTTPSProxy : 127.0.0.1
  HTTPEnable : 1
  HTTPPort : 8080
  HTTPProxy : 10.0.0.2
  SOCKSEnable : 0
}`
	if got := darwinSystemProxyFromOutput(sample); got != "http://127.0.0.1:7890" {
		t.Errorf("HTTPS 优先: got %q", got)
	}
	httpOnly := `<dictionary> {
  HTTPEnable : 1
  HTTPPort : 8080
  HTTPProxy : 10.0.0.2
}`
	if got := darwinSystemProxyFromOutput(httpOnly); got != "http://10.0.0.2:8080" {
		t.Errorf("HTTP 回退: got %q", got)
	}
	noHost := `<dictionary> {
  HTTPEnable : 1
  HTTPPort : 8080
}`
	if got := darwinSystemProxyFromOutput(noHost); got != "" {
		t.Errorf("无主机应返回空: got %q", got)
	}
	noPort := `<dictionary> {
  HTTPProxy : 10.0.0.2
}`
	if got := darwinSystemProxyFromOutput(noPort); got != "http://10.0.0.2:80" {
		t.Errorf("缺端口默认 80: got %q", got)
	}
}

func TestParseWindowsRegProxy(t *testing.T) {
	enableOn := "    ProxyEnable    REG_DWORD    0x1"
	enableOff := "    ProxyEnable    REG_DWORD    0x0"
	serverSimple := "    ProxyServer    REG_SZ    127.0.0.1:7890"
	serverMap := "    ProxyServer    REG_SZ    http=127.0.0.1:8080;https=127.0.0.1:7891"

	if got := windowsSystemProxyFromOutput(enableOff, serverSimple); got != "" {
		t.Errorf("未启用应返回空: got %q", got)
	}
	if got := windowsSystemProxyFromOutput(enableOn, serverSimple); got != "http://127.0.0.1:7890" {
		t.Errorf("裸地址: got %q", got)
	}
	if got := windowsSystemProxyFromOutput(enableOn, serverMap); got != "http://127.0.0.1:7891" {
		t.Errorf("https 段优先: got %q", got)
	}
}

func TestNormalizeProxy(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:7890":   "http://127.0.0.1:7890",
		"http://1.2.3.4":   "http://1.2.3.4",
		"https://1.2.3.4":  "https://1.2.3.4",
		"7890":             "",
	}
	for in, want := range cases {
		if got := normalizeProxy(in); got != want {
			t.Errorf("normalizeProxy(%q) = %q, want %q", in, got, want)
		}
	}
}
