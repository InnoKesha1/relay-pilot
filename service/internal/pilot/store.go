package pilot

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"time"
)

type Store struct{ DB *sql.DB }
type User struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Expires          int64  `json:"expires"`
	Revoked          bool   `json:"revoked"`
	EntryUUID        string `json:"-"`
	ExitUUID         string `json:"-"`
	HysteriaPassword string `json:"-"`
}

func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, err
		}
		f.Close()
		if err = os.Chmod(path, 0600); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL;
	CREATE TABLE IF NOT EXISTS users(id TEXT PRIMARY KEY, name TEXT NOT NULL, token_hash TEXT UNIQUE NOT NULL,
	expires INTEGER NOT NULL, revoked INTEGER NOT NULL DEFAULT 0, entry_uuid TEXT NOT NULL,
	exit_uuid TEXT NOT NULL, hysteria_password TEXT NOT NULL);
	CREATE TABLE IF NOT EXISTS generations(id INTEGER PRIMARY KEY CHECK(id=1), value INTEGER NOT NULL);
	INSERT OR IGNORE INTO generations VALUES(1,0);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.DB.Close() }

func secret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func uuid() (string, error) {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}
func hashToken(t string) string { h := sha256.Sum256([]byte(t)); return hex.EncodeToString(h[:]) }

func (s *Store) Add(name string, expires time.Time) (User, string, error) {
	var u User
	if len(name) == 0 || len(name) > 100 || !expires.After(time.Now()) {
		return u, "", errors.New("name and future expiry required")
	}
	u.Name = name
	u.Expires = expires.Unix()
	var err error
	if u.ID, err = uuid(); err != nil {
		return u, "", err
	}
	if err = credentials(&u); err != nil {
		return u, "", err
	}
	token, err := secret(32)
	if err != nil {
		return u, "", err
	}
	// The count and insert are a single statement, including across CLI processes.
	r, err := s.DB.Exec(`INSERT INTO users SELECT ?,?,?,?,0,?,?,? WHERE
	(SELECT count(*) FROM users WHERE revoked=0 AND expires>?) < ?`, u.ID, name, hashToken(token), u.Expires, u.EntryUUID, u.ExitUUID, u.HysteriaPassword, time.Now().Unix(), MaxUsers)
	if err != nil {
		return u, "", err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return u, "", errors.New("pilot limit: 10 active participants")
	}
	return u, token, nil
}
func credentials(u *User) error {
	var err error
	if u.EntryUUID, err = uuid(); err != nil {
		return err
	}
	if u.ExitUUID, err = uuid(); err != nil {
		return err
	}
	u.HysteriaPassword, err = secret(32)
	return err
}

const columns = `id,name,expires,revoked,entry_uuid,exit_uuid,hysteria_password`

func scan(row interface{ Scan(...any) error }) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Name, &u.Expires, &u.Revoked, &u.EntryUUID, &u.ExitUUID, &u.HysteriaPassword)
	return u, err
}
func (s *Store) ByToken(token string, now time.Time) (User, error) {
	if len(token) != 43 {
		return User{}, sql.ErrNoRows
	}
	return scan(s.DB.QueryRow(`SELECT `+columns+` FROM users WHERE token_hash=? AND revoked=0 AND expires>?`, hashToken(token), now.Unix()))
}
func (s *Store) Users(active bool, now time.Time) ([]User, error) {
	query := `SELECT ` + columns + ` FROM users`
	args := []any{}
	if active {
		query += ` WHERE revoked=0 AND expires>?`
		args = append(args, now.Unix())
	}
	query += ` ORDER BY id`
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		u, err := scan(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
func (s *Store) Revoke(id string) error {
	r, e := s.DB.Exec(`UPDATE users SET revoked=1 WHERE id=?`, id)
	return affected(r, e)
}
func (s *Store) Expire(id string, when time.Time) error {
	r, e := s.DB.Exec(`UPDATE users SET expires=? WHERE id=? AND
	(?<=? OR revoked=1 OR expires>? OR
	 (SELECT count(*) FROM users WHERE revoked=0 AND expires>?) < ?)`,
		when.Unix(), id, when.Unix(), time.Now().Unix(), time.Now().Unix(), time.Now().Unix(), MaxUsers)
	return affected(r, e)
}
func affected(r sql.Result, e error) error {
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e == nil && n == 0 {
		return sql.ErrNoRows
	}
	return e
}
func (s *Store) Rotate(id string) (string, error) {
	u := User{}
	if err := credentials(&u); err != nil {
		return "", err
	}
	token, err := secret(32)
	if err != nil {
		return "", err
	}
	r, err := s.DB.Exec(`UPDATE users SET token_hash=?,entry_uuid=?,exit_uuid=?,hysteria_password=? WHERE id=? AND revoked=0 AND expires>?`, hashToken(token), u.EntryUUID, u.ExitUUID, u.HysteriaPassword, id, time.Now().Unix())
	return token, affected(r, err)
}
func (s *Store) Backup(path string) error {
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		return fmt.Errorf("backup destination must not exist")
	}
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	// VACUUM INTO includes committed WAL data and takes a consistent snapshot.
	if _, e := s.DB.Exec(`VACUUM INTO ?`, path); e != nil {
		return e
	}
	return os.Chmod(path, 0600)
}
