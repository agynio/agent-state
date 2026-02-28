package state

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) runTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func parseUUID(id string) (uuid.UUID, error) {
	value, err := uuid.Parse(id)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("invalid uuid: %w", err)
	}
	return value, nil
}

func scanConversationMetadata(row pgx.Row, id uuid.UUID) (ConversationMetadata, error) {
	var metadata ConversationMetadata
	metadata.ID = id
	if err := row.Scan(&metadata.MessageCount, &metadata.UpdatedAt, &metadata.Version); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ConversationMetadata{}, ErrConversationNotFound
		}
		return ConversationMetadata{}, err
	}
	return metadata, nil
}

func (s *Store) loadConversationMetadata(ctx context.Context, id uuid.UUID) (ConversationMetadata, error) {
	row := s.pool.QueryRow(ctx, `SELECT message_count, updated_at, version FROM conversations WHERE id = $1`, id)
	return scanConversationMetadata(row, id)
}

func (s *Store) loadConversationMetadataForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (ConversationMetadata, error) {
	row := tx.QueryRow(ctx, `SELECT message_count, updated_at, version FROM conversations WHERE id = $1 FOR UPDATE`, id)
	return scanConversationMetadata(row, id)
}
