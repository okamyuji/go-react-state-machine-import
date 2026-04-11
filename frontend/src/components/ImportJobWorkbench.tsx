// ImportJobWorkbenchサンプルアプリのメイン画面です。
// ジョブの作成・状態遷移のトリガー・現状の可視化を1画面で行い、
// 状態機械が「1つの正しい状態」に収束している様子を目で確認できます。

import { useCallback, useEffect, useState } from "react";
import {
  applyEvent,
  createJob,
  listJobs,
  ApiError,
  type Job,
} from "../api/client";
import {
  allowedEvents,
  eventLabel,
  stateLabel,
  type JobEvent,
} from "../state/importJobMachine";

interface Props {
  // API呼び出しを差し替えられるようにしています。既定値は本物のクライアントです。
  deps?: {
    list: typeof listJobs;
    create: typeof createJob;
    apply: typeof applyEvent;
  };
}

const defaultDeps = { list: listJobs, create: createJob, apply: applyEvent };

export function ImportJobWorkbench({ deps = defaultDeps }: Props) {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [filename, setFilename] = useState("users.csv");
  const [rows, setRows] = useState(100);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // refreshは依存配列からselectedIDを外し、関数が再生成されないようにしています。
  // こうしないと選択変更のたびにuseEffectが走ってlist() が呼ばれ、
  // 楽観更新したjobsがサーバ応答で上書きされてしまいます。
  const refresh = useCallback(async () => {
    try {
      const list = await deps.list();
      setJobs(list);
      setSelectedID((prev) => prev ?? list[0]?.id ?? null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [deps]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const selected = jobs.find((j) => j.id === selectedID) ?? null;

  const handleCreate = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const job = await deps.create({ filename, rows });
      setJobs((prev) => [job, ...prev]);
      setSelectedID(job.id);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [deps, filename, rows]);

  const handleEvent = useCallback(
    async (event: JobEvent) => {
      if (!selected) return;
      setLoading(true);
      setError(null);
      try {
        const updated = await deps.apply(selected.id, { event });
        setJobs((prev) => prev.map((j) => (j.id === updated.id ? updated : j)));
      } catch (e) {
        if (e instanceof ApiError) {
          setError(`${e.status}: ${e.message}`);
        } else {
          setError(e instanceof Error ? e.message : String(e));
        }
      } finally {
        setLoading(false);
      }
    },
    [deps, selected],
  );

  return (
    <div className="workbench">
      <header className="workbench__header">
        <h1>ImportJob 状態遷移ワークベンチ</h1>
        <p>
          Go バックエンドと同じ遷移テーブルを TypeScript でも宣言し、
          サーバから返ってきた <code>state</code> と <code>view</code> を
          そのまま描画します。ボタンは現在の状態で受け付け可能なイベントだけが
          活性化します。
        </p>
      </header>

      <section className="workbench__create" aria-labelledby="create-heading">
        <h2 id="create-heading">新しいジョブを作成する</h2>
        <div className="form-row">
          <label>
            ファイル名
            <input
              type="text"
              value={filename}
              onChange={(e) => setFilename(e.target.value)}
              aria-label="ファイル名"
            />
          </label>
          <label>
            行数
            <input
              type="number"
              min={1}
              value={rows}
              onChange={(e) => setRows(Number(e.target.value))}
              aria-label="行数"
            />
          </label>
          <button
            type="button"
            onClick={handleCreate}
            disabled={loading || !filename || rows <= 0}
          >
            作成
          </button>
        </div>
      </section>

      <section className="workbench__grid">
        <aside className="workbench__list" aria-labelledby="list-heading">
          <h2 id="list-heading">ジョブ一覧</h2>
          {jobs.length === 0 ? (
            <p className="muted">ジョブはまだありません</p>
          ) : (
            <ul aria-label="ジョブ一覧">
              {jobs.map((job) => (
                <li key={job.id}>
                  <button
                    type="button"
                    className={selectedID === job.id ? "selected" : ""}
                    onClick={() => setSelectedID(job.id)}
                  >
                    <span className="job-id">{job.id}</span>
                    <span className="job-filename">{job.filename}</span>
                    <span className={`state state--${job.state}`}>
                      {stateLabel[job.state]}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </aside>

        <main className="workbench__detail" aria-labelledby="detail-heading">
          <h2 id="detail-heading">
            詳細
            {selected && (
              <span className={`state state--${selected.state}`}>
                {stateLabel[selected.state]}
              </span>
            )}
          </h2>

          {error && (
            <div className="alert" role="alert">
              {error}
            </div>
          )}

          {!selected ? (
            <p className="muted">ジョブを選択してください</p>
          ) : (
            <>
              <dl className="detail-list">
                <div>
                  <dt>ID</dt>
                  <dd>{selected.id}</dd>
                </div>
                <div>
                  <dt>ファイル名</dt>
                  <dd>{selected.filename}</dd>
                </div>
                <div>
                  <dt>行数</dt>
                  <dd>
                    {selected.processed} / {selected.rows}
                  </dd>
                </div>
                <div>
                  <dt>View</dt>
                  <dd>
                    <code>{selected.view}</code>
                  </dd>
                </div>
                <div>
                  <dt>メッセージ</dt>
                  <dd>
                    {selected.message || <span className="muted">—</span>}
                  </dd>
                </div>
              </dl>

              <div
                className="event-buttons"
                aria-label="現在の状態で可能な遷移"
              >
                <h3>可能な遷移</h3>
                {allowedEvents(selected.state).length === 0 ? (
                  <p className="muted">このジョブは終端状態です</p>
                ) : (
                  <ul>
                    {allowedEvents(selected.state).map((ev) => (
                      <li key={ev}>
                        <button
                          type="button"
                          onClick={() => void handleEvent(ev)}
                          disabled={loading}
                          data-event={ev}
                        >
                          {eventLabel[ev]}
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </div>

              <ViewSurface job={selected} />

              <section className="history" aria-labelledby="history-heading">
                <h3 id="history-heading">遷移履歴</h3>
                {(selected.transitions ?? []).length === 0 ? (
                  <p className="muted">まだ遷移していません</p>
                ) : (
                  <ol>
                    {(selected.transitions ?? []).map((t, i) => (
                      <li key={`${t.at}-${i}`}>
                        <span className={`state state--${t.from}`}>
                          {stateLabel[t.from]}
                        </span>
                        <span aria-hidden="true">→</span>
                        <span className={`state state--${t.to}`}>
                          {stateLabel[t.to]}
                        </span>
                        <span className="event">({eventLabel[t.event]})</span>
                      </li>
                    ))}
                  </ol>
                )}
              </section>
            </>
          )}
        </main>
      </section>
    </div>
  );
}

// ViewSurface 「stateとviewが1対1対応している」様子を視覚的に見せる
// 中心のコンポーネントです。分岐はviewの値ごとにしかありません。
function ViewSurface({ job }: { job: Job }) {
  switch (job.view) {
    case "none":
      return (
        <div className="surface surface--none" data-view="none">
          <p>表示中のサーフェスはありません (draft)</p>
        </div>
      );
    case "modal":
      return (
        <div
          className="surface surface--modal"
          data-view="modal"
          role="dialog"
          aria-modal="true"
        >
          <h3>登録完了モーダル</h3>
          <p>
            {job.filename} を受け取り、バリデーションまで通過しました。
            「インポート開始」を押してインポートを実行してください。
          </p>
        </div>
      );
    case "overlay":
      return (
        <div
          className="surface surface--overlay"
          data-view="overlay"
          role="status"
        >
          <h3>進捗オーバーレイ</h3>
          <p>
            <strong>{stateLabel[job.state]}</strong> 中です
          </p>
          <progress max={job.rows} value={job.processed} />
          <p>
            {job.processed} / {job.rows} 行
          </p>
        </div>
      );
    case "error":
      return (
        <div className="surface surface--error" data-view="error" role="alert">
          <h3>{stateLabel[job.state]}</h3>
          <p>{job.message || "エラーメッセージは指定されていません"}</p>
          <p className="muted">
            上の「可能な遷移」から回復アクションを選んでください
          </p>
        </div>
      );
    case "result":
      return (
        <div className="surface surface--result" data-view="result">
          <h3>結果: {stateLabel[job.state]}</h3>
          <p>{job.message || "メッセージは指定されていません"}</p>
          <p>
            {job.processed} / {job.rows} 行が処理されました
          </p>
        </div>
      );
    default: {
      // 新しいViewを追加したらexhaustive checkで検知されます。
      const _exhaustive: never = job.view;
      return _exhaustive;
    }
  }
}
