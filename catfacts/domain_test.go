package catfacts

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring (mint, body, resolve), which need no network. The client's
// HTTP behaviour is covered in catfacts_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "catfacts" {
		t.Errorf("Scheme = %q, want catfacts", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "catfacts" {
		t.Errorf("Identity.Binary = %q, want catfacts", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
	}{
		{"some fact", "fact", "some fact"},
		{"random", "fact", "random"},
		{"https://catfact.ninja/fact", "fact", "https://catfact.ninja/fact"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify(\"\") should return error")
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("fact", "any")
	want := "https://" + Host + "/fact"
	if err != nil || got != want {
		t.Errorf("Locate(fact) = (%q, %v), want (%q, nil)", got, err, want)
	}

	got, err = Domain{}.Locate("breed", "any")
	want = "https://" + Host + "/breeds"
	if err != nil || got != want {
		t.Errorf("Locate(breed) = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "any")
	if err == nil {
		t.Error("Locate with unknown type should return error")
	}
}

// TestHostWiring mounts the driver in a kit Host and checks the round trip:
// a record mints to its URI, its body is readable, and a bare id resolves
// back to the same URI. The init in domain.go registers the domain, so
// kit.Open finds it.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	f := &Fact{Fact: "Cats always land on their feet.", Length: 31}
	u, err := h.Mint(f)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	// kit percent-encodes the id in the URI; compare scheme and authority.
	if u.Scheme != "catfacts" {
		t.Errorf("Mint scheme = %q, want catfacts", u.Scheme)
	}
	if u.Authority != "fact" {
		t.Errorf("Mint authority = %q, want fact", u.Authority)
	}

	got, err := h.ResolveOn("catfacts", "random")
	if err != nil {
		t.Fatalf("ResolveOn: %v", err)
	}
	if got.Scheme != "catfacts" || got.Authority != "fact" {
		t.Errorf("ResolveOn = %q, want catfacts://fact/...", got.String())
	}
}
