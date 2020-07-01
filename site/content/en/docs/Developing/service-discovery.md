---
title: "Service Discovery"
linkTitle: "Service Discovery"
weight: 3
---

Service Discovery is a way of letting services discover and connect with each other. Typically, services can only talk to each other if they expose a public endpoint - and even then, requests will have to go over the internet. With [ECS Service Discovery](https://docs.aws.amazon.com/whitepapers/latest/microservices-on-aws/service-discovery.html) each service you create is given a private address and DNS name - meaning each service can talk to each other without ever leaving the local network (VPC) and without exposing a public endpoint.  

### How Do I use Service Discovery?

Service Discovery is enabled for all services set up using the Copilot CLI. We'll show you how to use it by using an example. Imagine we have an app called `kudos` and two services, `api` and `front-end`. 

In this example we'll imagine our `front-end` service has a public endpoint and wants to call our `api` service using its service discovery endpoint. 

```golang

// Calling our api service from the front-end service using Service Discovery
func ServiceDiscoveryGet(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    endpoint := fmt.Sprintf("http://api.%s/some-request", os.Getenv("COPILOT_SERVICE_DISCOVERY_ENDPOINT"))
    resp, err := http.Get(endpoint)
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

```golang
endpoint := fmt.Sprintf("http://api.%s/some-request", os.Getenv("COPILOT_SERVICE_DISCOVERY_ENDPOINT"))
```