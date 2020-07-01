---
title: "Backend Service"
linkTitle: "Backend Service"
weight: 2
---

A service that's not reachable from the internet, but can be reached with [service discovery](https://github.com/aws/copilot-cli/wiki/Developing-With-Service-Discovery) from your other services.

#### Why
* **Privacy**. If your services are always accessible publicly, then you need to duplicate your authentication and authorization mechanisms across services. 
  You can remove this duplication by keeping your services private and moving the auth mechanisms into a single “frontend” load balanced web service that forwards requests to your backend services.
* **Encapsulation**. If you need to migrate a service to a new major version, then a backend service will allow you to seamless migrate traffic from your "frontend" service to the new version.

#### Common use-cases
* **APIs**. Any internal API service.

#### Architecture
<img src="https://user-images.githubusercontent.com/879348/86046929-e8673400-ba02-11ea-8676-addd6042e517.png" class="img-fluid">