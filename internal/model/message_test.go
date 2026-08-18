package model

import "testing"

func TestResidentBytes(t *testing.T) {
	m := &Message{
		Raw:  []byte("raw"),
		Text: "txt",
		HTML: "h",
		Attachments: []Attachment{
			{Data: []byte("ab"), Size: 2},
		},
	}
	want := int64(len(m.Raw) + len(m.Text) + len(m.HTML) + 2)
	if m.ResidentBytes() != want {
		t.Fatalf("got %d want %d", m.ResidentBytes(), want)
	}
	spilled := &Message{Size: 9, Text: "x"}
	if spilled.ResidentBytes() != 10 {
		t.Fatalf("spilled %d", spilled.ResidentBytes())
	}
}
