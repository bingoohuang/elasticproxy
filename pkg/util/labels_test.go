package util

import "testing"

// https://github.com/yahoo/vssh/blob/master/query.go

func TestQueryExprEval(t *testing.T) {
	labels := map[string]any{"POP": "LAX", "OS": "JUNOS", "Weight": 1}

	exprTests := []struct {
		expr     string
		expected bool
	}{
		{"POP==LAX", true},
		{"Weight==1", true},
		{"Weight>=1", true},
		{"POP!=LAX", false},
		{"POP==LAX && OS==JUNOS", true},
		{"POP==LAX && OS!=JUNOS", false},
		{"(POP==LAX || POP==BUR) && OS==JUNOS", true},
		{"OS==JUNOS && (POP==LAX || POP==BUR)", true},
		{"OS!=JUNOS && (POP==LAX || POP==BUR)", false},
		{"(OS==JUNOS) && (POP==LAX || POP==BUR)", true},
		{"((OS==JUNOS) && (POP==LAX || POP==BUR))", true},
	}

	for _, x := range exprTests {
		v, err := ParseLabelsExpr(x.expr)
		if err != nil {
			t.Fatal(err)
		}

		ok, err := v.MatchLabels(labels)
		if err != nil {
			t.Fatal(err)
		}

		if ok != x.expected {
			t.Fatalf("%s expect %t, got %t", x.expr, x.expected, ok)
		}
	}

	_, err := ParseLabelsExpr("OS=JUNOS")
	if err == nil {
		t.Fatal("expect error but got nil")
	}

	v, err := ParseLabelsExpr("OS")
	if err != nil {
		t.Fatal("expect error but got nil")
	}
	_, err = v.MatchLabels(labels)
	if err == nil {
		t.Fatal("expect error but got nil")
	}

	// not support operator
	ops := []string{"&", "+", "<=", "<"}
	for _, op := range ops {
		v, _ := ParseLabelsExpr("OS == JUNOS " + op + " POP == LAX")
		_, err = v.MatchLabels(labels)
		if err == nil {
			t.Fatal("expect error but got nil")
		}
	}
}

func BenchmarkQueryExprEval(b *testing.B) {
	labels := map[string]any{"POP": "LAX", "OS": "JUNOS"}
	expr := "POP==LAX"

	for i := 0; i < b.N; i++ {
		v, err := ParseLabelsExpr(expr)
		if err != nil {
			b.Fatal(err)
		}

		_, err = v.MatchLabels(labels)
		if err != nil {
			b.Fatal(err)
		}
	}
}
