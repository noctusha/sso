package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/noctusha/sso/internal/domain/models"
)

func NewToken(user models.User, app models.App, duration time.Duration) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["uid"] = user.ID
	claims["email"] = user.Email
	claims["exp"] = time.Now().Add(duration).Unix()
	claims["app_id"] = app.ID

	tokenString, err := token.SignedString([]byte(app.Secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func ParseToken(tokenString string, secret string) (int64, int, string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return 0, 0, "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, 0, "", fmt.Errorf("invalid token claims")
	}

	uidFloat, ok := claims["uid"].(float64)
	if !ok {
		return 0, 0, "", fmt.Errorf("user_id claim is missing or invalid")
	}
	userID := int64(uidFloat)

	aidFloat, ok := claims["app_id"].(float64)
	if !ok {
		return 0, 0, "", fmt.Errorf("app_id claim is missing or invalid")
	}
	appID := int(aidFloat)
	email := claims["email"].(string)

	return userID, appID, email, nil

}

// TODO: add tests
