// Command demo-data-reset はデモデータの日次リセット Lambda のエントリポイントである
// （docs/specs/demo-data-reset.md AC-6・AC-9〜AC-12・AC-15）。
//
// import してよいのは internal/domain・internal/adapter/gateway・
// internal/driver/persistence、cmd/bootstrap が使っている AWS SDK for Go v2 の
// 設定解決と SSM、aws-lambda-go、標準ライブラリだけである（AC-12-11）。
package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"
	_ "time/tzdata" // Asia/Tokyo をバイナリへ埋め込む（daily-record-entry.md D-8）。

	awslambda "github.com/aws/aws-lambda-go/lambda"

	"github.com/h-k741953/effort-tracker/services/api/internal/adapter/gateway"
	"github.com/h-k741953/effort-tracker/services/api/internal/domain/workmonth"
	"github.com/h-k741953/effort-tracker/services/api/internal/driver/persistence"
)

// neonConnectionStringParameterNameEnv は Neon 接続文字列を収める SSM パラメータの名前を
// 渡す環境変数の名前である（cmd/bootstrap と同じ。値ではなく名前）。
const neonConnectionStringParameterNameEnv = "NEON_CONNECTION_STRING_SSM_PARAMETER_NAME"

// demoEngineerIDEnv はデモの技術者識別子を渡す環境変数の名前である（AC-7-1・AC-7-3。値は構成側が持つ）。
const demoEngineerIDEnv = "DEMO_ENGINEER_ID"

// resetTimeZone はリセット日の基準タイムゾーン（AC-11-1）。
const resetTimeZone = "Asia/Tokyo"

//go:embed ddl/*.sql
var ddlFS embed.FS

// Handler は Lambda ランタイムへ登録するハンドラの形である。イベントの本文は読まない（AC-11-1）。
type Handler func(ctx context.Context, event json.RawMessage) error

var (
	errMissingParameterName = errors.New("demo-data-reset: SSM パラメータ名の環境変数が未設定または空である")
	errMissingEngineerID    = errors.New("demo-data-reset: デモの技術者識別子の環境変数が未設定または空である")
)

// Run はコールドスタートの手続きである。環境変数の探索 → 接続文字列の解決（1回）→
// 設定の組み立て → 接続の確立（1回）→ ハンドラの登録（1回）の順に行い、DB に触れない。
// いずれかに失敗したら登録せずにエラーを返す（AC-7-4・AC-11-7）。
// エラーの文言に接続文字列・パラメータ名・環境変数の値を含めない（AC-11-6）。
func Run(
	resolve func(ctx context.Context, parameterName string) (string, error),
	lookup func(name string) (string, bool),
	connect func(ctx context.Context, cfg persistence.Config) (gateway.DB, error),
	now func() time.Time,
	build func(resetDate workmonth.Date, engineerID string) ([]gateway.DemoContract, []*workmonth.WorkMonth, error),
	register func(h Handler),
) error {
	engineerID, ok := lookup(demoEngineerIDEnv)
	if !ok || engineerID == "" {
		return fmt.Errorf("%w: %s", errMissingEngineerID, demoEngineerIDEnv)
	}
	parameterName, ok := lookup(neonConnectionStringParameterNameEnv)
	if !ok || parameterName == "" {
		return fmt.Errorf("%w: %s", errMissingParameterName, neonConnectionStringParameterNameEnv)
	}
	loc, err := time.LoadLocation(resetTimeZone)
	if err != nil {
		return fmt.Errorf("demo-data-reset: 基準タイムゾーンの解決に失敗した: %w", err)
	}

	connectionString, err := resolve(context.Background(), parameterName)
	if err != nil {
		return fmt.Errorf("demo-data-reset: 接続文字列の解決に失敗した: %w", err)
	}
	cfg, err := persistence.LoadConfig(lookup, connectionString)
	if err != nil {
		return fmt.Errorf("demo-data-reset: 接続設定の組み立てに失敗した: %w", err)
	}
	db, err := connect(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("demo-data-reset: データベース接続の確立に失敗した: %w", err)
	}

	resetter := gateway.NewDemoResetter(db)
	register(func(ctx context.Context, _ json.RawMessage) error {
		return reset(ctx, db, resetter, now(), loc, engineerID, build)
	})
	return nil
}

// reset は1回のリセットである。リセット日を時計から求め（AC-11-1）、組み立て、DDL を適用し（AC-10-5）、
// 契約の書き換え・破棄・保存を gateway の1トランザクションで行う（AC-15）。
func reset(
	ctx context.Context,
	db gateway.DB,
	resetter *gateway.DemoResetter,
	now time.Time,
	loc *time.Location,
	engineerID string,
	build func(resetDate workmonth.Date, engineerID string) ([]gateway.DemoContract, []*workmonth.WorkMonth, error),
) error {
	local := now.In(loc)
	resetDate, err := workmonth.NewDate(local.Year(), int(local.Month()), local.Day())
	if err != nil {
		return fmt.Errorf("demo-data-reset: リセット日の算出に失敗した: %w", err)
	}
	contracts, workMonths, err := build(resetDate, engineerID)
	if err != nil {
		return fmt.Errorf("demo-data-reset: デモデータの組み立てに失敗した: %w", err)
	}
	if err := applyDDL(ctx, db); err != nil {
		return err
	}
	if err := resetter.Reset(ctx, contracts, workMonths); err != nil {
		return fmt.Errorf("demo-data-reset: リセットに失敗した: %w", err)
	}
	return nil
}

// applyDDL は埋め込んだ DDL を、ファイル名の順に1文ずつ Tx の外で適用する（AC-10-1・AC-10-3）。
// 失敗したら以降の文へ進まない。エラーの文言に SQL 文を含めない（AC-11-6）。
func applyDDL(ctx context.Context, db gateway.DB) error {
	entries, err := ddlFS.ReadDir("ddl")
	if err != nil {
		return fmt.Errorf("demo-data-reset: 埋め込んだ DDL の読み込みに失敗した: %w", err)
	}
	for i, entry := range entries {
		sql, err := ddlFS.ReadFile("ddl/" + entry.Name())
		if err != nil {
			return fmt.Errorf("demo-data-reset: 埋め込んだ DDL #%d の読み込みに失敗した: %w", i, err)
		}
		if err := db.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("demo-data-reset: DDL #%d の適用に失敗した: %w", i, err)
		}
	}
	return nil
}

// startLambda は Lambda ランタイムへの登録を Run が要求する形へ合わせる。
func startLambda(h Handler) { awslambda.Start(h) }

func main() {
	if err := Run(resolveConnectionString, os.LookupEnv, persistence.Connect, time.Now, buildDemoData, startLambda); err != nil {
		log.Fatalf("demo-data-reset: 起動に失敗した: %v", err)
	}
}
