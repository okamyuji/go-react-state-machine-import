// Package importjobはImportJobドメインモデルとその状態機械設定を定義します。
//
// この題材はあえて現場で起きがちな複雑さを持たせています。
// CSVインポートは「アップロード → パース → バリデーション → インポート」
// という4フェーズに分けて考えがちで、設計に慣れていない人はフェーズごとに
// 独立した状態遷移表（マトリクス）を作り、それらをbooleanフラグ
// (uploaded/parsed/validated/imported) で繋ごうとします。これはフラグ地獄の
// 典型パターンで、フェーズ境界の整合性を人手で守る羽目になります。
//
// 本実装ではすべてのフェーズを1つの遷移テーブルに統合することで、
// 状態同士の整合性を型レベルで保証します。
package importjob

import (
	"fmt"
	"time"

	"github.com/okamyuji/go-react-state-machine-import/backend/internal/statemachine"
)

// 状態の定数定義。全13状態で「ジョブが今どのフェーズのどの状況にあるか」を
// ただ1つの値で表現します。複数のbooleanを組み合わせる必要はありません。
const (
	StateDraft            statemachine.State = "draft"             // 入口: ファイル未選択
	StateUploading        statemachine.State = "uploading"         // アップロード中
	StateUploadFailed     statemachine.State = "upload_failed"     // アップロード失敗
	StateParsing          statemachine.State = "parsing"           // CSV パース中
	StateParseFailed      statemachine.State = "parse_failed"      // パース失敗
	StateValidating       statemachine.State = "validating"        // 入力値バリデーション中
	StateValidationFailed statemachine.State = "validation_failed" // バリデーション失敗
	StateReady            statemachine.State = "ready"             // インポート実行待ち（登録完了モーダルを出す状態）
	StateImporting        statemachine.State = "importing"         // インポート実行中（オーバーレイを出す状態）
	StatePaused           statemachine.State = "paused"            // 一時停止
	StateCompleted        statemachine.State = "completed"         // 完了
	StateFailed           statemachine.State = "failed"            // 失敗
	StateCancelled        statemachine.State = "cancelled"         // キャンセル
)

// イベントの定数定義。全15種類。
const (
	EventUpload          statemachine.Event = "upload"           // アップロード開始
	EventUploadSucceed   statemachine.Event = "upload_succeed"   // アップロード成功
	EventUploadFail      statemachine.Event = "upload_fail"      // アップロード失敗
	EventParseSucceed    statemachine.Event = "parse_succeed"    // パース成功
	EventParseFail       statemachine.Event = "parse_fail"       // パース失敗
	EventValidateSucceed statemachine.Event = "validate_succeed" // バリデーション成功
	EventValidateFail    statemachine.Event = "validate_fail"    // バリデーション失敗
	EventRetry           statemachine.Event = "retry"            // アップロード失敗からの再試行
	EventStart           statemachine.Event = "start"            // インポート開始
	EventPause           statemachine.Event = "pause"            // 一時停止
	EventResume          statemachine.Event = "resume"           // 再開
	EventCancel          statemachine.Event = "cancel"           // キャンセル
	EventComplete        statemachine.Event = "complete"         // 完了
	EventFail            statemachine.Event = "fail"             // 実行時エラー
	EventReset           statemachine.Event = "reset"            // 終端状態から初期状態へ戻す
)

// View状態に対応して表示すべきUIサーフェスを表します。
// 状態とビューは完全に1対1に対応しており、複数サーフェスが同時に表示されることは
// 構造上ありえません。これがフラグ地獄を置き換える最大の利点です。
type View string

// View値。
const (
	ViewNone    View = "none"    // 何も重ねない（draft）
	ViewModal   View = "modal"   // 登録完了モーダル（ready）
	ViewOverlay View = "overlay" // 進捗オーバーレイ（uploading/parsing/validating/importing/paused）
	ViewResult  View = "result"  // 結果表示（completed/failed/cancelled）
	ViewError   View = "error"   // インラインエラー（upload_failed/parse_failed/validation_failed）
)

// ViewFor状態からUIサーフェスを導出する純粋関数です。
// フロントエンド側も同じ対応表を参照するだけで済みます。
func ViewFor(s statemachine.State) View {
	switch s {
	case StateReady:
		return ViewModal
	case StateUploading, StateParsing, StateValidating, StateImporting, StatePaused:
		return ViewOverlay
	case StateCompleted, StateFailed, StateCancelled:
		return ViewResult
	case StateUploadFailed, StateParseFailed, StateValidationFailed:
		return ViewError
	default:
		return ViewNone
	}
}

// transitions ImportJobの全遷移テーブルです。
//
// 素人設計でありがちな失敗は、フェーズごとに別マトリクスを作ってしまうことです:
//
//	アップロードマトリクス : draft → uploading → uploaded/upload_failed
//	パースマトリクス       : uploaded → parsing → parsed/parse_failed
//	バリデーションマトリクス: parsed → validating → validated/validation_failed
//	インポートマトリクス   : validated → importing → completed/failed/cancelled
//
// そして各マトリクスの境界をuploaded/parsed/validated/importedといったboolean
// で繋ごうとし、結果的にフラグ地獄に戻ります。
//
// このスライスは4つのフェーズを1つに統合し、23本のエッジをひとまとめにしています。
// 状態遷移表は1つしか存在しないことが型レベルで保証されます。
var transitions = []statemachine.Transition{
	// --- アップロードフェーズ ---
	{From: StateDraft, Event: EventUpload, To: StateUploading},
	{From: StateUploading, Event: EventUploadSucceed, To: StateParsing},
	{From: StateUploading, Event: EventUploadFail, To: StateUploadFailed},
	{From: StateUploadFailed, Event: EventRetry, To: StateUploading},
	{From: StateUploadFailed, Event: EventReset, To: StateDraft},

	// --- パースフェーズ ---
	{From: StateParsing, Event: EventParseSucceed, To: StateValidating},
	{From: StateParsing, Event: EventParseFail, To: StateParseFailed},
	{From: StateParseFailed, Event: EventReset, To: StateDraft},

	// --- バリデーションフェーズ ---
	{From: StateValidating, Event: EventValidateSucceed, To: StateReady},
	{From: StateValidating, Event: EventValidateFail, To: StateValidationFailed},
	{From: StateValidationFailed, Event: EventReset, To: StateDraft},

	// --- インポートフェーズ ---
	{From: StateReady, Event: EventStart, To: StateImporting},
	{From: StateReady, Event: EventCancel, To: StateCancelled},
	{From: StateImporting, Event: EventPause, To: StatePaused},
	{From: StatePaused, Event: EventResume, To: StateImporting},
	{From: StateImporting, Event: EventCancel, To: StateCancelled},
	{From: StatePaused, Event: EventCancel, To: StateCancelled},
	{From: StateImporting, Event: EventComplete, To: StateCompleted},
	{From: StateImporting, Event: EventFail, To: StateFailed},

	// --- 終端からのリセット ---
	{From: StateCompleted, Event: EventReset, To: StateDraft},
	{From: StateFailed, Event: EventReset, To: StateDraft},
	{From: StateCancelled, Event: EventReset, To: StateDraft},
}

// NewMachine新しいImportJob状態機械を返します。遷移テーブルに不整合が
// あった場合はプログラミングミスなので起動時にpanicさせた方が発見しやすいです。
func NewMachine() *statemachine.Machine {
	m, err := statemachine.New(StateDraft, transitions)
	if err != nil {
		panic(fmt.Sprintf("importjob: 遷移テーブルが不正です: %v", err))
	}
	return m
}

// Jobインポートジョブ集約を表します。JSONタグはフロントエンドとの
// やり取りにそのまま使います。
type Job struct {
	ID          string             `json:"id"`
	Filename    string             `json:"filename"`
	Rows        int                `json:"rows"`
	Processed   int                `json:"processed"`
	State       statemachine.State `json:"state"`
	View        View               `json:"view"`
	Message     string             `json:"message"`
	CreatedAt   time.Time          `json:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt"`
	Transitions []TransitionEntry  `json:"transitions"`
}

// TransitionEntry 1回の状態変化の履歴を表します。監査用途やUX上の
// 「いつ何が起きたか」の表示に使えます。
type TransitionEntry struct {
	From  statemachine.State `json:"from"`
	To    statemachine.State `json:"to"`
	Event statemachine.Event `json:"event"`
	At    time.Time          `json:"at"`
}

// Clone深いコピーを返します。Repository内部の可変状態が外部に漏れるのを
// 防ぐためです。
func (j *Job) Clone() *Job {
	out := *j
	if len(j.Transitions) > 0 {
		out.Transitions = append([]TransitionEntry(nil), j.Transitions...)
	}
	return &out
}
