package lambda_test

// 検証対象: docs/specs/workmonth-implementation-design.md AC-12-17 ②
// （AC-10-17「イベントアダプタ」）。
//
// 手で組んだ Function URL のイベント（request_test.go と同じ形。aws-lambda-go の
// **イベント型は使うが、ランタイムを起動しない**＝lambda.Start を呼ばない）と、
// 受け取った要求を記録して固定の応答を書くだけの手書きの http.Handler を渡し、
// 次の3つを固定する。
//
//	(i)   ハンドラが書いたステータス・ヘッダ・本体が応答へ写る（base64 で
//	      包まれない）。
//	(ii)  渡した context.Context がハンドラの受け取る要求に載っている
//	      （値を載せた ctx を渡し、ハンドラ側で取り出して確認する）。
//	(iii) 変換に失敗する要求ではハンドラが1回も呼ばれず、応答が契約 AC-9-1 の
//	      形で `code` が INTERNAL_ERROR（AC-11-11）であり、ランタイムへ
//	      エラーを返さない。
//
// (i) と (iii) を対にして置く（対にしないと「常に INTERNAL_ERROR を返す」実装が
// Green になる）。
//
// 担保しないもの: Lambda ランタイムそのもの（登録した値が実際に呼ばれること・
// aws-lambda-go の挙動・実イベントの形）は観測しない（AC-13-24 ④）。

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"github.com/h-k741953/effort-tracker/services/api/internal/driver/lambda"
)

// eventCtxKey は (ii) の検査で ctx に載せる値のキー（テスト専用の型）。
type eventCtxKey struct{}

// recordingHandler は受け取った要求を記録して固定の応答を書くだけの手書きの
// http.Handler（AC-12-17 ②）。
type recordingHandler struct {
	// 記録
	calls    int
	method   string
	path     string
	rawQuery string
	body     string
	ctxValue any

	// 固定の応答
	status      int
	respHeaders map[string]string
	respBody    string
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.calls++
	h.method = r.Method
	h.path = r.URL.Path
	h.rawQuery = r.URL.RawQuery
	h.ctxValue = r.Context().Value(eventCtxKey{})

	if r.Body != nil {
		var sb strings.Builder
		buf := make([]byte, 512)
		for {
			n, err := r.Body.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		h.body = sb.String()
	}

	for name, value := range h.respHeaders {
		w.Header().Set(name, value)
	}
	w.WriteHeader(h.status)
	_, _ = w.Write([]byte(h.respBody))
}

// TestNewEventHandler_CopiesResponseAndCarriesContext は AC-12-17 ② の (i)(ii)。
func TestNewEventHandler_CopiesResponseAndCarriesContext(t *testing.T) {
	const (
		wantStatus = http.StatusConflict
		wantBody   = `{"error":{"code":"INVALID_STATE_FOR_APPROVE","message":"diagnostic"}}`
		wantCtx    = "ctx-value-for-event-adapter"
	)

	handler := &recordingHandler{
		status: wantStatus,
		respHeaders: map[string]string{
			"Content-Type":     "application/json",
			"X-Adapter-Marker": "copied",
		},
		respBody: wantBody,
	}

	event := events.LambdaFunctionURLRequest{
		RawPath:        "/work-months/ctr-adapter/2026-07/approve",
		RawQueryString: "",
		Headers: map[string]string{
			"x-actor-id":   "apr-adapter",
			"x-actor-role": "Approver",
		},
		RequestContext: events.LambdaFunctionURLRequestContext{
			HTTP: events.LambdaFunctionURLRequestContextHTTPDescription{
				Method: http.MethodPost,
			},
		},
	}

	ctx := context.WithValue(context.Background(), eventCtxKey{}, wantCtx)

	resp, err := lambda.NewEventHandler(handler)(ctx, event)
	if err != nil {
		t.Fatalf("イベントアダプタがランタイムへエラーを返した: %v（AC-10-17）", err)
	}

	if handler.calls != 1 {
		t.Fatalf("http.Handler の呼び出し回数 = %d, want 1", handler.calls)
	}

	// (i) ステータス・ヘッダ・本体が応答へ写る。
	if resp.StatusCode != wantStatus {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, wantStatus)
	}
	if resp.Body != wantBody {
		t.Errorf("Body = %q, want %q", resp.Body, wantBody)
	}
	if resp.IsBase64Encoded {
		t.Errorf("IsBase64Encoded = true, want false（本体はそのまま載せ base64 で包まない＝AC-10-17 ③）")
	}
	for name, want := range handler.respHeaders {
		got, ok := headerValue(resp.Headers, name)
		if !ok {
			t.Errorf("応答ヘッダ %q が写っていない（AC-10-17 ③）", name)
			continue
		}
		if got != want {
			t.Errorf("応答ヘッダ %q = %q, want %q", name, got, want)
		}
	}

	// (ii) 渡した ctx がハンドラの受け取る要求に載っている。
	if handler.ctxValue != wantCtx {
		t.Errorf("ハンドラが受け取った要求の ctx の値 = %#v, want %q（AC-10-17 ②）", handler.ctxValue, wantCtx)
	}

	// 変換そのもの（AC-10-1）を再定義しない範囲で、要求が渡っていることだけ
	// 確認する（変換の規則自体は request_test.go が固定済み）。
	if handler.method != http.MethodPost {
		t.Errorf("ハンドラが受け取ったメソッド = %q, want %q", handler.method, http.MethodPost)
	}
	if handler.path != event.RawPath {
		t.Errorf("ハンドラが受け取ったパス = %q, want %q", handler.path, event.RawPath)
	}
}

// TestNewEventHandler_ConversionFailure は AC-12-17 ② の (iii)。
// 変換に失敗する要求（IsBase64Encoded が真なのにボディが base64 として不正。
// request_test.go の TestToHTTPRequest_DecodeErrors と同じもの）では、
// ハンドラが1回も呼ばれず、契約 AC-9-1 の形で INTERNAL_ERROR を返し、
// ランタイムへはエラーを返さない。
func TestNewEventHandler_ConversionFailure(t *testing.T) {
	handler := &recordingHandler{
		status:      http.StatusOK,
		respHeaders: map[string]string{"Content-Type": "application/json"},
		respBody:    `{"contractId":"ctr-should-not-be-called"}`,
	}

	event := events.LambdaFunctionURLRequest{
		RawPath:         "/work-months/ctr-adapter/2026-07/close",
		Body:            "!!!",
		IsBase64Encoded: true,
		RequestContext: events.LambdaFunctionURLRequestContext{
			HTTP: events.LambdaFunctionURLRequestContextHTTPDescription{
				Method: http.MethodPost,
			},
		},
	}

	resp, err := lambda.NewEventHandler(handler)(context.Background(), event)
	if err != nil {
		t.Fatalf("イベントアダプタがランタイムへエラーを返した: %v（AC-10-17。返すと契約 AC-9-1 の形でない応答になる）", err)
	}

	if handler.calls != 0 {
		t.Errorf("http.Handler の呼び出し回数 = %d, want 0（変換に失敗した要求では呼ばない＝AC-10-17 ④）", handler.calls)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want %d（INTERNAL_ERROR＝契約 AC-9）", resp.StatusCode, http.StatusInternalServerError)
	}
	if resp.IsBase64Encoded {
		t.Errorf("IsBase64Encoded = true, want false")
	}

	var body any
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("応答ボディの JSON デコードに失敗した: %v（body=%s）", err, resp.Body)
	}
	assertErrorBody(t, body, "INTERNAL_ERROR")
}

// headerValue は応答ヘッダを名前の大文字小文字を区別せずに引く。ヘッダ名の
// 正規化の仕方を本テストは固定しない（HTTP のヘッダ名は大文字小文字を
// 区別しない）。
func headerValue(headers map[string]string, name string) (string, bool) {
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return "", false
}
