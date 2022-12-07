# Service-to-Service Communication

## Service Connect <span class="version" > added in v1.24.0 </span>

[ECS Service Connect](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-connect.html) enables a client service to connect to its downstream services in a load-balanced and resilient fashion. Furthermore, it simplifies the way of exposing a service to its clients by specifying friendly aliases. With Service Connect in Copilot, each service you create is given the following private alias by default: `http://<your service name>`.

!!! attention
    Service Connect is not yet supported for [Request-Driven Web Services](../concepts/services.en.md#request-driven-web-service).

### How do I use Service Connect?
Imagine we have an app called `kudos` and two services: `api` and `front-end`, deployed in the same environment. In order to use Service Connect, both services' manifests need to have:

???+ note "Sample Service Connect manifest setups"

    === "Basic"
        ```yaml
        network:
          connect: true # Defaults to "false"
        ```

    === "Custom Alias"
        ```yaml
        network:
          connect:
            alias: frontend.local
        ```

After deploying both services, they should be able to talk to each other using the default Service Connect endpoint, which is the same as its service name. For example, `front-end` service can simply call `http://api`.

```go
// Calling the "api" service from the "front-end" service.
resp, err := http.Get("http://api/")
```

### Upgrading from Service Discovery

Prior to v1.24, Copilot enabled private service-to-service communication with [Service Discovery](#service-discovery). If you are already using Service Discovery and want to avoid any code changes, you can configure [`network.connect.alias`](../manifest/lb-web-service.en.md#network-connect-alias) field so that the Service Connect uses the same alias as Service Discovery. And if **both** the service and its client have Service Connect enabled, they'll connect via Service Connect instead of Service Discovery. For example, in the manifest of the `api` service we have

```yaml
network:
  connect:
    alias: ${COPILOT_SERVICE_NAME}.${COPILOT_ENVIRONMENT_NAME}.${COPILOT_APPLICATION_NAME}.local
```

and `front-end` also has the same setting. Then, they can keep using the same endpoint to make API calls via Service Connect instead of Service Discovery to leverage the benefits of load balancing and additional resiliency.

## Service Discovery

Service Discovery is a way of letting services discover and connect with each other. Typically, services can only talk to each other if they expose a public endpoint - and even then, requests will have to go over the internet. With [ECS Service Discovery](https://docs.aws.amazon.com/whitepapers/latest/microservices-on-aws/service-discovery.html), each service you create is given a private address and DNS name - meaning each service can talk to another without ever leaving the local network (VPC) and without exposing a public endpoint.  

### How do I use Service Discovery?

Service Discovery is enabled for all services set up using the Copilot CLI. We'll show you how to use it by using an example. Imagine we have the same `kudos` app with two services: `api` and `front-end`.

In this example we'll imagine our `front-end` service is deployed in the `test` environment, has a public endpoint and wants to call our `api` service using its service discovery endpoint.

```go
// Calling our api service from the front-end service using Service Discovery
func ServiceDiscoveryGet(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    endpoint := fmt.Sprintf("http://api.%s/some-request", os.Getenv("COPILOT_SERVICE_DISCOVERY_ENDPOINT"))
    resp, err := http.Get(endpoint /* http://api.test.kudos.local/some-request */)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer resp.Body.Close()
    body, _ := ioutil.ReadAll(resp.Body)
    w.WriteHeader(http.StatusOK)
    w.Write(body)
}
```

The important part is that our `front-end` service is making a request to our `api` service through a special endpoint:

```go
endpoint := fmt.Sprintf("http://api.%s/some-request", os.Getenv("COPILOT_SERVICE_DISCOVERY_ENDPOINT"))
```

`COPILOT_SERVICE_DISCOVERY_ENDPOINT` is a special environment variable that the Copilot CLI sets for you when it creates your service. It's of the format _{env name}.{app name}.local_ - so in this case in our _kudos_ app, when deployed in the _test_ environment, the request would be to `http://api.test.kudos.local/some-request`. Since our _api_ service is running on port 80, we're not specifying the port in the URL. However, if it was running on another port, say 8080, we'd need to include the port in the request, as well `http://api.test.kudos.local:8080/some-request`.

When our front-end makes this request, the endpoint `api.test.kudos.local` resolves to a private IP address and is routed privately within your VPC.

### Legacy Environments and Service Discovery

Prior to Copilot v1.9.0, the service discovery namespace used the format _{app name}.local_, without including the environment. This limitation made it impossible to deploy multiple environments in the same VPC. Any environments created with Copilot v1.9.0 and newer can share a VPC with any other environment.

When your environments are upgraded, Copilot will honor the service discovery namespace that the environment was created with. That means that endpoints for your services will not change. Any new environments created with Copilot v1.9.0 and above will use the _{env name}.{app name}.local_ format for service discovery, and can share VPCs with older environments.
