package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/markosoft2000/auth/internal/domain/models"
	"github.com/markosoft2000/auth/internal/storage"
	"github.com/redis/rueidis"
)

const (
	tokenKey = "{app:%d}:refresh-token:%s"
	appTag   = "{app:%d}:tag"

	deleteAppTokenLimit = 10000
)

func getTokenKey(token string, appID int) string {
	return fmt.Sprintf(tokenKey, appID, token)
}

func getAppTagKey(appID int) string {
	return fmt.Sprintf(appTag, appID)
}

func (s *Storage) RefreshToken(
	ctx context.Context,
	token string,
	appID int,
) (*models.RefreshToken, error) {
	const op = "storage.redis.RefreshToken"

	ctxOp, OpCancel := context.WithTimeout(ctx, s.cfg.OperationTimeout)
	defer OpCancel()

	key := getTokenKey(token, appID)

	resp := s.client.Do(ctxOp, s.client.B().Get().Key(key).Build())

	// Handle "Key Not Found" specifically
	if rueidis.IsRedisNil(resp.Error()) {
		return nil, fmt.Errorf(
			"%s: refresh token not found: %w",
			op,
			storage.ErrRefreshTokenNotFound,
		)
	}

	if err := resp.Error(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	data, err := resp.AsBytes()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	storedToken := &models.RefreshToken{}
	if err := json.Unmarshal(data, &storedToken); err != nil {
		return nil, fmt.Errorf("%s: unmarshaling: %w", op, err)
	}

	return storedToken, nil
}

func (s *Storage) SaveRefreshToken(
	ctx context.Context,
	token *models.RefreshToken,
) error {
	const op = "storage.redis.SaveRefreshToken"

	ctxOp, OpCancel := context.WithTimeout(ctx, s.cfg.OperationTimeout)
	defer OpCancel()

	key := getTokenKey(token.Token, token.AppID)
	tag := getAppTagKey(token.AppID)
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("%s: marshaling: %w", op, err)
	}

	// 1. Save the token
	// 2. Add the token key to the app's tag set
	// 3. Add expire timeout for the tag
	cmds := make(rueidis.Commands, 0, 3)
	cmds = append(
		cmds,
		s.client.B().Set().
			Key(key).
			Value(string(data)).
			Ex(s.cfg.RefreshTokenTTL).
			Build(),
	)
	cmds = append(cmds, s.client.B().Sadd().Key(tag).Member(key).Build())
	cmds = append(
		cmds,
		s.client.B().Expire().
			Key(tag).
			Seconds(int64(s.cfg.RefreshTokenTTL.Seconds())).
			Build(),
	)

	for _, res := range s.client.DoMulti(ctxOp, cmds...) {
		if err := res.Error(); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	return nil
}

func (s *Storage) RevokeToken(
	ctx context.Context,
	token string,
	appID int,
) error {
	const op = "storage.redis.RevokeToken"

	ctxOp, OpCancel := context.WithTimeout(ctx, s.cfg.OperationTimeout)
	defer OpCancel()

	key := getTokenKey(token, appID)
	tag := getAppTagKey(appID)

	cmds := make(rueidis.Commands, 0, 2)
	cmds = append(cmds, s.client.B().Del().Key(key).Build())
	cmds = append(cmds, s.client.B().Srem().Key(tag).Member(key).Build())
	resps := s.client.DoMulti(ctxOp, cmds...)

	if err := resps[0].Error(); err != nil {
		return fmt.Errorf("%s: internal failure: %w", op, err)
	}

	// Detect "No Such Key"
	count, _ := resps[0].AsInt64()
	if count == 0 {
		return fmt.Errorf(
			"%s: refresh token not found: %w",
			op,
			storage.ErrRefreshTokenNotFound,
		)
	}

	return nil
}

func (s *Storage) RevokeAllUserTokens(ctx context.Context, userID int64) error {
	return nil
}

func (s *Storage) RevokeAllAppTokens(ctx context.Context, appID int) error {
	const op = "storage.redis.RevokeAllAppTokens"

	tag := getAppTagKey(appID)

	// Get all master nodes in the cluster
	nodes := s.client.Nodes()
	wg := &sync.WaitGroup{}
	errChan := make(chan error, len(nodes))

	for nodeName, node := range nodes {
		wg.Add(1)

		go func(nodeName string, n rueidis.Client) {
			defer wg.Done()

			var cursor uint64
			var errs error

			// remove tag members - tokens with app-tag
			for {
				chunkCtx, chunkCancel := context.WithTimeout(ctx, time.Second)

				resp := s.client.Do(
					chunkCtx,
					s.client.B().
						Sscan().
						Key(tag).
						Cursor(cursor).
						Count(deleteAppTokenLimit).
						Build(),
				)
				chunkCancel()
				if err := resp.Error(); err != nil {
					// Check if it's a timeout error
					if errors.Is(err, context.DeadlineExceeded) {
						// If the main ctx time is up, we must stop
						if ctx.Err() != nil {
							errs = errors.Join(
								errs,
								fmt.Errorf(
									"%s: main timeout exceeded on node %s: %w",
									op,
									nodeName,
									ctx.Err(),
								),
							)

							break
						}

						// Otherwise, it was just a slow chunk. retry.
						continue
					}

					errs = errors.Join(
						errs,
						fmt.Errorf("%s on node %s: %w", op, nodeName, err),
					)
				}

				entry, _ := resp.AsScanEntry()

				if len(entry.Elements) > 0 {
				RetryDelete:
					delCtx, delCancel := context.WithTimeout(ctx, time.Second)

					err := s.client.Do(
						delCtx,
						s.client.B().
							Del().
							Key(entry.Elements...).
							Build(),
					).Error()
					delCancel()

					if err != nil {
						// Check if it's a timeout error
						if errors.Is(err, context.DeadlineExceeded) {
							// If the main ctx time is up, we must stop
							if ctx.Err() != nil {
								errs = errors.Join(
									errs,
									fmt.Errorf(
										"%s: main timeout exceeded on node %s: %w",
										op,
										nodeName,
										ctx.Err(),
									),
								)
								break
							}

							// Otherwise, it was just a slow chunk. retry.
							goto RetryDelete
						}

						errs = errors.Join(
							errs,
							fmt.Errorf("%s on node %s: %w", op, nodeName, err),
						)
					}
				}

				cursor = entry.Cursor
				if cursor == 0 {
					break
				}
			}

			// remove the tag index itself
			err := s.client.Do(ctx, s.client.B().Del().Key(tag).Build()).Error()
			if err != nil {
				errs = errors.Join(
					errs,
					fmt.Errorf("%s on node %s: %w", op, nodeName, err),
				)
			}

			if errs != nil {
				errChan <- errs
			}

		}(nodeName, node)
	}

	wg.Wait()
	close(errChan)

	// Check for any errors during the parallel scan
	var errs error
	for err := range errChan {
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}
