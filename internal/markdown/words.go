package markdown

// Words counts the words you type in a unit: runs of letters and digits
// outside skip cells.
func Words(u *Unit) int {
	if !u.Typeable() {
		return 0
	}
	n, in := 0, false
	for _, c := range u.Cells {
		w := !c.Skip && IsAlnum(c.R)
		if w && !in {
			n++
		}
		in = w
	}
	return n
}
