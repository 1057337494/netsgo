package server

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"

	"netsgo/pkg/protocol"
)

func unifiedOfflineCreatePayload(name, clientID, ingressType string, port int, domain string) []byte {
	ingressCfg := ""
	switch ingressType {
	case "tcp_listen", "udp_listen":
		ingressCfg = `"bind_ip":"0.0.0.0","port":` + strconv.Itoa(port) + `,"allowed_source_cidrs":["0.0.0.0/0","::/0"]`
	case "http_host":
		ingressCfg = `"domain":"` + domain + `","allowed_source_cidrs":["0.0.0.0/0","::/0"],"auth":{"type":"none"}`
	}
	targetType := "tcp_service"
	if ingressType == "udp_listen" {
		targetType = "udp_service"
	}
	return []byte(`{"name":"` + name + `","topology":"server_expose","ingress":{"location":"server","type":"` + ingressType + `","config":{` + ingressCfg + `}},"target":{"location":"client","client_id":"` + clientID + `","type":"` + targetType + `","config":{"ip":"127.0.0.1","port":8080}},"transport_policy":"server_relay_only"}`)
}
func unifiedOfflineCreatePayloadWithBindIP(name, clientID, bindIP string, port int) []byte {
	return []byte(`{"name":"` + name + `","topology":"server_expose","ingress":{"location":"server","type":"tcp_listen","config":{"bind_ip":"` + bindIP + `","port":` + strconv.Itoa(port) + `,"allowed_source_cidrs":["0.0.0.0/0","::/0"]}},"target":{"location":"client","client_id":"` + clientID + `","type":"tcp_service","config":{"ip":"127.0.0.1","port":8080}},"transport_policy":"server_relay_only"}`)
}

func TestOfflineManagedTunnel_Create_StoreFirstAcrossTypes(t *testing.T) {
	testCases := []struct {
		name        string
		ingressType string
		remotePort  int
		domain      string
	}{
		{name: "tcp", ingressType: "tcp_listen", remotePort: reserveTCPPort(t)},
		{name: "udp", ingressType: "udp_listen", remotePort: reserveUDPPort(t)},
		{name: "http", ingressType: "http_host", remotePort: 0, domain: "offline-created.example.com"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, handler, token, cleanup := setupTestServerWithStores(t, true)
			defer cleanup()
			clientID := registerOfflineHTTPTestClient(t, s, "offline-create-"+tc.name)
			resp := doMuxRequest(t, handler, http.MethodPost, "/api/tunnels", token,
				unifiedOfflineCreatePayload("offline-"+tc.name, clientID, tc.ingressType, tc.remotePort, tc.domain))
			if resp.Code != http.StatusCreated {
				t.Fatalf("Offline %s create expected 201, got %d, body=%s", tc.ingressType, resp.Code, resp.Body.String())
			}
			var created tunnelSpecAPI
			if err := mustDecodeJSON(t, resp.Body, &created); err != nil {
				t.Fatalf("Failed to parse create response: %v", err)
			}
			if created.ID == "" {
				t.Fatal("Create response should include server-owned id")
			}
			stored, err := s.store.GetTunnelByIDE(clientID, created.ID)
			if err != nil {
				t.Fatalf("Offline %s create should have tunnel record in store: %v", tc.ingressType, err)
			}
			if stored.DesiredState != protocol.ProxyDesiredStateRunning {
				t.Fatalf("desired_state expected running, got %s", stored.DesiredState)
			}
			if stored.RuntimeState != protocol.ProxyRuntimeStateOffline {
				t.Fatalf("runtime_state expected offline, got %s", stored.RuntimeState)
			}
			switch tc.ingressType {
			case "tcp_listen":
				ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", tc.remotePort))
				if err != nil {
					t.Fatalf("Offline TCP create port should not be listened, got %v", err)
				}
				_ = ln.Close()
			case "udp_listen":
				conn, err := net.ListenPacket("udp", fmt.Sprintf("127.0.0.1:%d", tc.remotePort))
				if err != nil {
					t.Fatalf("Offline UDP create port should not be listened, got %v", err)
				}
				_ = conn.Close()
			case "http_host":
				if err := checkDomainConflict(tc.domain, "", "", s); err == nil {
					t.Fatalf("Offline HTTP create domain %s should be reserved immediately", tc.domain)
				}
			}
		})
	}
}

func TestOfflineManagedTunnel_Update_StoreFirstForTCPAndUDP(t *testing.T) {
	testCases := []struct {
		name        string
		ingressType string
		oldPort     int
		newPort     int
	}{
		{name: "tcp", ingressType: "tcp_listen", oldPort: reserveTCPPort(t), newPort: reserveTCPPort(t)},
		{name: "udp", ingressType: "udp_listen", oldPort: reserveUDPPort(t), newPort: reserveUDPPort(t)},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, handler, token, cleanup := setupTestServerWithStores(t, true)
			defer cleanup()
			clientID := registerOfflineHTTPTestClient(t, s, "offline-update-"+tc.name)
			createResp := doMuxRequest(t, handler, http.MethodPost, "/api/tunnels", token,
				unifiedOfflineCreatePayload("offline-"+tc.name, clientID, tc.ingressType, tc.oldPort, ""))
			if createResp.Code != http.StatusCreated {
				t.Fatalf("create expected 201, got %d body=%s", createResp.Code, createResp.Body.String())
			}
			var created tunnelSpecAPI
			if err := mustDecodeJSON(t, createResp.Body, &created); err != nil {
				t.Fatalf("decode create response: %v", err)
			}
			targetType := "tcp_service"
			if tc.ingressType == "udp_listen" {
				targetType = "udp_service"
			}
			updateBody := []byte(`{"expected_revision":1,"spec":{"name":"offline-` + tc.name + `","topology":"server_expose","ingress":{"location":"server","type":"` + tc.ingressType + `","config":{"bind_ip":"0.0.0.0","port":` + strconv.Itoa(tc.newPort) + `,"allowed_source_cidrs":["0.0.0.0/0","::/0"]}},"target":{"location":"client","client_id":"` + clientID + `","type":"` + targetType + `","config":{"ip":"192.168.1.50","port":9090}},"transport_policy":"server_relay_only"}}`)
			resp := doMuxRequest(t, handler, http.MethodPut, "/api/tunnels/"+created.ID, token, updateBody)
			if resp.Code != http.StatusOK {
				t.Fatalf("Offline %s update expected 200, got %d, body=%s", tc.ingressType, resp.Code, resp.Body.String())
			}
			stored, err := s.store.GetTunnelByIDE(clientID, created.ID)
			if err != nil {
				t.Fatalf("Offline %s update should retain record in store: %v", tc.ingressType, err)
			}
			if stored.LocalIP != "192.168.1.50" || stored.LocalPort != 9090 || stored.RemotePort != tc.newPort {
				t.Fatalf("Offline %s update fields not written correctly, got %+v", tc.ingressType, stored)
			}
		})
	}
}

func TestOfflineManagedTunnel_UpdatePreservesBindIPForConflictCheck(t *testing.T) {
	s, handler, token, cleanup := setupTestServerWithStores(t, true)
	defer cleanup()
	clientID := registerOfflineHTTPTestClient(t, s, "offline-update-bind")
	remotePort := reserveTCPPort(t)
	createOne := doMuxRequest(t, handler, http.MethodPost, "/api/tunnels", token,
		unifiedOfflineCreatePayloadWithBindIP("loopback-one", clientID, "127.0.0.1", remotePort))
	if createOne.Code != http.StatusCreated {
		t.Fatalf("create loopback-one expected 201, got %d body=%s", createOne.Code, createOne.Body.String())
	}
	var one tunnelSpecAPI
	if err := mustDecodeJSON(t, createOne.Body, &one); err != nil {
		t.Fatalf("decode create one: %v", err)
	}
	// Seed the second tunnel directly to bypass preflight listen on 127.0.0.2
	// (not available on all platforms). The conflict check during update should
	// still differentiate by bind_ip.
	seedStoredTunnel(t, s, clientID, protocol.ProxyNewRequest{
		Name:       "loopback-two",
		Type:       protocol.ProxyTypeTCP,
		BindIP:     "127.0.0.2",
		LocalIP:    "127.0.0.1",
		LocalPort:  8081,
		RemotePort: remotePort,
	}, protocol.ProxyStatusActive)
	updateBody := []byte(`{"expected_revision":1,"spec":{"name":"loopback-one","topology":"server_expose","ingress":{"location":"server","type":"tcp_listen","config":{"bind_ip":"127.0.0.1","port":` + strconv.Itoa(remotePort) + `,"allowed_source_cidrs":["0.0.0.0/0","::/0"]}},"target":{"location":"client","client_id":"` + clientID + `","type":"tcp_service","config":{"ip":"192.168.1.50","port":9090}},"transport_policy":"server_relay_only"}}`)
	resp := doMuxRequest(t, handler, http.MethodPut, "/api/tunnels/"+one.ID, token, updateBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("offline update should preserve existing bind_ip during conflict checks, got %d body=%s", resp.Code, resp.Body.String())
	}
	stored, err := s.store.GetTunnelByIDE(clientID, one.ID)
	if err != nil {
		t.Fatalf("updated tunnel should remain in store: %v", err)
	}
	if stored.BindIP != "127.0.0.1" {
		t.Fatalf("update should preserve bind_ip, got %s", stored.BindIP)
	}
}

func TestOfflineManagedTunnel_Stop_StoreFirstForTCPAndUDP(t *testing.T) {
	testCases := []struct {
		name        string
		ingressType string
		remotePort  int
	}{
		{name: "tcp", ingressType: "tcp_listen", remotePort: reserveTCPPort(t)},
		{name: "udp", ingressType: "udp_listen", remotePort: reserveUDPPort(t)},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, handler, token, cleanup := setupTestServerWithStores(t, true)
			defer cleanup()
			clientID := registerOfflineHTTPTestClient(t, s, "offline-stop-"+tc.name)
			createResp := doMuxRequest(t, handler, http.MethodPost, "/api/tunnels", token,
				unifiedOfflineCreatePayload("offline-"+tc.name, clientID, tc.ingressType, tc.remotePort, ""))
			if createResp.Code != http.StatusCreated {
				t.Fatalf("create expected 201, got %d body=%s", createResp.Code, createResp.Body.String())
			}
			var created tunnelSpecAPI
			if err := mustDecodeJSON(t, createResp.Body, &created); err != nil {
				t.Fatalf("decode create response: %v", err)
			}
			resp := doMuxRequest(t, handler, http.MethodPut, "/api/tunnels/"+created.ID+"/stop", token, nil)
			if resp.Code != http.StatusOK {
				t.Fatalf("offline %s stop expected 200, got %d, body=%s", tc.ingressType, resp.Code, resp.Body.String())
			}
			stored, err := s.store.GetTunnelByIDE(clientID, created.ID)
			if err != nil {
				t.Fatalf("offline %s stop should retain record in store: %v", tc.ingressType, err)
			}
			if stored.DesiredState != protocol.ProxyDesiredStateStopped || stored.RuntimeState != protocol.ProxyRuntimeStateIdle {
				t.Fatalf("offline %s stop state error, got desired=%s runtime=%s", tc.ingressType, stored.DesiredState, stored.RuntimeState)
			}
		})
	}
}

func TestOfflineManagedTunnel_Resume_StoreFirstForTCPAndUDP(t *testing.T) {
	testCases := []struct {
		name        string
		ingressType string
		remotePort  int
	}{
		{name: "tcp", ingressType: "tcp_listen", remotePort: reserveTCPPort(t)},
		{name: "udp", ingressType: "udp_listen", remotePort: reserveUDPPort(t)},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, handler, token, cleanup := setupTestServerWithStores(t, true)
			defer cleanup()
			clientID := registerOfflineHTTPTestClient(t, s, "offline-resume-"+tc.name)
			createResp := doMuxRequest(t, handler, http.MethodPost, "/api/tunnels", token,
				unifiedOfflineCreatePayload("offline-"+tc.name, clientID, tc.ingressType, tc.remotePort, ""))
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
				t.Fatalf("Offline %s resume expected 200, got %d, body=%s", tc.ingressType, resp.Code, resp.Body.String())
			}
			stored, err := s.store.GetTunnelByIDE(clientID, created.ID)
			if err != nil {
				t.Fatalf("Offline %s resume should retain record in store: %v", tc.ingressType, err)
			}
			if stored.DesiredState != protocol.ProxyDesiredStateRunning || stored.RuntimeState != protocol.ProxyRuntimeStateOffline {
				t.Fatalf("Offline %s resume state error, got desired=%s runtime=%s", tc.ingressType, stored.DesiredState, stored.RuntimeState)
			}
		})
	}
}

func TestOfflineManagedTunnel_Delete_StoreFirstForTCPAndUDP(t *testing.T) {
	testCases := []struct {
		name        string
		ingressType string
		remotePort  int
	}{
		{name: "tcp", ingressType: "tcp_listen", remotePort: reserveTCPPort(t)},
		{name: "udp", ingressType: "udp_listen", remotePort: reserveUDPPort(t)},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, handler, token, cleanup := setupTestServerWithStores(t, true)
			defer cleanup()
			clientID := registerOfflineHTTPTestClient(t, s, "offline-delete-"+tc.name)
			createResp := doMuxRequest(t, handler, http.MethodPost, "/api/tunnels", token,
				unifiedOfflineCreatePayload("offline-"+tc.name, clientID, tc.ingressType, tc.remotePort, ""))
			if createResp.Code != http.StatusCreated {
				t.Fatalf("create expected 201, got %d body=%s", createResp.Code, createResp.Body.String())
			}
			var created tunnelSpecAPI
			if err := mustDecodeJSON(t, createResp.Body, &created); err != nil {
				t.Fatalf("decode create response: %v", err)
			}
			resp := doMuxRequest(t, handler, http.MethodDelete, "/api/tunnels/"+created.ID, token, nil)
			if resp.Code != http.StatusNoContent {
				t.Fatalf("Offline %s delete expected 204, got %d, body=%s", tc.ingressType, resp.Code, resp.Body.String())
			}
			if _, err := s.store.GetTunnelByIDE(clientID, created.ID); err == nil {
				t.Fatalf("Offline %s delete store record should be deleted", tc.ingressType)
			}
		})
	}
}
