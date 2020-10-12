# Manifest

The AWS Copilot CLI manifests describe a service’s architecture as infrastructure-as-code. 

It is a file generated from `copilot init` or `copilot svc init` that gets converted to a AWS CloudFormation template. Unlike raw CloudFormation templates, the manifest allows you to focus on the most common settings for the _architecture_ of your service and not the individual resources.

Manifest files are stored under `copilot/<your service name>/manifest.yml`.