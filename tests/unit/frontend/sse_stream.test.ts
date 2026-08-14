// 対応仕様: 実装計画 W-12（/home/masashiohashi/.claude/plans/shiny-napping-treehouse.md）
// 実装指示: tasks/instructions_zundamon_wave_w12.md（§2 確定済み契約 / §4 テスト観点）
// 契約根拠: バックエンド実装（internal/sse・internal/handler）から確定済みのSSEイベント5種。
import { describe, it, expect } from 'vitest';

import { parseServerEvent } from '../../../src/lib/sseStream';
import type { ServerEvent } from '../../../src/lib/sseStream';
import type { Emotion } from '../../../src/lib/sseEventRouter';

// 契約値をテスト側にも独立に書き写す（仕様の二重記述＝回帰検知）。
const ALL_EMOTIONS: Emotion[] = ['喜び', '怒り', '悲しみ', '楽しい', '照れ', '困惑', 'ドヤ顔'];
const FALLBACK_EMOTION: Emotion = '困惑';
const REQUEST_ID = '01J0000000000000000000000A';

describe('parseServerEvent - 正常系', () => {
  it.each(ALL_EMOTIONS)('TC-W12-01: emotionイベントは7種のラベル「%s」をそのまま変換する', (label) => {
    const event = parseServerEvent('emotion', JSON.stringify({ request_id: REQUEST_ID, label }));

    expect(event).toEqual<ServerEvent>({ type: 'emotion', requestId: REQUEST_ID, emotion: label });
  });

  it('TC-W12-02: textイベントはrequest_id/chunkを変換する', () => {
    const event = parseServerEvent('text', JSON.stringify({ request_id: REQUEST_ID, chunk: 'こんにちはなのだ' }));

    expect(event).toEqual<ServerEvent>({ type: 'text', requestId: REQUEST_ID, chunk: 'こんにちはなのだ' });
  });

  it('TC-W12-03: audio_urlイベントはtypeを audioUrl にキャメルケース変換する', () => {
    const event = parseServerEvent(
      'audio_url',
      JSON.stringify({ request_id: REQUEST_ID, url: '/audio/01J0000000000000000000000B' }),
    );

    expect(event).toEqual<ServerEvent>({
      type: 'audioUrl',
      requestId: REQUEST_ID,
      url: '/audio/01J0000000000000000000000B',
    });
  });

  it('TC-W12-04: doneイベントはrequest_idのみで変換される', () => {
    const event = parseServerEvent('done', JSON.stringify({ request_id: REQUEST_ID }));

    expect(event).toEqual<ServerEvent>({ type: 'done', requestId: REQUEST_ID });
  });

  it('TC-W12-05: errorイベントはrequest_id/messageを変換する', () => {
    const event = parseServerEvent(
      'error',
      JSON.stringify({ request_id: REQUEST_ID, message: 'サーバーエラーが発生しました' }),
    );

    expect(event).toEqual<ServerEvent>({
      type: 'error',
      requestId: REQUEST_ID,
      message: 'サーバーエラーが発生しました',
    });
  });

  it('TC-W12-06: errorのrequest_idが空文字列でも正当な値として変換される（接続レベル障害）', () => {
    const event = parseServerEvent('error', JSON.stringify({ request_id: '', message: '接続が切れました' }));

    expect(event).toEqual<ServerEvent>({ type: 'error', requestId: '', message: '接続が切れました' });
  });

  it('TC-W12-07: textのchunkが空文字列でも破棄せず変換する（空チャンクは正当な値）', () => {
    const event = parseServerEvent('text', JSON.stringify({ request_id: REQUEST_ID, chunk: '' }));

    expect(event).toEqual<ServerEvent>({ type: 'text', requestId: REQUEST_ID, chunk: '' });
  });

  it('TC-W12-08: 契約外の余分なフィールドは無視され、必要な値のみが取り出される', () => {
    const event = parseServerEvent(
      'done',
      JSON.stringify({ request_id: REQUEST_ID, unexpected: { nested: true }, seq: 42 }),
    );

    expect(event).toEqual<ServerEvent>({ type: 'done', requestId: REQUEST_ID });
  });

  it('TC-W12-09: 1000文字のchunkもそのまま保持される（長大値）', () => {
    const longChunk = 'あ'.repeat(1000);

    const event = parseServerEvent('text', JSON.stringify({ request_id: REQUEST_ID, chunk: longChunk }));

    expect(event).toEqual<ServerEvent>({ type: 'text', requestId: REQUEST_ID, chunk: longChunk });
  });
});

describe('parseServerEvent - 異常系（不正入力は null で黙って無視する）', () => {
  it('TC-W12-10: 不正なJSON文字列は null', () => {
    expect(parseServerEvent('text', '{request_id:')).toBeNull();
  });

  it('TC-W12-11: 空文字列のdataは null', () => {
    expect(parseServerEvent('done', '')).toBeNull();
  });

  it.each(['null', '123', '"文字列"', '[]'])('TC-W12-12: JSONオブジェクトでないdata（%s）は null', (rawData) => {
    expect(parseServerEvent('done', rawData)).toBeNull();
  });

  it.each(['unknown', 'text_chunk', 'audioUrl', 'ping', ''])(
    'TC-W12-13: 未知のイベント名（%s）は null',
    (eventName) => {
      expect(parseServerEvent(eventName, JSON.stringify({ request_id: REQUEST_ID, chunk: 'x' }))).toBeNull();
    },
  );

  it('TC-W12-14: request_idが数値型（文字列以外）は null', () => {
    expect(parseServerEvent('done', JSON.stringify({ request_id: 123 }))).toBeNull();
  });

  it('TC-W12-15: request_idキー自体が無い場合は null', () => {
    expect(parseServerEvent('text', JSON.stringify({ chunk: 'こんにちは' }))).toBeNull();
  });

  it('TC-W12-16: request_idがnullは null（JSONのnullを空文字列として通さない）', () => {
    expect(parseServerEvent('done', JSON.stringify({ request_id: null }))).toBeNull();
  });

  it('TC-W12-17: emotionでlabelキーが欠落している場合は null（パース失敗。困惑にはしない）', () => {
    expect(parseServerEvent('emotion', JSON.stringify({ request_id: REQUEST_ID }))).toBeNull();
  });

  it('TC-W12-18: emotionでlabelが文字列以外（数値）の場合は null（型不一致）', () => {
    expect(parseServerEvent('emotion', JSON.stringify({ request_id: REQUEST_ID, label: 42 }))).toBeNull();
  });

  it('TC-W12-19: emotionで未知の感情ラベル（7種外の文字列）は困惑にフォールバックする', () => {
    const event = parseServerEvent('emotion', JSON.stringify({ request_id: REQUEST_ID, label: '興奮' }));

    expect(event).toEqual<ServerEvent>({ type: 'emotion', requestId: REQUEST_ID, emotion: FALLBACK_EMOTION });
  });

  it('TC-W12-20: emotionでlabelが空文字列でも困惑にフォールバックする（キーは存在するため）', () => {
    const event = parseServerEvent('emotion', JSON.stringify({ request_id: REQUEST_ID, label: '' }));

    expect(event).toEqual<ServerEvent>({ type: 'emotion', requestId: REQUEST_ID, emotion: FALLBACK_EMOTION });
  });

  it('TC-W12-21: textでchunkキーが欠落している場合は null', () => {
    expect(parseServerEvent('text', JSON.stringify({ request_id: REQUEST_ID }))).toBeNull();
  });

  it('TC-W12-22: textでchunkが文字列以外（数値）の場合は null', () => {
    expect(parseServerEvent('text', JSON.stringify({ request_id: REQUEST_ID, chunk: 1 }))).toBeNull();
  });

  it('TC-W12-23: audio_urlでurlキーが欠落している場合は null', () => {
    expect(parseServerEvent('audio_url', JSON.stringify({ request_id: REQUEST_ID }))).toBeNull();
  });

  it('TC-W12-24: audio_urlでurlが文字列以外（null）の場合は null', () => {
    expect(parseServerEvent('audio_url', JSON.stringify({ request_id: REQUEST_ID, url: null }))).toBeNull();
  });

  it('TC-W12-25: errorでmessageキーが欠落していても空文字列にフォールバックする（nullにしない）', () => {
    const event = parseServerEvent('error', JSON.stringify({ request_id: REQUEST_ID }));

    expect(event).toEqual<ServerEvent>({ type: 'error', requestId: REQUEST_ID, message: '' });
  });

  it('TC-W12-26: errorでmessageが空文字列でも空文字列のまま返す（文言の既定値差し替えは呼び出し側の責務）', () => {
    const event = parseServerEvent('error', JSON.stringify({ request_id: REQUEST_ID, message: '' }));

    expect(event).toEqual<ServerEvent>({ type: 'error', requestId: REQUEST_ID, message: '' });
  });

  it('TC-W12-27: errorでmessageが文字列以外（数値）でも空文字列にフォールバックする', () => {
    const event = parseServerEvent('error', JSON.stringify({ request_id: REQUEST_ID, message: 500 }));

    expect(event).toEqual<ServerEvent>({ type: 'error', requestId: REQUEST_ID, message: '' });
  });
});
