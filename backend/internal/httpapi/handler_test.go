package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/okamyuji/go-react-state-machine-import/backend/internal/httpapi"
	"github.com/okamyuji/go-react-state-machine-import/backend/internal/importjob"
)

type fixedClock struct{ t time.Time }

func (f *fixedClock) Now() time.Time { f.t = f.t.Add(time.Second); return f.t }

// newHandlerテスト用のハンドラを組み立てます。ログは捨てて静かなテストにします。
func newHandler(t *testing.T) http.Handler {
	t.Helper()
	repo := importjob.NewInMemoryRepository()
	svc := importjob.NewService(
		repo,
		&fixedClock{t: time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)},
		importjob.NewSequentialID("job"),
	)
	return httpapi.NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil))).Routes()
}

// doテスト用の簡易リクエスト実行ヘルパーです。
func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthzReturns200(t *testing.T) {
	t.Parallel()
	h := newHandler(t)
	rec := do(t, h, http.MethodGet, "/api/healthz", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("200 を期待したが %d", rec.Code)
	}
}

func TestCreateJob(t *testing.T) {
	t.Parallel()
	t.Run("正常系: 201 とジョブ情報が返る", func(t *testing.T) {
		h := newHandler(t)
		rec := do(t, h, http.MethodPost, "/api/import_jobs", map[string]any{
			"filename": "users.csv",
			"rows":     10,
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("201 を期待したが %d body=%s", rec.Code, rec.Body.String())
		}
		var job importjob.Job
		if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if job.Filename != "users.csv" {
			t.Errorf("users.csv を期待したが %q", job.Filename)
		}
		if job.State != importjob.StateDraft {
			t.Errorf("draft を期待したが %q", job.State)
		}
	})

	t.Run("バリデーションエラーは 422 を返す", func(t *testing.T) {
		h := newHandler(t)
		rec := do(t, h, http.MethodPost, "/api/import_jobs", map[string]any{
			"filename": "",
			"rows":     10,
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("422 を期待したが %d", rec.Code)
		}
	})
}

func TestApplyEvent(t *testing.T) {
	t.Parallel()
	t.Run("正常系: upload イベントで uploading + overlay になる", func(t *testing.T) {
		h := newHandler(t)
		rec := do(t, h, http.MethodPost, "/api/import_jobs", map[string]any{
			"filename": "users.csv", "rows": 3,
		})
		var created importjob.Job
		_ = json.Unmarshal(rec.Body.Bytes(), &created)

		rec = do(t, h, http.MethodPost, "/api/import_jobs/"+created.ID+"/events", map[string]any{
			"event": "upload",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("200 を期待したが %d body=%s", rec.Code, rec.Body.String())
		}
		var updated importjob.Job
		_ = json.Unmarshal(rec.Body.Bytes(), &updated)
		if updated.State != importjob.StateUploading {
			t.Errorf("uploading を期待したが %q", updated.State)
		}
		if updated.View != importjob.ViewOverlay {
			t.Errorf("overlay を期待したが %q", updated.View)
		}
	})

	t.Run("無効な遷移は 409 を返す", func(t *testing.T) {
		h := newHandler(t)
		rec := do(t, h, http.MethodPost, "/api/import_jobs", map[string]any{
			"filename": "users.csv", "rows": 3,
		})
		var created importjob.Job
		_ = json.Unmarshal(rec.Body.Bytes(), &created)

		rec = do(t, h, http.MethodPost, "/api/import_jobs/"+created.ID+"/events", map[string]any{
			"event": "start", // draft からはいきなり start できない
		})
		if rec.Code != http.StatusConflict {
			t.Errorf("409 を期待したが %d", rec.Code)
		}
	})
}

func TestGetUnknownIDReturns404(t *testing.T) {
	t.Parallel()
	h := newHandler(t)
	rec := do(t, h, http.MethodGet, "/api/import_jobs/missing", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("404 を期待したが %d", rec.Code)
	}
}

func TestListReturnsArray(t *testing.T) {
	t.Parallel()
	h := newHandler(t)
	_ = do(t, h, http.MethodPost, "/api/import_jobs", map[string]any{"filename": "a.csv", "rows": 1})
	_ = do(t, h, http.MethodPost, "/api/import_jobs", map[string]any{"filename": "b.csv", "rows": 1})
	rec := do(t, h, http.MethodGet, "/api/import_jobs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("200 を期待したが %d", rec.Code)
	}
	var jobs []importjob.Job
	if err := json.Unmarshal(rec.Body.Bytes(), &jobs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("2件を期待したが %d 件", len(jobs))
	}
}

func TestCORSPreflightReturns204(t *testing.T) {
	t.Parallel()
	h := newHandler(t)
	rec := do(t, h, http.MethodOptions, "/api/import_jobs", nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("204 を期待したが %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("CORS ヘッダが設定されていません")
	}
}
