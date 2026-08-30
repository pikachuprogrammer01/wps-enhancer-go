package license

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wps-enhancer-go/internal/errs"
)

// timeoutContext 带超时的 context（命令执行防护）。
func timeoutContext(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// sha256Hex 计算 SHA-256 hex（设备指纹输出格式）。
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Stored 本地持久化的激活状态（契约 §2.3 存储格式）。
type Stored struct {
	Key         string   `json:"key"`         // 完整激活码
	Payload     *Payload `json:"payload"`     // 载荷原样
	ActivatedAt int64    `json:"activatedAt"` // 激活时间戳（ms）
}

// Store 激活状态持久化（与 settings.json 分离，契约 §2.3）。
type Store struct {
	path string
}

// NewStore 创建持久化存储（路径：<configDir>/license.json）。
func NewStore(configDir string) *Store {
	return &Store{path: filepath.Join(configDir, "license.json")}
}

// Load 读取激活状态；文件缺失/损坏返回 nil（视为未激活，不崩溃）。
func (s *Store) Load() *Stored {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return nil
	}
	var stored Stored
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil
	}
	return &stored
}

// Save 写入激活状态（原子写入）。
func (s *Store) Save(stored *Stored) error {
	body, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: %v", errs.ErrSettings, err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("%w: %v", errs.ErrSettings, err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return fmt.Errorf("%w: %v", errs.ErrSettings, err)
	}
	return os.Rename(tmp, s.path)
}

// Clear 清除本地激活状态（解绑后调用）。
func (s *Store) Clear() error {
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%w: %v", errs.ErrSettings, err)
	}
	return nil
}

// State 激活状态内存判定（业务层唯一入口 IsPro；启动时 Load 一次，运行期走内存）。
type State struct {
	mu     sync.RWMutex
	store  *Store
	stored *Stored // nil = 未激活
}

// NewState 创建状态判定（自动从持久化加载）。
func NewState(store *Store) *State {
	return &State{store: store, stored: store.Load()}
}

// IsPro 业务层唯一入口：有本地记录且未过期 → Pro。
func (s *State) IsPro() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.stored == nil || s.stored.Payload == nil {
		return false
	}
	return !s.stored.Payload.IsExpired()
}

// Payload 返回当前激活码载荷（nil=未激活；UI 展示用）。
func (s *State) Payload() *Payload {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.stored == nil {
		return nil
	}
	return s.stored.Payload
}

// Stored 返回当前本地激活记录（nil=未激活；解绑流程取 key 用）。
func (s *State) Stored() *Stored {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stored
}

// Activate 激活成功：持久化 + 更新内存（契约 §2.2 200 分支）。
func (s *State) Activate(key string, payload *Payload) error {
	stored := &Stored{
		Key:         key,
		Payload:     payload,
		ActivatedAt: time.Now().UnixMilli(),
	}
	if err := s.store.Save(stored); err != nil {
		return err
	}
	s.mu.Lock()
	s.stored = stored
	s.mu.Unlock()
	return nil
}

// Deactivate 解绑成功：清除本地状态（契约 §4 200 分支）。
func (s *State) Deactivate() error {
	if err := s.store.Clear(); err != nil {
		return err
	}
	s.mu.Lock()
	s.stored = nil
	s.mu.Unlock()
	return nil
}
