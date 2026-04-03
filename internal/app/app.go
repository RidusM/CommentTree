package app

import (
	"context"
	"fmt"
	"time"

	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
	"github.com/wb-go/wbf/dbpg/pgx-driver/transaction"
	rediswbf "github.com/wb-go/wbf/redis"
	"github.com/wb-go/wbf/logger"

	"comtree/internal/config"
	"comtree/internal/handler"
	"comtree/internal/repository"
	"comtree/internal/service"
)

type App struct {
	cfg     *config.Config
	log     logger.Logger
	httpSrv *handler.HTTPServer // или handler.CommentHandler, если сервер внутри него
	handler *handler.CommentHandler
}

func New(cfg *config.Config, log logger.Logger) (*App, error) {
	// 1. БД
	db, err := pgxdriver.New(context.Background(), cfg.Database.DSN, pgxdriver.WithMaxPoolSize(cfg.Database.PoolMax))
	if err != nil {
		return nil, fmt.Errorf("init db: %w", err)
	}
	tm := transaction.NewManager(db)

	// 2. Redis
	rdb, err := rediswbf.New(context.Background(), rediswbf.Config{
		Addr: cfg.Cache.Addr, Password: cfg.Cache.Password, PoolSize: cfg.Cache.PoolSize,
		MinIdleCons: cfg.Cache.MinIdleCons, PoolTimeout: cfg.Cache.PoolTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("init redis: %w", err)
	}

	// 3. Слои
	commentRepo := repository.NewCommentRepository(db)
	cacheRepo := repository.NewCacheRepository(rdb)

	commentSvc := service.NewCommentService(commentRepo, cacheRepo, tm, log,
		service.WithMaxDepth(cfg.Service.MaxDepth),
		service.WithDefaultPageSize(cfg.Service.DefaultPageSize),
		service.WithMaxPageSize(cfg.Service.MaxPageSize),
	)

	commentHandler := handler.NewCommentHandler(commentSvc, log)

	return &App{
		cfg:     cfg,
		log:     log,
		handler: commentHandler,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:    a.cfg.HTTP.Host + ":" + a.cfg.HTTP.Port,
		Handler: a.handler.Engine(),
	}

	go func() {
		a.log.LogAttrs(ctx, logger.InfoLevel, "HTTP server started", logger.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.log.LogAttrs(ctx, logger.ErrorLevel, "HTTP server error", logger.Any("error", err))
		}
	}()

	<-ctx.Done()
	a.log.LogAttrs(ctx, logger.InfoLevel, "shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.HTTP.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}