//! Typed JSON Schema for tool parameters.
//!
//! Codex generates these via `schemars` from Rust argument structs. This crate
//! stays dependency-free, so authors build an equivalent schema with the
//! builders below; the wire still carries opaque JSON Schema bytes (same as
//! Go's `Parameters map[string]any` after `json.Marshal`).

use std::collections::BTreeMap;

/// JSON Schema body for an LLM tool's parameters
/// (`type` / `properties` / `required` / …).
#[derive(Debug, Clone)]
pub struct Schema {
    inner: SchemaInner,
}

#[derive(Debug, Clone)]
enum SchemaInner {
    Built(Node),
    /// Escape hatch for hand-written JSON Schema bytes.
    Raw(Vec<u8>),
}

#[derive(Debug, Clone)]
struct Node {
    kind: Kind,
    description: Option<String>,
    properties: BTreeMap<String, Node>,
    required: Vec<String>,
    additional_properties: Option<bool>,
    enum_values: Vec<String>,
    items: Option<Box<Node>>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Kind {
    Object,
    String,
    Number,
    Integer,
    Boolean,
    Array,
}

impl Default for Node {
    fn default() -> Self {
        Self {
            kind: Kind::Object,
            description: None,
            properties: BTreeMap::new(),
            required: Vec::new(),
            additional_properties: None,
            enum_values: Vec::new(),
            items: None,
        }
    }
}

impl Schema {
    fn from_node(node: Node) -> Self {
        Self {
            inner: SchemaInner::Built(node),
        }
    }

    /// Object schema (`{"type":"object",…}`). Default for tool parameters.
    pub fn object() -> Self {
        Self::from_node(Node {
            kind: Kind::Object,
            ..Node::default()
        })
    }

    pub fn string() -> Self {
        Self::from_node(Node {
            kind: Kind::String,
            ..Node::default()
        })
    }

    pub fn number() -> Self {
        Self::from_node(Node {
            kind: Kind::Number,
            ..Node::default()
        })
    }

    pub fn integer() -> Self {
        Self::from_node(Node {
            kind: Kind::Integer,
            ..Node::default()
        })
    }

    pub fn boolean() -> Self {
        Self::from_node(Node {
            kind: Kind::Boolean,
            ..Node::default()
        })
    }

    /// Array schema. `items` must be a builder schema (not [`Schema::raw`]).
    pub fn array(items: Schema) -> Self {
        let SchemaInner::Built(items_node) = items.inner else {
            panic!("Schema::array requires a builder schema, not Schema::raw");
        };
        Self::from_node(Node {
            kind: Kind::Array,
            items: Some(Box::new(items_node)),
            ..Node::default()
        })
    }

    /// Opaque JSON Schema bytes (the previous `Vec<u8>` API).
    pub fn raw(json: impl Into<Vec<u8>>) -> Self {
        Self {
            inner: SchemaInner::Raw(json.into()),
        }
    }

    pub fn description(mut self, d: impl Into<String>) -> Self {
        if let SchemaInner::Built(n) = &mut self.inner {
            n.description = Some(d.into());
        }
        self
    }

    /// Add an object property. No-op on non-object / raw schemas.
    pub fn property(mut self, name: impl Into<String>, schema: Schema) -> Self {
        if let SchemaInner::Built(n) = &mut self.inner {
            if n.kind == Kind::Object {
                if let SchemaInner::Built(child) = schema.inner {
                    n.properties.insert(name.into(), child);
                }
            }
        }
        self
    }

    /// Mark object property names as required.
    pub fn required<I, S>(mut self, names: I) -> Self
    where
        I: IntoIterator<Item = S>,
        S: Into<String>,
    {
        if let SchemaInner::Built(n) = &mut self.inner {
            if n.kind == Kind::Object {
                n.required.extend(names.into_iter().map(Into::into));
            }
        }
        self
    }

    pub fn additional_properties(mut self, allow: bool) -> Self {
        if let SchemaInner::Built(n) = &mut self.inner {
            if n.kind == Kind::Object {
                n.additional_properties = Some(allow);
            }
        }
        self
    }

    /// Restrict a string schema to an enum (Codex-style compact enums).
    pub fn enum_values<I, S>(mut self, values: I) -> Self
    where
        I: IntoIterator<Item = S>,
        S: Into<String>,
    {
        if let SchemaInner::Built(n) = &mut self.inner {
            if n.kind == Kind::String {
                n.enum_values.extend(values.into_iter().map(Into::into));
            }
        }
        self
    }

    /// Serialize to JSON Schema bytes for `RegisterTool`.
    pub fn to_json_bytes(&self) -> Vec<u8> {
        match &self.inner {
            SchemaInner::Raw(b) => b.clone(),
            SchemaInner::Built(n) => {
                let mut s = String::new();
                write_node(&mut s, n);
                s.into_bytes()
            }
        }
    }
}

impl From<Vec<u8>> for Schema {
    fn from(json: Vec<u8>) -> Self {
        Schema::raw(json)
    }
}

impl From<&[u8]> for Schema {
    fn from(json: &[u8]) -> Self {
        Schema::raw(json.to_vec())
    }
}

fn write_node(out: &mut String, n: &Node) {
    out.push('{');
    let mut first = true;
    push_key(out, &mut first, "type");
    push_json_string(out, kind_str(n.kind));

    if let Some(d) = &n.description {
        push_key(out, &mut first, "description");
        push_json_string(out, d);
    }

    if n.kind == Kind::Object {
        push_key(out, &mut first, "properties");
        out.push('{');
        let mut pfirst = true;
        for (name, child) in &n.properties {
            if !pfirst {
                out.push(',');
            }
            pfirst = false;
            push_json_string(out, name);
            out.push(':');
            write_node(out, child);
        }
        out.push('}');

        if !n.required.is_empty() {
            push_key(out, &mut first, "required");
            out.push('[');
            for (i, name) in n.required.iter().enumerate() {
                if i > 0 {
                    out.push(',');
                }
                push_json_string(out, name);
            }
            out.push(']');
        }

        if let Some(allow) = n.additional_properties {
            push_key(out, &mut first, "additionalProperties");
            out.push_str(if allow { "true" } else { "false" });
        }
    }

    if n.kind == Kind::String && !n.enum_values.is_empty() {
        push_key(out, &mut first, "enum");
        out.push('[');
        for (i, v) in n.enum_values.iter().enumerate() {
            if i > 0 {
                out.push(',');
            }
            push_json_string(out, v);
        }
        out.push(']');
    }

    if n.kind == Kind::Array {
        if let Some(items) = &n.items {
            push_key(out, &mut first, "items");
            write_node(out, items);
        }
    }

    out.push('}');
}

fn kind_str(k: Kind) -> &'static str {
    match k {
        Kind::Object => "object",
        Kind::String => "string",
        Kind::Number => "number",
        Kind::Integer => "integer",
        Kind::Boolean => "boolean",
        Kind::Array => "array",
    }
}

fn push_key(out: &mut String, first: &mut bool, key: &str) {
    if !*first {
        out.push(',');
    }
    *first = false;
    push_json_string(out, key);
    out.push(':');
}

fn push_json_string(out: &mut String, s: &str) {
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            c if (c as u32) < 0x20 => out.push_str(&format!("\\u{:04x}", c as u32)),
            c => out.push(c),
        }
    }
    out.push('"');
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn object_with_required_string_prop() {
        let s = Schema::object()
            .property("text", Schema::string().description("input text"))
            .required(["text"]);
        assert_eq!(
            String::from_utf8(s.to_json_bytes()).unwrap(),
            r#"{"type":"object","properties":{"text":{"type":"string","description":"input text"}},"required":["text"]}"#
        );
    }

    #[test]
    fn string_enum_and_additional_properties() {
        let s = Schema::object()
            .property(
                "mode",
                Schema::string().enum_values(["read-only", "workspace-write"]),
            )
            .additional_properties(false);
        let json = String::from_utf8(s.to_json_bytes()).unwrap();
        assert!(json.contains(r#""enum":["read-only","workspace-write"]"#));
        assert!(json.contains(r#""additionalProperties":false"#));
    }

    #[test]
    fn array_of_strings() {
        let s = Schema::object().property("tags", Schema::array(Schema::string()));
        assert_eq!(
            String::from_utf8(s.to_json_bytes()).unwrap(),
            r#"{"type":"object","properties":{"tags":{"type":"array","items":{"type":"string"}}}}"#
        );
    }

    #[test]
    fn raw_passthrough() {
        let raw = br#"{"type":"object"}"#;
        assert_eq!(Schema::raw(raw.to_vec()).to_json_bytes(), raw);
    }
}
