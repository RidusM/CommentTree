package app

import (
	"comtree/internal/config"
	"comtree/internal/repository"
	"comtree/internal/service"
	handler "comtree/internal/transport/http"
	"context"
	"errors"
	"fmt"

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
		tm  transaction.Manager
		err error
	)

	defer func() {
		if db != nil {
			db.Close()
			log.Info("database connection closed")
		}
		if rdb != nil {
			if closeErr := rdb.Close(); closeErr != nil {
				log.Warn("failed to close cache", "error", closeErr)
			}
		}
		log.Info("all resources cleaned up")
	}()

	db, err = initDatabase(&cfg.Database, log)
	if err != nil {
		return fmt.Errorf("init database: %w", err)
	}
	log.Info("database initialized successfully")

	tm, err = initTransactionManager(db, log)
	if err != nil {
		return fmt.Errorf("init transaction manager: %w", err)
	}

	rdb, err = initCache(ctx, &cfg.Cache)
	if err != nil {
		return fmt.Errorf("init cache: %w", err)
	}
	log.Info("cache initialized successfully")

	eg, ctx := errgroup.WithContext(ctx)

svc := initCommentService(&cfg.Service, db, tm, rdb, log)

	handlers := handler.NewCommentHandler(svc, log)
	httpServer := handler.NewHTTPServer(handlers, &cfg.HTTP, log)
	eg.Go(func() error {
		return httpServer.Start(ctx)
	})

	if egErr := eg.Wait(); egErr != nil && !errors.Is(egErr, context.Canceled) {
		return fmt.Errorf("app execution failed: %w", egErr)
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

func initTransactionManager(db *pgxdriver.Postgres, log logger.Logger) (transaction.Manager, error) {
	tm, err := transaction.NewManager(db, log)
	if err != nil {
		return nil, fmt.Errorf("create transaction manager: %w", err)
	}
	return tm, nil
}

func initCache(ctx context.Context, cfg *config.Cache) (*redis.Client, error) {
	initCtx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()

	rdb := redis.New(cfg.Addr, cfg.Password, cfg.DB)

	if err := rdb.Ping(initCtx); err != nil {
		_ = rdb.Close() //nolint:gosec // error during ping is more important
		return nil, fmt.Errorf("cache ping failed: %w", err)
	}
	return rdb, nil
}

func initCommentService(
	cfg *config.Service,
	db *pgxdriver.Postgres,
	tm transaction.Manager,
	rdb *redis.Client,
	log logger.Logger,
) *service.CommentService {
	commentRepo := repository.NewCommentRepository(db)
	cacheRepo := repository.NewCacheRepository(rdb)

	svc := service.NewCommentService(
		commentRepo,
		cacheRepo,
		tm,
		log,
		service.WithDefaultPageSize(cfg.DefaultPageSize),
		service.WithMaxDepth(cfg.MaxDepth),
		service.WithMaxPageSize(cfg.MaxPageSize),
	)
	return svc
}