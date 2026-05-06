package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/noctusha/sso/internal/domain/models"
	libjwt "github.com/noctusha/sso/internal/lib/jwt"
	"github.com/noctusha/sso/internal/lib/logger/sl"
	"github.com/noctusha/sso/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	log            *slog.Logger
	userSaver      UserSaver
	userProvider   UserProvider
	appProvider    AppProvider
	tokenBlacklist TokenBlacklist
	tokenTTL       time.Duration
}

type UserSaver interface {
	SaveUser(ctx context.Context, email string, passHash []byte) (uid int64, err error)
}

type UserProvider interface {
	User(ctx context.Context, email string) (models.User, error)
	IsAdmin(ctx context.Context, userID int64) (bool, error)
}

type AppProvider interface {
	App(ctx context.Context, appID int) (models.App, error)
}

type TokenBlacklist interface {
	Add(ctx context.Context, token string, ttl time.Duration) error
	IsBlackListed(ctx context.Context, token string) (bool, error)
}

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidAppID       = errors.New("invalid app id")
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidToken       = errors.New("token is invalid")
)

func New(log *slog.Logger, userSaver UserSaver, userProvider UserProvider, appProvider AppProvider, tokenBlacklist TokenBlacklist, tokenTTL time.Duration) *Auth {
	return &Auth{
		log:            log,
		userSaver:      userSaver,
		userProvider:   userProvider,
		appProvider:    appProvider,
		tokenBlacklist: tokenBlacklist,
		tokenTTL:       tokenTTL,
	}
}

// RegisterNewUser registers new user in the system and returns user ID.
// If user with given username already exists, returns error.
func (a *Auth) RegisterNewUser(ctx context.Context, email string, pass string) (int64, error) {
	const op = "auth.RegisterNewUser"

	log := a.log.With(
		slog.String("op", op),
		slog.String("email", email),
	)

	log.Info("registering user")

	passHash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to generate hash", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := a.userSaver.SaveUser(ctx, email, passHash)
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			log.Warn("user already exists", sl.Err(err))
			return 0, fmt.Errorf("%s: %w", op, ErrUserExists)
		}

		log.Error("failed to save user", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user registered")

	return id, nil
}

// Login checks if user with given credentials exists in the system and returns access token.
//
// If user exists, but password is incorrect, returns error.
// If user doesn't exist, returns error.
func (a *Auth) Login(ctx context.Context, email string, pass string, appID int) (string, error) {
	const op = "auth.Login"

	log := a.log.With(
		slog.String("op", op),
		slog.String("email", email),
	)

	log.Info("attempting to login user")

	user, err := a.userProvider.User(ctx, email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			a.log.Warn("user not found", sl.Err(err))
			return "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
		}
		a.log.Error("failed to get user", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	if err := bcrypt.CompareHashAndPassword(user.PassHash, []byte(pass)); err != nil {
		a.log.Info("invalid credentials", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
	}

	app, err := a.appProvider.App(ctx, appID)
	if err != nil {
		return "", fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user logged in successfully")

	token, err := libjwt.NewToken(user, app, a.tokenTTL)
	if err != nil {
		a.log.Info("failed to generate token", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return token, nil
}

func (a *Auth) Logout(ctx context.Context, userID int64, appID int, token string) error {
	const op = "auth.Logout"

	log := a.log.With(
		slog.String("op", op),
		slog.Int64("user_ID", userID),
		slog.Int("app_ID", appID),
	)

	log.Info("attempting to logout user")

	err := a.tokenBlacklist.Add(ctx, token, a.tokenTTL)
	if err != nil {
		// TODO: check error log and logic behind this method
		log.Error("failed to add token to a blacklist", sl.Err(err))
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user logged out successfully")

	return nil
}

// IsAdmin checks if user is admin.
func (a *Auth) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	const op = "auth.IsAdmin"

	log := a.log.With(
		slog.String("op", op),
		slog.Int64("user_ID", userID),
	)

	log.Info("checking if user is admin")

	isAdmin, err := a.userProvider.IsAdmin(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Warn("app not found", sl.Err(err))
			return false, fmt.Errorf("%s: %w", op, ErrInvalidAppID)
		}

		log.Error("failed to check if user is admin", sl.Err(err))
		return false, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("checked if user is admin", slog.Bool("is_admin", isAdmin))

	return isAdmin, nil
}

func (a *Auth) ValidateToken(ctx context.Context, tokenString string) (int64, int32, string, error) {
	const op = "auth.ValidateToken"

	log := a.log.With(
		slog.String("op", op),
	)

	log.Info("validating user")

	blacklisted, err := a.tokenBlacklist.IsBlackListed(ctx, tokenString)
	if err != nil {
		log.Error("failed to check blacklist", sl.Err(err))
		return 0, 0, "", fmt.Errorf("%s: %w", op, err)
	}
	if blacklisted {
		log.Warn("token is blacklisted")
		return 0, 0, "", fmt.Errorf("%s: %w", op, ErrInvalidToken)
	}

	claims := jwt.MapClaims{}
	_, _, err = new(jwt.Parser).ParseUnverified(tokenString, claims)
	if err != nil {
		log.Error("failed to parse unverified token", sl.Err(err))
		return 0, 0, "", fmt.Errorf("%s: %w", op, ErrInvalidToken)
	}

	appID, ok := claims["app_id"].(float64)
	if !ok {
		return 0, 0, "", fmt.Errorf("%s: %w", op, ErrInvalidToken)
	}

	app, err := a.appProvider.App(ctx, int(appID))
	if err != nil {
		log.Error("failed to get app", sl.Err(err))
		return 0, 0, "", fmt.Errorf("%s: %w", op, err)
	}

	userID, _, email, err := libjwt.ParseToken(tokenString, app.Secret)
	if err != nil {
		log.Warn("invalid token", sl.Err(err))
		return 0, 0, "", fmt.Errorf("%s: %w", op, ErrInvalidToken)
	}

	log.Info("token validated successfully", slog.Int64("uid", userID))

	return userID, int32(appID), email, nil
}

// TODO: проверить все сообщения о логах и возвращаемые ошибки
