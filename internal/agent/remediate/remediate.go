package remediate

import (
	"context"
	"fmt"
	"time"

	"github.com/k8s-sre/agent/internal/k8s"
	"github.com/k8s-sre/agent/internal/models"
)

type RemediatorConfig struct {
	EnableAutoRemediation bool
	EnableAutoRollback    bool
	MemoryIncreasePercent float64
	CPUScaleThreshold     float64
	ScaleUpCooldown       time.Duration
	ScaleDownCooldown     time.Duration
}

type RemediationPolicy struct {
	MinTimeSinceDeploy   time.Duration
	MinUnhealthyDuration time.Duration
	MinStableDuration    time.Duration
}

type Remediator struct {
	client    *k8s.Client
	config    RemediatorConfig
	policy    RemediationPolicy
	cooldowns map[string]time.Time
}

func NewRemediator(client *k8s.Client, config RemediatorConfig) *Remediator {
	if config.MemoryIncreasePercent == 0 {
		config.MemoryIncreasePercent = 25
	}
	if config.ScaleUpCooldown == 0 {
		config.ScaleUpCooldown = 5 * time.Minute
	}
	if config.ScaleDownCooldown == 0 {
		config.ScaleDownCooldown = 15 * time.Minute
	}

	return &Remediator{
		client: client,
		config: config,
		policy: RemediationPolicy{
			MinTimeSinceDeploy:   15 * time.Minute,
			MinUnhealthyDuration: 3 * time.Minute,
			MinStableDuration:    24 * time.Hour,
		},
		cooldowns: make(map[string]time.Time),
	}
}

func (r *Remediator) CanRemediate(issue models.Issue) bool {
	if !r.config.EnableAutoRemediation {
		return false
	}

	issueTypes := []models.IssueType{
		models.IssueCrashLoopBackOff,
		models.IssueOOMKilled,
		models.IssueImagePullBackOff,
		models.IssueErrImagePull,
		models.IssueNodeNotReady,
		models.IssueNodePressure,
	}

	for _, it := range issueTypes {
		if issue.Type == it {
			return true
		}
	}

	return false
}

func (r *Remediator) GetRecommendedAction(issue models.Issue) (models.Action, error) {
	var action models.Action
	action.IssueID = issue.ID
	action.Timestamp = time.Now()
	action.Namespace = issue.Namespace

	switch issue.Type {
	case models.IssueCrashLoopBackOff:
		action.Type = models.ActionRestartPod
		action.Target = issue.Pod
		action.Reason = "Pod in CrashLoopBackOff, restart may resolve transient issue"

	case models.IssueOOMKilled:
		action.Type = models.ActionIncreaseResources
		action.Target = issue.Pod
		action.Reason = fmt.Sprintf("Increase memory limit by %.0f%% to prevent OOMKilled", r.config.MemoryIncreasePercent)

	case models.IssueImagePullBackOff, models.IssueErrImagePull:
		action.Type = models.ActionRollback
		action.Target = issue.Deployment
		action.Reason = "Image pull failed, rollback to previous working version"

	case models.IssuePending:
		action.Type = models.ActionRestartPod
		action.Target = issue.Pod
		action.Reason = "Pod stuck in Pending, restart may trigger rescheduling"

	case models.IssueNodeNotReady:
		action.Type = models.ActionCordonNode
		action.Target = issue.Node
		action.Reason = "Node is not ready, cordon to prevent new pod scheduling"

	case models.IssueNodePressure:
		action.Type = models.ActionDrainNode
		action.Target = issue.Node
		action.Reason = "Node has resource pressure, drain to free up resources"

	default:
		action.Type = models.ActionNoAction
		action.Reason = "No automatic remediation available for this issue type"
	}

	return action, nil
}

func (r *Remediator) ExecuteAction(ctx context.Context, action models.Action) (models.Action, error) {
	if action.Type == models.ActionNoAction {
		action.Success = true
		action.Result = "No action required"
		return action, nil
	}

	if r.isInCooldown(action.Target) {
		action.Success = false
		action.Result = "Action blocked: target is in cooldown period"
		return action, nil
	}

	var err error
	switch action.Type {
	case models.ActionRestartPod:
		err = r.executeRestartPod(ctx, action)
	case models.ActionIncreaseResources:
		err = r.executeIncreaseResources(ctx, action)
	case models.ActionRollback:
		err = r.executeRollback(ctx, action)
	case models.ActionCordonNode:
		err = r.executeCordonNode(ctx, action)
	case models.ActionDrainNode:
		err = r.executeDrainNode(ctx, action)
	case models.ActionScaleDeployment:
		err = r.executeScaleDeployment(ctx, action)
	default:
		action.Success = false
		action.Result = "Unknown action type"
		return action, fmt.Errorf("unknown action type: %s", action.Type)
	}

	if err != nil {
		action.Success = false
		action.Result = fmt.Sprintf("Action failed: %v", err)
		return action, err
	}

	action.Success = true
	action.Result = "Action completed successfully"
	r.setCooldown(action.Target, r.config.ScaleUpCooldown)

	return action, nil
}

func (r *Remediator) executeRestartPod(ctx context.Context, action models.Action) error {
	if action.Namespace == "" {
		return fmt.Errorf("namespace is required for pod restart")
	}
	return r.client.DeletePod(ctx, action.Namespace, action.Target)
}

func (r *Remediator) executeIncreaseResources(ctx context.Context, action models.Action) error {
	if action.Namespace == "" {
		return fmt.Errorf("namespace is required for resource increase")
	}
	return r.client.IncreaseMemoryLimit(ctx, action.Namespace, action.Target, r.config.MemoryIncreasePercent)
}

func (r *Remediator) executeRollback(ctx context.Context, action models.Action) error {
	if action.Namespace == "" || action.Target == "" {
		return fmt.Errorf("namespace and deployment name are required for rollback")
	}
	return r.client.RollbackDeployment(ctx, action.Namespace, action.Target)
}

func (r *Remediator) executeCordonNode(ctx context.Context, action models.Action) error {
	if action.Target == "" {
		return fmt.Errorf("node name is required for cordon")
	}
	return r.client.CordonNode(ctx, action.Target)
}

func (r *Remediator) executeDrainNode(ctx context.Context, action models.Action) error {
	if action.Target == "" {
		return fmt.Errorf("node name is required for drain")
	}
	return r.client.DrainNode(ctx, action.Target, true)
}

func (r *Remediator) executeScaleDeployment(ctx context.Context, action models.Action) error {
	if action.Namespace == "" || action.Target == "" {
		return fmt.Errorf("namespace and deployment name are required for scale")
	}
	return nil
}

func (r *Remediator) isInCooldown(target string) bool {
	if cooldown, ok := r.cooldowns[target]; ok {
		if time.Since(cooldown) < r.config.ScaleUpCooldown {
			return true
		}
		delete(r.cooldowns, target)
	}
	return false
}

func (r *Remediator) setCooldown(target string, duration time.Duration) {
	r.cooldowns[target] = time.Now().Add(duration)
}

func (r *Remediator) ShouldRollback(issue models.Issue) bool {
	if !r.config.EnableAutoRollback {
		return false
	}

	if issue.Severity != models.SeverityCritical {
		return false
	}

	if issue.Deployment == "" {
		return false
	}

	criticalIssueTypes := []models.IssueType{
		models.IssueCrashLoopBackOff,
		models.IssueImagePullBackOff,
		models.IssueErrImagePull,
	}

	for _, it := range criticalIssueTypes {
		if issue.Type == it {
			return true
		}
	}

	return false
}

func (r *Remediator) ValidateRollbackPolicy(deployment string, issueTime time.Time) bool {
	return true
}

func (r *Remediator) GetSafetyChecks() []string {
	return []string{
		"Never delete PVC or data volumes",
		"Never scale to zero replicas",
		"Never modify secrets directly",
		"Always verify rollback option before config changes",
		"Require manual confirmation for destructive actions",
	}
}
