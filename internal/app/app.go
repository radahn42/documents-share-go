package app

import (
	"context"
	"document-server/internal/config"
	"document-server/internal/domain/services"
	"document-server/internal/infrastructure/cache"
	"document-server/internal/infrastructure/database"
	"document-server/internal/infrastructure/database/repositories"
	"document-server/internal/interfaces/handlers"
	"document-server/pkg/logger"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Run(cfg config.Config) error {
	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		logger.Error("Failed to connect to database", zap.Error(err))
		return err
	}
	defer db.Close()

	redisClient, err := cache.NewRedisCache(cfg.Redis)
	if err != nil {
		logger.Error("Failed to connect to redis", zap.Error(err))
		return err
	}
	defer redisClient.Close()

	userRepo := repositories.NewUserRepository(db.Pool())
	docRepo := repositories.NewDocumentRepository(db.Pool())
	sessionRepo := repositories.NewSessionRepository(db.Pool())

	cacheSvc := services.NewRedisCacheService(redisClient, cfg.Auth.CacheDuration)
	authSvc := services.NewAuthService(userRepo, sessionRepo, cfg.Auth.AdminToken, cfg.Auth.TokenDuration)
	docSvc := services.NewDocumentService(docRepo, userRepo, cacheSvc)

	authHandler := handlers.NewAuthHandler(authSvc)
	docHandler := handlers.NewDocumentHandler(docSvc, authSvc, cfg.Storage.Path)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(handlers.HeadToGetMiddleware())
	r.Use(handlers.CORSMiddleware())
	r.HandleMethodNotAllowed = true

	api := r.Group("/api")
	{
		api.POST("/register", authHandler.Register)
		api.POST("/auth", authHandler.Authenticate)
		api.DELETE("/auth/:token", authHandler.Logout)

		api.POST("/docs", docHandler.Create)
		api.GET("/docs", docHandler.GetList)
		api.HEAD("/docs", docHandler.GetList)
		api.GET("/docs/:id", docHandler.GetByID)
		api.HEAD("/docs/:id", docHandler.GetByID)
		api.DELETE("/docs/:id", docHandler.Delete)
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		logger.Info("Starting server", zap.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("Failed to listen server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
