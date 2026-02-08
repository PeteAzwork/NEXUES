package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/org/nexus/internal/models"
)

// snapshotSummary returns a compact one-line summary of a snapshot.
func snapshotSummary(s models.Snapshot) string {
	return fmt.Sprintf("[%s] %s %s → %d (hash: %s, seen: %d times, last: %s)",
		s.Hash[:8],
		s.Request.Method,
		s.Request.Path,
		s.Response.StatusCode,
		s.Hash,
		s.Metadata.OccurrenceCount,
		s.Metadata.LastSeen.Format("2006-01-02 15:04:05"),
	)
}

// snapshotToText returns a human-readable text representation of a snapshot.
func snapshotToText(s *models.Snapshot, section string) string {
	var b strings.Builder

	switch section {
	case "request_only":
		writeRequest(&b, s)
	case "response_only":
		writeResponse(&b, s)
	case "attributes_only":
		writeAttributes(&b, s)
	case "metadata_only":
		writeMetadata(&b, s)
	default:
		fmt.Fprintf(&b, "=== Snapshot %s ===\n\n", s.Hash)
		writeRequest(&b, s)
		b.WriteString("\n")
		writeResponse(&b, s)
		b.WriteString("\n")
		writeAttributes(&b, s)
		b.WriteString("\n")
		writeMetadata(&b, s)
	}

	return b.String()
}

func writeRequest(b *strings.Builder, s *models.Snapshot) {
	fmt.Fprintf(b, "--- Request ---\n")
	fmt.Fprintf(b, "Method: %s\n", s.Request.Method)
	fmt.Fprintf(b, "Path:   %s\n", s.Request.Path)
	if len(s.Request.Headers) > 0 {
		b.WriteString("Headers:\n")
		for k, v := range s.Request.Headers {
			fmt.Fprintf(b, "  %s: %s\n", k, v)
		}
	}
	if s.Request.Body != nil {
		body, _ := json.MarshalIndent(s.Request.Body, "  ", "  ")
		fmt.Fprintf(b, "Body:\n  %s\n", string(body))
	}
}

func writeResponse(b *strings.Builder, s *models.Snapshot) {
	fmt.Fprintf(b, "--- Response ---\n")
	fmt.Fprintf(b, "Status: %d\n", s.Response.StatusCode)
	if len(s.Response.Headers) > 0 {
		b.WriteString("Headers:\n")
		for k, v := range s.Response.Headers {
			fmt.Fprintf(b, "  %s: %s\n", k, v)
		}
	}
	if s.Response.Body != nil {
		body, _ := json.MarshalIndent(s.Response.Body, "  ", "  ")
		fmt.Fprintf(b, "Body:\n  %s\n", string(body))
	}
}

func writeAttributes(b *strings.Builder, s *models.Snapshot) {
	fmt.Fprintf(b, "--- Attributes ---\n")
	writeAttrMap(b, "Subject", s.AttributeState.Subject)
	writeAttrMap(b, "Resource", s.AttributeState.Resource)
	writeAttrMap(b, "Environment", s.AttributeState.Environment)
	writeAttrMap(b, "Context", s.AttributeState.Context)
}

func writeAttrMap(b *strings.Builder, name string, m map[string]interface{}) {
	if len(m) == 0 {
		return
	}
	fmt.Fprintf(b, "  %s:\n", name)
	for k, v := range m {
		fmt.Fprintf(b, "    %s: %v\n", k, v)
	}
}

func writeMetadata(b *strings.Builder, s *models.Snapshot) {
	fmt.Fprintf(b, "--- Metadata ---\n")
	fmt.Fprintf(b, "Hash:        %s\n", s.Hash)
	fmt.Fprintf(b, "First Seen:  %s\n", s.Metadata.FirstSeen.Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(b, "Last Seen:   %s\n", s.Metadata.LastSeen.Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(b, "Occurrences: %d\n", s.Metadata.OccurrenceCount)
}

// snapshotToJSON returns a formatted JSON representation.
func snapshotToJSON(s *models.Snapshot) string {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal snapshot: %s"}`, err.Error())
	}
	return string(data)
}

// statsToText formats snapshot statistics as human-readable text.
func statsToText(stats *models.SnapshotStats) string {
	var b strings.Builder

	fmt.Fprintf(&b, "=== Snapshot Statistics ===\n\n")
	fmt.Fprintf(&b, "Total Snapshots: %d\n\n", stats.TotalCount)

	if len(stats.ByMethod) > 0 {
		b.WriteString("By HTTP Method:\n")
		for method, count := range stats.ByMethod {
			fmt.Fprintf(&b, "  %-8s %d\n", method, count)
		}
		b.WriteString("\n")
	}

	if len(stats.ByStatusRange) > 0 {
		b.WriteString("By Status Range:\n")
		for _, r := range []string{"2xx", "3xx", "4xx", "5xx"} {
			if count, ok := stats.ByStatusRange[r]; ok {
				fmt.Fprintf(&b, "  %-8s %d\n", r, count)
			}
		}
		b.WriteString("\n")
	}

	if len(stats.TopEndpoints) > 0 {
		b.WriteString("Top Endpoints:\n")
		for i, ep := range stats.TopEndpoints {
			fmt.Fprintf(&b, "  %d. %s (%d occurrences, %d unique)\n",
				i+1, ep.Path, ep.TotalOccurrences, ep.UniqueSnapshots)
		}
		b.WriteString("\n")
	}

	if !stats.TimeRange.Earliest.IsZero() {
		fmt.Fprintf(&b, "Time Range: %s to %s\n",
			stats.TimeRange.Earliest.Format("2006-01-02 15:04:05"),
			stats.TimeRange.Latest.Format("2006-01-02 15:04:05"))
	}

	return b.String()
}

// diffSnapshots compares two snapshots and returns a human-readable diff.
func diffSnapshots(a, b *models.Snapshot, sections string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "=== Diff: %s vs %s ===\n\n", a.Hash[:8], b.Hash[:8])

	showAll := sections == "" || sections == "all"

	if showAll || sections == "request" {
		sb.WriteString("--- Request ---\n")
		if a.Request.Method != b.Request.Method {
			fmt.Fprintf(&sb, "  Method: %s → %s\n", a.Request.Method, b.Request.Method)
		}
		if a.Request.Path != b.Request.Path {
			fmt.Fprintf(&sb, "  Path: %s → %s\n", a.Request.Path, b.Request.Path)
		}
		diffHeaders(&sb, "  Request Headers", a.Request.Headers, b.Request.Headers)
		diffBody(&sb, "  Request Body", a.Request.Body, b.Request.Body)
		sb.WriteString("\n")
	}

	if showAll || sections == "response" {
		sb.WriteString("--- Response ---\n")
		if a.Response.StatusCode != b.Response.StatusCode {
			fmt.Fprintf(&sb, "  Status: %d → %d\n", a.Response.StatusCode, b.Response.StatusCode)
		}
		diffHeaders(&sb, "  Response Headers", a.Response.Headers, b.Response.Headers)
		diffBody(&sb, "  Response Body", a.Response.Body, b.Response.Body)
		sb.WriteString("\n")
	}

	if showAll || sections == "attributes" {
		sb.WriteString("--- Attributes ---\n")
		diffAttrMap(&sb, "  Subject", a.AttributeState.Subject, b.AttributeState.Subject)
		diffAttrMap(&sb, "  Resource", a.AttributeState.Resource, b.AttributeState.Resource)
		diffAttrMap(&sb, "  Environment", a.AttributeState.Environment, b.AttributeState.Environment)
		diffAttrMap(&sb, "  Context", a.AttributeState.Context, b.AttributeState.Context)
		sb.WriteString("\n")
	}

	if showAll || sections == "metadata" {
		sb.WriteString("--- Metadata ---\n")
		fmt.Fprintf(&sb, "  Hash A:        %s\n", a.Hash)
		fmt.Fprintf(&sb, "  Hash B:        %s\n", b.Hash)
		fmt.Fprintf(&sb, "  Occurrences A: %d\n", a.Metadata.OccurrenceCount)
		fmt.Fprintf(&sb, "  Occurrences B: %d\n", b.Metadata.OccurrenceCount)
	}

	return sb.String()
}

func diffHeaders(sb *strings.Builder, label string, a, b map[string]string) {
	allKeys := make(map[string]bool)
	for k := range a {
		allKeys[k] = true
	}
	for k := range b {
		allKeys[k] = true
	}

	changes := false
	for k := range allKeys {
		va, oka := a[k]
		vb, okb := b[k]
		if oka && !okb {
			if !changes {
				fmt.Fprintf(sb, "%s:\n", label)
				changes = true
			}
			fmt.Fprintf(sb, "    - %s: %s\n", k, va)
		} else if !oka && okb {
			if !changes {
				fmt.Fprintf(sb, "%s:\n", label)
				changes = true
			}
			fmt.Fprintf(sb, "    + %s: %s\n", k, vb)
		} else if va != vb {
			if !changes {
				fmt.Fprintf(sb, "%s:\n", label)
				changes = true
			}
			fmt.Fprintf(sb, "    ~ %s: %s → %s\n", k, va, vb)
		}
	}
}

func diffBody(sb *strings.Builder, label string, a, b interface{}) {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		fmt.Fprintf(sb, "%s changed:\n", label)
		fmt.Fprintf(sb, "    - %s\n", string(ja))
		fmt.Fprintf(sb, "    + %s\n", string(jb))
	}
}

func diffAttrMap(sb *strings.Builder, label string, a, b map[string]interface{}) {
	allKeys := make(map[string]bool)
	for k := range a {
		allKeys[k] = true
	}
	for k := range b {
		allKeys[k] = true
	}

	changes := false
	for k := range allKeys {
		va, oka := a[k]
		vb, okb := b[k]
		if oka && !okb {
			if !changes {
				fmt.Fprintf(sb, "%s:\n", label)
				changes = true
			}
			fmt.Fprintf(sb, "    - %s: %v\n", k, va)
		} else if !oka && okb {
			if !changes {
				fmt.Fprintf(sb, "%s:\n", label)
				changes = true
			}
			fmt.Fprintf(sb, "    + %s: %v\n", k, vb)
		} else if fmt.Sprintf("%v", va) != fmt.Sprintf("%v", vb) {
			if !changes {
				fmt.Fprintf(sb, "%s:\n", label)
				changes = true
			}
			fmt.Fprintf(sb, "    ~ %s: %v → %v\n", k, va, vb)
		}
	}
}
