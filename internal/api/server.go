package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/k8s-sre/agent/internal/agent"
	"github.com/k8s-sre/agent/internal/auth"
	"github.com/k8s-sre/agent/internal/k8s"
	"github.com/k8s-sre/agent/internal/models"
)

func (s *Server) requireRole(w http.ResponseWriter, r *http.Request, allowed ...auth.Role) bool {
	roleStr := r.Header.Get("X-User-Role")
	userRole := auth.Role(roleStr)

	for _, allowedRole := range allowed {
		if userRole == allowedRole {
			return true
		}
	}
	http.Error(w, "Insufficient permissions", http.StatusForbidden)
	return false
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Server struct {
	agent          *agent.Agent
	client         *k8s.Client
	router         *mux.Router
	server         *http.Server
	wsConns        map[*websocket.Conn]bool
	wsMu           sync.RWMutex
	authMiddleware *auth.AuthMiddleware
}

func NewServer(agent *agent.Agent, client *k8s.Client, port int) *Server {
	log.Printf("[API] Creating API server on port %d...\n", port)
	s := &Server{
		agent:   agent,
		client:  client,
		wsConns: make(map[*websocket.Conn]bool),
	}

	log.Println("[API] Creating router...")
	s.router = mux.NewRouter()

	log.Println("[API] Setting up routes...")
	s.setupRoutes()

	log.Printf("[API] Configuring HTTP server on :%d...", port)
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Println("[API] Server created successfully")
	return s
}

func (s *Server) SetAuthMiddleware(m *auth.AuthMiddleware) {
	s.authMiddleware = m
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if s.authMiddleware != nil {
		s.authMiddleware.HandleLogin(w, r)
	} else {
		http.Error(w, "Authentication not configured", http.StatusInternalServerError)
	}
}

func (s *Server) setupRoutes() {
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/api/health", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/api/auth/login", s.handleAuthLogin).Methods("POST", "OPTIONS")

	if s.authMiddleware != nil && s.authMiddleware.IsEnabled() {
		s.router.Use(s.authMiddleware.Handler())

		s.router.HandleFunc("/api/pods", s.handleListPods).Methods("GET")
		s.router.HandleFunc("/api/pods/{namespace}/{name}", s.handleGetPod).Methods("GET")
		s.router.HandleFunc("/api/pods/{namespace}/{name}/logs", s.handleGetPodLogs).Methods("GET")
		s.router.HandleFunc("/api/pods/{namespace}/{name}/describe", s.handleDescribePod).Methods("GET")
		s.router.HandleFunc("/api/nodes", s.handleListNodes).Methods("GET")
		s.router.HandleFunc("/api/deployments", s.handleListDeployments).Methods("GET")

		s.router.HandleFunc("/api/deployments/{namespace}/{name}/rollback", s.handleRollbackDeployment).Methods("POST")
		s.router.HandleFunc("/api/deployments/{namespace}/{name}/restart", s.handleRestartDeployment).Methods("POST")

		s.router.HandleFunc("/api/events", s.handleListEvents).Methods("GET")
		s.router.HandleFunc("/api/issues", s.handleListIssues).Methods("GET")
		s.router.HandleFunc("/api/issues/{id}", s.handleGetIssue).Methods("GET")
		s.router.HandleFunc("/api/issues/{id}/resolve", s.handleResolveIssue).Methods("POST")
		s.router.HandleFunc("/api/actions", s.handleListActions).Methods("GET")
		s.router.HandleFunc("/api/actions/{id}", s.handleGetAction).Methods("GET")
		s.router.HandleFunc("/api/audit", s.handleAuditLogs).Methods("GET")
		s.router.HandleFunc("/api/cluster-history", s.handleClusterHistory).Methods("GET")
		s.router.HandleFunc("/api/config", s.handleGetConfig).Methods("GET")

		s.router.HandleFunc("/api/diagnose", s.handleDiagnose).Methods("POST")
		s.router.HandleFunc("/api/remediate", s.handleRemediate).Methods("POST")
		s.router.HandleFunc("/api/debug", s.handleDebug).Methods("GET")
		s.router.HandleFunc("/api/config", s.handleUpdateConfig).Methods("PUT")
	} else {
		s.router.HandleFunc("/api/pods", s.handleListPods).Methods("GET")
		s.router.HandleFunc("/api/pods/{namespace}/{name}", s.handleGetPod).Methods("GET")
		s.router.HandleFunc("/api/pods/{namespace}/{name}/logs", s.handleGetPodLogs).Methods("GET")
		s.router.HandleFunc("/api/pods/{namespace}/{name}/describe", s.handleDescribePod).Methods("GET")
		s.router.HandleFunc("/api/nodes", s.handleListNodes).Methods("GET")
		s.router.HandleFunc("/api/deployments", s.handleListDeployments).Methods("GET")
		s.router.HandleFunc("/api/deployments/{namespace}/{name}/rollback", s.handleRollbackDeployment).Methods("POST")
		s.router.HandleFunc("/api/deployments/{namespace}/{name}/restart", s.handleRestartDeployment).Methods("POST")
		s.router.HandleFunc("/api/events", s.handleListEvents).Methods("GET")
		s.router.HandleFunc("/api/issues", s.handleListIssues).Methods("GET")
		s.router.HandleFunc("/api/issues/{id}", s.handleGetIssue).Methods("GET")
		s.router.HandleFunc("/api/issues/{id}/resolve", s.handleResolveIssue).Methods("POST")
		s.router.HandleFunc("/api/actions", s.handleListActions).Methods("GET")
		s.router.HandleFunc("/api/actions/{id}", s.handleGetAction).Methods("GET")
		s.router.HandleFunc("/api/audit", s.handleAuditLogs).Methods("GET")
		s.router.HandleFunc("/api/diagnose", s.handleDiagnose).Methods("POST")
		s.router.HandleFunc("/api/remediate", s.handleRemediate).Methods("POST")
		s.router.HandleFunc("/api/cluster-history", s.handleClusterHistory).Methods("GET")
		s.router.HandleFunc("/api/debug", s.handleDebug).Methods("GET")
		s.router.HandleFunc("/api/config", s.handleGetConfig).Methods("GET")
		s.router.HandleFunc("/api/config", s.handleUpdateConfig).Methods("PUT")
	}

	s.router.HandleFunc("/ws", s.handleWebSocket)
}

func (s *Server) Start() error {
	log.Println("[API] Starting server...")

	log.Println("[API] Adding CORS middleware...")
	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	log.Println("[API] Setting up final routes...")
	s.setupRoutes()

	go s.broadcastLoop()
	log.Println("[API] Broadcast loop started")
	log.Printf("[API] Server listening on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) Stop() error {
	log.Println("[API] Stopping server...")
	s.wsMu.Lock()
	for conn := range s.wsConns {
		conn.Close()
	}
	s.wsMu.Unlock()

	log.Println("[API] Server stopped")
	return s.server.Shutdown(context.Background())
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	log.Println("[API] Handling /api/health request...")
	health := s.agent.GetHealth()
	if health == nil {
		log.Println("[API] No health data available")
		health = &models.ClusterHealth{
			Timestamp:      time.Now(),
			OverallStatus:  "unknown",
			NodesReady:     0,
			NodesTotal:     0,
			PodsRunning:    0,
			PodsTotal:      0,
			PodsUnhealthy:  0,
			CriticalIssues: 0,
			WarningIssues:  0,
			RecentActions:  0,
		}
	} else {
		log.Printf("[API] Health status: %s, Nodes: %d/%d, Pods: %d/%d",
			health.OverallStatus, health.NodesReady, health.NodesTotal, health.PodsRunning, health.PodsTotal)
	}

	s.writeJSON(w, health)
}

func (s *Server) handleDebug(w http.ResponseWriter, r *http.Request) {
	nodes := s.agent.GetNodes()
	pods := s.agent.GetPods("")
	health := s.agent.GetHealth()
	s.writeJSON(w, map[string]interface{}{
		"nodes":  len(nodes),
		"pods":   len(pods),
		"health": health,
	})
}

func (s *Server) handleListPods(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	pods := s.agent.GetPods(namespace)

	s.writeJSON(w, pods)
}

func (s *Server) handleGetPod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	name := vars["name"]

	ctx := context.Background()
	pod, err := s.client.GetPod(ctx, namespace, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	s.writeJSON(w, pod)
}

func (s *Server) handleGetPodLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	name := vars["name"]
	previous := r.URL.Query().Get("previous") == "true"

	ctx := context.Background()
	logs, err := s.client.GetPodLogs(ctx, namespace, name, previous)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(logs))
}

func (s *Server) handleDescribePod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	name := vars["name"]

	ctx := context.Background()
	desc, err := s.client.DescribePod(ctx, namespace, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(desc))
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes := s.agent.GetNodes()
	s.writeJSON(w, nodes)
}

func (s *Server) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	deployments := s.agent.GetDeployments(namespace)

	if namespace != "" && namespace != "all" {
		var filtered []models.Deployment
		for _, d := range deployments {
			if d.Namespace == namespace {
				filtered = append(filtered, d)
			}
		}
		deployments = filtered
	}

	s.writeJSON(w, deployments)
}

func (s *Server) handleRollbackDeployment(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, auth.RoleAdmin, auth.RoleEditor) {
		return
	}
	log.Println("[API] Handling rollback request...")
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	name := vars["name"]

	log.Printf("[API] Rollback for deployment: %s/%s", namespace, name)

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Reason = "Manual rollback requested"
	}

	log.Printf("[API] Reason: %s", req.Reason)

	ctx := context.Background()
	err := s.client.RollbackDeployment(ctx, namespace, name)
	if err != nil {
		log.Printf("[API] Rollback error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("[API] Rollback successful")
	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Rollback initiated for deployment %s/%s", namespace, name),
		"reason":  req.Reason,
	})
}

func (s *Server) handleRestartDeployment(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, auth.RoleAdmin, auth.RoleEditor) {
		return
	}
	log.Println("[API] Handling restart request...")
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	name := vars["name"]

	log.Printf("[API] Restart for deployment: %s/%s", namespace, name)

	ctx := context.Background()
	err := s.client.RestartDeployment(ctx, namespace, name)
	if err != nil {
		log.Printf("[API] Restart error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("[API] Restart successful")
	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Restart initiated for deployment %s/%s", namespace, name),
	})
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	events := s.agent.GetEvents(namespace)

	s.writeJSON(w, events)
}

func (s *Server) handleListIssues(w http.ResponseWriter, r *http.Request) {
	limit := 100
	issues := s.agent.GetIssues(limit)
	s.writeJSON(w, issues)
}

func (s *Server) handleGetIssue(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	issue, err := s.agent.GetIssue(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	s.writeJSON(w, issue)
}

func (s *Server) handleResolveIssue(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, auth.RoleAdmin, auth.RoleEditor) {
		return
	}
	vars := mux.Vars(r)
	id := vars["id"]

	err := s.agent.ResolveIssue(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, map[string]string{"status": "success"})
}

func (s *Server) handleListActions(w http.ResponseWriter, r *http.Request) {
	limit := 100
	actions := s.agent.GetActions(limit)
	s.writeJSON(w, actions)
}

func (s *Server) handleGetAction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	action, err := s.agent.GetAction(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	s.writeJSON(w, action)
}

func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	logs := s.agent.GetAuditLogs(limit)
	s.writeJSON(w, logs)
}

func (s *Server) handleDiagnose(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Namespace string `json:"namespace"`
		Pod       string `json:"pod"`
		IssueType string `json:"issue_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	issue := models.Issue{
		Type:      models.IssueType(req.IssueType),
		Namespace: req.Namespace,
		Pod:       req.Pod,
		Timestamp: time.Now(),
	}

	ctx := context.Background()
	result, err := s.agent.Diagnose(ctx, issue)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, result)
}

func (s *Server) handleRemediate(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, auth.RoleAdmin, auth.RoleEditor) {
		return
	}
	var req struct {
		IssueID string `json:"issue_id"`
		Force   bool   `json:"force"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	action, err := s.agent.Remediate(ctx, req.IssueID, req.Force)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, action)
}

func (s *Server) handleClusterHistory(w http.ResponseWriter, r *http.Request) {
	hours := 24
	history := s.agent.GetClusterHistory(hours)
	s.writeJSON(w, history)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	config := s.agent.GetConfig()
	s.writeJSON(w, config)
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var config models.RemediationConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.agent.UpdateConfig(config)
	s.writeJSON(w, map[string]string{"status": "success"})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	s.wsMu.Lock()
	s.wsConns[conn] = true
	s.wsMu.Unlock()

	defer func() {
		s.wsMu.Lock()
		delete(s.wsConns, conn)
		s.wsMu.Unlock()
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *Server) broadcastLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.broadcast()
	}
}

func (s *Server) broadcast() {
	s.wsMu.RLock()
	defer s.wsMu.RUnlock()

	if len(s.wsConns) == 0 {
		return
	}

	health := s.agent.GetHealth()
	if health == nil {
		return
	}

	data, err := json.Marshal(map[string]interface{}{
		"type":    "health",
		"data":    health,
		"version": time.Now().Unix(),
	})
	if err != nil {
		return
	}

	for conn := range s.wsConns {
		err := conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			conn.Close()
		}
	}
}

func (s *Server) BroadcastEvent(eventType string, data interface{}) {
	s.wsMu.RLock()
	defer s.wsMu.RUnlock()

	msg := map[string]interface{}{
		"type":    eventType,
		"data":    data,
		"version": time.Now().Unix(),
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for conn := range s.wsConns {
		err := conn.WriteMessage(websocket.TextMessage, jsonData)
		if err != nil {
			conn.Close()
		}
	}
}

func (s *Server) writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
