# hello — PXB extension sample

Build and install:

```bash
cd examples/extensions/hello
go build -o hello .
mkdir -p ~/.phi/extensions/hello
cp hello phi.yaml ~/.phi/extensions/hello/
```

Reload in TUI: **Ctrl+K → extensions → reload**.

- `/hello` — toast from the extension
- LLM tool `ext_hello` — greet by name
