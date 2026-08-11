package messagebubble

// setStyleProbesForTest swaps the terminal probes behind style resolution and
// returns a function restoring the previous ones. Any cached resolution is
// cleared on both swap and restore so cases cannot leak into each other.
func setStyleProbesForTest(isTerminal, isDark func() bool) func() {
	prevTerminal, prevDark := stdoutIsTerminal, backgroundIsDark
	stdoutIsTerminal, backgroundIsDark = isTerminal, isDark
	resetGlamourStyleForTest()
	return func() {
		stdoutIsTerminal, backgroundIsDark = prevTerminal, prevDark
		resetGlamourStyleForTest()
	}
}
