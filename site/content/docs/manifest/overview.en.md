# Manifest

The AWS Copilot CLI manifest describes a service’s or job's architecture as infrastructure-as-code.

It is a file generated from `copilot init`, `copilot svc init`, or `copilot job init` that gets converted to an AWS CloudFormation template.
Unlike raw CloudFormation templates, the manifest allows you to focus on the most common settings for the _architecture_ of your service or job, and not the individual resources.

Manifest files are stored under `copilot/<your service or job name>/manifest.yml`.
