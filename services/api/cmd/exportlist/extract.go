package main

// extract.go は docs/specs/public-api-diff-check.md AC-3（抽出規則）の実装
// である。構文解析だけで決まることしか見ない（依存解決・ネットワーク
// アクセス・ビルドタグの評価を行わない。同 AC-3 前文、ADR 0018 決定7-3）。
// **型解決は例外**であり、AC-3-9-3（複合リテラルのキーの弁別）のためだけに、
// 解析対象パッケージ内へ閉じた範囲で行う（AC-3-11 / AC-3-11-1。
// ADR 0019 決定2・3・5・6 が ADR 0018 決定7-3 の「型解決を採らない」を
// 置換する）。
//
// 依存は標準ライブラリの go/parser・go/ast・go/printer・go/token に加え、
// 型解決のための go/types・go/importer のみ（AC-3-11 / ADR 0019 決定6）。
// services/api/go.mod の require は1件も増やさない（AC-3-12。上記はすべて
// 標準ライブラリである）。
//
// トップレベルの識別子はすべて非公開にする（AC-3-15。抽出器自身が抽出対象
// に入る自己言及の帰結。詳細は main_test.go）。

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// record は抽出結果の1レコードを表す。フィールドは
// docs/specs/public-api-diff-check.md AC-2（エクスポート一覧のレコード書式）
// の4フィールドに対応する。
type record struct {
	Pkg       string
	Kind      string
	Name      string
	Signature string
}

// typeInfo は AC-3-11 / AC-3-11-1（ADR 0019 決定2・3・5）が許す範囲の、
// 1つの Go パッケージ（1ディレクトリ）分の型解決結果を運ぶ。用途は
// AC-3-9-3（複合リテラルのキーの弁別）に限る —— <signature> の綴りは
// 引き続き書かれた型式から作り、型解決の結果で置き換えない（決定5）。
//
// パッケージ単位でスコープが変わる値であるため、グローバル変数には置かず、
// fset と並ぶ明示的な引数として呼び出しの連鎖全体（typeExprSignature とその
// 再帰の入口すべて）へ渡す。
type typeInfo struct {
	// info は nil になりうる（対象ディレクトリの型検査が致命的に失敗し、
	// Uses が一切埋まらなかった場合を含む）。nil は「その識別子について
	// 型情報が得られない」（AC-3-9-3 (c)）と同じに扱う。
	info *types.Info
}

// removeCompositeKey は AC-3-9-3 の判定フローそのものを実装する
// （docs/specs/public-api-diff-check.md「複合リテラルのキーを型解決で弁別
// する理由と、判定のフロー」の mermaid 図。ADR 0019 決定4）。真を返すのは
// (b) の非公開フィールド名の枝に限る —— それ以外（裸の識別子でない、型
// 情報が得られない、構造体フィールドと判らない、公開名）はすべて false
// （＝書かれたまま出力する）。
//
// key は、この typeInfo を作った型検査に使ったのと同じ *ast.File 上に現れる
// コピー前の元のノードでなければならない。types.Info.Uses はノードの
// ポインタ同一性で引かれるため、typeExprSignature の *ast.KeyValueExpr
// ケースは、子を再帰コピーする前にこの判定を呼ぶ。
func (ti typeInfo) removeCompositeKey(key ast.Expr) bool {
	ident, ok := key.(*ast.Ident)
	if !ok {
		// (a) 裸の識別子でなければ書かれたまま出力する。
		return false
	}
	if ti.info == nil {
		// (c) パッケージの型検査が致命的に失敗し、型情報が一切無い。
		return false
	}
	obj, ok := ti.info.Uses[ident]
	if !ok || obj == nil {
		// (c) このキーについて型情報が得られない
		// （複合リテラルの型自体を解決できなかった場合を含む）。
		return false
	}
	v, ok := obj.(*types.Var)
	if !ok || !v.IsField() {
		// 構造体フィールドと判らない（配列・スライスの複合リテラルの
		// インデックスに現れる定数・変数への参照など）。AC-3-9 の対象
		// ではないため除去しない。
		return false
	}
	// (b) 構造体フィールドと判った。公開名は書かれたまま、非公開名は
	// AC-3-9 により除去する。
	return !v.Exported()
}

// buildPackageTypeInfo は files（すべて同一ディレクトリ＝同一 Go パッケージ
// に属し、AC-3-1 の除外を通過済みの構文木）に対し、AC-3-11 が許す範囲
// （パッケージ内へ閉じた型解決。ADR 0019 決定2・3）で型検査を行う。
//
// import 先の解決に失敗しても型検査は継続し、パッケージ内で完結する情報
// （複合リテラルのキーの Uses など）は得られる（ADR 0019 決定3 実測A）。
// Config.Error にはエラーを集めて捨てるだけのコールバックを渡し、Check の
// 戻り値のエラーで抽出全体を中断しない（AC-3-14-1。型検査のエラーを抽出器の
// 非ゼロ終了の理由にしない。決定6）。
//
// 型検査そのものが致命的に失敗して Uses が一切埋まらなくても、呼び出し側
// （removeCompositeKey）は「型情報が得られない」既定（AC-3-9-3 (c)）へ
// 安全側に倒れる。
func buildPackageTypeInfo(fset *token.FileSet, pkgPath string, files []*ast.File) *types.Info {
	info := &types.Info{
		Uses: make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{
		Importer: importer.Default(),
		Error:    func(error) {},
	}
	// Check の戻り値のエラーは AC-3-14-1 により致命にしない。エラーが
	// あっても info.Uses は部分的に埋まる（決定3 実測A）。
	_, _ = conf.Check(pkgPath, fset, files, info)
	return info
}

// extractRecords は dir 配下（再帰的）の *.go を AC-3 の規則で走査し、公開
// シンボルのレコード一覧を返す。
//
// AC-3-13: dir 配下に *.go が1つも無ければ、0件のレコードとエラーを返す
// （「対象が無いので通った」を作らない）。
// AC-3-14: 構文解析に失敗したファイルがあれば、その時点でエラーを返す
// （読めなかったファイルを黙って飛ばさない）。
func extractRecords(dir string) ([]record, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("extractRecords: ディレクトリを解決できない: %s: %w", dir, err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return nil, fmt.Errorf("extractRecords: ディレクトリを読めない: %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("extractRecords: ディレクトリではない: %s", dir)
	}

	var allGoFiles []string
	walkErr := filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			allGoFiles = append(allGoFiles, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("extractRecords: ディレクトリを走査できない: %s: %w", dir, walkErr)
	}

	// AC-3-13: *.go が1つも見つからない（除外規則を適用する前の時点で0件）。
	if len(allGoFiles) == 0 {
		return nil, fmt.Errorf(
			"extractRecords: 抽出対象の Go パッケージが1つも見つからない: %s", dir,
		)
	}

	// 決定的な処理順にする（AC-2-7 の出力ソートとは別に、走査順序が
	// ファイルシステムの列挙順に依存しないようにするため）。
	sort.Strings(allGoFiles)

	fset := token.NewFileSet()

	// parsedFile は1ファイル分のパース結果と、それが属する Go パッケージ
	// （＝ディレクトリ）を保持する。型解決（buildPackageTypeInfo）は
	// パッケージ単位でしか行えない一方、抽出結果の記録順は既存どおり
	// ファイル単位で積み上げるため、パースを1パス目、型解決を2パス目、
	// 抽出を3パス目に分ける。
	type parsedFile struct {
		file    *ast.File
		pkgPath string
	}
	var parsedFiles []parsedFile
	filesByPkg := make(map[string][]*ast.File)

	for _, f := range allGoFiles {
		if isExcludedPath(absDir, f) {
			continue
		}

		// ParseComments フラグを付けないため、コメントは AST に一切
		// 保持されない。printer がコメントを出力に混入させることは
		// 構造的に無い（AC-3-7「コメントを出力に含めない」）。
		astFile, perr := parser.ParseFile(fset, f, nil, parser.SkipObjectResolution)
		if perr != nil {
			return nil, fmt.Errorf(
				"extractRecords: 構文解析に失敗した: %s: %w", f, perr,
			)
		}

		relDir, rerr := filepath.Rel(absDir, filepath.Dir(f))
		if rerr != nil {
			return nil, fmt.Errorf(
				"extractRecords: 相対パスを解決できない: %s: %w", f, rerr,
			)
		}
		pkgPath := filepath.ToSlash(relDir)

		parsedFiles = append(parsedFiles, parsedFile{file: astFile, pkgPath: pkgPath})
		filesByPkg[pkgPath] = append(filesByPkg[pkgPath], astFile)
	}

	// AC-3-11 / AC-3-11-1: パッケージ（ディレクトリ）単位に閉じた型解決を
	// 1回だけ行う。用途は AC-3-9-3 の複合リテラルのキー弁別に限る。
	typeInfoByPkg := make(map[string]*types.Info, len(filesByPkg))
	for pkgPath, files := range filesByPkg {
		typeInfoByPkg[pkgPath] = buildPackageTypeInfo(fset, pkgPath, files)
	}

	var records []record
	for _, pf := range parsedFiles {
		ti := typeInfo{info: typeInfoByPkg[pf.pkgPath]}
		records = append(records, extractFromFile(fset, ti, pf.file, pf.pkgPath)...)
	}

	return records, nil
}

// isExcludedPath は AC-3-1 の除外規則: (a) ファイル名が _test.go で終わる
// もの、(b) root からの相対パスの要素に testdata または vendor を含むもの。
func isExcludedPath(root, path string) bool {
	if strings.HasSuffix(path, "_test.go") {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == "testdata" || part == "vendor" {
			return true
		}
	}
	return false
}

// extractFromFile はファイル1つ分のトップレベル宣言から、公開シンボルの
// レコードを取り出す（AC-3-4: トップレベルのみ。関数内ローカル宣言・
// 非公開の宣言は対象外）。ti は file が属するパッケージの型解決結果
// （AC-3-9-3 の弁別にのみ使う。用途を広げない — AC-3-11-1）。
func extractFromFile(fset *token.FileSet, ti typeInfo, file *ast.File, pkgPath string) []record {
	var out []record
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			out = append(out, extractFuncDecl(fset, ti, d, pkgPath)...)
		case *ast.GenDecl:
			out = append(out, extractGenDecl(fset, ti, d, pkgPath)...)
		}
	}
	return out
}

// extractFuncDecl は func 宣言（メソッドを含む）を扱う。
//
// AC-3-5: メソッドは受信者の基底型名とメソッド名の両方が公開のときのみ
// 対象とする。
func extractFuncDecl(fset *token.FileSet, ti typeInfo, d *ast.FuncDecl, pkgPath string) []record {
	if d.Recv == nil {
		if !isExported(d.Name.Name) {
			return nil
		}
		sig := funcTypeSignature(fset, ti, d.Type, true)
		return []record{{Pkg: pkgPath, Kind: "func", Name: d.Name.Name, Signature: sig}}
	}

	if len(d.Recv.List) == 0 {
		return nil
	}
	recvExpr := d.Recv.List[0].Type
	baseName := receiverBaseName(recvExpr)
	if !isExported(baseName) || !isExported(d.Name.Name) {
		return nil
	}

	recvStr := normalizeWhitespace(printNode(fset, typeExprSignature(fset, ti, recvExpr)))
	argsRes := funcTypeSignature(fset, ti, d.Type, false)
	sig := "(" + recvStr + ") " + argsRes
	return []record{{
		Pkg:       pkgPath,
		Kind:      "method",
		Name:      baseName + "." + d.Name.Name,
		Signature: sig,
	}}
}

// extractGenDecl は type / var / const 宣言を扱う。
func extractGenDecl(fset *token.FileSet, ti typeInfo, d *ast.GenDecl, pkgPath string) []record {
	var out []record
	switch d.Tok {
	case token.TYPE:
		for _, spec := range d.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || !isExported(ts.Name.Name) {
				continue
			}
			out = append(out, record{
				Pkg:       pkgPath,
				Kind:      "type",
				Name:      ts.Name.Name,
				Signature: typeSpecSignature(fset, ti, ts),
			})
		}
	case token.VAR, token.CONST:
		kind := "var"
		if d.Tok == token.CONST {
			kind = "const"
		}
		for _, spec := range d.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range vs.Names {
				if !isExported(name.Name) {
					continue
				}
				sig := "-"
				if vs.Type != nil {
					sig = normalizeWhitespace(printNode(fset, typeExprSignature(fset, ti, vs.Type)))
				}
				out = append(out, record{
					Pkg:       pkgPath,
					Kind:      kind,
					Name:      name.Name,
					Signature: sig,
				})
			}
		}
	}
	return out
}

// typeSpecSignature は AC-3-6 の type 行を作る:
// 型パラメータがあれば [...] を先頭に置き、続けて右辺の型式。型エイリアス
// は右辺の前に "= " を付ける（AC-3-9 / AC-3-10 の非公開メンバー除去を含む）。
//
// typeExprSignature は ts.Type そのものを書き換えず、印字用のコピーを返す。
func typeSpecSignature(fset *token.FileSet, ti typeInfo, ts *ast.TypeSpec) string {
	body := normalizeWhitespace(printNode(fset, typeExprSignature(fset, ti, ts.Type)))

	var sb strings.Builder
	if ts.TypeParams != nil && len(ts.TypeParams.List) > 0 {
		sb.WriteString(typeParamsString(fset, ti, ts.TypeParams))
		sb.WriteString(" ")
	}
	if ts.Assign != token.NoPos {
		sb.WriteString("= ")
	}
	sb.WriteString(body)
	return sb.String()
}

// typeExprSignature は e のコピーを返し、<signature> に現れるあらゆる
// 入れ子位置へ一様に次の4つを適用する（Issue #93 reviewer 往復2の
// 指摘 W-2r: 経路ごとに個別対応すると適用漏れが再発するため、型式を
// 1回のコピー再帰で走査する単一の入口をここに設ける）。
//
//   - AC-3-9: 構造体の非公開フィールド／インターフェースの非公開メソッド
//     の除去
//   - AC-3-10: 埋め込みフィールドは、埋め込まれた型名の公開性で除去を判定
//   - AC-3-6: 関数の引数名・結果名の除去
//   - AC-3-6-2: 関数リテラルの本体を固定綴りへ畳む
//   - AC-3-9-3: 複合リテラルのキーが非公開フィールド名と判ったときに限り
//     除去する（ti が運ぶ、パッケージ内へ閉じた型解決結果を使う。
//     AC-3-11-1 により用途はこれに限る）
//
// e 自身、および e から go/parser が返した既存の AST を辿って届く一切の
// ノードは書き換えない。変更が要る場所でだけ新しいノードを作って返し、
// 変更が不要な部分木はそのまま共有する（共有される部分木は以後どこからも
// 変更されないため安全）。この単一の入口は、type 宣言の右辺
// （typeSpecSignature）・var/const の宣言型（extractGenDecl）・関数の
// 引数型／結果型（fieldListTypesString）・型パラメータの制約
// （typeParamsString）のすべてから呼ばれる。printNode へ渡す直前は必ず
// ここを通す。
func typeExprSignature(fset *token.FileSet, ti typeInfo, e ast.Expr) ast.Expr {
	switch v := e.(type) {
	case nil:
		return nil
	case *ast.StructType:
		// 構造体のフィールドリストは、もはや go/printer に FieldList ごと
		// 渡さない。フィールド間の境界（AC-3-9-2）を go/printer の
		// 改行判断に委ねると、改行が normalizeWhitespace で空白1つへ
		// 畳まれた時点で境界の情報そのものが失われる（go/printer は複数
		// フィールドの struct を必ず改行区切りで出力し、1行のセミコロン
		// 区切りへは畳めない —— 実測済み）。そこで、フィールドごとに
		// 個別に印字した文字列を自前で "; " で連結し、その結果を
		// ast.Ident に埋め込んで返す。go/printer は Ident.Name をトークン
		// 検証なしにそのまま出力するため、上位のどの入れ子位置
		// （*、[]、map の key/value、他の struct のフィールド型など）に
		// 置かれても正しく出力される（printFieldMembers のコメント参照）。
		return &ast.Ident{Name: printFieldMembers(fset, "struct", filterFieldList(fset, ti, v.Fields))}
	case *ast.InterfaceType:
		return &ast.Ident{Name: printFieldMembers(fset, "interface", filterFieldList(fset, ti, v.Methods))}
	case *ast.FuncType:
		nv := *v
		nv.Params = stripFieldListNames(fset, ti, v.Params)
		nv.Results = stripFieldListNames(fset, ti, v.Results)
		return &nv
	case *ast.FuncLit:
		// AC-3-6-2（ADR 0019 決定1）: 関数リテラルの本体を固定綴りへ畳む。
		// 関数型の部分（引数名・結果名の除去を含む）は *ast.FuncType と
		// 同じ経路にそのまま委ね、本体だけを固定文字列に置き換える。
		// 本体を落として式ごと消すのではなく固定綴りで畳むのは、関数
		// リテラルであることと関数型であることの区別を綴りに残すため
		// （AC-3-6-1 の期待値表 (viii)/(ix) を緩めない）。
		ftStr := normalizeWhitespace(printNode(fset, typeExprSignature(fset, ti, v.Type)))
		return &ast.Ident{Name: ftStr + " " + foldedFuncLitBody}
	case *ast.KeyValueExpr:
		// AC-3-9-3: 複合リテラルのキー。ti.removeCompositeKey は v.Key
		// （コピー前の元のノード）で判定する必要があるため、コピーを
		// 作る前にここで判定する。
		if ti.removeCompositeKey(v.Key) {
			// (b) 非公開フィールド名: キーの綴りを落とし、値だけを残す
			// （除去するのはキーであり、値は落とさない。値の位置にも
			// 3-6-1 の一様性を掛けるため typeExprSignature へ委ねる）。
			return typeExprSignature(fset, ti, v.Value)
		}
		nv := *v
		nv.Key = typeExprSignature(fset, ti, v.Key)
		nv.Value = typeExprSignature(fset, ti, v.Value)
		return &nv
	case *ast.StarExpr:
		nv := *v
		nv.X = typeExprSignature(fset, ti, v.X)
		return &nv
	case *ast.ArrayType:
		nv := *v
		// v.Len（配列長の式。nil ならスライス型）の内部にも一様に掛ける
		// （AC-3-6-1）。typeExprSignature(fset, ti, nil) は case nil で nil
		// を返すため、スライス型でも安全。
		nv.Len = typeExprSignature(fset, ti, v.Len)
		nv.Elt = typeExprSignature(fset, ti, v.Elt)
		return &nv
	case *ast.Ellipsis:
		nv := *v
		nv.Elt = typeExprSignature(fset, ti, v.Elt)
		return &nv
	case *ast.MapType:
		nv := *v
		nv.Key = typeExprSignature(fset, ti, v.Key)
		nv.Value = typeExprSignature(fset, ti, v.Value)
		return &nv
	case *ast.ChanType:
		nv := *v
		nv.Value = typeExprSignature(fset, ti, v.Value)
		return &nv
	case *ast.ParenExpr:
		nv := *v
		nv.X = typeExprSignature(fset, ti, v.X)
		return &nv
	case *ast.IndexExpr:
		nv := *v
		nv.X = typeExprSignature(fset, ti, v.X)
		nv.Index = typeExprSignature(fset, ti, v.Index)
		return &nv
	case *ast.IndexListExpr:
		nv := *v
		nv.X = typeExprSignature(fset, ti, v.X)
		newIndices := make([]ast.Expr, len(v.Indices))
		for i, idx := range v.Indices {
			newIndices[i] = typeExprSignature(fset, ti, idx)
		}
		nv.Indices = newIndices
		// 括弧（ここでは大かっこ）の位置は、ここでは落とさない。位置の
		// 一律落としは printNode が印字直前に汎用処理として一度だけ適用する
		// （stripPositions。Issue #93 reviewer 往復8 で人間が承認した設計）。
		// 経路ごとに Lbrack/Rbrack だけを個別に NoPos 化する対処は、行取りが
		// 漏れる位置を数え上げる形になり、修飾識別子の内部・配列長の式の
		// 内部など数え上げから漏れた位置に同じ偽陽性を残す（AC-3-7-1 根拠）。
		return &nv
	case *ast.BinaryExpr:
		// 型集合の union 項（`T1 | T2`）。AC-3-9 が除去を許すのは
		// インターフェースの非公開メソッド、AC-3-10 が除去を許すのは
		// 埋め込みフィールドだけであり、union 項はどちらでもないため
		// ここでは除去しない（Issue #93 reviewer 往復2 の指摘 C-2:
		// 当時の埋め込み名判定（現 embeddableTypeName）が名前を解釈できない
		// ことを非公開と同一視して
		// 丸ごと消していた偽 Green の再発防止）。内部に関数型が現れうる
		// ため、名前剥がしだけは再帰する。
		nv := *v
		nv.X = typeExprSignature(fset, ti, v.X)
		nv.Y = typeExprSignature(fset, ti, v.Y)
		return &nv
	case *ast.UnaryExpr:
		// 型集合の `~T`。BinaryExpr と同じ理由で除去しない。
		nv := *v
		nv.X = typeExprSignature(fset, ti, v.X)
		return &nv
	default:
		// 上記いずれの case にも当たらないノード（*ast.CallExpr・
		// *ast.CompositeLit、および将来 go/ast に増える構文）は、種類ごとに
		// 個別の case を足さない（AC-3-6-1 根拠3・根拠4: 式の種類・呼び出
		// される関数の名前〔len / unsafe.Sizeof など〕で場合分けすると、
		// 数え上げから漏れた式に同じ穴を残す）。代わりに e を子ノードまで
		// 取りこぼさず再帰する descendExprStructure へ委ね、子孫に
		// struct / interface / func 型・関数リテラル・複合リテラルのキーが
		// 現れれば上の case が既存の処理（非公開除去・フィールド展開・
		// メンバー境界の区切り・引数名剥がし・本体の畳み込み・キーの弁別）
		// を掛ける（Issue #93 reviewer 往復10 の指摘 C-9-1）。Ident・
		// SelectorExpr のように変換対象を持たないノードは、
		// descendExprStructure を通しても中身は変わらない（コピーが増える
		// だけで出力は同じ）。
		//
		// e（根）は descendExprStructure を直接呼び、recurseExprFields が
		// 行う「動的型が ast.Expr を実装しているか」の判定は経由させない。
		// 根 e の動的型（*ast.CallExpr 等）は当然 ast.Expr を実装しており、
		// この判定を根に掛けると typeExprSignature(fset, ti, e) → default →
		// 同じ判定 → … と即座に無限再帰する（Issue #93 実装往復12。
		// recurseExprFields のコメント参照）。
		nv := descendExprStructure(fset, ti, reflect.ValueOf(e))
		if !nv.IsValid() {
			return e
		}
		result, ok := nv.Interface().(ast.Expr)
		if !ok {
			return e
		}
		return result
	}
}

// foldedFuncLitBody は AC-3-6-2 が要求する、関数リテラルの本体を畳んだ
// 固定綴り。綴りそのものは仕様が固定しない（3-6-2 前文。3-7-1 / 3-9-2 /
// 3-10-1 の期待値表と同じ扱い）。関数型の綴りと連結したときに関数リテラル
// であることが読み取れるよう、波括弧を含む形にする。
const foldedFuncLitBody = "{ ... }"

// exprIfaceType は ast.Expr インターフェースの reflect.Type
// （recurseExprFields が子の値の**動的型**と突き合わせるための基準。
// stripPositions が posType を使う形と同じ）。
var exprIfaceType = reflect.TypeOf((*ast.Expr)(nil)).Elem()

// recurseExprFields は v（型式の子の位置にある値 —— 構造体フィールドまたは
// スライス要素）のディープコピーを返しつつ、v 自身が ast.Expr を実装して
// いれば typeExprSignature を適用し、実装していなければ判定を掛けずに
// descendExprStructure へ構造走査を委ねる。typeExprSignature の default
// ケースおよび descendExprStructure の各分岐（子を再帰する箇所すべて）から
// 呼ばれる、子の判定の唯一の入口である。
//
// 【なぜ静的型ではなく動的型で判定するか（Issue #93 実装往復12・C-11-1）】
//
//	以前の実装は「v の静的型がちょうど ast.Expr インターフェースである
//	こと」（v.Type() == exprIfaceType）で判定していた。しかし go/ast は
//	フィールドの静的型を ast.Expr ではなく具体型で宣言している箇所がある
//	（例: ast.FuncLit.Type の静的型は *ast.FuncType）。静的型で判定すると、
//	そうしたフィールドは下の switch の reflect.Struct 分岐をただ構造的に
//	通過するだけで、typeExprSignature の struct / interface / func 型の
//	専用ケース（非公開除去・メンバー境界区切り・引数名剥がし）に一度も
//	渡らない。動的型（v.Type().Implements(exprIfaceType)）で判定すれば、
//	静的型が具体型でも動的に ast.Expr を実装している値をすべて
//	typeExprSignature へ渡せる（AC-3-6-1 根拠3・根拠4と同じ「経路ごとに
//	場合分けしない」方針の延長。フィールドを名指しで足すと同じ形の漏れが
//	別のフィールドで再発する）。
//
// 【なぜ根（typeExprSignature の default ケースに渡される e 自身）には
// この判定を掛けないか】
//
//	根の動的型（*ast.CallExpr 等）は当然 ast.Expr を実装している。根に
//	同じ判定を掛けると typeExprSignature(fset, e) → default →
//	recurseExprFields(fset, reflect.ValueOf(e)) → 判定が真 →
//	typeExprSignature(fset, e) → … と即座に無限再帰する。そのため根からの
//	最初の呼び出しは判定を経由しない descendExprStructure を直接呼び、
//	recurseExprFields は「判定を通過した後の子」からしか呼ばれない設計に
//	する。
//
// 【代入可能性のフォールバック（Issue #93 実装往復12・panic の危険）】
//
//	typeExprSignature は入力と異なる具体型を返すことがある
//	（*ast.StructType / *ast.InterfaceType を渡すと *ast.Ident を返す）。
//	v の静的型（例えば将来 go/ast に増えるかもしれない、静的型が
//	*ast.StructType のフィールド）へ戻り値をそのまま Set すると、型が
//	合わずに panic する。そのため戻り値が v の静的型へ代入可能かを
//	AssignableTo で確認し、代入できない場合は結果を捨てて
//	descendExprStructure による構造走査（非公開除去等は掛からないが
//	panic はしない）へフォールバックする。現行の go/ast にこの分岐を
//	踏む具体的なフィールドは無い（struct 経由でしか
//	typeExprSignature に渡らない、かつ静的型が StructType/InterfaceType
//	であるフィールドは無い）が、将来 go/ast に増えた場合の安全側の
//	倒し方として残す。
//
// v 自身・v から辿れる既存の AST は一切書き換えない（stripPositions と
// 同じ設計）。
//
// reflect の使用は標準ライブラリの範囲内であり、cmd/exportlist は domain
// ではないため AC-3-11 / AC-3-12・check-domain-deps・ADR 0007 のいずれにも
// 抵触しない（services/api/go.mod の require は増やしていない。
// stripPositions のコメントと同じ）。
func recurseExprFields(fset *token.FileSet, ti typeInfo, v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		// ast.Expr を実装する go/ast の具体型はすべてポインタ型であり、
		// ast.Expr 自身もインターフェース型なので、判定の対象はこの2種の
		// Kind に限られる（Struct・Slice 等は ast.Expr を実装しない）。
		if v.IsNil() {
			return v
		}
		if v.Type().Implements(exprIfaceType) {
			if expr, ok := v.Interface().(ast.Expr); ok {
				newExpr := typeExprSignature(fset, ti, expr)
				if newExpr == nil {
					return reflect.Zero(v.Type())
				}
				nv := reflect.ValueOf(newExpr)
				if nv.Type().AssignableTo(v.Type()) {
					return nv
				}
				// 代入不可能なら結果を捨てて構造走査へフォールバックする
				// （上のコメント「代入可能性のフォールバック」参照）。
			}
		}
	}
	return descendExprStructure(fset, ti, v)
}

// descendExprStructure は v の構造（ポインタの中身・インターフェースの
// 中身・構造体の各フィールド・スライスの各要素）だけを辿り、ast.Expr の
// 判定は一切行わずに、各子の再帰の入口を recurseExprFields へ戻す。
// typeExprSignature の default ケース（構造走査の根。判定を経由しない）と、
// recurseExprFields が判定を素通りさせた場合（子だが ast.Expr を実装して
// いない、または代入不可能だった場合）の両方から呼ばれる。v 自身・v から
// 辿れる既存の AST は一切書き換えない（stripPositions と同じ設計）。
func descendExprStructure(fset *token.FileSet, ti typeInfo, v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		nv := reflect.New(v.Type().Elem())
		nv.Elem().Set(recurseExprFields(fset, ti, v.Elem()))
		return nv
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		nv := reflect.New(v.Type()).Elem()
		nv.Set(recurseExprFields(fset, ti, v.Elem()))
		return nv
	case reflect.Struct:
		nv := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			nv.Field(i).Set(recurseExprFields(fset, ti, v.Field(i)))
		}
		return nv
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		nv := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			nv.Index(i).Set(recurseExprFields(fset, ti, v.Index(i)))
		}
		return nv
	default:
		return v
	}
}

// filterFieldList は fl（構造体のフィールドリスト、またはインターフェース
// のメソッド／型集合の要素リスト）のコピーを返す。AC-3-9 の非公開メンバー
// 除去・AC-3-10 の埋め込みフィールド除去・AC-3-10-1 の定義済み型名の例外を
// 適用したうえで、残す各メンバーの型に typeExprSignature を再帰適用する。
// fl 自身・fl.List の要素は一切書き換えない（新しい FieldList / Field を
// 作って返す）。戻り値は go/printer にそのまま渡されることはなく、
// printFieldMembers が各 Field を個別に印字して自前で連結する
// （filterFieldList のコメント内「メンバー境界の区切り」を参照）。
//
// 名前付きフィールド（Names が空でない）は、AC-3-9-1 のとおり非公開除去を
// 先に適用したうえで、残った公開の名前ごとに Field を1つずつ作って展開
// する（`A, B int` → `A int` / `B int` の2つの Field。Issue #93 reviewer
// 往復7 の指摘 W-7-1）。フィールド名は AC-3-6 が引数名・結果名・受信者
// 変数名に限って落とす対象へ含まれないため、展開後も残す（引数側の
// stripFieldListNames が名前を落とすのとは扱いが違う）。名前の個数で
// 分岐せず、keep の要素数ぶん一様にループするため「1個 / 2個以上」の
// 場合分けを持ち込まない。
//
// 無名（Names が空）のメンバーは、埋め込みフィールド（AC-3-10）または
// インターフェースの型集合の要素（union `T1 | T2`・`~T`）のいずれか。
// embeddableTypeName が名前を取り出せる構文（Ident・SelectorExpr・
// StarExpr・IndexExpr・IndexListExpr）のときだけ、その名前の公開性で
// 除去を判定する。名前を取り出せない構文（union の *ast.BinaryExpr・`~T`
// の *ast.UnaryExpr など）は無条件で残す —— 「名前として解釈できない」を
// 「非公開」と同一視すると型集合の要素が丸ごと消え、破壊的な型集合の変更
// が黙って見えなくなる（Issue #93 reviewer 往復2 の指摘 C-2、実装済みの
// 偽 Green）。
//
// AC-3-10-1: 名前が非公開（小文字）と判定された場合でも、その埋め込みが
// (a) パッケージ修飾子を持たずに書かれ、かつ (b) predeclaredTypeNames の
// 22個に含まれるときは除去しない（unqualifiedEmbedName で (a) を判定する。
// SelectorExpr は修飾子を持つため対象外のまま — 3-10 の除去を維持する）。
//
// # メンバー境界の区切り（AC-3-9-2。Issue #93 reviewer 往復8 で人間が承認した設計）
//
// go/printer は構造体・インターフェースのフィールドリストを、フィールドが
// 2個以上あると常に改行区切りで出力する（1行のセミコロン区切りへは畳めない
// —— 実測で確認済み。開き括弧・閉じ括弧の位置を揃えても変わらない）。
// normalizeWhitespace は改行を空白1つへ畳むため、go/printer に FieldList
// ごと渡す設計では、フィールドが2個以上あるときに境界の情報（区切り）が
// 完全に失われる。名前付きフィールド1個（名前+型）と埋め込み2個が同じ綴り
// になる偽 Green（AC-3-9-2 根拠1）はこれが原因である。
//
// そこで、FieldList を go/printer にまるごと渡すのをやめ、printFieldMembers
// が各 Field を個別に印字してから "; " で連結する。区切りは行取り
// （元ソースが1行か複数行か）に依存せず、除去後に残ったメンバーの並びだけで
// 決まる。
func filterFieldList(fset *token.FileSet, ti typeInfo, fl *ast.FieldList) *ast.FieldList {
	if fl == nil {
		return nil
	}
	nfl := *fl
	var newList []*ast.Field
	for _, f := range fl.List {
		if len(f.Names) == 0 {
			if name, ok := embeddableTypeName(f.Type); ok && !isExported(name) {
				if uname, uok := unqualifiedEmbedName(f.Type); !uok || !predeclaredTypeNames[uname] {
					continue
				}
			}
			nf := *f
			nf.Type = typeExprSignature(fset, ti, f.Type)
			newList = append(newList, &nf)
			continue
		}
		var keep []*ast.Ident
		for _, n := range f.Names {
			if isExported(n.Name) {
				keep = append(keep, n)
			}
		}
		if len(keep) == 0 {
			continue
		}
		// AC-3-9-1: 残った公開の名前ごとにフィールドを分割する（型は
		// 名前の数だけ繰り返す）。typeExprSignature は f.Type に対して
		// 1回だけ呼び、生成したコピーを各 Field で共有する（printer は
		// 読み取り専用に辿るだけなので、同じ部分木を複数の Field から
		// 参照しても安全）。
		typ := typeExprSignature(fset, ti, f.Type)
		for _, n := range keep {
			nf := *f
			nf.Names = []*ast.Ident{n}
			nf.Type = typ
			newList = append(newList, &nf)
		}
	}
	nfl.List = newList
	return &nfl
}

// embeddableTypeName は埋め込みフィールド／埋め込みインターフェースの
// 型名を取り出す。修飾がある場合は最終要素の識別子（AC-3-10）。ポインタ・
// 型パラメータ実体化も剥がす。ok が偽なのは、そもそも型名を持たない構文
// （型集合の union 項・`~T` など）のとき。この ok を「非公開」に読み替え
// ないこと（filterFieldList のコメントを参照）。
func embeddableTypeName(e ast.Expr) (name string, ok bool) {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name, true
	case *ast.SelectorExpr:
		return v.Sel.Name, true
	case *ast.StarExpr:
		return embeddableTypeName(v.X)
	case *ast.IndexExpr:
		return embeddableTypeName(v.X)
	case *ast.IndexListExpr:
		return embeddableTypeName(v.X)
	default:
		return "", false
	}
}

// predeclaredTypeNames は AC-3-10-1 が定める、埋め込みとして現れた場合に
// 公開扱いで保持する定義済み（predeclared）型名の一覧。Go 言語仕様の
// predeclared identifiers のうち型名であるもの全22個に限る（型名でない
// true/false/iota/nil と組み込み関数は埋め込みとして合法に書けないため
// 対象にしない。AC-3-10-1 前文）。
var predeclaredTypeNames = map[string]bool{
	"any":        true,
	"bool":       true,
	"byte":       true,
	"comparable": true,
	"complex64":  true,
	"complex128": true,
	"error":      true,
	"float32":    true,
	"float64":    true,
	"int":        true,
	"int8":       true,
	"int16":      true,
	"int32":      true,
	"int64":      true,
	"rune":       true,
	"string":     true,
	"uint":       true,
	"uint8":      true,
	"uint16":     true,
	"uint32":     true,
	"uint64":     true,
	"uintptr":    true,
}

// unqualifiedEmbedName は埋め込みフィールド／埋め込み要素の型式が、
// パッケージ修飾子を持たずに書かれた識別子であるときにその名前を返す
// （AC-3-10-1 適用条件 (a)）。ポインタ・型パラメータ実体化は識別子部分まで
// 剥がして判定する（条文「*T の形で書かれていてもよく、判定は識別子部分で
// 行う」）。*ast.SelectorExpr（pkg.T の形）はパッケージ修飾子を持つため
// 対象外（ok=false）— embeddableTypeName とは異なり、ここでは修飾子付きを
// 拾わない。
func unqualifiedEmbedName(e ast.Expr) (name string, ok bool) {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name, true
	case *ast.StarExpr:
		return unqualifiedEmbedName(v.X)
	case *ast.IndexExpr:
		return unqualifiedEmbedName(v.X)
	case *ast.IndexListExpr:
		return unqualifiedEmbedName(v.X)
	default:
		return "", false
	}
}

// receiverBaseName はメソッド受信者の基底型名を取り出す（AC-3-5）。
// T / *T / T[K, V] / *T[K, V] のいずれでも基底の識別子を返す。
func receiverBaseName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return receiverBaseName(v.X)
	case *ast.IndexExpr:
		return receiverBaseName(v.X)
	case *ast.IndexListExpr:
		return receiverBaseName(v.X)
	default:
		return ""
	}
}

// funcTypeSignature は func / method の "(<引数型…>) (<結果型…>)" を作る
// （AC-3-6・AC-3-8）。includeTypeParams が真のとき、型パラメータがあれば
// 先頭に "[T any, U comparable]" の形で付ける（func のみ。method の型
// パラメータは受信者側にあり funcTypeSignature の対象外）。
func funcTypeSignature(fset *token.FileSet, ti typeInfo, ft *ast.FuncType, includeTypeParams bool) string {
	var sb strings.Builder
	if includeTypeParams && ft.TypeParams != nil && len(ft.TypeParams.List) > 0 {
		sb.WriteString(typeParamsString(fset, ti, ft.TypeParams))
		sb.WriteString(" ")
	}
	sb.WriteString(fieldListTypesString(fset, ti, ft.Params))
	sb.WriteString(" ")
	sb.WriteString(fieldListTypesString(fset, ti, ft.Results))
	return sb.String()
}

// fieldListTypesString は引数・結果それぞれの型だけを取り出し、
// "(<型>, <型>, ...)" の形にする（引数名・結果名は出力しない＝AC-3-6）。
// 複数名をまとめた宣言（`x, y int`）は名前の数だけ型を繰り返す。
// 可変長引数はフィールドの型（*ast.Ellipsis）をそのまま印字すると
// "...T" になる（AC-3-8）。0個は "()"（AC-3-8）。
func fieldListTypesString(fset *token.FileSet, ti typeInfo, fl *ast.FieldList) string {
	if fl == nil {
		return "()"
	}
	var parts []string
	for _, field := range fl.List {
		typeStr := normalizeWhitespace(printNode(fset, typeExprSignature(fset, ti, field.Type)))
		n := len(field.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			parts = append(parts, typeStr)
		}
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// stripFieldListNames は関数型の引数リスト・結果リスト用: 各フィールドの
// Names を落とし、Type は再帰的に typeExprSignature を適用したコピーへ
// 差し替える。fl 自身・fl.List の要素は書き換えない（新しい FieldList /
// Field を作って返す）。typeExprSignature の FuncType ケースから呼ばれる
// ほか、fieldListTypesString が関数の引数・結果型を組み立てる際にも
// typeExprSignature 経由で使われる。
//
// 複数名をまとめた宣言（`a, b int`）は、名前の数だけ Field を展開する。
// AC-3-6 が出力しないと定めるのは「引数名・結果名・受信者変数名」であって
// 「引数そのもの」ではなく、AC-3-8 は n 個の引数が n 個の型として現れる
// ことを要求する。この規則は fieldListTypesString（トップレベル関数の
// 引数・結果を組み立てる側）に既に入っており、本関数はそれと同じ規則を、
// stripFieldListNames を通るすべての経路（interface のメソッド引数、
// 構造体フィールドの関数型の引数・結果、関数の引数位置に現れる関数型、
// 型パラメータ制約に現れる関数型）へも一様に掛ける（Issue #93 reviewer
// 往復7 の指摘 C-7-1: 経路ごとに個別対応すると適用漏れが再発するため、
// 名前の個数で場合分けせず「名前の数だけ Field を作る」ループ1本で
// 両経路に同じ規則を掛ける）。展開後は Names を nil にする（引数名は
// AC-3-6 により出力しない — フィールド名を残す filterFieldList の
// AC-3-9-1 とは扱いが違う）。
//
// Opening / Closing はここでは落とさない。位置の一律落としは printNode が
// 印字直前に汎用処理として一度だけ適用する（stripPositions。Issue #93
// reviewer 往復8 で人間が承認した設計）。引数リスト・結果リストは
// filterFieldList と違い、AC-3-8 の "カンマ + 空白1つ" の区切りをそのまま
// go/printer に出させる（本項の対象外。区切りは 3-8 が既に持つ）ため、
// FieldList は go/printer にまるごと渡す設計を維持する。
func stripFieldListNames(fset *token.FileSet, ti typeInfo, fl *ast.FieldList) *ast.FieldList {
	if fl == nil {
		return nil
	}
	nfl := *fl
	var newList []*ast.Field
	for _, f := range fl.List {
		typ := typeExprSignature(fset, ti, f.Type)
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			nf := *f
			nf.Names = nil
			nf.Type = typ
			newList = append(newList, &nf)
		}
	}
	nfl.List = newList
	return &nfl
}

// typeParamsString は "[T any, U comparable]" の形を作る。型パラメータの
// 名前はここでは剥がさない（引数名とは異なり、型パラメータ名自体が
// シグネチャの一部であるため。AC-9-7）。制約の型式には typeExprSignature
// を適用し、制約が関数型のとき内側の引数名（AC-3-6）を剥がす
// （Issue #93 reviewer 往復2 の指摘 C-1r-(b)）。
//
// AC-3-8-1: まとめて宣言された型パラメータ（`[T, U any]`）は、名前ごとに
// 分けて宣言した形（`[T any, U any]`）と展開後にバイト一致させる。
// filterFieldList / fieldListTypesString / stripFieldListNames が既に
// 採っている「名前の数だけ Field 相当の要素を作る」ループと同じ形へ揃え、
// 個数で場合分けしない（Issue #93 reviewer 往復8 で人間が承認した設計）。
func typeParamsString(fset *token.FileSet, ti typeInfo, fl *ast.FieldList) string {
	var parts []string
	for _, field := range fl.List {
		typeStr := normalizeWhitespace(printNode(fset, typeExprSignature(fset, ti, field.Type)))
		if len(field.Names) == 0 {
			parts = append(parts, typeStr)
			continue
		}
		for _, n := range field.Names {
			parts = append(parts, n.Name+" "+typeStr)
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// printNode は go/printer の出力をそのまま返す（正規化前）。
//
// go/printer に渡す直前に stripPositions（下記）を通し、node のコピー内の
// あらゆる token.Pos を token.NoPos にする（AC-3-7-1。Issue #93 reviewer
// 往復8 で人間が承認した設計）。<signature> に現れるすべての位置
// （struct/interface のフィールドリストの括弧・型引数リストの大かっこ・
// 修飾識別子の内部・配列長の複合リテラルの内部を含む、あらゆる入れ子位置）
// を一様にこの1箇所で処理することで、構文位置ごとに Opening/Closing や
// Lbrack/Rbrack を個別に NoPos 化する対処（数え上げ漏れを構造的に残す）を
// やめる。stripPositions は node のディープコピーを作るだけで、node
// 自身・node から辿れる既存の AST は一切書き換えない。
//
// go/printer が返すエラーは、渡すノードが nil や不正な型のときにしか
// 起こらない。呼び出し側はいずれも構文解析済みの AST から取り出したノード
// だけを渡すため、ここでは無視して空文字列を返す（extractRecords 全体と
// しては AC-3-14 が「構文解析の失敗」を別途エラーとして捕まえており、
// 正規化前の印字失敗を握り潰しても検査の意図を損なわない）。
func printNode(fset *token.FileSet, node any) string {
	var buf bytes.Buffer
	stripped := stripPositions(reflect.ValueOf(node))
	var toPrint any
	if stripped.IsValid() {
		toPrint = stripped.Interface()
	}
	if err := printer.Fprint(&buf, fset, toPrint); err != nil {
		return ""
	}
	return buf.String()
}

// posType は token.Pos そのものの reflect.Type（stripPositions が
// フィールドの型を突き合わせるための基準）。
var posType = reflect.TypeOf(token.NoPos)

// stripPositions は v のディープコピーを返し、コピー内のあらゆる
// token.Pos 型フィールドを token.NoPos にする（AC-3-7-1 / AC-3-8-1 が
// 要求する「<signature> は元ソースの行取りで変わらない」を、go/printer へ
// 渡す直前の一箇所で一様に満たすための汎用処理。printNode のコメント参照）。
// v 自身が指す既存のノードは一切書き換えない —— 新しいコピーだけを作る。
//
// extractRecords は parser.ParseFile を parser.SkipObjectResolution 付きで
// 呼ぶため、Ident.Obj は常に nil で、Object.Decl 経由で元の宣言ノードへ
// 戻る循環参照は構造的に生じない（stripPositions・recurseExprFields の
// どちらも *ast.Object / *ast.Scope を特別扱いせず、他のポインタと同じ
// 経路でそのまま再帰できる）。
//
// reflect の使用は標準ライブラリの範囲内であり、cmd/exportlist は domain
// ではないため AC-3-11 / AC-3-12・check-domain-deps・ADR 0007 のいずれにも
// 抵触しない（services/api/go.mod の require は増やしていない）。
func stripPositions(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}
	if v.Type() == posType {
		return reflect.ValueOf(token.NoPos)
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		nv := reflect.New(v.Type().Elem())
		nv.Elem().Set(stripPositions(v.Elem()))
		return nv
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		nv := reflect.New(v.Type()).Elem()
		nv.Set(stripPositions(v.Elem()))
		return nv
	case reflect.Struct:
		nv := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			nv.Field(i).Set(stripPositions(v.Field(i)))
		}
		return nv
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		nv := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			nv.Index(i).Set(stripPositions(v.Index(i)))
		}
		return nv
	default:
		return v
	}
}

// printFieldMembers は keyword（"struct" / "interface"）と、既に
// filterFieldList を通した fl から "<keyword> { <member>; <member>; ... }"
// を組み立てる（AC-3-9-2: メンバー境界に区切り "; " を出す。Issue #93
// reviewer 往復8 で人間が承認した設計）。
//
// go/printer は複数フィールドを持つ struct/interface を必ず改行区切りで
// 出力し、normalizeWhitespace の空白畳み込みを通すと区切りの情報そのものが
// 失われる（printNode のコメント、filterFieldList のコメント参照）。この
// 偽 Green を避けるため、fl 全体を go/printer にまるごと渡すのをやめ、
// 各 Field を「その Field だけを持つ struct/interface」として個別に印字し
// （printSingleMember）、その結果を自前で "; " で連結する。
//
// 各 Field を個別に go/printer へ渡す（インターフェースのメソッドなら
// "func" キーワードを省いた "A() error" の形、構造体の関数型フィールドなら
// "Cb func(...) ..." の形にする判断を go/printer の既存の書式ロジックへ
// そのまま委ね、再実装しない）。この判断は struct のフィールドと
// interface のメソッドとで綴りが異なる（同じ *ast.Field{Names, *ast.FuncType}
// という表現でも、"struct" の文脈と "interface" の文脈で go/printer の
// 出力が変わる）ため、手書きで組み立て直すと綴りの再現漏れが起こる。
//
// メンバー0個は "<keyword> { }"、1個は区切りを出さず
// "<keyword> { <member> }"（AC-3-9-2: 境界の無いところに区切りを出さない）。
// 2個以上は "; " で連結する。
func printFieldMembers(fset *token.FileSet, keyword string, fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return keyword + " { }"
	}
	var parts []string
	for _, f := range fl.List {
		parts = append(parts, printSingleMember(fset, keyword, f))
	}
	return keyword + " { " + strings.Join(parts, "; ") + " }"
}

// printSingleMember は f だけを持つ struct/interface（keyword が示す方）を
// go/printer で印字し、"<keyword> { " / " }" の外枠を取り除いた中身
// （1メンバー分の綴り）を返す。fl の要素数が1個のときの go/printer の
// 書式（構造体フィールドの "名前 型"、インターフェースメソッドの
// "名前(引数) 結果"、埋め込みの "型" のみ、型集合要素の "~T" / "T1 | T2"
// など）をそのまま流用するための一手段（printFieldMembers のコメント
// 参照）。f 自身・f.Type は書き換えない（新しい FieldList / StructType /
// InterfaceType を作って渡すだけ）。
func printSingleMember(fset *token.FileSet, keyword string, f *ast.Field) string {
	singleFL := &ast.FieldList{List: []*ast.Field{f}}
	var node ast.Expr
	switch keyword {
	case "struct":
		node = &ast.StructType{Fields: singleFL}
	case "interface":
		node = &ast.InterfaceType{Methods: singleFL}
	}
	s := normalizeWhitespace(printNode(fset, node))
	s = strings.TrimPrefix(s, keyword+" {")
	s = strings.TrimSuffix(s, "}")
	return strings.TrimSpace(s)
}

var whitespaceRunRe = regexp.MustCompile(`\s+`)

// normalizeWhitespace は AC-3-7 の正規化: 改行・タブ・連続空白を空白1つへ
// 畳み、前後の空白を落とす。
func normalizeWhitespace(s string) string {
	return strings.TrimSpace(whitespaceRunRe.ReplaceAllString(s, " "))
}

// isExported は Go のエクスポート規則（先頭が Unicode 大文字）を満たすか
// 判定する（AC-3-4）。
func isExported(name string) bool {
	if name == "" {
		return false
	}
	r := []rune(name)[0]
	return unicode.IsUpper(r)
}
