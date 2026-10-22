package port

import (
	"context"

	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
)

// WorkTrendQuery は稼働の推移（E-9）の参照ポート（実装設計 AC-6-9・AC-6-11）。
// ListWorkTrends だけが使う。Reconstruct で復元した集約を、契約識別子の昇順・
// 同じ契約の中は年月の昇順で返す。
type WorkTrendQuery interface {
	ListByEngineer(ctx context.Context, engineerID string) ([]*workmonth.WorkMonth, error)
}
