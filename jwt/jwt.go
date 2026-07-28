package jwt

import (
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// Claims 自定义的 JWT 载荷，可直接内嵌到项目里扩展字段
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwtlib.RegisteredClaims
}

// JWT JWT 实例 — 不是全局变量，每个服务独立创建
type JWT struct {
	Secret string
	Expire int64 // 过期时间，单位：秒
}

// New 创建一个 JWT 实例
//
//	secret  - 签名密钥（生产环境建议从环境变量读取）
//	expire  - token 有效期（秒），传 0 则不设置过期时间
func New(secret string, expire int64) *JWT {
	return &JWT{Secret: secret, Expire: expire}
}

// Generate 生成 token
//
//	userID   - 用户 ID
//	username - 用户名
func (j *JWT) Generate(userID uint, username string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwtlib.RegisteredClaims{
			IssuedAt: jwtlib.NewNumericDate(time.Now()),
		},
	}

	if j.Expire > 0 {
		claims.ExpiresAt = jwtlib.NewNumericDate(time.Now().Add(time.Duration(j.Expire) * time.Second))
	}

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.Secret))
}

// Parse 解析 token，返回 Claims
func (j *JWT) Parse(tokenString string) (*Claims, error) {
	token, err := jwtlib.ParseWithClaims(tokenString, &Claims{}, func(token *jwtlib.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("不支持的签名算法: %v", token.Header["alg"])
		}
		return []byte(j.Secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwtlib.ErrSignatureInvalid
	}

	return claims, nil
}
