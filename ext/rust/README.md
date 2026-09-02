# Phi Rust extension SDK (`ext/rust`)

Rust is a **first-class language for Phi extensions** — on par with the Go SDK
in [`ext/go`](../go). Same PXB wire protocol on stdin/stdout, same host
features (LLM tools, slash commands, intercepts, event subscriptions, confirm
dialogs), byte-for-byte interop, and the same install flow (`phi.yaml` + a
binary under `~/.phi/extensions/<name>/`). This crate is the zero-dependency
Rust authoring side: no JSON, no reflection, no runtime deps.

Wire compatibility with the Go SDK is pinned byte-for-byte by golden tests
against `ext/go/pxb/testdata/*.bin` (`tests/pxb_test.rs`).

## Layout

| Path | Role |
|------|------|
| `src/pxb/` | Wire protocol: frames (`codec`), tagged fields (`fields`), message codecs (`msg`), types/events (`types`) |
| `src/phi/` | Author SDK: `Extension`, `Tool`, `Command`, `Context` |
| `examples/` | Runnable extensions: `hello` (commands, intercepts, subscribe), `full` (tool, confirm, submit) |
| `tests/` | Golden byte-compat + end-to-end fake-host tests |

## Authoring

Add the crate from the repo. Until the first release tag that contains
`ext/rust` exists, track `main`; once a release is cut, pin the
corresponding `ext/vX.Y.Z` tag instead:

```toml
[dependencies]
# pre-release: tracks the latest main
phi-ext = { git = "https://github.com/pulseaiclub/phi", branch = "main" }
# release: pin the ext tag (first one containing ext/rust)
# phi-ext = { git = "https://github.com/pulseaiclub/phi", tag = "ext/v0.19.2" }
```

```rust,no_run
use phi_ext::{phi, pxb};

fn main() -> Result<(), phi::Error> {
    let mut m = phi::Extension::new("hello", "0.1.0");
    m.register_command("hello", phi::Command::new("Say hi", |_args, ctx| {
        ctx.notify("info", "Hello!");
        // ctx.submit("follow-up");      // after /hello returns
        // ctx.send_user_message("…");   // enqueue a turn anytime
        Ok(())
    }));
    m.on_user_input(|_ev| None);   // return Some(UserInputResult { handled: true, .. }) to swallow
    m.on_tool_call(|_ev| None);    // return Some(ToolCallResult { block: true, reason: "…", .. }) to deny
    m.on_tool_result(|_ev| None);  // return Some(ToolResultResult { stop: true, .. }) to end the loop
    m.on_turn_stopping(|_ev| None); // return Some(TurnStoppingResult { continue_: true, message: "…", .. }) to steer
    m.subscribe(pxb::Event::SessionStart, |_ev| {});
    m.run()
}
```

Command handlers get a [`phi::Context`](src/phi.rs) for host interaction:
`notify`, `set_status`, `submit`, `send_user_message`, `confirm`,
`confirm_opts`.

Build and install (a `phi.yaml` manifest must live next to the binary):

```bash
cargo build --release --example hello
mkdir -p ~/.phi/extensions/hello
cp target/release/examples/hello phi.yaml ~/.phi/extensions/hello/
```

Reload in the TUI: **Ctrl+K → extensions → reload**.

## Performance

Codec throughput on Apple Silicon (release build, single-threaded):

| Implementation | Hello payload encode+decode | Frame write+read (in-memory) | Allocs |
|---|---|---|---|
| Rust PXB (`phi-ext`) | ~0.12 µs | ~0.06 µs | — |
| Go PXB (`ext/go/pxb`) | ~0.11 µs | ~0.05 µs | 3 / op |
| Go JSON lines (marshal+unmarshal) | ~1.2 µs | — | 15 / op |

The ~10× gap over JSON lines comes from the protocol itself — fixed binary
header + tagged fields, no JSON parsing or reflection (see
`doc/extensions.md`, "Why not JSON lines?"). The language choice is within
noise: Rust and Go are at parity on identical codec work.

Codec CPU is not the bottleneck anyway: an extension's latency is dominated
by process spawn (~ms) and pipe round trips (~µs), identical for both SDKs.
Re-run the probe with `cargo run --release --example bench`.

## Development

```bash
cargo test            # unit + golden + end-to-end fake-host tests
cargo fmt --check
cargo clippy --all-targets -- -D warnings
```

The run loop is single-threaded; the borrow checker replaces the Go SDK's
mutexes (handlers get `&mut` state and cannot alias the registry).
