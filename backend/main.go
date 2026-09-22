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

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/gorilla/websocket"
)

const (
	PluginID = "vocat-sip"
)

type Config struct {
	ListenAddr      string `json:"listen_addr"`
	SIPPort         int    `json:"sip_port"`
	RTPPortStart    int    `json:"rtp_port_start"`
	RTPPortEnd      int    `json:"rtp_port_end"`
	Domain          string `json:"domain"`
	Realm           string `json:"realm"`
	DataDir         string `json:"data_dir"`
	VoCatAPIBase    string `json:"vocat_api_base"`
	VoCatAPIToken   string `json:"vocat_api_token"`
	EnableTLS       bool   `json:"enable_tls"`
	TLSCertPath     string `json:"tls_cert_path"`
	TLSKeyPath      string `json:"tls_key_path"`
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
	ID           string    `json:"id"`
	AccountID    string    `json:"account_id"`
	DeviceID     string    `json:"device_id"`
	Direction    string    `json:"direction"` // inbound, outbound
	From         string    `json:"from"`
	To           string    `json:"to"`
	State        string    `json:"state"` // trying, ringing, in-progress, completed, failed
	StartTime    time.Time `json:"start_time"`
	AnswerTime   *time.Time `json:"answer_time,omitempty"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	Duration     int       `json:"duration"`
	RTPPort      int       `json:"rtp_port,omitempty"`
	RemoteRTPAddr string   `json:"remote_rtp_addr,omitempty"`
	Codec        string    `json:"codec,omitempty"`
}

type Server struct {
	config     *Config
	sipServer  *sipgo.Server
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
	
	filePath := configPath + "/config.json"
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(configPath), nil
		}
		return nil, err
	}
	
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
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
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.config.DataDir+"/config.json", data, 0644)
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

	// Build SIP server
	sipServer, err := sipgo.NewServer(
		sipgo.WithServerAddr(fmt.Sprintf("%s:%d", config.ListenAddr, config.SIPPort)),
		sipgo.WithServerTransport("udp"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SIP server: %w", err)
	}

	s.sipServer = sipServer
	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	// Start SIP server
	if err := s.sipServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start SIP server: %w", err)
	}

	// Register handlers
	s.registerHandlers()

	// Start HTTP API server
	s.wg.Add(1)
	go s.runHTTPServer(ctx)

	// Start WebSocket broadcaster
	s.wg.Add(1)
	go s.broadcastLoop(ctx)

	log.Printf("SIP server started on %s:%d", s.config.ListenAddr, s.config.SIPPort)
	return nil
}

func (s *Server) registerHandlers() {
	// REGISTER
	s.sipServer.OnRequest(sip.REGISTER, s.handleRegister)
	// INVITE
	s.sipServer.OnRequest(sip.INVITE, s.handleInvite)
	// ACK
	s.sipServer.OnRequest(sip.ACK, s.handleAck)
	// BYE
	s.sipServer.OnRequest(sip.BYE, s.handleBye)
	// CANCEL
	s.sipServer.OnRequest(sip.CANCEL, s.handleCancel)
	// OPTIONS
	s.sipServer.OnRequest(sip.OPTIONS, s.handleOptions)
	// MESSAGE (for SMS)
	s.sipServer.OnRequest(sip.MESSAGE, s.handleMessage)
	// SUBSCRIBE
	s.sipServer.OnRequest(sip.SUBSCRIBE, s.handleSubscribe)
	// NOTIFY
	s.sipServer.OnRequest(sip.NOTIFY, s.handleNotify)
	// INFO
	s.sipServer.OnRequest(sip.INFO, s.handleInfo)
}

func (s *Server) handleRegister(req *sip.Request, tx sip.ServerTransaction) {
	log.Printf("REGISTER from %s", req.From())
	
	// Extract credentials
	authHeader := req.GetHeader("Authorization")
	if authHeader == nil {
		// Challenge with digest auth
		s.challengeAuth(req, tx, false)
		return
	}

	// Parse digest auth
	username, password, ok := s.parseDigestAuth(authHeader.Value())
	if !ok {
		s.challengeAuth(req, tx, false)
		return
	}

	// Find account
	account, ok := s.findAccountByUsername(username)
	if !ok || !account.Enabled {
		tx.Respond(sip.NewResponseFromRequest("", req, sip.StatusUnauthorized, "Invalid credentials"))
		return
	}

	// Verify password (in production, use proper digest verification)
	if password != account.Password {
		s.challengeAuth(req, tx, false)
		return
	}

	// Check Contact header
	contact := req.GetHeader("Contact")
	if contact == nil {
		tx.Respond(sip.NewResponseFromRequest("", req, sip.StatusBadRequest, "Missing Contact"))
		return
	}

	// Success
	resp := sip.NewResponseFromRequest("", req, sip.StatusOK, "OK")
	resp.AppendHeader(sip.NewHeader("Contact", contact.Value()))
	resp.AppendHeader(sip.NewHeader("Expires", "3600"))
	tx.Respond(resp)
	
	log.Printf("Account %s registered successfully", account.Username)
	s.broadcastEvent("account_registered", map[string]any{"account_id": account.ID})
}

func (s *Server) challengeAuth(req *sip.Request, tx sip.ServerTransaction, proxy bool) {
	nonce := generateNonce()
	realm := s.config.Realm
	
	var authHeader string
	if proxy {
		authHeader = fmt.Sprintf(`Proxy-Authenticate: Digest realm="%s", nonce="%s", algorithm=MD5`, realm, nonce)
	} else {
		authHeader = fmt.Sprintf(`WWW-Authenticate: Digest realm="%s", nonce="%s", algorithm=MD5`, realm, nonce)
	}
	
	resp := sip.NewResponseFromRequest("", req, sip.StatusUnauthorized, "Unauthorized")
	resp.AppendHeader(sip.NewHeader("WWW-Authenticate", fmt.Sprintf(`Digest realm="%s", nonce="%s", algorithm=MD5`, realm, nonce)))
	tx.Respond(resp)
}

func (s *Server) parseDigestAuth(auth string) (username, password string, ok bool) {
	// Simplified digest auth parsing - in production use proper parsing
	return "", "", false
}

func (s *Server) findAccountByUsername(username string) (*SIPAccount, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, acc := range s.accounts {
		if acc.Username == username {
			return acc, true
		}
	}
	return nil, false
}

func (s *Server) handleInvite(req *sip.Request, tx sip.ServerTransaction) {
	log.Printf("INVITE from %s to %s", req.From(), req.To())
	
	// Find account by From header
	fromURI := req.From().Address
	account := s.findAccountByFromURI(fromURI.String())
	if account == nil {
		tx.Respond(sip.NewResponseFromRequest("", req, sip.StatusForbidden, "Account not found"))
		return
	}

	// Allocate RTP port
	rtpPort := s.allocateRTPPort()
	if rtpPort == 0 {
		tx.Respond(sip.NewResponseFromRequest("", req, sip.StatusServiceUnavailable, "No RTP ports available"))
		return
	}

	// Create call
	callID := generateCallID()
	call := &Call{
		ID:        callID,
		AccountID: account.ID,
		DeviceID:  account.DeviceID,
		Direction: "inbound",
		From:      req.From().Address.String(),
		To:        req.To().Address.String(),
		State:     "ringing",
		StartTime: time.Now(),
		RTPPort:   rtpPort,
	}
	
	s.mu.Lock()
	s.calls[callID] = call
	s.mu.Unlock()
	s.saveCalls()

	// Send 100 Trying
	trying := sip.NewResponseFromRequest("", req, sip.StatusTrying, "Trying")
	tx.Respond(trying)

	// Send 180 Ringing
	ringing := sip.NewResponseFromRequest("", req, sip.StatusRinging, "Ringing")
	ringing.AppendHeader(sip.NewHeader("Contact", fmt.Sprintf("<sip:%s@%s:%d;transport=udp>", account.Username, s.config.Domain, s.config.SIPPort)))
	tx.Respond(ringing)

	// Notify via WebSocket
	s.broadcastEvent("call_incoming", map[string]any{
		"call_id":   callID,
		"from":      call.From,
		"to":        call.To,
		"account":   account.Username,
	})

	// Wait for ACK (client answers) - in real implementation, this would be async
	// For now, we'll simulate auto-answer after 3 seconds
	go func() {
		time.Sleep(3 * time.Second)
		s.answerCall(callID)
	}()
}

func (s *Server) answerCall(callID string) {
	s.mu.Lock()
	call, ok := s.calls[callID]
	s.mu.Unlock()
	if !ok {
		return
	}

	now := time.Now()
	call.State = "in-progress"
	call.AnswerTime = &now
	s.saveCalls()
	s.broadcastEvent("call_answered", map[string]any{"call_id": callID})
}

func (s *Server) handleAck(req *sip.Request, tx sip.ServerTransaction) {
	// ACK doesn't create a transaction response
	log.Printf("ACK received")
}

func (s *Server) handleBye(req *sip.Request, tx sip.ServerTransaction) {
	callID := req.CallID().Value()
	log.Printf("BYE for call %s", callID)
	
	s.endCall(callID, "completed")
	
	resp := sip.NewResponseFromRequest("", req, sip.StatusOK, "OK")
	tx.Respond(resp)
}

func (s *Server) handleCancel(req *sip.Request, tx sip.ServerTransaction) {
	callID := req.CallID().Value()
	log.Printf("CANCEL for call %s", callID)
	
	s.endCall(callID, "canceled")
	
	resp := sip.NewResponseFromRequest("", req, sip.StatusOK, "OK")
	tx.Respond(resp)
}

func (s *Server) endCall(callID, state string) {
	s.mu.Lock()
	call, ok := s.calls[callID]
	if ok {
		call.State = state
		now := time.Now()
		call.EndTime = &now
		if call.AnswerTime != nil {
			call.Duration = int(now.Sub(*call.AnswerTime).Seconds())
		}
		if call.RTPPort > 0 {
			s.releaseRTPPort(call.RTPPort)
		}
		s.saveCalls()
	}
	s.mu.Unlock()
	
	s.broadcastEvent("call_ended", map[string]any{
		"call_id": callID,
		"state":   state,
	})
}

func (s *Server) handleOptions(req *sip.Request, tx sip.ServerTransaction) {
	resp := sip.NewResponseFromRequest("", req, sip.StatusOK, "OK")
	resp.AppendHeader(sip.NewHeader("Allow", "INVITE, ACK, BYE, CANCEL, OPTIONS, REGISTER, MESSAGE, SUBSCRIBE, NOTIFY, INFO"))
	tx.Respond(resp)
}

func (s *Server) handleMessage(req *sip.Request, tx sip.ServerTransaction) {
	log.Printf("MESSAGE from %s", req.From())
	
	// Handle SIP MESSAGE for SMS
	body := req.Body()
	to := req.To().Address.String()
	
	// Forward to VoCat for SMS sending via cellular
	go s.sendSMSViaVoCat(req.From().Address.String(), to, string(body))
	
	resp := sip.NewResponseFromRequest("", req, sip.StatusOK, "OK")
	tx.Respond(resp)
}

func (s *Server) handleSubscribe(req *sip.Request, tx sip.ServerTransaction) {
	// Handle presence subscription
	resp := sip.NewResponseFromRequest("", req, sip.StatusOK, "OK")
	tx.Respond(resp)
}

func (s *Server) handleNotify(req *sip.Request, tx sip.ServerTransaction) {
	resp := sip.NewResponseFromRequest("", req, sip.StatusOK, "OK")
	tx.Respond(resp)
}

func (s *Server) handleInfo(req *sip.Request, tx sip.ServerTransaction) {
	// Handle DTMF via INFO
	resp := sip.NewResponseFromRequest("", req, sip.StatusOK, "OK")
	tx.Respond(resp)
}

func (s *Server) findAccountByFromURI(uri string) *SIPAccount {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, acc := range s.accounts {
		if fmt.Sprintf("sip:%s@%s", acc.Username, s.config.Domain) == uri {
			return acc
		}
	}
	return nil
}

func (s *Server) sendSMSViaVoCat(from, to, body string) {
	// Call VoCat API to send SMS via cellular modem
	// This would need the device ID from the account
	log.Printf("SMS from %s to %s: %s", from, to, body)
}

func (s *Server) runHTTPServer(ctx context.Context) {
	defer s.wg.Done()
	
	mux := http.NewServeMux()
	
	// REST API
	mux.HandleFunc("/api/accounts", s.handleAccountsAPI)
	mux.HandleFunc("/api/accounts/", s.handleAccountAPI)
	mux.HandleFunc("/api/calls", s.handleCallsAPI)
	mux.HandleFunc("/api/calls/", s.handleCallAPI)
	mux.HandleFunc("/api/calls/", s.handleCallActionAPI)
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
			// Don't expose password
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

func (s *Server) handleCallActionAPI(w http.ResponseWriter, r *http.Request) {
	// Handle call actions like answer, hangup, hold, etc.
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
	
	// Fetch devices from VoCat API
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
		"type": "event",
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
	
	if s.sipServer != nil {
		s.sipServer.Stop()
	}
	
	s.wg.Wait()
	return nil
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
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