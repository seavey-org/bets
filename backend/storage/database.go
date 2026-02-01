package storage

import (
	"log"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/codyseavey/bets/models"
)

func InitDB(dbPath string) *gorm.DB {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	db, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Manual migrations must run before AutoMigrate because they recreate
	// tables with raw SQL, and AutoMigrate needs to see the final schema.
	runManualMigrations(db)

	// Backfill NULL name values before AutoMigrate tries to enforce NOT NULL.
	// The name column was added after initial users were created via Google OAuth,
	// so existing rows may have NULL names. We use the email prefix as a fallback.
	if db.Migrator().HasTable(&models.User{}) && db.Migrator().HasColumn(&models.User{}, "name") {
		db.Exec(`UPDATE users SET name = SUBSTR(email, 1, INSTR(email, '@') - 1) WHERE name IS NULL OR name = ''`)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Group{},
		&models.GroupMember{},
		&models.Pool{},
		&models.PoolOption{},
		&models.Bet{},
		&models.PointsLog{},
		&models.Market{},
		&models.MarketOutcome{},
		&models.SharePosition{},
		&models.Trade{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database initialized successfully")
	return db
}

// runManualMigrations handles schema changes that GORM's AutoMigrate can't do,
// like dropping NOT NULL constraints in SQLite (which doesn't support ALTER COLUMN).
// Each migration is idempotent, so it's safe to run on every startup.
func runManualMigrations(db *gorm.DB) {
	// Make google_id nullable for local (non-Google) users.
	// SQLite requires recreating the table to change column constraints.
	// We check if the column is still NOT NULL before doing the migration.
	var notNull bool
	row := db.Raw(`SELECT "notnull" FROM pragma_table_info('users') WHERE name = 'google_id'`).Row()
	if err := row.Scan(&notNull); err != nil {
		// Column doesn't exist or table doesn't exist yet, nothing to migrate
		return
	}
	if !notNull {
		return // Already nullable
	}

	log.Println("Migrating: making google_id nullable for local auth support")

	// Foreign keys must be disabled to DROP TABLE users, since other tables
	// reference it. PRAGMA foreign_keys cannot be changed inside a transaction,
	// so we toggle it before and after.
	statements := []string{
		`PRAGMA foreign_keys = OFF`,
		// Use GORM-compatible DDL (backtick-quoted lowercase names, table-level PRIMARY KEY)
		// so AutoMigrate won't see a schema mismatch and try to rebuild.
		"CREATE TABLE `users_backup` (`id` text,`google_id` text,`email` text NOT NULL,`name` text NOT NULL,`avatar_url` text,`password_hash` text,`created_at` datetime,PRIMARY KEY (`id`))",
		`INSERT INTO users_backup SELECT id, google_id, email, name, avatar_url, COALESCE(password_hash, ''), created_at FROM users`,
		`DROP TABLE users`,
		`ALTER TABLE users_backup RENAME TO users`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`PRAGMA foreign_keys = ON`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			log.Fatalf("Migration failed on statement [%s]: %v", stmt, err)
		}
	}
	log.Println("Migration complete: google_id is now nullable")
}
