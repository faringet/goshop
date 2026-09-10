# GoShop

A training project I use as a sandbox for learning Go backend development and distributed systems. It models a small e-commerce backend split into several services, with PostgreSQL, Redis, gRPC and Kafka.

The order flow is built as an event-driven Saga using choreography, with Transactional Outbox and Inbox patterns for reliable event processing and Redis-backed idempotency at the gateway. The project can run with Docker Compose or in a local Kubernetes cluster with kind, with Prometheus and Grafana for observability. There is also a small local Ollama-based ops assistant that can reconstruct an order timeline and explain what happened to it.

This is not a production store or a finished product. It is deliberately a learning playground for experimenting with backend architecture, distributed-system patterns, infrastructure and failure handling in a system large enough to make those problems visible.
