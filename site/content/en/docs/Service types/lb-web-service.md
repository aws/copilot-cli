---
title: "Load Balanced Web Service"
linkTitle: "Load Balanced Web Service"
weight: 1
---

An internet-facing service that's behind a load balancer, orchestrated by Amazon ECS on AWS Fargate.

#### Why
* **Public**. Any application that needs to accept traffic from the public internet to perform an operation.

#### Common use-cases
* **Website**. Rendering a server-side website.
* **Public API**. A "front-end" API app that fans-out to other backend applications.

#### Architecture

<img src="https://user-images.githubusercontent.com/879348/86045951-39762880-ba01-11ea-9a47-fc9278600154.png" class="img-fluid">