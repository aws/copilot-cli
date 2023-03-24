# Overriding Copilot generated CloudFormation templates with YAML Patches

The file `cfn.patches.yml` contains a list of YAML/JSON patches to apply to
your template before AWS Copilot deploys it.

To view examples and an explanation of how YAML patches work, check out the [documentation](https://aws.github.io/copilot-cli/docs/developing/overrides/yamlpatch).

Note only [`add`](https://www.rfc-editor.org/rfc/rfc6902#section-4.1),
[`remove`](https://www.rfc-editor.org/rfc/rfc6902#section-4.2), and
[`replace`](https://www.rfc-editor.org/rfc/rfc6902#section-4.3)
operations are supported by Copilot.
Patches are applied in the order specified in the file.

## Troubleshooting

* `copilot [noun] package` preview the transformed template by writing to stdout.
* `copilot [noun] package --diff` show the difference against the template deployed in your environment.