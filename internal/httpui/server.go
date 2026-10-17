package httpui

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/cadmium228/GoToBedAlready/internal/domain"
	"github.com/cadmium228/GoToBedAlready/internal/energy"
)

type Monitor interface {
	Processes(context.Context) ([]domain.Process, error)
	Start(context.Context, domain.Process, domain.Preferences) error
	Cancel()
	Status() domain.Status
}

type Options struct {
	Assets        string
	Token         string
	Host          string
	Demo          bool
	Version       string
	Energy        interface{ Status() energy.Status }
	PowerSettings interface{ SetEnergy(energy.Options) error }
}

type server struct {
	monitor Monitor
	options Options
}

func New(monitor Monitor, options Options) http.Handler {
	s := &server{monitor: monitor, options: options}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("GET /api/processes", s.processes)
	mux.HandleFunc("POST /api/start", s.start)
	mux.HandleFunc("POST /api/cancel", s.cancel)
	mux.HandleFunc("POST /api/power", s.power)
	mux.HandleFunc("POST /api/diagnostics", s.diagnostics)
	for _, asset := range []struct{ path, file string }{
		{"/{$}", "index.html"}, {"/app.js", "app.js"}, {"/style.css", "style.css"},
	} {
		file := filepath.Join(options.Assets, "web", asset.file)
		mux.HandleFunc("GET "+asset.path, func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, file)
		})
	}
	return s.secure(mux)
}

func (s *server) secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		policy := "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'"
		if !s.options.Demo {
			policy += "; frame-ancestors 'none'"
			w.Header().Set("X-Frame-Options", "DENY")
		}
		w.Header().Set("Content-Security-Policy", policy)
		if !s.options.Demo {
			if r.Host != s.options.Host {
				writeError(w, http.StatusForbidden, "Недопустимый Host")
				return
			}
			origin := r.Header.Get("Origin")
			if origin != "" && origin != "http://"+s.options.Host {
				writeError(w, http.StatusForbidden, "Недопустимый Origin")
				return
			}
			if strings.HasPrefix(r.URL.Path, "/api/") && (s.options.Token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Session-Token")), []byte(s.options.Token)) != 1) {
				writeError(w, http.StatusUnauthorized, "Сессия истекла. Перезапустите приложение")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) status(w http.ResponseWriter, r *http.Request) {
	power := energy.Status{}
	if s.options.Energy != nil {
		power = s.options.Energy.Status()
	}
	writeJSON(w, http.StatusOK, struct {
		Power   energy.Status `json:"power"`
		Status  domain.Status `json:"status"`
		Demo    bool          `json:"demo"`
		Version string        `json:"version"`
	}{power, s.monitor.Status(), s.options.Demo, s.options.Version})
}

func (s *server) processes(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	processes, err := s.monitor.Processes(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, processes)
}

func (s *server) start(w http.ResponseWriter, r *http.Request) {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "Ожидается application/json")
		return
	}
	var request struct {
		Process     domain.Process     `json:"process"`
		Preferences domain.Preferences `json:"preferences"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "Некорректный запрос")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "Ожидается один JSON-объект")
		return
	}
	request.Preferences.DryRun = s.options.Demo
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	if err := s.monitor.Start(ctx, request.Process, request.Preferences); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.monitor.Status())
}

func (s *server) cancel(w http.ResponseWriter, r *http.Request) {
	s.monitor.Cancel()
	writeJSON(w, http.StatusOK, s.monitor.Status())
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func (s *server) power(w http.ResponseWriter, r *http.Request) {
	if s.options.PowerSettings == nil {
		writeError(w, 503, "Параметры питания недоступны")
		return
	}
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		writeError(w, 415, "Ожидается application/json")
		return
	}
	var options energy.Options
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&options); err != nil {
		writeError(w, 400, "Некорректные параметры питания")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, 400, "Ожидается один JSON-объект")
		return
	}
	if err := s.options.PowerSettings.SetEnergy(options); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, options)
}

func (s *server) diagnostics(w http.ResponseWriter, r *http.Request) {
	inspector, ok := s.monitor.(interface {
		Inspect(context.Context, domain.Process) (json.RawMessage, error)
	})
	if !ok || s.options.Demo {
		writeError(w, 503, "Диагностика доступна только на Windows")
		return
	}
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		writeError(w, 415, "Ожидается application/json")
		return
	}
	var process domain.Process
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&process); err != nil || process.PID <= 0 || process.StartedAt == "" {
		writeError(w, 400, "Выберите процесс")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, 400, "Ожидается один JSON-объект")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	result, err := inspector.Inspect(ctx, process)
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	writeJSON(w, 200, result)
}
