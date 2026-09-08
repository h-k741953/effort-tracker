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
					sig = normalizeWhitespace(printNode(fset, typeExprSignature(vs.Type)))
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
func typeSpecSignature(fset *token.FileSet, ts *ast.TypeSpec) string {
	body := normalizeWhitespace(printNode(fset, typeExprSignature(ts.Type)))

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

// typeExprSignature は e のコピーを返し、<signature> に現れるあらゆる
// 入れ子位置へ一様に次の2つを適用する（Issue #93 reviewer 往復2の
// 指摘 W-2r: 経路ごとに個別対応すると適用漏れが再発するため、型式を
// 1回のコピー再帰で走査する単一の入口をここに設ける）。
//
//   - AC-3-9: 構造体の非公開フィールド／インターフェースの非公開メソッド
//     の除去
//   - AC-3-10: 埋め込みフィールドは、埋め込まれた型名の公開性で除去を判定
//   - AC-3-6: 関数の引数名・結果名の除去
//
// e 自身、および e から go/parser が返した既存の AST を辿って届く一切の
// ノードは書き換えない。変更が要る場所でだけ新しいノードを作って返し、
// 変更が不要な部分木はそのまま共有する（共有される部分木は以後どこからも
// 変更されないため安全）。この単一の入口は、type 宣言の右辺
// （typeSpecSignature）・var/const の宣言型（extractGenDecl）・関数の
// 引数型／結果型（fieldListTypesString）・型パラメータの制約
// （typeParamsString）のすべてから呼ばれる。printNode へ渡す直前は必ず
// ここを通す。
func typeExprSignature(e ast.Expr) ast.Expr {
	switch v := e.(type) {
	case nil:
		return nil
	case *ast.StructType:
		nv := *v
		nv.Fields = filterFieldList(v.Fields)
		return &nv
	case *ast.InterfaceType:
		nv := *v
		nv.Methods = filterFieldList(v.Methods)
		return &nv
	case *ast.FuncType:
		nv := *v
		nv.Params = stripFieldListNames(v.Params)
		nv.Results = stripFieldListNames(v.Results)
		return &nv
	case *ast.StarExpr:
		nv := *v
		nv.X = typeExprSignature(v.X)
		return &nv
	case *ast.ArrayType:
		nv := *v
		nv.Elt = typeExprSignature(v.Elt)
		return &nv
	case *ast.Ellipsis:
		nv := *v
		nv.Elt = typeExprSignature(v.Elt)
		return &nv
	case *ast.MapType:
		nv := *v
		nv.Key = typeExprSignature(v.Key)
		nv.Value = typeExprSignature(v.Value)
		return &nv
	case *ast.ChanType:
		nv := *v
		nv.Value = typeExprSignature(v.Value)
		return &nv
	case *ast.ParenExpr:
		nv := *v
		nv.X = typeExprSignature(v.X)
		return &nv
	case *ast.IndexExpr:
		nv := *v
		nv.X = typeExprSignature(v.X)
		nv.Index = typeExprSignature(v.Index)
		return &nv
	case *ast.IndexListExpr:
		nv := *v
		nv.X = typeExprSignature(v.X)
		newIndices := make([]ast.Expr, len(v.Indices))
		for i, idx := range v.Indices {
			newIndices[i] = typeExprSignature(idx)
		}
		nv.Indices = newIndices
		return &nv
	case *ast.BinaryExpr:
		// 型集合の union 項（`T1 | T2`）。AC-3-9 が除去を許すのは
		// インターフェースの非公開メソッド、AC-3-10 が除去を許すのは
		// 埋め込みフィールドだけであり、union 項はどちらでもないため
		// ここでは除去しない（Issue #93 reviewer 往復2 の指摘 C-2:
		// embeddedName が名前を解釈できないことを非公開と同一視して
		// 丸ごと消していた偽 Green の再発防止）。内部に関数型が現れうる
		// ため、名前剥がしだけは再帰する。
		nv := *v
		nv.X = typeExprSignature(v.X)
		nv.Y = typeExprSignature(v.Y)
		return &nv
	case *ast.UnaryExpr:
		// 型集合の `~T`。BinaryExpr と同じ理由で除去しない。
		nv := *v
		nv.X = typeExprSignature(v.X)
		return &nv
	default:
		// Ident・SelectorExpr など、名前を含む余地の無いノードはそのまま
		// 返す（コピー不要。呼び出し側もこれを書き換えない）。
		return e
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
// メンバーが残るケースの表記を変えないため）。fl は呼び出し側が新しく
// 割り当てたコピーであることを前提とする（元の AST を書き換えない）。
func collapseIfEmpty(fl *ast.FieldList) {
	if len(fl.List) == 0 {
		fl.Closing = fl.Opening
	}
}

// filterFieldList は fl（構造体のフィールドリスト、またはインターフェース
// のメソッド／型集合の要素リスト）のコピーを返す。AC-3-9 の非公開メンバー
// 除去・AC-3-10 の埋め込みフィールド除去を適用したうえで、残す各メンバーの
// 型に typeExprSignature を再帰適用する。fl 自身・fl.List の要素は一切
// 書き換えない（新しい FieldList / Field を作って返す）。
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
func filterFieldList(fl *ast.FieldList) *ast.FieldList {
	if fl == nil {
		return nil
	}
	nfl := *fl
	var newList []*ast.Field
	for _, f := range fl.List {
		if len(f.Names) == 0 {
			if name, ok := embeddableTypeName(f.Type); ok && !isExported(name) {
				continue
			}
			nf := *f
			nf.Type = typeExprSignature(f.Type)
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
		nf := *f
		nf.Names = keep
		nf.Type = typeExprSignature(f.Type)
		newList = append(newList, &nf)
	}
	nfl.List = newList
	collapseIfEmpty(&nfl)
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
		typeStr := normalizeWhitespace(printNode(fset, typeExprSignature(field.Type)))
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
func stripFieldListNames(fl *ast.FieldList) *ast.FieldList {
	if fl == nil {
		return nil
	}
	nfl := *fl
	newList := make([]*ast.Field, len(fl.List))
	for i, f := range fl.List {
		nf := *f
		nf.Names = nil
		nf.Type = typeExprSignature(f.Type)
		newList[i] = &nf
	}
	nfl.List = newList
	return &nfl
}

// typeParamsString は "[T any, U comparable]" の形を作る。型パラメータの
// 名前はここでは剥がさない（引数名とは異なり、型パラメータ名自体が
// シグネチャの一部であるため。AC-9-7）。制約の型式には typeExprSignature
// を適用し、制約が関数型のとき内側の引数名（AC-3-6）を剥がす
// （Issue #93 reviewer 往復2 の指摘 C-1r-(b)）。
func typeParamsString(fset *token.FileSet, fl *ast.FieldList) string {
	var parts []string
	for _, field := range fl.List {
		var names []string
		for _, n := range field.Names {
			names = append(names, n.Name)
		}
		typeStr := normalizeWhitespace(printNode(fset, typeExprSignature(field.Type)))
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
