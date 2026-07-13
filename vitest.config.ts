import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

// フェーズ4（フロントエンドロジック）のRED phase用スカフォールド。
// 対象実装（src/lib・src/components）は未実装のため、テスト実行はimportエラーで失敗する。
export default defineConfig({
  plugins: [svelte()],
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
