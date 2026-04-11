// Goバックエンドと通信する最小のAPIクライアントです。
// サードパーティのクライアントを入れず、標準のfetchだけで書いています。

import type { JobState, JobEvent, View } from "../state/importJobMachine";

const API_BASE =
  (import.meta.env.VITE_API_BASE as string | undefined) ??
  "http://localhost:8080";

export interface TransitionEntry {
  from: JobState;
  to: JobState;
  event: JobEvent;
  at: string;
}

export interface Job {
  id: string;
  filename: string;
  rows: number;
  processed: number;
  state: JobState;
  view: View;
  message: string;
  createdAt: string;
  updatedAt: string;
  transitions: TransitionEntry[] | null;
}

export interface CreateJobInput {
  filename: string;
  rows: number;
}

export interface ApplyEventInput {
  event: JobEvent;
  processed?: number;
  message?: string;
}

// ApiErrorレスポンスが2xxでないときの共通例外です。
// erasableSyntaxOnlyが有効なのでパラメータプロパティは使わずに書いています。
export class ApiError extends Error {
  readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function parse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) msg = body.error;
    } catch {
      /* 本文なしケース */
    }
    throw new ApiError(res.status, msg);
  }
  return (await res.json()) as T;
}

export async function listJobs(): Promise<Job[]> {
  const res = await fetch(`${API_BASE}/api/import_jobs`);
  const jobs = await parse<Job[] | null>(res);
  return jobs ?? [];
}

export async function createJob(input: CreateJobInput): Promise<Job> {
  const res = await fetch(`${API_BASE}/api/import_jobs`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  return parse<Job>(res);
}

export async function applyEvent(
  id: string,
  input: ApplyEventInput,
): Promise<Job> {
  const res = await fetch(`${API_BASE}/api/import_jobs/${id}/events`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  return parse<Job>(res);
}
