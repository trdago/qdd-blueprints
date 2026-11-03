// Package manifiesto compara las variables de entorno que lee el código con las que declara serverless.yml
// (BEM-C08). Una variable leída y no declarada no existe en la nube: el despliegue reemplaza todas las variables de
// la función por las del manifiesto.
//
// Uso típico, como prueba del proyecto:
//
//	leidas, _ := manifiesto.Leidas("../..")          // raíz del módulo
//	declaradas, _ := manifiesto.Declaradas("../../serverless.yml")
//	faltan := manifiesto.Faltan(leidas, declaradas, "AWS_LAMBDA_RUNTIME_API")
package manifiesto

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// funcionesQueLeen son las llamadas cuyo primer argumento literal es el nombre de una variable. Se amplía con las
// del cargador propio del proyecto (p. ej. «obligatoria», «secreto»).
var funcionesQueLeen = map[string]bool{
	"Getenv": true, "LookupEnv": true, "obligatoria": true, "secreto": true, "lista": true, "entero": true, "segundos": true,
}

// Leidas recorre los .go (sin _test.go ni vendor) y devuelve los nombres literales que el código lee.
func Leidas(raiz string) ([]string, error) {
	vistos := map[string]bool{}
	err := filepath.WalkDir(raiz, func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".")) && ruta != raiz {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(ruta, ".go") || strings.HasSuffix(ruta, "_test.go") {
			return nil
		}
		archivo, err := parser.ParseFile(token.NewFileSet(), ruta, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(archivo, func(n ast.Node) bool {
			llamada, ok := n.(*ast.CallExpr)
			if !ok || len(llamada.Args) == 0 || !funcionesQueLeen[nombreFuncion(llamada.Fun)] {
				return true
			}
			lit, ok := llamada.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if v, err := strconv.Unquote(lit.Value); err == nil && v != "" {
				vistos[v] = true
			}
			return true
		})
		return nil
	})
	return ordenadas(vistos), err
}

func nombreFuncion(e ast.Expr) string {
	switch f := e.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// Declaradas lee provider.environment y functions.*.environment de serverless.yml.
func Declaradas(serverlessYML string) ([]string, error) {
	data, err := os.ReadFile(serverlessYML)
	if err != nil {
		return nil, err
	}
	var m struct {
		Provider struct {
			Environment map[string]any `yaml:"environment"`
		} `yaml:"provider"`
		Functions map[string]struct {
			Environment map[string]any `yaml:"environment"`
		} `yaml:"functions"`
	}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	vistos := map[string]bool{}
	for k := range m.Provider.Environment {
		vistos[k] = true
	}
	for _, f := range m.Functions {
		for k := range f.Environment {
			vistos[k] = true
		}
	}
	return ordenadas(vistos), nil
}

// Faltan devuelve las leídas que no están declaradas, sin las que pone la plataforma (ignorar).
func Faltan(leidas, declaradas []string, ignorar ...string) []string {
	ok := map[string]bool{}
	for _, d := range declaradas {
		ok[d] = true
	}
	for _, i := range ignorar {
		ok[i] = true
	}
	var faltan []string
	for _, l := range leidas {
		if !ok[l] {
			faltan = append(faltan, l)
		}
	}
	return faltan
}

func ordenadas(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
