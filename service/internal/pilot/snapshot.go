package pilot

import (
	"context"
	"time"
)

// A snapshot is serialized with account mutations across CLI and daemon processes.
// Nodes reject older generations, so a delayed sync cannot resurrect revoked keys.
func (s *Store) Snapshot(ctx context.Context) ([]User, int64, int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, 0, err
	}
	defer tx.Rollback()
	now := time.Now()
	_, err = tx.ExecContext(ctx, `UPDATE generations SET value=max(value+1,?) WHERE id=1`, now.UnixNano())
	if err != nil {
		return nil, 0, 0, err
	}
	var generation int64
	if err = tx.QueryRowContext(ctx, `SELECT value FROM generations WHERE id=1`).Scan(&generation); err != nil {
		return nil, 0, 0, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+columns+` FROM users WHERE revoked=0 AND expires>? ORDER BY id`, now.Unix())
	if err != nil {
		return nil, 0, 0, err
	}
	users := []User{}
	deadline := now.Add(60 * time.Second).Unix()
	for rows.Next() {
		u, e := scan(rows)
		if e != nil {
			rows.Close()
			return nil, 0, 0, e
		}
		users = append(users, u)
		if u.Expires < deadline {
			deadline = u.Expires
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, 0, 0, err
	}
	return users, generation, deadline, tx.Commit()
}
