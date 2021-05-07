Copilot な仲間達とアプリケーションや動画、ブログポストを共有しよう！

## ブログポスト

| タイトル     | 概要                         |
| ----------- | ------------------------------------ |
| [**Automatically deploying your container application with AWS Copilot**](https://aws.amazon.com/blogs/containers/automatically-deploying-your-container-application-with-aws-copilot/) by <a href="https://twitter.com/nathankpeck">@nathankpeck</a> | Nathan shows how to setup a release pipeline with the CLI that builds, pushes, and deploys an application. Finally, he sets up integration tests for validation before releasing to production. |
| [**Deploying containers with the AWS Copilot CLI**](https://maartenbruntink.nl/blog/2020/08/16/deploying-containers-with-the-aws-copilot-cli-part-1/) by <a href="https://twitter.com/maartenbruntink">@maartenbruntink</a> | Maarten shows to deploy the [sample Docker voting app](https://github.com/dockersamples/example-voting-app) with the AWS Copilot CLI that showcases how to setup your own Redis and Postgres servers. In the [second part](https://maartenbruntink.nl/blog/2020/08/16/deploying-containers-with-the-aws-copilot-cli-part-2), he automates the release process. |
| [**AWS Copilot: an application-first CLI for containers on AWS**](https://aws.amazon.com/blogs/containers/aws-copilot-an-application-first-cli-for-containers-on-aws/) by <a href="https://twitter.com/efekarakus">@efekarakus</a> | Efe walks through the design tenets of the CLI, why they were chosen, how they map to Copilot features, and the vision for how the CLI will evolve in the future.  |
| [**Introducing AWS Copilot**](https://aws.amazon.com/blogs/containers/introducing-aws-copilot/) by <a href="https://twitter.com/nathankpeck">@nathankpeck</a> | Nathan explains how with the AWS Copilot CLI you can go from idea to implementation much faster, with the confidence that the infrastructure you have deployed has production-ready configuration. |



## 動画

| タイトル     | 概要                         |
| ----------- | ------------------------------------ |
| [**Containers from the Couch series**](https://www.youtube.com/c/ContainersfromtheCouch/search?query=copilot) by <a href="https://twitter.com/realadamjkeller">@realadamjkeller</a>, <a href="https://twitter.com/brentContained">@brentContained</a>, and guests | Join Adam and Brent to learn about all the existing features of AWS Copilot with fun demos. From setting up a three-tier application with autoscaling to creating a continuous delivery pipeline with integration tests. |
| [**AWS re:Invent 2020: AWS Copilot: Simplifying container development**](https://youtu.be/EqW--TKQ_PQ) by <a href="https://twitter.com/efekarakus">@efekarakus</a> | Learn about the motivation behind AWS Copilot, get an overview of the existing commands and a demo of how to deploy a three-tier application. |
| [**How to Deploy a .NET Application to Amazon Elastic Container Service (ECS) with AWS Copilot**](https://youtu.be/nWaw8Rp8JgQ) by <a href="https://twitter.com/ignacioafuentes">@ignacioafuentes</a> | Get a demo on how to build and deploy a .NET application on Amazon ECS and AWS Fargate. |
| [**AWS What's Next**](https://www.youtube.com/watch?v=vmTJgVDERZU) by <a href="https://twitter.com/nathankpeck">@nathankpeck</a> and <a href="https://twitter.com/efekarakus">@efekarakus</a> | Nathan and Efe discusses what makes AWS Copilot unique compared to other infrastructure provisioning tools and then demo an overview of the existing commands. |



## コード・サンプル

| リポジトリ     | 詳細                         | 特徴 |
| ----------- | ------------------------------------ | ------------ |
[**github.com/copilot-example-voting-app**](https://github.com/copilot-example-voting-app) | A voting application distributed over three ECS services created with AWS Copilot. | Amazon Aurora PostgreSQL database, service discovery, autoscaling |
[**#1925**](https://github.com/aws/copilot-cli/discussions/1925) | Show and tell explaining how you can do continuous deployments from branches with AWS Copilot pipelines. | Branch-based deploys, AWS CodePipeline |


## ワークショップ

| タイトル     | 詳細                         |
| ----------- | ------------------------------------ |
[**ECS Workshop**](https://ecsworkshop.com/microservices/) | In this workshop, we deploy a three tier microservices application using the copilot-cli |
