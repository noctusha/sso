package app

import (
	"log/slog"
	"time"

	grpcapp "github.com/noctusha/sso/internal/app/grpc"
	"github.com/noctusha/sso/internal/services/auth"
	"github.com/noctusha/sso/internal/storage/redis"
	"github.com/noctusha/sso/internal/storage/sqlite"
)

type App struct {
	GRPCSrv *grpcapp.App
}

func New(log *slog.Logger, grpcPort int, storagePath string, redisAddress string, tokenTTL time.Duration) *App {
	storage, err := sqlite.New(storagePath)
	if err != nil {
		panic(err)
	}

	//rdb, err := redis.New(redisAddress)
	//if err != nil {
	//	panic(err)
	//}

	// TODO: now in memory, not redis
	rdb := redis.NewInMemory(log)

	authService := auth.New(log, storage, storage, storage, rdb, tokenTTL)

	grpcApp := grpcapp.New(log, grpcPort, authService)

	return &App{
		GRPCSrv: grpcApp,
	}
}
