//! Rust author SDK for [Phi] PXB extensions — a faithful port of the Go SDK
//! under `ext/go`.
//!
//! Two layers, mirroring the Go module:
//!
//! - [`pxb`]: the binary wire protocol (frames, tagged fields, message codecs)
//! - [`phi`]: the author-facing API ([`phi::Extension`]) that speaks PXB over
//!   stdin/stdout
//!
//! The crate is dependency-free on purpose: the wire format needs no JSON and
//! no reflection, so hand-rolled codecs keep the SDK lean.
//!
//! # Example
//!
//! ```no_run
//! use phi_ext::{phi, pxb};
//!
//! fn main() -> Result<(), phi::Error> {
//!     let mut m = phi::Extension::new("hello", "0.1.0");
//!     m.register_command(
//!         "hello",
//!         phi::Command::new("Say hi", |_args, ctx| {
//!             ctx.notify("info", "Hello!");
//!             Ok(())
//!         }),
//!     );
//!     m.subscribe(pxb::Event::SessionStart, |_ev| {});
//!     m.run()
//! }
//! ```
//!
//! [Phi]: https://github.com/pulseaiclub/phi

pub mod phi;
pub mod pxb;
