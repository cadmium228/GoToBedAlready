package httpui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cadmium228/GoToBedAlready/internal/domain"
	"github.com/cadmium228/GoToBedAlready/internal/energy"
)

type fakeMonitor struct {
	starts      int
	preferences domain.Preferences
}

func (m *fakeMonitor) Processes(context.Context) ([]domain.Process, error) {
	return []domain.Process{}, nil
}
func (m *fakeMonitor) Start(ctx context.Context, process domain.Process, preferences domain.Preferences) error {
	m.starts++
	m.preferences = preferences
	return nil
}
func (m *fakeMonitor) Cancel()               {}
func (m *fakeMonitor) Status() domain.Status { return domain.Status{Phase: "idle"} }

func TestSessionSecurity(t *testing.T) {
	for _, test := range []struct {
		name, host, origin, token string
		want                      int
	}{
		{"valid", "127.0.0.1:9000", "http://127.0.0.1:9000", "secret", 200},
		{"missing token", "127.0.0.1:9000", "", "", 401},
		{"wrong token", "127.0.0.1:9000", "", "wrong", 401},
		{"cross origin", "127.0.0.1:9000", "https://attacker.test", "secret", 403},
		{"dns rebinding", "attacker.test:9000", "", "secret", 403},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := New(&fakeMonitor{}, Options{Host: "127.0.0.1:9000", Token: "secret"})
			request := httptest.NewRequest("GET", "http://"+test.host+"/api/status", nil)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("X-Session-Token", test.token)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("got %d want %d", response.Code, test.want)
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing no-store")
			}
		})
	}
}

func TestDemoForcesDryRun(t *testing.T) {
	monitor := &fakeMonitor{}
	handler := New(monitor, Options{Demo: true})
	request := httptest.NewRequest("POST", "/api/start", strings.NewReader(`{"process":{"pid":1},"preferences":{"delayMinutes":5,"dryRun":false}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || monitor.starts != 1 || !monitor.preferences.DryRun {
		t.Fatal("demo can request real shutdown")
	}
}

func TestMalformedRequestsRejected(t *testing.T) {
	for _, body := range []string{`{`, `{"extra":true}`, `{} {}`, strings.Repeat(" ", 4097) + `{}`} {
		monitor := &fakeMonitor{}
		request := httptest.NewRequest("POST", "/api/start", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		New(monitor, Options{Demo: true}).ServeHTTP(response, request)
		if response.Code != 400 || monitor.starts != 0 {
			t.Fatalf("accepted malformed request: %d", response.Code)
		}
	}
}

func TestStateChangesRequirePostAndJSON(t *testing.T) {
	handler := New(&fakeMonitor{}, Options{Demo: true})
	for _, test := range []struct {
		method, path string
		code         int
	}{{"GET", "/api/cancel", 405}, {"GET", "/api/start", 405}, {"POST", "/api/start", 415}} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.code {
			t.Fatalf("%s: got %d want %d", test.path, response.Code, test.code)
		}
	}
}

type fakePowerSettings struct {
	options energy.Options
	calls   int
}

func (p *fakePowerSettings) SetEnergy(options energy.Options) error {
	p.options = options
	p.calls++
	return nil
}

func TestPowerEndpointRequiresSessionAndStrictJSON(t *testing.T) {
	settings := &fakePowerSettings{}
	handler := New(&fakeMonitor{}, Options{Host: "127.0.0.1:9000", Token: "secret", PowerSettings: settings})
	request := httptest.NewRequest("POST", "http://127.0.0.1:9000/api/power", strings.NewReader(`{"blankDisplay":true,"preventSleep":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 401 || settings.calls != 0 {
		t.Fatal("unauthorized power change")
	}
	for _, body := range []string{`{"blankDisplay":true,"preventSleep":true}`, `{"unknown":true}`, `{} {}`} {
		request = httptest.NewRequest("POST", "http://127.0.0.1:9000/api/power", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Session-Token", "secret")
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if strings.Contains(body, "blankDisplay") && response.Code != 200 {
			t.Fatal(response.Body.String())
		}
		if !strings.Contains(body, "blankDisplay") && response.Code != 400 {
			t.Fatal("accepted malformed power settings")
		}
	}
	if settings.calls != 1 || !settings.options.BlankDisplay || !settings.options.PreventSleep {
		t.Fatal(settings)
	}
}

func TestRealUIHasNoPersistedDryRunMode(t *testing.T) {
	monitor := &fakeMonitor{}
	request := httptest.NewRequest("POST", "http://127.0.0.1:9000/api/start", strings.NewReader(`{"process":{"pid":1},"preferences":{"delayMinutes":5,"dryRun":true}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Session-Token", "secret")
	response := httptest.NewRecorder()
	New(monitor, Options{Host: "127.0.0.1:9000", Token: "secret"}).ServeHTTP(response, request)
	if response.Code != 200 || monitor.preferences.DryRun {
		t.Fatal("hidden dry-run mode still controls real UI")
	}
}
