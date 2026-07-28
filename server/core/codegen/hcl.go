// Package codegen implements the W2 MVP code generation engine: it converts
// form values + module contracts into Terraform/Terramate file trees.
//
// hcl.go implements the HCL value renderer. Go values (string/bool/number/
// map/list/nil) are rendered into valid HCL attribute expressions so that
// text/template can emit them verbatim into main.tf. This keeps templates free
// of type-awareness and guarantees the only place HCL syntax is decided is a
// single, well-tested function (D19 determinism).
//
// Rendering rules (mirrors cty → HCL literal syntax):
//   - string        → "value"   (double-quoted, HCL-escaped)
//   - bool          → true/false
//   - int/int64/float64 → 123 (unquoted)
//   - map[string]any → { k = v, k2 = v2 }   (HCL object)
//   - map[string]string → { k = "v" }
//   - []any         → ["a", "b"]            (HCL tuple/list)
//   - []string / []int / ... → same shape, elements rendered per type
//   - nil           → null
package codegen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// renderHCLValue renders a Go value into HCL attribute-value syntax.
//
// It is the single source of truth for how codegen turns a resolved parameter
// into HCL text. Templates reference it via the "renderHCL" FuncMap entry so
// the rendering logic never leaks into the template layer.
func renderHCLValue(val any) string {
	if val == nil {
		return "null"
	}
	switch v := val.(type) {
	case string:
		return hclString(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		// HCL numbers are decimal; render integers without trailing ".0".
		return renderFloat(v)
	case float32:
		return renderFloat(float64(v))
	case map[string]any:
		return renderHCLObject(v)
	case map[string]string:
		// Promote to map[string]any for uniform rendering.
		obj := make(map[string]any, len(v))
		for k, vv := range v {
			obj[k] = vv
		}
		return renderHCLObject(obj)
	case []any:
		return renderHCLList(v)
	case []string:
		out := make([]any, len(v))
		for i, vv := range v {
			out[i] = vv
		}
		return renderHCLList(out)
	case []int:
		out := make([]any, len(v))
		for i, vv := range v {
			out[i] = vv
		}
		return renderHCLList(out)
	case []int64:
		out := make([]any, len(v))
		for i, vv := range v {
			out[i] = vv
		}
		return renderHCLList(out)
	case []float64:
		out := make([]any, len(v))
		for i, vv := range v {
			out[i] = vv
		}
		return renderHCLList(out)
	case []bool:
		out := make([]any, len(v))
		for i, vv := range v {
			out[i] = vv
		}
		return renderHCLList(out)
	case []map[string]any:
		out := make([]any, len(v))
		for i, vv := range v {
			out[i] = vv
		}
		return renderHCLList(out)
	default:
		// Unknown type — fall back to Go's default formatting rather than
		// silently emitting garbage. This surfaces in generated HCL where a
		// reviewer can spot it (better than a panic mid-generation).
		return fmt.Sprintf("%v", v)
	}
}

// renderHCLObject renders a map[string]any as an HCL object literal.
//
//	{ key1 = val1, key2 = val2 }
//
// Keys are sorted (deterministic output, D19) and quoted only when necessary
// (HCL identifiers stay bare; everything else is string-quoted the key).
func renderHCLObject(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("{ ")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(hclKey(k))
		b.WriteString(" = ")
		b.WriteString(renderHCLValue(m[k]))
	}
	b.WriteString(" }")
	return b.String()
}

// renderHCLList renders a slice as an HCL tuple/list literal.
//
//	["a", "b", 3]
func renderHCLList(items []any) string {
	if len(items) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteString("[")
	for i, it := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(renderHCLValue(it))
	}
	b.WriteString("]")
	return b.String()
}

// renderFloat renders a float64 as HCL number syntax: integers without a
// decimal point (e.g. 200 instead of 200.0), fractions preserved.
func renderFloat(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// hclKey renders an HCL attribute key: bare for identifiers, quoted otherwise.
func hclKey(k string) string {
	if isHCLIdentifier(k) {
		return k
	}
	return hclString(k)
}

// isHCLIdentifier reports whether s is a bare HCL identifier (so the key can
// be written without quotes). Conservative: ASCII letters/digits/underscore,
// must start with a letter or underscore.
func isHCLIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				return false
			}
			continue
		}
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// hclString renders a Go string as a double-quoted HCL string literal with
// minimal, safe escaping (backslash, double-quote, newline, tab). HCL uses the
// same escape grammar as Go/JSON for these characters, so the output round-
// trips through terraform fmt cleanly.
func hclString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
