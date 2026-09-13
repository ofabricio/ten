package ten

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/ofabricio/nom"
)

func Compile(src string) (Template, error) {
	c := Compiler{Parser: nom.New(src)}
	var t Template
	c.Compile(&t)
	return t, c.Err
}

type Compiler struct {
	nom.Parser
}

func (c *Compiler) Compile(out *Template) bool {
	var t Template
	c.stmts(&t.Statements)
	*out = t
	return len(t.Statements) > 0
}

func (c *Compiler) stmts(out *[]AST) bool {
	var o AST
	for c.stmt(&o) {
		if _, ok := o.(End); ok {
			break
		}
		*out = append(*out, o)
	}
	return true
}

func (c *Compiler) stmt(out *AST) bool {
	return c.tag(out) || c.text(out)
}

func (c *Compiler) text(out *AST) bool {
	if m := c.Mark(); c.Find("{{") {
		*out = Text{Value: c.Token(m)}
		return true
	}
	return false
}

func (c *Compiler) tag(out *AST) bool {
	if c.Match("{{") && c.ws() && c.tagBody(out) {
		if _, ok := (*out).(End); ok {
			return true
		}
		return c.ws() && c.Exp("}}")
	}
	return false
}

func (c *Compiler) tagBody(out *AST) bool {
	return c.end(out) || c.elseStmt(out) || c.ifStmt(out) || c.forStmt(out) || c.assignment(out) || c.value(out)
}

func (c *Compiler) end(out *AST) bool {
	if c.Match("end") {
		*out = End{}
		return true
	}
	return false
}

func (c *Compiler) elseStmt(out *AST) bool {
	if c.Match("else") {
		*out = Else{}
		return true
	}
	return false
}

func (c *Compiler) assignment(out *AST) bool {
	var lhs, rhs AST
	if c.Undo(c.Mark(), c.variable(&lhs) && c.ws() && c.Match("=") && c.value(&rhs)) {
		*out = Assignment{LHS: lhs.(Variable), RHS: rhs}
		return true
	}
	return false
}

func (c *Compiler) literal(out *AST) bool {
	var v nom.Token
	if c.MatchOut("true", &v) || c.MatchOut("false", &v) {
		*out = Value{v.Text == "true"}
		return true
	}
	if c.MatchOut(nom.DIGITS, &v) || c.MatchOut(nom.STRING, &v) {
		*out = Value{v.Text}
		return true
	}
	return false
}

func (c *Compiler) variable(out *AST) bool {

	idxOrVar := func(v nom.Token) AST {
		if i := 0; c.idx(&i) {
			return Index{v, i}
		}
		return Variable{v}
	}

	var p []AST
	var v nom.Token

	if c.MatchOut(".", &v) {
		p = append(p, idxOrVar(v))
	}

	for c.Match(".") && c.ExpOut(nom.WORD, &v) {
		p = append(p, idxOrVar(v))
	}

	if c.MatchOut(nom.WORD, &v) {
		p = append(p, idxOrVar(v))
		for c.Match(".") && c.ExpOut(nom.WORD, &v) {
			p = append(p, idxOrVar(v))
		}
	}

	if len(p) > 1 {
		*out = Path{Path: p}
		return true
	}
	if len(p) == 1 {
		*out = p[0]
		return true
	}
	return false
}

func (c *Compiler) idx(out *int) bool {
	var i nom.Token
	if c.Match("[") && c.ExpOut(nom.DIGITS, &i) && c.Exp("]") {
		idx, _ := strconv.Atoi(i.Text)
		*out = idx
		return true
	}
	return false
}

func (c *Compiler) forStmt(out *AST) bool {
	var list AST
	var stmt []AST
	var v, i nom.Token
	if c.Match("for") && c.forVar(&v, &i) && c.value(&list) && c.Exp("}}") && c.stmts(&stmt) {
		*out = For{Var: v, Idx: i, List: list, Stmt: stmt}
		return true
	}
	return false
}

func (c *Compiler) forVar(v, i *nom.Token) bool {
	return c.ws() && c.MatchOut(nom.WORD, v) && c.forIdx(i) && c.ws() && c.Match(":") || true
}

func (c *Compiler) forIdx(v *nom.Token) bool {
	return c.ws() && c.Match(",") && c.ws() && c.MatchOut(nom.WORD, v) || true
}

func (c *Compiler) ifStmt(out *AST) bool {
	var cond AST
	var stmt []AST
	if c.Match("if") && c.value(&cond) && c.ws() && c.Exp("}}") && c.stmts(&stmt) {
		var Then, Elze []AST
		for i, s := range stmt {
			if _, ok := s.(Else); ok {
				Then = stmt[:i]
				Elze = stmt[i+1:]
				break
			}
		}
		*out = If{Cond: cond, Then: Then, Else: Elze}
		return true
	}
	return false
}

func (c *Compiler) value(out *AST) bool {
	c.ws()
	if c.literal(out) || c.variable(out) {
		return true
	}
	if m := c.Mark(); c.Find("}}") {
		var obj any
		if err := json.Unmarshal([]byte(c.Token(m).Text), &obj); err != nil {
			return c.Expected(err.Error())
		}
		*out = Value{obj}
		return true
	}
	return false
}

func (c *Compiler) ws() bool {
	return c.Match(nom.WS) || true
}

type AST any
type End struct{}
type Else struct{}

type Template struct {
	Statements []AST
	Variables  map[string]any
}

type Path struct {
	Path []AST
}

type Variable struct {
	Name nom.Token
}

type Index struct {
	Var nom.Token
	Idx int
}

type Value struct {
	Value any
}

type Text struct {
	Value nom.Token
}

type Assignment struct {
	LHS Variable
	RHS AST
}

type For struct {
	Var  nom.Token
	Idx  nom.Token
	List any
	Stmt []AST
}

type If struct {
	Cond AST
	Then []AST
	Else []AST
}

func (t *Template) Execute(v any, w io.Writer) {
	t.Variables = make(map[string]any)
	t.Variables["."] = v
	t.execute(*t, w)
}

func (t *Template) execute(n AST, w io.Writer) {
	switch n := n.(type) {
	case Template:
		for _, stmt := range n.Statements {
			t.execute(stmt, w)
		}
	case For:
		if list, ok := n.List.(Value); ok {
			if arr, ok := list.Value.([]any); ok {
				for i, item := range arr {
					ctx := t.Variables["."]
					t.Variables["."] = item
					t.Variables[n.Idx.Text] = i
					t.Variables[n.Var.Text] = item
					for _, stmt := range n.Stmt {
						t.execute(stmt, w)
					}
					t.Variables["."] = ctx
				}
			}
		}
	case If:
		switch cond := n.Cond.(type) {
		case bool:
			stmts := n.Else
			if cond {
				stmts = n.Then
			}
			for _, stmt := range stmts {
				t.execute(stmt, w)
			}
		case Value:
			n.Cond = cond.Value
			t.execute(n, w)
		case Variable:
			n.Cond = t.Variables[cond.Name.Text]
			t.execute(n, w)
		}
	case Path:
		var v reflect.Value
		switch p := n.Path[0].(type) {
		case Variable:
			v = reflect.ValueOf(t.Variables[p.Name.Text])
		case Index:
			v = reflect.ValueOf(t.Variables[p.Var.Text]).Index(p.Idx)
		}
		for _, p := range n.Path[1:] {
			if v.Kind() == reflect.Interface {
				v = v.Elem()
			}
			switch v.Kind() {
			case reflect.Map:
				switch p := p.(type) {
				case Variable:
					v = v.MapIndex(reflect.ValueOf(p.Name.Text))
				case Index:
					v = v.MapIndex(reflect.ValueOf(p.Var.Text)).Elem().Index(p.Idx)
				}
			}
		}
		t.execute(v.Interface(), w)
	case Assignment:
		switch rhs := n.RHS.(type) {
		case Value:
			t.Variables[n.LHS.Name.Text] = rhs.Value
		case Variable:
			t.Variables[n.LHS.Name.Text] = t.Variables[rhs.Name.Text]
		}
	case Index:
		if arr, ok := t.Variables[n.Var.Text].([]any); ok {
			t.execute(arr[n.Idx], w)
		}
	case Variable:
		t.execute(t.Variables[n.Name.Text], w)
	case Value:
		t.execute(n.Value, w)
	case Text:
		fmt.Fprint(w, n.Value.Text)
	default:
		fmt.Fprint(w, n)
	}
}
