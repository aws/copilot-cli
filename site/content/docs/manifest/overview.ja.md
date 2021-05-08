# Manifest

AWS Copilot CLI の Manifest とは、Service や Job のアーキテクチャを "Infrastructure as Code" として記述したものです。

AWS Copilot CLI の Manifest は、`copilot init`、`copilot svc init` あるいは `copilot job init` から生成されるファイルで、最終的には AWS CloudFormation テンプレートに変換されます。
素の CloudFormation テンプレートが個々のリソースにフォーカスすることに比べ、Manifest では Service や Job の**アーキテクチャ**設定にフォーカスできます。

Manifest ファイルは、`copilot/<your service or job name>/manifest.yml`に格納されます。
