package models

import (
	"time"
)

type PodStatus string

const (
	PodStatusRunning   PodStatus = "Running"
	PodStatusPending   PodStatus = "Pending"
	PodStatusSucceeded PodStatus = "Succeeded"
	PodStatusFailed    PodStatus = "Failed"
	PodStatusUnknown   PodStatus = "Unknown"
)

type IssueSeverity string

const (
	SeverityCritical IssueSeverity = "critical"
	SeverityWarning  IssueSeverity = "warning"
	SeverityInfo     IssueSeverity = "info"
)

type IssueType string

const (
	IssueCrashLoopBackOff IssueType = "CrashLoopBackOff"
	IssueOOMKilled        IssueType = "OOMKilled"
	IssueImagePullBackOff IssueType = "ImagePullBackOff"
	IssueErrImagePull     IssueType = "ErrImagePull"
	IssuePending          IssueType = "Pending"
	IssueReadinessFailure IssueType = "ReadinessFailure"
	IssueLivenessFailure  IssueType = "LivenessFailure"
	IssueNodeNotReady     IssueType = "NodeNotReady"
	IssueNodePressure     IssueType = "NodePressure"
	IssuePVCPending       IssueType = "PVCPending"
	IssueHighRestart      IssueType = "HighRestart"
	IssueUnknown          IssueType = "Unknown"
)

type RootCause string

const (
	RootCauseResourceExhaustion RootCause = "ResourceExhaustion"
	RootCauseBadImage           RootCause = "BadImage"
	RootCauseMisconfiguration   RootCause = "Misconfiguration"
	RootCauseNodeIssue          RootCause = "NodeIssue"
	RootCauseNetworkIssue       RootCause = "NetworkIssue"
	RootCauseStorageIssue       RootCause = "StorageIssue"
	RootCauseTransient          RootCause = "Transient"
	RootCauseUnknown            RootCause = "Unknown"
)

type ActionType string

const (
	ActionRestartPod        ActionType = "RestartPod"
	ActionIncreaseResources ActionType = "IncreaseResources"
	ActionRollback          ActionType = "Rollback"
	ActionCordonNode        ActionType = "CordonNode"
	ActionDrainNode         ActionType = "DrainNode"
	ActionScaleDeployment   ActionType = "ScaleDeployment"
	ActionRebindPVC         ActionType = "RebindPVC"
	ActionNoAction          ActionType = "NoAction"
)

type Node struct {
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	Ready          bool      `json:"ready"`
	CPU            float64   `json:"cpu"`
	Memory         float64   `json:"memory"`
	Conditions     []string  `json:"conditions"`
	LastTransition time.Time `json:"last_transition"`
}

type Pod struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Status            PodStatus         `json:"status"`
	Ready             bool              `json:"ready"`
	Restarts          int               `json:"restarts"`
	Image             string            `json:"image"`
	Node              string            `json:"node"`
	StartTime         time.Time         `json:"start_time"`
	IssueTypes        []IssueType       `json:"issue_types"`
	Reason            string            `json:"reason"`
	Message           string            `json:"message"`
	ContainerStatuses []ContainerStatus `json:"container_statuses"`
}

type ContainerStatus struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int    `json:"restart_count"`
	State        string `json:"state"`
	Reason       string `json:"reason"`
	Message      string `json:"message"`
}

type Deployment struct {
	Name              string    `json:"name"`
	Namespace         string    `json:"namespace"`
	Replicas          int32     `json:"replicas"`
	ReadyReplicas     int32     `json:"ready_replicas"`
	AvailableReplicas int32     `json:"available_replicas"`
	UpdatedReplicas   int32     `json:"updated_replicas"`
	Image             string    `json:"image"`
	LastUpdate        time.Time `json:"last_update"`
	RolloutHistory    []int64   `json:"rollout_history"`
}

type Event struct {
	Type      string    `json:"type"`
	Reason    string    `json:"reason"`
	Message   string    `json:"message"`
	Involved  string    `json:"involved"`
	Namespace string    `json:"namespace"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	Count     int       `json:"count"`
}

type Issue struct {
	ID         string        `json:"id"`
	Timestamp  time.Time     `json:"timestamp"`
	Severity   IssueSeverity `json:"severity"`
	Type       IssueType     `json:"type"`
	Namespace  string        `json:"namespace"`
	Pod        string        `json:"pod"`
	Node       string        `json:"node"`
	Deployment string        `json:"deployment"`
	Reason     string        `json:"reason"`
	Message    string        `json:"message"`
	RootCause  RootCause     `json:"root_cause"`
	Evidence   []string      `json:"evidence"`
	Resolved   bool          `json:"resolved"`
	ResolvedAt *time.Time    `json:"resolved_at"`
}

type Action struct {
	ID           string     `json:"id"`
	Timestamp    time.Time  `json:"timestamp"`
	IssueID      string     `json:"issue_id"`
	Type         ActionType `json:"type"`
	Target       string     `json:"target"`
	Namespace    string     `json:"namespace"`
	Reason       string     `json:"reason"`
	Evidence     []string   `json:"evidence"`
	Success      bool       `json:"success"`
	Result       string     `json:"result"`
	RollbackFrom bool       `json:"rollback_from"`
	PrevVersion  string     `json:"prev_version,omitempty"`
	NewVersion   string     `json:"new_version,omitempty"`
}

type AuditLog struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Component string    `json:"component"`
	Message   string    `json:"message"`
	IssueID   string    `json:"issue_id,omitempty"`
	ActionID  string    `json:"action_id,omitempty"`
	Metadata  string    `json:"metadata,omitempty"`
}

type ClusterHealth struct {
	Timestamp      time.Time `json:"timestamp"`
	OverallStatus  string    `json:"overall_status"`
	NodesReady     int       `json:"nodes_ready"`
	NodesTotal     int       `json:"nodes_total"`
	PodsRunning    int       `json:"pods_running"`
	PodsTotal      int       `json:"pods_total"`
	PodsUnhealthy  int       `json:"pods_unhealthy"`
	CriticalIssues int       `json:"critical_issues"`
	WarningIssues  int       `json:"warning_issues"`
	RecentActions  int       `json:"recent_actions"`
	CPUUsage       float64   `json:"cpu_usage"`
	MemoryUsage    float64   `json:"memory_usage"`
	WarningEvents  int       `json:"warning_events"`
}

type RollbackPolicy struct {
	MinTimeSinceDeploy   time.Duration `json:"min_time_since_deploy"`
	MinErrorRateIncrease float64       `json:"min_error_rate_increase"`
	MinUnhealthyDuration time.Duration `json:"min_unhealthy_duration"`
	MinStableDuration    time.Duration `json:"min_stable_duration"`
}

type RemediationConfig struct {
	EnableAutoRemediation bool           `json:"enable_auto_remediation"`
	EnableAutoRollback    bool           `json:"enable_auto_rollback"`
	MemoryIncreasePercent float64        `json:"memory_increase_percent"`
	CPUScaleThreshold     float64        `json:"cpu_scale_threshold"`
	ScaleUpCooldown       time.Duration  `json:"scale_up_cooldown"`
	ScaleDownCooldown     time.Duration  `json:"scale_down_cooldown"`
	RollbackPolicy        RollbackPolicy `json:"rollback_policy"`
}

type DeploymentHistory struct {
	Revision   int64  `json:"revision"`
	Image      string `json:"image"`
	ChangedBy  string `json:"changed_by"`
	ChangeDate string `json:"change_date"`
	IsActive   bool   `json:"is_active"`
}
