// 対応仕様: docs/03_unit_test/08_test_specification.md 4.1 オーケストレーション層（観点2-1、TC-2-1-01〜07）
package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"zuncha/internal/model"
	"zuncha/internal/repository"
	"zuncha/internal/service"
)

type mockConversationRepository struct {
	mock.Mock
}

// GCはZuncha全体のClock注入方針（1-5のIsExpired、そらのTC-2-1-11指摘を受けた設計変更）に合わせ、
// 判定基準時刻nowを呼び出し元から明示的に受け取る。
func (m *mockConversationRepository) GC(ctx context.Context, now time.Time) (int64, error) {
	args := m.Called(ctx, now)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockConversationRepository) InsertConversation(ctx context.Context, conv *model.Conversation) error {
	args := m.Called(ctx, conv)
	return args.Error(0)
}

// SetFirstText は CreateConversationService からは呼ばれない（W-07の呼び出し側で使う）。
// そらの指摘: m.Called() を通さないスタブは AssertNotCalled や呼び出し順序の検証で
// 静かに偽陽性になる（testify は呼び出しを記録できないため「呼ばれていない」と判定する）。
// W-07 の「最初のユーザー発話のみ記録する = 2回目は呼ばない」がまさにその当たり所なので、
// 呼ばれない現時点でも m.Called() 形式にしておく。
func (m *mockConversationRepository) SetFirstText(ctx context.Context, conversationID, text string) error {
	args := m.Called(ctx, conversationID, text)
	return args.Error(0)
}

// Exists は CreateConversationService からは呼ばれない（W-06のハンドラで404判定に使う）。
func (m *mockConversationRepository) Exists(ctx context.Context, conversationID string) (bool, error) {
	args := m.Called(ctx, conversationID)
	return args.Bool(0), args.Error(1)
}

var _ repository.ConversationRepository = (*mockConversationRepository)(nil)

func TestCreateConversation(t *testing.T) {
	t.Run("TC-2-1-01_GC成功時に新規会話が作成されGC・Insertが1回ずつ呼ばれる", func(t *testing.T) {
		repo := new(mockConversationRepository)
		repo.On("GC", mock.Anything, mock.AnythingOfType("time.Time")).Return(int64(0), nil)
		repo.On("InsertConversation", mock.Anything, mock.AnythingOfType("*model.Conversation")).Return(nil)

		svc := service.NewCreateConversationService(repo)
		conv, err := svc.CreateConversation(context.Background())

		require.NoError(t, err)
		assert.NotNil(t, conv)
		repo.AssertNumberOfCalls(t, "GC", 1)
		repo.AssertNumberOfCalls(t, "InsertConversation", 1)
	})

	t.Run("TC-2-1-02_GC対象なしでも新規会話作成が成功する", func(t *testing.T) {
		repo := new(mockConversationRepository)
		repo.On("GC", mock.Anything, mock.AnythingOfType("time.Time")).Return(int64(0), nil)
		repo.On("InsertConversation", mock.Anything, mock.AnythingOfType("*model.Conversation")).Return(nil)

		svc := service.NewCreateConversationService(repo)
		_, err := svc.CreateConversation(context.Background())

		assert.NoError(t, err)
	})

	t.Run("TC-2-1-04_GC失敗時もエラーを握りつぶし新規会話作成は成功する", func(t *testing.T) {
		repo := new(mockConversationRepository)
		repo.On("GC", mock.Anything, mock.AnythingOfType("time.Time")).Return(int64(0), errors.New("gc deadlock"))
		repo.On("InsertConversation", mock.Anything, mock.AnythingOfType("*model.Conversation")).Return(nil)

		svc := service.NewCreateConversationService(repo)
		conv, err := svc.CreateConversation(context.Background())

		assert.NoError(t, err)
		assert.NotNil(t, conv)
		repo.AssertCalled(t, "InsertConversation", mock.Anything, mock.AnythingOfType("*model.Conversation"))
	})

	t.Run("TC-2-1-05_Insert失敗時はエラーを返す", func(t *testing.T) {
		repo := new(mockConversationRepository)
		repo.On("GC", mock.Anything, mock.AnythingOfType("time.Time")).Return(int64(0), nil)
		repo.On("InsertConversation", mock.Anything, mock.AnythingOfType("*model.Conversation")).Return(errors.New("db connection lost"))

		svc := service.NewCreateConversationService(repo)
		conv, err := svc.CreateConversation(context.Background())

		assert.Error(t, err)
		assert.Nil(t, conv)
	})
}

func TestCreateConversation_呼び出しのたびに毎回GCが1回呼ばれる(t *testing.T) {
	// TC-2-1-03
	repo := new(mockConversationRepository)
	repo.On("GC", mock.Anything, mock.AnythingOfType("time.Time")).Return(int64(0), nil)
	repo.On("InsertConversation", mock.Anything, mock.AnythingOfType("*model.Conversation")).Return(nil)

	svc := service.NewCreateConversationService(repo)
	for i := 0; i < 3; i++ {
		_, err := svc.CreateConversation(context.Background())
		require.NoError(t, err)
	}

	repo.AssertNumberOfCalls(t, "GC", 3)
}

func TestCreateConversation_GCがInsertConversationより先に呼ばれる(t *testing.T) {
	// TC-2-1-06
	repo := new(mockConversationRepository)
	var callOrder []string
	repo.On("GC", mock.Anything, mock.AnythingOfType("time.Time")).Run(func(args mock.Arguments) {
		callOrder = append(callOrder, "GC")
	}).Return(int64(0), nil)
	repo.On("InsertConversation", mock.Anything, mock.AnythingOfType("*model.Conversation")).Run(func(args mock.Arguments) {
		callOrder = append(callOrder, "InsertConversation")
	}).Return(nil)

	svc := service.NewCreateConversationService(repo)
	_, err := svc.CreateConversation(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []string{"GC", "InsertConversation"}, callOrder)
}

func TestCreateConversation_連続呼び出しで異なるULIDが採番される(t *testing.T) {
	// TC-2-1-07
	repo := new(mockConversationRepository)
	var insertedIDs []string
	repo.On("GC", mock.Anything, mock.AnythingOfType("time.Time")).Return(int64(0), nil)
	repo.On("InsertConversation", mock.Anything, mock.AnythingOfType("*model.Conversation")).Run(func(args mock.Arguments) {
		conv := args.Get(1).(*model.Conversation)
		insertedIDs = append(insertedIDs, conv.ID)
	}).Return(nil)

	svc := service.NewCreateConversationService(repo)
	_, err1 := svc.CreateConversation(context.Background())
	_, err2 := svc.CreateConversation(context.Background())

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.Len(t, insertedIDs, 2)
	assert.NotEqual(t, insertedIDs[0], insertedIDs[1])
}
