// Vitest + Testing Library用の共通セットアップ。
// afterEachで自動クリーンアップを有効化し、jest-domのマッチャーを読み込みます。
import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

afterEach(() => {
  cleanup();
});
