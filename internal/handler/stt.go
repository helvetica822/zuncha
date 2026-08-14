package handler

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"zuncha/internal/stt"
	"zuncha/internal/validation"
)

const (
	// sttTimeout は音声認識(ffmpeg変換 + whisper-server)全体の上限。
	//
	// 根拠: STT はユーザーが「変換中」表示のまま待つ同期処理で、体感の許容上限が
	// そのまま予算になる。録音は無音8秒で打ち切られる(01_screen_design.md 9-2)ため
	// 入力音声はせいぜい十数秒で、whisper.cpp の CPU 実行がほぼ実時間なら
	// 通常は数秒で返る。30秒はその数倍の余裕を見た打ち切り点であり、
	// 応答生成の予算(responseTimeout = 60秒 = LLM 30 + TTS 20 + 余地)の半分に相当する。
	// 内訳は whisper-server 側 25秒(whispercpp.defaultRequestTimeout)+
	// multipart受信・ffmpeg変換・存在チェックの余地 5秒。
	sttTimeout = 30 * time.Second

	// sttAudioField はフロントとの契約(multipart のフィールド名)。
	sttAudioField = "audio"

	// sttMaxAudioBytes はアップロード音声の上限。
	// webm/opus はおおむね 3KB/秒程度なので、10MB は 50分近い録音に相当する
	// (無音8秒で打ち切られる運用では到達しない)。無制限に読み込むと
	// 1リクエストでメモリを食い潰せるため上限を置く。
	sttMaxAudioBytes = 10 << 20
)

// sttSuccessResponse は認識成功時のレスポンス本文(01_screen_design.md §7.3)。
type sttSuccessResponse struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

// sttFailedResponse は認識失敗時のレスポンス本文(04_realtime_wiring_design.md D-3)。
type sttFailedResponse struct {
	Failed bool `json:"failed"`
}

// HandleSTT は POST /conversations/{id}/stt を処理する。
//
// 同期処理なので r.Context() を土台にタイムアウトを被せるだけでよい
// (PostMessage のような 202 + goroutine 方式は不要。結果を本文で返すため)。
func (h *Handler) HandleSTT(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validation.IsValidULID(id) {
		respondError(w, http.StatusBadRequest, "会話IDの形式が不正です")
		return
	}

	// 音声本体を読む前に存在確認する(存在しない会話のために最大10MBを
	// 読み込む・変換するのは無駄なため)。
	exists, err := h.convRepo.Exists(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "会話の確認に失敗しました")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "会話が見つかりません")
		return
	}

	audio, ok := readAudioUpload(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), sttTimeout)
	defer cancel()

	result, err := h.speechToText.Transcribe(ctx, audio)
	if err != nil {
		// 内部の接続先やコマンド出力をクライアントへ返さない(NF-SEC-01)。詳細はログのみ。
		log.Printf("音声認識に失敗: conversation_id=%s: %v", id, err)
		respondError(w, http.StatusInternalServerError, "音声認識に失敗しました")
		return
	}

	// 認識失敗はクライアントエラーではなく正常系の一部。400/500 にしない
	// (フロントは 200 {failed:true} を handleSttFailure() で受ける)。
	if stt.IsRecognitionFailed(result) {
		respondJSON(w, http.StatusOK, sttFailedResponse{Failed: true})
		return
	}

	respondJSON(w, http.StatusOK, sttSuccessResponse{
		Text:       result.Text,
		Confidence: result.Confidence,
	})
}

// readAudioUpload は multipart/form-data から音声バイト列を取り出す。
// 取り出せなかった場合は 400 を返して ok=false にする。
func readAudioUpload(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, sttMaxAudioBytes)

	// maxMemory を上限と同じにして、一時ファイルへ溢れさせない
	// (どのみち全量をメモリへ読むうえ、ディスクI/Oを避ける方針のため)。
	if err := r.ParseMultipartForm(sttMaxAudioBytes); err != nil {
		respondError(w, http.StatusBadRequest, "音声データの形式が不正です")
		return nil, false
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, _, err := r.FormFile(sttAudioField)
	if err != nil {
		respondError(w, http.StatusBadRequest, "音声データが見つかりません")
		return nil, false
	}
	defer file.Close()

	audio, err := io.ReadAll(file)
	if err != nil {
		respondError(w, http.StatusBadRequest, "音声データの読み取りに失敗しました")
		return nil, false
	}
	if len(audio) == 0 {
		respondError(w, http.StatusBadRequest, "音声データが空です")
		return nil, false
	}
	return audio, true
}
