// ffmpeg 音声変換の単体テスト。
// 対応仕様: docs/04_implementation/04_realtime_wiring_design.md D-3、
// tasks/instructions_zundamon_wave_w10.md §3
//
// 【この開発環境には ffmpeg が無い】
// そのため本ファイルのテストは3系統に分かれる:
//  1. WrapPCMAsWAV（純粋関数）        … ffmpeg 不要。常に実行される。
//  2. 偽 ffmpeg バイナリ経由の結合検証 … /bin/sh のみ必要。常に実行される。
//     引数・stdin・stdout・stderr・終了コードの扱いを実際に exec して固定する。
//  3. 実 ffmpeg での往復変換          … ffmpeg が無ければ t.Skip（＝この環境では未検証）。
package unit

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/audioconv"
)

// whisper-server が受け付ける音声仕様（D-3: 16kHz・モノラル・16bit PCM）。
const (
	wantSampleRate    = 16000
	wantNumChannels   = 1
	wantBitsPerSample = 16
	wantWAVHeaderSize = 44
)

// ---------------------------------------------------------------------------
// 1. WrapPCMAsWAV（純粋関数）
// ---------------------------------------------------------------------------

func TestWrapPCMAsWAV_ヘッダの各フィールド(t *testing.T) {
	pcm := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	got := audioconv.WrapPCMAsWAV(pcm)

	require.Len(t, got, wantWAVHeaderSize+len(pcm))
	assert.Equal(t, "RIFF", string(got[0:4]))
	assert.Equal(t, uint32(36+len(pcm)), binary.LittleEndian.Uint32(got[4:8]), "RIFFチャンクサイズ")
	assert.Equal(t, "WAVE", string(got[8:12]))
	assert.Equal(t, "fmt ", string(got[12:16]))
	assert.Equal(t, uint32(16), binary.LittleEndian.Uint32(got[16:20]), "fmtチャンクサイズ(PCMは16)")
	assert.Equal(t, uint16(1), binary.LittleEndian.Uint16(got[20:22]), "オーディオフォーマット(1=リニアPCM)")
	assert.Equal(t, uint16(wantNumChannels), binary.LittleEndian.Uint16(got[22:24]), "チャンネル数")
	assert.Equal(t, uint32(wantSampleRate), binary.LittleEndian.Uint32(got[24:28]), "サンプリングレート")
	// バイトレート = サンプルレート × チャンネル数 × ビット深度/8
	assert.Equal(t, uint32(wantSampleRate*wantNumChannels*wantBitsPerSample/8),
		binary.LittleEndian.Uint32(got[28:32]), "バイトレート")
	assert.Equal(t, uint16(wantNumChannels*wantBitsPerSample/8),
		binary.LittleEndian.Uint16(got[32:34]), "ブロックアライン")
	assert.Equal(t, uint16(wantBitsPerSample), binary.LittleEndian.Uint16(got[34:36]), "ビット深度")
	assert.Equal(t, "data", string(got[36:40]))
	assert.Equal(t, uint32(len(pcm)), binary.LittleEndian.Uint32(got[40:44]), "dataチャンクサイズ")
	assert.Equal(t, pcm, got[44:], "PCM本体がそのまま連結されている")
}

func TestWrapPCMAsWAV_サイズフィールドは実長を反映する(t *testing.T) {
	// ffmpeg の wav マルチプレクサはパイプ出力(非シーク可能)だと
	// サイズを 0xFFFFFFFF のまま残す（libavformat/wavenc.c: wav_write_trailer は
	// AVIO_SEEKABLE_NORMAL のときしかサイズ欄を埋め戻さない）。
	// whisper.cpp が使う miniaudio(dr_wav) は 0xFFFFFFFF を番兵として走査し直す
	// ため実害はないが、デコーダ側の番兵処理に依存せずサイズが正しい正準WAVを
	// 自前で組み立てる方が堅牢なため、ここが実長であることが要。
	tests := []struct {
		name   string
		pcmLen int
	}{
		{name: "空のPCM", pcmLen: 0},
		{name: "1サンプル", pcmLen: 2},
		{name: "1秒ぶん", pcmLen: wantSampleRate * 2},
		{name: "奇数長でもそのまま", pcmLen: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := audioconv.WrapPCMAsWAV(bytes.Repeat([]byte{0x7f}, tt.pcmLen))

			require.Len(t, got, wantWAVHeaderSize+tt.pcmLen)
			assert.Equal(t, uint32(36+tt.pcmLen), binary.LittleEndian.Uint32(got[4:8]))
			assert.Equal(t, uint32(tt.pcmLen), binary.LittleEndian.Uint32(got[40:44]))
			assert.NotEqual(t, uint32(0xFFFFFFFF), binary.LittleEndian.Uint32(got[40:44]),
				"サイズ未確定のプレースホルダが残っている")
		})
	}
}

func TestWrapPCMAsWAV_nilのPCMでもヘッダのみのWAVになる(t *testing.T) {
	got := audioconv.WrapPCMAsWAV(nil)

	require.Len(t, got, wantWAVHeaderSize)
	assert.Equal(t, "RIFF", string(got[0:4]))
	assert.Equal(t, uint32(0), binary.LittleEndian.Uint32(got[40:44]))
}

// ---------------------------------------------------------------------------
// 2. 偽 ffmpeg バイナリ経由の結合検証（ffmpeg 不要・/bin/sh のみ）
// ---------------------------------------------------------------------------

// fakeFFmpeg は引数と標準入力を記録し、指定の標準出力・標準エラー・終了コードを返す
// シェルスクリプトを作って、そのパスと記録先を返す。
type fakeFFmpeg struct {
	path      string
	argsFile  string
	stdinFile string
}

func newFakeFFmpeg(t *testing.T, stdout, stderr string, exitCode int) fakeFFmpeg {
	t.Helper()
	dir := t.TempDir()
	f := fakeFFmpeg{
		path:      filepath.Join(dir, "fake-ffmpeg"),
		argsFile:  filepath.Join(dir, "args"),
		stdinFile: filepath.Join(dir, "stdin"),
	}

	// 出力はファイルへ置いて cat で流す。シェル引数へ直接埋めると
	// NUL や非ASCIIバイトを渡せず、PCM の代役にならないため。
	stdoutFile := filepath.Join(dir, "stdout")
	stderrFile := filepath.Join(dir, "stderr")
	require.NoError(t, os.WriteFile(stdoutFile, []byte(stdout), 0o644))
	require.NoError(t, os.WriteFile(stderrFile, []byte(stderr), 0o644))

	script := fmt.Sprintf(`#!/bin/sh
for a in "$@"; do printf '%%s\n' "$a"; done > %q
cat > %q
cat %q
cat %q >&2
exit %d
`, f.argsFile, f.stdinFile, stdoutFile, stderrFile, exitCode)

	require.NoError(t, os.WriteFile(f.path, []byte(script), 0o755))
	return f
}

func (f fakeFFmpeg) args(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(f.argsFile)
	require.NoError(t, err, "偽ffmpegが起動されていない")
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

func (f fakeFFmpeg) stdin(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(f.stdinFile)
	require.NoError(t, err, "偽ffmpegが起動されていない")
	return raw
}

func TestConverter_ffmpegへ渡す引数(t *testing.T) {
	// 引数が壊れても「変換に失敗する」だけで原因が見えないため、コマンドラインを固定する。
	// 根拠: whisper.cpp 本体の convert_to_wav も
	//   ffmpeg -i <in> -y -ar 16000 -ac 1 -c:a pcm_s16le <out.wav>
	// を使う（examples/server/server.cpp）。本実装はパイプ出力のため
	// 出力コンテナだけ生PCM(s16le)にし、WAVヘッダは Go 側で付ける。
	fake := newFakeFFmpeg(t, "pcm", "", 0)
	c := audioconv.NewConverter(audioconv.WithBinary(fake.path))

	_, err := c.Convert(context.Background(), []byte("webm-bytes"))
	require.NoError(t, err)

	args := fake.args(t)
	assert.Contains(t, args, "-i")
	assert.Contains(t, args, "pipe:0", "標準入力から読む")
	assert.Contains(t, args, "pipe:1", "標準出力へ書く")
	assert.Contains(t, args, "-ar")
	assert.Contains(t, args, "16000", "サンプリングレート16kHz")
	assert.Contains(t, args, "-ac")
	assert.Contains(t, args, "1", "モノラル")
	assert.Contains(t, args, "-f")
	assert.Contains(t, args, "s16le", "出力は16bitリトルエンディアンの生PCM")

	// -i の直後が入力指定であること（順序が壊れると ffmpeg は別解釈をする）。
	for i, a := range args {
		if a == "-i" {
			require.Less(t, i+1, len(args))
			assert.Equal(t, "pipe:0", args[i+1])
		}
	}
	// 出力指定は最後（ffmpeg は「オプションは直後の入出力に係る」文法）。
	assert.Equal(t, "pipe:1", args[len(args)-1])
}

func TestConverter_標準入力へ入力データを渡す(t *testing.T) {
	input := []byte("\x1a\x45\xdf\xa3webm-container-bytes")
	fake := newFakeFFmpeg(t, "pcm", "", 0)
	c := audioconv.NewConverter(audioconv.WithBinary(fake.path))

	_, err := c.Convert(context.Background(), input)
	require.NoError(t, err)

	assert.Equal(t, input, fake.stdin(t), "入力が標準入力へそのまま渡っていない")
}

func TestConverter_標準出力のPCMをWAVへ包んで返す(t *testing.T) {
	pcm := "\x01\x00\x02\x00\x03\x00"
	fake := newFakeFFmpeg(t, pcm, "", 0)
	c := audioconv.NewConverter(audioconv.WithBinary(fake.path))

	got, err := c.Convert(context.Background(), []byte("webm-bytes"))

	require.NoError(t, err)
	require.Len(t, got, wantWAVHeaderSize+len(pcm))
	assert.Equal(t, audioconv.WrapPCMAsWAV([]byte(pcm)), got)
	assert.Equal(t, "RIFF", string(got[0:4]))
	assert.Equal(t, uint32(len(pcm)), binary.LittleEndian.Uint32(got[40:44]))
}

func TestConverter_ffmpegが失敗したらstderrを含むエラー(t *testing.T) {
	const stderrMsg = "Invalid data found when processing input"
	fake := newFakeFFmpeg(t, "", stderrMsg, 1)
	c := audioconv.NewConverter(audioconv.WithBinary(fake.path))

	got, err := c.Convert(context.Background(), []byte("broken"))

	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), stderrMsg, "切り分けのため stderr をエラーへ含める")
}

func TestConverter_エラーに音声データを載せない(t *testing.T) {
	// 発話内容はエラー・ログへ出さない（NF-SEC-01 の趣旨・ガイドライン10.2）。
	const secret = "SECRET-AUDIO-PAYLOAD"
	fake := newFakeFFmpeg(t, "", "boom", 1)
	c := audioconv.NewConverter(audioconv.WithBinary(fake.path))

	_, err := c.Convert(context.Background(), []byte(secret))

	require.Error(t, err)
	assert.NotContains(t, err.Error(), secret)
}

func TestConverter_出力が空ならエラー(t *testing.T) {
	// 終了コード0でも出力が空なら変換は成立していない。
	// ヘッダだけのWAVを whisper-server へ送っても無意味なので、ここで落とす。
	fake := newFakeFFmpeg(t, "", "", 0)
	c := audioconv.NewConverter(audioconv.WithBinary(fake.path))

	got, err := c.Convert(context.Background(), []byte("webm-bytes"))

	require.Error(t, err)
	assert.Nil(t, got)
}

func TestConverter_キャンセル済みctxはエラー(t *testing.T) {
	fake := newFakeFFmpeg(t, "pcm", "", 0)
	c := audioconv.NewConverter(audioconv.WithBinary(fake.path))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Convert(ctx, []byte("webm-bytes"))

	require.Error(t, err)
}

func TestConverter_バイナリが見つからなければエラー(t *testing.T) {
	c := audioconv.NewConverter(audioconv.WithBinary(filepath.Join(t.TempDir(), "no-such-ffmpeg")))

	got, err := c.Convert(context.Background(), []byte("webm-bytes"))

	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "audioconv",
		"どのパッケージで失敗したか分かるようラップされていない: %v", err)
}

func TestConverter_入力が空ならffmpegを起動せずエラー(t *testing.T) {
	fake := newFakeFFmpeg(t, "pcm", "", 0)
	c := audioconv.NewConverter(audioconv.WithBinary(fake.path))

	_, err := c.Convert(context.Background(), nil)

	require.Error(t, err)
	_, statErr := os.Stat(fake.argsFile)
	assert.True(t, os.IsNotExist(statErr), "空入力でも ffmpeg を起動している")
}

// ---------------------------------------------------------------------------
// 3. 実 ffmpeg での往復変換（ffmpeg が無ければスキップ）
// ---------------------------------------------------------------------------

func TestConverter_実ffmpegでWAVへ変換できる(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Log("【未検証】ffmpeg が未インストールのためスキップしました。" +
			"このテストは緑ではなく『実行されていない』状態です。" +
			"実 ffmpeg との引数互換は W-11(Docker Compose、api イメージに ffmpeg 同梱)で初めて実証されます。")
		t.Skip("ffmpegが未インストールのためスキップ")
	}

	// 無音1秒を ffmpeg 自身に作らせ（lavfi anullsrc）、それを変換対象にする。
	src, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-t", "1",
		"-f", "webm", "-c:a", "libopus", "pipe:1").Output()
	require.NoError(t, err, "テスト用の入力音声を作れませんでした")

	c := audioconv.NewConverter()
	got, err := c.Convert(context.Background(), src)

	require.NoError(t, err)
	require.Greater(t, len(got), wantWAVHeaderSize)
	assert.Equal(t, "RIFF", string(got[0:4]))
	assert.Equal(t, "WAVE", string(got[8:12]))
	assert.Equal(t, uint16(wantNumChannels), binary.LittleEndian.Uint16(got[22:24]))
	assert.Equal(t, uint32(wantSampleRate), binary.LittleEndian.Uint32(got[24:28]))
	assert.Equal(t, uint32(len(got)-wantWAVHeaderSize), binary.LittleEndian.Uint32(got[40:44]))
	// 1秒 × 16000Hz × モノラル × 2バイト ≒ 32000バイト（コーデック都合の端数は許容）。
	assert.InDelta(t, wantSampleRate*2, len(got)-wantWAVHeaderSize, float64(wantSampleRate*2)*0.1)
}

func TestConverter_実ffmpegで壊れた入力はエラー(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Log("【未検証】ffmpeg が未インストールのためスキップしました。" +
			"このテストは緑ではなく『実行されていない』状態です。")
		t.Skip("ffmpegが未インストールのためスキップ")
	}

	c := audioconv.NewConverter()
	_, err := c.Convert(context.Background(), []byte("this is not audio"))

	require.Error(t, err)
}
