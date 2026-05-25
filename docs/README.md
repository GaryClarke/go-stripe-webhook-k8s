# Documentation

## PROJECT_KNOWLEDGE.md

The main project knowledge document. It covers:

- **What we're building** - Goals, components, event types
- **Local vs Lambda** - Two entry points, two binaries, shared logic, config (SQS vs in-memory)
- **How Lambda is invoked** - Request path, API Gateway → Lambda wiring, what the handler receives, no "magic"
- **Phase 1 scope** - Local-only, no AWS or Stripe CLI required
- **Build order (Lambda era)** - Branch-by-branch plan ([§5](PROJECT_KNOWLEDGE.md#5-build-order-branches)); index: [original-branches/README.md](original-branches/README.md)
- **Build order (Kubernetes / HTTP)** - [PLAN.md](../PLAN.md); branch write-ups: [branches/README.md](branches/README.md)

Read this first when onboarding to the project or when you need to recall decisions and architecture.

---

As the project grows, this folder can also hold:

- `branches/` - K8s track: per-branch learning logs ([index](branches/README.md))
- `openshift/` - OpenShift Sandbox runbook ([index](openshift/README.md))
- `original-branches/` - Historical Lambda track ([index](original-branches/README.md))
- `decisions/` - ADRs or decision records
- `runbooks/` - How to run locally, deploy, test
