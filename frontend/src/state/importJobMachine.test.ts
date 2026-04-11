import { describe, it, expect } from "vitest";
import {
  transitions,
  viewFor,
  canApply,
  allowedEvents,
  type JobState,
  type JobEvent,
} from "./importJobMachine";

describe("ImportJob 状態機械", () => {
  describe("viewFor", () => {
    it("draft は none を返す", () => {
      expect(viewFor("draft")).toBe("none");
    });
    it("ready はモーダルを返す", () => {
      expect(viewFor("ready")).toBe("modal");
    });
    it("importing と paused はオーバーレイを返す", () => {
      expect(viewFor("importing")).toBe("overlay");
      expect(viewFor("paused")).toBe("overlay");
    });
    it("各エラー状態はインラインエラーを返す", () => {
      expect(viewFor("upload_failed")).toBe("error");
      expect(viewFor("parse_failed")).toBe("error");
      expect(viewFor("validation_failed")).toBe("error");
    });
    it("completed/failed/cancelled は結果表示を返す", () => {
      expect(viewFor("completed")).toBe("result");
      expect(viewFor("failed")).toBe("result");
      expect(viewFor("cancelled")).toBe("result");
    });
  });

  describe("canApply", () => {
    it("draft から upload は可能", () => {
      expect(canApply("draft", "upload")).toBe(true);
    });
    it("draft から start はできない", () => {
      expect(canApply("draft", "start")).toBe(false);
    });
    it("ready から start が可能でキャンセルも可能", () => {
      expect(canApply("ready", "start")).toBe(true);
      expect(canApply("ready", "cancel")).toBe(true);
    });
    it("completed から reset のみ可能", () => {
      expect(canApply("completed", "reset")).toBe(true);
      expect(canApply("completed", "start")).toBe(false);
    });
  });

  describe("allowedEvents", () => {
    it("importing では pause, cancel, complete, fail が可能", () => {
      const events = allowedEvents("importing").sort();
      expect(events).toEqual(["cancel", "complete", "fail", "pause"]);
    });
  });

  describe("遷移テーブルの網羅性", () => {
    it("全13状態に到達できる（draft を除く始点なし状態は存在しない）", () => {
      const reachable = new Set<JobState>(["draft"]);
      let changed = true;
      while (changed) {
        changed = false;
        for (const t of transitions) {
          if (reachable.has(t.from) && !reachable.has(t.to)) {
            reachable.add(t.to);
            changed = true;
          }
        }
      }
      const all: JobState[] = [
        "draft",
        "uploading",
        "upload_failed",
        "parsing",
        "parse_failed",
        "validating",
        "validation_failed",
        "ready",
        "importing",
        "paused",
        "completed",
        "failed",
        "cancelled",
      ];
      for (const s of all) {
        expect(reachable.has(s)).toBe(true);
      }
    });

    it("同じ (from, event) を持つ遷移は存在しない", () => {
      const seen = new Set<string>();
      for (const t of transitions) {
        const key = `${t.from}:${t.event}`;
        expect(seen.has(key)).toBe(false);
        seen.add(key);
      }
    });

    it("全ての終端状態から reset で draft に戻れる", () => {
      const terminals: JobState[] = ["completed", "failed", "cancelled"];
      for (const term of terminals) {
        const reset: JobEvent = "reset";
        const t = transitions.find((x) => x.from === term && x.event === reset);
        expect(t?.to).toBe("draft");
      }
    });
  });
});
