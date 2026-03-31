package main

// ConstFold performs a constant folding pass on the AST.
// If it finds a BinOpNode with both children being NumberNode, it folds the node to a single NumberNode.
func ConstFold(node Node) Node {
	switch n := node.(type) {
	case *BinOpNode:
		left := ConstFold(n.Left)
		right := ConstFold(n.Right)
		// Attempt fold if both sides are numbers
		lNum, lOk := left.(*NumberNode)
		rNum, rOk := right.(*NumberNode)
		if lOk && rOk {
			var v float64
			switch n.Op {
			case "+":
				v = lNum.Value + rNum.Value
			case "-":
				v = lNum.Value - rNum.Value
			case "*":
				v = lNum.Value * rNum.Value
			case "/":
				v = lNum.Value / rNum.Value
			default:
				return &BinOpNode{n.Op, left, right}
			}
			return &NumberNode{v}
		}
		return &BinOpNode{n.Op, left, right}
	default:
		return node
	}
}
