import type { InputMode } from './inputMode';

export type InputState = 'idle' | 'recording' | 'transcribing' | 'editable' | 'sending';

interface SendButtonParams {
  mode: InputMode;
  text: string;
  inputState: InputState;
}

// 送信ボタンの無効判定。editable以外の状態は常に無効。
// editableのとき、音声モードは空でも有効、テキストモードはtrim後に中身が無ければ無効。
export const isSendButtonDisabled = ({ mode, text, inputState }: SendButtonParams): boolean => {
  if (inputState !== 'editable') {
    return true;
  }
  if (mode === 'voice') {
    return false;
  }
  return text.trim() === '';
};
