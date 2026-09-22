package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	PluginID = "vocat-sip"
)

type Config struct {
	ListenAddr      string `yaml:"listen_addr" json:"listen_addr"`
	SIPPort         int    `yaml:"sip_port" json:"sip_port"`
	RTPPortStart    int    `yaml:"rtp_port_start" json:"rtp_port_start"`
	RTPPortEnd      int    `yaml:"rtp_port_end" json:"rtp_port_end"`
	Domain          string `yaml:"domain" json:"domain"`
	Realm           string `yaml:"realm" json:"realm"`
	DataDir         string `yaml:"data_dir" json:"data_dir"`
	VoCatAPIBase    string `yaml:"vocat_api_base" json:"vocat_api_base"`
	VoCatAPIToken   string `yaml:"vocat_api_token" json:"vocat_api_token"`
	EnableTLS       bool   `yaml:"enable_tls" json:"enable_tls"`
	TLSCertPath     string `yaml:"tls_cert_path" json:"tls_cert_path"`
	TLSKeyPath      string `yaml:"tls_key_path" json:"tls_key_path"`
}

type SIPAccount struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	DisplayName  string `json:"display_name"`
	DeviceID     string `json:"device_id"`
	Enabled      bool   `json:"enabled"`
	AuthUsername string `json:"auth_username,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type Call struct {
	ID            string    `json:"id"`
	AccountID     string    `json:"account_id"`
	DeviceID      string    `json:"device_id"`
	Direction     string    `json:"direction"`
	From          string    `json:"from"`
	To            string    `json:"to"`
	State         string    `json:"state"`
	StartTime     time.Time `json:"start_time"`
	AnswerTime    *time.Time `json:"answer_time,omitempty"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	Duration      int       `json:"duration"`
	RTPPort       int       `json:"rtp_port,omitempty"`
	RemoteRTPAddr string    `json:"remote_rtp_addr,omitempty"`
	Codec         string    `json:"codec,omitempty"`
}

type Server struct {
	config     *Config
	sipConn    *net.UDPConn
	accounts   map[string]*SIPAccount
	calls      map[string]*Call
	rtpPorts   map[int]bool
	mu         sync.RWMutex
	wsUpgrader websocket.Upgrader
	wsClients  map[*websocket.Conn]bool
	wsMu       sync.RWMutex
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

func loadConfig() (*Config, error) {
	configPath := os.Getenv("VOCAT_PLUGIN_DATA_DIR")
	if configPath == "" {
		configPath = "/opt/vocat/data/plugins/vocat-sip"
	}
	
	filePath := configPath + "/config.yaml"
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(configPath), nil
		}
		return nil, err
	}
	
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func defaultConfig(dataDir string) *Config {
	return &Config{
		ListenAddr:   "0.0.0.0",
		SIPPort:      5060,
		RTPPortStart: 10000,
		RTPPortEnd:   10200,
		Domain:       "vocat.local",
		Realm:        "VoCat SIP",
		DataDir:      dataDir,
		VoCatAPIBase: "http://127.0.0.1:7575",
	}
}

func (s *Server) saveConfig() error {
	data, err := yaml.Marshal(s.config)
	if err != nil {
		return err
	}
	return os.WriteFile(s.config.DataDir+"/config.yaml", data, 0644)
}

func (s *Server) loadAccounts() error {
	data, err := os.ReadFile(s.config.DataDir + "/accounts.json")
	if err != nil {
		if os.IsNotExist(err) {
			s.accounts = make(map[string]*SIPAccount)
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &s.accounts)
}

func (s *Server) saveAccounts() error {
	data, err := json.MarshalIndent(s.accounts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.config.DataDir+"/accounts.json", data, 0644)
}

func (s *Server) loadCalls() error {
	data, err := os.ReadFile(s.config.DataDir + "/calls.json")
	if err != nil {
		if os.IsNotExist(err) {
			s.calls = make(map[string]*Call)
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &s.calls)
}

func (s *Server) saveCalls() error {
	data, err := json.MarshalIndent(s.calls, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.config.DataDir+"/calls.json", data, 0644)
}

func (s *Server) allocateRTPPort() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	for port := s.config.RTPPortStart; port <= s.config.RTPPortEnd; port += 2 {
		if !s.rtpPorts[port] {
			s.rtpPorts[port] = true
			s.rtpPorts[port+1] = true
			return port
		}
	}
	return 0
}

func (s *Server) releaseRTPPort(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rtpPorts, port)
	delete(s.rtpPorts, port+1)
}

func NewServer(config *Config) (*Server, error) {
	if err := os.MkdirAll(config.DataDir, 0750); err != nil {
		return nil, err
	}

	s := &Server{
		config:   config,
		accounts: make(map[string]*SIPAccount),
		calls:    make(map[string]*Call),
		rtpPorts: make(map[int]bool),
		stopCh:   make(chan struct{}),
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		wsClients: make(map[*websocket.Conn]bool),
	}

	if err := s.loadAccounts(); err != nil {
		return nil, err
	}
	if err := s.loadCalls(); err != nil {
		return nil, err
	}

	// Create UDP socket for SIP
	addr := fmt.Sprintf("%s:%d", config.ListenAddr, config.SIPPort)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("resolve UDP addr: %w", err)
	}
	
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, fmt.Errorf("listen UDP: %w", err)
	}
	
	s.sipConn = conn
	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	// Start SIP UDP listener
	s.wg.Add(1)
	go s.sipListener(ctx)

	// Start HTTP API server
	s.wg.Add(1)
	go s.runHTTPServer(ctx)

	// Start WebSocket broadcaster
	s.wg.Add(1)
	go s.broadcastLoop(ctx)

	log.Printf("SIP server started on %s:%d (UDP)", s.config.ListenAddr, s.config.SIPPort)
	return nil
}

func (s *Server) sipListener(ctx context.Context) {
	defer s.wg.Done()
	defer s.sipConn.Close()
	
	buf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
			s.sipConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, remoteAddr, err := s.sipConn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				if !isClosedError(err) {
					log.Printf("SIP read error: %v", err)
				}
				continue
			}
			
			if n > 0 {
				go s.handleSIPPacket(buf[:n], remoteAddr)
			}
		}
	}
}

func isClosedError(err error) bool {
	return err.Error() == "use of closed network connection"
}

func (s *Server) handleSIPPacket(data []byte, remoteAddr *net.UDPAddr) {
	// Simple SIP parsing - just log for now
	msg := string(data)
	log.Printf("SIP packet from %s: %s", remoteAddr, msg[:min(len(msg), 200)])
	
	// Respond to OPTIONS
	if len(data) > 7 && string(data[:7]) == "OPTIONS" {
		response := "SIP/2.0 200 OK\r\nAllow: INVITE, ACK, BYE, CANCEL, OPTIONS, REGISTER, MESSAGE\r\n\r\n"
		s.sipConn.WriteToUDP([]byte(response), remoteAddr)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *Server) runHTTPServer(ctx context.Context) {
	defer s.wg.Done()
	
	mux := http.NewServeMux()
	
	// REST API
	mux.HandleFunc("/api/accounts", s.handleAccountsAPI)
	mux.HandleFunc("/api/accounts/", s.handleAccountAPI)
	mux.HandleFunc("/api/calls", s.handleCallsAPI)
	mux.HandleFunc("/api/calls/", s.handleCallAPI)
	mux.HandleFunc("/api/settings", s.handleSettingsAPI)
	mux.HandleFunc("/api/devices", s.handleDevicesAPI)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:8080", s.config.ListenAddr),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	if s.config.EnableTLS {
		log.Fatal(server.ListenAndServeTLS(s.config.TLSCertPath, s.config.TLSKeyPath))
	} else {
		log.Fatal(server.ListenAndServe())
	}
}

func (s *Server) handleAccountsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		accounts := make([]*SIPAccount, 0, len(s.accounts))
		for _, acc := range s.accounts {
			accCopy := *acc
			accCopy.Password = ""
			accounts = append(accounts, &accCopy)
		}
		s.mu.RUnlock()
		json.NewEncoder(w).Encode(accounts)
	case http.MethodPost:
		var acc SIPAccount
		if err := json.NewDecoder(r.Body).Decode(&acc); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		acc.ID = generateID()
		acc.CreatedAt = time.Now().Format(time.RFC3339)
		acc.UpdatedAt = acc.CreatedAt
		s.mu.Lock()
		s.accounts[acc.ID] = &acc
		s.mu.Unlock()
		s.saveAccounts()
		s.broadcastEvent("account_created", map[string]any{"account": acc})
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(acc)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAccountAPI(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/accounts/"):]
	
	s.mu.Lock()
	acc, ok := s.accounts[id]
	s.mu.Unlock()
	
	if !ok {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		accCopy := *acc
		accCopy.Password = ""
		json.NewEncoder(w).Encode(accCopy)
	case http.MethodPut:
		var updated SIPAccount
		if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		updated.ID = id
		updated.CreatedAt = acc.CreatedAt
		updated.UpdatedAt = time.Now().Format(time.RFC3339)
		s.mu.Lock()
		s.accounts[id] = &updated
		s.mu.Unlock()
		s.saveAccounts()
		s.broadcastEvent("account_updated", map[string]any{"account": updated})
		json.NewEncoder(w).Encode(updated)
	case http.MethodDelete:
		s.mu.Lock()
		delete(s.accounts, id)
		s.mu.Unlock()
		s.saveAccounts()
		s.broadcastEvent("account_deleted", map[string]any{"account_id": id})
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCallsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	s.mu.RLock()
	calls := make([]*Call, 0, len(s.calls))
	for _, call := range s.calls {
		calls = append(calls, call)
	}
	s.mu.RUnlock()
	json.NewEncoder(w).Encode(calls)
}

func (s *Server) handleCallAPI(w http.ResponseWriter, r *http.Request) {
	// Handle /api/calls/{id}
}

func (s *Server) handleSettingsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfgCopy := *s.config
		cfgCopy.VoCatAPIToken = ""
		json.NewEncoder(w).Encode(cfgCopy)
	case http.MethodPut:
		var cfg Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.config = &cfg
		s.saveConfig()
		json.NewEncoder(w).Encode(cfg)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDevicesAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	req, _ := http.NewRequestWithContext(r.Context(), "GET", s.config.VoCatAPIBase+"/api/devices", nil)
	if s.config.VoCatAPIToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.VoCatAPIToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(json.RawMessage{})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	
	s.wsMu.Lock()
	s.wsClients[conn] = true
	s.wsMu.Unlock()
	
	// Send initial state
	s.sendWSState(conn)
	
	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
	
	s.wsMu.Lock()
	delete(s.wsClients, conn)
	s.wsMu.Unlock()
	conn.Close()
}

func (s *Server) sendWSState(conn *websocket.Conn) {
	s.mu.RLock()
	accounts := make([]*SIPAccount, 0, len(s.accounts))
	for _, acc := range s.accounts {
		accCopy := *acc
		accCopy.Password = ""
		accounts = append(accounts, &accCopy)
	}
	calls := make([]*Call, 0, len(s.calls))
	for _, call := range s.calls {
		calls = append(calls, call)
	}
	s.mu.RUnlock()
	
	msg := map[string]any{
		"type":      "state",
		"accounts":  accounts,
		"calls":     calls,
		"config":    s.config,
	}
	conn.WriteJSON(msg)
}

func (s *Server) broadcastLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.broadcastEvent("heartbeat", map[string]any{"time": time.Now()})
		}
	}
}

func (s *Server) broadcastEvent(eventType string, data map[string]any) {
	msg := map[string]any{
		"type":  "event",
		"event": eventType,
		"data":  data,
		"time":  time.Now(),
	}
	
	s.wsMu.RLock()
	conns := make([]*websocket.Conn, 0, len(s.wsClients))
	for conn := range s.wsClients {
		conns = append(conns, conn)
	}
	s.wsMu.RUnlock()
	
	for _, conn := range conns {
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("WebSocket write error: %v", err)
			conn.Close()
			s.wsMu.Lock()
			delete(s.wsClients, conn)
			s.wsMu.Unlock()
		}
	}
}

func (s *Server) Stop() error {
	close(s.stopCh)
	
	s.mu.Lock()
	for _, call := range s.calls {
		if call.RTPPort > 0 {
			s.releaseRTPPort(call.RTPPort)
		}
	}
	s.mu.Unlock()
	
	if s.sipConn != nil {
		s.sipConn.Close()
	}
	
	s.wg.Wait()
	return nil
}

func generateID() string {
	return uuid.New().String()
}

func generateCallID() string {
	return fmt.Sprintf("call-%d", time.Now().UnixNano())
}

func generateNonce() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

func main() {
	var (
		configPath = flag.String("config", "", "Config file path")
		version    = flag.Bool("version", false, "Print version")
	)
	flag.Parse()
	
	if *version {
		fmt.Println("vocat-sip v1.0.0")
		return
	}
	
	if *configPath != "" {
		os.Setenv("VOCAT_PLUGIN_DATA_DIR", *configPath)
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	server, err := NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	
	if err := server.Start(ctx); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	
	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	
	log.Println("Shutting down...")
	cancel()
	server.Stop()
	log.Println("Server stopped")
}