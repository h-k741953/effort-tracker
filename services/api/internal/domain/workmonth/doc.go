// Package workmonth は勤務月（WorkMonth）集約を提供する。
//
// 勤務月は本システム唯一の集約ルートであり、契約 × 年月によって一意に決まる。
// 用語の定義は docs/domain/ubiquitous-language.md を参照。
//
// このパッケージおよび配下は Go の標準ライブラリのみに依存する。
// フレームワーク・ORM・AWS SDK・ログライブラリを import してはならない。
// 規約は make check-domain-deps および CI により機械的に検査される。
package workmonth
