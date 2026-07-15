package mailbox

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

const (
	oauthStateBytes      = 32
	defaultOAuthStateTTL = 10 * time.Minute
	maxOAuthStates       = 10000
)

var (
	errInvalidOAuthStateInput = errors.New("invalid OAuth state input")
	errOAuthStateStoreFull    = errors.New("OAuth state store is full")
)

// OAuthState 是服务端保存的 OAuth 授权请求元数据。state 本身是随机、
// 不透明的字符串，不包含用户 ID、JWT 或其他可伪造信息。
type OAuthState struct {
	UserID   int64
	Provider string
}

// OAuthStateStore 保存并一次性消费 OAuth state，防止 callback 被伪造或重放。
// 生产多实例部署时应以 Redis/数据库的原子 get-and-delete + TTL 实现替换内存实现。
type OAuthStateStore interface {
	Create(userID int64, provider, browserBinding string) (string, error)
	Consume(state, provider, browserBinding string) (OAuthState, bool)
}

type storedOAuthState struct {
	value              OAuthState
	browserBindingHash [sha256.Size]byte
	expiresAt          time.Time
}

// InMemoryOAuthStateStore 是单进程使用的一次性 OAuth state 存储。
// 服务重启会让尚未完成的授权请求失效；用户重新发起授权即可。
type InMemoryOAuthStateStore struct {
	mu     sync.Mutex
	states map[string]storedOAuthState
	now    func() time.Time
	ttl    time.Duration
}

func NewInMemoryOAuthStateStore() *InMemoryOAuthStateStore {
	return newInMemoryOAuthStateStore(time.Now, defaultOAuthStateTTL)
}

func newInMemoryOAuthStateStore(now func() time.Time, ttl time.Duration) *InMemoryOAuthStateStore {
	if now == nil {
		now = time.Now
	}
	if ttl <= 0 {
		ttl = defaultOAuthStateTTL
	}
	return &InMemoryOAuthStateStore{
		states: make(map[string]storedOAuthState),
		now:    now,
		ttl:    ttl,
	}
}

// Create 生成 256-bit 的随机 state，并保存十分钟。每条 state 都绑定到浏览器的
// HttpOnly transaction cookie；callback 必须携带同一个 cookie 才能消费该 state。
func (s *InMemoryOAuthStateStore) Create(userID int64, provider, browserBinding string) (string, error) {
	if userID <= 0 || provider == "" || browserBinding == "" {
		return "", errInvalidOAuthStateInput
	}
	browserBindingHash := sha256.Sum256([]byte(browserBinding))

	for attempt := 0; attempt < 3; attempt++ {
		bytes := make([]byte, oauthStateBytes)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		state := base64.RawURLEncoding.EncodeToString(bytes)

		now := s.now()
		s.mu.Lock()
		s.removeExpiredLocked(now)
		if len(s.states) >= maxOAuthStates {
			s.mu.Unlock()
			return "", errOAuthStateStoreFull
		}
		if _, exists := s.states[state]; !exists {
			s.states[state] = storedOAuthState{
				value: OAuthState{
					UserID:   userID,
					Provider: provider,
				},
				browserBindingHash: browserBindingHash,
				expiresAt:          now.Add(s.ttl),
			}
			s.mu.Unlock()
			return state, nil
		}
		s.mu.Unlock()
	}

	return "", errors.New("unable to generate unique OAuth state")
}

// Consume 原子地校验并删除 state。只有 provider 和浏览器 transaction cookie
// 一致、未过期且此前未被消费的 state 才会返回 true，从而阻止 callback 伪造和重放。
func (s *InMemoryOAuthStateStore) Consume(state, provider, browserBinding string) (OAuthState, bool) {
	if state == "" || provider == "" || browserBinding == "" {
		return OAuthState{}, false
	}
	browserBindingHash := sha256.Sum256([]byte(browserBinding))

	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()

	s.removeExpiredLocked(now)
	stored, ok := s.states[state]
	if !ok || stored.value.Provider != provider || stored.value.UserID <= 0 ||
		subtle.ConstantTimeCompare(stored.browserBindingHash[:], browserBindingHash[:]) != 1 {
		return OAuthState{}, false
	}

	delete(s.states, state)
	return stored.value, true
}

func (s *InMemoryOAuthStateStore) removeExpiredLocked(now time.Time) {
	for state, stored := range s.states {
		if !stored.expiresAt.After(now) {
			delete(s.states, state)
		}
	}
}
