import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom/vitest";
import { ImportJobWorkbench } from "./ImportJobWorkbench";
import type { Job } from "../api/client";

function makeJob(overrides: Partial<Job> = {}): Job {
  return {
    id: "job-1",
    filename: "users.csv",
    rows: 10,
    processed: 0,
    state: "draft",
    view: "none",
    message: "",
    createdAt: "2026-04-11T00:00:00Z",
    updatedAt: "2026-04-11T00:00:00Z",
    transitions: [],
    ...overrides,
  };
}

describe("ImportJobWorkbench", () => {
  it("初期表示でジョブ一覧を取得する", async () => {
    const deps = {
      list: vi.fn().mockResolvedValue([makeJob()]),
      create: vi.fn(),
      apply: vi.fn(),
    };
    render(<ImportJobWorkbench deps={deps} />);
    await waitFor(() => expect(deps.list).toHaveBeenCalled());
    const sidebar = await screen.findByRole("list", { name: "ジョブ一覧" });
    expect(within(sidebar).getByText("job-1")).toBeInTheDocument();
  });

  it("ジョブを作成すると一覧の先頭に追加される", async () => {
    const created = makeJob({ id: "job-new", filename: "new.csv" });
    const deps = {
      list: vi.fn().mockResolvedValue([]),
      create: vi.fn().mockResolvedValue(created),
      apply: vi.fn(),
    };
    render(<ImportJobWorkbench deps={deps} />);
    const user = userEvent.setup();
    const filenameInput = await screen.findByLabelText("ファイル名");
    await user.clear(filenameInput);
    await user.type(filenameInput, "new.csv");
    await user.click(screen.getByRole("button", { name: "作成" }));
    await waitFor(() =>
      expect(deps.create).toHaveBeenCalledWith({
        filename: "new.csv",
        rows: 100,
      }),
    );
    const sidebar = await screen.findByRole("list", { name: "ジョブ一覧" });
    expect(within(sidebar).getByText("job-new")).toBeInTheDocument();
  });

  it("draft 状態では upload ボタンだけが表示される", async () => {
    const deps = {
      list: vi.fn().mockResolvedValue([makeJob()]),
      create: vi.fn(),
      apply: vi.fn(),
    };
    render(<ImportJobWorkbench deps={deps} />);
    const eventRegion = await screen.findByLabelText("現在の状態で可能な遷移");
    const buttons = within(eventRegion).getAllByRole("button");
    expect(buttons).toHaveLength(1);
    expect(buttons[0]).toHaveTextContent("アップロード");
  });

  it("ready 状態でモーダルサーフェスが表示される", async () => {
    const deps = {
      list: vi
        .fn()
        .mockResolvedValue([makeJob({ state: "ready", view: "modal" })]),
      create: vi.fn(),
      apply: vi.fn(),
    };
    render(<ImportJobWorkbench deps={deps} />);
    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent("登録完了モーダル");
    // モーダルとオーバーレイが同時に表示されることはない
    expect(
      screen.queryByRole("status", { name: /進捗オーバーレイ/ }),
    ).not.toBeInTheDocument();
  });

  it("importing 状態でオーバーレイが表示されモーダルは消える", async () => {
    const deps = {
      list: vi.fn().mockResolvedValue([
        makeJob({
          state: "importing",
          view: "overlay",
          processed: 3,
          rows: 10,
        }),
      ]),
      create: vi.fn(),
      apply: vi.fn(),
    };
    render(<ImportJobWorkbench deps={deps} />);
    const overlay = await screen.findByRole("status");
    expect(overlay).toHaveTextContent("インポート中");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("エラー状態ではアラートが表示される", async () => {
    const deps = {
      list: vi.fn().mockResolvedValue([
        makeJob({
          state: "validation_failed",
          view: "error",
          message: "必須列が不足",
        }),
      ]),
      create: vi.fn(),
      apply: vi.fn(),
    };
    render(<ImportJobWorkbench deps={deps} />);
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("必須列が不足");
  });

  it("イベントボタンを押すと API が呼ばれて更新される", async () => {
    const draft = makeJob();
    const uploading = makeJob({ state: "uploading", view: "overlay" });
    const deps = {
      list: vi.fn().mockResolvedValue([draft]),
      create: vi.fn(),
      apply: vi.fn().mockResolvedValue(uploading),
    };
    render(<ImportJobWorkbench deps={deps} />);
    const user = userEvent.setup();
    const button = await screen.findByRole("button", {
      name: "アップロード",
    });
    await user.click(button);
    await waitFor(() =>
      expect(deps.apply).toHaveBeenCalledWith("job-1", { event: "upload" }),
    );
    expect(await screen.findByRole("status")).toHaveTextContent(
      "アップロード中",
    );
  });
});
