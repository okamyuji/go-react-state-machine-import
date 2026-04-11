package statemachine_test

import (
	"errors"
	"sort"
	"testing"

	"github.com/okamyuji/go-react-state-machine-import/backend/internal/statemachine"
)

// buildTestMachine各テストで使うシンプルな状態機械を組み立てます。
func buildTestMachine(t *testing.T) *statemachine.Machine {
	t.Helper()
	m, err := statemachine.New("idle", []statemachine.Transition{
		{From: "idle", Event: "start", To: "running"},
		{From: "running", Event: "finish", To: "done"},
		{From: "running", Event: "fail", To: "failed"},
	})
	if err != nil {
		t.Fatalf("想定外のエラー: %v", err)
	}
	return m
}

func TestNext(t *testing.T) {
	t.Parallel()
	t.Run("正しい遷移は遷移先の状態を返す", func(t *testing.T) {
		m := buildTestMachine(t)
		got, err := m.Next("idle", "start")
		if err != nil {
			t.Fatalf("想定外のエラー: %v", err)
		}
		if got != "running" {
			t.Errorf("running を期待したが %q が返った", got)
		}
	})

	t.Run("未定義の遷移では ErrInvalidTransition を返す", func(t *testing.T) {
		m := buildTestMachine(t)
		_, err := m.Next("done", "start")
		if !errors.Is(err, statemachine.ErrInvalidTransition) {
			t.Errorf("ErrInvalidTransition を期待したが %v が返った", err)
		}
	})
}

func TestNewRejectsDuplicateTransitions(t *testing.T) {
	t.Parallel()
	_, err := statemachine.New("a", []statemachine.Transition{
		{From: "a", Event: "x", To: "b"},
		{From: "a", Event: "x", To: "c"},
	})
	if err == nil {
		t.Fatal("重複遷移でエラーになるはず")
	}
}

func TestAllowedEvents(t *testing.T) {
	t.Parallel()
	t.Run("到達可能なイベントだけを列挙する", func(t *testing.T) {
		m := buildTestMachine(t)
		got := m.AllowedEvents("running")
		sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
		if len(got) != 2 || got[0] != "fail" || got[1] != "finish" {
			t.Errorf("想定外のイベント一覧: %v", got)
		}
	})

	t.Run("終端状態では空リストを返す", func(t *testing.T) {
		m := buildTestMachine(t)
		if ev := m.AllowedEvents("done"); len(ev) != 0 {
			t.Errorf("終端状態ではイベント無しのはずが %v", ev)
		}
	})
}

func TestInitialReturnsConfiguredState(t *testing.T) {
	t.Parallel()
	m := buildTestMachine(t)
	if got := m.Initial(); got != "idle" {
		t.Errorf("idle を期待したが %q が返った", got)
	}
}
