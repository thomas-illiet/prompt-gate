# Prompt Gate Documentation

This directory contains operator and developer documentation for Prompt Gate
Backend. The docs describe the current implementation and the deployment model
used by the repository.

## Core Guides

| Guide | Use it for |
| --- | --- |
| [Architecture](architecture.md) | Understanding the API, proxy, Model Lab, scheduler, data stores, and hot-reload flow. |
| [API reference](api.md) | Finding route groups, Model Lab tickets, authentication requirements, admin capabilities, and the interactive OpenAPI explorer. |
| [Proxy runtime](proxy.md) | Configuring and operating the LLM proxy, API-token and Model Lab authentication, provider routing, OPA authorization, and usage recorder. |
| [Scheduler](scheduler.md) | Running cleanup, access expiration, monitoring, quota projection, and quota-reset jobs. |
| [Security model](security.md) | Reviewing OIDC, sessions, API tokens, Model Lab tickets, roles, OPA behavior, CORS, and secret storage. |
| [Development guide](development.md) | Running the stack locally, testing, migrations, and common Make targets. |
| [User FAQ](../faq.md) | Understanding roles, virtual keys, quotas, reset credits, usage, and common failures. |

## Operations

| Guide | Use it for |
| --- | --- |
| [Deployment](deployment.md) | Production deployment order, process layout, health checks, and Docker examples. |
| [YAML configuration](environment.md) | Complete structured runtime configuration reference by command. |
| [Release process](release.md) | Tagging, CI verification, image publishing, and rollback. |

## Existing Package Notes

Some domain packages also include focused implementation notes:

- [Auth package notes](../internal/domain/auth/README.md)

Those files are useful when changing package internals. The docs in this
directory are aimed at operators, integrators, and contributors who need the
system-level view.
