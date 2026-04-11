package importjob_test

import (
	"errors"
	"testing"
	"time"

	"github.com/okamyuji/go-react-state-machine-import/backend/internal/importjob"
	"github.com/okamyuji/go-react-state-machine-import/backend/internal/statemachine"
)

// fixedClock呼び出しごとに1秒進む決定論的なクロックです。
type fixedClock struct{ t time.Time }

func (f *fixedClock) Now() time.Time { f.t = f.t.Add(time.Second); return f.t }

// staticIDsあらかじめ用意したIDを順番に返すテスト用IDSourceです。
type staticIDs struct {
	ids []string
	i   int
}

func (s *staticIDs) NextID() string {
	id := s.ids[s.i]
	s.i++
	return id
}

func newTestService(t *testing.T) importjob.Service {
	t.Helper()
	repo := importjob.NewInMemoryRepository()
	return importjob.NewService(
		repo,
		&fixedClock{t: time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)},
		&staticIDs{ids: []string{"job-1", "job-2", "job-3"}},
	)
}

func TestServiceCreate(t *testing.T) {
	t.Parallel()
	t.Run("正常系: draft 状態で View は none になる", func(t *testing.T) {
		svc := newTestService(t)
		job, err := svc.Create("users.csv", 10)
		if err != nil {
			t.Fatalf("想定外のエラー: %v", err)
		}
		if job.State != importjob.StateDraft {
			t.Errorf("draft を期待したが %q", job.State)
		}
		if job.View != importjob.ViewNone {
			t.Errorf("none を期待したが %q", job.View)
		}
		if job.ID != "job-1" {
			t.Errorf("job-1 を期待したが %q", job.ID)
		}
	})

	t.Run("filename が空ならエラー", func(t *testing.T) {
		svc := newTestService(t)
		if _, err := svc.Create("", 10); err == nil {
			t.Fatal("空文字でエラーが出るはず")
		}
	})

	t.Run("rows が0以下ならエラー", func(t *testing.T) {
		svc := newTestService(t)
		if _, err := svc.Create("users.csv", 0); err == nil {
			t.Fatal("0 行でエラーが出るはず")
		}
	})
}

// TestServiceApplyHappyPath draftからcompletedまでの代表的な1本道を検証します。
// 途中でViewがmodal → overlayと切り替わる瞬間が記事の肝要な主張です。
func TestServiceApplyHappyPath(t *testing.T) {
	t.Parallel()
	svc := newTestService(t)
	job, err := svc.Create("users.csv", 100)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	steps := []struct {
		event    statemachine.Event
		want     statemachine.State
		wantView importjob.View
	}{
		{importjob.EventUpload, importjob.StateUploading, importjob.ViewOverlay},
		{importjob.EventUploadSucceed, importjob.StateParsing, importjob.ViewOverlay},
		{importjob.EventParseSucceed, importjob.StateValidating, importjob.ViewOverlay},
		{importjob.EventValidateSucceed, importjob.StateReady, importjob.ViewModal},
		{importjob.EventStart, importjob.StateImporting, importjob.ViewOverlay},
		{importjob.EventPause, importjob.StatePaused, importjob.ViewOverlay},
		{importjob.EventResume, importjob.StateImporting, importjob.ViewOverlay},
		{importjob.EventComplete, importjob.StateCompleted, importjob.ViewResult},
		{importjob.EventReset, importjob.StateDraft, importjob.ViewNone},
	}
	for _, step := range steps {
		job, err = svc.Apply(job.ID, step.event, -1, "")
		if err != nil {
			t.Fatalf("Apply %s: %v", step.event, err)
		}
		if job.State != step.want {
			t.Errorf("%s 後に %q を期待したが %q", step.event, step.want, job.State)
		}
		if job.View != step.wantView {
			t.Errorf("%s 後のビューは %q を期待したが %q", step.event, step.wantView, job.View)
		}
	}
	if n := len(job.Transitions); n != len(steps) {
		t.Errorf("遷移履歴は %d 件を期待したが %d 件", len(steps), n)
	}
}

// TestServiceApplyErrorRecoveryPaths各エラー状態からの回復/リセットを網羅的に確認します。
func TestServiceApplyErrorRecoveryPaths(t *testing.T) {
	t.Parallel()
	t.Run("アップロード失敗 → リトライで uploading に戻る", func(t *testing.T) {
		svc := newTestService(t)
		j, _ := svc.Create("x.csv", 10)
		j, _ = svc.Apply(j.ID, importjob.EventUpload, -1, "")
		j, _ = svc.Apply(j.ID, importjob.EventUploadFail, -1, "ネットワーク切断")
		if j.State != importjob.StateUploadFailed || j.View != importjob.ViewError {
			t.Fatalf("upload_failed/error を期待: %q/%q", j.State, j.View)
		}
		j, _ = svc.Apply(j.ID, importjob.EventRetry, -1, "")
		if j.State != importjob.StateUploading {
			t.Errorf("uploading を期待したが %q", j.State)
		}
	})

	t.Run("パース失敗 → reset で draft に戻る", func(t *testing.T) {
		svc := newTestService(t)
		j, _ := svc.Create("x.csv", 10)
		j, _ = svc.Apply(j.ID, importjob.EventUpload, -1, "")
		j, _ = svc.Apply(j.ID, importjob.EventUploadSucceed, -1, "")
		j, _ = svc.Apply(j.ID, importjob.EventParseFail, -1, "不正な CSV")
		if j.State != importjob.StateParseFailed {
			t.Fatalf("parse_failed を期待: %q", j.State)
		}
		j, _ = svc.Apply(j.ID, importjob.EventReset, -1, "")
		if j.State != importjob.StateDraft {
			t.Errorf("draft を期待したが %q", j.State)
		}
	})

	t.Run("バリデーション失敗 → reset で draft に戻る", func(t *testing.T) {
		svc := newTestService(t)
		j, _ := svc.Create("x.csv", 10)
		j, _ = svc.Apply(j.ID, importjob.EventUpload, -1, "")
		j, _ = svc.Apply(j.ID, importjob.EventUploadSucceed, -1, "")
		j, _ = svc.Apply(j.ID, importjob.EventParseSucceed, -1, "")
		j, _ = svc.Apply(j.ID, importjob.EventValidateFail, -1, "必須列不足")
		if j.State != importjob.StateValidationFailed {
			t.Fatalf("validation_failed を期待: %q", j.State)
		}
		j, _ = svc.Apply(j.ID, importjob.EventReset, -1, "")
		if j.State != importjob.StateDraft {
			t.Errorf("draft を期待したが %q", j.State)
		}
	})

	t.Run("インポート中にキャンセルで cancelled になる", func(t *testing.T) {
		svc := newTestService(t)
		j := advanceTo(t, svc, importjob.StateImporting)
		j, err := svc.Apply(j.ID, importjob.EventCancel, -1, "")
		if err != nil {
			t.Fatalf("cancel: %v", err)
		}
		if j.State != importjob.StateCancelled {
			t.Errorf("cancelled を期待したが %q", j.State)
		}
	})
}

// advanceTo指定の状態までジョブを進めます。テストの共通前処理です。
func advanceTo(t *testing.T, svc importjob.Service, target statemachine.State) *importjob.Job {
	t.Helper()
	j, _ := svc.Create("x.csv", 10)
	sequence := []statemachine.Event{
		importjob.EventUpload,
		importjob.EventUploadSucceed,
		importjob.EventParseSucceed,
		importjob.EventValidateSucceed,
		importjob.EventStart,
	}
	for _, ev := range sequence {
		j, _ = svc.Apply(j.ID, ev, -1, "")
		if j.State == target {
			return j
		}
	}
	t.Fatalf("目的状態 %q に到達できませんでした (現状 %q)", target, j.State)
	return nil
}

func TestServiceApplyRejectsInvalidTransition(t *testing.T) {
	t.Parallel()
	svc := newTestService(t)
	job, _ := svc.Create("x.csv", 1)
	// draftからいきなりstartはできない
	if _, err := svc.Apply(job.ID, importjob.EventStart, -1, ""); !errors.Is(err, statemachine.ErrInvalidTransition) {
		t.Errorf("ErrInvalidTransition を期待したが %v", err)
	}
}

func TestServiceApplyUpdatesProcessedAndMessage(t *testing.T) {
	t.Parallel()
	svc := newTestService(t)
	j := advanceTo(t, svc, importjob.StateImporting)
	updated, err := svc.Apply(j.ID, importjob.EventComplete, 100, "完了")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if updated.Processed != 100 {
		t.Errorf("processed=100 を期待したが %d", updated.Processed)
	}
	if updated.Message != "完了" {
		t.Errorf("message='完了' を期待したが %q", updated.Message)
	}
}

func TestServiceGetReturnsNotFound(t *testing.T) {
	t.Parallel()
	svc := newTestService(t)
	if _, err := svc.Get("missing"); !errors.Is(err, importjob.ErrNotFound) {
		t.Errorf("ErrNotFound を期待したが %v", err)
	}
}

func TestServiceListNewestFirst(t *testing.T) {
	t.Parallel()
	svc := newTestService(t)
	_, _ = svc.Create("a.csv", 1)
	_, _ = svc.Create("b.csv", 1)
	_, _ = svc.Create("c.csv", 1)
	jobs := svc.List()
	if len(jobs) != 3 {
		t.Fatalf("3件を期待したが %d 件", len(jobs))
	}
	if jobs[0].Filename != "c.csv" || jobs[2].Filename != "a.csv" {
		t.Errorf("並び順が想定と異なります: %v %v %v", jobs[0].Filename, jobs[1].Filename, jobs[2].Filename)
	}
}

// TestViewForIsExclusive状態とビューが必ず1対1対応であることを、
// 全ての状態について確認します。モーダルとオーバーレイが「同時表示される」
// 誤解を構造的に防ぐ、記事の肝要な主張の裏付けです。
func TestViewForIsExclusive(t *testing.T) {
	t.Parallel()
	cases := map[statemachine.State]importjob.View{
		importjob.StateDraft:            importjob.ViewNone,
		importjob.StateUploading:        importjob.ViewOverlay,
		importjob.StateUploadFailed:     importjob.ViewError,
		importjob.StateParsing:          importjob.ViewOverlay,
		importjob.StateParseFailed:      importjob.ViewError,
		importjob.StateValidating:       importjob.ViewOverlay,
		importjob.StateValidationFailed: importjob.ViewError,
		importjob.StateReady:            importjob.ViewModal,
		importjob.StateImporting:        importjob.ViewOverlay,
		importjob.StatePaused:           importjob.ViewOverlay,
		importjob.StateCompleted:        importjob.ViewResult,
		importjob.StateFailed:           importjob.ViewResult,
		importjob.StateCancelled:        importjob.ViewResult,
	}
	for state, want := range cases {
		if got := importjob.ViewFor(state); got != want {
			t.Errorf("ViewFor(%q) = %q, 期待 %q", state, got, want)
		}
	}
}
