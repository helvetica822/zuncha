package service

import (
	"context"
	"time"

	"zuncha/internal/repository"
)

// FetchAudioService は音声取得〜削除ユースケースを担う。
type FetchAudioService struct {
	repo  repository.AudioRepository
	files FileStore
}

func NewFetchAudioService(repo repository.AudioRepository, files FileStore) *FetchAudioService {
	return &FetchAudioService{repo: repo, files: files}
}

// FetchAudio は GetByULID→Read→UpdateFetchedAt→Delete→DeleteRecord の順で実行し、
// 途中で失敗したら後続を呼ばず中断してエラーを返す。
func (s *FetchAudioService) FetchAudio(ctx context.Context, ulid string) ([]byte, error) {
	audio, err := s.repo.GetByULID(ctx, ulid)
	if err != nil {
		return nil, err
	}
	data, err := s.files.Read(audio.FilePath)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateFetchedAt(ctx, ulid, time.Now()); err != nil {
		return nil, err
	}
	if err := s.files.Delete(audio.FilePath); err != nil {
		return nil, err
	}
	if err := s.repo.DeleteRecord(ctx, ulid); err != nil {
		return nil, err
	}
	return data, nil
}
