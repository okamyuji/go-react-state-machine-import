// ImportJobの状態機械定義です。Goバックエンドのtransitionsと1対1で対応しています。
// 意図的にバックエンドとフロントエンドで同じテーブルを書き下すことで、
// 「遷移の単一の正」がチーム全員の目に入る場所にあることを示しています。

export type JobState =
  | "draft"
  | "uploading"
  | "upload_failed"
  | "parsing"
  | "parse_failed"
  | "validating"
  | "validation_failed"
  | "ready"
  | "importing"
  | "paused"
  | "completed"
  | "failed"
  | "cancelled";

export type JobEvent =
  | "upload"
  | "upload_succeed"
  | "upload_fail"
  | "parse_succeed"
  | "parse_fail"
  | "validate_succeed"
  | "validate_fail"
  | "retry"
  | "start"
  | "pause"
  | "resume"
  | "cancel"
  | "complete"
  | "fail"
  | "reset";

// Viewフロントエンドのルート描画を決定する唯一の入力です。
// 状態とViewは1対1に対応しているので、モーダルとオーバーレイが「同時に存在する」
// ような誤解は構造的にありえません。
export type View = "none" | "modal" | "overlay" | "result" | "error";

// transitionsバックエンドと同じ遷移テーブルです。
// フロントエンドでも楽観的にローカル遷移を計算するために持っています。
// 真の状態遷移はサーバ側の同名テーブルが担い、フロントはその結果を受け取って表示します。
export interface Transition {
  from: JobState;
  event: JobEvent;
  to: JobState;
}

export const transitions: readonly Transition[] = [
  // --- アップロードフェーズ ---
  { from: "draft", event: "upload", to: "uploading" },
  { from: "uploading", event: "upload_succeed", to: "parsing" },
  { from: "uploading", event: "upload_fail", to: "upload_failed" },
  { from: "upload_failed", event: "retry", to: "uploading" },
  { from: "upload_failed", event: "reset", to: "draft" },
  // --- パースフェーズ ---
  { from: "parsing", event: "parse_succeed", to: "validating" },
  { from: "parsing", event: "parse_fail", to: "parse_failed" },
  { from: "parse_failed", event: "reset", to: "draft" },
  // --- バリデーションフェーズ ---
  { from: "validating", event: "validate_succeed", to: "ready" },
  { from: "validating", event: "validate_fail", to: "validation_failed" },
  { from: "validation_failed", event: "reset", to: "draft" },
  // --- インポートフェーズ ---
  { from: "ready", event: "start", to: "importing" },
  { from: "ready", event: "cancel", to: "cancelled" },
  { from: "importing", event: "pause", to: "paused" },
  { from: "paused", event: "resume", to: "importing" },
  { from: "importing", event: "cancel", to: "cancelled" },
  { from: "paused", event: "cancel", to: "cancelled" },
  { from: "importing", event: "complete", to: "completed" },
  { from: "importing", event: "fail", to: "failed" },
  // --- 終端からのリセット ---
  { from: "completed", event: "reset", to: "draft" },
  { from: "failed", event: "reset", to: "draft" },
  { from: "cancelled", event: "reset", to: "draft" },
];

// viewFor状態から描画すべきサーフェスを返す純粋関数です。
// switch文が全ての状態を網羅しているかはTypeScriptのexhaustive checkで
// 静的に保証されます (defaultブロックのnever代入)。
export function viewFor(state: JobState): View {
  switch (state) {
    case "draft":
      return "none";
    case "ready":
      return "modal";
    case "uploading":
    case "parsing":
    case "validating":
    case "importing":
    case "paused":
      return "overlay";
    case "completed":
    case "failed":
    case "cancelled":
      return "result";
    case "upload_failed":
    case "parse_failed":
    case "validation_failed":
      return "error";
    default: {
      // 新しいStateを追加した際にここでコンパイルエラーになります。
      const _exhaustive: never = state;
      return _exhaustive;
    }
  }
}

// allowedEvents現在の状態で受け付けられるイベント一覧を返します。
// ボタンの活性制御に使います。
export function allowedEvents(state: JobState): JobEvent[] {
  return transitions.filter((t) => t.from === state).map((t) => t.event);
}

// canApply特定のイベントが現在の状態で受け付けられるかを判定します。
export function canApply(state: JobState, event: JobEvent): boolean {
  return transitions.some((t) => t.from === state && t.event === event);
}

// 表示用のラベル。コードロジックはstate値だけで動くので、
// 文言はこの1箇所にまとめて国際化もしやすくしています。
export const stateLabel: Record<JobState, string> = {
  draft: "下書き",
  uploading: "アップロード中",
  upload_failed: "アップロード失敗",
  parsing: "パース中",
  parse_failed: "パース失敗",
  validating: "バリデーション中",
  validation_failed: "バリデーション失敗",
  ready: "インポート準備完了",
  importing: "インポート中",
  paused: "一時停止",
  completed: "完了",
  failed: "失敗",
  cancelled: "キャンセル",
};

export const eventLabel: Record<JobEvent, string> = {
  upload: "アップロード",
  upload_succeed: "アップロード完了",
  upload_fail: "アップロード失敗",
  parse_succeed: "パース成功",
  parse_fail: "パース失敗",
  validate_succeed: "バリデーション成功",
  validate_fail: "バリデーション失敗",
  retry: "再試行",
  start: "インポート開始",
  pause: "一時停止",
  resume: "再開",
  cancel: "キャンセル",
  complete: "完了",
  fail: "失敗",
  reset: "リセット",
};
