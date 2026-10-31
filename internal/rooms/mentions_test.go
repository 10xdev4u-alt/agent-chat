package rooms

import "testing"

func TestParseMentionsBasic(t *testing.T) {
	got := ParseMentions("hello @alice and @bob", []string{"alice", "bob", "carol"})
	want := []string{"alice", "bob"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestParseMentionsCodeBlock(t *testing.T) {
	body := "mention @alice in `@bob` and ```\n@carol\n```"
	got := ParseMentions(body, []string{"alice", "bob", "carol"})
	if len(got) != 1 || got[0] != "alice" {
		t.Fatalf("got %v", got)
	}
}

func TestParseMentionsLongestFirst(t *testing.T) {
	got := ParseMentions("hi @bobby", []string{"bob", "bobby"})
	if len(got) != 1 || got[0] != "bobby" {
		t.Fatalf("got %v", got)
	}
}

func TestParseMentionsAllHere(t *testing.T) {
	if !IsMentionAll("hey @all") {
		t.Fatal("expected all")
	}
	if !IsMentionHere("hey @here") {
		t.Fatal("expected here")
	}
	got := ParseMentions("hey @all @here @alice", []string{"alice"})
	if len(got) != 1 || got[0] != "alice" {
		t.Fatalf("got %v", got)
	}
}

func TestParseMentionsDuplicates(t *testing.T) {
	got := ParseMentions("@alice @alice @bob", []string{"alice", "bob"})
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}
