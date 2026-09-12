package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestSourceMessagesExistInEnglishCatalog(t *testing.T) {
	check := func(file, source string) {
		t.Helper()
		if _, ok := Get("en").Messages[source]; !ok {
			t.Errorf("%s: message is missing from the English catalog: %q", file, source)
		}
	}
	root := "../.."
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			index := -1
			switch function := call.Fun.(type) {
			case *ast.SelectorExpr:
				if object, ok := function.X.(*ast.Ident); ok && object.Name == "i18n" && function.Sel.Name == "Text" {
					index = 1
				}
				if function.Sel.Name == "problem" {
					index = 2
				}
			case *ast.Ident:
				if function.Name == "Invalid" || function.Name == "Conflict" {
					index = 0
				}
			}
			if index >= 0 && index < len(call.Args) {
				if literal, ok := call.Args[index].(*ast.BasicLit); ok && literal.Kind == token.STRING {
					if source, err := strconv.Unquote(literal.Value); err == nil {
						check(path, source)
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	javaLiteral := regexp.MustCompile(`I18n\.text\(("(?:\\.|[^"\\])*")`)
	paths, err := filepath.Glob(filepath.Join(root, "android/src/app/foodie/mobile/*.java"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range javaLiteral.FindAllSubmatch(data, -1) {
			source, err := strconv.Unquote(string(match[1]))
			if err != nil {
				t.Fatal(err)
			}
			check(path, source)
		}
	}
	path := filepath.Join(root, "public/app.js")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	jsLiteral := regexp.MustCompile(`\bt\('((?:\\.|[^'\\])*)'`)
	for _, match := range jsLiteral.FindAllSubmatch(data, -1) {
		raw := strings.ReplaceAll(string(match[1]), `\'`, `'`)
		source, err := strconv.Unquote(`"` + strings.ReplaceAll(raw, `"`, `\"`) + `"`)
		if err != nil {
			t.Fatal(err)
		}
		check(path, source)
	}
	path = filepath.Join(root, "public/index.html")
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, match := range regexp.MustCompile(`>([^<>]+)<`).FindAllSubmatch(data, -1) {
		source := html.UnescapeString(strings.TrimSpace(string(match[1])))
		if source == "" || source == "." || source == "?" || source == "✦" || source == "✧" {
			continue
		}
		isUnit := false
		for _, forms := range Get("en").Units {
			isUnit = isUnit || source == forms[1]
		}
		if isUnit {
			continue
		}
		check(path, source)
	}
}

func TestBrowserPackContainsOnlySelectedPresentation(t *testing.T) {
	for _, language := range []string{"en", "da"} {
		data := string(JSON(language))
		if strings.Contains(data, `"voice"`) || strings.Contains(data, `"numbers"`) || !strings.Contains(data, `"language":"`+language+`"`) {
			t.Fatal("browser received recognition grammar or a different language")
		}
	}
}
