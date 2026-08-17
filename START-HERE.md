# Start here

You are implementing **LabMail**, a MailDev parity rewrite in Go. Default deploy is a receive-only SMTP sink. Code may not exist yet.

1. Read [AGENTS.md](AGENTS.md) completely. Those rules are not optional.
2. Read [docs/README.md](docs/README.md) and [docs/00-evaluation.md](docs/00-evaluation.md).
3. Pick a task from [docs/waves/00-program-board.md](docs/waves/00-program-board.md) whose dependencies are done and whose exclusive files are free.
4. Follow [docs/waves/agent-task-template.md](docs/waves/agent-task-template.md).
5. After you push, watch CI. If a chain of PRs is merging, watch the last one, then `main`. Fix and harden failures ([AGENTS.md](AGENTS.md) §2.6).

Wave CMP ([docs/waves/wave-cmp-comparison-lab.md](docs/waves/wave-cmp-comparison-lab.md)) can start as soon as Docker is available: characterize original MailDev before LabMail exists. Spec: [docs/22-comparison-lab.md](docs/22-comparison-lab.md).

Do not enable outgoing in mcp-integration-lab examples. Do not implement MCP by calling REST. Do not skip tests or docs. Comparison-lab relay may target only `relay-sink`.
