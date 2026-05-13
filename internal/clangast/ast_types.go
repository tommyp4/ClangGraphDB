package clangast

// ASTNode represents a node in Clang's JSON AST dump.
// Only the fields we need are mapped; unknown fields are ignored by the JSON decoder.
type ASTNode struct {
	ID                  string    `json:"id"`
	Kind                string    `json:"kind"`
	Name                string    `json:"name,omitempty"`
	MangledName         string    `json:"mangledName,omitempty"`
	IsImplicit          bool      `json:"isImplicit,omitempty"`
	StorageClass        string    `json:"storageClass,omitempty"`
	CompleteDefinition  bool      `json:"completeDefinition,omitempty"`
	TagUsed             string    `json:"tagUsed,omitempty"`

	Loc   SourceLocation `json:"loc"`
	Range SourceRange    `json:"range"`

	Type *TypeInfo `json:"type,omitempty"`

	// Function-specific
	Inner []ASTNode `json:"inner,omitempty"`

	// CXXRecordDecl bases
	Bases []BaseSpec `json:"bases,omitempty"`

	// DeclRefExpr
	ReferencedDecl *RefDecl `json:"referencedDecl,omitempty"`

	// MemberExpr
	ReferencedMemberDecl string `json:"referencedMemberDecl,omitempty"`
}

type SourceLocation struct {
	Offset int    `json:"offset,omitempty"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Col    int    `json:"col,omitempty"`
}

type SourceRange struct {
	Begin SourceLocation `json:"begin"`
	End   SourceLocation `json:"end"`
}

type TypeInfo struct {
	QualType         string `json:"qualType"`
	DesugaredQualType string `json:"desugaredQualType,omitempty"`
}

type BaseSpec struct {
	Access   string   `json:"access,omitempty"`
	Type     TypeInfo `json:"type"`
	WrittenAccess string `json:"writtenAccess,omitempty"`
}

type RefDecl struct {
	ID   string    `json:"id"`
	Kind string    `json:"kind"`
	Name string    `json:"name,omitempty"`
	Type *TypeInfo `json:"type,omitempty"`
}
