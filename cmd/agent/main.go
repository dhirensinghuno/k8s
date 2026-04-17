package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/k8s-sre/agent/internal/agent"
	"github.com/k8s-sre/agent/internal/api"
	"github.com/k8s-sre/agent/internal/k8s"
	"github.com/k8s-sre/agent/internal/store"
)

var (
	port       int
	dbHost     string
	dbPort     int
	dbUser     string
	dbPassword string
	dbName     string
	enableDB   bool
)

func main() {
	flag.IntVar(&port, "port", 8080, "HTTP server port")
	flag.StringVar(&dbHost, "db-host", "localhost", "PostgreSQL host")
	flag.IntVar(&dbPort, "db-port", 5432, "PostgreSQL port")
	flag.StringVar(&dbUser, "db-user", "postgres", "PostgreSQL user")
	flag.StringVar(&dbPassword, "db-password", "", "PostgreSQL password")
	flag.StringVar(&dbName, "db-name", "k8s_sre", "PostgreSQL database name")
	flag.BoolVar(&enableDB, "enable-db", false, "Enable PostgreSQL storage")
	flag.Parse()

	log.Println("Starting Kubernetes SRE Agent...")

	client, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}
	log.Printf("Connected to Kubernetes cluster (provider: %s)", client.CloudProvider())

	var db *store.Store
	if enableDB {
		db, err = store.NewStore(dbHost, dbPort, dbUser, dbPassword, dbName)
		if err != nil {
			log.Printf("Warning: Failed to connect to database: %v", err)
			log.Println("Continuing without database storage...")
			db = nil
		} else {
			log.Println("Connected to PostgreSQL database")
		}
	}

	cfg := agent.DefaultConfig()
	sreAgent := agent.NewAgent(client, db, cfg)

	if err := sreAgent.Start(); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}
	log.Println("SRE Agent started successfully")

	server := api.NewServer(sreAgent, client, port)
	go func() {
		log.Printf("Starting HTTP server on port %d", port)
		if err := server.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Printf("API server started on http://localhost:%d", port)
	log.Printf("WebSocket endpoint: ws://localhost:%d/ws", port)
	log.Println("\nAvailable endpoints:")
	log.Println("  GET  /health              - Cluster health status")
	log.Println("  GET  /api/pods            - List all pods")
	log.Println("  GET  /api/nodes           - List all nodes")
	log.Println("  GET  /api/deployments     - List all deployments")
	log.Println("  GET  /api/events          - List warning events")
	log.Println("  GET  /api/issues          - List detected issues")
	log.Println("  GET  /api/actions         - List remediation actions")
	log.Println("  POST /api/diagnose        - Diagnose a pod/issue")
	log.Println("  POST /api/remediate       - Trigger remediation")
	log.Println("  POST /api/deployments/{ns}/{name}/rollback - Rollback deployment")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	sreAgent.Stop()
	if db != nil {
		db.Close()
	}
	log.Println("Shutdown complete")
}
