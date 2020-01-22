package command

import "testing"

func TestShell(t *testing.T) {
	output, err := NewLocal().RunCommand("ls")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(output)
}
