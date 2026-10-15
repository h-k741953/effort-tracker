package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"regexp"
	"strconv"

	"github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"
)

// enterDailyRecordInvoker は EnterDailyRecord（E-3）の呼び出し先が満たす
// 最小の interface（AC-9-8-a）。
type enterDailyRecordInvoker interface {
	Execute(ctx context.Context, input port.EnterDailyRecordInput)
}

// HandleEnterDailyRecord は E-3
// （PUT /work-months/{contractId}/{yearMonth}/daily-records/{date}）を
// 入力 DTO（port.EnterDailyRecordInput）へ変換し invoker を呼ぶ（AC-9-5-c）。
//
// 更新系は契約 AC-9 順1 の対象。両ヘッダ不在なら入力 DTO を組み立てずに
// port.ErrUnauthenticated を errorPresenter へ渡す（決定10・AC-9-7-a①）。
// 時刻は値域も前後関係も検査せず素の整数のまま写す。弾くのは欠落と型不一致だけ
// （AC-9-6-e・契約 AC-4-9）。未来日・当該月外・
// 状態・認可はいずれも検査しない（AC-9-6-d・AC-9-6-f）。
func HandleEnterDailyRecord(r *http.Request, invoker enterDailyRecordInvoker, output errorPresenter) {
	actor, err := requireActorHeader(r)
	if err != nil {
		output.PresentError(err)
		return
	}

	// 契約 AC-1-2 が固定するのはメディアタイプであって charset 等のパラメータ
	// ではないため、mime.ParseMediaType でメディアタイプだけを比較する
	// （`application/json; charset=utf-8` のようなパラメータ付きの値を
	// 誤って弾かないため）。
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		output.PresentError(fmt.Errorf("%w: Content-Type must be application/json", port.ErrInvalidRequest))
		return
	}

	contractID, err := parseContractID(r.PathValue("contractId"))
	if err != nil {
		output.PresentError(err)
		return
	}

	yearMonth, err := parseYearMonth(r.PathValue("yearMonth"))
	if err != nil {
		output.PresentError(err)
		return
	}

	date, err := parseDate(r.PathValue("date"))
	if err != nil {
		output.PresentError(err)
		return
	}

	// ボディは最上位のキーごとに生のまま受ける。未知フィールドは落とす
	// （エラーにしない。AC-9-6-g）。`null` や配列などオブジェクト以外は弾く（契約 AC-4-9-j）。
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body == nil {
		output.PresentError(fmt.Errorf("%w: body is not a JSON object", port.ErrInvalidRequest))
		return
	}

	startTime, err := parseTimeOfDay(body["startTime"])
	if err != nil {
		output.PresentError(err)
		return
	}
	endTime, err := parseTimeOfDay(body["endTime"])
	if err != nil {
		output.PresentError(err)
		return
	}
	brk, err := parseBreak(body["break"])
	if err != nil {
		output.PresentError(err)
		return
	}

	invoker.Execute(r.Context(), port.EnterDailyRecordInput{
		Actor:      actor,
		ContractID: contractID,
		YearMonth:  yearMonth,
		Date:       date,
		StartTime:  startTime,
		EndTime:    endTime,
		Break:      brk,
	})
}

// parseBreak は `break` を読む。キーの省略は休憩なしとみなさず弾く（契約 D-18・AC-4-9-c）。
// `null` は休憩なし（nil）、オブジェクトは休憩あり、それ以外は型の不正（AC-4-9-d）。
func parseBreak(raw json.RawMessage) (*port.Break, error) {
	if raw == nil {
		return nil, fmt.Errorf("%w: break is missing", port.ErrInvalidRequest)
	}
	if string(raw) == "null" {
		return nil, nil
	}
	fields, err := decodeObject(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: break must be null or an object", port.ErrInvalidRequest)
	}
	start, err := parseTimeOfDay(fields["start"])
	if err != nil {
		return nil, err
	}
	end, err := parseTimeOfDay(fields["end"])
	if err != nil {
		return nil, err
	}
	return &port.Break{Start: start, End: end}, nil
}

// parseTimeOfDay は時刻オブジェクト `{"hour": n, "minute": n}` を読む（契約 D-14）。
// 欠落・null・オブジェクト以外・整数でない値を弾く。値域は検査しない（AC-9-6-e）。
func parseTimeOfDay(raw json.RawMessage) (port.TimeOfDay, error) {
	fields, err := decodeObject(raw)
	if err != nil {
		return port.TimeOfDay{}, fmt.Errorf("%w: time of day must be an object", port.ErrInvalidRequest)
	}
	hour, err := parseInteger(fields["hour"])
	if err != nil {
		return port.TimeOfDay{}, err
	}
	minute, err := parseInteger(fields["minute"])
	if err != nil {
		return port.TimeOfDay{}, err
	}
	return port.TimeOfDay{Hour: hour, Minute: minute}, nil
}

// decodeObject は生の JSON が（null でない）オブジェクトであるときにだけ復号する。
func decodeObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, port.ErrInvalidRequest
	}
	return fields, nil
}

// integerLiteral は小数部も指数部も持たない JSON の整数（契約 AC-4-9-g）。
var integerLiteral = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

// parseInteger は JSON の整数を int として読む。キーの欠落・null・整数でない値・
// 64 ビット符号付き整数で表せない値は型の不正として弾く（契約 AC-4-9-f〜h）。
func parseInteger(raw json.RawMessage) (int, error) {
	if !integerLiteral.Match(raw) {
		return 0, fmt.Errorf("%w: hour and minute must be integers", port.ErrInvalidRequest)
	}
	v, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: integer does not fit in 64 bits", port.ErrInvalidRequest)
	}
	return int(v), nil
}
