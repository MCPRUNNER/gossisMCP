package types

import "testing"

func TestGetAllExecutables(t *testing.T) {
	execs := Executables{Tasks: []Task{{Name: "TaskA"}, {Name: "TaskB"}}}
	all := execs.GetAllExecutables()
	if len(all) != 2 {
		t.Fatalf("expected two executables, got %d", len(all))
	}
	if all[0].Name != "TaskA" || all[1].Name != "TaskB" {
		t.Fatalf("unexpected task order: %+v", all)
	}
}
