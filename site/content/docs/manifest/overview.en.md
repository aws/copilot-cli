# Manifest

The AWS Copilot CLI manifest describes a service's, a job's, a pipeline's or an environment's architecture as infrastructure-as-code.

It is a file generated from `copilot init`, `copilot svc init`, `copilot job init`, `copilot pipeline init`, or `copilot env init` that gets converted to an AWS CloudFormation template.
Unlike raw CloudFormation templates, the manifest allows you to focus on the most common settings for the _architecture_ of your service, job, pipeline or environment, and not the individual resources.

Manifest files are stored under `copilot/<your service, job, or environment name>/manifest.yml` or `copilot/pipelines/<your pipeline name>/manifest.yml`.
