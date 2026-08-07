export type Emotion = '喜び' | '怒り' | '悲しみ' | '楽しい' | '照れ' | '困惑' | 'ドヤ顔';

export type SSEEvent =
  | { type: 'error'; payload: { message?: string } }
  | { type: 'message'; payload: { text: string; emotion: Emotion } };

export interface SSEEventHandlers {
  onError: (message: string) => void;
  onMessage: (text: string, emotion: Emotion) => void;
}

export const DEFAULT_ERROR_MESSAGE = 'エラーが発生しました。もう一度お試しください。';

// error は onError のみ、message は onMessage のみ（排他）。
// error の message が欠落/空文字なら DEFAULT_ERROR_MESSAGE にフォールバックする。
export const routeSSEEvent = (event: SSEEvent, handlers: SSEEventHandlers): void => {
  if (event.type === 'error') {
    handlers.onError(event.payload.message || DEFAULT_ERROR_MESSAGE);
    return;
  }
  handlers.onMessage(event.payload.text, event.payload.emotion);
};
