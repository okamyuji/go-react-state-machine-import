// Package httpapiはImportJobサービスをHTTP経由で公開します。
// ハンドラはimportjob.Service interfaceにのみ依存しているため、インメモリの
// fakeに差し替えた単体テストが容易に書けます。
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/okamyuji/go-react-state-machine-import/backend/internal/importjob"
	"github.com/okamyuji/go-react-state-machine-import/backend/internal/statemachine"
)

// Handler HTTPルートをImportJobサービスにつなぎ込みます。
type Handler struct {
	svc    importjob.Service
	logger *slog.Logger
}

// NewHandler指定したサービスとロガーでハンドラを生成します。
func NewHandler(svc importjob.Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{svc: svc, logger: logger}
}

// RoutesすべてのAPIルートを登録したhttp.Handlerを返します。
// Go 1.22+ のServeMuxパターンマッチングを使えば、CRUD程度のAPIなら
// サードパーティのルーターは不要です。
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/import_jobs", h.list)
	mux.HandleFunc("POST /api/import_jobs", h.create)
	mux.HandleFunc("GET /api/import_jobs/{id}", h.get)
	mux.HandleFunc("POST /api/import_jobs/{id}/events", h.applyEvent)
	mux.HandleFunc("GET /api/healthz", h.healthz)
	return withCORS(mux)
}

type createRequest struct {
	Filename string `json:"filename"`
	Rows     int    `json:"rows"`
}

type eventRequest struct {
	Event     string `json:"event"`
	Processed int    `json:"processed"`
	Message   string `json:"message"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) list(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.List())
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Filename = strings.TrimSpace(req.Filename)
	job, err := h.svc.Create(req.Filename, req.Rows)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := h.svc.Get(id)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *Handler) applyEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req eventRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Event == "" {
		writeError(w, http.StatusBadRequest, errors.New("event は必須です"))
		return
	}
	// 0は「未指定」として扱い、正の値のみ進捗を更新します。
	processed := -1
	if req.Processed > 0 {
		processed = req.Processed
	}
	job, err := h.svc.Apply(id, statemachine.Event(req.Event), processed, req.Message)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// writeDomainError Serviceから返されたエラーをHTTPステータスに変換します。
func (h *Handler) writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, importjob.ErrNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, statemachine.ErrInvalidTransition):
		writeError(w, http.StatusConflict, err)
	default:
		h.logger.Error("想定外のエラー", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, err)
	}
}

func decodeJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return errors.New("リクエストボディが空です")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, errorResponse{Error: err.Error()})
}

// withCORS Vite devサーバからのAPI呼び出しを許可する最小のミドルウェアです。
// 本番では同一オリジンでフロントを配信するので、このシンプルな実装で十分です。
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
