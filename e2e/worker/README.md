# Worker Service E2E tests

The goal of the `worker` e2e test is to validate the most common usecases for a
[Worker Service](https://aws.github.io/copilot-cli/docs/concepts/services/#worker-service).

- Receiving and deleting messages from the default SQS queue: `COPILOT_QUEUE_URI`
- Making requests via service discovery to other services in the application.