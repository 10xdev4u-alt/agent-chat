package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Agent represents a registered agent.
type Agent struct {
	ID         string
	Name       string
	Harness    string
	Repo       string
	Branch     string
	Status     string
	Doing      string
	APIKeyHash string
	Role       string
	CreatedAt  string
	UpdatedAt  string
}

func generateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return "bus_" + hex.EncodeToString(b), nil
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// CreateAgent creates an agent, generates an API key, stores its bcrypt hash, and returns id and key.
func CreateAgent(db *sql.DB, name, harness, repo, branch, role string) (string, string, error) {
	if strings.TrimSpace(name) == "" {
		return "", "", fmt.Errorf("name is required")
	}
	if strings.TrimSpace(harness) == "" {
		return "", "", fmt.Errorf("harness is required")
	}
	if role == "" {
		role = "agent"
	}
	id := newID()
	if id == "" {
		return "", "", fmt.Errorf("generate id")
	}
	key, err := generateAPIKey()
	if err != nil {
		return "", "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("hash key: %w", err)
	}
	_, err = db.Exec(
		`INSERT INTO agents (id, name, harness, repo, branch, api_key_hash, role) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, name, harness, nullable(repo), nullable(branch), string(hash), role,
	)
	if err != nil {
		return "", "", fmt.Errorf("insert agent: %w", err)
	}
	return id, key, nil
}

func nullable(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// GetAgent looks up an agent by ID.
func GetAgent(db *sql.DB, id string) (*Agent, error) {
	row := db.QueryRow(
		`SELECT id, name, harness, repo, branch, status, doing, api_key_hash, role, created_at, updated_at FROM agents WHERE id = ?`, id)
	return scanAgent(row)
}

// GetAgentByKeyHash looks up an agent by stored api_key_hash (string equality).
func GetAgentByKeyHash(db *sql.DB, hash string) (*Agent, error) {
	row := db.QueryRow(
		`SELECT id, name, harness, repo, branch, status, doing, api_key_hash, role, created_at, updated_at FROM agents WHERE api_key_hash = ?`, hash)
	return scanAgent(row)
}

// GetAgentByKey finds the agent whose stored hash matches the given raw key (bcrypt compare).
func GetAgentByKey(db *sql.DB, key string) (*Agent, error) {
	rows, err := db.Query(
		`SELECT id, name, harness, repo, branch, status, doing, api_key_hash, role, created_at, updated_at FROM agents`)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		a, err := scanAgentRows(rows)
		if err != nil {
			return nil, err
		}
		if bcrypt.CompareHashAndPassword([]byte(a.APIKeyHash), []byte(key)) == nil {
			return a, nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("agent not found")
}

// UpdateAgentStatus updates an agent's status and doing text.
func UpdateAgentStatus(db *sql.DB, id, status, doing string) error {
	res, err := db.Exec(
		`UPDATE agents SET status = ?, doing = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id = ?`,
		status, nullable(doing), id,
	)
	if err != nil {
		return fmt.Errorf("update agent status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("agent not found: %s", id)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAgent(row *sql.Row) (*Agent, error) {
	var a Agent
	var repo, branch, doing sql.NullString
	err := row.Scan(&a.ID, &a.Name, &a.Harness, &repo, &branch, &a.Status, &doing, &a.APIKeyHash, &a.Role, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	a.Repo = repo.String
	a.Branch = branch.String
	a.Doing = doing.String
	return &a, nil
}

// ListAgents returns all registered agents.
func ListAgents(db *sql.DB) ([]Agent, error) {
	rows, err := db.Query(`SELECT id, name, harness, repo, branch, status, doing, api_key_hash, role, created_at, updated_at FROM agents ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []Agent
	for rows.Next() {
		a, err := scanAgentRows(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, *a)
	}
	return agents, rows.Err()
}

func scanAgentRows(rows *sql.Rows) (*Agent, error) {
	var a Agent
	var repo, branch, doing sql.NullString
	err := rows.Scan(&a.ID, &a.Name, &a.Harness, &repo, &branch, &a.Status, &doing, &a.APIKeyHash, &a.Role, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	a.Repo = repo.String
	a.Branch = branch.String
	a.Doing = doing.String
	return &a, nil
}
