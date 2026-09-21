package persistence

import (
	"os"
	"testing"
)

type sampleDoc struct {
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

func TestSaveLoadRoundTrip(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	in := sampleDoc{Name: "n", Items: []string{"a", "b"}}
	if err := s.Save("doc", in); err != nil {
		t.Fatal(err)
	}
	var out sampleDoc
	ok, err := s.Load("doc", &out)
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if out.Name != "n" || len(out.Items) != 2 {
		t.Fatalf("mismatch: %+v", out)
	}
}

func TestMissingDoc(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var out sampleDoc
	ok, err := s.Load("nope", &out)
	if err != nil || ok {
		t.Fatalf("expected miss, got ok=%v err=%v", ok, err)
	}
}

func TestCorruptDoc(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save("doc", sampleDoc{Name: "x"}); err != nil {
		t.Fatal(err)
	}
	// Corrupt the file behind the store's back.
	if err := os.WriteFile(dir+"/doc.json", []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out sampleDoc
	if _, err := s.Load("doc", &out); err == nil {
		t.Fatalf("expected corruption error")
	}
	// The other document must be unaffected.
	if err := s.Save("other", sampleDoc{Name: "y"}); err != nil {
		t.Fatal(err)
	}
	var other sampleDoc
	if ok, err := s.Load("other", &other); !ok || err != nil || other.Name != "y" {
		t.Fatalf("isolation broken: %+v %v", other, err)
	}
}
