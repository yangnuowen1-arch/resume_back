package auth

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   int64    `json:"userId"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

func GenerateToken(userID int64, username string, roles []string, secret string, expireHours string) (string, error) {
	//把字符串 "24" 转成整数 24
	hours, err := strconv.Atoi(expireHours)
	if err != nil || hours <= 0 {
		hours = 2
	}

	now := time.Now()

	//JWT token 里面要存的信息
	claims := Claims{
		UserID:   userID,
		Username: username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(hours) * time.Hour)),
			//token 是什么时候签发的
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	//把 claims 这些用户信息装进 JWT 里面
	//并且指定签名算法是 HS256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	//用这个 secret 给 token 签名，生成最终字符串
	return token.SignedString([]byte(secret))
}

func ParseToken(tokenString string, secret string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("无效的 token 签名算法")
			}
			//把字符串密钥转成字节数组
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("token 无效")
	}

	return claims, nil
}
