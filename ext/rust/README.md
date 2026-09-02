# Phi Rust extension SDK (`ext/rust`)

Zero-dependency Rust port of the Go author SDK in [`ext/go`](../go). Write
extensions that speak the PXB binary protocol on stdin/stdout — no JSON, no
reflection, no runtime deps.

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

Add the crate from the repo (tagged with the `ext/vX.Y.Z` release tags):

```toml
[dependencies]
phi-ext = { git = "https://github.com/pulseaiclub/phi", tag = "ext/v0.19.1" }
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

## Development

```bash
cargo test            # unit + golden + end-to-end fake-host tests
cargo fmt --check
cargo clippy --all-targets -- -D warnings
```

The run loop is single-threaded; the borrow checker replaces the Go SDK's
mutexes (handlers get `&mut` state and cannot alias the registry).
