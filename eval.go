package main

import "fmt"

// Eval evaluates an expression AST given a mapping of variable names to values.
func Eval(node Node, env map[string]float64) (float64, error) {
	switch n := node.(type) {
	case *NumberNode:
		return n.Value, nil
	case *IdentNode:
		v, ok := env[n.Name]
		if !ok {
			return 0, fmt.Errorf("undefined variable %s", n.Name)
		}
		return v, nil
	case *BinOpNode:
		l, err := Eval(n.Left, env)
		if err != nil {
			return 0, err
		}
		r, err := Eval(n.Right, env)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case "+":
			return l + r, nil
		case "-":
			return l - r, nil
		case "*":
			return l * r, nil
		case "/":
			return l / r, nil
		default:
			return 0, fmt.Errorf("unknown op %s", n.Op)
		}
	default:
		return 0, fmt.Errorf("unknown node type %T", node)
	}
}
