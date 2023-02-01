# Manifest

AWS Copilot CLI の Manifest とは、Service、Job または Environment のアーキテクチャを "Infrastructure as Code" として記述したものです。

AWS Copilot CLI の Manifest は、`copilot init`、`copilot svc init`、`copilot job init` あるいは `copilot env init` から生成されるファイルで、最終的には AWS CloudFormation テンプレートに変換されます。
素の CloudFormation テンプレートが個々のリソースにフォーカスすることに比べ、Manifest では Service 、Job や Environment の**アーキテクチャ**設定にフォーカスできます。

Manifest ファイルは、`copilot/<your service, job, or environment name>/manifest.yml`に格納されます。
