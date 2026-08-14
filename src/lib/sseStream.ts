import type { Emotion } from './sseEventRouter';

// バックエンドが実際に送出するSSEイベント5種を型付きで表現する。
// SSEの`event:`行はスネークケース（`audio_url`）だが、フロント側の型はキャメルケース（`audioUrl`）に揃える。
export type ServerEvent =
  | { type: 'emotion'; requestId: string; emotion: Emotion }
  | { type: 'text'; requestId: string; chunk: string }
  | { type: 'audioUrl'; requestId: string; url: string }
  | { type: 'done'; requestId: string }
  | { type: 'error'; requestId: string; message: string };

// Emotion型の実体を実行時にも判定するための一覧。型は sseEventRouter を唯一の源泉とし、
// ここでは `Emotion[]` として宣言することで型と値の乖離をコンパイル時に検出する。
const EMOTIONS: readonly Emotion[] = ['喜び', '怒り', '悲しみ', '楽しい', '照れ', '困惑', 'ドヤ顔'];

// 7種外のラベルが来たときの既定値。internal/llm.ParseLLMResponse（validation.FallbackEmotion）と同じ方針。
const FALLBACK_EMOTION: Emotion = '困惑';

// errorイベントの message は「イベント自体は届いている」ため欠落しても破棄せず空文字列にする。
// 空文字列を DEFAULT_ERROR_MESSAGE に差し替えるのは呼び出し側（sseEventRouter / W-14）の責務であり、
// 文言のフォールバック規則をこのファイルに重複させない。
const EMPTY_MESSAGE = '';

// data行の1行JSONをオブジェクトとして解釈する。
// 構文エラー・JSONオブジェクト以外（null/数値/文字列/配列）は解釈不能として null を返す。
// この型ガード自体は公開APIからは観測できない（配列や数値は `request_id` を持ち得ず、
// 後続の必須フィールド判定でどのみち null になるため＝ミューテーション実測で等価ミュータントと確認済み）。
// それでも残すのは、「dataはJSONオブジェクトである」という契約を明示し、
// プリミティブへのプロパティアクセスが undefined を返すというJSの挙動に依存させないため。
const parseObject = (rawData: string): Record<string, unknown> | null => {
  let parsed: unknown;
  try {
    parsed = JSON.parse(rawData);
  } catch {
    return null;
  }
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    return null;
  }
  return parsed as Record<string, unknown>;
};

// 必須の文字列フィールドを取り出す。キー欠落・null・非文字列はすべて null（＝パース失敗）。
// 空文字列は正当な値として通す（error の request_id は接続レベル障害で空になりうる）。
const requiredString = (record: Record<string, unknown>, key: string): string | null => {
  const value = record[key];
  return typeof value === 'string' ? value : null;
};

// eventName（SSEの`event:`行）と rawData（`data:`行の1行JSON文字列）を受け取り、型付きイベントへ変換する。
// 不正JSON・未知イベント名・必須フィールドの型不一致は null を返して黙って無視する
// （サーバーの実装ミスでフロントをクラッシュさせないための防御）。
export const parseServerEvent = (eventName: string, rawData: string): ServerEvent | null => {
  const record = parseObject(rawData);
  if (record === null) {
    return null;
  }

  const requestId = requiredString(record, 'request_id');
  if (requestId === null) {
    return null;
  }

  switch (eventName) {
    case 'emotion': {
      // 2段階の区別:
      //   (1) labelキー自体が無い/文字列でない → パース失敗として null。
      //       これはサーバーがイベント契約を満たしていない＝データが壊れている状態であり、
      //       感情を推測して表示するより破棄する方が安全（internal/llm の ErrSchema/ErrValue に相当）。
      //   (2) labelは文字列だが7種外（空文字列を含む） → 「困惑」にフォールバック。
      //       LLMが未知のラベルを返すのは想定内の揺らぎであり、
      //       internal/llm.ParseLLMResponse が validation.FallbackEmotion へ倒すのと同じ防御方針に揃える。
      const label = requiredString(record, 'label');
      if (label === null) {
        return null;
      }
      const emotion = EMOTIONS.includes(label as Emotion) ? (label as Emotion) : FALLBACK_EMOTION;
      return { type: 'emotion', requestId, emotion };
    }
    case 'text': {
      const chunk = requiredString(record, 'chunk');
      if (chunk === null) {
        return null;
      }
      return { type: 'text', requestId, chunk };
    }
    case 'audio_url': {
      const url = requiredString(record, 'url');
      if (url === null) {
        return null;
      }
      return { type: 'audioUrl', requestId, url };
    }
    case 'done':
      return { type: 'done', requestId };
    case 'error':
      return { type: 'error', requestId, message: requiredString(record, 'message') ?? EMPTY_MESSAGE };
    default:
      return null;
  }
};
