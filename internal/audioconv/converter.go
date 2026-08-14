// Package audioconv は録音データ(webm/opus 等)を whisper-server が受け付ける
// 16kHz・モノラル・16bit の WAV へ変換する。
//
// 変換は ffmpeg バイナリへのパイプ処理で行い、一時ファイルを作らない
// (docs/04_implementation/04_realtime_wiring_design.md D-3。W-11 で api イメージへ ffmpeg を同梱する)。
package audioconv

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// whisper-server が期待する音声仕様(D-3)。
const (
	sampleRate    = 16000
	numChannels   = 1
	bitsPerSample = 16
	// wavHeaderSize は「RIFF + fmt(PCM) + data」だけの正準WAVヘッダの長さ。
	wavHeaderSize = 44
	// pcmFormatTag は WAVE_FORMAT_PCM(リニアPCM)。
	pcmFormatTag = 1
)

const (
	defaultBinary = "ffmpeg"
	// pcmCodec / pcmContainer は「ヘッダの無い生PCM」を指す ffmpeg の指定。
	pcmCodec     = "pcm_s16le"
	pcmContainer = "s16le"
	stdinPipe    = "pipe:0"
	stdoutPipe   = "pipe:1"
)

// Converter は音声データを ffmpeg で 16kHz mono WAV へ変換する。
type Converter struct {
	binary string
}

// Option は Converter 生成時のオプション。
type Option func(*Converter)

// WithBinary は起動する ffmpeg バイナリ(パス)を差し替える。
// コンテナ内の配置が標準と異なる場合と、テストで偽バイナリへ差し替える場合に使う。
func WithBinary(path string) Option {
	return func(c *Converter) { c.binary = path }
}

// NewConverter は ffmpeg 変換器を生成する。
func NewConverter(opts ...Option) *Converter {
	c := &Converter{binary: defaultBinary}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Convert は input(webm/opus 等)を 16kHz mono WAV へ変換して返す。
//
// ffmpeg には「生PCM」を出力させ、WAV ヘッダは WrapPCMAsWAV で Go 側が付ける。
// ffmpeg の wav マルチプレクサに直接 WAV を吐かせないのは、パイプ出力が
// 非シーク可能で RIFF/data のサイズを後から確定できないため
// (libavformat/riffenc.c の ff_start_tag はサイズに -1 を書き、
// wavenc.c の wav_write_trailer は AVIO_SEEKABLE_NORMAL のときしか
// ff_end_tag で埋め戻さない)。サイズが 0xFFFFFFFF のままの WAV を
// whisper-server へ渡すと、miniaudio の ma_decoder_get_length_in_pcm_frames が
// 巨大な値を返し、受け側が数GBの確保を試みる。
func (c *Converter) Convert(ctx context.Context, input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, errors.New("audioconv: 入力音声が空です")
	}

	args := []string{
		// 変換ログを最小限にする(stderr はエラー時のメッセージにのみ使う)。
		"-hide_banner", "-loglevel", "error",
		// 入力形式は自動判定させる。標準入力から読む。
		"-i", stdinPipe,
		// 録音に映像トラックが混ざっていても音声だけを扱う。
		"-vn",
		"-ac", strconv.Itoa(numChannels),
		"-ar", strconv.Itoa(sampleRate),
		"-c:a", pcmCodec,
		"-f", pcmContainer,
		// 出力指定は最後(ffmpeg のオプションは直後の入出力に係る文法のため)。
		stdoutPipe,
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.binary, args...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// stderr は切り分けに要るので含める。音声データ自体は絶対に含めない(NF-SEC-01)。
		return nil, fmt.Errorf("audioconv: ffmpegの実行に失敗しました (stderr: %s): %w",
			strings.TrimSpace(stderr.String()), err)
	}
	if stdout.Len() == 0 {
		// 終了コード0でも出力が無ければ変換は成立していない。
		// ヘッダだけの WAV を whisper-server へ送っても意味が無いのでここで落とす。
		return nil, fmt.Errorf("audioconv: ffmpegが音声を出力しませんでした (stderr: %s)",
			strings.TrimSpace(stderr.String()))
	}

	return WrapPCMAsWAV(stdout.Bytes()), nil
}

// WrapPCMAsWAV は 16bit/16kHz/モノラルのリニアPCMへ正準の WAV(RIFF) ヘッダを付けて返す。
//
// サイズフィールドには実測値を書く(プレースホルダを残さない)。Convert のコメント参照。
func WrapPCMAsWAV(pcm []byte) []byte {
	const (
		fmtChunkSize = 16
		byteRate     = sampleRate * numChannels * bitsPerSample / 8
		blockAlign   = numChannels * bitsPerSample / 8
	)

	out := make([]byte, wavHeaderSize, wavHeaderSize+len(pcm))
	copy(out[0:4], "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], uint32(wavHeaderSize-8+len(pcm)))
	copy(out[8:12], "WAVE")
	copy(out[12:16], "fmt ")
	binary.LittleEndian.PutUint32(out[16:20], fmtChunkSize)
	binary.LittleEndian.PutUint16(out[20:22], pcmFormatTag)
	binary.LittleEndian.PutUint16(out[22:24], numChannels)
	binary.LittleEndian.PutUint32(out[24:28], sampleRate)
	binary.LittleEndian.PutUint32(out[28:32], byteRate)
	binary.LittleEndian.PutUint16(out[32:34], blockAlign)
	binary.LittleEndian.PutUint16(out[34:36], bitsPerSample)
	copy(out[36:40], "data")
	binary.LittleEndian.PutUint32(out[40:44], uint32(len(pcm)))

	return append(out, pcm...)
}
