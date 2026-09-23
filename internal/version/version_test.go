package version

import "testing"

func TestParse(t *testing.T) {
	cases := map[string]string{
		"v1.34.2-eks-3025e55": "1.34.2",
		"3.5.5":               "3.5.5",
		"v2.0":                "2.0",
		"1.6.0-rc.1":          "1.6.0",
	}
	for in, want := range cases {
		v, ok := Parse(in)
		if !ok || v.String() != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", in, v.String(), ok, want)
		}
	}
	if _, ok := Parse("latest"); ok {
		t.Error("Parse(latest) should fail")
	}
}

func TestPrereleaseAndCompare(t *testing.T) {
	rc, _ := Parse("v1.6.0-rc.1")
	ga, _ := Parse("v1.6.0")
	eks, _ := Parse("v1.6.0-eks-abc")
	if !rc.Prerelease() || ga.Prerelease() || eks.Prerelease() {
		t.Fatal("unexpected prerelease detection")
	}
	if Compare(rc, ga) != -1 || Compare(ga, eks) != 0 {
		t.Fatal("unexpected compare result")
	}
	if CompareStrings("1.10.0", "1.9.3") != 1 {
		t.Fatal("expected numeric comparison")
	}
}

func TestMatches(t *testing.T) {
	if !Matches("1.34", "v1.34.2-eks-1") {
		t.Error("1.34 should match 1.34.2")
	}
	if Matches("1.34.1", "1.34.2") {
		t.Error("1.34.1 should not match 1.34.2")
	}
	if Matches("2", "1.9") {
		t.Error("2 should not match 1.9")
	}
	if Matches("2.0.1", "v2.0.1-rc.3") || !Matches("2.0.1-rc.3", "v2.0.1-rc.3") || !Matches("2.0", "2.0.1-rc.3") {
		t.Error("unexpected prerelease matching")
	}
}

func TestDisplay(t *testing.T) {
	for in, want := range map[string]string{"v2.0.1-rc.3": "2.0.1-rc.3", "v1.34.8-eks-3025e55": "1.34.8", "v2.0.0-rc-1a2b3c4d": "2.0.0-rc-1a2b3c4d"} {
		if v, _ := Parse(in); v.Display() != want {
			t.Errorf("Display(%q) = %q, want %q", in, v.Display(), want)
		}
	}
}
