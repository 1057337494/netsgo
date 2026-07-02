package server

import (
	"net/http"
	"testing"
	"time"

	"netsgo/pkg/protocol"
)

func registerOfflineHTTPTestClient(t *testing.T, s *Server, hostname string) string {
	t.Helper()

	capabilities := protocol.DefaultClientCapabilities()
	record, err := s.auth.adminStore.GetOrCreateClient(
		"install-"+hostname,
		protocol.ClientInfo{
			Hostname:     hostname,
			OS:           "linux",
			Arch:         "amd64",
			Version:      "test",
			Capabilities: &capabilities,
		},
		"127.0.0.1:12345",
	)
	if err != nil {
		t.Fatalf("failed to register offline client: %v", err)
	}
	return record.ID
}

func TestLoadOfflineManagedTunnelBySelectorPrefersNameOverID(t *testing.T) {
	s, _, _, cleanup := setupTestServerWithStores(t, true)
	defer cleanup()

	clientID := registerOfflineHTTPTestClient(t, s, "offline-selector")
	seedStoredTunnel(t, s, clientID, protocol.ProxyNewRequest{
		ID:         "name-tunnel-id",
		Name:       "id-of-other",
		Type:       protocol.ProxyTypeTCP,
		RemotePort: 18081,
	}, protocol.ProxyStatusStopped)
	seedStoredTunnel(t, s, clientID, protocol.ProxyNewRequest{
		ID:         "id-of-other",
		Name:       "other",
		Type:       protocol.ProxyTypeTCP,
		RemotePort: 18082,
	}, protocol.ProxyStatusStopped)

	stored, err := s.loadOfflineManagedTunnelBySelector(clientID, "id-of-other")
	if err != nil {
		t.Fatalf("loadOfflineManagedTunnelBySelector failed: %v", err)
	}
	if stored.Name != "id-of-other" || stored.ID != "name-tunnel-id" {
		t.Fatalf("selector should prefer exact name matches over ID matches, got name=%q id=%q", stored.Name, stored.ID)
	}
}

func TestOfflineHTTPTunnel_Update_StoreFirst(t *testing.T) {
	s, handler, token, cleanup := setupTestServerWithStores(t, true)
	defer cleanup()

	clientID := registerOfflineHTTPTestClient(t, s, "offline-update")
	createResp := doMuxRequest(t, handler, http.MethodPost, "/api/tunnels", token,
		unifiedOfflineCreatePayload("offline-http", clientID, "http_host", 0, "old.example.com"))
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d body=%s", createResp.Code, createResp.Body.String())
	}
	var created tunnelSpecAPI
	if err := mustDecodeJSON(t, createResp.Body, &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	if err := checkDomainConflict("old.example.com", "", "", s); err == nil {
		t.Fatal("before update, old.example.com should be claimed by the existing HTTP tunnel")
	}

	updateBody := []byte(`{"expected_revision":1,"spec":{"name":"offline-http","topology":"server_expose","ingress":{"location":"server","type":"http_host","config":{"domain":"new.example.com","allowed_source_cidrs":["0.0.0.0/0","::/0"],"auth":{"type":"none"}}},"target":{"location":"client","client_id":"` + clientID + `","type":"tcp_service","config":{"ip":"192.168.1.50","port":8080}},"transport_policy":"server_relay_only"}}`)
	resp := doMuxRequest(t, handler, http.MethodPut, "/api/tunnels/"+created.ID, token, updateBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("offline HTTP update: want 200, got %d, body=%s", resp.Code, resp.Body.String())
	}

	var payload struct {
		Tunnel tunnelSpecAPI `json:"tunnel"`
	}
	if err := mustDecodeJSON(t, resp.Body, &payload); err != nil {
		t.Fatalf("failed to parse update response: %v", err)
	}
	if payload.Tunnel.Capabilities == nil {
		t.Fatalf("update response should include capabilities, got %v", payload.Tunnel.Capabilities)
	}

	stored, err := s.store.GetTunnelByIDE(clientID, created.ID)
	if err != nil {
		t.Fatalf("updated tunnel should remain in store: %v", err)
	}
	if stored.Domain != "new.example.com" {
		t.Fatalf("Domain after update: want new.example.com, got %s", stored.Domain)
	}
	if stored.LocalIP != "192.168.1.50" || stored.LocalPort != 8080 {
		t.Fatalf("update fields mismatch: got LocalIP=%s LocalPort=%d", stored.LocalIP, stored.LocalPort)
	}
	if stored.DesiredState != protocol.ProxyDesiredStateRunning || stored.RuntimeState != protocol.ProxyRuntimeStateOffline {
		t.Fatalf("offline running HTTP tunnel should remain running/offline after update, got %s/%s", stored.DesiredState, stored.RuntimeState)
	}
}

func TestOfflineHTTPTunnel_Stop_StoreFirstUsesStoppedState(t *testing.T) {
	s, handler, token, cleanup := setupTestServerWithStores(t, true)
	defer cleanup()

	clientID := registerOfflineHTTPTestClient(t, s, "offline-stop-stopped")
	createResp := doMuxRequest(t, handler, http.MethodPost, "/api/tunnels", token,
		unifiedOfflineCreatePayload("offline-http", clientID, "http_host", 0, "stop.example.com"))
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d body=%s", createResp.Code, createResp.Body.String())
	}
	var created tunnelSpecAPI
	if err := mustDecodeJSON(t, createResp.Body, &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	resp := doMuxRequest(t, handler, http.MethodPut, "/api/tunnels/"+created.ID+"/stop", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("offline HTTP stop: want 200, got %d, body=%s", resp.Code, resp.Body.String())
	}

	stored, err := s.store.GetTunnelByIDE(clientID, created.ID)
	if err != nil {
		t.Fatal("store record should still exist after offline stop")
	}
	if stored.DesiredState != protocol.ProxyDesiredStateStopped {
		t.Fatalf("desired_state after offline stop: want stopped, got %s", stored.DesiredState)
	}
	if stored.RuntimeState != protocol.ProxyRuntimeStateIdle {
		t.Fatalf("runtime_state after offline stop: want idle, got %s", stored.RuntimeState)
	}
}

func TestOfflineHTTPTunnel_Delete_StoreFirst(t *testing.T) {
	s, handler, token, cleanup := setupTestServerWithStores(t, true)
	defer cleanup()
	clientID := registerOfflineHTTPTestClient(t, s, "offline-delete")
	createResp := doMuxRequest(t, handler, http.MethodPost, "/api/tunnels", token,
		unifiedOfflineCreatePayload("offline-http", clientID, "http_host", 0, "delete.example.com"))
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d body=%s", createResp.Code, createResp.Body.String())
	}
	var created tunnelSpecAPI
	if err := mustDecodeJSON(t, createResp.Body, &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	resp := doMuxRequest(t, handler, http.MethodDelete, "/api/tunnels/"+created.ID, token, nil)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("offline HTTP delete: want 204, got %d, body=%s", resp.Code, resp.Body.String())
	}
	if _, err := s.store.GetTunnelByIDE(clientID, created.ID); err == nil {
		t.Fatal("store record should be deleted after offline delete")
	}
	if err := checkDomainConflict("delete.example.com", "", "", s); err != nil {
		t.Fatalf("domain should be released after delete, got err: %v", err)
	}
}

func TestOfflineHTTPTunnel_Resume_StoreFirst(t *testing.T) {
	s, handler, token, cleanup := setupTestServerWithStores(t, true)
	defer cleanup()
	clientID := registerOfflineHTTPTestClient(t, s, "offline-resume")
	createResp := doMuxRequest(t, handler, http.MethodPost, "/api/tunnels", token,
		unifiedOfflineCreatePayload("offline-http", clientID, "http_host", 0, "resume.example.com"))
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d body=%s", createResp.Code, createResp.Body.String())
	}
	var created tunnelSpecAPI
	if err := mustDecodeJSON(t, createResp.Body, &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	stopResp := doMuxRequest(t, handler, http.MethodPut, "/api/tunnels/"+created.ID+"/stop", token, nil)
	if stopResp.Code != http.StatusOK {
		t.Fatalf("stop expected 200, got %d body=%s", stopResp.Code, stopResp.Body.String())
	}
	resp := doMuxRequest(t, handler, http.MethodPut, "/api/tunnels/"+created.ID+"/resume", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("offline HTTP resume: want 200, got %d, body=%s", resp.Code, resp.Body.String())
	}
	stored, err := s.store.GetTunnelByIDE(clientID, created.ID)
	if err != nil {
		t.Fatal("store record should still exist after offline resume")
	}
	if stored.DesiredState != protocol.ProxyDesiredStateRunning || stored.RuntimeState != protocol.ProxyRuntimeStateOffline {
		t.Fatalf("after offline resume: want running/offline, got %s/%s", stored.DesiredState, stored.RuntimeState)
	}
}

func TestOfflineHTTPTunnel_ResumeRunningTunnelReturnsNotAllowed(t *testing.T) {
	s, handler, token, cleanup := setupTestServerWithStores(t, true)
	defer cleanup()
	clientID := registerOfflineHTTPTestClient(t, s, "offline-resume-running")
	createResp := doMuxRequest(t, handler, http.MethodPost, "/api/tunnels", token,
		unifiedOfflineCreatePayload("offline-http", clientID, "http_host", 0, "resume-running.example.com"))
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d body=%s", createResp.Code, createResp.Body.String())
	}
	var created tunnelSpecAPI
	if err := mustDecodeJSON(t, createResp.Body, &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	resp := doMuxRequest(t, handler, http.MethodPut, "/api/tunnels/"+created.ID+"/resume", token, nil)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("resume running tunnel: want 400, got %d body=%s", resp.Code, resp.Body.String())
	}
	var body tunnelMutationErrorResponse
	if err := mustDecodeJSON(t, resp.Body, &body); err != nil {
		t.Fatalf("decode resume error: %v", err)
	}
	if body.ErrorCode != protocol.TunnelMutationErrorCodeResumeNotAllowed || body.Code != protocol.TunnelMutationErrorCodeResumeNotAllowed {
		t.Fatalf("resume error code mismatch: %+v", body)
	}
	stored, err := s.store.GetTunnelByIDE(clientID, created.ID)
	if err != nil {
		t.Fatalf("stored tunnel should remain present: %v", err)
	}
	if stored.DesiredState != protocol.ProxyDesiredStateRunning || stored.RuntimeState != protocol.ProxyRuntimeStateOffline {
		t.Fatalf("resume rejection should not change state, got %s/%s", stored.DesiredState, stored.RuntimeState)
	}
}

func TestOfflineHTTPTunnel_Stop_StoreFirst(t *testing.T) {
	s, handler, token, cleanup := setupTestServerWithStores(t, true)
	defer cleanup()
	clientID := registerOfflineHTTPTestClient(t, s, "offline-stop")
	createResp := doMuxRequest(t, handler, http.MethodPost, "/api/tunnels", token,
		unifiedOfflineCreatePayload("offline-http", clientID, "http_host", 0, "stop2.example.com"))
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d body=%s", createResp.Code, createResp.Body.String())
	}
	var created tunnelSpecAPI
	if err := mustDecodeJSON(t, createResp.Body, &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	resp := doMuxRequest(t, handler, http.MethodPut, "/api/tunnels/"+created.ID+"/stop", token, []byte(`{}`))
	if resp.Code != http.StatusOK {
		t.Fatalf("offline HTTP stop: want 200, got %d, body=%s", resp.Code, resp.Body.String())
	}
	stored, err := s.store.GetTunnelByIDE(clientID, created.ID)
	if err != nil {
		t.Fatal("store record should still exist after offline stop")
	}
	if stored.DesiredState != protocol.ProxyDesiredStateStopped || stored.RuntimeState != protocol.ProxyRuntimeStateIdle {
		t.Fatalf("after offline stop: want stopped/idle, got %s/%s", stored.DesiredState, stored.RuntimeState)
	}
}

func TestLifecycle_ClientDisconnect_DoesNotRewriteStoreState(t *testing.T) {
	s, ts, cleanup := setupWSTestNoConn(t)
	defer cleanup()

	s.store = newTestTunnelStore(t)

	wsConn, authResp := connectAndAuth(t, ts, "disconnect-http-store")
	defer mustClose(t, wsConn)
	deadline := time.Now().Add(2 * time.Second)
	var liveClient *ClientConn
	for time.Now().Before(deadline) {
		if value, ok := s.clients.Load(authResp.ClientID); ok {
			client := value.(*ClientConn)
			if client.getState() == clientStateLive {
				liveClient = client
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if liveClient == nil {
		t.Fatal("timed out waiting for live client")
		return
	}

	liveClient.proxyMu.Lock()
	liveClient.proxies["active-http"] = &ProxyTunnel{
		Config: protocol.ProxyConfig{
			Name:         "active-http",
			Type:         protocol.ProxyTypeHTTP,
			LocalIP:      "127.0.0.1",
			LocalPort:    3000,
			Domain:       "keep-active.example.com",
			ClientID:     authResp.ClientID,
			DesiredState: protocol.ProxyDesiredStateRunning,
			RuntimeState: protocol.ProxyRuntimeStateExposed,
		},
		done: make(chan struct{}),
	}
	liveClient.proxyMu.Unlock()

	mustAddStableTunnel(t, s.store, StoredTunnel{
		ProxyNewRequest: protocol.ProxyNewRequest{
			Name:      "active-http",
			Type:      protocol.ProxyTypeHTTP,
			LocalIP:   "127.0.0.1",
			LocalPort: 3000,
			Domain:    "keep-active.example.com",
		},
		DesiredState: protocol.ProxyDesiredStateRunning,
		RuntimeState: protocol.ProxyRuntimeStateExposed,
		ClientID:     authResp.ClientID,
		Hostname:     "disconnect-http-store",
	})

	if !s.invalidateLogicalSessionIfCurrent(authResp.ClientID, liveClient.generation, "test_disconnect") {
		t.Fatal("disconnect should successfully invalidate the current logical session")
	}

	stored, exists := s.store.GetTunnel(authResp.ClientID, "active-http")
	if !exists {
		t.Fatal("HTTP tunnel record in the store should remain after disconnect")
	}
	if stored.DesiredState != protocol.ProxyDesiredStateRunning || stored.RuntimeState != protocol.ProxyRuntimeStateExposed {
		t.Fatalf("client disconnect should not rewrite the store target state, got %s/%s", stored.DesiredState, stored.RuntimeState)
	}
	if stored.Domain != "keep-active.example.com" {
		t.Fatalf("Domain should be preserved after client disconnect, got %s", stored.Domain)
	}
}
