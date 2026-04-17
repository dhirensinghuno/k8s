package diagnose

import (
	"context"
	"time"

	"github.com/k8s-sre/agent/internal/k8s"
	"github.com/k8s-sre/agent/internal/models"
)

type Diagnoser struct {
	client      *k8s.Client
	history     map[string]*IssueContext
	deployments map[string]time.Time
}

type IssueContext struct {
	StartedAt  time.Time
	DeployTime time.Time
	EventCount int
	PrevImages map[string]string
}

func NewDiagnoser(client *k8s.Client) *Diagnoser {
	return &Diagnoser{
		client:      client,
		history:     make(map[string]*IssueContext),
		deployments: make(map[string]time.Time),
	}
}

func (d *Diagnoser) Diagnose(ctx context.Context, issue models.Issue) (*models.Issue, error) {
	evidence := make([]string, 0)

	evidence = append(evidence, "Issue: "+string(issue.Type))
	evidence = append(evidence, "Namespace: "+issue.Namespace)
	evidence = append(evidence, "Pod: "+issue.Pod)
	evidence = append(evidence, "Reason: "+issue.Reason)
	evidence = append(evidence, "Message: "+issue.Message)

	var rootCause models.RootCause

	switch issue.Type {
	case models.IssueCrashLoopBackOff:
		rootCause, evidence = d.diagnoseCrashLoop(ctx, issue, evidence)
	case models.IssueOOMKilled:
		rootCause = models.RootCauseResourceExhaustion
		evidence = append(evidence, "Container was killed due to out of memory")
		evidence = append(evidence, "Memory limit may be too low for workload")
	case models.IssueImagePullBackOff, models.IssueErrImagePull:
		rootCause = models.RootCauseBadImage
		evidence = append(evidence, "Failed to pull container image")
		evidence = append(evidence, "Check image name, tag, and registry access")
	case models.IssuePending:
		rootCause = d.diagnosePending(ctx, issue, evidence)
	case models.IssueReadinessFailure:
		rootCause = models.RootCauseMisconfiguration
		evidence = append(evidence, "Pod readiness probe failing")
		evidence = append(evidence, "Check application health endpoint and network connectivity")
	case models.IssueLivenessFailure:
		rootCause = models.RootCauseMisconfiguration
		evidence = append(evidence, "Pod liveness probe failing")
		evidence = append(evidence, "Check application health or increase probe timeout")
	case models.IssueNodeNotReady:
		rootCause = models.RootCauseNodeIssue
		evidence = append(evidence, "Node status is NotReady")
		evidence = append(evidence, "Check node health and kubelet status")
	case models.IssueNodePressure:
		rootCause = models.RootCauseNodeIssue
		evidence = append(evidence, "Node has resource pressure")
		evidence = append(evidence, "Check node CPU/memory/disk resources")
	case models.IssueHighRestart:
		rootCause = models.RootCauseTransient
		evidence = append(evidence, "Pod has excessive restarts")
		evidence = append(evidence, "May indicate transient issues or resource problems")
	default:
		rootCause = models.RootCauseUnknown
	}

	issue.RootCause = rootCause
	issue.Evidence = evidence

	return &issue, nil
}

func (d *Diagnoser) diagnoseCrashLoop(ctx context.Context, issue models.Issue, evidence []string) (models.RootCause, []string) {
	logs, err := d.client.GetPodLogs(ctx, issue.Namespace, issue.Pod, false)
	if err == nil && len(logs) > 0 {
		evidence = append(evidence, "Recent logs: "+truncate(logs, 500))
	}

	prevLogs, err := d.client.GetPodLogs(ctx, issue.Namespace, issue.Pod, true)
	if err == nil && len(prevLogs) > 0 {
		evidence = append(evidence, "Previous container logs: "+truncate(prevLogs, 500))
	}

	if d.isRecentDeployment(issue.Deployment) {
		evidence = append(evidence, "Deployment "+issue.Deployment+" was recently updated")
		return models.RootCauseBadImage, evidence
	}

	evidence = append(evidence, "No recent deployment detected")
	return models.RootCauseMisconfiguration, evidence
}

func (d *Diagnoser) diagnosePending(ctx context.Context, issue models.Issue, evidence []string) models.RootCause {
	pvcs, err := d.client.ListPVCs(ctx, issue.Namespace)
	if err == nil {
		for _, pvc := range pvcs {
			if pvc.Status.Phase == "Pending" {
				evidence = append(evidence, "Found pending PVC: "+pvc.Name)
				return models.RootCauseStorageIssue
			}
		}
	}

	evidence = append(evidence, "Pod may be waiting for scheduler or resources")
	return models.RootCauseResourceExhaustion
}

func (d *Diagnoser) isRecentDeployment(deployment string) bool {
	if deployment == "" {
		return false
	}
	depTime, ok := d.deployments[deployment]
	if !ok {
		return false
	}
	return time.Since(depTime) < 15*time.Minute
}

func (d *Diagnoser) RecordDeployment(namespace, name string) {
	key := namespace + "/" + name
	d.deployments[key] = time.Now()
}

func (d *Diagnoser) ShouldRollback(issue models.Issue) bool {
	if issue.Severity != models.SeverityCritical {
		return false
	}

	if issue.Deployment == "" {
		return false
	}

	key := issue.Namespace + "/" + issue.Deployment
	depTime, ok := d.deployments[key]
	if !ok {
		return false
	}

	if time.Since(depTime) > 15*time.Minute {
		return false
	}

	issueTypes := []models.IssueType{
		models.IssueCrashLoopBackOff,
		models.IssueImagePullBackOff,
		models.IssueErrImagePull,
	}

	for _, it := range issueTypes {
		if issue.Type == it {
			return true
		}
	}

	return false
}

func (d *Diagnoser) GetRootCauseDescription(rootCause models.RootCause) string {
	switch rootCause {
	case models.RootCauseResourceExhaustion:
		return "Container exceeded memory or CPU limits. Consider increasing resource limits."
	case models.RootCauseBadImage:
		return "Container image is invalid or inaccessible. Check image name, tag, and registry."
	case models.RootCauseMisconfiguration:
		return "Application configuration is incorrect. Check environment variables, configs, and probes."
	case models.RootCauseNodeIssue:
		return "Node hosting the pod has issues. Check node health and kubelet status."
	case models.RootCauseNetworkIssue:
		return "Network connectivity problem. Check network policies and CNI configuration."
	case models.RootCauseStorageIssue:
		return "Storage volume issue. Check PVC status and storage class availability."
	case models.RootCauseTransient:
		return "Transient issue that may resolve on its own. Pod restart may help."
	default:
		return "Unknown root cause. Manual investigation required."
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
