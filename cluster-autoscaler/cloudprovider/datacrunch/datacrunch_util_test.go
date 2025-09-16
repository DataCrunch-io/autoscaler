package datacrunch

import (
	"strings"
	"testing"
)

func TestParseAsgSpec_Valid(t *testing.T) {
	spec := "1:10:CPU.c2m4:asg-test"
	got, err := parseAsgSpec(spec)
	if err != nil {
		t.Fatalf("parseAsgSpec returned error: %v", err)
	}
	if got.minSize != 1 || got.maxSize != 10 || got.instanceType != "CPU.c2m4" || got.name != "asg-test" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestParseAsgSpec_InvalidFormat(t *testing.T) {
	_, err := parseAsgSpec("1:10:CPU.c2m4") // only 3 parts
	if err == nil {
		t.Fatalf("expected error for invalid format, got nil")
	}
}

func TestInstanceRefFromProviderId_Valid(t *testing.T) {
	pid := "datacrunch://FIN-03/asg-x-77-FIN-03-1700000000"
	ref, err := instanceRefFromProviderId(pid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.ProviderID != pid {
		t.Fatalf("providerID mismatch: %s", ref.ProviderID)
	}
	if ref.Hostname != "asg-x-77-FIN-03-1700000000" {
		t.Fatalf("hostname mismatch: %s", ref.Hostname)
	}
}

func TestInstanceRefFromProviderId_Invalid(t *testing.T) {
	_, err := instanceRefFromProviderId("datacrunch://malformed")
	if err == nil {
		t.Fatalf("expected error for malformed provider id, got nil")
	}
}

func TestExtractAsgNameFromHostname_NewFormat(t *testing.T) {
	host := "asg-prod-77-FIN-03-1700000000"
	name, err := extractAsgNameFromHostname(host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "asg-prod" {
		t.Fatalf("expected asg-prod, got %s", name)
	}
}

func TestExtractAsgNameFromHostname_NoMagic(t *testing.T) {
	_, err := extractAsgNameFromHostname("asg-prod-FIN-03-1700000000")
	if err == nil {
		t.Fatalf("expected error when magic separator is missing")
	}
}

func TestIsGPUInstanceType(t *testing.T) {
	if !isGPUInstanceType("GPU.A100.x1") {
		t.Fatalf("expected GPU type to be detected")
	}
	if isGPUInstanceType("CPU.c2m4") {
		t.Fatalf("expected CPU type not to be detected as GPU")
	}
}

func TestConvertConfigLabelsToK8sLabels(t *testing.T) {
	asg := &Asg{AsgRef: AsgRef{Name: "asg-x"}, instanceType: "CPU.c2m4"}
	labels := convertConfigLabelsToK8sLabels([]string{"env=prod"}, asg)
	// Should include our input, plus GPU label and node group label (using provider-specific keys)
	if !containsAll(labels, []string{"env=prod", GPULabel + "=CPU.c2m4", nodeGroupLabel + "=asg-x"}) {
		t.Fatalf("unexpected labels: %s", labels)
	}
}

// containsAll checks that every substring in wants is present in s
func containsAll(s string, wants []string) bool {
	for _, w := range wants {
		if !strings.Contains(s, w) {
			return false
		}
	}
	return true
}
