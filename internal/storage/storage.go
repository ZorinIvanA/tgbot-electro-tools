package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Storage represents database storage interface
type Storage interface {
	// User operations
	GetOrCreateUser(telegramID int64) (*User, error)
	UpdateUserMessageCount(telegramID int64) error
	UpdateUserFSMState(telegramID int64, state string) error
	UpdateUserEmail(telegramID int64, email string, consentGranted bool) error
	GetUser(telegramID int64) (*User, error)

	// Settings operations
	GetSettings() (*Settings, error)
	UpdateSettings(settings *Settings) error

	// Message logging
	LogMessage(userID int64, text string, direction string) error

	// Metrics
	GetActiveUsersCount24h() (int64, error)
	GetTotalMessagesCount() (int64, error)
	GetUsersByFSMState() (map[string]int64, error)

	// Rate limiting
	CheckRateLimit(telegramID int64, maxPerMinute int) (bool, error)

	// Close database connection
	Close() error
}

// User represents a user in the system
type User struct {
	ID             int64
	TelegramID     int64
	MessageCount   int
	FSMState       string
	Email          string
	ConsentGranted bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Settings represents bot configuration
type Settings struct {
	ID                  int
	TriggerMessageCount int
	SiteURL             string
	UpdatedAt           time.Time
}

// PostgresStorage implements Storage interface for PostgreSQL
type PostgresStorage struct {
	db *sql.DB
}

// NewPostgresStorage creates a new PostgreSQL storage instance
func NewPostgresStorage(host, port, user, password, dbname, sslmode string) (*PostgresStorage, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &PostgresStorage{db: db}, nil
}

// GetOrCreateUser retrieves or creates a user
func (s *PostgresStorage) GetOrCreateUser(telegramID int64) (*User, error) {
	user := &User{}

	query := `
		INSERT INTO users (telegram_id, message_count, fsm_state)
		VALUES ($1, 0, 'idle')
		ON CONFLICT (telegram_id) DO NOTHING
		RETURNING id, telegram_id, message_count, fsm_state, email, consent_granted, created_at, updated_at
	`

	err := s.db.QueryRow(query, telegramID).Scan(
		&user.ID,
		&user.TelegramID,
		&user.MessageCount,
		&user.FSMState,
		&user.Email,
		&user.ConsentGranted,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// User already exists, get it
		return s.GetUser(telegramID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get or create user: %w", err)
	}

	return user, nil
}

// UpdateUserMessageCount increments user's message count
func (s *PostgresStorage) UpdateUserMessageCount(telegramID int64) error {
	query := `UPDATE users SET message_count = message_count + 1, updated_at = NOW() WHERE telegram_id = $1`
	_, err := s.db.Exec(query, telegramID)
	if err != nil {
		return fmt.Errorf("failed to update message count: %w", err)
	}
	return nil
}

// UpdateUserFSMState updates user's FSM state
func (s *PostgresStorage) UpdateUserFSMState(telegramID int64, state string) error {
	query := `UPDATE users SET fsm_state = $1, updated_at = NOW() WHERE telegram_id = $2`
	_, err := s.db.Exec(query, state, telegramID)
	if err != nil {
		return fmt.Errorf("failed to update FSM state: %w", err)
	}
	return nil
}

// UpdateUserEmail updates user's email and consent
func (s *PostgresStorage) UpdateUserEmail(telegramID int64, email string, consentGranted bool) error {
	query := `UPDATE users SET email = $1, consent_granted = $2, updated_at = NOW() WHERE telegram_id = $3`
	_, err := s.db.Exec(query, email, consentGranted, telegramID)
	if err != nil {
		return fmt.Errorf("failed to update user email: %w", err)
	}
	return nil
}

// GetUser retrieves a user by Telegram ID
func (s *PostgresStorage) GetUser(telegramID int64) (*User, error) {
	user := &User{}
	query := `
		SELECT id, telegram_id, message_count, fsm_state, email, consent_granted, created_at, updated_at
		FROM users
		WHERE telegram_id = $1
	`

	err := s.db.QueryRow(query, telegramID).Scan(
		&user.ID,
		&user.TelegramID,
		&user.MessageCount,
		&user.FSMState,
		&user.Email,
		&user.ConsentGranted,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetSettings retrieves bot settings
func (s *PostgresStorage) GetSettings() (*Settings, error) {
	settings := &Settings{}
	query := `SELECT id, trigger_message_count, site_url, updated_at FROM settings WHERE id = 1`

	err := s.db.QueryRow(query).Scan(
		&settings.ID,
		&settings.TriggerMessageCount,
		&settings.SiteURL,
		&settings.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}

	return settings, nil
}

// UpdateSettings updates bot settings
func (s *PostgresStorage) UpdateSettings(settings *Settings) error {
	query := `UPDATE settings SET trigger_message_count = $1, site_url = $2, updated_at = NOW() WHERE id = 1`
	_, err := s.db.Exec(query, settings.TriggerMessageCount, settings.SiteURL)
	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}
	return nil
}

// LogMessage logs a message to the database
func (s *PostgresStorage) LogMessage(userID int64, text string, direction string) error {
	query := `INSERT INTO messages (user_id, message_text, direction) VALUES ($1, $2, $3)`
	_, err := s.db.Exec(query, userID, text, direction)
	if err != nil {
		return fmt.Errorf("failed to log message: %w", err)
	}
	return nil
}

// GetActiveUsersCount24h returns count of unique users in last 24 hours
func (s *PostgresStorage) GetActiveUsersCount24h() (int64, error) {
	var count int64
	query := `
		SELECT COUNT(DISTINCT user_id)
		FROM messages
		WHERE created_at >= NOW() - INTERVAL '24 hours'
	`

	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get active users count: %w", err)
	}

	return count, nil
}

// GetTotalMessagesCount returns total count of all messages
func (s *PostgresStorage) GetTotalMessagesCount() (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM messages`

	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get total messages count: %w", err)
	}

	return count, nil
}

// GetUsersByFSMState returns count of users per FSM state
func (s *PostgresStorage) GetUsersByFSMState() (map[string]int64, error) {
	query := `SELECT fsm_state, COUNT(*) FROM users GROUP BY fsm_state`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by FSM state: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var state string
		var count int64
		if err := rows.Scan(&state, &count); err != nil {
			return nil, fmt.Errorf("failed to scan FSM state row: %w", err)
		}
		result[state] = count
	}

	return result, nil
}

// CheckRateLimit checks if user exceeded rate limit
func (s *PostgresStorage) CheckRateLimit(telegramID int64, maxPerMinute int) (bool, error) {
	now := time.Now().Unix()
	oneMinuteAgo := now - 60

	// Get existing timestamps
	var timestamps []int64
	query := `SELECT message_timestamps FROM rate_limits WHERE telegram_id = $1`

	var timestampsArray []byte
	err := s.db.QueryRow(query, telegramID).Scan(&timestampsArray)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("failed to get rate limit data: %w", err)
	}

	// Parse timestamps from PostgreSQL array
	if err != sql.ErrNoRows && len(timestampsArray) > 0 {
		// Simple parsing for bigint array
		timestampQuery := `SELECT unnest(message_timestamps) FROM rate_limits WHERE telegram_id = $1`
		rows, err := s.db.Query(timestampQuery, telegramID)
		if err != nil {
			return false, err
		}
		defer rows.Close()

		for rows.Next() {
			var ts int64
			if err := rows.Scan(&ts); err != nil {
				continue
			}
			if ts >= oneMinuteAgo {
				timestamps = append(timestamps, ts)
			}
		}
	}

	// Check if rate limit exceeded
	if len(timestamps) >= maxPerMinute {
		return false, nil // Rate limit exceeded
	}

	// Add current timestamp
	timestamps = append(timestamps, now)

	// Update database
	upsertQuery := `
		INSERT INTO rate_limits (telegram_id, message_timestamps, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (telegram_id)
		DO UPDATE SET message_timestamps = $2, updated_at = NOW()
	`

	_, err = s.db.Exec(upsertQuery, telegramID, timestamps)
	if err != nil {
		return false, fmt.Errorf("failed to update rate limit: %w", err)
	}

	return true, nil
}

// Close closes the database connection
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}
