# Feature Implementation Plan: Multi-Language Dependency Injection Support

## 📋 Todo Checklist
- [x] **Phase 1: Java Support**
    - [x] Update `java.go` Tree-sitter query for Constructor Parameters.
    - [x] Refactor `java.go` Field Type extraction to be robust.
    - [x] Implement `DEPENDS_ON` edge creation for Java.
    - [x] Verify with `test/fixtures/java/`.
- [x] **Phase 2: TypeScript Support**
    - [x] Update `typescript.go` Tree-sitter query for Constructor Parameters.
    - [x] Update `typescript.go` to capture Field Types (Type Annotations).
    - [x] Implement `DEPENDS_ON` edge creation for TS.
    - [x] Verify with `test/fixtures/typescript/`.
- [x] **Phase 3: C++ Support**
    - [x] Update `cpp.go` to capture Member Variable Types.
    - [x] Update `cpp.go` to capture Constructor Parameters.
    - [x] Implement `DEPENDS_ON` edge creation for C++.
    - [x] Verify with `test/fixtures/cpp/`.
- [ ] **Phase 4: VB.NET Assessment**
    - [ ] Investigate potential for Regex improvements or move to Tree-sitter (Low Priority).

## 🔍 Analysis & Investigation

### Goal
Extend the "Dependency Injection" detection logic (currently implemented for C#) to Java, TypeScript, and C++. This involves analyzing class constructors and field definitions to identify dependencies and linking them in the graph.

### Current Gaps
| Language | Field Types | Constructor Params | Generics | `DEPENDS_ON` Edges |
| :--- | :--- | :--- | :--- | :--- |
| **Java** | Fragile (Linear Scan) | ❌ Missing | ⚠️ Stripped | ❌ Missing |
| **TypeScript** | ❌ Missing | ❌ Missing | ❌ Missing | ❌ Missing |
| **C++** | ❌ Missing | ❌ Missing | ❌ Missing | ❌ Missing |
| **VB.NET** | ❌ Missing | ❌ Missing | ❌ Missing | ❌ Missing |

### 1. Java (`internal/analysis/java.go`)
*   **Current:** Captures `field.name` and `field.type` but processes them in a potentially fragile linear loop.
*   **Missing:** 
    *   `constructor_declaration` parameters are ignored.
    *   Generics like `List<Service>` are stripped to `List` (ignoring `Service`).
    *   No `DEPENDS_ON` edges are created.

### 2. TypeScript (`internal/analysis/typescript.go`)
*   **Current:** Captures `public_field_definition` names but **ignores type annotations**.
*   **Missing:**
    *   Constructor parameters (critical for Angular/NestJS).
    *   Field type annotations.
    *   Generics.

### 3. C++ (`internal/analysis/cpp.go`)
*   **Current:** Captures field names but **ignores types**.
*   **Missing:**
    *   Field types.
    *   Constructor parameters.
    *   Template types.

### 4. VB.NET (`internal/analysis/vbnet.go`)
*   **Current:** Regex-based. Very limited.
*   **Strategy:** Defer deep DI support until a proper parser is available, or use "Best Effort" regex for `Private _x As Type`.

## 📝 Implementation Plan

### Common Strategy
For each language:
1.  **Enhance Tree-sitter Query:** Add captures for type identifiers in fields and constructors.
2.  **Refactor Parse Logic:** Iterate over these new captures.
3.  **Resolve Types:** Extract the "Base Type" and "Generic Arguments" (e.g., `Provider<Service>` -> depends on `Service`).
4.  **Create Edges:** `Class --[DEPENDS_ON]--> Type`.

### Phase 1: Java Implementation

#### 1.A Update Query
```scheme
(class_declaration
    name: (identifier) @class.name
    body: (class_body
        (field_declaration
            type: (_) @field.type
            declarator: (variable_declarator name: (identifier) @field.name)
        )
        (constructor_declaration
            parameters: (formal_parameters
                (formal_parameter 
                    type: (_) @param.type
                    name: (identifier) @param.name
                )
            )
        )
    )
)
```
*Note: This structure ensures context.*

#### 1.B Logic Update
*   **Generics:** If type is `parameterized_type`, extract the `type_arguments`.
    *   Example: `List<User>` -> Create edge to `User`.
*   **Constructors:** Iterate `formal_parameters`. Link `Class -> ParamType`.

### Phase 2: TypeScript Implementation

#### 2.A Update Query
```scheme
(class_declaration
    name: (type_identifier) @class.name
    body: (class_body
        (public_field_definition
            name: (property_identifier) @field.name
            type: (type_annotation (type_identifier) @field.type)?
        )
        (method_definition
            name: (property_identifier) @method.name
            parameters: (formal_parameters
                (required_parameter
                    pattern: (identifier) @param.name
                    type: (type_annotation (type_identifier) @param.type)?
                )
                (public_field_definition 
                    name: (property_identifier) @ctor.field.name
                    type: (type_annotation (type_identifier) @ctor.field.type)?
                ) @ctor.prop
            )
        )
    )
)
```
*Note: TS Constructors can define fields inline (e.g. `constructor(private service: Service)`). The query must capture these `parameter_property` nodes.*

#### 2.B Logic Update
*   Identify `constructor` specifically (method name `constructor`).
*   Handle `parameter_property` (e.g., `private service: Service`) which defines a field AND a dependency.

### Phase 3: C++ Implementation

#### 3.A Update Query
```scheme
(field_declaration
    type: (_) @field.type
    declarator: (field_identifier) @field.name
)
(function_definition
    declarator: (function_declarator
        declarator: (identifier) @func.name
        parameters: (parameter_list
            (parameter_declaration
                type: (_) @param.type
            )
        )
    )
)
```

#### 3.B Logic Update
*   Map `field.type` to `DEPENDS_ON`.
*   Identify Constructors (function name == class name).
*   Handle `template_type`.

### Testing Strategy
1.  **Fixtures:** Use existing `test/fixtures/` files.
2.  **Verification Command:**
    *   Run `go run cmd/graphdb/main.go --analyze test/fixtures/[lang]/`
    *   Check output graph for `DEPENDS_ON` edges.

## 🎯 Success Criteria
1.  **Java:** `DEPENDS_ON` edges exist for fields and constructor params.
2.  **TypeScript:** `DEPENDS_ON` edges exist for constructor injections (Angular style).
3.  **C++:** `DEPENDS_ON` edges exist for member types.
