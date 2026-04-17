package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	serverURL string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "k8s-sre",
		Short: "Kubernetes SRE CLI Tool",
		Long:  "CLI tool for Kubernetes Site Reliability Engineering operations",
	}

	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8080", "SRE Agent server URL")

	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(podsCmd)
	rootCmd.AddCommand(nodesCmd)
	rootCmd.AddCommand(deploymentsCmd)
	rootCmd.AddCommand(eventsCmd)
	rootCmd.AddCommand(issuesCmd)
	rootCmd.AddCommand(actionsCmd)
	rootCmd.AddCommand(diagnoseCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(auditCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check cluster health",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := http.Get(serverURL + "/api/health")
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()
		return printJSON(resp.Body)
	},
}

var podsCmd = &cobra.Command{
	Use:   "pods [namespace]",
	Short: "List pods",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := serverURL + "/api/pods"
		if len(args) > 0 {
			url += "?namespace=" + args[0]
		}
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to get pods: %w", err)
		}
		defer resp.Body.Close()
		return printJSON(resp.Body)
	},
}

var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "List nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := http.Get(serverURL + "/api/nodes")
		if err != nil {
			return fmt.Errorf("failed to get nodes: %w", err)
		}
		defer resp.Body.Close()
		return printJSON(resp.Body)
	},
}

var deploymentsCmd = &cobra.Command{
	Use:   "deployments [namespace]",
	Short: "List deployments",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := serverURL + "/api/deployments"
		if len(args) > 0 {
			url += "?namespace=" + args[0]
		}
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to get deployments: %w", err)
		}
		defer resp.Body.Close()
		return printJSON(resp.Body)
	},
}

var eventsCmd = &cobra.Command{
	Use:   "events [namespace]",
	Short: "List warning events",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := serverURL + "/api/events"
		if len(args) > 0 {
			url += "?namespace=" + args[0]
		}
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to get events: %w", err)
		}
		defer resp.Body.Close()
		return printJSON(resp.Body)
	},
}

var issuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "List detected issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := http.Get(serverURL + "/api/issues")
		if err != nil {
			return fmt.Errorf("failed to get issues: %w", err)
		}
		defer resp.Body.Close()
		return printJSON(resp.Body)
	},
}

var actionsCmd = &cobra.Command{
	Use:   "actions",
	Short: "List remediation actions",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := http.Get(serverURL + "/api/actions")
		if err != nil {
			return fmt.Errorf("failed to get actions: %w", err)
		}
		defer resp.Body.Close()
		return printJSON(resp.Body)
	},
}

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose <namespace> <pod>",
	Short: "Diagnose a pod",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		namespace := args[0]
		pod := args[1]

		body := map[string]string{
			"namespace": namespace,
			"pod":       pod,
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(serverURL+"/api/diagnose", "application/json", strings.NewReader(string(jsonBody)))
		if err != nil {
			return fmt.Errorf("failed to diagnose: %w", err)
		}
		defer resp.Body.Close()
		return printJSON(resp.Body)
	},
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback <namespace> <deployment>",
	Short: "Rollback a deployment to previous version",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		namespace := args[0]
		deployment := args[1]

		reason, _ := cmd.Flags().GetString("reason")
		if reason == "" {
			reason = "Manual rollback via CLI"
		}

		body := map[string]string{"reason": reason}
		jsonBody, _ := json.Marshal(body)

		url := fmt.Sprintf("%s/api/deployments/%s/%s/rollback", serverURL, namespace, deployment)
		req, _ := http.NewRequest("POST", url, strings.NewReader(string(jsonBody)))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to rollback: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("rollback failed: %s", string(body))
		}

		fmt.Println("Rollback initiated successfully")
		return printJSON(resp.Body)
	},
}

func init() {
	rollbackCmd.Flags().StringP("reason", "r", "", "Reason for rollback")
}

var restartCmd = &cobra.Command{
	Use:   "restart <namespace> <deployment>",
	Short: "Restart a deployment",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		namespace := args[0]
		deployment := args[1]

		url := fmt.Sprintf("%s/api/deployments/%s/%s/restart", serverURL, namespace, deployment)
		resp, err := http.Post(url, "application/json", nil)
		if err != nil {
			return fmt.Errorf("failed to restart: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("restart failed: %s", string(body))
		}

		fmt.Println("Restart initiated successfully")
		return printJSON(resp.Body)
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs <namespace> <pod>",
	Short: "Get pod logs",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		namespace := args[0]
		pod := args[1]
		previous, _ := cmd.Flags().GetBool("previous")

		url := fmt.Sprintf("%s/api/pods/%s/%s/logs", serverURL, namespace, pod)
		if previous {
			url += "?previous=true"
		}

		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		fmt.Print(string(body))
		return nil
	},
}

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "View audit logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := http.Get(serverURL + "/api/audit")
		if err != nil {
			return fmt.Errorf("failed to get audit logs: %w", err)
		}
		defer resp.Body.Close()
		return printJSON(resp.Body)
	},
}

func printJSON(r io.Reader) error {
	var data interface{}
	if err := json.NewDecoder(r).Decode(&data); err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
