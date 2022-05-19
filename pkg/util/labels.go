package util

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"strconv"
	"strings"
)

const (
	idLogic = iota
	idOp
	idName
	idValue
	idEvald

	opAnd = 34
	opOr  = 35
)

func MatchLabels(matcher LabelsMatcher, labels map[string]any) bool {
	ok, err := matcher.MatchLabels(labels)
	if err != nil {
		log.Printf("E! eval labels failed: %v", err)
	}

	return ok
}

type LabelsMatcher interface {
	MatchLabels(labels map[string]any) (bool, error)
}

type trueEvalLabels struct{}

func (trueEvalLabels) MatchLabels(map[string]any) (bool, error) { return true, nil }

var (
	ErrNotSupportOperator = errors.New("operator doesn't support")
	ErrQuery              = errors.New("query error")
)

func ParseLabelsExpr(expr string) (LabelsMatcher, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return &trueEvalLabels{}, nil
	}

	e, err := parser.ParseExpr(expr)
	if err != nil {
		return nil, err
	}

	v := LabelVisitor{expr: expr}
	ast.Walk(&v, e)

	return &v, nil
}

func (v *LabelVisitor) MatchLabels(labels map[string]any) (bool, error) {
	var results []ident
	i := 0

	for {
		if len(v.idents[i:]) < 3 {
			break
		}

		if v.idents[i].vType == 0 {
			results = append(results, v.idents[i])
			i++
			continue
		}

		if relOpEval(v.idents[i:i+3], labels) {
			results = append(results, ident{value: "true", vType: idEvald})
		} else {
			results = append(results, ident{value: "false", vType: idEvald})
		}

		if i+3 < len(v.idents) {
			i = i + 3
		} else {
			break
		}
	}

	if len(results) < 1 {
		return false, ErrQuery
	}

	ok, err := binOpEval(&results)
	if err != nil {
		return ok, err
	}

	return ok, nil
}

type LabelVisitor struct {
	idents []ident
	expr   string
}

type ident struct {
	value string
	Kind  token.Token
	vType int

	intValue int64
}

func (v *LabelVisitor) Visit(n ast.Node) ast.Visitor {
	switch d := n.(type) {
	case *ast.Ident:
		v.idents = append(v.idents, ident{value: d.Name, vType: idName})
	case *ast.BasicLit:

		lit := ident{value: d.Value, Kind: d.Kind, vType: idValue}
		switch d.Kind {
		case token.INT, token.FLOAT:
			lit.intValue, _ = strconv.ParseInt(d.Value, 10, 64)
		}
		v.idents = append(v.idents, lit)
	case *ast.BinaryExpr:
		if d.Op == opAnd || d.Op == opOr {
			v.idents = append(v.idents, ident{value: d.Op.String(), vType: idLogic})
		} else {
			v.idents = append(v.idents, ident{value: d.Op.String(), vType: idOp})
		}
	}

	return v
}

func binOpEval(idents *[]ident) (bool, error) {
	i := 0
	for {
		if len(*idents) == 1 {
			r, _ := strconv.ParseBool((*idents)[0].value)
			return r, nil
		}

		if (*idents)[i].vType == 0 && (*idents)[i+1].vType == 0 {
			i++
			continue
		}

		if len(*idents) > 2 && (*idents)[i].vType == idLogic &&
			(*idents)[i+1].vType == idEvald && (*idents)[i+2].vType == idEvald {

			r1, _ := strconv.ParseBool((*idents)[i+1].value)
			r2, _ := strconv.ParseBool((*idents)[i+2].value)

			switch (*idents)[i].value {
			case "&&":
				(*idents)[i].value = strconv.FormatBool(r1 && r2)
			case "||":
				(*idents)[i].value = strconv.FormatBool(r1 || r2)
			default:
				return false, ErrNotSupportOperator
			}

			(*idents)[i].vType = idEvald
			if len(*idents) > 2 {
				*idents = append((*idents)[:i+1], (*idents)[i+3:]...)
				i = 0
				continue
			} else {
				tmp := (*idents)[0:1]
				*idents = tmp
			}
		}

		if len(*idents) > 4 {
			i += 2
		} else {
			return false, ErrNotSupportOperator
		}
	}
}

func relOpEval(idents []ident, labels map[string]any) bool {
	labelValue, ok := labels[idents[1].value]
	if !ok {
		return false
	}

	switch idents[2].Kind {
	case token.INT, token.FLOAT:
		f1, _ := strconv.ParseInt(fmt.Sprintf("%v", labelValue), 10, 64)
		switch idents[0].value {
		case "==":
			return f1 == idents[2].intValue
		case "!=":
			return f1 != idents[2].intValue
		case ">":
			return f1 > idents[2].intValue
		case ">=":
			return f1 >= idents[2].intValue
		case "<":
			return f1 < idents[2].intValue
		case "<=":
			return f1 <= idents[2].intValue
		}
	default:
		switch idents[0].value {
		case "==":
			return labelValue == idents[2].value
		case "!=":
			return labelValue != idents[2].value
		}
	}

	return false
}
