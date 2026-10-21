package database

import (
	"context"
	"os"
	"testing"

	"github.com/edkariuki/clinic-booking/migrations"
)

func TestMigrateURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"postgres://u:p@localhost:5432/db?sslmode=disable", "pgx5://u:p@localhost:5432/db?sslmode=disable"},
		{"postgresql://u:p@host/db", "pgx5://u:p@host/db"},
		{"pgx5://already", "pgx5://already"},
	}
	for _, tc := range tests {
		if got := migrateURL(tc.in); got != tc.want {
			t.Errorf("migrateURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMigrateIsSafeToRepeat(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set; skipping database tests")
	}

	for i := 1; i <= 2; i++ {
		if err := Migrate(url, migrations.FS); err != nil {
			t.Fatalf("Migrate call %d: %v", i, err)
		}
	}

	pool, err := Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.tables WHERE table_name IN ('doctors','doctor_schedules','appointments')`,
	).Scan(&n); err != nil || n != 3 {
		t.Errorf("found %d of the 3 tables (err %v)", n, err)
	}
}
