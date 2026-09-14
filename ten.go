package ten

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

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
	for c.More() && c.stmt(&o) {
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
	if m := c.Mark(); c.Find("{{") || !c.More() {
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
		return c.closeTag()
	}
	return false
}

func (c *Compiler) closeTag() bool {
	c.ws()
	if c.Match("-") {
		return c.Exp("}}") && c.ws()
	}
	return c.Exp("}}")
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
	var lhs nom.Token
	var rhs AST
	if c.Undo(c.Mark(), (c.MatchOut(".", &lhs) || c.MatchOut(nom.WORD, &lhs)) && c.ws() && c.Match("=") && c.value(&rhs)) {
		*out = Assignment{LHS: Variable{lhs}, RHS: rhs}
		return true
	}
	return false
}

func (c *Compiler) forStmt(out *AST) bool {
	var list AST
	var stmt []AST
	var v, i nom.Token
	if c.Match("for") && c.forVar(&v, &i) && c.forList(&list) && c.closeTag() && c.stmts(&stmt) {
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

func (c *Compiler) forList(out *AST) bool {
	c.ws()
	return c.jsonValue(out) || c.variable(out)
}

func (c *Compiler) ifStmt(out *AST) bool {
	var cond AST
	var stmt []AST
	if c.Match("if") && c.expr(&cond) && c.closeTag() && c.stmts(&stmt) {
		var Then, Elze []AST
		Then = stmt
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

func (c *Compiler) expr(out *AST) bool {
	return c.boolCmpExpr(out)
}

func (c *Compiler) boolCmpExpr(out *AST) bool {
	var l, r AST
	if c.boolExpr(&l) {
		c.ws()
		var o nom.Token
		if (c.MatchOut("==", &o) || c.MatchOut("!=", &o)) && c.boolCmpExpr(&r) {
			*out = BoolExpr{o.Text, l, r}
			return true
		}
		if (c.MatchOut(">=", &o) || c.MatchOut("<=", &o) || c.MatchOut(">", &o) || c.MatchOut("<", &o)) && c.boolCmpExpr(&r) {
			*out = BoolExpr{o.Text, l, r}
			return true
		}
		*out = l
		return true
	}
	return false
}

func (c *Compiler) boolExpr(out *AST) bool {
	var l, r AST
	if c.boolAnd(&l) {
		c.ws()
		if c.Match("|") && c.boolExpr(&r) {
			*out = BoolExpr{"|", l, r}
			return true
		}
		*out = l
		return true
	}
	return false
}

func (c *Compiler) boolAnd(out *AST) bool {
	var l, r AST
	if c.boolFact(&l) {
		c.ws()
		if c.Match("&") && c.boolAnd(&r) {
			*out = BoolExpr{"&", l, r}
			return true
		}
		*out = l
		return true
	}
	return false
}

func (c *Compiler) boolFact(out *AST) bool {
	c.ws()
	return c.Match("(") && c.boolExpr(out) && c.Exp(")") || c.bool(out) || c.mathExpr(out)
}

func (c *Compiler) mathExpr(out *AST) bool {
	var l, r AST
	if c.mathTerm(&l) {
		c.ws()
		if c.Match("+") && c.mathExpr(&r) {
			*out = MathExpr{"+", l, r}
			return true
		}
		*out = l
		return true
	}
	return false
}

func (c *Compiler) mathTerm(out *AST) bool {
	var l, r AST
	if c.mathFact(&l) {
		c.ws()
		if c.Match("*") && c.mathTerm(&r) {
			*out = MathExpr{"*", l, r}
			return true
		}
		*out = l
		return true
	}
	return false
}

func (c *Compiler) mathFact(out *AST) bool {
	c.ws()
	return c.Match("(") && c.mathExpr(out) && c.Exp(")") || c.number(out) || c.variable(out)
}

func (c *Compiler) value(out *AST) bool {
	c.ws()
	return c.str(out) || c.jsonValue(out) || c.expr(out)
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

	if len(p) > 0 {
		*out = Path{Path: p}
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

func (c *Compiler) bool(out *AST) bool {
	var v nom.Token
	if c.MatchOut("true", &v) || c.MatchOut("false", &v) {
		*out = Literal[bool]{Token: v, Value: v.Text == "true"}
		return true
	}
	return false
}

func (c *Compiler) number(out *AST) bool {
	var v nom.Token
	if c.MatchOut(nom.DIGITS, &v) {
		f, _ := strconv.ParseFloat(v.Text, 64)
		*out = Literal[float64]{Token: v, Value: f}
		return true
	}
	return false
}

func (c *Compiler) str(out *AST) bool {
	var v nom.Token
	if c.MatchOut(nom.STRING, &v) {
		str, _ := strconv.Unquote(v.Text)
		*out = Literal[string]{Token: v, Value: str}
		return true
	}
	return false
}

func (c *Compiler) jsonValue(out *AST) bool {
	// Temporary naive implementation.
	if m := c.Mark(); c.naiveTag("{", "}") || c.naiveTag("[", "]") {
		var obj any
		if err := json.Unmarshal([]byte(c.Token(m).Text), &obj); err != nil {
			return c.Expected(err.Error())
		}
		*out = Literal[any]{Token: c.Token(m), Value: obj}
		return true
	}
	return false
}

func (p *Compiler) naiveTag(open, close string) bool {
	c := 1
	if p.Match(open) {
		for p.More() && c != 0 {
			switch {
			case p.Match(open):
				c++
			case p.Match(close):
				c--
			default:
				p.Next()
			}
		}
	}
	return c == 0
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

func (p *Path) Resolve(vars map[string]any) any {
	var v reflect.Value
	for i, p := range p.Path {
		if i == 0 {
			switch p := p.(type) {
			case Variable:
				v = reflect.ValueOf(vars[p.Name.Text])
			case Index:
				v = reflect.ValueOf(vars[p.Var.Text]).Index(p.Idx)
			}
			continue
		}
		if v.Kind() == reflect.Interface {
			v = v.Elem()
		}
		if v.Kind() == reflect.Pointer {
			v = reflect.Indirect(v)
		}
		switch v.Kind() {
		case reflect.Map:
			switch p := p.(type) {
			case Variable:
				v = v.MapIndex(reflect.ValueOf(p.Name.Text))
			case Index:
				v = v.MapIndex(reflect.ValueOf(p.Var.Text)).Elem().Index(p.Idx)
			}
		case reflect.Struct:
			switch p := p.(type) {
			case Variable:
				v = v.FieldByName(p.Name.Text)
			case Index:
				v = v.FieldByName(p.Var.Text).Index(p.Idx)
			}
		}
	}
	if v.Kind() == reflect.Invalid {
		return nil
	}
	return v.Interface()
}

type Variable struct {
	Name nom.Token
}

type Index struct {
	Var nom.Token
	Idx int
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
	List AST
	Stmt []AST
}

type If struct {
	Cond AST
	Then []AST
	Else []AST
}

type Literal[T any] struct {
	Token nom.Token
	Value T
}

type MathExpr struct {
	V string
	L AST
	R AST
}

func (b *MathExpr) Evaluate(vars map[string]any) float64 {
	switch b.V {
	case "+":
		return extractFloat64(b.L, vars) + extractFloat64(b.R, vars)
	case "*":
		return extractFloat64(b.L, vars) * extractFloat64(b.R, vars)
	}
	return 0
}

type BoolExpr struct {
	V string
	L AST
	R AST
}

func (b *BoolExpr) Evaluate(vars map[string]any) bool {
	switch b.V {
	case "&":
		return extractBool(b.L, vars) && extractBool(b.R, vars)
	case "|":
		return extractBool(b.L, vars) || extractBool(b.R, vars)
	case "==":
		return extractBool(b.L, vars) == extractBool(b.R, vars)
	case "!=":
		return extractBool(b.L, vars) != extractBool(b.R, vars)
	case ">=":
		return extractFloat64(b.L, vars) >= extractFloat64(b.R, vars)
	case ">":
		return extractFloat64(b.L, vars) > extractFloat64(b.R, vars)
	case "<=":
		return extractFloat64(b.L, vars) <= extractFloat64(b.R, vars)
	case "<":
		return extractFloat64(b.L, vars) < extractFloat64(b.R, vars)
	}
	return false
}

func extractBool(n AST, vars map[string]any) bool {
	switch v := n.(type) {
	case MathExpr:
		return v.Evaluate(vars) != 0
	case BoolExpr:
		return v.Evaluate(vars)
	case Path:
		return v.Resolve(vars).(bool)
	case Literal[float64]:
		return v.Value != 0
	case Literal[bool]:
		return v.Value
	}
	return false
}

func extractFloat64(n AST, vars map[string]any) float64 {
	switch v := n.(type) {
	case MathExpr:
		return v.Evaluate(vars)
	case BoolExpr:
		if v.Evaluate(vars) {
			return 1
		}
	case Path:
		return v.Resolve(vars).(float64)
	case Literal[float64]:
		return v.Value
	case Literal[bool]:
		if v.Value {
			return 1
		}
	}
	return 0
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
		var arr []any
		switch n := n.List.(type) {
		case Path:
			arr = n.Resolve(t.Variables).([]any)
		case Literal[any]:
			arr = n.Value.([]any)
		}
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
	case If:
		var ok bool
		switch cond := n.Cond.(type) {
		case BoolExpr:
			ok = cond.Evaluate(t.Variables)
		case Path:
			switch v := cond.Resolve(t.Variables).(type) {
			case bool:
				ok = v
			case float64:
				ok = v != 0
			case string:
				ok = len(v) > 0
			}
		case Literal[bool]:
			ok = cond.Value
		case Literal[float64]:
			ok = cond.Value != 0
		}
		stmts := n.Else
		if ok {
			stmts = n.Then
		}
		for _, stmt := range stmts {
			t.execute(stmt, w)
		}
	case Path:
		t.execute(n.Resolve(t.Variables), w)
	case Assignment:
		switch rhs := n.RHS.(type) {
		case Literal[string]:
			t.Variables[n.LHS.Name.Text] = rhs.Value
		case Literal[float64]:
			t.Variables[n.LHS.Name.Text] = rhs.Value
		case Literal[any]:
			t.Variables[n.LHS.Name.Text] = rhs.Value
		case Path:
			t.Variables[n.LHS.Name.Text] = rhs.Resolve(t.Variables)
		}
	case BoolExpr:
		t.execute(n.Evaluate(t.Variables), w)
	case MathExpr:
		t.execute(n.Evaluate(t.Variables), w)
	case Literal[bool]:
		fmt.Fprint(w, n.Token.Text)
	case Literal[string]:
		fmt.Fprint(w, n.Value)
	case Literal[float64]:
		fmt.Fprint(w, n.Token.Text)
	case Literal[any]:
		fmt.Fprint(w, n.Token.Text)
	case Text:
		fmt.Fprint(w, n.Value.Text)
	default:
		fmt.Fprint(w, n)
	}
}

func (t *Template) print(n AST, depth int) {
	switch n := n.(type) {
	case Template:
		t.print("Template [", depth)
		t.print(fmt.Sprintf("Variables: %v", t.Variables), depth+1)
		for _, stmt := range n.Statements {
			t.print(stmt, depth+1)
		}
		t.print("]", depth)
	case For:
		t.print("For [", depth)
		t.print(fmt.Sprintf("Var: '%s'", n.Var.Text), depth+1)
		t.print(fmt.Sprintf("Idx: '%s'", n.Idx.Text), depth+1)
		t.print("List [", depth)
		t.print(n.List, depth+1)
		t.print("]", depth)
		t.print("Body [", depth)
		for _, stmt := range n.Stmt {
			t.print(stmt, depth+1)
		}
		t.print("]", depth)
	case If:
		t.print("If [", depth)
		t.print(n.Cond, depth+1)
		t.print("]", depth)
		t.print("Then [", depth)
		for _, stmt := range n.Then {
			t.print(stmt, depth+1)
		}
		t.print("]", depth)
		t.print("Else [", depth)
		for _, stmt := range n.Else {
			t.print(stmt, depth+1)
		}
		t.print("]", depth)
	case Assignment:
		t.print("Assignment [", depth)
		t.print("LHS [", depth+1)
		t.print(n.LHS, depth+2)
		t.print("]", depth+1)
		t.print("RHS [", depth+1)
		t.print(n.RHS, depth+2)
		t.print("]", depth+1)
		t.print("]", depth)
	case BoolExpr:
		t.print("BoolExpr [", depth)
		t.print(fmt.Sprintf("Value: %v", n.V), depth+1)
		t.print("Left [", depth+1)
		t.print(n.L, depth+2)
		t.print("]", depth+1)
		t.print("Right [", depth+1)
		t.print(n.R, depth+2)
		t.print("]", depth+1)
		t.print("]", depth)
	case MathExpr:
		t.print("MathExpr [", depth)
		t.print(fmt.Sprintf("Value: %v", n.V), depth+1)
		t.print("Left [", depth+1)
		t.print(n.L, depth+2)
		t.print("]", depth+1)
		t.print("Right [", depth+1)
		t.print(n.R, depth+2)
		t.print("]", depth+1)
		t.print("]", depth)
	case Literal[bool]:
		t.print(fmt.Sprintf("Literal[bool]: %v", n.Value), depth)
	case Literal[string]:
		t.print(fmt.Sprintf("Literal[string]: %v", n.Value), depth)
	case Literal[float64]:
		t.print(fmt.Sprintf("Literal[float64]: %v", n.Value), depth)
	case Literal[any]:
		t.print(fmt.Sprintf("Literal[any]: %v", n.Value), depth)
	case Text:
		t.print(fmt.Sprintf("Text: %v", n.Value.Text), depth)
	case Variable:
		t.print(fmt.Sprintf("Variable: %s", n.Name.Text), depth)
	case Path:
		t.print("Path [", depth)
		for _, stmt := range n.Path {
			t.print(stmt, depth+1)
		}
		t.print("]", depth)
	default:
		fmt.Printf("%s%v\n", strings.Repeat("    ", depth), n)
	}
}
