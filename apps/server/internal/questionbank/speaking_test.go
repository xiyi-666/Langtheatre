package questionbank

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func fixture() Bank {
	b := Bank{Version: 1}
	for _, topic := range []string{"home", "study"} {
		for _, id := range []string{"1", "2", "3"} {
			b.Items = append(b.Items, Item{ID: topic + id, Part: 1, Topic: topic, Question: "What do you like about this?", Source: "test.pdf", SourcePage: 1})
		}
	}
	for _, group := range []string{"a", "b"} {
		b.Items = append(b.Items, Item{ID: group, Part: 2, Group: group, CueCard: "Describe a skill. You should say: what it is and why you like it.", Source: "test.pdf", SourcePage: 2})
		for _, id := range []string{"1", "2", "3"} {
			b.Items = append(b.Items, Item{ID: group + id, Part: 3, Group: group, Question: "How can people develop this skill?", Source: "test.pdf", SourcePage: 3})
		}
	}
	return b
}
func TestSelectPreservesPairedQuestions(t *testing.T) {
	b := fixture()
	for n := 0; n < 30; n++ {
		p, err := b.Select()
		if err != nil {
			t.Fatal(err)
		}
		if len(p) != 10 {
			t.Fatal(len(p))
		}
		group := p[6].QuestionID
		for _, q := range p[7:] {
			if q.Part != 3 || q.QuestionID[:1] != group {
				t.Fatal("mismatched follow-up")
			}
		}
	}
}
func TestSelectExcludingDoesNotRepeatPromptSet(t *testing.T) {
	b := fixture()
	excluded := map[string]struct{}{}
	for attempt := 0; attempt < 2; attempt++ {
		prompts, err := b.SelectExcluding(excluded)
		if err != nil {
			t.Fatal(err)
		}
		fingerprint := SpeakingSelectionFingerprint(prompts)
		if fingerprint == "" {
			t.Fatal("missing selection fingerprint")
		}
		if _, repeated := excluded[fingerprint]; repeated {
			t.Fatal("repeated an excluded prompt set")
		}
		excluded[fingerprint] = struct{}{}
	}
	if _, err := b.SelectExcluding(excluded); err == nil {
		t.Fatal("accepted selection after every complete set was excluded")
	}
}
func TestRejectIncompleteBank(t *testing.T) {
	if _, err := (Bank{}).Select(); err == nil {
		t.Fatal("accepted empty bank")
	}
}
func TestLoadValidation(t *testing.T) {
	b := fixture()
	path := filepath.Join(t.TempDir(), "bank.json")
	save := func() {
		raw, _ := json.Marshal(b)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save()
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	b.Items[0].AudioURL = "/media/../secret"
	save()
	if _, err := Load(path); err == nil {
		t.Fatal("accepted unsafe audio")
	}
	b.Items[0].AudioURL = ""
	b.Items[1].ID = b.Items[0].ID
	save()
	if _, err := Load(path); err == nil {
		t.Fatal("accepted duplicate")
	}
}
func TestLocalImportedBank(t *testing.T) {
	path := os.Getenv("TEST_SPEAKING_BANK")
	if path == "" {
		t.Skip("local licensed corpus not bundled with CI")
	}
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 100; n++ {
		p, e := b.Select()
		if e != nil {
			t.Fatal(e)
		}
		if len(p) < 10 {
			t.Fatal("not enough prompts")
		}
		for _, q := range p {
			if q.Source == "" || q.QuestionID == "" {
				t.Fatal("missing provenance")
			}
		}
	}
	t.Logf("validated %d licensed items and 100 selections", len(b.Items))
}
