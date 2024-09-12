package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type JWT struct {
    privateKey []byte
    publicKey []byte
}

type MyJWTClaims struct {
	*jwt.RegisteredClaims
	UserInfo interface{}
}

type UserJWTData struct {
	Username string
	Admin    bool
}

func NewJWT(privateKey, publicKey []byte) *JWT {
    return &JWT{
        privateKey: privateKey,
        publicKey: publicKey,
    }
}

func (j *JWT) Create(sub string, userInfo interface{}) (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(j.privateKey)
	if err != nil {
		return "", fmt.Errorf("create: parse key: %w", err)
	}

	exp := time.Now().Add(time.Hour * 12)

	claims := &MyJWTClaims{
		&jwt.RegisteredClaims{
			Subject:   sub,
			ExpiresAt: jwt.NewNumericDate(exp),
		},
		userInfo,
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		return "", fmt.Errorf("create: sign token: %w", err)
	}

	return token, nil
}

func GetClaimsFromToken(publicKey []byte, tokenString string) (jwt.MapClaims, error) {
	key, err := jwt.ParseRSAPublicKeyFromPEM(publicKey)
	if err != nil {
		return nil, fmt.Errorf("get claims: parse key: %w", err)
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return key, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, err
}
