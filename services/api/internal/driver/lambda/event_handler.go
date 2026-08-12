package lambda

import (
	"context"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/presenter"
)

// 本ファイルは AC-10-17「イベントアダプタ」を実装する（AC-10-1 の残り半分。
// テストは AC-12-17 ②）。行うのは次の4つだけである。
//
//	① イベント → *http.Request 相当への変換（ToHTTPRequest＝AC-10-1。変換の
//	   規則を再定義しない）。
//	② ランタイムから受け取った context.Context を要求へ載せる（Query / Exec /
//	   Begin が ctx を受け取る＝AC-9-14-e ため、載せないと取消し・期限が SQL
//	   実行へ伝わらない）。
//	③ 受け取った http.Handler を呼び、書かれたステータス・ヘッダ・本体を
//	   応答へ写す（本体はそのまま載せ、base64 で包まない）。
//	④ 変換に失敗した要求では http.Handler を呼ばず、応答を契約 AC-9-1 の形で
//	   返す。
//
// `code`／ステータスの対応表は自ら持たない（AC-11-10）。変換の失敗は presenter
// の対応表に無いエラーとして渡し、AC-11-11 の「いずれにも該当しないエラー」＝
// INTERNAL_ERROR として写る（notFoundHandler・writeResultOrDelegate が既に
// 採っているのと同じ形）。
//
// ランタイムへエラーを返さない（返すとランタイム側が契約 AC-9-1 の形でない
// 応答を生成し、呼び出し側＝BFF が契約どおりに読めなくなる）。
//
// DB もランタイムも起動しない（aws-lambda-go のイベント型は使うが、登録・実行は
// しない。登録はエントリポイント＝AC-10-15 ⑤ の責務）。

// EventHandler は Lambda ランタイムへ登録できる形（Function URL のイベントを
// 受けて応答を返す関数）である（AC-10-17）。
type EventHandler func(ctx context.Context, event events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error)

// NewEventHandler は http.Handler を受け取り、EventHandler を返す（AC-10-17）。
func NewEventHandler(h http.Handler) EventHandler {
	return func(ctx context.Context, event events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
		rec := newResponseRecorder()

		req, err := ToHTTPRequest(event)
		if err != nil {
			// ④ 変換に失敗した要求ではハンドラを呼ばない。`code`／ステータスは
			// presenter へ委ねる（AC-11-10・AC-11-11）。
			writeConversionFailure(rec, err)
			return rec.response(), nil
		}

		// ② ランタイムの ctx を要求へ載せる。
		h.ServeHTTP(rec, req.WithContext(ctx))

		return rec.response(), nil
	}
}

// writeConversionFailure は変換の失敗を契約 AC-9-1 の形の応答として書く。
// driver/lambda は対応表を持たず、presenter の対応表に無いエラーとして渡す
// （AC-11-10・AC-11-11）。PresentError は必ず result を設定するため、result の
// 有無を判定する分岐は持たない（notFoundHandler と同じ形）。
func writeConversionFailure(w http.ResponseWriter, err error) {
	out := presenter.NewWorkMonthPresenter()
	out.PresentError(err)
	result, _ := out.Result()
	writeResult(w, result)
}

// responseRecorder は http.Handler が書いた応答を記録する http.ResponseWriter で
// ある。Function URL の応答へ写すために要る最小限（ステータス・ヘッダ・本体）
// だけを保持する（AC-10-17 ③）。
type responseRecorder struct {
	header      http.Header
	status      int
	wroteHeader bool
	body        strings.Builder
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		header: make(http.Header),
		// WriteHeader を呼ばずに書き込まれた場合の既定（net/http と同じ）。
		status: http.StatusOK,
	}
}

func (r *responseRecorder) Header() http.Header { return r.header }

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.body.Write(b)
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = status
}

// response は記録した応答を Function URL の応答へ写す（AC-10-17 ③）。
// 本体はそのまま載せ、base64 で包まない。
func (r *responseRecorder) response() events.LambdaFunctionURLResponse {
	headers := make(map[string]string, len(r.header))
	for name, values := range r.header {
		// HTTP のヘッダは同名で複数回書けるが、Function URL の応答は
		// 名前ごとに1つの値を持つ。契約（AC-9-1・AC-10-1・AC-10-2）が同名の
		// 複数値を要さないため、標準の表現（カンマ区切り）で1つにまとめる。
		headers[name] = strings.Join(values, ", ")
	}

	return events.LambdaFunctionURLResponse{
		StatusCode:      r.status,
		Headers:         headers,
		Body:            r.body.String(),
		IsBase64Encoded: false,
	}
}
