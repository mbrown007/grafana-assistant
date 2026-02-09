# Testing

The Monitoring Assistant project has a comprehensive testing strategy that includes unit tests, integration tests, and end-to-end (E2E) tests.

## Backend Testing (Go)

The backend tests are written in Go and use the standard `testing` package.

### Unit Tests

Unit tests cover individual functions and packages in isolation.

To run all the Go unit tests, use the following command:

```bash
make test
```

This will run all files ending in `_test.go` in the project.

### Integration Tests

Integration tests cover the interaction between different components of the system. These tests are located in the `tests/integration/` directory and are tagged with `integration`.

To run the integration tests, you need to have a running Grafana instance. You can set the `TEST_GRAFANA_URL` environment variable to point to your Grafana instance.

```bash
make test-integration
```

## Frontend Testing (React)

The frontend tests are written with Vitest and the React Testing Library.

To run the frontend tests, use the following command:

```bash
npm --prefix frontend run test
```
or
```bash
make frontend-test
```
This will run all the tests in the `frontend/src/` directory.

To run the tests in watch mode, use:
```bash
npm --prefix frontend run test:watch
```

## End-to-End (E2E) Tests

E2E tests cover the entire application, from the frontend to the backend and the integrated services. These tests are located in the `tests/e2e/` directory and are tagged with `e2e`.

To run the E2E tests, you first need to start the E2E environment, which includes the application and all its dependencies (Grafana, Prometheus, etc.).

```bash
make e2e-up
```

Once the environment is up, you can run the E2E tests:

```bash
make test-e2e
```

After you are done, you can tear down the E2E environment:

```bash
make e2e-down
```

## Running All Tests

You can run both the Go unit tests and the frontend tests with a single command:

```bash
make test-all
```

## Linting

To run the linter on the Go code, use:

```bash
make lint
```
This uses `golangci-lint` to check for code quality and style issues.

## Eval Pipeline (Baseline + Judge + Guard + Gate)

The project also has a quality-eval workflow used for roadmap quality gates.

Run baseline:

```bash
make eval-baseline
```

Run baseline in mock replay mode (no Docker required):

```bash
make eval-baseline-mock
```

CI note:
- PR pipeline uses mock-based eval quality gate for fast deterministic checks.
- Live Docker-stack eval quality gate runs on scheduled/manual workflow runs.

Run judge scoring:

```bash
make eval-judge RUN_ARTIFACT=tests/evals/results/baseline-<timestamp>.json
```

Run deterministic guards:

```bash
make eval-guard RUN_ARTIFACT=tests/evals/results/baseline-<timestamp>.json
```

Run quality gate thresholds:

```bash
make eval-quality-gate \
  JUDGE_REPORT=tests/evals/results/judge-<timestamp>.json \
  GUARD_REPORT=tests/evals/results/guard-<timestamp>.json
```

For adding new eval cases safely, see:

- [Eval Case Authoring](Eval-Case-Authoring)

## Mock MCP Sandbox (Deterministic Evals)

Phase 8 adds a local mock MCP server that replays fixture responses over stdio so evals can run without a live Grafana stack.

Build the mock server:

```bash
make mock-mcp-build
```

Run it with fixture replay:

```bash
./bin/mock-mcp -fixtures tests/evals/fixtures
```

Fixture files live under `tests/evals/fixtures/` and include:
- `tool_name` (for example `grafana__query_prometheus`)
- `match` map of regex patterns against tool arguments
- `response` payload returned when matched

When no fixture matches a tool call, the server returns a non-error "no data" success payload.

Record fixtures from a live environment:

```bash
scripts/record_eval_fixtures.sh
```

The assistant must be started with fixture recording enabled:
- `eval_fixture_record_dir: tests/evals/fixtures` in `config.yaml`, or
- `ASSISTANT_EVAL_FIXTURE_RECORD_DIR=tests/evals/fixtures`
