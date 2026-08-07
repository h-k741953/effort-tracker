package lambda

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"

	"github.com/aws/aws-lambda-go/events"
)

// ToHTTPRequest は Lambda Function URL のイベント（payload format 2.0）を
// *http.Request 相当へ変換する（AC-10-1）。メソッド・パス・クエリ文字列・
// ヘッダー・ボディ（IsBase64Encoded のデコードを含む）を保つ。
//
// 標準 net/http のルーティングでの振り分けと ViewModel の JSON 応答は
// この関数の射程外（AC-13-19。DI 配線は別途 driver/lambda が担う）。
func ToHTTPRequest(event events.LambdaFunctionURLRequest) (*http.Request, error) {
	body, err := decodeBody(event)
	if err != nil {
		return nil, err
	}

	// RawPath は URL エンコード済みの表現（AWS Lambda Function URL の
	// payload format 2.0）。url.URL{Path: ...} はデコード済み表現を期待する
	// ため、url.Parse でエンコード・デコード双方（Path / RawPath）を
	// 正しく分離させる（AC-10-1）。
	u, err := url.Parse(event.RawPath)
	if err != nil {
		return nil, fmt.Errorf("function URL イベントの RawPath の解析に失敗した: %w", err)
	}
	u.RawQuery = event.RawQueryString

	// http.NewRequest はメソッドが空文字なら "GET" にフォールバックするが、
	// Function URL イベントの RequestContext.HTTP.Method は常に設定される
	// ため、この関数の射程では到達しない。
	req, err := http.NewRequest(event.RequestContext.HTTP.Method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("function URL イベントの *http.Request への変換に失敗した: %w", err)
	}

	for name, value := range event.Headers {
		req.Header.Set(name, value)
	}

	// event.Cookies（payload format 2.0 の cookie 配列）は意図的に捨てる。
	// 契約（domain-api-http-contract.md D-2／AC-1-4／AC-1-6）は操作者の伝達を
	// X-Actor-Id / X-Actor-Role ヘッダーで行い、cookie を使わない。

	return req, nil
}

// decodeBody はイベントのボディを、IsBase64Encoded が真の場合はデコードして
// 返す（AC-10-1）。
func decodeBody(event events.LambdaFunctionURLRequest) ([]byte, error) {
	if event.Body == "" {
		return nil, nil
	}

	if !event.IsBase64Encoded {
		return []byte(event.Body), nil
	}

	decoded, err := base64.StdEncoding.DecodeString(event.Body)
	if err != nil {
		return nil, fmt.Errorf("リクエストボディの base64 デコードに失敗した: %w", err)
	}

	return decoded, nil
}
