package controller_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// テストダブルはすべて手書きのインメモリ spy とする
// （ADR 0007・docs/specs/workmonth-implementation-design.md AC-12-2・AC-12-3・AC-12-9）。
// モックライブラリは使わない。

// ---- 操作者ヘッダ ---------------------------------------------------------

// 契約 D-2・AC-1-4 のヘッダ名。
const (
	headerActorID   = "X-Actor-Id"
	headerActorRole = "X-Actor-Role"
)

// ---- 呼び出し先（invoker）の spy -------------------------------------------

// invokerSpy は controller が宣言する最小の invoker interface（AC-9-8-a）に対する
// 手書きの spy。受け取った入力 DTO を記録するだけで、他のことをしない
// （AC-9-8-c・AC-12-9）。すべてのハンドラの invoker が
// 「入力 DTO を1つ受け取り、戻り値を返さない」形（AC-9-8-a）で揃っているため、
// 型引数で共用する。
type invokerSpy[T any] struct {
	calls []T
}

func (s *invokerSpy[T]) Execute(_ context.Context, input T) {
	s.calls = append(s.calls, input)
}

// onlyCall は Execute がちょうど1回呼ばれたことを確認して、その入力を返す。
func (s *invokerSpy[T]) onlyCall(t *testing.T) T {
	t.Helper()
	if len(s.calls) != 1 {
		t.Fatalf("invoker の呼び出し回数 = %d, want 1（AC-9-8-c）: %+v", len(s.calls), s.calls)
	}
	return s.calls[0]
}

// wantNoCall は Execute が1度も呼ばれていないことを確認する（AC-12-9「controller が
// 弾いた要求では spy が1回も呼ばれていない」）。
func (s *invokerSpy[T]) wantNoCall(t *testing.T) {
	t.Helper()
	if len(s.calls) != 0 {
		t.Fatalf("invoker が呼ばれてはいけないのに呼ばれた（回数 = %d）: %+v", len(s.calls), s.calls)
	}
}

// ---- errorPresenter の spy --------------------------------------------------

// errorPresenterSpy は controller が早期に失敗を報告する際に呼ぶ最小の
// interface（PresentError のみ。AC-9-7-a・decision 11）に対する手書きの spy。
type errorPresenterSpy struct {
	errs []error
}

func (s *errorPresenterSpy) PresentError(err error) {
	s.errs = append(s.errs, err)
}

// onlyErr は PresentError がちょうど1回呼ばれたことを確認して、そのエラーを返す。
func (s *errorPresenterSpy) onlyErr(t *testing.T) error {
	t.Helper()
	if len(s.errs) != 1 {
		t.Fatalf("PresentError の呼び出し回数 = %d, want 1: %v", len(s.errs), s.errs)
	}
	return s.errs[0]
}

// wantNoErr は PresentError が1度も呼ばれていないことを確認する（要求が
// 弾かれずに invoker まで到達したことの裏返し）。
func (s *errorPresenterSpy) wantNoErr(t *testing.T) {
	t.Helper()
	if len(s.errs) != 0 {
		t.Fatalf("PresentError が呼ばれてはいけないのに呼ばれた（回数 = %d）: %v", len(s.errs), s.errs)
	}
}

// ---- リクエストの組み立て ---------------------------------------------------

// pathValue はパスパターンの1変数（名前・値）を表す。httptest.NewRequest は
// 標準 net/http のパスパターン一致を行わないため、driver/lambda のルーティング
// （AC-10-1）を経ずに r.SetPathValue で直接与える。ルーティングそのものの
// 検査は本 AC の対象外（AC-13-19）であり、controller 単体の検査に閉じる
// （AC-12-5「Lambda を起動しない」）。
type pathValue struct {
	name  string
	value string
}

// newRequest は httptest で HTTP リクエストを組み立てる（AC-12-5）。
func newRequest(method, target string, body []byte, headers map[string]string, paths ...pathValue) *http.Request {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, target, bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	for _, p := range paths {
		r.SetPathValue(p.name, p.value)
	}
	return r
}

// actorHeaders は操作者ヘッダ2本を組み立てる（契約 AC-1-4）。
func actorHeaders(id, role string) map[string]string {
	return map[string]string{headerActorID: id, headerActorRole: role}
}
