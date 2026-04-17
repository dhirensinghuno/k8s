package rollback

import (
	"context"
	"fmt"
	"time"

	"github.com/k8s-sre/agent/internal/k8s"
	"github.com/k8s-sre/agent/internal/models"
)

type RollbackManager struct {
	client            *k8s.Client
	activeRollbacks   map[string]*RollbackContext
	deploymentHistory map[string][]DeploymentSnapshot
}

type RollbackContext struct {
	Deployment string
	Namespace  string
	StartedAt  time.Time
	PrevImage  string
	NewImage   string
	Reason     string
	Completed  bool
	Success    bool
	Result     string
}

type DeploymentSnapshot struct {
	Timestamp time.Time
	Image     string
	Reason    string
}

func NewRollbackManager(client *k8s.Client) *RollbackManager {
	return &RollbackManager{
		client:            client,
		activeRollbacks:   make(map[string]*RollbackContext),
		deploymentHistory: make(map[string][]DeploymentSnapshot),
	}
}

func (r *RollbackManager) ShouldRollback(issue models.Issue) bool {
	if issue.Severity != models.SeverityCritical {
		return false
	}

	if issue.Deployment == "" {
		return false
	}

	issueTypes := []models.IssueType{
		models.IssueCrashLoopBackOff,
		models.IssueImagePullBackOff,
		models.IssueErrImagePull,
		models.IssuePending,
	}

	for _, it := range issueTypes {
		if issue.Type == it {
			return true
		}
	}

	return false
}

func (r *RollbackManager) ValidateRollback(deployment, namespace string, issue models.Issue) (bool, string) {
	key := namespace + "/" + deployment

	snapshots, ok := r.deploymentHistory[key]
	if !ok || len(snapshots) < 2 {
		return false, "No previous version available for rollback"
	}

	if issue.Timestamp.Sub(r.getLastUpdate(key)) < 15*time.Minute {
		return true, "Issue started within 15 minutes of deployment"
	}

	return false, "Issue not related to recent deployment"
}

func (r *RollbackManager) getLastUpdate(key string) time.Time {
	snapshots, ok := r.deploymentHistory[key]
	if !ok || len(snapshots) == 0 {
		return time.Time{}
	}
	return snapshots[len(snapshots)-1].Timestamp
}

func (r *RollbackManager) ExecuteRollback(ctx context.Context, deployment, namespace string, reason string) (*RollbackContext, error) {
	key := namespace + "/" + deployment

	snapshots, ok := r.deploymentHistory[key]
	if !ok || len(snapshots) < 2 {
		currentDep, err := r.client.ListDeployments(ctx, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get deployment: %w", err)
		}
		for _, dep := range currentDep {
			if dep.Name == deployment {
				return nil, fmt.Errorf("no previous version available for rollback")
			}
		}
	}

	prevImage := snapshots[len(snapshots)-2].Image
	newImage := snapshots[len(snapshots)-1].Image

	rc := &RollbackContext{
		Deployment: deployment,
		Namespace:  namespace,
		StartedAt:  time.Now(),
		PrevImage:  prevImage,
		NewImage:   newImage,
		Reason:     reason,
	}
	r.activeRollbacks[key] = rc

	err := r.client.RollbackDeployment(ctx, namespace, deployment)
	if err != nil {
		rc.Completed = true
		rc.Success = false
		rc.Result = fmt.Sprintf("Rollback failed: %v", err)
		return rc, fmt.Errorf("rollback failed: %w", err)
	}

	rc.Completed = true
	rc.Success = true
	rc.Result = fmt.Sprintf("Successfully rolled back from %s to %s", newImage, prevImage)

	return rc, nil
}

func (r *RollbackManager) RecordDeployment(deployment, namespace, image, reason string) {
	key := namespace + "/" + deployment
	snapshot := DeploymentSnapshot{
		Timestamp: time.Now(),
		Image:     image,
		Reason:    reason,
	}

	if _, ok := r.deploymentHistory[key]; !ok {
		r.deploymentHistory[key] = make([]DeploymentSnapshot, 0)
	}

	r.deploymentHistory[key] = append(r.deploymentHistory[key], snapshot)

	if len(r.deploymentHistory[key]) > 10 {
		r.deploymentHistory[key] = r.deploymentHistory[key][len(r.deploymentHistory[key])-10:]
	}
}

func (r *RollbackManager) GetRollbackHistory(deployment, namespace string) []DeploymentSnapshot {
	key := namespace + "/" + deployment
	return r.deploymentHistory[key]
}

func (r *RollbackManager) GetActiveRollback(deployment, namespace string) *RollbackContext {
	key := namespace + "/" + deployment
	return r.activeRollbacks[key]
}

func (r *RollbackManager) WaitForRollbackCompletion(ctx context.Context, deployment, namespace string, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return false, fmt.Errorf("timeout waiting for rollback completion")
			}

			deps, err := r.client.ListDeployments(ctx, namespace)
			if err != nil {
				continue
			}

			for _, dep := range deps {
				if dep.Name == deployment {
					if dep.ReadyReplicas == dep.Replicas && dep.UpdatedReplicas == dep.Replicas {
						return true, nil
					}
				}
			}
		}
	}
}

func (r *RollbackManager) VerifyRollback(ctx context.Context, deployment, namespace string) (bool, error) {
	deps, err := r.client.ListDeployments(ctx, namespace)
	if err != nil {
		return false, err
	}

	for _, dep := range deps {
		if dep.Name == deployment {
			return dep.ReadyReplicas == dep.Replicas && dep.UpdatedReplicas == dep.Replicas, nil
		}
	}

	return false, fmt.Errorf("deployment not found")
}

func (r *RollbackManager) MonitorAfterRollback(ctx context.Context, deployment, namespace string) {
	go func() {
		monitorCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				healthy, _ := r.VerifyRollback(ctx, deployment, namespace)
				if healthy {
					return
				}
			}
		}
	}()
}

func (r *RollbackManager) FormatRollbackInfo(rc *RollbackContext) string {
	return fmt.Sprintf("Rollback of %s/%s: %s -> %s. Reason: %s",
		rc.Namespace, rc.Deployment, rc.NewImage, rc.PrevImage, rc.Reason)
}
