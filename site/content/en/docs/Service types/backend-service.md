---
title: "Backend Service"
linkTitle: "Backend Service"
weight: 2
---

A service that's not reachable from the internet, but can be reached with [service discovery](/docs/develop/service-discovery/) from your other services.

#### Why
**Privacy**. If your services are always accessible publicly, then you need to duplicate your authentication and authorization mechanisms across them. 
  You can remove this duplication by keeping your services private and moving the auth mechanisms into a single “frontend” load balanced web service that forwards requests to your backend services.

**Encapsulation**. If you need to migrate a service to a new major version, then a backend service will allow you to seamless migrate traffic from your "frontend" service to the new version.

#### Common use-cases
**APIs**. Any internal API service.

#### Architecture

<img src="https://user-images.githubusercontent.com/879348/78615432-5f3ac980-7826-11ea-9c03-ff0e6866152c.png" class="img-fluid">
