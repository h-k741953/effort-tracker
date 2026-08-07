package lambda_test

import (
	"encoding/base64"
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/driver/lambda"
)

// 検証対象の受け入れ条件:
//   - docs/specs/workmonth-implementation-design.md AC-10-1（driver/lambda の
//     責務のうち、Function URL のイベントを *http.Request 相当へ変換する部分
//     のみ。標準 net/http のルーティングでの振り分けと ViewModel の JSON 応答は
//     対象外＝AC-13-19。ルーティング・DI 配線を検査するテストは本仕様が要求
//     しない）。
//   - AC-12-4（非公開フィールドの比較は公開アクセサ経由のビューで行う）。
//
// *http.Request は非公開フィールド（ctx 等）を持つため go-cmp で直接比較せず、
// clock_test.go の dateView と同じ流儀で、公開アクセサ経由の reqView を介して
// 比較する。
//
// 変換が保持すべき要素（メソッド・パス・クエリ文字列・ヘッダー・ボディ）は
// AC-10-1 の「*http.Request 相当へ変換」という文言と、Function URL イベント
// （payload format 2.0）自体の技術的な対応関係から一意に定まる（業務ルールを
// 含まない）。パス変数の解決（r.PathValue）は標準 net/http のルーティングを
// 通って初めて設定されるため、本テストの射程外（AC-13-19）。

// reqView は *http.Request の非公開フィールドを避けて比較するための公開
// アクセサ経由のビュー（AC-12-4）。
type reqView struct {
	Method   string
	Path     string
	RawQuery string
	Header   http.Header
	Body     string
}

func viewOfRequest(t *testing.T, r *http.Request) reqView {
	t.Helper()

	body := ""
	if r.Body != nil {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("リクエストボディの読み取りに失敗した: %v", err)
		}
		body = string(b)
	}

	return reqView{
		Method:   r.Method,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
		Header:   r.Header,
		Body:     body,
	}
}

// TestToHTTPRequest は lambda.ToHTTPRequest が Function URL のイベントを
// *http.Request 相当へ変換することを検証する（AC-10-1）。
func TestToHTTPRequest(t *testing.T) {
	tests := []struct {
		name  string
		event events.LambdaFunctionURLRequest
		want  reqView
	}{
		{
			name: "GETの一覧はメソッド・パス・クエリ文字列・ヘッダーがそのまま渡る",
			event: events.LambdaFunctionURLRequest{
				RawPath:        "/work-months",
				RawQueryString: "engineerId=e-1&limit=20",
				Headers: map[string]string{
					"X-Actor-Id":   "e-1",
					"X-Actor-Role": "Approver",
				},
				RequestContext: events.LambdaFunctionURLRequestContext{
					HTTP: events.LambdaFunctionURLRequestContextHTTPDescription{
						Method: "GET",
					},
				},
			},
			want: reqView{
				Method:   "GET",
				Path:     "/work-months",
				RawQuery: "engineerId=e-1&limit=20",
				Header: http.Header{
					"X-Actor-Id":   []string{"e-1"},
					"X-Actor-Role": []string{"Approver"},
				},
				Body: "",
			},
		},
		{
			name: "PUTのボディとContent-Typeがそのまま渡る",
			event: events.LambdaFunctionURLRequest{
				RawPath: "/work-months/c-1/2026-08/daily-records/2026-08-05",
				Headers: map[string]string{
					"Content-Type": "application/json",
					"X-Actor-Id":   "e-1",
					"X-Actor-Role": "Engineer",
				},
				Body: `{"workingHours":8}`,
				RequestContext: events.LambdaFunctionURLRequestContext{
					HTTP: events.LambdaFunctionURLRequestContextHTTPDescription{
						Method: "PUT",
					},
				},
			},
			want: reqView{
				Method:   "PUT",
				Path:     "/work-months/c-1/2026-08/daily-records/2026-08-05",
				RawQuery: "",
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"X-Actor-Id":   []string{"e-1"},
					"X-Actor-Role": []string{"Engineer"},
				},
				Body: `{"workingHours":8}`,
			},
		},
		{
			name: "IsBase64Encodedのボディがデコードされて渡る",
			event: events.LambdaFunctionURLRequest{
				RawPath:         "/work-months/c-1/2026-08/close",
				Body:            base64.StdEncoding.EncodeToString([]byte(`{}`)),
				IsBase64Encoded: true,
				RequestContext: events.LambdaFunctionURLRequestContext{
					HTTP: events.LambdaFunctionURLRequestContextHTTPDescription{
						Method: "POST",
					},
				},
			},
			want: reqView{
				Method:   "POST",
				Path:     "/work-months/c-1/2026-08/close",
				RawQuery: "",
				Header:   http.Header{},
				Body:     `{}`,
			},
		},
		{
			name: "ヘッダー・クエリ・ボディが無いGETはゼロ値のまま渡る",
			event: events.LambdaFunctionURLRequest{
				RawPath: "/work-months/c-1/2026-08",
				RequestContext: events.LambdaFunctionURLRequestContext{
					HTTP: events.LambdaFunctionURLRequestContextHTTPDescription{
						Method: "GET",
					},
				},
			},
			want: reqView{
				Method:   "GET",
				Path:     "/work-months/c-1/2026-08",
				RawQuery: "",
				Header:   http.Header{},
				Body:     "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lambda.ToHTTPRequest(tt.event)
			if err != nil {
				t.Fatalf("ToHTTPRequest が予期せぬエラーを返した: %v", err)
			}

			if diff := cmp.Diff(tt.want, viewOfRequest(t, got)); diff != "" {
				t.Errorf("ToHTTPRequest の変換結果が不一致 (-want +got):\n%s（AC-10-1）", diff)
			}
		})
	}
}
