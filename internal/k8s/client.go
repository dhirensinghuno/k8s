package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/k8s-sre/agent/internal/models"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	clientset     *kubernetes.Clientset
	restConfig    *rest.Config
	cloudProvider string
}

type ClientOptions struct {
	KubeconfigPath string
	AWSProfile     string
	AWSRegion      string
	EKSClusterName string
}

func NewClient() (*Client, error) {
	log.Println("[K8sClient] Creating new client with default options...")
	return NewClientWithOptions(ClientOptions{})
}

func NewClientWithOptions(opts ClientOptions) (*Client, error) {
	log.Println("[K8sClient] Creating client with options...")
	var config *rest.Config
	var err error

	if opts.EKSClusterName != "" || opts.AWSProfile != "" || opts.AWSRegion != "" {
		log.Printf("[K8sClient] EKS options detected, cluster=%s region=%s profile=%s\n",
			opts.EKSClusterName, opts.AWSRegion, opts.AWSProfile)
		return NewEKSClient(opts)
	}

	kubeconfig := os.Getenv("KUBECONFIG")
	if opts.KubeconfigPath != "" {
		kubeconfig = opts.KubeconfigPath
		log.Printf("[K8sClient] Using kubeconfig from flag: %s", kubeconfig)
	}

	homeDir, _ := os.UserHomeDir()

	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else if _, err := os.Stat(homeDir + "/.kube/config"); err == nil {
		config, err = clientcmd.BuildConfigFromFlags("", homeDir+"/.kube/config")
	} else {
		return nil, fmt.Errorf("no Kubernetes configuration found. Please set KUBECONFIG or ensure ~/.kube/config exists")
	}

	if err != nil {
		log.Printf("[K8sClient] Failed to build config: %v", err)
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	config.Timeout = 30 * time.Second
	log.Printf("[K8sClient] Config built successfully, API server: %s", config.Host)

	log.Println("[K8sClient] Creating Kubernetes clientset...")
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("[K8sClient] Failed to create clientset: %v", err)
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}
	log.Println("[K8sClient] Clientset created successfully")

	provider := "standard"
	if config.Host != "" {
		provider = detectCloudProvider(config.Host)
	}
	log.Printf("[K8sClient] Detected cloud provider: %s", provider)

	log.Printf("[K8sClient] Client created successfully (provider=%s)", provider)
	return &Client{
		clientset:     clientset,
		restConfig:    config,
		cloudProvider: provider,
	}, nil
}

func NewEKSClient(opts ClientOptions) (*Client, error) {
	log.Println("[K8sClient] Creating EKS client...")
	if opts.EKSClusterName == "" {
		return nil, fmt.Errorf("EKS cluster name is required when using AWS EKS")
	}

	region := opts.AWSRegion
	if region == "" {
		region = os.Getenv("AWS_REGION")
		if region == "" {
			region = "us-east-1"
		}
	}

	log.Printf("[K8sClient] Connecting to EKS cluster: %s in region: %s", opts.EKSClusterName, region)

	var cmd *exec.Cmd
	if opts.AWSProfile != "" {
		log.Printf("[K8sClient] Using AWS profile: %s", opts.AWSProfile)
		cmd = exec.Command("aws", "eks", "update-kubeconfig", "--name", opts.EKSClusterName, "--region", region, "--profile", opts.AWSProfile)
	} else {
		cmd = exec.Command("aws", "eks", "update-kubeconfig", "--name", opts.EKSClusterName, "--region", region)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[K8sClient] Warning: Failed to update kubeconfig via AWS CLI: %v", err)
	}

	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		homeDir, _ := os.UserHomeDir()
		kubeconfigPath = homeDir + "/.kube/config"
	}
	log.Printf("[K8sClient] Using kubeconfig: %s", kubeconfigPath)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Printf("[K8sClient] Failed to build config from kubeconfig: %v", err)
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	config.Timeout = 30 * time.Second

	log.Println("[K8sClient] Creating EKS clientset...")
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("[K8sClient] Failed to create EKS clientset: %v", err)
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}
	log.Println("[K8sClient] EKS clientset created successfully")

	log.Printf("[K8sClient] EKS client created successfully (cluster=%s)", opts.EKSClusterName)
	return &Client{
		clientset:     clientset,
		restConfig:    config,
		cloudProvider: "eks",
	}, nil
}

func detectCloudProvider(host string) string {
	host = strings.ToLower(host)
	if strings.Contains(host, "eks") || strings.Contains(host, "amazonaws") {
		return "eks"
	}
	if strings.Contains(host, "gke") || strings.Contains(host, "google") || strings.Contains(host, "containers.googleapis") {
		return "gke"
	}
	if strings.Contains(host, "aks") || strings.Contains(host, "azure") || strings.Contains(host, "azmk8s") {
		return "aks"
	}
	return "standard"
}

func (c *Client) CloudProvider() string {
	return c.cloudProvider
}

func (c *Client) ListPods(ctx context.Context, namespace string) ([]models.Pod, error) {
	var pods *corev1.PodList
	var err error

	if namespace == "" || namespace == "all" {
		pods, err = c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	} else {
		pods, err = c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	result := make([]models.Pod, 0, len(pods.Items))
	for _, p := range pods.Items {
		result = append(result, c.convertPod(&p))
	}
	return result, nil
}

func (c *Client) GetPod(ctx context.Context, namespace, name string) (*models.Pod, error) {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}
	result := c.convertPod(pod)
	return &result, nil
}

func (c *Client) DescribePod(ctx context.Context, namespace, name string) (string, error) {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod: %w", err)
	}

	var buf bytes.Buffer
	writer := NewPodDescriber(&buf)
	writer.Describe(pod)
	return buf.String(), nil
}

func (c *Client) GetPodLogs(ctx context.Context, namespace, name string, previous bool) (string, error) {
	tailLines := int64(100)
	req := c.clientset.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{
		Previous:  previous,
		TailLines: &tailLines,
	})
	logs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	defer logs.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(logs)
	return buf.String(), nil
}

func (c *Client) convertPod(pod *corev1.Pod) models.Pod {
	status := models.PodStatusUnknown
	ready := false

	switch pod.Status.Phase {
	case corev1.PodRunning:
		status = models.PodStatusRunning
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady {
				ready = cond.Status == corev1.ConditionTrue
				break
			}
		}
	case corev1.PodPending:
		status = models.PodStatusPending
	case corev1.PodSucceeded:
		status = models.PodStatusSucceeded
		ready = true
	case corev1.PodFailed:
		status = models.PodStatusFailed
	}

	restarts := 0
	containerStatuses := make([]models.ContainerStatus, 0)
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += int(cs.RestartCount)
		state := "running"
		reason := ""
		message := ""
		if cs.State.Waiting != nil {
			state = "waiting"
			reason = cs.State.Waiting.Reason
			message = cs.State.Waiting.Message
		} else if cs.State.Terminated != nil {
			state = "terminated"
			reason = cs.State.Terminated.Reason
			message = cs.State.Terminated.Message
		}
		containerStatuses = append(containerStatuses, models.ContainerStatus{
			Name:         cs.Name,
			Ready:        cs.Ready,
			RestartCount: int(cs.RestartCount),
			State:        state,
			Reason:       reason,
			Message:      message,
		})
	}

	var startTime time.Time
	if pod.Status.StartTime != nil {
		startTime = pod.Status.StartTime.Time
	}

	issueTypes := c.detectIssueTypes(pod, containerStatuses)

	return models.Pod{
		Name:              pod.Name,
		Namespace:         pod.Namespace,
		Status:            status,
		Ready:             ready,
		Restarts:          restarts,
		Image:             c.getPrimaryImage(pod),
		Node:              pod.Spec.NodeName,
		StartTime:         startTime,
		IssueTypes:        issueTypes,
		Reason:            c.getPodReason(pod, containerStatuses),
		Message:           c.getPodMessage(pod, containerStatuses),
		ContainerStatuses: containerStatuses,
	}
}

func (c *Client) getPrimaryImage(pod *corev1.Pod) string {
	if len(pod.Spec.Containers) > 0 {
		return pod.Spec.Containers[0].Image
	}
	return ""
}

func (c *Client) detectIssueTypes(pod *corev1.Pod, containerStatuses []models.ContainerStatus) []models.IssueType {
	var issues []models.IssueType

	for _, cs := range containerStatuses {
		if cs.Reason == "CrashLoopBackOff" {
			issues = append(issues, models.IssueCrashLoopBackOff)
		}
		if cs.Reason == "OOMKilled" {
			issues = append(issues, models.IssueOOMKilled)
		}
		if cs.Reason == "ImagePullBackOff" {
			issues = append(issues, models.IssueImagePullBackOff)
		}
		if cs.Reason == "ErrImagePull" {
			issues = append(issues, models.IssueErrImagePull)
		}
		if cs.RestartCount > 5 {
			issues = append(issues, models.IssueHighRestart)
		}
	}

	if pod.Status.Phase == corev1.PodPending {
		issues = append(issues, models.IssuePending)
	}

	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionFalse {
			issues = append(issues, models.IssueReadinessFailure)
		}
	}

	return issues
}

func (c *Client) getPodReason(pod *corev1.Pod, containerStatuses []models.ContainerStatus) string {
	for _, cs := range containerStatuses {
		if cs.Reason != "" && cs.Reason != "running" {
			return cs.Reason
		}
	}
	if pod.Status.Reason != "" {
		return pod.Status.Reason
	}
	return "Unknown"
}

func (c *Client) getPodMessage(pod *corev1.Pod, containerStatuses []models.ContainerStatus) string {
	for _, cs := range containerStatuses {
		if cs.Message != "" {
			return cs.Message
		}
	}
	return pod.Status.Message
}

func (c *Client) ListNodes(ctx context.Context) ([]models.Node, error) {
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	result := make([]models.Node, 0, len(nodes.Items))
	for _, n := range nodes.Items {
		result = append(result, c.convertNode(&n))
	}
	return result, nil
}

func (c *Client) convertNode(n *corev1.Node) models.Node {
	ready := false
	var conditions []string

	for _, cond := range n.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status == corev1.ConditionTrue {
				ready = true
			}
		}
		if cond.Status != corev1.ConditionTrue {
			conditions = append(conditions, fmt.Sprintf("%s:%s", cond.Type, cond.Status))
		}
	}

	status := "Ready"
	if !ready {
		for _, cond := range n.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status != corev1.ConditionTrue {
				status = cond.Reason
				break
			}
		}
		if status == "Ready" {
			status = "NotReady"
		}
	}

	return models.Node{
		Name:       n.Name,
		Status:     status,
		Ready:      ready,
		Conditions: conditions,
	}
}

func (c *Client) ListEvents(ctx context.Context, namespace string, sortByTime bool) ([]models.Event, error) {
	var events *corev1.EventList
	var err error

	if namespace == "" || namespace == "all" {
		events, err = c.clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	} else {
		events, err = c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	result := make([]models.Event, 0, len(events.Items))
	for _, e := range events.Items {
		if e.Type != "Normal" {
			result = append(result, models.Event{
				Type:      e.Type,
				Reason:    e.Reason,
				Message:   e.Message,
				Involved:  e.InvolvedObject.Name,
				Namespace: e.InvolvedObject.Namespace,
				FirstSeen: e.FirstTimestamp.Time,
				LastSeen:  e.LastTimestamp.Time,
				Count:     int(e.Count),
			})
		}
	}
	return result, nil
}

func (c *Client) ListDeployments(ctx context.Context, namespace string) ([]models.Deployment, error) {
	var deps *appsv1.DeploymentList
	var err error

	if namespace == "" || namespace == "all" {
		deps, err = c.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	} else {
		deps, err = c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	result := make([]models.Deployment, 0, len(deps.Items))
	for _, d := range deps.Items {
		image := ""
		if len(d.Spec.Template.Spec.Containers) > 0 {
			image = d.Spec.Template.Spec.Containers[0].Image
		}
		result = append(result, models.Deployment{
			Name:              d.Name,
			Namespace:         d.Namespace,
			Replicas:          *d.Spec.Replicas,
			ReadyReplicas:     d.Status.ReadyReplicas,
			AvailableReplicas: d.Status.AvailableReplicas,
			UpdatedReplicas:   d.Status.UpdatedReplicas,
			Image:             image,
			LastUpdate:        d.ObjectMeta.CreationTimestamp.Time,
		})
	}
	return result, nil
}

func (c *Client) RollbackDeployment(ctx context.Context, namespace, name string) error {
	patchData := fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}}}`,
		time.Now().Format(time.RFC3339))
	_, err := c.clientset.AppsV1().Deployments(namespace).Patch(ctx, name, "application/strategic-merge-patch+json",
		[]byte(patchData), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to rollback: %w", err)
	}
	return nil
}

func (c *Client) RestartDeployment(ctx context.Context, namespace, name string) error {
	return c.RollbackDeployment(ctx, namespace, name)
}

func (c *Client) ScaleDeployment(ctx context.Context, namespace, name string, replicas int32) error {
	dep, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}
	dep.Spec.Replicas = &replicas
	_, err = c.clientset.AppsV1().Deployments(namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (c *Client) DeletePod(ctx context.Context, namespace, name string) error {
	err := c.clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete pod: %w", err)
	}
	return nil
}

func (c *Client) CordonNode(ctx context.Context, name string) error {
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	node.Spec.Unschedulable = true
	_, err = c.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to cordon node: %w", err)
	}
	return nil
}

func (c *Client) UncordonNode(ctx context.Context, name string) error {
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	node.Spec.Unschedulable = false
	_, err = c.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to uncordon node: %w", err)
	}
	return nil
}

func (c *Client) DrainNode(ctx context.Context, name string, force bool) error {
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", name),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		if pod.DeletionTimestamp != nil {
			continue
		}
		err := c.clientset.CoreV1().Pods(pod.Namespace).Evict(ctx, &policyv1beta1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			},
		})
		if err != nil && !errors.IsNotFound(err) {
			if !force {
				return fmt.Errorf("failed to evict pod %s: %w", pod.Name, err)
			}
		}
	}
	return nil
}

func (c *Client) ListPVCs(ctx context.Context, namespace string) ([]corev1.PersistentVolumeClaim, error) {
	var pvcs *corev1.PersistentVolumeClaimList
	var err error

	if namespace == "" || namespace == "all" {
		pvcs, err = c.clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	} else {
		pvcs, err = c.clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list PVCs: %w", err)
	}
	return pvcs.Items, nil
}

func (c *Client) GetServiceEndpoints(ctx context.Context, namespace, name string) (*corev1.Endpoints, error) {
	ep, err := c.clientset.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoints: %w", err)
	}
	return ep, nil
}

func (c *Client) UpdateDeploymentImage(ctx context.Context, namespace, name, image string) error {
	dep, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	for i := range dep.Spec.Template.Spec.Containers {
		dep.Spec.Template.Spec.Containers[i].Image = image
	}

	_, err = c.clientset.AppsV1().Deployments(namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (c *Client) IncreaseMemoryLimit(ctx context.Context, namespace, podName string, percent float64) error {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Resources.Limits != nil {
			memLimit := pod.Spec.Containers[i].Resources.Limits[corev1.ResourceMemory]
			if !memLimit.IsZero() {
				newLimit := float64(memLimit.Value()) * (1 + percent/100)
				pod.Spec.Containers[i].Resources.Limits[corev1.ResourceMemory] = *resource.NewQuantity(int64(newLimit), resource.DecimalSI)
			}
		}
	}

	_, err = c.clientset.CoreV1().Pods(namespace).Update(ctx, pod, metav1.UpdateOptions{})
	return err
}

func (c *Client) Client() *kubernetes.Clientset {
	return c.clientset
}

type PodDescriber struct {
	out io.Writer
}

func NewPodDescriber(out io.Writer) *PodDescriber {
	return &PodDescriber{out: out}
}

func (d *PodDescriber) Describe(pod *corev1.Pod) {
	fmt.Fprintf(d.out, "Name:         %s\n", pod.Name)
	fmt.Fprintf(d.out, "Namespace:    %s\n", pod.Namespace)
	fmt.Fprintf(d.out, "Node:         %s\n", pod.Spec.NodeName)
	fmt.Fprintf(d.out, "Start Time:   %s\n", pod.Status.StartTime)
	fmt.Fprintf(d.out, "Status:       %s\n", pod.Status.Phase)
	fmt.Fprintf(d.out, "IP:           %s\n", pod.Status.PodIP)

	fmt.Fprintf(d.out, "\nContainers:\n")
	for _, c := range pod.Status.ContainerStatuses {
		fmt.Fprintf(d.out, "  %s:\n", c.Name)
		fmt.Fprintf(d.out, "    Container ID:  %s\n", c.ContainerID)
		fmt.Fprintf(d.out, "    Image:         %s\n", c.Image)
		fmt.Fprintf(d.out, "    Image ID:      %s\n", c.ImageID)
		fmt.Fprintf(d.out, "    State:         %s\n", c.State)
		fmt.Fprintf(d.out, "    Ready:         %v\n", c.Ready)
		fmt.Fprintf(d.out, "    Restart Count: %d\n", c.RestartCount)
		if c.LastTerminationState.Terminated != nil {
			fmt.Fprintf(d.out, "    Last State:    Terminated\n")
			fmt.Fprintf(d.out, "      Reason:       %s\n", c.LastTerminationState.Terminated.Reason)
			fmt.Fprintf(d.out, "      Message:      %s\n", c.LastTerminationState.Terminated.Message)
		}
	}
}
