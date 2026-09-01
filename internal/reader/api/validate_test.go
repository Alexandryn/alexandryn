package api_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/reader/api"
)

func TestValidateCFI(t *testing.T) {
	good := []string{
		"epubcfi(/6/4[chap01]!/4/2/1:0)",
		"epubcfi(/6/4!/4/10)",
		"epubcfi(/6/14[body01]!/4[p-3]/2/1:22)",
	}
	for _, c := range good {
		if err := api.ValidateCFI(c); err != nil {
			t.Fatalf("valid CFI %q rejected: %v", c, err)
		}
	}

	bad := map[string]string{
		"empty":         "",
		"no prefix":     "/6/4!/4/2",
		"no suffix":     "epubcfi(/6/4!/4/2",
		"unbalanced":    "epubcfi(/6/4[chap01!/4/2)",
		"script inside": "epubcfi(<script>alert(1)</script>)",
		"quote inside":  `epubcfi(/6/4"onerror=x)`,
	}
	for name, c := range bad {
		if err := api.ValidateCFI(c); domain.CategoryOf(err) != domain.InvalidInput {
			t.Fatalf("%s: expected InvalidInput, got %v", name, err)
		}
	}
}

func TestValidateCFI_RejectsOversized(t *testing.T) {
	huge := "epubcfi(" + string(make([]byte, 4000)) + ")"
	if err := api.ValidateCFI(huge); err == nil {
		t.Fatal("oversized CFI accepted")
	}
}

func TestCFISortsBefore(t *testing.T) {
	if !api.CFISortsBefore("epubcfi(/6/4!/4/2/1:0)", "epubcfi(/6/4!/4/2/1:22)") {
		t.Fatal("offset 0 should sort before offset 22")
	}
	if api.CFISortsBefore("epubcfi(/6/4!/4/10)", "epubcfi(/6/4!/4/2)") {
		t.Fatal("step 10 should not sort before step 2")
	}
	if api.CFISortsBefore("epubcfi(/6/4!/4/2)", "epubcfi(/6/4!/4/2)") {
		t.Fatal("equal CFIs — neither sorts before the other")
	}
}

func TestValidateDeviceID(t *testing.T) {
	if err := api.ValidateDeviceID("3f2504e0-4f89-41d3-9a0c-0305e82c3301"); err != nil {
		t.Fatalf("valid v4 UUID rejected: %v", err)
	}
	for _, bad := range []string{"", "not-a-uuid", "3f2504e0-4f89-11d3-9a0c-0305e82c3301", "3f2504e04f8941d39a0c0305e82c3301"} {
		if err := api.ValidateDeviceID(bad); domain.CategoryOf(err) != domain.InvalidInput {
			t.Fatalf("%q: expected InvalidInput, got %v", bad, err)
		}
	}
}

func TestValidateObservedEpochAndPercentage(t *testing.T) {
	if err := api.ValidateObservedEpoch(-1); err == nil {
		t.Fatal("negative epoch accepted")
	}
	if err := api.ValidateObservedEpoch(0); err != nil {
		t.Fatalf("0 epoch rejected: %v", err)
	}
	if err := api.ValidatePercentage(1.5); err == nil {
		t.Fatal("percentage > 1 accepted")
	}
	if err := api.ValidatePercentage(0.5); err != nil {
		t.Fatalf("0.5 rejected: %v", err)
	}
}
