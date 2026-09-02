//! The Phi eXtension Binary wire protocol — a Rust port of `ext/go/pxb`.
//!
//! # Frame
//!
//! Every message is one length-prefixed binary frame on a duplex byte stream
//! (typically a child process stdin/stdout). No JSON, no newlines.
//!
//! ```text
//! ┌──────── header (16 bytes, little-endian) ────────┐
//! │ magic[4]="PXB\x01" │ typ u16 │ flags u16 │ id u32│
//! │ payload_len u32                                  │
//! └──────────────────────────────────────────────────┘
//! │ payload: tagged fields                           │
//! ```
//!
//! # Evolution rules
//!
//! 1. New message fields: allocate a new tag (≥1). Never reuse a tag.
//! 2. New lifecycle events: allocate a new `Ev*` code. Never reuse a code.
//! 3. New frame types: allocate a `Type*` in the Ext→Host (1–99) or
//!    Host→Ext (100–199) range; peers skip unknown types by length.
//! 4. Incompatible renames / semantic breaks: bump
//!    [`PROTOCOL_VERSION`](crate::pxb::PROTOCOL_VERSION) and refuse old peers.
//! 5. Experimental tags use 128+ and must remain skippable.
//!
//! These rules keep host and extension binaries independently upgradable
//! without a shared JSON schema or lockstep release.

mod codec;
mod fields;
mod msg;
mod types;

pub use codec::{decode_header, encode_header, read_frame, write_frame, Error, Frame, Header};
pub use fields::{walk_fields, FieldReader, FieldWriter, WIRE_BYTES, WIRE_U64};
pub use msg::{
    decode_command_invoked, decode_command_response, decode_event_notify, decode_hello,
    decode_hello_ack, decode_host_request, decode_host_result, decode_intercept_req,
    decode_intercept_resp, decode_notify, decode_register_command, decode_register_tool,
    decode_session_meta, decode_subscribe, decode_tool_invoke, decode_tool_result,
    encode_command_invoked, encode_command_response, encode_event_notify, encode_hello,
    encode_hello_ack, encode_host_request, encode_host_result, encode_intercept_req,
    encode_intercept_resp, encode_notify, encode_register_command, encode_register_tool,
    encode_session_meta, encode_subscribe, encode_tool_invoke, encode_tool_result, CommandInvoked,
    CommandResponse, EventNotify, Hello, HelloAck, HostRequest, HostResult, InterceptReq,
    InterceptResp, NotifyMsg, RegisterCommand, RegisterTool, SessionMeta, Subscribe, ToolInvoke,
    ToolResultMsg,
};
pub use types::{
    Event, FrameType, CAP_COMMANDS, CAP_EVENTS, CAP_INTERCEPT, CAP_TOOLS, FLAG_HAS_ID, HEADER_SIZE,
    MAGIC, MAX_PAYLOAD, PROTOCOL_VERSION, TYPE_COMMAND_INVOKED, TYPE_COMMAND_RESPONSE, TYPE_EVENT,
    TYPE_HELLO, TYPE_HELLO_ACK, TYPE_HOST_REQUEST, TYPE_HOST_RESULT, TYPE_INTERCEPT,
    TYPE_INTERCEPT_RESPONSE, TYPE_NOTIFY, TYPE_READY, TYPE_REGISTER_COMMAND, TYPE_REGISTER_TOOL,
    TYPE_SESSION_META, TYPE_SHUTDOWN, TYPE_SHUTDOWN_ACK, TYPE_SUBSCRIBE, TYPE_TOOL_INVOKE,
    TYPE_TOOL_RESULT,
};
