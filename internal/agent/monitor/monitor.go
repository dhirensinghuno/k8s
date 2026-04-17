package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/k8s-sre/agent/internal/k8s"
	"github.com/k8s-sre/agent/internal/models"
)

type Monitor struct {
	client       *k8s.Client
	pods         []models.Pod
	nodes        []models.Node
	deployments  []models.Deployment
	events       []models.Event
	health       *models.ClusterHealth
	mu           sync.RWMutex
	stopCh       chan struct{}
	onIssue      func(issue models.Issue)
	onEvent      func(event models.Event)
	pollInterval time.Duration
}

type MonitorConfig struct {
	PollInterval time.Duration
	OnIssue      func(issue models.Issue)
	OnEvent      func(event models.Event)
}

func NewMonitor(client *k8s.Client, cfg MonitorConfig) *Monitor {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 10 * time.Second
	}
	return &Monitor{
		client:       client,
		pollInterval: cfg.PollInterval,
		onIssue:      cfg.OnIssue,
		onEvent:      cfg.OnEvent,
		stopCh:       make(chan struct{}),
		health: &models.ClusterHealth{
			OverallStatus: "unknown",
		},
	}
}

func (m *Monitor) Start(ctx context.Context) {
	refreshCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	m.refreshAll(refreshCtx)
	go m.runMonitorLoop(ctx)
}

func (m *Monitor) Stop() {
	close(m.stopCh)
}

func (m *Monitor) runMonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			go m.refreshAll(ctx)
		}
	}
}

func (m *Monitor) refreshAll(ctx context.Context) {
	var wg sync.WaitGroup

	wg.Add(4)

	go func() {
		defer wg.Done()
		pods, _ := m.client.ListPods(ctx, "all")
		if pods != nil {
			m.mu.Lock()
			m.pods = pods
			m.mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		nodes, _ := m.client.ListNodes(ctx)
		if nodes != nil {
			m.mu.Lock()
			m.nodes = nodes
			m.mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		deps, _ := m.client.ListDeployments(ctx, "all")
		if deps != nil {
			m.mu.Lock()
			m.deployments = deps
			m.mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		evts, _ := m.client.ListEvents(ctx, "all", true)
		if evts != nil {
			m.mu.Lock()
			m.events = evts
			m.mu.Unlock()
		}
	}()

	wg.Wait()
	health := m.calculateHealthUnsafe()
	m.mu.Lock()
	m.health = health
	m.mu.Unlock()
	m.detectIssues(ctx)
}

func (m *Monitor) calculateHealthUnsafe() *models.ClusterHealth {
	health := &models.ClusterHealth{
		Timestamp: time.Now(),
	}

	health.NodesTotal = len(m.nodes)
	for _, n := range m.nodes {
		if n.Ready {
			health.NodesReady++
		}
	}

	health.PodsTotal = len(m.pods)
	for _, p := range m.pods {
		if p.Status == models.PodStatusRunning {
			health.PodsRunning++
		}
		if len(p.IssueTypes) > 0 {
			health.PodsUnhealthy++
			for _, issue := range p.IssueTypes {
				if issue == models.IssueCrashLoopBackOff || issue == models.IssueOOMKilled ||
					issue == models.IssueImagePullBackOff || issue == models.IssuePending {
					health.CriticalIssues++
					break
				} else {
					health.WarningIssues++
					break
				}
			}
		}
	}

	for _, e := range m.events {
		if e.Type == "Warning" {
			health.WarningEvents++
		}
	}

	if health.CriticalIssues > 0 {
		health.OverallStatus = "critical"
	} else if health.WarningIssues > 0 || health.WarningEvents > 0 {
		health.OverallStatus = "warning"
	} else if health.NodesReady == health.NodesTotal {
		health.OverallStatus = "healthy"
	} else {
		health.OverallStatus = "degraded"
	}

	return health
}

func (m *Monitor) calculateHealth() *models.ClusterHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calculateHealthUnsafe()
}

func (m *Monitor) detectIssues(ctx context.Context) {
	m.mu.RLock()
	pods := m.pods
	nodes := m.nodes
	m.mu.RUnlock()

	for _, pod := range pods {
		for _, issueType := range pod.IssueTypes {
			issue := m.createIssueFromPod(ctx, pod, issueType)
			if m.onIssue != nil && issue != nil {
				m.onIssue(*issue)
			}
		}
	}

	for _, node := range nodes {
		if !node.Ready {
			issue := m.createIssueFromNode(node)
			if m.onIssue != nil && issue != nil {
				m.onIssue(*issue)
			}
		}
		for _, cond := range node.Conditions {
			if cond == "MemoryPressure:True" || cond == "DiskPressure:True" {
				issue := m.createNodePressureIssue(node, cond)
				if m.onIssue != nil && issue != nil {
					m.onIssue(*issue)
				}
			}
		}
	}
}

func (m *Monitor) createIssueFromPod(ctx context.Context, pod models.Pod, issueType models.IssueType) *models.Issue {
	severity := models.SeverityWarning
	if issueType == models.IssueCrashLoopBackOff || issueType == models.IssueOOMKilled ||
		issueType == models.IssueImagePullBackOff || issueType == models.IssuePending {
		severity = models.SeverityCritical
	}

	deployment := m.findDeploymentForPod(ctx, pod.Namespace, pod.Name)

	return &models.Issue{
		ID:         generateIssueID(pod.Namespace, pod.Name, issueType),
		Timestamp:  time.Now(),
		Severity:   severity,
		Type:       issueType,
		Namespace:  pod.Namespace,
		Pod:        pod.Name,
		Node:       pod.Node,
		Deployment: deployment,
		Reason:     pod.Reason,
		Message:    pod.Message,
	}
}

func (m *Monitor) createIssueFromNode(node models.Node) *models.Issue {
	return &models.Issue{
		ID:        generateIssueID("node", node.Name, models.IssueNodeNotReady),
		Timestamp: time.Now(),
		Severity:  models.SeverityCritical,
		Type:      models.IssueNodeNotReady,
		Namespace: "",
		Pod:       "",
		Node:      node.Name,
		Reason:    node.Status,
		Message:   "Node is not ready",
	}
}

func (m *Monitor) createNodePressureIssue(node models.Node, condition string) *models.Issue {
	return &models.Issue{
		ID:        generateIssueID("node", node.Name, models.IssueNodePressure),
		Timestamp: time.Now(),
		Severity:  models.SeverityWarning,
		Type:      models.IssueNodePressure,
		Namespace: "",
		Pod:       "",
		Node:      node.Name,
		Reason:    condition,
		Message:   "Node has pressure condition",
	}
}

func (m *Monitor) findDeploymentForPod(ctx context.Context, namespace, podName string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, dep := range m.deployments {
		if dep.Namespace == namespace {
			return dep.Name
		}
	}
	return ""
}

func (m *Monitor) GetPods() []models.Pod {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pods
}

func (m *Monitor) GetNodes() []models.Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nodes
}

func (m *Monitor) GetDeployments() []models.Deployment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.deployments
}

func (m *Monitor) GetEvents() []models.Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.events
}

func (m *Monitor) GetHealth() *models.ClusterHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.health
}

func generateIssueID(namespace, name string, issueType models.IssueType) string {
	return time.Now().Format("20060102150405") + "-" + namespace + "-" + name + "-" + string(issueType)
}
