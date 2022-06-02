---
title: 'AWS Copilot v1.18: 証明書のインポート、Pipeline でのデプロイの順序など'
twitter_title: 'AWS Copilot v1.18'
image: 'https://user-images.githubusercontent.com/879348/166855730-541cb7d5-82e0-4255-afcc-646058cfc626.png'
image_alt: 'Copilot の Pipeline でのデプロイ順序を制御する'
image_width: '1337'
image_height: '316'
---

# AWS Copilot v1.18: 証明書のインポート、Pipeline でのデプロイの順序付け、他

The AWS Copilot コアチームは、Copilot v1.18 リリースを発表します。このリリースに貢献した [@corey-cole](https://github.com/corey-cole) に感謝を申し上げます。私たちのパブリックな[コミュニティチャット](https://gitter.im/aws/copilot-cli)は常に成長しており、オンラインでは約 280 人、GitHub では 2.2k のスターを獲得しています。AWS Copilot へご支援、ご支持いただいている皆様お一人お一人に感謝をいたします。

Copilot v1.18 では、いくつかの新機能提供と改善が行われました:

* **証明書のインポート:** `copilot env init --import-cert-arns` を実行すると、検証済みの ACM 証明書を環境のロードバランサーリスナーにインポートできるようになりました。[詳細は
こちら](#%E8%A8%BC%E6%98%8E%E6%9B%B8%E3%81%AE%E3%82%A4%E3%83%B3%E3%83%9D%E3%83%BC%E3%83%88)をご覧ください。
* **Pipeline でのデプロイの順序付け:** 継続的デリバリーの Pipeline において、Service や Job がデプロイされる順番を制御できるようになりました。[詳細は
こちら](.#controlling-order-of-deployments-in-a-pipeline)をご覧ください。
* **Additional pipeline improvements:** Besides deployment orders, you can now limit which services or jobs to deploy in your pipeline or deploy custom cloudformation stacks in a pipeline. [See detailed section](./#additional-pipeline-improvements).
* **"recreate" strategy for faster redeployments:** You can now specify "recreate" deployment strategy so that ECS will stop old tasks in your service before starting new ones. [See detailed section](./#recreate-strategy-for-faster-redeployments).
* **Tracing for Load Balanced Web, Worker, and Backend Service:** To collect and ship traces to AWS X-Ray from ECS tasks, we are introducing `observability.tracing` configuration in the manifest to add an [AWS Distro for OpenTelemetry Collector](https://github.com/aws-observability/aws-otel-collector) sidecar container. [See detailed section](./#tracing-for-load-balanced-web-service-worker-service-and-backend-service).

## AWS Copilot とは?

AWS Copilot CLI は AWS 上でプロダクションレディなコンテナ化されたアプリケーションのビルド、リリース、そして運用のためのツールです。
開発のスタートからステージング環境へのプッシュ、本番環境へのリリースまで、Copilot はアプリケーション開発ライフサイクル全体の管理を容易にします。
Copilot の基礎となるのは、 AWS CloudFormation です。CloudFormation により、インプラストラクチャを 1 回の操作でコードとしてプロビジョニングできます。
Copilot は、 さまざまなタイプのマイクロサービスの作成と運用の為に、事前定義された CloudFormation テンプレートと、ユーザーフレンドリーなワークフローを提供します。
デプロイメントスクリプトを記述する代わりに、アプリケーションの開発に集中できます。

より詳細な AWS Copilot の紹介については、[Overview](../docs/concepts/overview.ja.md) を確認してください。

## 証明書のインポート
_Contributed by [Penghao He](https://github.com/iamhopaul123/)_

If you have domains managed outside of Route 53, or want to enable HTTPS without having a domain associated with your application, you can now use the new `--import-cert-arns` flag to import any validated certificates when creating your environments.

```
$ copilot env init --import-cert-arns arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012 --import-cert-arns arn:aws:acm:us-east-1:123456789012:certificate/87654321-4321-4321-4321-210987654321
```

For example, one of the certificates has `example.com` as its domain and `*.example.com` as a subject alternative name (SAN):

???+ example "Sample certificate"
    ```json
    {
      "Certificate": {
        "CertificateArn": "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012",
        "DomainName": "example.com",
        "SubjectAlternativeNames": [
          "*.example.com"
        ],
        "DomainValidationOptions": [
          {
            "DomainName": "example.com",
            "ValidationDomain": "example.com",
            "ValidationStatus": "SUCCESS",
            "ResourceRecord": {
              "Name": "_45c8aa9ac85568e905a6c3852e62ebc6.example.com.",
              "Type": "CNAME",
              "Value": "_f8be688050b7d23184863690b3d4baa8.xrchbtpdjs.acm-validations.aws."
            },
            "ValidationMethod": "DNS"
          }
        ],
        ...
    }
    ```
Then, you need to specify aliases that are valid against any of the imported certificates in a [Load Balanced Web Service manifest](../docs/manifest/lb-web-service.en.md):
```yaml
name: frontend
type: Load Balanced Web Service
http:
  path: 'frontend'
  alias: v1.example.com
```
!!!attention
    Specifying `http.alias` in service manifests is required for deploying services to an environment with imported certificates.
After the deployment, add the DNS of the Application Load Balancer (ALB) created in the environment as an A record to where your alias domain is hosted. For example, if your alias domain is hosted in Route 53:
???+ example "Sample Route 53 A Record"
    ```json
    {
      "Name": "v1.example.com.",
      "Type": "A",
      "AliasTarget": {
        "HostedZoneId": "Z1H1FL3HABSF5",
        "DNSName": "demo-publi-1d328e3bqag4r-1914228528.us-west-2.elb.amazonaws.com.",
        "EvaluateTargetHealth": true
      }
    }
    ```
Now, your service has HTTPS enabled using your own certificates and can be accessed via `https://v1.example.com`!

## Controlling Order of Deployments in a Pipeline
_Contributed by [Efe Karakus](https://github.com/efekarakus/)_

Copilot provides the `copilot pipeline` commands to create continuous delivery pipelines to automatically release microservices in your git repository.  
Prior to v1.18, all services and jobs defined in your git repository got deployed in parallel for each stage.
For example, given a monorepo with three microservices: `frontend`, `orders`, `warehouse`. All of them got deployed at the same time
to the `test` and `prod` environments:
=== "Pipeline"
    ![Rendered pipeline](../assets/images/pipeline-default.png)  
=== "Pipeline Manifest"
    ```yaml
    name: release
    source:
      provider: GitHub
      properties:
        branch: main
        repository: https://github.com/user/repo
    stages:
    - name: test
    - name: prod
      requires_approval: true
    ```
=== "Repository Layout"
    ```
    copilot
    ├── frontend
    │   └── manifest.yml
    ├── orders
    │   └── manifest.yml
    └── warehouse
        └── manifest.yml
    ```
Starting with v1.18, you can control the order of your deployments in your pipeline with the new [`deployments` field](../docs/manifest/pipeline.en.md#stages-deployments).  
```yaml
stages:
  - name: test
    deployments:
      orders:
      warehouse:
      frontend:
        depends_on: [orders, warehouse]
  - name: prod
    require_approval: true
    deployments:
      orders:
      warehouse:
      frontend:
        depends_on: [orders, warehouse]
```
With the manifest above, we're declaring that the `orders` and `warehouse` services should be deployed prior to the `frontend` so that clients can't send new API requests
before the downstream services are ready to accept them. Copilot figures out in which order the stacks should be deployed, and the resulting CodePipeline looks as follows:
![Rendered ordered pipeline](../assets/images/pipeline-ordered.png)
### Additional pipeline improvements
There are a few other enhancements that come with the new `deployments` field:
1. It is now possible for monorepos to configure which services or jobs to deploy in a pipeline. For example, I can limit 
   the pipeline to only deploy the `orders` microservice:
   ```yaml
   deployments:
     orders:
   ```
2. Your pipelines can now deploy standalone CloudFormation templates that are not generated by Copilot. For example, if we have a repository structured as follows:
   ```
   copilot
    ├── api
    │   └── manifest.yml
    └── templates
        ├── cognito.params.json
        └── cognito.yml
   ```
   Then I can leverage the new [`stack_name`](../docs/manifest/pipeline.en.md#stages-deployments-stackname), [`template_path`](../docs/manifest/pipeline.en.md#stages-deployments-templatepath) and [`template_config`](../docs/manifest/pipeline.en.md#stages-deployments-templateconfig) fields under deployments to specify deploying the cognito cloudformation stack in my pipeline:
   ```yaml
   deployments:
     cognito:
       stack_name: myapp-test-cognito
       template_path: infrastructure/cognito.yml
       template_config: infrastructure/cognito.params.json
     api:
   ```
   The final step would be modifying the copilot generated buildspec to copy the files under `copilot/templates`
   to `infrastructure/` with `cp -r copilot/templates infrastructure/` so that the `template_path` and `template_config`
   fields point to existing files.

## "recreate" Strategy for Faster Redeployments
_Contributed by [Parag Bhingre](https://github.com/paragbhingre/)_

!!!alert
    Due to the possible service downtime caused by "recreate", we do **not** recommend using it for your production services.

Before v1.18, a Copilot ECS-based service (Load Balanced Web Service, Backend Service, and Worker Service) redeployment always spun up new tasks, waited for them to be stable, and then stopped the old tasks. In order to support faster redeployments for ECS-based services in the development stage, users can specify `"recreate"` as the deployment strategy in the service manifest:

```yaml
deployment:
  rolling: recreate
```

Under the hood, Copilot sets [minimumHealthyPercent and maximumPercent](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DeploymentConfiguration.html) to `0` and `100` respectively (defaults are `100` and `200`), so that old tasks are stopped before spinning up any new tasks.

## Tracing for Load Balanced Web Service, Worker Service, and Backend Service
_Contributed by [Danny Randall](https://github.com/dannyrandall/)_

In [v1.17](./release-v117.en.md#send-your-request-driven-web-services-traces-to-aws-x-ray), Copilot launched support for sending traces from your Request-Driven Web Services to [AWS X-Ray](https://aws.amazon.com/xray/).
Now, you can easily export traces from your Load Balanced Web, Worker, and Backend services to X-Ray by configuring `observability` in your service's manifest:
```yaml
observability:
  tracing: awsxray
```

For these service types, Copilot will deploy an [AWS Distro for OpenTelemetry Collector](https://github.com/aws-observability/aws-otel-collector) sidecar container to collect traces from your service and export them to X-Ray.
After [instrumenting your service](../docs/developing/observability.en.md#instrumenting-your-service) to send traces, you can view the end-to-end journey of requests through your services to aid in debugging and monitoring performance of your application.

![X-Ray Service Map Example](https://user-images.githubusercontent.com/10566468/166986340-e3b7c0e2-c84d-4671-bf37-ba95bdb1d6b2.png)

Read our documentation on [Observability](../docs/developing/observability.en.md) to learn more about tracing and get started!

## What’s next?

Download the new Copilot CLI version by following the link below and leave your feedback on [GitHub](https://github.com/aws/copilot-cli/) or our [Community Chat](https://gitter.im/aws/copilot-cli):

* Download [the latest CLI version](../docs/getting-started/install.en.md)
* Try our [Getting Started Guide](../docs/getting-started/first-app-tutorial.en.md)
* Read full release notes on [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.18.0)

## 次は？

以下のリンクより、新しい Copilot CLI バージョンをダウンロードし、[GitHub](https://github.com/aws/copilot-cli/) や [コミュニティチャット](https://gitter.im/aws/copilot-cli)に
フィードバックを残してください。

* [最新 CLI バージョン](../docs/getting-started/install.ja.md)のダウンロード
* [スタートガイド](../docs/getting-started/first-app-tutorial.ja.md)を試す
* [GitHub](https://github.com/aws/copilot-cli/releases/tag/v1.18.0) でリリースノートの全文を読む
