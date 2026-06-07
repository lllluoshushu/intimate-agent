package memory

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"intimate-agent/internal/models"
)

// Store provides SQLite-backed memory CRUD operations.
type Store struct {
	db *sql.DB
}

// NewStore opens (or creates) the SQLite database and ensures the schema exists.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS user_profiles (
		user_id    TEXT PRIMARY KEY,
		name       TEXT NOT NULL DEFAULT '',
		age        INTEGER NOT NULL DEFAULT 0,
		occupation TEXT NOT NULL DEFAULT '',
		city       TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS memories (
		id            TEXT PRIMARY KEY,
		user_id       TEXT NOT NULL,
		category      TEXT NOT NULL,
		key           TEXT NOT NULL,
		value         TEXT NOT NULL,
		source        TEXT NOT NULL DEFAULT '',
		confidence    REAL NOT NULL DEFAULT 0.5,
		created_at    DATETIME NOT NULL,
		updated_at    DATETIME NOT NULL,
		superseded_by TEXT NOT NULL DEFAULT '',
		active        INTEGER NOT NULL DEFAULT 1,
		FOREIGN KEY (user_id) REFERENCES user_profiles(user_id)
	);

	CREATE INDEX IF NOT EXISTS idx_memories_user ON memories(user_id);
	CREATE INDEX IF NOT EXISTS idx_memories_active ON memories(user_id, active);
	CREATE INDEX IF NOT EXISTS idx_memories_category ON memories(user_id, category);
	`
	_, err := db.Exec(schema)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// --- User Profile ---

// GetOrCreateProfile retrieves or creates a user profile.
func (s *Store) GetOrCreateProfile(userID string) (models.UserProfile, error) {
	var p models.UserProfile
	err := s.db.QueryRow(
		"SELECT user_id, name, age, occupation, city, created_at, updated_at FROM user_profiles WHERE user_id = ?",
		userID,
	).Scan(&p.UserID, &p.Name, &p.Age, &p.Occupation, &p.City, &p.CreatedAt, &p.UpdatedAt)

	if err == sql.ErrNoRows {
		now := time.Now()
		p = models.UserProfile{UserID: userID, CreatedAt: now, UpdatedAt: now}
		_, err = s.db.Exec(
			"INSERT INTO user_profiles (user_id, name, age, occupation, city, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			p.UserID, p.Name, p.Age, p.Occupation, p.City, p.CreatedAt, p.UpdatedAt,
		)
		if err != nil {
			return p, fmt.Errorf("insert profile: %w", err)
		}
		return p, nil
	}
	if err != nil {
		return p, fmt.Errorf("query profile: %w", err)
	}
	return p, nil
}

// UpdateProfile updates a user profile field.
func (s *Store) UpdateProfile(userID string, field string, value interface{}) error {
	allowed := map[string]bool{"name": true, "age": true, "occupation": true, "city": true}
	if !allowed[field] {
		return fmt.Errorf("disallowed profile field: %s", field)
	}
	_, err := s.db.Exec(
		fmt.Sprintf("UPDATE user_profiles SET %s = ?, updated_at = ? WHERE user_id = ?", field),
		value, time.Now(), userID,
	)
	return err
}

// --- Memory CRUD ---

// AddMemory inserts a new memory item.
func (s *Store) AddMemory(item models.MemoryItem) error {
	_, err := s.db.Exec(
		`INSERT INTO memories (id, user_id, category, key, value, source, confidence, created_at, updated_at, superseded_by, active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.UserID, string(item.Category), item.Key, item.Value,
		item.Source, item.Confidence, item.CreatedAt, item.UpdatedAt,
		item.SupersededBy, boolToInt(item.Active),
	)
	return err
}

// GetActiveMemories returns all active (non-superseded) memories for a user.
func (s *Store) GetActiveMemories(userID string) ([]models.MemoryItem, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, category, key, value, source, confidence, created_at, updated_at, superseded_by, active
		 FROM memories WHERE user_id = ? AND active = 1 ORDER BY updated_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.MemoryItem
	for rows.Next() {
		var m models.MemoryItem
		var cat string
		var activeInt int
		if err := rows.Scan(&m.ID, &m.UserID, &cat, &m.Key, &m.Value,
			&m.Source, &m.Confidence, &m.CreatedAt, &m.UpdatedAt,
			&m.SupersededBy, &activeInt); err != nil {
			return nil, err
		}
		m.Category = models.MemoryCategory(cat)
		m.Active = activeInt == 1
		items = append(items, m)
	}
	return items, rows.Err()
}

// GetMemoriesByCategory returns active memories for a user filtered by category.
func (s *Store) GetMemoriesByCategory(userID string, cat models.MemoryCategory) ([]models.MemoryItem, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, category, key, value, source, confidence, created_at, updated_at, superseded_by, active
		 FROM memories WHERE user_id = ? AND category = ? AND active = 1 ORDER BY updated_at DESC`,
		userID, string(cat),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.MemoryItem
	for rows.Next() {
		var m models.MemoryItem
		var c string
		var activeInt int
		if err := rows.Scan(&m.ID, &m.UserID, &c, &m.Key, &m.Value,
			&m.Source, &m.Confidence, &m.CreatedAt, &m.UpdatedAt,
			&m.SupersededBy, &activeInt); err != nil {
			return nil, err
		}
		m.Category = models.MemoryCategory(c)
		m.Active = activeInt == 1
		items = append(items, m)
	}
	return items, rows.Err()
}

// GetMemoryByKey looks up a specific active memory by user, category, and key.
func (s *Store) GetMemoryByKey(userID string, cat models.MemoryCategory, key string) (*models.MemoryItem, error) {
	var m models.MemoryItem
	var c string
	var activeInt int
	err := s.db.QueryRow(
		`SELECT id, user_id, category, key, value, source, confidence, created_at, updated_at, superseded_by, active
		 FROM memories WHERE user_id = ? AND category = ? AND key = ? AND active = 1`,
		userID, string(cat), key,
	).Scan(&m.ID, &m.UserID, &c, &m.Key, &m.Value,
		&m.Source, &m.Confidence, &m.CreatedAt, &m.UpdatedAt,
		&m.SupersededBy, &activeInt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.Category = models.MemoryCategory(c)
	m.Active = activeInt == 1
	return &m, nil
}

// SupersedeMemory marks an old memory as superseded by a new one.
func (s *Store) SupersedeMemory(oldID, newID string) error {
	_, err := s.db.Exec(
		"UPDATE memories SET superseded_by = ?, active = 0, updated_at = ? WHERE id = ?",
		newID, time.Now(), oldID,
	)
	return err
}

// GetAllMemories returns all memories (including inactive) for a user.
func (s *Store) GetAllMemories(userID string) ([]models.MemoryItem, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, category, key, value, source, confidence, created_at, updated_at, superseded_by, active
		 FROM memories WHERE user_id = ? ORDER BY updated_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.MemoryItem
	for rows.Next() {
		var m models.MemoryItem
		var cat string
		var activeInt int
		if err := rows.Scan(&m.ID, &m.UserID, &cat, &m.Key, &m.Value,
			&m.Source, &m.Confidence, &m.CreatedAt, &m.UpdatedAt,
			&m.SupersededBy, &activeInt); err != nil {
			return nil, err
		}
		m.Category = models.MemoryCategory(cat)
		m.Active = activeInt == 1
		items = append(items, m)
	}
	return items, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
