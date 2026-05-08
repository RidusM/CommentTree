package app

import (
	"context"
	"errors"
	"fmt"

	"ctree/internal/config"
	"ctree/internal/repository"
	"ctree/internal/service"
	handler "ctree/internal/transport/http"

	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
	"github.com/wb-go/wbf/dbpg/pgx-driver/transaction"
	"github.com/wb-go/wbf/logger"
	"github.com/wb-go/wbf/redis"
	"golang.org/x/sync/errgroup"
)

func Run(ctx context.Context, cfg *config.Config, log logger.Logger) error {
	var (
		db  *pgxdriver.Postgres
		rdb *redis.Client
		err error
	)

	defer func() {
		closeResources(ctx, db, rdb, log)
	}()

	db, rdb, err = initInfrastructure(ctx, cfg, log)
	if err != nil {
		return err
	}

	tm, err := transaction.NewManager(db, log)
	if err != nil {
		return fmt.Errorf("init transaction manager: %w", err)
	}

	handler := initHandler(cfg, db, rdb, tm, log)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return startHTTPServer(ctx, handler, &cfg.HTTP, log)
	})

	if egErr := eg.Wait(); egErr != nil && !errors.Is(egErr, context.Canceled) {
		return fmt.Errorf("app execution failed: %w", egErr)
	}

	return nil
}

func closeResources(
	ctx context.Context,
	db *pgxdriver.Postgres,
	rdb *redis.Client,
	log logger.Logger,
) {
	if db != nil {
		db.Close()
		log.LogAttrs(ctx, logger.InfoLevel, "database connection closed")
	}
	if rdb != nil {
		if closeErr := rdb.Close(); closeErr != nil {
			log.LogAttrs(ctx, logger.WarnLevel, "failed to close cache",
				logger.Any("error", closeErr),
			)
		}
	}
	log.LogAttrs(ctx, logger.InfoLevel, "all resources cleaned up")
}

func initInfrastructure(
	ctx context.Context,
	cfg *config.Config,
	log logger.Logger,
) (*pgxdriver.Postgres, *redis.Client, error) {
	db, err := initDatabase(&cfg.Database, log)
	if err != nil {
		return nil, nil, fmt.Errorf("init database: %w", err)
	}
	log.LogAttrs(ctx, logger.InfoLevel, "database initialized successfully")

	rdb, err := initCache(ctx, &cfg.Cache)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("init cache: %w", err)
	}
	log.LogAttrs(ctx, logger.InfoLevel, "cache initialized successfully")

	return db, rdb, nil
}

func initHandler(
	cfg *config.Config,
	db *pgxdriver.Postgres,
	rdb *redis.Client,
	tm transaction.Manager,
	log logger.Logger,
) *handler.CommentHandler {
	commRepo := repository.NewCommentRepository(db)
	cacheRepo := repository.NewCacheRepository(rdb)

	svc := service.NewCommentService(
		commRepo,
		cacheRepo,
		tm,
		log,
		service.DefaultPageSize(cfg.Service.DefaultPageSize),
		service.MaxPageSize(cfg.Service.MaxPageSize),
		service.MaxDepth(cfg.Service.MaxDepth),
	)

	handler := handler.NewCommentHandler(svc, log)
	return handler
}

func startHTTPServer(ctx context.Context, h *handler.CommentHandler, cfg *config.HTTP, log logger.Logger) error {
	server := handler.NewHTTPServer(h, cfg, log)
	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("start http server: %w", err)
	}
	return nil
}

func initDatabase(cfg *config.Database, log logger.Logger) (*pgxdriver.Postgres, error) {
	db, err := pgxdriver.New(
		cfg.DSN,
		log,
		pgxdriver.MaxPoolSize(cfg.PoolMax),
		pgxdriver.MaxConnAttempts(cfg.ConnAttempts),
		pgxdriver.BaseRetryDelay(cfg.BaseRetryDelay),
		pgxdriver.MaxRetryDelay(cfg.MaxRetryDelay),
	)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	return db, nil
}

func initCache(ctx context.Context, cfg *config.Cache) (*redis.Client, error) {
	initCtx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()

	rdb := redis.New(cfg.Addr, cfg.Password, cfg.DB)

	if err := rdb.Ping(initCtx); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("cache ping failed: %w", err)
	}
	return rdb, nil
}
