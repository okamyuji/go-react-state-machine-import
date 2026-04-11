// Package statemachineはテーブル駆動の汎用有限状態機械（FSM）を提供します。
// 記事のサンプルを他プロジェクトに移植しやすいよう、依存を増やさず標準ライブラリ
// のみで実装しています。
package statemachine

import (
	"errors"
	"fmt"
)

// State状態機械における単一の状態を表します。
type State string

// Event状態遷移を引き起こすトリガーを表します。
type Event string

// Transition遷移テーブル上の1本のエッジ (from, event) → toを表します。
type Transition struct {
	From  State
	Event Event
	To    State
}

// ErrInvalidTransition現在の状態がイベントを受け付けない場合に返されます。
var ErrInvalidTransition = errors.New("無効な遷移です")

// Machine不変な遷移テーブルです。生成後は読み取り専用なので並行アクセスに安全です。
type Machine struct {
	initial State
	table   map[State]map[Event]State
}

// New初期状態と遷移一覧から状態機械を構築します。
// 同じ (from, event) が2回指定されているなどの設定ミスは起動時に検知できるよう
// エラーとして返します。
func New(initial State, transitions []Transition) (*Machine, error) {
	table := make(map[State]map[Event]State)
	for _, t := range transitions {
		if _, ok := table[t.From]; !ok {
			table[t.From] = make(map[Event]State)
		}
		if _, exists := table[t.From][t.Event]; exists {
			return nil, fmt.Errorf("遷移が重複しています state=%q event=%q", t.From, t.Event)
		}
		table[t.From][t.Event] = t.To
	}
	return &Machine{initial: initial, table: table}, nil
}

// Initial設定された初期状態を返します。
func (m *Machine) Initial() State { return m.initial }

// Next (from, event) に対応する遷移先の状態を返します。該当がなければ
// ErrInvalidTransitionをラップしたエラーを返します。
func (m *Machine) Next(from State, event Event) (State, error) {
	if events, ok := m.table[from]; ok {
		if to, ok := events[event]; ok {
			return to, nil
		}
	}
	return "", fmt.Errorf("%w: state=%q event=%q", ErrInvalidTransition, from, event)
}

// AllowedEvents現在の状態で受け付け可能なイベント一覧を返します。
// テーブルから派生させているため、実行時の挙動と乖離することはありません。
func (m *Machine) AllowedEvents(from State) []Event {
	events := m.table[from]
	out := make([]Event, 0, len(events))
	for e := range events {
		out = append(out, e)
	}
	return out
}
