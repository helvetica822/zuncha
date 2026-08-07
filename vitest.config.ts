import { defineConfig } from 'vitest/config';
import { svelte, vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

// フェーズ4（フロントエンドロジック）のテスト設定。
// src/lib・src/components の実装を順次追加中。未実装クラスタのテストはimportエラーで失敗する。
export default defineConfig({
  // .svelte内の <script lang="ts"> をTypeScriptとして解釈するためのプリプロセッサ。
  plugins: [svelte({ preprocess: vitePreprocess() })],
  resolve: {
    alias: {
      $lib: path.resolve(__dirname, 'src/lib'),
      $components: path.resolve(__dirname, 'src/components'),
    },
  },
  test: {
    environment: 'jsdom',
    include: ['tests/unit/frontend/**/*.test.ts'],
    globals: true,
    setupFiles: ['./vitest-setup.ts'],
  },
});
