package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	_ "modernc.org/sqlite" // sqlite driver

	"github.com/gosom/google-maps-scraper/web"
)

type repo struct {
	db *sql.DB
}

func New(path string) (web.JobRepository, error) {
	db, err := initDatabase(path)
	if err != nil {
		return nil, err
	}

	return &repo{db: db}, nil
}

func (repo *repo) Get(ctx context.Context, id string) (web.Job, error) {
	const q = `SELECT * from jobs WHERE id = ?`

	row := repo.db.QueryRowContext(ctx, q, id)

	return rowToJob(row)
}

func (repo *repo) Create(ctx context.Context, job *web.Job) error {
	item, err := jobToRow(job)
	if err != nil {
		return err
	}

	const q = `INSERT INTO jobs (id, name, status, data, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`

	_, err = repo.db.ExecContext(ctx, q, item.ID, item.Name, item.Status, item.Data, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return err
	}

	return nil
}

func (repo *repo) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM jobs WHERE id = ?`

	_, err := repo.db.ExecContext(ctx, q, id)

	return err
}

func (repo *repo) Select(ctx context.Context, params web.SelectParams) ([]web.Job, error) {
	q := `SELECT * from jobs`

	var args []any

	if params.Status != "" {
		q += ` WHERE status = ?`

		args = append(args, params.Status)
	}

	q += " ORDER BY created_at DESC, id DESC"

	if params.Limit > 0 {
		q += " LIMIT ?"

		args = append(args, params.Limit)
	}

	if params.Offset > 0 {
		if params.Limit <= 0 {
			q += " LIMIT -1"
		}

		q += " OFFSET ?"

		args = append(args, params.Offset)
	}

	rows, err := repo.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var ans []web.Job

	for rows.Next() {
		job, err := rowToJob(rows)
		if err != nil {
			return nil, err
		}

		ans = append(ans, job)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ans, nil
}

func (repo *repo) Count(ctx context.Context, params web.SelectParams) (int, error) {
	q := `SELECT COUNT(*) from jobs`

	var args []any

	if params.Status != "" {
		q += ` WHERE status = ?`

		args = append(args, params.Status)
	}

	var count int

	err := repo.db.QueryRowContext(ctx, q, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (repo *repo) Update(ctx context.Context, job *web.Job) error {
	item, err := jobToRow(job)
	if err != nil {
		return err
	}

	const q = `UPDATE jobs SET name = ?, status = ?, data = ?, updated_at = ? WHERE id = ?`

	_, err = repo.db.ExecContext(ctx, q, item.Name, item.Status, item.Data, item.UpdatedAt, item.ID)

	return err
}

func (repo *repo) SetProspectStage(ctx context.Context, key string, stage web.ProspectStage) (web.ProspectState, error) {
	if !web.ValidProspectKey(key) || !stage.Valid() {
		return web.ProspectState{}, sql.ErrNoRows
	}

	now := time.Now().UTC()
	const q = `INSERT INTO prospects (place_key, stage, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(place_key) DO UPDATE SET stage = excluded.stage, updated_at = excluded.updated_at`
	if _, err := repo.db.ExecContext(ctx, q, key, string(stage), now.Unix()); err != nil {
		return web.ProspectState{}, err
	}
	states, err := repo.ListProspects(ctx, []string{key})
	if err != nil {
		return web.ProspectState{}, err
	}
	return states[key], nil
}

func (repo *repo) AddProspectTouch(ctx context.Context, key string, stage web.ProspectStage, message string) (web.ProspectTouch, error) {
	if !web.ValidProspectKey(key) || !stage.Valid() || strings.TrimSpace(message) == "" || len(message) > 2000 {
		return web.ProspectTouch{}, sql.ErrNoRows
	}

	now := time.Now().UTC()
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return web.ProspectTouch{}, err
	}
	defer tx.Rollback()

	const upsert = `INSERT INTO prospects (place_key, stage, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(place_key) DO UPDATE SET stage = excluded.stage, updated_at = excluded.updated_at`
	if _, err := tx.ExecContext(ctx, upsert, key, string(stage), now.Unix()); err != nil {
		return web.ProspectTouch{}, err
	}

	const insert = `INSERT INTO prospect_touches (place_key, stage, message, outcome, created_at)
		VALUES (?, ?, ?, ?, ?)`
	result, err := tx.ExecContext(ctx, insert, key, string(stage), message, web.ProspectTouchOpened, now.Unix())
	if err != nil {
		return web.ProspectTouch{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return web.ProspectTouch{}, err
	}
	if err := tx.Commit(); err != nil {
		return web.ProspectTouch{}, err
	}

	return web.ProspectTouch{ID: id, Key: key, Stage: stage, Message: message, Outcome: web.ProspectTouchOpened, CreatedAt: now}, nil
}

func (repo *repo) ListProspects(ctx context.Context, keys []string) (map[string]web.ProspectState, error) {
	ans := make(map[string]web.ProspectState, len(keys))
	if len(keys) == 0 {
		return ans, nil
	}

	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))
	for i, key := range keys {
		placeholders[i] = "?"
		args[i] = key
	}
	q := `SELECT p.place_key, p.stage, p.updated_at,
		(SELECT COUNT(*) FROM prospect_touches x WHERE x.place_key = p.place_key),
		t.id, t.stage, t.message, t.outcome, t.created_at
		FROM prospects p
		LEFT JOIN prospect_touches t ON t.id = (SELECT id FROM prospect_touches x
			WHERE x.place_key = p.place_key ORDER BY id DESC LIMIT 1)
		WHERE p.place_key IN (` + strings.Join(placeholders, ",") + `)`

	rows, err := repo.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var state web.ProspectState
		var stage string
		var updatedAt int64
		var touchID, touchAt sql.NullInt64
		var touchStage, message, outcome sql.NullString
		if err := rows.Scan(&state.Key, &stage, &updatedAt, &state.TouchCount, &touchID, &touchStage, &message, &outcome, &touchAt); err != nil {
			return nil, err
		}
		state.Stage = web.ProspectStage(stage)
		state.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		if touchID.Valid {
			state.LastTouch = &web.ProspectTouch{
				ID:        touchID.Int64,
				Key:       state.Key,
				Stage:     web.ProspectStage(touchStage.String),
				Message:   message.String,
				Outcome:   outcome.String,
				CreatedAt: time.Unix(touchAt.Int64, 0).UTC(),
			}
		}
		ans[state.Key] = state
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ans, nil
}

func (repo *repo) ListProspectTouches(ctx context.Context, key string) ([]web.ProspectTouch, error) {
	const q = `SELECT id, place_key, stage, message, outcome, created_at
		FROM prospect_touches WHERE place_key = ? ORDER BY id ASC`
	rows, err := repo.db.QueryContext(ctx, q, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ans := []web.ProspectTouch{}
	for rows.Next() {
		var touch web.ProspectTouch
		var stage string
		var createdAt int64
		if err := rows.Scan(&touch.ID, &touch.Key, &stage, &touch.Message, &touch.Outcome, &createdAt); err != nil {
			return nil, err
		}
		touch.Stage = web.ProspectStage(stage)
		touch.CreatedAt = time.Unix(createdAt, 0).UTC()
		ans = append(ans, touch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ans, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func rowToJob(row scannable) (web.Job, error) {
	var j job

	err := row.Scan(&j.ID, &j.Name, &j.Status, &j.Data, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return web.Job{}, err
	}

	ans := web.Job{
		ID:     j.ID,
		Name:   j.Name,
		Status: j.Status,
		Date:   time.Unix(j.CreatedAt, 0).UTC(),
	}

	err = json.Unmarshal([]byte(j.Data), &ans.Data)
	if err != nil {
		return web.Job{}, err
	}

	return ans, nil
}

func jobToRow(item *web.Job) (job, error) {
	data, err := json.Marshal(item.Data)
	if err != nil {
		return job{}, err
	}

	return job{
		ID:        item.ID,
		Name:      item.Name,
		Status:    item.Status,
		Data:      string(data),
		CreatedAt: item.Date.Unix(),
		UpdatedAt: time.Now().UTC().Unix(),
	}, nil
}

type job struct {
	ID        string
	Name      string
	Status    string
	Data      string
	CreatedAt int64
	UpdatedAt int64
}

func initDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(30 * time.Minute)

	_, err = db.Exec("PRAGMA busy_timeout = 5000")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("PRAGMA synchronous=NORMAL")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("PRAGMA cache_size=1000")
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, createSchema(db)
}

func createSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			data TEXT NOT NULL,
			created_at INT NOT NULL,
			updated_at INT NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_jobs_created_at_id
			ON jobs (created_at DESC, id DESC);

		CREATE TABLE IF NOT EXISTS prospects (
			place_key TEXT PRIMARY KEY,
			stage TEXT NOT NULL,
			updated_at INT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS prospect_touches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			place_key TEXT NOT NULL,
			stage TEXT NOT NULL,
			message TEXT NOT NULL,
			outcome TEXT NOT NULL,
			created_at INT NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_prospect_touches_key_id
			ON prospect_touches (place_key, id);
	`)

	return err
}
