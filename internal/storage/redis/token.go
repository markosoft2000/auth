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
	"golang.org/x/sync/semaphore"
)

const (
	tokenKey = "{app:%d}:refresh-token:%s"
	appTag   = "{app:%d}:tag"
	userTag  = "{user:%d}:tag"

	deleteAppTokenLimit                 = 10000
	revokeAllUserTokensConcurrencyLimit = 10
)

func getTokenKey(token string, appID int) string {
	return fmt.Sprintf(tokenKey, appID, token)
}

func getAppTagKey(appID int) string {
	return fmt.Sprintf(appTag, appID)
}

func getUserTagKey(userID int64) string {
	return fmt.Sprintf(userTag, userID)
}

func (s *Storage) RefreshToken(
	ctx context.Context,
	token string,
	userID int64,
	appID int,
) (*models.RefreshToken, error) {
	const op = "storage.redis.RefreshToken"

	ctxOp, OpCancel := context.WithTimeout(ctx, s.cfg.OperationTimeout)
	defer OpCancel()

	if token == "" {
		return nil, nil
	}

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
	appTagKey := getAppTagKey(token.AppID)
	userTagKey := getUserTagKey(token.UserID)

	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("%s: marshaling: %w", op, err)
	}

	// 1. Save the token
	// 2. Add the token key to the app's tag set
	// 3. Add expire timeout for the app-set
	// 4. Add the token key to the user's tag set
	// 5. Add expire timeout for the user-set
	cmds := make(rueidis.Commands, 0, 5)
	cmds = append(
		cmds,
		s.client.B().Set().
			Key(key).
			Value(string(data)).
			Ex(s.cfg.RefreshTokenTTL).
			Build(),
	)
	cmds = append(cmds, s.addKeyToSet(appTagKey, key, s.cfg.RefreshTokenTTL)...)
	cmds = append(cmds, s.addKeyToSet(userTagKey, key, s.cfg.RefreshTokenTTL)...)

	for _, res := range s.client.DoMulti(ctxOp, cmds...) {
		if err := res.Error(); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	return nil
}

// addKeyToSet returns 2 commands
func (s *Storage) addKeyToSet(tag, key string, expire time.Duration) rueidis.Commands {
	cmds := make(rueidis.Commands, 0, 2)

	cmds = append(cmds, s.client.B().Sadd().Key(tag).Member(key).Build())
	cmds = append(
		cmds,
		s.client.B().Expire().
			Key(tag).
			Seconds(int64(expire.Seconds())).
			Build(),
	)

	return cmds
}

func (s *Storage) RevokeToken(
	ctx context.Context,
	token string,
	userID int64,
	appID int,
) error {
	const op = "storage.redis.RevokeToken"

	ctxOp, OpCancel := context.WithTimeout(ctx, s.cfg.OperationTimeout)
	defer OpCancel()

	key := getTokenKey(token, appID)
	appTagKey := getAppTagKey(appID)
	userTagKey := getUserTagKey(userID)

	cmds := make(rueidis.Commands, 0, 3)
	cmds = append(cmds, s.client.B().Del().Key(key).Build())
	cmds = append(cmds, s.client.B().Srem().Key(appTagKey).Member(key).Build())
	cmds = append(cmds, s.client.B().Srem().Key(userTagKey).Member(key).Build())
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
	op := "storage.redis.RevokeAllUserTokens"

	userTagKey := getUserTagKey(userID)

	var cursor uint64

	for {
		chunkCtx, chunkCancel := context.WithTimeout(ctx, s.cfg.OperationTimeout)
		resp := s.client.Do(
			chunkCtx,
			s.client.B().
				Sscan().
				Key(userTagKey).
				Cursor(cursor).
				Count(deleteAppTokenLimit).
				Build(),
		)
		chunkCancel()

		if err := resp.Error(); err != nil {
			if rueidis.IsRedisNil(err) {
				break
			}

			return fmt.Errorf("%s: sscan failure: %w", op, err)
		}

		entry, _ := resp.AsScanEntry()

		if len(entry.Elements) > 0 {
			// Individual deletion is required because in a Redis Cluster, multiple keys in a single
			// command must share the same hash slot. Tokens for different apps belong to different slots.
			var wg sync.WaitGroup
			sem := semaphore.NewWeighted(revokeAllUserTokensConcurrencyLimit)
			for _, key := range entry.Elements {
				wg.Add(1)

				if err := sem.Acquire(ctx, 1); err != nil {
					return fmt.Errorf("%s: Failed to acquire semaphore: %w", op, err)
				}

				go func(k string) {
					defer sem.Release(1)
					defer wg.Done()

					delCtx, delCancel := context.WithTimeout(ctx, s.cfg.OperationTimeout)
					defer delCancel()

					_ = s.client.Do(delCtx, s.client.B().Del().Key(k).Build())
					//@TODO: delete user tokens from app-set - the problem app-sets are unknown (there might be several apps user logged in)
					// _ = s.client.Do(delCtx, s.client.B().Srem().Key(appTagKey...).Member(k).Build())
				}(key)
			}
			wg.Wait()
		}

		cursor = entry.Cursor
		if cursor == 0 {
			break
		}
	}

	// remove the tag index itself
	return s.client.Do(ctx, s.client.B().Del().Key(userTagKey).Build()).Error()
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

				resp := n.Do(
					chunkCtx,
					n.B().
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
						if rueidis.IsRedisNil(err) {
							break
						}

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

					err := n.Do(
						delCtx,
						n.B().
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
			err := n.Do(ctx, s.client.B().Del().Key(tag).Build()).Error()
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
