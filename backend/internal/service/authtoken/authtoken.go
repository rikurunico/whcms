// Package authtoken is the token core of the auth module (CONTRACTS.md §3):
// HS256 access JWTs (TTL 15m; claims sub/role/cid/jti) and opaque refresh
// tokens stored in Redis under whmcs:refresh:<token> (TTL 30d, rotated on
// use, revocable). The full auth module builds on this package.
package authtoken

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Token lifetimes.
const (
	AccessTTL  = 15 * time.Minute
	RefreshTTL = 30 * 24 * time.Hour
)

// AccessClaims is the decoded identity of an access token.
type AccessClaims struct {
	UserID   int64  // "sub"
	Role     string // "role": admin|staff|client
	ClientID int64  // "cid": 0 for staff/admin
	JTI      string // "jti"
}

// jwtClaims is the wire format.
type jwtClaims struct {
	Role string `json:"role"`
	CID  int64  `json:"cid"`
	jwt.RegisteredClaims
}

// Manager issues/parses access tokens and manages refresh tokens.
type Manager struct {
	secret []byte
	rdb    *redis.Client
	clock  ports.Clock
}

// New builds a Manager. rdb may be nil when only access-token operations are
// used (e.g. handler tests).
func New(secret string, rdb *redis.Client, clock ports.Clock) *Manager {
	return &Manager{secret: []byte(secret), rdb: rdb, clock: clock}
}

// IssueAccess creates a signed HS256 access JWT. Returns the token and its jti.
func (m *Manager) IssueAccess(userID int64, role string, clientID int64) (token, jti string, err error) {
	now := m.clock.Now()
	jti = uuid.NewString()
	claims := jwtClaims{
		Role: role,
		CID:  clientID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)),
		},
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", "", fmt.Errorf("authtoken: sign: %w", err)
	}
	return token, jti, nil
}

// ParseAccess validates the token (signature, HS256 method, expiry) and
// returns its claims. Invalid tokens return an UNAUTHORIZED apperr.
func (m *Manager) ParseAccess(tokenString string) (*AccessClaims, error) {
	var claims jwtClaims
	token, err := jwt.ParseWithClaims(tokenString, &claims,
		func(t *jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(func() time.Time { return m.clock.Now() }),
	)
	if err != nil || !token.Valid {
		return nil, apperr.Unauthorized("invalid or expired token").WithCause(err)
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return nil, apperr.Unauthorized("invalid token subject")
	}
	return &AccessClaims{
		UserID:   userID,
		Role:     claims.Role,
		ClientID: claims.CID,
		JTI:      claims.ID,
	}, nil
}

// refreshRecord is the JSON stored at whmcs:refresh:<token>.
type refreshRecord struct {
	UserID int64  `json:"user_id"`
	JTI    string `json:"jti"`
}

func refreshKey(token string) string { return redisx.Key("refresh", token) }

// userRefreshSetKey indexes a user's outstanding refresh tokens so they can
// be revoked in bulk on password reset/change. Stale members (tokens that
// already expired) are harmless: deleting a missing key is a no-op.
func userRefreshSetKey(userID int64) string {
	return redisx.Key("refresh_user", strconv.FormatInt(userID, 10))
}

// CreateRefresh issues an opaque 43-char refresh token bound to userID/jti.
func (m *Manager) CreateRefresh(ctx context.Context, userID int64, jti string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("authtoken: rand: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw) // 43 chars
	payload, _ := json.Marshal(refreshRecord{UserID: userID, JTI: jti})
	pipe := m.rdb.TxPipeline()
	pipe.Set(ctx, refreshKey(token), payload, RefreshTTL)
	pipe.SAdd(ctx, userRefreshSetKey(userID), token)
	pipe.Expire(ctx, userRefreshSetKey(userID), RefreshTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("authtoken: store refresh: %w", err)
	}
	return token, nil
}

// ConsumeRefresh atomically consumes (deletes) a refresh token and returns
// its binding. Unknown/expired/reused tokens return UNAUTHORIZED. Callers
// rotate by issuing a new access token + CreateRefresh.
func (m *Manager) ConsumeRefresh(ctx context.Context, token string) (userID int64, jti string, err error) {
	raw, err := m.rdb.GetDel(ctx, refreshKey(token)).Bytes()
	if err == redis.Nil {
		return 0, "", apperr.Unauthorized("refresh token invalid or expired")
	}
	if err != nil {
		return 0, "", fmt.Errorf("authtoken: consume refresh: %w", err)
	}
	var rec refreshRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return 0, "", fmt.Errorf("authtoken: decode refresh: %w", err)
	}
	// Best-effort index maintenance; a leftover member is harmless.
	_ = m.rdb.SRem(ctx, userRefreshSetKey(rec.UserID), token).Err()
	return rec.UserID, rec.JTI, nil
}

// RotateRefresh consumes the old token and issues a replacement bound to the
// same user with the given new jti.
func (m *Manager) RotateRefresh(ctx context.Context, oldToken, newJTI string) (userID int64, newToken string, err error) {
	userID, _, err = m.ConsumeRefresh(ctx, oldToken)
	if err != nil {
		return 0, "", err
	}
	newToken, err = m.CreateRefresh(ctx, userID, newJTI)
	if err != nil {
		return 0, "", err
	}
	return userID, newToken, nil
}

// RevokeRefresh deletes a refresh token (logout). Unknown tokens are a no-op.
func (m *Manager) RevokeRefresh(ctx context.Context, token string) error {
	raw, err := m.rdb.GetDel(ctx, refreshKey(token)).Bytes()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("authtoken: revoke refresh: %w", err)
	}
	var rec refreshRecord
	if json.Unmarshal(raw, &rec) == nil {
		_ = m.rdb.SRem(ctx, userRefreshSetKey(rec.UserID), token).Err()
	}
	return nil
}

// RevokeAllForUser deletes every outstanding refresh token of userID -
// invalidating all their sessions (password reset/change, admin lockout).
func (m *Manager) RevokeAllForUser(ctx context.Context, userID int64) error {
	setKey := userRefreshSetKey(userID)
	tokens, err := m.rdb.SMembers(ctx, setKey).Result()
	if err != nil {
		return fmt.Errorf("authtoken: list refresh tokens: %w", err)
	}
	keys := make([]string, 0, len(tokens)+1)
	for _, t := range tokens {
		keys = append(keys, refreshKey(t))
	}
	keys = append(keys, setKey)
	if err := m.rdb.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("authtoken: revoke all refresh: %w", err)
	}
	return nil
}
