// 対応仕様: docs/03_unit_test/08_test_specification.md 4.3 オーケストレーション層（観点2-3、TC-2-3-01〜08）
package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"zuncha/internal/model"
	"zuncha/internal/repository"
	"zuncha/internal/service"
)

type mockAudioRepository struct {
	mock.Mock
}

func (m *mockAudioRepository) GetByULID(ctx context.Context, ulid string) (*model.AudioFile, error) {
	args := m.Called(ctx, ulid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.AudioFile), args.Error(1)
}

func (m *mockAudioRepository) UpdateFetchedAt(ctx context.Context, ulid string, fetchedAt time.Time) error {
	args := m.Called(ctx, ulid, fetchedAt)
	return args.Error(0)
}

func (m *mockAudioRepository) DeleteRecord(ctx context.Context, ulid string) error {
	args := m.Called(ctx, ulid)
	return args.Error(0)
}

// InsertRecord は FetchAudioService からは呼ばれない（W-09のTTS側で使う）。
// そらの指摘: m.Called() を通さないスタブは AssertNotCalled や呼び出し順序の検証で
// 静かに偽陽性になるため、呼ばれない現時点でも m.Called() 形式にしておく。
func (m *mockAudioRepository) InsertRecord(ctx context.Context, audio *model.AudioFile) error {
	args := m.Called(ctx, audio)
	return args.Error(0)
}

var _ repository.AudioRepository = (*mockAudioRepository)(nil)

type mockFileStore struct {
	mock.Mock
}

func (m *mockFileStore) Read(path string) ([]byte, error) {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockFileStore) Delete(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

const testULID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

func testAudioFile() *model.AudioFile {
	return &model.AudioFile{
		ID:             testULID,
		ConversationID: "01BRZ3NDEKTSV4RRFFQ69G5FAV",
		MessageID:      "01CRZ3NDEKTSV4RRFFQ69G5FAV",
		FilePath:       "/tmp/audio/" + testULID + ".wav",
	}
}

func TestFetchAudio(t *testing.T) {
	t.Run("TC-2-3-01_全ステップが順序通り実行され成功する", func(t *testing.T) {
		repo := new(mockAudioRepository)
		files := new(mockFileStore)
		audio := testAudioFile()
		var callOrder []string

		repo.On("GetByULID", mock.Anything, testULID).Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "GetByULID")
		}).Return(audio, nil)
		files.On("Read", audio.FilePath).Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "Read")
		}).Return([]byte("wav-data"), nil)
		repo.On("UpdateFetchedAt", mock.Anything, testULID, mock.AnythingOfType("time.Time")).Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "UpdateFetchedAt")
		}).Return(nil)
		files.On("Delete", audio.FilePath).Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "Delete")
		}).Return(nil)
		repo.On("DeleteRecord", mock.Anything, testULID).Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "DeleteRecord")
		}).Return(nil)

		svc := service.NewFetchAudioService(repo, files)
		data, err := svc.FetchAudio(context.Background(), testULID)

		assert.NoError(t, err)
		assert.Equal(t, []byte("wav-data"), data)
		assert.Equal(t, []string{"GetByULID", "Read", "UpdateFetchedAt", "Delete", "DeleteRecord"}, callOrder)
		repo.AssertNumberOfCalls(t, "UpdateFetchedAt", 1)
		files.AssertNumberOfCalls(t, "Delete", 1)
		repo.AssertNumberOfCalls(t, "DeleteRecord", 1)
	})

	t.Run("TC-2-3-02_レコード不存在時は404相当でUpdateFetchedAt以降を呼ばない", func(t *testing.T) {
		repo := new(mockAudioRepository)
		files := new(mockFileStore)

		repo.On("GetByULID", mock.Anything, testULID).Return(nil, repository.ErrAudioNotFound)

		svc := service.NewFetchAudioService(repo, files)
		data, err := svc.FetchAudio(context.Background(), testULID)

		assert.ErrorIs(t, err, repository.ErrAudioNotFound)
		assert.Nil(t, data)
		repo.AssertNotCalled(t, "UpdateFetchedAt", mock.Anything, mock.Anything, mock.Anything)
		files.AssertNotCalled(t, "Delete", mock.Anything)
		repo.AssertNotCalled(t, "DeleteRecord", mock.Anything, mock.Anything)
	})

	t.Run("TC-2-3-03_Read失敗時は後続を呼ばない", func(t *testing.T) {
		repo := new(mockAudioRepository)
		files := new(mockFileStore)
		audio := testAudioFile()

		repo.On("GetByULID", mock.Anything, testULID).Return(audio, nil)
		files.On("Read", audio.FilePath).Return(nil, errors.New("file not found on disk"))

		svc := service.NewFetchAudioService(repo, files)
		_, err := svc.FetchAudio(context.Background(), testULID)

		assert.Error(t, err)
		repo.AssertNotCalled(t, "UpdateFetchedAt", mock.Anything, mock.Anything, mock.Anything)
		files.AssertNotCalled(t, "Delete", mock.Anything)
		repo.AssertNotCalled(t, "DeleteRecord", mock.Anything, mock.Anything)
	})

	t.Run("TC-2-3-04_UpdateFetchedAt失敗時はファイル削除以降を呼ばない", func(t *testing.T) {
		repo := new(mockAudioRepository)
		files := new(mockFileStore)
		audio := testAudioFile()

		repo.On("GetByULID", mock.Anything, testULID).Return(audio, nil)
		files.On("Read", audio.FilePath).Return([]byte("wav-data"), nil)
		repo.On("UpdateFetchedAt", mock.Anything, testULID, mock.AnythingOfType("time.Time")).Return(errors.New("db connection lost"))

		svc := service.NewFetchAudioService(repo, files)
		_, err := svc.FetchAudio(context.Background(), testULID)

		assert.Error(t, err)
		files.AssertNotCalled(t, "Delete", mock.Anything)
		repo.AssertNotCalled(t, "DeleteRecord", mock.Anything, mock.Anything)
	})

	t.Run("TC-2-3-05_Delete失敗時はDeleteRecordを呼ばない", func(t *testing.T) {
		repo := new(mockAudioRepository)
		files := new(mockFileStore)
		audio := testAudioFile()

		repo.On("GetByULID", mock.Anything, testULID).Return(audio, nil)
		files.On("Read", audio.FilePath).Return([]byte("wav-data"), nil)
		repo.On("UpdateFetchedAt", mock.Anything, testULID, mock.AnythingOfType("time.Time")).Return(nil)
		files.On("Delete", audio.FilePath).Return(errors.New("permission denied"))

		svc := service.NewFetchAudioService(repo, files)
		_, err := svc.FetchAudio(context.Background(), testULID)

		assert.Error(t, err)
		repo.AssertNotCalled(t, "DeleteRecord", mock.Anything, mock.Anything)
	})

	t.Run("TC-2-3-06_Delete失敗時に自動リトライしない", func(t *testing.T) {
		repo := new(mockAudioRepository)
		files := new(mockFileStore)
		audio := testAudioFile()

		repo.On("GetByULID", mock.Anything, testULID).Return(audio, nil)
		files.On("Read", audio.FilePath).Return([]byte("wav-data"), nil)
		repo.On("UpdateFetchedAt", mock.Anything, testULID, mock.AnythingOfType("time.Time")).Return(nil)
		files.On("Delete", audio.FilePath).Return(errors.New("permission denied"))

		svc := service.NewFetchAudioService(repo, files)
		_, err := svc.FetchAudio(context.Background(), testULID)

		assert.Error(t, err)
		files.AssertNumberOfCalls(t, "Delete", 1)
	})

	t.Run("TC-2-3-07_DeleteRecord失敗時は幽霊レコード状態を許容する", func(t *testing.T) {
		repo := new(mockAudioRepository)
		files := new(mockFileStore)
		audio := testAudioFile()

		repo.On("GetByULID", mock.Anything, testULID).Return(audio, nil)
		files.On("Read", audio.FilePath).Return([]byte("wav-data"), nil)
		repo.On("UpdateFetchedAt", mock.Anything, testULID, mock.AnythingOfType("time.Time")).Return(nil)
		files.On("Delete", audio.FilePath).Return(nil)
		repo.On("DeleteRecord", mock.Anything, testULID).Return(errors.New("db connection lost"))

		svc := service.NewFetchAudioService(repo, files)
		_, err := svc.FetchAudio(context.Background(), testULID)

		assert.Error(t, err)
	})

	t.Run("TC-2-3-08_対象0件は冪等に成功扱いとする", func(t *testing.T) {
		repo := new(mockAudioRepository)
		files := new(mockFileStore)
		audio := testAudioFile()

		repo.On("GetByULID", mock.Anything, testULID).Return(audio, nil)
		files.On("Read", audio.FilePath).Return([]byte("wav-data"), nil)
		// 対象0件（CASCADE削除で先に消えていた）でもエラーを返さない冪等な契約
		repo.On("UpdateFetchedAt", mock.Anything, testULID, mock.AnythingOfType("time.Time")).Return(nil)
		files.On("Delete", audio.FilePath).Return(nil)
		repo.On("DeleteRecord", mock.Anything, testULID).Return(nil)

		svc := service.NewFetchAudioService(repo, files)
		data, err := svc.FetchAudio(context.Background(), testULID)

		assert.NoError(t, err)
		assert.Equal(t, []byte("wav-data"), data)
	})
}
