package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/k8s-sre/agent/internal/models"
	_ "github.com/lib/pq"
)

type Store struct {
	db *sql.DB
}

func NewStore(host string, port int, user, password, dbname string) (*Store, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	store := &Store{db: db}
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

func NewStoreWithDSN(dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	store := &Store{db: db}
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS issues (
		id VARCHAR(255) PRIMARY KEY,
		timestamp TIMESTAMP NOT NULL,
		severity VARCHAR(20) NOT NULL,
		type VARCHAR(50) NOT NULL,
		namespace VARCHAR(255),
		pod VARCHAR(255),
		node VARCHAR(255),
		deployment VARCHAR(255),
		reason TEXT,
		message TEXT,
		root_cause VARCHAR(50),
		evidence TEXT[],
		resolved BOOLEAN DEFAULT FALSE,
		resolved_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS actions (
		id VARCHAR(255) PRIMARY KEY,
		timestamp TIMESTAMP NOT NULL,
		issue_id VARCHAR(255),
		type VARCHAR(50) NOT NULL,
		target VARCHAR(255) NOT NULL,
		namespace VARCHAR(255),
		reason TEXT,
		evidence TEXT[],
		success BOOLEAN NOT NULL,
		result TEXT,
		rollback_from BOOLEAN DEFAULT FALSE,
		prev_version TEXT,
		new_version TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (issue_id) REFERENCES issues(id)
	);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id SERIAL PRIMARY KEY,
		timestamp TIMESTAMP NOT NULL,
		level VARCHAR(20) NOT NULL,
		component VARCHAR(100) NOT NULL,
		message TEXT NOT NULL,
		issue_id VARCHAR(255),
		action_id VARCHAR(255),
		metadata JSONB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS cluster_snapshots (
		id SERIAL PRIMARY KEY,
		timestamp TIMESTAMP NOT NULL,
		health_status VARCHAR(20) NOT NULL,
		nodes_ready INT,
		nodes_total INT,
		pods_running INT,
		pods_total INT,
		pods_unhealthy INT,
		critical_issues INT,
		warning_issues INT,
		recent_actions INT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_issues_timestamp ON issues(timestamp);
	CREATE INDEX IF NOT EXISTS idx_issues_severity ON issues(severity);
	CREATE INDEX IF NOT EXISTS idx_issues_type ON issues(type);
	CREATE INDEX IF NOT EXISTS idx_issues_namespace ON issues(namespace);
	CREATE INDEX IF NOT EXISTS idx_issues_resolved ON issues(resolved);
	CREATE INDEX IF NOT EXISTS idx_actions_timestamp ON actions(timestamp);
	CREATE INDEX IF NOT EXISTS idx_actions_issue_id ON actions(issue_id);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_level ON audit_logs(level);
	`

	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) SaveIssue(issue *models.Issue) error {
	query := `
	INSERT INTO issues (id, timestamp, severity, type, namespace, pod, node, deployment, 
		reason, message, root_cause, evidence, resolved, resolved_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	ON CONFLICT (id) DO UPDATE SET
		resolved = EXCLUDED.resolved,
		resolved_at = EXCLUDED.resolved_at
	`

	_, err := s.db.Exec(query,
		issue.ID, issue.Timestamp, issue.Severity, issue.Type,
		issue.Namespace, issue.Pod, issue.Node, issue.Deployment,
		issue.Reason, issue.Message, issue.RootCause, issue.Evidence,
		issue.Resolved, issue.ResolvedAt)

	return err
}

func (s *Store) GetIssue(id string) (*models.Issue, error) {
	query := `
	SELECT id, timestamp, severity, type, namespace, pod, node, deployment,
		reason, message, root_cause, evidence, resolved, resolved_at
	FROM issues WHERE id = $1
	`

	issue := &models.Issue{}
	var evidence []string
	var resolvedAt sql.NullTime

	err := s.db.QueryRow(query, id).Scan(
		&issue.ID, &issue.Timestamp, &issue.Severity, &issue.Type,
		&issue.Namespace, &issue.Pod, &issue.Node, &issue.Deployment,
		&issue.Reason, &issue.Message, &issue.RootCause, &evidence,
		&issue.Resolved, &resolvedAt)

	if err != nil {
		return nil, err
	}

	issue.Evidence = evidence
	if resolvedAt.Valid {
		issue.ResolvedAt = &resolvedAt.Time
	}

	return issue, nil
}

func (s *Store) GetRecentIssues(limit int) ([]models.Issue, error) {
	query := `
	SELECT id, timestamp, severity, type, namespace, pod, node, deployment,
		reason, message, root_cause, evidence, resolved, resolved_at
	FROM issues ORDER BY timestamp DESC LIMIT $1
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanIssues(rows)
}

func (s *Store) GetUnresolvedIssues() ([]models.Issue, error) {
	query := `
	SELECT id, timestamp, severity, type, namespace, pod, node, deployment,
		reason, message, root_cause, evidence, resolved, resolved_at
	FROM issues WHERE resolved = FALSE ORDER BY timestamp DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanIssues(rows)
}

func (s *Store) ResolveIssue(id string) error {
	query := `UPDATE issues SET resolved = TRUE, resolved_at = $1 WHERE id = $2`
	_, err := s.db.Exec(query, time.Now(), id)
	return err
}

func (s *Store) scanIssues(rows *sql.Rows) ([]models.Issue, error) {
	var issues []models.Issue

	for rows.Next() {
		issue := models.Issue{}
		var evidence []string
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&issue.ID, &issue.Timestamp, &issue.Severity, &issue.Type,
			&issue.Namespace, &issue.Pod, &issue.Node, &issue.Deployment,
			&issue.Reason, &issue.Message, &issue.RootCause, &evidence,
			&issue.Resolved, &resolvedAt)

		if err != nil {
			return nil, err
		}

		issue.Evidence = evidence
		if resolvedAt.Valid {
			issue.ResolvedAt = &resolvedAt.Time
		}

		issues = append(issues, issue)
	}

	return issues, nil
}

func (s *Store) SaveAction(action *models.Action) error {
	query := `
	INSERT INTO actions (id, timestamp, issue_id, type, target, namespace, reason,
		evidence, success, result, rollback_from, prev_version, new_version)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err := s.db.Exec(query,
		action.ID, action.Timestamp, action.IssueID, action.Type,
		action.Target, action.Namespace, action.Reason, action.Evidence,
		action.Success, action.Result, action.RollbackFrom,
		action.PrevVersion, action.NewVersion)

	return err
}

func (s *Store) GetRecentActions(limit int) ([]models.Action, error) {
	query := `
	SELECT id, timestamp, issue_id, type, target, namespace, reason,
		evidence, success, result, rollback_from, prev_version, new_version
	FROM actions ORDER BY timestamp DESC LIMIT $1
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanActions(rows)
}

func (s *Store) scanActions(rows *sql.Rows) ([]models.Action, error) {
	var actions []models.Action

	for rows.Next() {
		action := models.Action{}
		var evidence []string

		err := rows.Scan(
			&action.ID, &action.Timestamp, &action.IssueID, &action.Type,
			&action.Target, &action.Namespace, &action.Reason, &evidence,
			&action.Success, &action.Result, &action.RollbackFrom,
			&action.PrevVersion, &action.NewVersion)

		if err != nil {
			return nil, err
		}

		action.Evidence = evidence
		actions = append(actions, action)
	}

	return actions, nil
}

func (s *Store) Log(level, component, message, issueID, actionID string) error {
	query := `
	INSERT INTO audit_logs (timestamp, level, component, message, issue_id, action_id)
	VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := s.db.Exec(query, time.Now(), level, component, message, issueID, actionID)
	return err
}

func (s *Store) GetAuditLogs(limit int) ([]models.AuditLog, error) {
	query := `
	SELECT id, timestamp, level, component, message, issue_id, action_id, metadata
	FROM audit_logs ORDER BY timestamp DESC LIMIT $1
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		log := models.AuditLog{}
		var issueID, actionID sql.NullString
		var metadata []byte

		err := rows.Scan(&log.ID, &log.Timestamp, &log.Level, &log.Component,
			&log.Message, &issueID, &actionID, &metadata)

		if err != nil {
			return nil, err
		}

		if issueID.Valid {
			log.IssueID = issueID.String
		}
		if actionID.Valid {
			log.ActionID = actionID.String
		}
		if metadata != nil {
			log.Metadata = string(metadata)
		}

		logs = append(logs, log)
	}

	return logs, nil
}

func (s *Store) SaveClusterSnapshot(health *models.ClusterHealth) error {
	query := `
	INSERT INTO cluster_snapshots (timestamp, health_status, nodes_ready, nodes_total,
		pods_running, pods_total, pods_unhealthy, critical_issues, warning_issues, recent_actions)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := s.db.Exec(query,
		health.Timestamp, health.OverallStatus, health.NodesReady, health.NodesTotal,
		health.PodsRunning, health.PodsTotal, health.PodsUnhealthy,
		health.CriticalIssues, health.WarningIssues, health.RecentActions)

	return err
}

func (s *Store) GetClusterHistory(hours int) ([]models.ClusterHealth, error) {
	query := `
	SELECT timestamp, health_status, nodes_ready, nodes_total,
		pods_running, pods_total, pods_unhealthy, critical_issues, warning_issues, recent_actions
	FROM cluster_snapshots
	WHERE timestamp > NOW() - INTERVAL '%d hours'
	ORDER BY timestamp DESC
	`

	rows, err := s.db.Query(fmt.Sprintf(query, hours))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.ClusterHealth
	for rows.Next() {
		h := models.ClusterHealth{}
		err := rows.Scan(&h.Timestamp, &h.OverallStatus, &h.NodesReady, &h.NodesTotal,
			&h.PodsRunning, &h.PodsTotal, &h.PodsUnhealthy,
			&h.CriticalIssues, &h.WarningIssues, &h.RecentActions)

		if err != nil {
			return nil, err
		}
		history = append(history, h)
	}

	return history, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
