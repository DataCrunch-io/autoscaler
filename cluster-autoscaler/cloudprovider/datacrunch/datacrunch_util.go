/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package datacrunch

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// Matches: [optional spaces][optional "export "][VAR][=][VALUE][optional inline comment]
var assignRe = regexp.MustCompile(`^\s*(export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*?)(\s+#.*)?$`)

// rewriteAss
type quoteStyle int

const (
	none quoteStyle = iota
	doubleQ
	singleQ
)

func patchScript(script []byte, envMap map[string]string) []byte {
	var out bytes.Buffer
	sc := bufio.NewScanner(bytes.NewReader(script))
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	inHeredoc := false
	heredocEnd := ""

	for sc.Scan() {
		line := sc.Text()

		// Detect heredoc start (e.g., <<'EOF', <<EOF, <<-EOF)
		if !inHeredoc {
			if del, ok := heredocStartDelimiter(line); ok {
				inHeredoc = true
				heredocEnd = del
				out.WriteString(line)
				out.WriteByte('\n')
				continue
			}
			// Not in heredoc: try to rewrite assignments
			out.WriteString(rewriteAssignment(line, envMap))
			out.WriteByte('\n')
			continue
		}

		// In heredoc: pass-through until terminator line matches exactly
		if strings.TrimRight(line, "\r\n") == heredocEnd {
			inHeredoc = false
			heredocEnd = ""
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	// Preserve final newline behavior of input
	return out.Bytes()
}

// Detects heredoc start and returns the normalized delimiter (without quotes).
func heredocStartDelimiter(line string) (string, bool) {
	// This pattern finds the first "<<", optional "-", optional quotes
	// and captures the delimiter token letters/numbers/underscore.
	// Examples matched: <<EOF, <<'EOF', <<-EOF, <<-'EOF'
	hd := regexp.MustCompile(`<<-?'?([A-Za-z0-9_]+)'?`)
	m := hd.FindStringSubmatch(line)
	if len(m) == 2 {
		return m[1], true
	}
	return "", false
}
func rewriteAssignment(line string, envMap map[string]string) string {
	m := assignRe.FindStringSubmatch(line)
	if m == nil {
		return line
	}
	exportPrefix := m[1]              // "export " or ""
	varName := m[2]                   // VAR
	rawVal := strings.TrimSpace(m[3]) // VALUE (may be quoted or empty)
	comment := ""
	if len(m) >= 5 && m[4] != "" {
		comment = m[4]
	}

	newVal, ok := envMap[varName]
	if !ok {
		return line
	}

	// Determine original quoting style
	quoted := quotingStyle(rawVal)

	// Build replacement with safe quoting
	repl := formatValueWithStyle(newVal, quoted)

	// If there was no quoting originally and the new value is safe as bare word, keep it bare.
	// Otherwise, use the computed quoting.
	if quoted == none && isBareWord(newVal) {
		repl = newVal
	}

	// Reconstruct line (preserve export and inline comment)
	var b strings.Builder
	if exportPrefix != "" {
		b.WriteString(strings.TrimRight(exportPrefix, " "))
		b.WriteByte(' ')
	}
	b.WriteString(varName)
	b.WriteByte('=')
	b.WriteString(repl)
	if comment != "" {
		b.WriteString(comment)
	}
	return b.String()
}
func quotingStyle(s string) quoteStyle {
	s = strings.TrimSpace(s)
	if s == "" {
		return none
	}
	// Remove potential trailing inline comment before checking quotes
	// (already done by regex), so here s is just the value.
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(stripCR(s), `"`) && len(s) >= 2 {
		return doubleQ
	}
	if strings.HasPrefix(s, `'`) && strings.HasSuffix(stripCR(s), `'`) && len(s) >= 2 {
		return singleQ
	}
	return none
}

func stripCR(s string) string {
	return strings.TrimRight(s, "\r")
}

func isBareWord(s string) bool {
	// Allow common path/addr chars: alnum, _, ., /, :, -, @
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			continue
		}
		switch r {
		case '_', '.', '/', ':', '-', '@':
			continue
		default:
			return false
		}
	}
	return true
}

func escapeDoubleQuotes(s string) string {
	// Escape backslash and double-quote. We also escape dollar to prevent accidental expansion.
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\', '"', '$':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func formatValueWithStyle(value string, style quoteStyle) string {
	switch style {
	case singleQ:
		// Single-quoted strings cannot contain single quotes. If present, fall back to double quotes.
		if strings.ContainsRune(value, '\'') {
			return `"` + escapeDoubleQuotes(value) + `"`
		}
		return `'` + value + `'`
	case doubleQ:
		return `"` + escapeDoubleQuotes(value) + `"`
	default:
		// No quoting originally; we’ll decide outside whether we can keep it bare.
		// Provide a double-quoted fallback if the caller wants to force quoting.
		return `"` + escapeDoubleQuotes(value) + `"`
	}
}

// check if the node labels has given label and value
func nodeHasLabel(node *v1.Node, label string, value string) bool {
	_, hasGpuLabel := node.Labels[label]
	return hasGpuLabel
}

func convertConfigLabelsToK8sLabels(labels []string, asg *Asg) string {
	if asg == nil {
		return ""
	}
	if len(labels) == 0 {
		labels = make([]string, 2)
	}

	labels = append(labels, fmt.Sprintf("%s=%s", GPULabel, asg.instanceType))
	labels = append(labels, fmt.Sprintf("%s=%s", nodeGroupLabel, asg.Name))

	_k8sLabels := strings.Join(labels, ",")
	return _k8sLabels
}

// parse format: min:max:instance-type:asg-name
func parseAsgSpec(spec string) (*DatacrunchAsgSpec, error) {
	klog.Infof("[DEBUG] Parsing ASG spec: %s", spec)
	parts := strings.Split(spec, ":")
	if len(parts) != 4 {
		klog.Errorf("[DEBUG] Invalid ASG spec format: expected 4 parts, got %d - %v", len(parts), parts)
		return nil, fmt.Errorf("invalid ASG spec: %s", spec)
	}

	instanceType := parts[2]
	asgName := parts[3]

	minSize, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid min size: %s", parts[0])
	}

	maxSize, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid max size: %s", parts[1])
	}

	validAsgName := regexp.MustCompile(`^[a-z0-9A-Z]+[a-z0-9A-Z\-\.\_]*[a-z0-9A-Z]+$|^[a-z0-9A-Z]{1}$`)
	if !validAsgName.MatchString(asgName) {
		return nil, fmt.Errorf("invalid ASG name: %s", asgName)
	}

	klog.Infof("[DEBUG] Parsed ASG spec successfully: min=%d, max=%d, instanceType=%s, name=%s",
		minSize, maxSize, instanceType, asgName)
	return &DatacrunchAsgSpec{
		minSize:      minSize,
		maxSize:      maxSize,
		instanceType: instanceType,
		name:         asgName,
	}, nil
}

// isGPUInstanceType determines if an instance type is GPU-based
func isGPUInstanceType(instanceType string) bool {
	// Common GPU instance type patterns
	// instanceType start with "CPU." will be CPU others will be GPU
	return !strings.HasPrefix(strings.ToUpper(instanceType), "CPU.")
}

// InstanceRefFromProviderId returns the InstanceRef from the provider ID
func instanceRefFromProviderId(providerId string) (*InstanceRef, error) {
	// trim datacrunchProviderIDPrefix from providerId
	providerIdBase := strings.TrimPrefix(providerId, datacrunchProviderIDPrefix)
	parts := strings.Split(providerIdBase, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid provider ID: %s", providerId)
	}
	return &InstanceRef{Hostname: parts[len(parts)-1], ProviderID: providerId}, nil
}

// extractAsgNameFromHostname extracts the ASG name from a hostname using the magic separator
// Hostname format: {asg-name}-{magic-number}-{location}-{timestamp}
// Example: as-test-1b20030v-77-FIN-03-1756144020 → "as-test-1b20030v"
// Also handles legacy format: {asg-name}-{location}-{timestamp} (for backward compatibility)
func extractAsgNameFromHostname(hostname string) (string, error) {
	// Try new format first with magic separator
	separator := fmt.Sprintf("-%s-", ASG_SEPARATOR_MAGIC_NUMBER)

	parts := strings.Split(hostname, separator)
	if len(parts) == 2 {
		asgName := parts[0]
		if asgName == "" {
			return "", fmt.Errorf("empty ASG name extracted from hostname: %s", hostname)
		}
		return asgName, nil
	}

	// For now, return error to force fallback to description-based lookup
	// TODO: Implement smarter legacy parsing if needed
	return "", fmt.Errorf("hostname does not contain magic separator '%s' and legacy parsing not implemented: %s", separator, hostname)
}

// safeDeref safely dereferences an int64 pointer, returning the value or "nil" if pointer is nil
func safeDeref(p *int64) interface{} {
	if p == nil {
		return "nil"
	}
	return *p
}
