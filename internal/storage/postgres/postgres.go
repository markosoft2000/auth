package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Host     string
	User     string
	Password string
	Database string
	SSLMode  string
	Port     int
}

type Storage struct {
	masterPool  *pgxpool.Pool // uses for writes and transactions (reading or writing)
	replicaPool *pgxpool.Pool // uses for reads
}

func New(masterPoolCfg, replicaPoolCfg *Config) (*Storage, error) {
	rwPool, err := createPool(masterPoolCfg)
	if err != nil {
		return nil, err
	}

	roPool := rwPool

	if replicaPoolCfg.Host != "" {
		roPool, err = createPool(replicaPoolCfg)
		if err != nil {
			return nil, err
		}
	}

	return &Storage{
		masterPool:  rwPool,
		replicaPool: roPool,
	}, nil
}

func createPool(cfg *Config) (*pgxpool.Pool, error) {
	const op = "storage.postgres.createPool"

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s default_query_exec_mode=simple_protocol",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Database,
		cfg.SSLMode,
	)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return pool, nil
}

func (s *Storage) Ping(ctx context.Context) error {
	if err := s.masterPool.Ping(ctx); err != nil {
		return err
	}

	return s.replicaPool.Ping(ctx)
}

func (s *Storage) Stop() {
	s.masterPool.Close()
	s.replicaPool.Close()
}
