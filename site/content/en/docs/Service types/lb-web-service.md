---
title: "Load Balanced Web Service"
linkTitle: "Load Balanced Web Service"
weight: 1
---

An internet-facing service that's behind a load balancer, orchestrated by Amazon ECS on AWS Fargate.

#### Why
**Public**. Any service that needs to accept traffic from the public internet to perform an operation.

#### Common use-cases
**Website**. Rendering a server-side website.

**Public API**. A "front-end" API that fans-out to other backend services.

#### Architecture
![lb-web-svc](https://user-images.githubusercontent.com/879348/69385723-20d8af80-0c75-11ea-9521-ddd361a0cf64.png)