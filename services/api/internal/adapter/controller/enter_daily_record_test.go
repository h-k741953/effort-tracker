package controller_test

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/controller"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// 検証対象の受け入れ条件: AC-9-5-c・AC-9-6（a〜i）・AC-9-7-a①・AC-9-8・AC-12-9・決定10。
// 契約 AC-1-2（Content-Type）・AC-4-5・AC-4-9-a〜4-9-j・AC-4-10・AC-4-10-a・AC-4-10-b・AC-4-13-a。
// 実装設計 AC-12-18 ③・AC-9-6-e・AC-9-6-g。リクエストボディの形は契約 AC-4（2026-10-15 改定）。

// enterTarget は httptest.NewRequest 用の URL を組み立てる（closeTarget と同じ
// 理由でエスケープする）。
func enterTarget(contractID, yearMonth, date string) string {
	return "/work-months/" + url.PathEscape(contractID) + "/" + url.PathEscape(yearMonth) + "/daily-records/" + url.PathEscape(date)
}

func enterPaths(contractID, yearMonth, date string) []pathValue {
	return []pathValue{
		{name: "contractId", value: contractID},
		{name: "yearMonth", value: yearMonth},
		{name: "date", value: date},
	}
}

func jsonHeaders(id, role string) map[string]string {
	h := actorHeaders(id, role)
	h["Content-Type"] = "application/json"
	return h
}

// validBody は契約 AC-4 のリクエストボディの例（daily-record-entry.md AC-2-6-a の入力）。
const validBody = `{
  "startTime": {"hour": 9, "minute": 15},
  "endTime": {"hour": 17, "minute": 45},
  "break": {"start": {"hour": 12, "minute": 30}, "end": {"hour": 13, "minute": 15}}
}`

// wantInput027 は 2026-07-01 の E-3 の要求から期待される入力 DTO のうち、時刻以外の部分。
func baseInput(t *testing.T) port.EnterDailyRecordInput {
	t.Helper()
	return port.EnterDailyRecordInput{
		Actor:      port.Actor{ID: testActorID, Role: port.RoleEngineer, Authenticated: true},
		ContractID: mustContractID(t, testContractID),
		YearMonth:  mustYearMonth(t, 2026, 7),
		Date:       mustDate(t, 2026, 7, 1),
	}
}

var enterCmpOpts = cmp.AllowUnexported(workmonth.ContractID{}, workmonth.YearMonth{}, workmonth.Date{})

// runEnter は body を E-3 へ渡し、invoker と errorPresenter の spy を返す。
func runEnter(t *testing.T, body string) (*invokerSpy[port.EnterDailyRecordInput], *errorPresenterSpy) {
	t.Helper()
	invoker := &invokerSpy[port.EnterDailyRecordInput]{}
	output := &errorPresenterSpy{}
	r := newRequest(
		http.MethodPut, enterTarget(testContractID, "2026-07", "2026-07-01"), []byte(body),
		jsonHeaders(testActorID, string(port.RoleEngineer)),
		enterPaths(testContractID, "2026-07", "2026-07-01")...,
	)
	controller.HandleEnterDailyRecord(r, invoker, output)
	return invoker, output
}

// TestHandleEnterDailyRecord_MapsInput は妥当な要求から
// port.EnterDailyRecordInput（AC-9-5-c）がちょうど1回 invoker へ渡ることを検証する。
// 時刻は素の整数のまま写す（AC-9-6-e）。休憩の有無は break が null かオブジェクトかで写す
// （契約 D-15。休憩なしは Break が nil、休憩ありは非 nil）。
func TestHandleEnterDailyRecord_MapsInput(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantStart port.TimeOfDay
		wantEnd   port.TimeOfDay
		wantBreak *port.Break
	}{
		{
			name:      "休憩あり（daily-record-entry.md AC-2-6-a の入力）",
			body:      validBody,
			wantStart: port.TimeOfDay{Hour: 9, Minute: 15},
			wantEnd:   port.TimeOfDay{Hour: 17, Minute: 45},
			wantBreak: &port.Break{Start: port.TimeOfDay{Hour: 12, Minute: 30}, End: port.TimeOfDay{Hour: 13, Minute: 15}},
		},
		{
			name:      `"break": null は休憩なし（Break が nil。AC-2-6-b の入力）`,
			body:      `{"startTime":{"hour":9,"minute":0},"endTime":{"hour":18,"minute":0},"break":null}`,
			wantStart: port.TimeOfDay{Hour: 9, Minute: 0},
			wantEnd:   port.TimeOfDay{Hour: 18, Minute: 0},
			wantBreak: nil,
		},
		{
			// AC-12-18 ③: 0:00〜0:15 に当たる休憩のオブジェクトが非 nil になること。
			// 休憩なし（nil）を時刻のゼロ値で表すと、この入力と区別できなくなる。
			name:      "0:00〜0:15 の休憩のオブジェクトは非 nil（AC-2-6-c の入力。休憩なしと取り違えない）",
			body:      `{"startTime":{"hour":0,"minute":0},"endTime":{"hour":23,"minute":45},"break":{"start":{"hour":0,"minute":0},"end":{"hour":0,"minute":15}}}`,
			wantStart: port.TimeOfDay{Hour: 0, Minute: 0},
			wantEnd:   port.TimeOfDay{Hour: 23, Minute: 45},
			wantBreak: &port.Break{Start: port.TimeOfDay{Hour: 0, Minute: 0}, End: port.TimeOfDay{Hour: 0, Minute: 15}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker, output := runEnter(t, tt.body)

			output.wantNoErr(t)
			want := baseInput(t)
			want.StartTime, want.EndTime, want.Break = tt.wantStart, tt.wantEnd, tt.wantBreak
			got := invoker.onlyCall(t)
			if diff := cmp.Diff(want, got, enterCmpOpts); diff != "" {
				t.Errorf("invoker へ渡った入力 DTO が不一致 (-want +got):\n%s（AC-9-5-c・AC-9-6-e）", diff)
			}
		})
	}
}

// TestHandleEnterDailyRecord_PassesThroughOutOfRangeAndInvalidCombination は、整数として読める時刻が
// 値域外（契約 AC-4-5）でも、組み合わせが不正（契約 AC-4-5-a）でも、controller が弾かずに
// 素の整数のまま入力 DTO へ渡すことを検証する（AC-9-6-e。判定は順6＝domain が行う）。
// 型の不正（4-9-g・4-9-h）との境目は、次の RejectsMissingOrMistypedTimes と対になる。
func TestHandleEnterDailyRecord_PassesThroughOutOfRangeAndInvalidCombination(t *testing.T) {
	// 4つの時刻のどこに現れても弾かない（3-1-a・3-2-a・3-2-b・負の整数）
	positions := []struct {
		name  string
		build func(bad string) string
	}{
		{"startTime", func(bad string) string {
			return `{"startTime":` + bad + `,"endTime":{"hour":18,"minute":0},"break":{"start":{"hour":12,"minute":0},"end":{"hour":13,"minute":0}}}`
		}},
		{"endTime", func(bad string) string {
			return `{"startTime":{"hour":9,"minute":0},"endTime":` + bad + `,"break":{"start":{"hour":12,"minute":0},"end":{"hour":13,"minute":0}}}`
		}},
		{"break.start", func(bad string) string {
			return `{"startTime":{"hour":9,"minute":0},"endTime":{"hour":18,"minute":0},"break":{"start":` + bad + `,"end":{"hour":13,"minute":0}}}`
		}},
		{"break.end", func(bad string) string {
			return `{"startTime":{"hour":9,"minute":0},"endTime":{"hour":18,"minute":0},"break":{"start":{"hour":12,"minute":0},"end":` + bad + `}}`
		}},
	}
	bads := []struct {
		name string
		json string
		want port.TimeOfDay
	}{
		{"時が負（3-2-a）", `{"hour":-1,"minute":0}`, port.TimeOfDay{Hour: -1, Minute: 0}},
		{"時が 25（3-2-a）", `{"hour":25,"minute":0}`, port.TimeOfDay{Hour: 25, Minute: 0}},
		{"時が 24（3-2-b）", `{"hour":24,"minute":0}`, port.TimeOfDay{Hour: 24, Minute: 0}},
		{"分が粒度の外（3-1-a: 9:10）", `{"hour":9,"minute":10}`, port.TimeOfDay{Hour: 9, Minute: 10}},
		{"分が 60（3-1-a: 9:60）", `{"hour":9,"minute":60}`, port.TimeOfDay{Hour: 9, Minute: 60}},
		{"分が負（3-1-a）", `{"hour":9,"minute":-15}`, port.TimeOfDay{Hour: 9, Minute: -15}},
	}

	for _, pos := range positions {
		for _, bad := range bads {
			t.Run(pos.name+"に"+bad.name, func(t *testing.T) {
				invoker, output := runEnter(t, pos.build(bad.json))

				output.wantNoErr(t)
				got := invoker.onlyCall(t)
				var gotTod port.TimeOfDay
				switch pos.name {
				case "startTime":
					gotTod = got.StartTime
				case "endTime":
					gotTod = got.EndTime
				case "break.start":
					if got.Break == nil {
						t.Fatalf("Break = nil, want 非 nil")
					}
					gotTod = got.Break.Start
				case "break.end":
					if got.Break == nil {
						t.Fatalf("Break = nil, want 非 nil")
					}
					gotTod = got.Break.End
				}
				if gotTod != bad.want {
					t.Errorf("%s = %+v, want %+v（値域を検査せず素の整数のまま渡すこと。AC-9-6-e）", pos.name, gotTod, bad.want)
				}
			})
		}
	}

	combos := []struct {
		name string
		body string
	}{
		{"3-5-a: 終了＝開始", `{"startTime":{"hour":9,"minute":0},"endTime":{"hour":9,"minute":0},"break":null}`},
		{"3-5-b: 終了が開始より前", `{"startTime":{"hour":18,"minute":0},"endTime":{"hour":9,"minute":0},"break":null}`},
		{"3-6-b: 休憩終了が休憩開始より前", `{"startTime":{"hour":9,"minute":0},"endTime":{"hour":18,"minute":0},"break":{"start":{"hour":13,"minute":0},"end":{"hour":12,"minute":0}}}`},
		{"3-8-b: 休憩が勤務時間の外", `{"startTime":{"hour":9,"minute":0},"endTime":{"hour":18,"minute":0},"break":{"start":{"hour":19,"minute":0},"end":{"hour":20,"minute":0}}}`},
		{"3-9: 稼働 0", `{"startTime":{"hour":9,"minute":0},"endTime":{"hour":18,"minute":0},"break":{"start":{"hour":9,"minute":0},"end":{"hour":18,"minute":0}}}`},
	}
	for _, c := range combos {
		t.Run("組み合わせの不正も弾かない: "+c.name, func(t *testing.T) {
			invoker, output := runEnter(t, c.body)

			output.wantNoErr(t)
			invoker.onlyCall(t)
		})
	}
}

// TestHandleEnterDailyRecord_PassesThroughFutureDate は対象日が当日より後でも
// controller が弾かず、そのまま入力 DTO へ渡すことを検証する（AC-9-6-f。
// 未来日の判定は集約が「当日」を引数に受け取って行う）。
func TestHandleEnterDailyRecord_PassesThroughFutureDate(t *testing.T) {
	invoker := &invokerSpy[port.EnterDailyRecordInput]{}
	output := &errorPresenterSpy{}
	body := []byte(`{"startTime":{"hour":9,"minute":0},"endTime":{"hour":18,"minute":0},"break":null}`)
	r := newRequest(
		http.MethodPut, enterTarget(testContractID, "2099-07", "2099-07-01"), body,
		jsonHeaders(testActorID, string(port.RoleEngineer)),
		enterPaths(testContractID, "2099-07", "2099-07-01")...,
	)

	controller.HandleEnterDailyRecord(r, invoker, output)

	output.wantNoErr(t)
	got := invoker.onlyCall(t)
	wantDate := mustDate(t, 2099, 7, 1)
	if diff := cmp.Diff(wantDate, got.Date, cmp.AllowUnexported(workmonth.Date{})); diff != "" {
		t.Errorf("Date が不一致 (-want +got):\n%s（AC-9-6-f。未来日でも弾かずそのまま渡すこと）", diff)
	}
}

// TestHandleEnterDailyRecord_IgnoresUnknownBodyFields はボディの未知フィールドをエラーにせず
// 落とすことを検証する（AC-9-6-g。契約 AC-4-10）。入れ子（break・時刻オブジェクト）の中の
// 未知フィールドも同じ。2つ目の休憩に当たる値（契約 AC-4-10-a）と稼働時間（契約 AC-4-10-b）は、
// 値が時刻の値域外・組み合わせの不正であっても 400 にせず、入力 DTO に現れない。
func TestHandleEnterDailyRecord_IgnoresUnknownBodyFields(t *testing.T) {
	want := baseInput(t)
	want.StartTime = port.TimeOfDay{Hour: 9, Minute: 15}
	want.EndTime = port.TimeOfDay{Hour: 17, Minute: 45}
	want.Break = &port.Break{Start: port.TimeOfDay{Hour: 12, Minute: 30}, End: port.TimeOfDay{Hour: 13, Minute: 15}}

	const start = `"startTime":{"hour":9,"minute":15}`
	const end = `"endTime":{"hour":17,"minute":45}`
	const brk = `"break":{"start":{"hour":12,"minute":30},"end":{"hour":13,"minute":15}}`
	tests := []struct {
		name string
		body string
	}{
		{"トップレベルの未知フィールド", `{` + start + `,` + end + `,` + brk + `,"comment":"未知フィールド"}`},
		{"break の中の未知フィールド", `{` + start + `,` + end + `,"break":{"start":{"hour":12,"minute":30},"end":{"hour":13,"minute":15},"memo":"x"}}`},
		{"時刻オブジェクトの中の未知フィールド", `{"startTime":{"hour":9,"minute":15,"second":30},` + end + `,` + brk + `}`},
		{"4-10-a: break2（値は値域外・組み合わせ不正でも 400 にしない）", `{` + start + `,` + end + `,` + brk + `,"break2":{"start":{"hour":99,"minute":99},"end":{"hour":1,"minute":0}}}`},
		{"4-10-a: breaks（配列）", `{` + start + `,` + end + `,` + brk + `,"breaks":[{"start":{"hour":14,"minute":0},"end":{"hour":14,"minute":15}}]}`},
		{"4-10-b: 稼働時間", `{` + start + `,` + end + `,` + brk + `,"workingHours":{"hours":1,"minutes":0}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker, output := runEnter(t, tt.body)

			output.wantNoErr(t)
			got := invoker.onlyCall(t)
			if diff := cmp.Diff(want, got, enterCmpOpts); diff != "" {
				t.Errorf("入力 DTO が未知フィールドの影響を受けている (-want +got):\n%s（AC-9-6-g・契約 AC-4-10）", diff)
			}
		})
	}
}

// TestHandleEnterDailyRecord_RejectsMissingOrMistypedTimes は時刻・休憩の欠落と型不一致を
// 弾くことを検証する（AC-9-6-e「弾くのは欠落と型不一致だけ」。契約 AC-4-9-a〜4-9-j）。
// いずれも INVALID_REQUEST（port.ErrInvalidRequest）で、invoker は呼ばれない。
// 型の不正（4-9-g・4-9-h）と値域外の整数（4-5）の境目は PassesThroughOutOfRange... と対になる。
func TestHandleEnterDailyRecord_RejectsMissingOrMistypedTimes(t *testing.T) {
	const s = `"startTime":{"hour":9,"minute":0}`
	const e = `"endTime":{"hour":18,"minute":0}`
	const b = `"break":null`
	tests := []struct {
		name string
		body string
	}{
		// 4-9-a
		{"4-9-a: startTime のキーが無い", `{` + e + `,` + b + `}`},
		{"4-9-a: startTime が null", `{"startTime":null,` + e + `,` + b + `}`},
		{"4-9-a: endTime のキーが無い", `{` + s + `,` + b + `}`},
		{"4-9-a: endTime が null", `{` + s + `,"endTime":null,` + b + `}`},
		// 4-9-b
		{"4-9-b: break が {}", `{` + s + `,` + e + `,"break":{}}`},
		{"4-9-b: break.end のキーが無い", `{` + s + `,` + e + `,"break":{"start":{"hour":12,"minute":0}}}`},
		{"4-9-b: break.start のキーが無い", `{` + s + `,` + e + `,"break":{"end":{"hour":13,"minute":0}}}`},
		{"4-9-b: break.start が null", `{` + s + `,` + e + `,"break":{"start":null,"end":{"hour":13,"minute":0}}}`},
		{"4-9-b: break.end が null", `{` + s + `,` + e + `,"break":{"start":{"hour":12,"minute":0},"end":null}}`},
		// 4-9-c / D-18
		{"4-9-c: break のキーが無い（休憩なしとみなさない）", `{` + s + `,` + e + `}`},
		// 4-9-d
		{"4-9-d: break が false", `{` + s + `,` + e + `,"break":false}`},
		{"4-9-d: break が 0", `{` + s + `,` + e + `,"break":0}`},
		{"4-9-d: break が文字列", `{` + s + `,` + e + `,"break":"none"}`},
		{"4-9-d: break が空配列", `{` + s + `,` + e + `,"break":[]}`},
		{"4-9-d: break が休憩2つの配列", `{` + s + `,` + e + `,"break":[{"start":{"hour":12,"minute":0},"end":{"hour":12,"minute":15}},{"start":{"hour":15,"minute":0},"end":{"hour":15,"minute":15}}]}`},
		// 4-9-e
		{"4-9-e: startTime が文字列", `{"startTime":"9:15",` + e + `,` + b + `}`},
		{"4-9-e: endTime が数値", `{` + s + `,"endTime":1800,` + b + `}`},
		{"4-9-e: startTime が配列", `{"startTime":[9,15],` + e + `,` + b + `}`},
		{"4-9-e: break.start が文字列", `{` + s + `,` + e + `,"break":{"start":"12:00","end":{"hour":13,"minute":0}}}`},
		// 4-9-f
		{"4-9-f: startTime に minute が無い", `{"startTime":{"hour":9},` + e + `,` + b + `}`},
		{"4-9-f: startTime に hour が無い", `{"startTime":{"minute":0},` + e + `,` + b + `}`},
		{"4-9-f: hour が null", `{"startTime":{"hour":null,"minute":0},` + e + `,` + b + `}`},
		{"4-9-f: minute が null", `{"startTime":{"hour":9,"minute":null},` + e + `,` + b + `}`},
		{"4-9-f: break.end の minute が無い", `{` + s + `,` + e + `,"break":{"start":{"hour":12,"minute":0},"end":{"hour":13}}}`},
		// 4-9-g
		{"4-9-g: hour が文字列", `{"startTime":{"hour":"9","minute":0},` + e + `,` + b + `}`},
		{"4-9-g: minute が真偽値", `{"startTime":{"hour":9,"minute":true},` + e + `,` + b + `}`},
		{"4-9-g: hour が 9.5", `{"startTime":{"hour":9.5,"minute":0},` + e + `,` + b + `}`},
		{"4-9-g: hour が 9.0", `{"startTime":{"hour":9.0,"minute":0},` + e + `,` + b + `}`},
		{"4-9-g: minute が 0e0（指数部）", `{"startTime":{"hour":9,"minute":0e0},` + e + `,` + b + `}`},
		{"4-9-g: break.start の hour が 12.5", `{` + s + `,` + e + `,"break":{"start":{"hour":12.5,"minute":0},"end":{"hour":13,"minute":0}}}`},
		// 4-9-h
		{"4-9-h: hour が 64 ビット符号付き整数の上限を超える", `{"startTime":{"hour":9223372036854775808,"minute":0},` + e + `,` + b + `}`},
		{"4-9-h: minute が 64 ビット符号付き整数の下限を下回る", `{"startTime":{"hour":9,"minute":-9223372036854775809},` + e + `,` + b + `}`},
		{"4-9-h: break.end の hour が上限を超える", `{` + s + `,` + e + `,"break":{"start":{"hour":12,"minute":0},"end":{"hour":99999999999999999999,"minute":0}}}`},
		// 4-9-i
		{"4-9-i: 旧形式のボディ（workingHours だけ）", `{"workingHours":{"hours":8,"minutes":30}}`},
		// 4-9-j
		{"4-9-j: JSON として読めない", `{"startTime":`},
		{"4-9-j: JSON の配列", `[]`},
		{"4-9-j: JSON の null", `null`},
		{"4-9-j: 空のボディ", ``},
		{"4-9-j: オブジェクトでない（文字列）", `"startTime"`},
		// 4-13-a
		{"4-13-a: endTime が欠落し、かつ startTime が 3-2-b（時 24）", `{"startTime":{"hour":24,"minute":0},` + b + `}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker, output := runEnter(t, tt.body)

			err := output.onlyErr(t)
			if !errors.Is(err, port.ErrInvalidRequest) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrInvalidRequest)（AC-9-6-e・契約 AC-4-9）", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleEnterDailyRecord_RejectsContentTypeMismatch はボディを持つ E-3 で
// Content-Type を検査することを検証する（AC-9-6-i。契約 AC-1-2）。
func TestHandleEnterDailyRecord_RejectsContentTypeMismatch(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		setHeader   bool
	}{
		{name: "Content-Type が無い", setHeader: false},
		{name: "Content-Type が application/json でない", contentType: "text/plain", setHeader: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.EnterDailyRecordInput]{}
			output := &errorPresenterSpy{}
			headers := actorHeaders(testActorID, string(port.RoleEngineer))
			if tt.setHeader {
				headers["Content-Type"] = tt.contentType
			}
			body := []byte(validBody)
			r := newRequest(
				http.MethodPut, enterTarget(testContractID, "2026-07", "2026-07-01"), body, headers,
				enterPaths(testContractID, "2026-07", "2026-07-01")...,
			)

			controller.HandleEnterDailyRecord(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, port.ErrInvalidRequest) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrInvalidRequest)（AC-9-6-i）", err)
			}
			invoker.wantNoCall(t)
		})
	}
}

// TestHandleEnterDailyRecord_AcceptsContentTypeWithParameters は、契約 AC-1-2 が
// 固定するのが**メディアタイプ**（`application/json`）であって charset 等の
// パラメータではないことを検証する（AC-9-6-i）。
//
// パラメータ付きの値（`application/json; charset=utf-8`）を弾くと、標準的な
// HTTP クライアントが付ける charset だけで 400 になる。文字列の完全一致で
// 比較する実装ではこの表が Red になる（受理側の各行が INVALID_REQUEST に
// なるため）。上の RejectsContentTypeMismatch と対にして、メディアタイプが
// 異なる値は依然として弾かれることを保つ。
func TestHandleEnterDailyRecord_AcceptsContentTypeWithParameters(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
	}{
		{name: "パラメータなし", contentType: "application/json"},
		{name: "charset パラメータ付き", contentType: "application/json; charset=utf-8"},
		{name: "charset パラメータ付き（空白なし・大文字）", contentType: "application/json;charset=UTF-8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.EnterDailyRecordInput]{}
			output := &errorPresenterSpy{}
			headers := actorHeaders(testActorID, string(port.RoleEngineer))
			headers["Content-Type"] = tt.contentType
			body := []byte(validBody)
			r := newRequest(
				http.MethodPut, enterTarget(testContractID, "2026-07", "2026-07-01"), body, headers,
				enterPaths(testContractID, "2026-07", "2026-07-01")...,
			)

			controller.HandleEnterDailyRecord(r, invoker, output)

			output.wantNoErr(t)
			want := baseInput(t)
			want.StartTime = port.TimeOfDay{Hour: 9, Minute: 15}
			want.EndTime = port.TimeOfDay{Hour: 17, Minute: 45}
			want.Break = &port.Break{Start: port.TimeOfDay{Hour: 12, Minute: 30}, End: port.TimeOfDay{Hour: 13, Minute: 15}}
			got := invoker.onlyCall(t)
			if diff := cmp.Diff(want, got, cmp.AllowUnexported(workmonth.ContractID{}, workmonth.YearMonth{}, workmonth.Date{})); diff != "" {
				t.Errorf("invoker へ渡った入力 DTO が不一致 (-want +got):\n%s（AC-9-5-c・AC-9-6-i）", diff)
			}
		})
	}
}

// TestHandleEnterDailyRecord_RejectsWhenBothHeadersAbsent は更新系（契約 AC-9 順1
// の対象）で両ヘッダが不在の要求を弾くことを検証する（決定10・AC-9-7-a①）。
func TestHandleEnterDailyRecord_RejectsWhenBothHeadersAbsent(t *testing.T) {
	invoker := &invokerSpy[port.EnterDailyRecordInput]{}
	output := &errorPresenterSpy{}
	body := []byte(validBody)
	r := newRequest(
		http.MethodPut, enterTarget(testContractID, "2026-07", "2026-07-01"), body, map[string]string{"Content-Type": "application/json"},
		enterPaths(testContractID, "2026-07", "2026-07-01")...,
	)

	controller.HandleEnterDailyRecord(r, invoker, output)

	if err := output.onlyErr(t); !errors.Is(err, port.ErrUnauthenticated) {
		t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, port.ErrUnauthenticated)（決定10）", err)
	}
	invoker.wantNoCall(t)
}

// TestHandleEnterDailyRecord_HeaderAbsenceOrderedBeforeSyntaxCheck は決定10
// （操作者ヘッダの有無の判定を構文検査より前に置く）を E-3 でも固定する。
// close_work_month_test.go の同名の検証と対になる（AC-12-9）。
func TestHandleEnterDailyRecord_HeaderAbsenceOrderedBeforeSyntaxCheck(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		date       string
		wantErr    error
		wantNotErr error
	}{
		{
			name:       "両方不在 かつ 対象日の書式不正 → UNAUTHENTICATED が先（決定10）",
			headers:    map[string]string{"Content-Type": "application/json"},
			date:       "2026-02-30",
			wantErr:    port.ErrUnauthenticated,
			wantNotErr: port.ErrInvalidRequest,
		},
		{
			name:       "構文不正のみ（ヘッダは妥当） → INVALID_REQUEST",
			headers:    jsonHeaders(testActorID, string(port.RoleEngineer)),
			date:       "2026-02-30",
			wantErr:    port.ErrInvalidRequest,
			wantNotErr: port.ErrUnauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := &invokerSpy[port.EnterDailyRecordInput]{}
			output := &errorPresenterSpy{}
			r := newRequest(
				http.MethodPut, enterTarget(testContractID, "2026-07", tt.date), []byte(validBody), tt.headers,
				enterPaths(testContractID, "2026-07", tt.date)...,
			)

			controller.HandleEnterDailyRecord(r, invoker, output)

			err := output.onlyErr(t)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("PresentError に渡されたエラー = %v, want errors.Is(err, %v)", err, tt.wantErr)
			}
			if errors.Is(err, tt.wantNotErr) {
				t.Fatalf("PresentError に渡されたエラーが %v であってはならない: %v", tt.wantNotErr, err)
			}
			invoker.wantNoCall(t)
		})
	}
}
