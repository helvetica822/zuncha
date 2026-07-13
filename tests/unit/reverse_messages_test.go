// 対応仕様: docs/03_unit_test/08_test_specification.md 4.2 純粋ロジック層（観点2-2、TC-2-2-01〜04）
package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"zuncha/internal/model"
	"zuncha/internal/repository"
)

func newMessage(id string, createdAt time.Time) model.Message {
	return model.Message{
		ID:             id,
		ConversationID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Role:           "user",
		Content:        "テストメッセージ" + id,
		CreatedAt:      createdAt,
	}
}

func messageIDs(messages []model.Message) []string {
	ids := make([]string, len(messages))
	for i, m := range messages {
		ids[i] = m.ID
	}
	return ids
}

func TestReverseMessages(t *testing.T) {
	base := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

	t.Run("TC-2-2-01_新しい順の5件を古い順に反転する", func(t *testing.T) {
		newestFirst := []model.Message{
			newMessage("m5", base.Add(4*time.Minute)),
			newMessage("m4", base.Add(3*time.Minute)),
			newMessage("m3", base.Add(2*time.Minute)),
			newMessage("m2", base.Add(1*time.Minute)),
			newMessage("m1", base),
		}

		got := repository.ReverseMessages(newestFirst)

		assert.Equal(t, []string{"m1", "m2", "m3", "m4", "m5"}, messageIDs(got))
	})

	t.Run("TC-2-2-02_空スライスは空スライスのまま返す", func(t *testing.T) {
		got := repository.ReverseMessages([]model.Message{})

		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("TC-2-2-03_1件のみはそのまま返す", func(t *testing.T) {
		only := []model.Message{newMessage("m1", base)}

		got := repository.ReverseMessages(only)

		assert.Equal(t, only, got)
	})

	t.Run("TC-2-2-04_20件でも正しく反転される", func(t *testing.T) {
		const size = 20
		newestFirst := make([]model.Message, size)
		wantIDs := make([]string, size)
		for i := 0; i < size; i++ {
			id := "m" + string(rune('a'+i))
			// i=0が最新（base+19分）、i=19が最古（base+0分）の「新しい→古い」順で並べる
			newestFirst[i] = newMessage(id, base.Add(time.Duration(size-1-i)*time.Minute))
			// 反転後は古い→新しい順になるため、期待値は入力の逆順
			wantIDs[size-1-i] = id
		}

		got := repository.ReverseMessages(newestFirst)

		assert.Equal(t, wantIDs, messageIDs(got))
	})
}
