package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/k8s-sre/agent/internal/agent/diagnose"
	"github.com/k8s-sre/agent/internal/agent/monitor"
	"github.com/k8s-sre/agent/internal/agent/remediate"
	"github.com/k8s-sre/agent/internal/agent/rollback"
	"github.com/k8s-sre/agent/internal/k8s"
	"github.com/k8s-sre/agent/internal/models"
	"github.com/k8s-sre/agent/internal/store"
)

type Agent struct {
	client       *k8s.Client
	monitor      *monitor.Monitor
	diagnoser    *diagnose.Diagnoser
	remediator   *remediate.Remediator
	rollbackMgr  *rollback.RollbackManager
	store        *store.Store
	config       *Config
	issues       []models.Issue
	actions      []models.Action
	issueHistory map[string]*models.Issue
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

type Config struct {
	PollInterval          time.Duration
	EnableAutoRemediation bool
	EnableAutoRollback    bool
	MemoryIncreasePercent float64
	RollbackThreshold     time.Duration
}

func DefaultConfig() *Config {
	return &Config{
		PollInterval:          10 * time.Second,
		EnableAutoRemediation: true,
		EnableAutoRollback:    true,
		MemoryIncreasePercent: 25,
		RollbackThreshold:     15 * time.Minute,
	}
}

func NewAgent(client *k8s.Client, store *store.Store, cfg *Config) *Agent {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	remediateCfg := remediate.RemediatorConfig{
		EnableAutoRemediation: cfg.EnableAutoRemediation,
		EnableAutoRollback:    cfg.EnableAutoRollback,
		MemoryIncreasePercent: cfg.MemoryIncreasePercent,
	}

	a := &Agent{
		client:       client,
		diagnoser:    diagnose.NewDiagnoser(client),
		remediator:   remediate.NewRemediator(client, remediateCfg),
		rollbackMgr:  rollback.NewRollbackManager(client),
		store:        store,
		config:       cfg,
		issues:       make([]models.Issue, 0),
		actions:      make([]models.Action, 0),
		issueHistory: make(map[string]*models.Issue),
		ctx:          ctx,
		cancel:       cancel,
	}

	a.monitor = monitor.NewMonitor(client, monitor.MonitorConfig{
		PollInterval: cfg.PollInterval,
		OnIssue:      a.handleIssue,
		OnEvent:      a.handleEvent,
	})

	return a
}

func (a *Agent) Start() error {
	a.monitor.Start(a.ctx)

	if a.store != nil {
		go a.storeLoop()
	}

	a.log("info", "Agent", "Kubernetes SRE Agent started")
	return nil
}

func (a *Agent) Stop() {
	a.cancel()
	a.monitor.Stop()
	a.log("info", "Agent", "Kubernetes SRE Agent stopped")
}

func (a *Agent) handleIssue(issue models.Issue) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := fmt.Sprintf("%s/%s/%s", issue.Namespace, issue.Pod, issue.Type)

	if existing, ok := a.issueHistory[key]; ok {
		if existing.Resolved {
			issue.Resolved = false
			issue.ResolvedAt = nil
		}
	}

	a.issues = append(a.issues, issue)
	if len(a.issues) > 1000 {
		a.issues = a.issues[len(a.issues)-1000:]
	}

	a.issueHistory[key] = &issue

	if a.store != nil {
		a.store.SaveIssue(&issue)
	}

	ctx := context.Background()
	diagnosed, _ := a.diagnoser.Diagnose(ctx, issue)

	a.log("info", "Diagnoser", fmt.Sprintf("Issue detected: %s %s/%s - %s",
		issue.Type, issue.Namespace, issue.Pod, diagnosed.RootCause))

	if a.config.EnableAutoRemediation && a.remediator.CanRemediate(issue) {
		a.autoRemediate(issue)
	}
}

func (a *Agent) handleEvent(event models.Event) {
	if a.store != nil {
		a.store.Log("info", "Event", fmt.Sprintf("%s: %s - %s",
			event.Type, event.Reason, event.Message), "", "")
	}
}

func (a *Agent) autoRemediate(issue models.Issue) {
	action, err := a.remediator.GetRecommendedAction(issue)
	if err != nil || action.Type == models.ActionNoAction {
		return
	}

	if a.remediator.ShouldRollback(issue) {
		action.Type = models.ActionRollback
		action.Target = issue.Deployment
		action.Reason = fmt.Sprintf("Auto-rollback due to %s", issue.Type)
	}

	action.ID = fmt.Sprintf("action-%d", time.Now().UnixNano())
	action.IssueID = issue.ID

	ctx := context.Background()
	executedAction, err := a.remediator.ExecuteAction(ctx, action)

	a.mu.Lock()
	a.actions = append(a.actions, executedAction)
	if len(a.actions) > 1000 {
		a.actions = a.actions[len(a.actions)-1000:]
	}
	a.mu.Unlock()

	if a.store != nil {
		a.store.SaveAction(&executedAction)
	}

	if executedAction.Success {
		a.log("info", "Remediator", fmt.Sprintf("Action executed: %s %s - %s",
			executedAction.Type, executedAction.Target, executedAction.Result))
	} else {
		a.log("warn", "Remediator", fmt.Sprintf("Action failed: %s %s - %s",
			executedAction.Type, executedAction.Target, executedAction.Result))
	}
}

func (a *Agent) Diagnose(ctx context.Context, issue models.Issue) (*models.Issue, error) {
	return a.diagnoser.Diagnose(ctx, issue)
}

func (a *Agent) Remediate(ctx context.Context, issueID string, force bool) (*models.Action, error) {
	var issue models.Issue
	found := false

	a.mu.RLock()
	for _, i := range a.issues {
		if i.ID == issueID {
			issue = i
			found = true
			break
		}
	}
	a.mu.RUnlock()

	if !found && a.store != nil {
		stored, err := a.store.GetIssue(issueID)
		if err == nil {
			issue = *stored
			found = true
		}
	}

	if !found {
		return nil, fmt.Errorf("issue not found: %s", issueID)
	}

	action, err := a.remediator.GetRecommendedAction(issue)
	if err != nil {
		return nil, err
	}

	if !force && !a.remediator.CanRemediate(issue) {
		return nil, fmt.Errorf("issue not eligible for auto-remediation")
	}

	action.ID = fmt.Sprintf("action-%d", time.Now().UnixNano())
	action.IssueID = issue.ID

	executedAction, err := a.remediator.ExecuteAction(ctx, action)

	a.mu.Lock()
	a.actions = append(a.actions, executedAction)
	a.mu.Unlock()

	if a.store != nil {
		a.store.SaveAction(&executedAction)
	}

	return &executedAction, nil
}

func (a *Agent) Rollback(deployment, namespace, reason string) (*models.Action, error) {
	ctx := context.Background()

	rc, err := a.rollbackMgr.ExecuteRollback(ctx, deployment, namespace, reason)
	if err != nil {
		return nil, err
	}

	action := &models.Action{
		ID:           fmt.Sprintf("rollback-%d", time.Now().UnixNano()),
		Timestamp:    time.Now(),
		Type:         models.ActionRollback,
		Target:       deployment,
		Namespace:    namespace,
		Reason:       reason,
		Success:      rc.Success,
		Result:       rc.Result,
		RollbackFrom: true,
		PrevVersion:  rc.PrevImage,
		NewVersion:   rc.NewImage,
	}

	a.mu.Lock()
	a.actions = append(a.actions, *action)
	a.mu.Unlock()

	if a.store != nil {
		a.store.SaveAction(action)
	}

	a.log("info", "Rollback", fmt.Sprintf("Rollback executed for %s/%s: %s -> %s",
		namespace, deployment, rc.NewImage, rc.PrevImage))

	go a.rollbackMgr.MonitorAfterRollback(ctx, deployment, namespace)

	return action, nil
}

func (a *Agent) GetHealth() *models.ClusterHealth {
	return a.monitor.GetHealth()
}

func (a *Agent) GetPods(namespace string) []models.Pod {
	return a.monitor.GetPods()
}

func (a *Agent) GetNodes() []models.Node {
	return a.monitor.GetNodes()
}

func (a *Agent) GetDeployments(namespace string) []models.Deployment {
	return a.monitor.GetDeployments()
}

func (a *Agent) GetEvents(namespace string) []models.Event {
	return a.monitor.GetEvents()
}

func (a *Agent) GetIssues(limit int) []models.Issue {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit > len(a.issues) {
		return a.issues
	}
	return a.issues[len(a.issues)-limit:]
}

func (a *Agent) GetIssue(id string) (*models.Issue, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, issue := range a.issues {
		if issue.ID == id {
			return &issue, nil
		}
	}

	if a.store != nil {
		return a.store.GetIssue(id)
	}

	return nil, fmt.Errorf("issue not found: %s", id)
}

func (a *Agent) ResolveIssue(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.issues {
		if a.issues[i].ID == id {
			a.issues[i].Resolved = true
			now := time.Now()
			a.issues[i].ResolvedAt = &now

			if a.store != nil {
				a.store.ResolveIssue(id)
			}
			return nil
		}
	}

	if a.store != nil {
		return a.store.ResolveIssue(id)
	}

	return fmt.Errorf("issue not found: %s", id)
}

func (a *Agent) GetActions(limit int) []models.Action {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit > len(a.actions) {
		return a.actions
	}
	return a.actions[len(a.actions)-limit:]
}

func (a *Agent) GetAction(id string) (*models.Action, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, action := range a.actions {
		if action.ID == id {
			return &action, nil
		}
	}

	return nil, fmt.Errorf("action not found: %s", id)
}

func (a *Agent) GetAuditLogs(limit int) []models.AuditLog {
	if a.store != nil {
		logs, _ := a.store.GetAuditLogs(limit)
		return logs
	}
	return nil
}

func (a *Agent) GetClusterHistory(hours int) []models.ClusterHealth {
	if a.store != nil {
		history, _ := a.store.GetClusterHistory(hours)
		return history
	}
	return nil
}

func (a *Agent) GetConfig() *Config {
	return a.config
}

func (a *Agent) UpdateConfig(cfg models.RemediationConfig) {
	a.config.EnableAutoRemediation = cfg.EnableAutoRemediation
	a.config.EnableAutoRollback = cfg.EnableAutoRollback
	a.config.MemoryIncreasePercent = cfg.MemoryIncreasePercent
}

func (a *Agent) storeLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			health := a.monitor.GetHealth()
			if health != nil && a.store != nil {
				a.store.SaveClusterSnapshot(health)
			}
		}
	}
}

func (a *Agent) log(level, component, message string) {
	if a.store != nil {
		a.store.Log(level, component, message, "", "")
	}
}
