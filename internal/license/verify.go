package license

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"wps-enhancer-go/internal/errs"
)

// PublicKeysPEM 内置公钥列表（PEM SPKI 格式，契约 §3：多公钥逐个尝试，任一通过即可）。
// LicenseHub 平台 WPS 产品公钥（2026-08-16 导出；轮换时新公钥追加到列表）。
var PublicKeysPEM = []string{
	`-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAuQ1nnuDoPdZD7LTAiH6S
+ty4dbNh6VJmifcxRugh9Qr7eb7suwKxxx7lNxnc4/rnoU8mg2xL4BuPd+0SaP7/
izV02lJ04b5p1k2MgNe+IsDvy+0BTEoiC6pIEu6MejWRoSEY2e+vBihTk2/hOXB9
5g5pxoFQxs0BrJhOhrKynyzLvAnd+7HICwJyb+MH7s8v+YejNhSHBnJRISp4K3fQ
tWGXvQYaacuQ867kzX+W1JADIf9fRQvtd5HjM41aPitA7S6ICn+voxpI3ejCyPdG
V888NTVCTRD+xd8pfEH5G9ZGsLYcdapfr2hCd3UiuI+7N64apmfMmxXH0D6YQe6Z
7QIDAQAB
-----END PUBLIC KEY-----`,
}

// SignatureAlgorithm 验签算法（产品档案 signature_algorithm："rsa256" | "ed25519"）。
const SignatureAlgorithm = "rsa256"

// VerifyLocal 用内置公钥逐个验签（契约 §3）：
// 签名对象 = payloadB64 字符串本身的字节（即 base64(UTF-8(JSON(payload)))），
// 算法按 SignatureAlgorithm（rsa256 = RSA-SHA256 PKCS1v15；ed25519 = 标准 Ed25519）。
// 任一公钥验签通过即成功；任何异常一律视为验签失败。
func VerifyLocal(payloadB64 string, signature []byte) bool {
	for _, pemStr := range PublicKeysPEM {
		block, _ := pem.Decode([]byte(pemStr))
		if block == nil {
			continue
		}
		if SignatureAlgorithm == "ed25519" {
			pub, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				continue
			}
			edPub, ok := pub.(ed25519.PublicKey)
			if !ok {
				continue
			}
			if ed25519.Verify(edPub, []byte(payloadB64), signature) {
				return true
			}
			continue
		}
		// rsa256：RSA-SHA256（PKCS1v15）
		pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			// 兼容 PKCS1 格式
			pub, pkcs1Err := x509.ParsePKCS1PublicKey(block.Bytes)
			if pkcs1Err != nil {
				continue
			}
			if rsaVerify(pub, []byte(payloadB64), signature) {
				return true
			}
			continue
		}
		pub, ok := pubAny.(*rsa.PublicKey)
		if !ok {
			continue
		}
		if rsaVerify(pub, []byte(payloadB64), signature) {
			return true
		}
	}
	return false
}

// rsaVerify RSA-SHA256 PKCS1v15 验签。
func rsaVerify(pub *rsa.PublicKey, payloadB64, signature []byte) bool {
	hashed := sha256.Sum256(payloadB64)
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, hashed[:], signature) == nil
}

// ParseAndVerify 解析 + 验签组合（契约 parseAndVerify 等价）：
// 前缀匹配 → 分割 → 逐个公钥验签 → 解析 payload。
// 验签失败/格式错误返回 ErrDataProcess（对应 INVALID_KEY）。
func ParseAndVerify(rawKey string) (*Payload, string, error) {
	payload, payloadB64, sig, err := ParseKey(rawKey)
	if err != nil {
		return nil, "", err
	}
	if !VerifyLocal(payloadB64, sig) {
		return nil, "", fmt.Errorf("%w: 激活码无效（验签失败）", errs.ErrDataProcess)
	}
	return payload, payloadB64, nil
}

// generateTestKeyPair 生成测试密钥对（仅供测试）。
func generateTestKeyPair() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// signPayload 用私钥对 payloadB64 字符串签名（测试辅助，模拟平台签发）。
func signPayload(priv *rsa.PrivateKey, payloadB64 string) ([]byte, error) {
	hashed := sha256.Sum256([]byte(payloadB64))
	return rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hashed[:])
}
