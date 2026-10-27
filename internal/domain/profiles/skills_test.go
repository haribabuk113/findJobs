package profiles

import "testing"

func TestSkillSignature(t *testing.T) {
	a := SkillSignature([]SkillRef{{ID: 3}, {ID: 1}, {ID: 2}})
	if b := SkillSignature([]SkillRef{{ID: 1}, {ID: 2}, {ID: 3}}); a != b {
		t.Fatalf("order changed the signature: %s %s", a, b)
	}
	if c := SkillSignature([]SkillRef{{ID: 1}, {ID: 2}}); a == c {
		t.Fatal("removing a skill kept the signature")
	}
	if d := SkillSignature([]SkillRef{{ID: 12}, {ID: 3}}); d == SkillSignature([]SkillRef{{ID: 1}, {ID: 23}}) {
		t.Fatal("ids must be delimited")
	}
	if e := NewSkillSet(nil); e.Skills == nil || e.Signature != SkillSignature(nil) {
		t.Fatalf("empty set: %+v", e)
	}
}
