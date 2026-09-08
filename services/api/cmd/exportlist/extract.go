package main

// extract.go は docs/specs/public-api-diff-check.md AC-3（抽出規則）の実装
// である。構文解析だけで決まることしか見ない（型解決・依存解決・ビルド
// タグの評価を行わない。同 AC-3 前文、ADR 0018 決定7-3）。
//
// 依存は標準ライブラリの go/parser・go/ast・go/printer・go/token のみ
// （AC-3-11）。services/api/go.mod の require は1件も増やさない（AC-3-12）。
//
// トップレベルの識別子はすべて非公開にする（AC-3-15。抽出器自身が抽出対象
// に入る自己言及の帰結。詳細は main_test.go）。

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
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
	var records []record
	for _, f := range allGoFiles {
		if isExcludedPath(absDir, f) {
			continue
		}

		// ParseComments フラグを付けないため、コメントは AST に一切
		// 保持されない。printer がコメントを出力に混入させることは
		// 構造的に無い（AC-3-7「コメントを出力に含めない」）。
		astFile, perr := parser.ParseFile(fset, f, nil, 0)
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

		records = append(records, extractFromFile(fset, astFile, pkgPath)...)
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
// 非公開の宣言は対象外）。
func extractFromFile(fset *token.FileSet, file *ast.File, pkgPath string) []record {
	var out []record
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			out = append(out, extractFuncDecl(fset, d, pkgPath)...)
		case *ast.GenDecl:
			out = append(out, extractGenDecl(fset, d, pkgPath)...)
		}
	}
	return out
}

// extractFuncDecl は func 宣言（メソッドを含む）を扱う。
//
// AC-3-5: メソッドは受信者の基底型名とメソッド名の両方が公開のときのみ
// 対象とする。
func extractFuncDecl(fset *token.FileSet, d *ast.FuncDecl, pkgPath string) []record {
	if d.Recv == nil {
		if !isExported(d.Name.Name) {
			return nil
		}
		sig := funcTypeSignature(fset, d.Type, true)
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

	recvStr := normalizeWhitespace(printNode(fset, recvExpr))
	argsRes := funcTypeSignature(fset, d.Type, false)
	sig := "(" + recvStr + ") " + argsRes
	return []record{{
		Pkg:       pkgPath,
		Kind:      "method",
		Name:      baseName + "." + d.Name.Name,
		Signature: sig,
	}}
}

// extractGenDecl は type / var / const 宣言を扱う。
func extractGenDecl(fset *token.FileSet, d *ast.GenDecl, pkgPath string) []record {
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
				Signature: typeSpecSignature(fset, ts),
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
					sig = normalizeWhitespace(printNode(fset, vs.Type))
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
func typeSpecSignature(fset *token.FileSet, ts *ast.TypeSpec) string {
	filterUnexportedMembers(ts.Type)
	body := normalizeWhitespace(printNode(fset, ts.Type))

	var sb strings.Builder
	if ts.TypeParams != nil && len(ts.TypeParams.List) > 0 {
		sb.WriteString(typeParamsString(fset, ts.TypeParams))
		sb.WriteString(" ")
	}
	if ts.Assign != token.NoPos {
		sb.WriteString("= ")
	}
	sb.WriteString(body)
	return sb.String()
}

// filterUnexportedMembers は AC-3-9（構造体の非公開フィールド／インター
// フェースの非公開メソッドを除去する）と AC-3-10（埋め込みフィールドは
// 埋め込まれた型名の公開性で判定する）を、type 宣言の右辺（トップレベル）
// に対して適用する。t のフィールドリストをその場で書き換える。
func filterUnexportedMembers(t ast.Expr) {
	switch v := t.(type) {
	case *ast.StructType:
		if v.Fields != nil {
			v.Fields.List = filterStructFields(v.Fields.List)
			collapseIfEmpty(v.Fields)
		}
	case *ast.InterfaceType:
		if v.Methods != nil {
			v.Methods.List = filterInterfaceMembers(v.Methods.List)
			collapseIfEmpty(v.Methods)
		}
	}
}

// collapseIfEmpty は非公開メンバーの除去によって fl.List が空になった場合、
// Closing を Opening に合わせる。
//
// go/printer は構造体・インターフェースの中身を、AST 上の開き括弧・閉じ括弧
// の「元の位置（行）が同じかどうか」で1行表記と複数行表記のどちらにするか
// 決める。除去によって List が空になっても、元の宣言が複数行だった場合の
// Opening/Closing の行番号はそのまま残るため、printer は「中身が無い複数行
// ブロック」として "struct {\n}" のように空白入りで印字してしまう
// （元から空の宣言 "struct{}" とは表記が食い違い、非公開メンバーを増減
// しただけで SIGNATURE_CHANGED の偽陽性を生む）。
//
// 除去後に List が空である場合に限り Closing を Opening に一致させ、
// printer に「同じ行」と認識させることで、元から空の宣言と同じ
// "struct{}" / "interface{}" 表記に揃える。メンバーが1つでも残る場合は
// 触らない（AC-3-7 の正規化の対象を「連続空白の畳み込み」に留め、
// メンバーが残るケースの表記を変えないため）。
func collapseIfEmpty(fl *ast.FieldList) {
	if len(fl.List) == 0 {
		fl.Closing = fl.Opening
	}
}

func filterStructFields(fields []*ast.Field) []*ast.Field {
	var out []*ast.Field
	for _, f := range fields {
		if len(f.Names) == 0 {
			// 埋め込みフィールド（AC-3-10）。
			if isExported(embeddedName(f.Type)) {
				out = append(out, f)
			}
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
		if len(keep) == len(f.Names) {
			out = append(out, f)
			continue
		}
		nf := *f
		nf.Names = keep
		out = append(out, &nf)
	}
	return out
}

func filterInterfaceMembers(fields []*ast.Field) []*ast.Field {
	var out []*ast.Field
	for _, f := range fields {
		if len(f.Names) == 0 {
			// 埋め込みインターフェース。
			if isExported(embeddedName(f.Type)) {
				out = append(out, f)
			}
			continue
		}
		if isExported(f.Names[0].Name) {
			out = append(out, f)
		}
	}
	return out
}

// embeddedName は埋め込みフィールドの型名を取り出す。修飾がある場合は
// 最終要素の識別子（AC-3-10）。ポインタ・型パラメータ実体化も剥がす。
func embeddedName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return v.Sel.Name
	case *ast.StarExpr:
		return embeddedName(v.X)
	case *ast.IndexExpr:
		return embeddedName(v.X)
	case *ast.IndexListExpr:
		return embeddedName(v.X)
	default:
		return ""
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
func funcTypeSignature(fset *token.FileSet, ft *ast.FuncType, includeTypeParams bool) string {
	var sb strings.Builder
	if includeTypeParams && ft.TypeParams != nil && len(ft.TypeParams.List) > 0 {
		sb.WriteString(typeParamsString(fset, ft.TypeParams))
		sb.WriteString(" ")
	}
	sb.WriteString(fieldListTypesString(fset, ft.Params))
	sb.WriteString(" ")
	sb.WriteString(fieldListTypesString(fset, ft.Results))
	return sb.String()
}

// fieldListTypesString は引数・結果それぞれの型だけを取り出し、
// "(<型>, <型>, ...)" の形にする（引数名・結果名は出力しない＝AC-3-6）。
// 複数名をまとめた宣言（`x, y int`）は名前の数だけ型を繰り返す。
// 可変長引数はフィールドの型（*ast.Ellipsis）をそのまま印字すると
// "...T" になる（AC-3-8）。0個は "()"（AC-3-8）。
func fieldListTypesString(fset *token.FileSet, fl *ast.FieldList) string {
	if fl == nil {
		return "()"
	}
	var parts []string
	for _, field := range fl.List {
		typeStr := normalizeWhitespace(printNode(fset, field.Type))
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

// typeParamsString は "[T any, U comparable]" の形を作る。型パラメータの
// 名前はここでは剥がさない（引数名とは異なり、型パラメータ名自体が
// シグネチャの一部であるため。AC-9-7）。
func typeParamsString(fset *token.FileSet, fl *ast.FieldList) string {
	var parts []string
	for _, field := range fl.List {
		var names []string
		for _, n := range field.Names {
			names = append(names, n.Name)
		}
		typeStr := normalizeWhitespace(printNode(fset, field.Type))
		if len(names) == 0 {
			parts = append(parts, typeStr)
			continue
		}
		parts = append(parts, strings.Join(names, ", ")+" "+typeStr)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// printNode は go/printer の出力をそのまま返す（正規化前）。
//
// go/printer が返すエラーは、渡すノードが nil や不正な型のときにしか
// 起こらない。呼び出し側はいずれも構文解析済みの AST から取り出したノード
// だけを渡すため、ここでは無視して空文字列を返す（extractRecords 全体と
// しては AC-3-14 が「構文解析の失敗」を別途エラーとして捕まえており、
// 正規化前の印字失敗を握り潰しても検査の意図を損なわない）。
func printNode(fset *token.FileSet, node any) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return ""
	}
	return buf.String()
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
