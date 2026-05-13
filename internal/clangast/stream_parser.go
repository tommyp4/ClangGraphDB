package clangast

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// streamParse reads the JSON AST from clang's -ast-dump=json output in a
// memory-efficient way. Instead of loading the entire 100s-of-MB JSON tree,
// it navigates to the "inner" array of the TranslationUnitDecl and decodes
// each top-level child individually, processing and discarding before moving
// to the next.
func (fc *fileContext) streamParse(r io.Reader) error {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()

	if err := fc.skipToInner(decoder); err != nil {
		return fmt.Errorf("skip to inner: %w", err)
	}

	// Clang's JSON AST uses inherited locations: when a node's loc.file is
	// empty, it inherits the file from the previous declaration. We track
	// the "current file" so we can properly filter system header declarations.
	lastFile := fc.filePath

	for decoder.More() {
		var child ASTNode
		if err := decoder.Decode(&child); err != nil {
			continue
		}

		if child.IsImplicit {
			continue
		}

		// Update inherited file tracking
		if child.Loc.File != "" {
			lastFile = child.Loc.File
		} else {
			child.Loc.File = lastFile
		}

		if !fc.isPathInRepo(child.Loc.File) {
			continue
		}

		fc.registerDecls(&child, "")
		fc.extractNodes(&child, "")
	}

	return nil
}

// skipToInner navigates the JSON token stream to position the decoder
// just inside the "inner" array of the root TranslationUnitDecl object.
func (fc *fileContext) skipToInner(decoder *json.Decoder) error {
	// Read opening { of root object
	tok, err := decoder.Token()
	if err != nil {
		return err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("expected {, got %v", tok)
	}

	// Scan keys until we find "inner"
	depth := 0
	for decoder.More() {
		tok, err := decoder.Token()
		if err != nil {
			return err
		}

		// Handle delimiters for depth tracking
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
			continue
		}

		if depth > 0 {
			continue
		}

		// At depth 0, tokens alternate: key, value, key, value...
		key, ok := tok.(string)
		if !ok {
			continue
		}

		if key == "inner" {
			// Next token should be [ opening the inner array
			tok, err := decoder.Token()
			if err != nil {
				return err
			}
			if delim, ok := tok.(json.Delim); !ok || delim != '[' {
				return fmt.Errorf("expected [ after inner, got %v", tok)
			}
			return nil
		}

		// Skip the value for this key
		// The value could be a primitive or a nested object/array
		if err := skipValue(decoder); err != nil {
			return err
		}
	}

	return fmt.Errorf("inner array not found")
}

// skipValue skips a single JSON value (primitive, object, or array) from the decoder.
func skipValue(decoder *json.Decoder) error {
	tok, err := decoder.Token()
	if err != nil {
		return err
	}

	delim, ok := tok.(json.Delim)
	if !ok {
		// Primitive value — already consumed
		return nil
	}

	// It's an object or array — need to skip until matching close
	var closeDelim json.Delim
	if delim == '{' {
		closeDelim = '}'
	} else if delim == '[' {
		closeDelim = ']'
	} else {
		return nil // closing delimiter, shouldn't happen
	}

	depth := 1
	for depth > 0 {
		tok, err := decoder.Token()
		if err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); ok {
			switch d {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 && d == closeDelim {
					return nil
				}
			}
		}
	}
	return nil
}

// extractTypeName extracts the base type name from a qualified type string,
// stripping pointers, references, const/volatile, and common prefixes.
func extractTypeName(qualType string) string {
	t := qualType
	t = strings.TrimSuffix(t, " *")
	t = strings.TrimSuffix(t, " &")
	t = strings.TrimSuffix(t, " &&")
	t = strings.TrimPrefix(t, "const ")
	t = strings.TrimPrefix(t, "volatile ")
	t = strings.TrimPrefix(t, "class ")
	t = strings.TrimPrefix(t, "struct ")

	builtins := map[string]bool{
		"int": true, "char": true, "float": true, "double": true,
		"bool": true, "void": true, "unsigned": true, "signed": true,
		"long": true, "short": true, "size_t": true, "wchar_t": true,
		"BOOL": true, "DWORD": true, "UINT": true, "LONG": true,
		"WORD": true, "BYTE": true, "LPCTSTR": true, "LPCSTR": true,
		"CString": true, "HRESULT": true,
	}
	if builtins[t] {
		return ""
	}

	if strings.Contains(t, "<") {
		return ""
	}

	return t
}
