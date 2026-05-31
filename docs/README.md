# Prompt Gate Documentation

This directory contains operator and developer documentation for Prompt Gate
Backend. The docs describe the current implementation and the deployment model
used by the repository.

## Core Guides

| Guide | Use it for |
| --- | --- |
| [Architecture](architecture.md) | Understanding the API, proxy, scheduler, data stores, and hot-reload flow. |
| [API reference](api.md) | Finding route groups, authentication requirements, and admin capabilities. |
| [Proxy runtime](proxy.md) | Configuring and operating the LLM proxy, provider routing, MCP, firewall, and usage recorder. |
| [Scheduler](scheduler.md) | Running background jobs for token cleanup and access expiration. |
| [Security model](security.md) | Reviewing OIDC, sessions, API tokens, roles, firewall behavior, CORS, and secret storage. |
| [Development guide](development.md) | Running the stack locally, testing, migrations, and common Make targets. |

## Operations

| Guide | Use it for |
| --- | --- |
| [Deployment](deployment.md) | Production deployment order, process layout, health checks, and Docker examples. |
| [Environment variables](environment.md) | Complete `PROMPTGATE_*` configuration reference by command. |
| [Release process](release.md) | Tagging, CI verification, image publishing, and rollback. |

## Existing Package Notes

Some domain packages also include focused implementation notes:

- [Auth package notes](../internal/domain/auth/README.md)
- [Firewall package notes](../internal/domain/firewall/README.md)

Those files are useful when changing package internals. The docs in this
directory are aimed at operators, integrators, and contributors who need the
system-level view.
