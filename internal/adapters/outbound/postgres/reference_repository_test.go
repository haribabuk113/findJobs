package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/reference"
)

func testPool(t *testing.T) *pgxpool.Pool {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		dsn = "postgres://dbuser:dbpass@127.0.0.1:5433/jportal"
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err == nil {
		err = pool.Ping(context.Background())
	}
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// names returns cleaned, test-unique names so runs don't collide with real data.
func names(t *testing.T, raw ...string) []reference.CleanedName {
	suffix := " zz" + uuid.NewString()[:8]
	out := make([]reference.CleanedName, len(raw))
	for i, r := range raw {
		c, err := reference.CleanName(r + suffix)
		if err != nil {
			t.Fatal(err)
		}
		out[i] = c
	}
	return out
}

func TestTaxonomyLifecycle(t *testing.T) {
	pool := testPool(t)
	repo := NewTaxonomyRepository(pool)
	ctx := context.Background()
	n := names(t, "Svelte", "Svelte.js", "SvelteJS")
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM skills WHERE name_norm = ANY($1)`,
			[]string{n[0].Normalized, n[1].Normalized, n[2].Normalized, n[0].Normalized + " renamed"})
	})

	// Custom terms are created as pending, in input order.
	terms, created, err := repo.Resolve(ctx, reference.Skill, n, "")
	if err != nil || !created || len(terms) != 3 || terms[0].Status != reference.StatusPending {
		t.Fatalf("resolve new: %+v created=%v err=%v", terms, created, err)
	}
	master, aliasA, aliasB := terms[0].ID, terms[1].ID, terms[2].ID

	// Resolving again creates nothing and returns the same ids.
	again, created, err := repo.Resolve(ctx, reference.Skill, n[:1], "")
	if err != nil || created || again[0].ID != master {
		t.Fatalf("resolve existing: %+v created=%v err=%v", again, created, err)
	}

	// Merge: aliases now resolve to the master; the master is verified.
	if err := repo.Merge(ctx, reference.Skill, []int64{aliasA}, master); err != nil {
		t.Fatal(err)
	}
	// Merging the master into another term flattens existing aliases onto it.
	if err := repo.Merge(ctx, reference.Skill, []int64{master}, aliasB); err != nil {
		t.Fatal(err)
	}
	resolved, _, err := repo.Resolve(ctx, reference.Skill, n, "")
	if err != nil {
		t.Fatal(err)
	}
	for i, term := range resolved {
		if term.ID != aliasB || term.Status != reference.StatusVerified {
			t.Errorf("name %d resolved to %+v, want verified master %d", i, term, aliasB)
		}
	}
	var depth int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM skills a JOIN skills b ON a.merged_into = b.id WHERE b.merged_into IS NOT NULL`).Scan(&depth)
	if depth != 0 {
		t.Errorf("%d alias chains deeper than one level", depth)
	}
	if err := repo.Merge(ctx, reference.Skill, []int64{aliasB}, master); !reference.IsValidation(err) {
		t.Errorf("merge into a merged term: want ValidationError, got %v", err)
	}

	// Export lists live terms only.
	exported, err := repo.Export(ctx, reference.Skill)
	if err != nil {
		t.Fatal(err)
	}
	for _, term := range exported {
		if term.ID == master || term.ID == aliasA {
			t.Errorf("merged term %d exported", term.ID)
		}
	}

	// Rename into an existing name conflicts; merged terms can't be edited.
	if _, err := repo.Update(ctx, reference.Skill, aliasB, &n[0], nil); !reference.IsConflict(err) {
		t.Errorf("rename to an alias name: want ConflictError, got %v", err)
	}
	if _, err := repo.Update(ctx, reference.Skill, master, nil, ptr("verified")); !reference.IsNotFound(err) {
		t.Errorf("update a merged term: want NotFoundError, got %v", err)
	}

	// Deleting the master removes its aliases too.
	if err := repo.Delete(ctx, reference.Skill, aliasB); err != nil {
		t.Fatal(err)
	}
	var left int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM skills WHERE id = ANY($1)`, []int64{master, aliasA, aliasB}).Scan(&left)
	if left != 0 {
		t.Errorf("%d terms left after deleting the master", left)
	}
}

func TestAssetSaveSkipsUnchangedAndPrunes(t *testing.T) {
	pool := testPool(t)
	repo := NewAssetRepository(pool)
	ctx := context.Background()
	// The kind CHECK only allows real kinds: snapshot that kind's rows and
	// restore them afterwards, so the dev database keeps its published file.
	kind := reference.AssetDesignations
	rows, err := pool.Query(ctx, `SELECT file, kind, content, sha256, created_at FROM published_assets WHERE kind = $1`, kind)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := pgx.CollectRows(rows, pgx.RowToStructByPos[struct {
		File, Kind string
		Content    []byte
		SHA256     string
		CreatedAt  time.Time
	}])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM published_assets WHERE kind = $1`, kind)
		for _, a := range snapshot {
			if _, err := pool.Exec(c, `INSERT INTO published_assets (file, kind, content, sha256, created_at) VALUES ($1, $2, $3, $4, $5)`,
				a.File, a.Kind, a.Content, a.SHA256, a.CreatedAt); err != nil {
				t.Errorf("restore %s: %v", a.File, err)
			}
		}
	})
	base := time.Now().Add(time.Hour) // newer than any real file
	var files []string
	save := func(i int, hash string) bool {
		file := "test-" + uuid.NewString() + ".json"
		files = append(files, file)
		stored, err := repo.Save(ctx, &reference.Asset{File: file, Kind: kind, Content: []byte("[]"), SHA256: hash, CreatedAt: base.Add(time.Duration(i) * time.Second)})
		if err != nil {
			t.Fatal(err)
		}
		return stored
	}
	// Other test files make the latest hash unpredictable, so start fresh.
	if !save(0, "h-"+uuid.NewString()) {
		t.Fatal("first save not stored")
	}
	same := "h-" + uuid.NewString()
	save(1, same)
	if save(2, same) {
		t.Error("identical content stored twice")
	}
	for i := 3; i < 8; i++ {
		save(i, "h-"+uuid.NewString())
	}
	var kept int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM published_assets WHERE kind = $1`, kind).Scan(&kept)
	if kept > keepVersions {
		t.Errorf("%d versions kept, want at most %d", kept, keepVersions)
	}
}

func ptr[T any](v T) *T { return &v }
