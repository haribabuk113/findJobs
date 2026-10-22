package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	models "findJobs/internal/domain/master"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	apperror "findJobs/internal/utils/errors"
)

// // Repositories bundles every repository for wiring.
// type Repositories struct {
// 	Profile ProfileRepository

// 	Master MasterRepository
// }

// func New(db *pgxpool.Pool) *Repositories {
// 	return &Repositories{
// 		Profile: &profileRepo{db},

// 		Master: &masterRepo{db},
// 	}
// }

// translate maps driver errors to application errors.
func translate(err error) error {
	if err == nil {
		return nil
	}
	var ae *apperror.Error
	if errors.As(err, &ae) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		return &apperror.Error{Status: http.StatusNotFound, Message: "resource not found"}
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return &apperror.Error{Status: http.StatusConflict, Message: "resource already exists"}
		case "23503": // foreign_key_violation (bad reference, or delete blocked by references)
			return &apperror.Error{Status: http.StatusUnprocessableEntity, Message: "conflicts with a related record"}
		case "23514": // check_violation
			return &apperror.Error{Status: http.StatusUnprocessableEntity, Message: "value violates a constraint"}
		case "22P02", "22007", "22008": // malformed text / date
			return &apperror.Error{Status: http.StatusBadRequest, Message: "malformed value"}
		}
	}
	return &apperror.Error{Status: http.StatusInternalServerError, Message: err.Error()}
}

// affected turns "0 rows changed" into NotFound. Ownership checks live in the
// WHERE clause of the statement, so 0 rows also covers "not yours".
func affected(res pgconn.CommandTag, err error) error {
	if err != nil {
		return translate(err)
	}
	if res.RowsAffected() == 0 {
		return &apperror.Error{Status: http.StatusNotFound, Message: "resource not found"}
	}

	return nil
}

func inTx(ctx context.Context, db *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return translate(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return translate(err)
	}
	return translate(tx.Commit(ctx))
}

func queryAll[T any](ctx context.Context, db *pgxpool.Pool, scan func(pgx.Rows) (T, error), query string, args ...any) ([]T, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	out := make([]T, 0)
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, translate(err)
		}
		out = append(out, v)
	}
	return out, translate(rows.Err())
}

// replaceSet atomically replaces a candidate's rows in a link table.
func replaceSet[T any](ctx context.Context, db *pgxpool.Pool, deleteQ, insertQ string, candidateID int64, items []T) error {
	return inTx(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, deleteQ, candidateID); err != nil {
			return err
		}
		for _, it := range items {
			if _, err := tx.Exec(ctx, insertQ, candidateID, it); err != nil {
				return err
			}
		}
		return nil
	})
}

func scanString(rs pgx.Rows) (string, error) {
	var s string
	err := rs.Scan(&s)
	return s, err
}

// likePattern escapes LIKE wildcards in user input and wraps it for a contains-search.
func likePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(q) + "%"
}

func decodeInto(raw []byte, dst any) error {
	return json.Unmarshal(raw, dst)
}

type masterTable struct{ table, idCol string }

var masterTables = map[models.MasterKind]masterTable{
	models.MasterDesignations: {"master_designations", "designation_id"},
	models.MasterSkills:       {"master_skills", "skill_id"},
}

func tableFor(kind models.MasterKind) (masterTable, error) {
	t, ok := masterTables[kind]
	if !ok {
		return t, &apperror.Error{Status: http.StatusNotFound, Message: "unknown master type"}
	}
	return t, nil
}

func scanMaster(rs pgx.Rows) (models.MasterEntry, error) {
	var m models.MasterEntry
	err := rs.Scan(&m.ID, &m.Name, &m.IsAdminApproved, &m.CreatedAt)
	return m, err
}
